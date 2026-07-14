package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Waelson/radio-library-service/internal/api/handlers"
	"github.com/Waelson/radio-library-service/internal/store"
)

// ─── fake hotkey store ────────────────────────────────────────────────────────

type fakeHotkeyStore struct {
	profiles map[string]*store.HotkeyProfile
	buttons  map[string]*store.HotkeyButton
	nextID   int
	err      error
}

func newFakeHotkeyStore() *fakeHotkeyStore {
	return &fakeHotkeyStore{
		profiles: make(map[string]*store.HotkeyProfile),
		buttons:  make(map[string]*store.HotkeyButton),
	}
}

func (f *fakeHotkeyStore) genID() string {
	f.nextID++
	return fmt.Sprintf("hk-%d", f.nextID)
}

func (f *fakeHotkeyStore) ListProfiles(_ context.Context) ([]store.HotkeyProfile, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []store.HotkeyProfile
	for _, p := range f.profiles {
		out = append(out, *p)
	}
	return out, nil
}

func (f *fakeHotkeyStore) CreateProfile(_ context.Context, name string, columns int) (store.HotkeyProfile, error) {
	if f.err != nil {
		return store.HotkeyProfile{}, f.err
	}
	if strings.TrimSpace(name) == "" {
		return store.HotkeyProfile{}, fmt.Errorf("name required")
	}
	if columns <= 0 {
		columns = 4
	}
	p := store.HotkeyProfile{
		ID: f.genID(), Name: name, Columns: columns,
		Buttons: []store.HotkeyButton{}, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	f.profiles[p.ID] = &p
	return p, nil
}

func (f *fakeHotkeyStore) FindProfileByID(_ context.Context, id string) (store.HotkeyProfile, error) {
	if f.err != nil {
		return store.HotkeyProfile{}, f.err
	}
	p, ok := f.profiles[id]
	if !ok {
		return store.HotkeyProfile{}, store.ErrNotFound
	}
	// Attach buttons.
	cp := *p
	cp.Buttons = nil
	for _, b := range f.buttons {
		if b.ProfileID == id {
			cp.Buttons = append(cp.Buttons, *b)
		}
	}
	return cp, nil
}

func (f *fakeHotkeyStore) UpdateProfile(_ context.Context, id, name string, columns int) error {
	if f.err != nil {
		return f.err
	}
	p, ok := f.profiles[id]
	if !ok {
		return store.ErrNotFound
	}
	p.Name = name
	if columns > 0 {
		p.Columns = columns
	}
	p.UpdatedAt = time.Now()
	return nil
}

func (f *fakeHotkeyStore) DeleteProfile(_ context.Context, id string) error {
	if f.err != nil {
		return f.err
	}
	delete(f.profiles, id)
	return nil
}

func (f *fakeHotkeyStore) AddButton(_ context.Context, profileID string, b store.HotkeyButton) (store.HotkeyButton, error) {
	if f.err != nil {
		return store.HotkeyButton{}, f.err
	}
	if _, ok := f.profiles[profileID]; !ok {
		return store.HotkeyButton{}, store.ErrNotFound
	}
	b.ID = f.genID()
	b.ProfileID = profileID
	b.Position = len(f.buttons) + 1
	b.CreatedAt = time.Now()
	f.buttons[b.ID] = &b
	return b, nil
}

func (f *fakeHotkeyStore) PatchButton(_ context.Context, buttonID string, patch store.HotkeyButtonPatch) (store.HotkeyButton, error) {
	if f.err != nil {
		return store.HotkeyButton{}, f.err
	}
	b, ok := f.buttons[buttonID]
	if !ok {
		return store.HotkeyButton{}, store.ErrNotFound
	}
	if patch.Label != nil {
		b.Label = *patch.Label
	}
	if patch.SubLabel != nil {
		b.SubLabel = *patch.SubLabel
	}
	if patch.TrackPath != nil {
		b.TrackPath = *patch.TrackPath
	}
	if patch.Position != nil {
		b.Position = *patch.Position
	}
	return *b, nil
}

func (f *fakeHotkeyStore) DeleteButton(_ context.Context, buttonID string) error {
	if f.err != nil {
		return f.err
	}
	delete(f.buttons, buttonID)
	return nil
}

func (f *fakeHotkeyStore) ReorderButtons(_ context.Context, profileID string, buttonIDs []string) error {
	if f.err != nil {
		return f.err
	}
	for i, id := range buttonIDs {
		b, ok := f.buttons[id]
		if !ok || b.ProfileID != profileID {
			return fmt.Errorf("button %q not in profile", id)
		}
		b.Position = i + 1
	}
	return nil
}

// ─── tests ────────────────────────────────────────────────────────────────────

func TestListHotkeyProfiles_Empty(t *testing.T) {
	hs := newFakeHotkeyStore()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/hotkeys/profiles", handlers.ListHotkeyProfiles(hs))

	r := httptest.NewRequest("GET", "/v1/hotkeys/profiles", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["ok"] != true {
		t.Errorf("expected ok=true, got %v", resp["ok"])
	}
}

func TestCreateHotkeyProfile(t *testing.T) {
	hs := newFakeHotkeyStore()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/hotkeys/profiles", handlers.CreateHotkeyProfile(hs))

	body := `{"name":"Manhã","columns":4}`
	r := httptest.NewRequest("POST", "/v1/hotkeys/profiles", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	data := resp["data"].(map[string]any)
	if data["name"] != "Manhã" {
		t.Errorf("expected name=Manhã, got %v", data["name"])
	}
}

func TestCreateHotkeyProfile_MissingName(t *testing.T) {
	hs := newFakeHotkeyStore()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/hotkeys/profiles", handlers.CreateHotkeyProfile(hs))

	r := httptest.NewRequest("POST", "/v1/hotkeys/profiles", strings.NewReader(`{"name":""}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetHotkeyProfile_NotFound(t *testing.T) {
	hs := newFakeHotkeyStore()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/hotkeys/profiles/{id}", handlers.GetHotkeyProfile(hs, fakeNR))

	r := httptest.NewRequest("GET", "/v1/hotkeys/profiles/nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestAddAndGetButton(t *testing.T) {
	hs := newFakeHotkeyStore()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/hotkeys/profiles", handlers.CreateHotkeyProfile(hs))
	mux.HandleFunc("GET /v1/hotkeys/profiles/{id}", handlers.GetHotkeyProfile(hs, fakeNR))
	mux.HandleFunc("POST /v1/hotkeys/profiles/{id}/buttons", handlers.AddHotkeyButton(hs))

	// Create profile.
	body := `{"name":"Tarde","columns":5}`
	r := httptest.NewRequest("POST", "/v1/hotkeys/profiles", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("create profile: expected 201, got %d", w.Code)
	}
	var createResp map[string]any
	_ = json.NewDecoder(w.Body).Decode(&createResp)
	profileID := createResp["data"].(map[string]any)["id"].(string)

	// Add button.
	btnBody := `{"label":"Abertura","track_path":"/audio/abertura.mp3","track_title":"Abertura","duration_ms":5000}`
	r2 := httptest.NewRequest("POST", "/v1/hotkeys/profiles/"+profileID+"/buttons", strings.NewReader(btnBody))
	r2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, r2)
	if w2.Code != http.StatusCreated {
		t.Fatalf("add button: expected 201, got %d: %s", w2.Code, w2.Body.String())
	}

	// Get profile with buttons.
	r3 := httptest.NewRequest("GET", "/v1/hotkeys/profiles/"+profileID, nil)
	w3 := httptest.NewRecorder()
	mux.ServeHTTP(w3, r3)
	if w3.Code != http.StatusOK {
		t.Fatalf("get profile: expected 200, got %d", w3.Code)
	}
	var getResp map[string]any
	_ = json.NewDecoder(w3.Body).Decode(&getResp)
	btns := getResp["data"].(map[string]any)["buttons"].([]any)
	if len(btns) != 1 {
		t.Errorf("expected 1 button, got %d", len(btns))
	}
}

func TestDeleteHotkeyProfile(t *testing.T) {
	hs := newFakeHotkeyStore()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/hotkeys/profiles", handlers.CreateHotkeyProfile(hs))
	mux.HandleFunc("DELETE /v1/hotkeys/profiles/{id}", handlers.DeleteHotkeyProfile(hs))
	mux.HandleFunc("GET /v1/hotkeys/profiles/{id}", handlers.GetHotkeyProfile(hs, fakeNR))

	// Create.
	r := httptest.NewRequest("POST", "/v1/hotkeys/profiles", strings.NewReader(`{"name":"Temp","columns":4}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	var resp map[string]any
	_ = json.NewDecoder(w.Body).Decode(&resp)
	id := resp["data"].(map[string]any)["id"].(string)

	// Delete.
	r2 := httptest.NewRequest("DELETE", "/v1/hotkeys/profiles/"+id, nil)
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, r2)
	if w2.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d", w2.Code)
	}

	// Get should 404.
	r3 := httptest.NewRequest("GET", "/v1/hotkeys/profiles/"+id, nil)
	w3 := httptest.NewRecorder()
	mux.ServeHTTP(w3, r3)
	if w3.Code != http.StatusNotFound {
		t.Fatalf("get after delete: expected 404, got %d", w3.Code)
	}
}

func TestPatchHotkeyButton(t *testing.T) {
	hs := newFakeHotkeyStore()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/hotkeys/profiles", handlers.CreateHotkeyProfile(hs))
	mux.HandleFunc("POST /v1/hotkeys/profiles/{id}/buttons", handlers.AddHotkeyButton(hs))
	mux.HandleFunc("PATCH /v1/hotkeys/buttons/{id}", handlers.PatchHotkeyButton(hs))

	// Create profile + button.
	r := httptest.NewRequest("POST", "/v1/hotkeys/profiles", strings.NewReader(`{"name":"P","columns":4}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	var pr map[string]any
	_ = json.NewDecoder(w.Body).Decode(&pr)
	pid := pr["data"].(map[string]any)["id"].(string)

	r2 := httptest.NewRequest("POST", "/v1/hotkeys/profiles/"+pid+"/buttons",
		strings.NewReader(`{"label":"Old"}`))
	r2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, r2)
	var br map[string]any
	_ = json.NewDecoder(w2.Body).Decode(&br)
	bid := br["data"].(map[string]any)["id"].(string)

	// Patch label.
	r3 := httptest.NewRequest("PATCH", "/v1/hotkeys/buttons/"+bid,
		strings.NewReader(`{"label":"New"}`))
	r3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	mux.ServeHTTP(w3, r3)
	if w3.Code != http.StatusOK {
		t.Fatalf("patch: expected 200, got %d: %s", w3.Code, w3.Body.String())
	}
	var patchResp map[string]any
	_ = json.NewDecoder(w3.Body).Decode(&patchResp)
	label := patchResp["data"].(map[string]any)["label"].(string)
	if label != "New" {
		t.Errorf("expected label=New, got %v", label)
	}
}
