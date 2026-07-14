package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Waelson/radio-library-service/internal/store"
)

// HotkeyStore is the store subset required by the hotkey handlers.
type HotkeyStore interface {
	ListProfiles(ctx context.Context) ([]store.HotkeyProfile, error)
	CreateProfile(ctx context.Context, name string, columns int) (store.HotkeyProfile, error)
	FindProfileByID(ctx context.Context, id string) (store.HotkeyProfile, error)
	UpdateProfile(ctx context.Context, id, name string, columns int) error
	DeleteProfile(ctx context.Context, id string) error
	AddButton(ctx context.Context, profileID string, b store.HotkeyButton) (store.HotkeyButton, error)
	PatchButton(ctx context.Context, buttonID string, patch store.HotkeyButtonPatch) (store.HotkeyButton, error)
	DeleteButton(ctx context.Context, buttonID string) error
	ReorderButtons(ctx context.Context, profileID string, buttonIDs []string) error
}

// ── JSON shapes ───────────────────────────────────────────────────────────────

type hotkeyProfileSummaryJSON struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Columns   int       `json:"columns"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type hotkeyProfileDetailJSON struct {
	ID        string             `json:"id"`
	Name      string             `json:"name"`
	Columns   int                `json:"columns"`
	Buttons   []hotkeyButtonJSON `json:"buttons"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
}

type hotkeyButtonJSON struct {
	ID          string    `json:"id"`
	ProfileID   string    `json:"profile_id"`
	Position    int       `json:"position"`
	Label       string    `json:"label"`
	SubLabel    string    `json:"sub_label"`
	Icon        string    `json:"icon"`
	Palette     int       `json:"palette"`
	TrackID     string    `json:"track_id,omitempty"`
	TrackPath   string    `json:"track_path"`
	TrackTitle  string    `json:"track_title"`
	TrackArtist string    `json:"track_artist"`
	TrackType   string    `json:"track_type"`
	DurationMS  int64     `json:"duration_ms"`
	GainDB      float64   `json:"gain_db"`
	CreatedAt   time.Time `json:"created_at"`
}

// ── Converters ────────────────────────────────────────────────────────────────

