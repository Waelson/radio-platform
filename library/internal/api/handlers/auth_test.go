package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Waelson/radio-library-service/internal/api/handlers"
	"github.com/Waelson/radio-library-service/internal/auth"
	"github.com/Waelson/radio-library-service/internal/store"
)

// ── test double ───────────────────────────────────────────────────────────────

type fakeUserStore struct {
	users map[string]store.User // keyed by email
}

func newFakeUserStore() *fakeUserStore { return &fakeUserStore{users: make(map[string]store.User)} }

func (f *fakeUserStore) add(u store.User) { f.users[u.Email] = u }

func (f *fakeUserStore) Authenticate(_ context.Context, email, password string) (store.User, error) {
	u, ok := f.users[email]
	if !ok {
		return store.User{}, store.ErrNotFound
	}
	if password != u.PasswordHash { // store plain text in tests for simplicity
		return store.User{}, store.ErrWrongPassword
	}
	return u, nil
}

func (f *fakeUserStore) GetByEmail(_ context.Context, email string) (store.User, error) {
	u, ok := f.users[email]
	if !ok {
		return store.User{}, store.ErrNotFound
	}
	return u, nil
}

func (f *fakeUserStore) GetByID(_ context.Context, id string) (store.User, error) {
	for _, u := range f.users {
		if u.ID == id {
			return u, nil
		}
	}
	return store.User{}, store.ErrNotFound
}

func (f *fakeUserStore) ChangePassword(_ context.Context, userID, currentPwd, newPwd string) error {
	for email, u := range f.users {
		if u.ID == userID {
			if u.PasswordHash != currentPwd {
				return store.ErrWrongPassword
			}
			u.PasswordHash = newPwd
			f.users[email] = u
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *fakeUserStore) ResetPassword(_ context.Context, userID, newPwd string) error {
	for email, u := range f.users {
		if u.ID == userID {
			u.PasswordHash = newPwd
			u.ForceChangePwd = false
			f.users[email] = u
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *fakeUserStore) SetPasswordHash(_ context.Context, userID, hash string, forceChange bool) error {
	for email, u := range f.users {
		if u.ID == userID {
			u.PasswordHash = hash
			u.ForceChangePwd = forceChange
			f.users[email] = u
			return nil
		}
	}
	return store.ErrNotFound
}

// ── fakeResetCodeStore ────────────────────────────────────────────────────────

type fakeResetCodeStore struct {
	codes    map[string]string // userID → plaintext code
	rateLim  map[string]bool   // userID → rate limited
	verified map[string]bool   // userID → already verified
}

func newFakeResetCodeStore() *fakeResetCodeStore {
	return &fakeResetCodeStore{
		codes:    make(map[string]string),
		rateLim:  make(map[string]bool),
		verified: make(map[string]bool),
	}
}

func (f *fakeResetCodeStore) CreateResetCode(_ context.Context, userID string) (string, error) {
	if f.rateLim[userID] {
		return "", store.ErrRateLimited
	}
	code := "123456"
	f.codes[userID] = code
	return code, nil
}

func (f *fakeResetCodeStore) VerifyResetCode(_ context.Context, userID, plainCode string) error {
	if f.verified[userID] {
		return store.ErrCodeInvalid
	}
	code, ok := f.codes[userID]
	if !ok || code != plainCode {
		return store.ErrCodeInvalid
	}
	f.verified[userID] = true
	return nil
}

// ── fakeMailer ────────────────────────────────────────────────────────────────

type fakeMailer struct{ sent []string }

func (m *fakeMailer) SendResetCode(to, _ string) error {
	m.sent = append(m.sent, to)
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func testAuthCfg() handlers.AuthConfig {
	return handlers.AuthConfig{
		JWTSecret: "test-secret-key",
		TokenTTL:  8 * time.Hour,
	}
}

func postJSON(t *testing.T, handler http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

// ── Login tests ───────────────────────────────────────────────────────────────

func TestLoginEndpoint_ValidCredentials_ReturnsJWT(t *testing.T) {
	fs := newFakeUserStore()
	fs.add(store.User{ID: "u1", Email: "op@radio.com", PasswordHash: "Pass123", Role: "operator"})

	h := handlers.Login(fs, testAuthCfg())
	w := postJSON(t, h, "/v1/auth/login", map[string]string{
		"email": "op@radio.com", "password": "Pass123",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — body: %s", w.Code, w.Body)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	data := resp["data"].(map[string]any)
	if data["token"] == "" {
		t.Error("expected non-empty token")
	}
}

func TestLoginEndpoint_InvalidCredentials_Returns401(t *testing.T) {
	fs := newFakeUserStore()
	fs.add(store.User{ID: "u2", Email: "op2@radio.com", PasswordHash: "RightPass1", Role: "operator"})

	h := handlers.Login(fs, testAuthCfg())
	w := postJSON(t, h, "/v1/auth/login", map[string]string{
		"email": "op2@radio.com", "password": "WrongPass1",
	})
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// ── Reset tests ───────────────────────────────────────────────────────────────

func TestResetRequest_UnknownEmail_Returns200(t *testing.T) {
	fs := newFakeUserStore()
	rcs := newFakeResetCodeStore()
	ml := &fakeMailer{}

	h := handlers.ResetRequest(fs, rcs, ml)
	w := postJSON(t, h, "/v1/auth/reset-request", map[string]string{"email": "ghost@radio.com"})
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for unknown email (no enumeration), got %d", w.Code)
	}
	if len(ml.sent) != 0 {
		t.Error("no e-mail should be sent for unknown address")
	}
}

func TestResetRequest_KnownEmail_SendsEmail(t *testing.T) {
	fs := newFakeUserStore()
	fs.add(store.User{ID: "u10", Email: "known@radio.com", PasswordHash: "Pass1234", Role: "operator"})
	rcs := newFakeResetCodeStore()
	ml := &fakeMailer{}

	h := handlers.ResetRequest(fs, rcs, ml)
	w := postJSON(t, h, "/v1/auth/reset-request", map[string]string{"email": "known@radio.com"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if len(ml.sent) != 1 || ml.sent[0] != "known@radio.com" {
		t.Errorf("expected e-mail sent to known@radio.com, got %v", ml.sent)
	}
}

func TestResetRequest_RateLimitExceeded_Returns429(t *testing.T) {
	fs := newFakeUserStore()
	fs.add(store.User{ID: "u11", Email: "rl@radio.com", PasswordHash: "Pass1234", Role: "operator"})
	rcs := newFakeResetCodeStore()
	rcs.rateLim["u11"] = true
	ml := &fakeMailer{}

	h := handlers.ResetRequest(fs, rcs, ml)
	w := postJSON(t, h, "/v1/auth/reset-request", map[string]string{"email": "rl@radio.com"})
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
}

func TestResetVerify_ValidCode_ReturnsResetToken(t *testing.T) {
	fs := newFakeUserStore()
	fs.add(store.User{ID: "u20", Email: "verify@radio.com", PasswordHash: "Pass1234", Role: "operator"})
	rcs := newFakeResetCodeStore()
	rcs.codes["u20"] = "123456"
	cfg := testAuthCfg()

	h := handlers.ResetVerify(fs, rcs, cfg)
	w := postJSON(t, h, "/v1/auth/reset-verify", map[string]string{
		"email": "verify@radio.com", "code": "123456",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", w.Code, w.Body)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	data := resp["data"].(map[string]any)
	if data["reset_token"] == "" {
		t.Error("expected non-empty reset_token")
	}
}

func TestResetVerify_InvalidCode_Returns400(t *testing.T) {
	fs := newFakeUserStore()
	fs.add(store.User{ID: "u21", Email: "badcode@radio.com", PasswordHash: "Pass1234", Role: "operator"})
	rcs := newFakeResetCodeStore()
	rcs.codes["u21"] = "999999"

	h := handlers.ResetVerify(fs, rcs, testAuthCfg())
	w := postJSON(t, h, "/v1/auth/reset-verify", map[string]string{
		"email": "badcode@radio.com", "code": "000000",
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestResetConfirm_ValidResetToken_UpdatesPassword(t *testing.T) {
	cfg := testAuthCfg()
	// Create a valid reset_token.
	resetToken, err := auth.Sign(auth.Claims{
		Sub:   "u30",
		Email: "confirm@radio.com",
		Scope: "reset",
	}, cfg.JWTSecret, 10*time.Minute)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	fs := newFakeUserStore()
	fs.add(store.User{ID: "u30", Email: "confirm@radio.com", PasswordHash: "OldPass1", Role: "operator"})

	h := handlers.ResetConfirm(fs, cfg)
	w := postJSON(t, h, "/v1/auth/reset-confirm", map[string]string{
		"reset_token": resetToken, "new_password": "NewPass456",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", w.Code, w.Body)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	data := resp["data"].(map[string]any)
	if data["token"] == "" {
		t.Error("expected session token in response")
	}
}

func TestResetConfirm_ExpiredResetToken_Returns401(t *testing.T) {
	cfg := testAuthCfg()
	// Create a token that expires in the past.
	expiredToken, _ := auth.Sign(auth.Claims{
		Sub:   "u31",
		Email: "expired@radio.com",
		Scope: "reset",
	}, cfg.JWTSecret, -1*time.Minute)

	fs := newFakeUserStore()
	h := handlers.ResetConfirm(fs, cfg)
	w := postJSON(t, h, "/v1/auth/reset-confirm", map[string]string{
		"reset_token": expiredToken, "new_password": "NewPass456",
	})
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestResetConfirm_WrongScope_Returns401(t *testing.T) {
	cfg := testAuthCfg()
	// Token with scope="" (regular session token) should be rejected.
	sessionToken, _ := auth.Sign(auth.Claims{
		Sub:   "u32",
		Email: "scope@radio.com",
		Role:  "operator",
	}, cfg.JWTSecret, 8*time.Hour)

	fs := newFakeUserStore()
	h := handlers.ResetConfirm(fs, cfg)
	w := postJSON(t, h, "/v1/auth/reset-confirm", map[string]string{
		"reset_token": sessionToken, "new_password": "NewPass456",
	})
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong scope, got %d", w.Code)
	}
}

// ── ChangePassword tests ──────────────────────────────────────────────────────

func TestChangePassword_Authenticated_Success(t *testing.T) {
	fs := newFakeUserStore()
	fs.add(store.User{ID: "u40", Email: "cp@radio.com", PasswordHash: "OldPass1", Role: "operator"})
	cfg := testAuthCfg()

	token, _ := auth.Sign(auth.Claims{Sub: "u40", Email: "cp@radio.com", Role: "operator"},
		cfg.JWTSecret, cfg.TokenTTL)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/change-password",
		bytes.NewBufferString(`{"current_password":"OldPass1","new_password":"NewPass456"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	// Inject claims into context manually (middleware would do this in production).
	claims, _ := auth.Verify(token, cfg.JWTSecret)
	req = req.WithContext(handlers.WithClaims(req.Context(), claims))

	w := httptest.NewRecorder()
	handlers.ChangePassword(fs).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d — %s", w.Code, w.Body)
	}
}

func TestChangePassword_WrongCurrentPassword_Error(t *testing.T) {
	fs := newFakeUserStore()
	fs.add(store.User{ID: "u41", Email: "cpwrong@radio.com", PasswordHash: "RightPass1", Role: "operator"})
	cfg := testAuthCfg()

	claims := auth.Claims{Sub: "u41", Email: "cpwrong@radio.com", Role: "operator"}
	token, _ := auth.Sign(claims, cfg.JWTSecret, cfg.TokenTTL)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/change-password",
		bytes.NewBufferString(`{"current_password":"WrongPass1","new_password":"NewPass456"}`))
	req.Header.Set("Content-Type", "application/json")
	verifClaims, _ := auth.Verify(token, cfg.JWTSecret)
	req = req.WithContext(handlers.WithClaims(req.Context(), verifClaims))

	w := httptest.NewRecorder()
	handlers.ChangePassword(fs).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// ── make fakeUserStore satisfy store.ErrRateLimited reference ─────────────────

var _ = errors.New // ensure errors import is used

func TestLoginEndpoint_ForceChangePwd_ClaimPresent(t *testing.T) {
	fs := newFakeUserStore()
	fs.add(store.User{
		ID: "u3", Email: "forced@radio.com", PasswordHash: "TempPass1",
		Role: "operator", ForceChangePwd: true,
	})

	h := handlers.Login(fs, testAuthCfg())
	w := postJSON(t, h, "/v1/auth/login", map[string]string{
		"email": "forced@radio.com", "password": "TempPass1",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	data := resp["data"].(map[string]any)
	if data["force_change_pwd"] != true {
		t.Error("expected force_change_pwd=true in response")
	}
}