func toProfileSummary(p store.HotkeyProfile) hotkeyProfileSummaryJSON {
	return hotkeyProfileSummaryJSON{
		ID: p.ID, Name: p.Name, Columns: p.Columns,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func toProfileDetail(p store.HotkeyProfile, ns store.NormalizationSettings) hotkeyProfileDetailJSON {
	btns := make([]hotkeyButtonJSON, len(p.Buttons))
	for i, b := range p.Buttons {
		btns[i] = toButtonJSON(b, ns)
	}
	return hotkeyProfileDetailJSON{
		ID: p.ID, Name: p.Name, Columns: p.Columns,
		Buttons: btns, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func toButtonJSON(b store.HotkeyButton, ns store.NormalizationSettings) hotkeyButtonJSON {
	var gainDB float64
	if ns.Enabled && b.LoudnessLUFS != nil {
		target := ns.TargetLUFS
		if ns.PerTypeEnabled {
			switch b.TrackType {
			case "MUSIC":
				target = ns.TargetMusic
			case "JINGLE":
				target = ns.TargetJingle
			case "VINHETA":
				target = ns.TargetVinheta
			case "SPOT":
				target = ns.TargetSpot
			}
		}
		gain := target - *b.LoudnessLUFS
		if gain > ns.MaxGainDB {
			gain = ns.MaxGainDB
		}
		gainDB = gain
	}
	return hotkeyButtonJSON{
		ID: b.ID, ProfileID: b.ProfileID, Position: b.Position,
		Label: b.Label, SubLabel: b.SubLabel, Icon: b.Icon, Palette: b.Palette,
		TrackID: b.TrackID, TrackPath: b.TrackPath, TrackTitle: b.TrackTitle,
		TrackArtist: b.TrackArtist, TrackType: b.TrackType,
		DurationMS: b.DurationMS, GainDB: gainDB, CreatedAt: b.CreatedAt,
	}
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// ListHotkeyProfiles returns a handler for GET /v1/hotkeys/profiles.
func ListHotkeyProfiles(hs HotkeyStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		profiles, err := hs.ListProfiles(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		out := make([]hotkeyProfileSummaryJSON, len(profiles))
		for i, p := range profiles {
			out[i] = toProfileSummary(p)
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": out})
	}
}

// CreateHotkeyProfile returns a handler for POST /v1/hotkeys/profiles.
func CreateHotkeyProfile(hs HotkeyStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name    string `json:"name"`
			Columns int    `json:"columns"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		if req.Name == "" {
			writeError(w, http.StatusBadRequest, "missing_name", "name is required")
			return
		}
		p, err := hs.CreateProfile(r.Context(), req.Name, req.Columns)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "data": toProfileDetail(p, store.NormalizationSettings{})})
	}
}

// GetHotkeyProfile returns a handler for GET /v1/hotkeys/profiles/{id}.
func GetHotkeyProfile(hs HotkeyStore, nr NormalizationReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		p, err := hs.FindProfileByID(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "profile not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		var ns store.NormalizationSettings
		if nr != nil {
			ns, _ = nr.NormalizationSettings(r.Context())
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": toProfileDetail(p, ns)})
	}
}

// UpdateHotkeyProfile returns a handler for PUT /v1/hotkeys/profiles/{id}.
func UpdateHotkeyProfile(hs HotkeyStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var req struct {
			Name    string `json:"name"`
			Columns int    `json:"columns"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		if req.Name == "" {
			writeError(w, http.StatusBadRequest, "missing_name", "name is required")
			return
		}
		if err := hs.UpdateProfile(r.Context(), id, req.Name, req.Columns); errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "profile not found")
			return
		} else if err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		p, err := hs.FindProfileByID(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": toProfileDetail(p, store.NormalizationSettings{})})
	}
}

// DeleteHotkeyProfile returns a handler for DELETE /v1/hotkeys/profiles/{id}.
func DeleteHotkeyProfile(hs HotkeyStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := hs.DeleteProfile(r.Context(), id); err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

// AddHotkeyButton returns a handler for POST /v1/hotkeys/profiles/{id}/buttons.
func AddHotkeyButton(hs HotkeyStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		profileID := r.PathValue("id")
		var req struct {
			Label       string `json:"label"`
			SubLabel    string `json:"sub_label"`
			Icon        string `json:"icon"`
			Palette     int    `json:"palette"`
			TrackID     string `json:"track_id"`
			TrackPath   string `json:"track_path"`
			TrackTitle  string `json:"track_title"`
			TrackArtist string `json:"track_artist"`
			TrackType   string `json:"track_type"`
			DurationMS  int64  `json:"duration_ms"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		btn, err := hs.AddButton(r.Context(), profileID, store.HotkeyButton{
			Label: req.Label, SubLabel: req.SubLabel, Icon: req.Icon, Palette: req.Palette,
			TrackID: req.TrackID, TrackPath: req.TrackPath, TrackTitle: req.TrackTitle,
			TrackArtist: req.TrackArtist, TrackType: req.TrackType, DurationMS: req.DurationMS,
		})
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "profile not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "data": toButtonJSON(btn, store.NormalizationSettings{})})
	}
}

// ReorderHotkeyButtons returns a handler for PUT /v1/hotkeys/profiles/{id}/buttons/reorder.
func ReorderHotkeyButtons(hs HotkeyStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		profileID := r.PathValue("id")
		var req struct {
			ButtonIDs []string `json:"button_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		if err := hs.ReorderButtons(r.Context(), profileID, req.ButtonIDs); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "profile not found")
				return
			}
			writeError(w, http.StatusBadRequest, "reorder_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

// PatchHotkeyButton returns a handler for PATCH /v1/hotkeys/buttons/{id}.
func PatchHotkeyButton(hs HotkeyStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		buttonID := r.PathValue("id")
		var req struct {
			Label       *string `json:"label"`
			SubLabel    *string `json:"sub_label"`
			Icon        *string `json:"icon"`
			Palette     *int    `json:"palette"`
			TrackID     *string `json:"track_id"`
			TrackPath   *string `json:"track_path"`
			TrackTitle  *string `json:"track_title"`
			TrackArtist *string `json:"track_artist"`
			TrackType   *string `json:"track_type"`
			DurationMS  *int64  `json:"duration_ms"`
			Position    *int    `json:"position"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		btn, err := hs.PatchButton(r.Context(), buttonID, store.HotkeyButtonPatch{
			Label: req.Label, SubLabel: req.SubLabel, Icon: req.Icon, Palette: req.Palette,
			TrackID: req.TrackID, TrackPath: req.TrackPath, TrackTitle: req.TrackTitle,
			TrackArtist: req.TrackArtist, TrackType: req.TrackType,
			DurationMS: req.DurationMS, Position: req.Position,
		})
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "button not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": toButtonJSON(btn, store.NormalizationSettings{})})
	}
}

// DeleteHotkeyButton returns a handler for DELETE /v1/hotkeys/buttons/{id}.
func DeleteHotkeyButton(hs HotkeyStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		buttonID := r.PathValue("id")
		if err := hs.DeleteButton(r.Context(), buttonID); err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}
