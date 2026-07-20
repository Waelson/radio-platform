package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Waelson/radio-library-service/internal/auth"
	"github.com/Waelson/radio-library-service/internal/store"
)

// AuthUserStore is the subset of UserStore used by auth and admin handlers.
type AuthUserStore interface {
	Create(ctx context.Context, in store.UserInput) (store.User, error)
	Authenticate(ctx context.Context, email, password string) (store.User, error)
	GetByEmail(ctx context.Context, email string) (store.User, error)
	GetByID(ctx context.Context, id string) (store.User, error)
	ChangePassword(ctx context.Context, userID, currentPwd, newPwd string) error
	ResetPassword(ctx context.Context, userID, newPwd string) error
	SetPasswordHash(ctx context.Context, userID, hash string, forceChange bool) error
}

// ResetCodeStore is the subset of ResetCodeStore used by auth handlers.
type ResetCodeStore interface {
	CreateResetCode(ctx context.Context, userID string) (string, error)
	VerifyResetCode(ctx context.Context, userID, plainCode string) error
}

// Mailer delivers transactional e-mails.
type Mailer interface {
	SendResetCode(to, code string) error
}

// AuthConfig carries JWT signing parameters.
type AuthConfig struct {
	JWTSecret    string
	TokenTTL     time.Duration
}

// ── POST /v1/auth/login ───────────────────────────────────────────────────────

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token         string    `json:"token"`
	ExpiresAt     time.Time `json:"expires_at"`
	UserID        string    `json:"user_id"`
	Email         string    `json:"email"`
	Name          string    `json:"name"`
	Role          string    `json:"role"`
	ForceChangePwd bool     `json:"force_change_pwd"`
}

// Login handles POST /v1/auth/login.
func Login(us AuthUserStore, cfg AuthConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body", "invalid JSON body")
			return
		}
		if req.Email == "" || req.Password == "" {
			writeError(w, http.StatusBadRequest, "missing_fields", "email and password are required")
			return
		}

		u, err := us.Authenticate(r.Context(), req.Email, req.Password)
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrWrongPassword) {
			writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "authentication failed")
			return
		}

		claims := auth.Claims{
			Sub:            u.ID,
			Email:          u.Email,
			Name:           u.Name,
			Role:           u.Role,
			ForceChangePwd: u.ForceChangePwd,
		}
		token, err := auth.Sign(claims, cfg.JWTSecret, cfg.TokenTTL)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not issue token")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ok": true,
			"data": loginResponse{
				Token:          token,
				ExpiresAt:      time.Now().UTC().Add(cfg.TokenTTL),
				UserID:         u.ID,
				Email:          u.Email,
				Name:           u.Name,
				Role:           u.Role,
				ForceChangePwd: u.ForceChangePwd,
			},
		})
	}
}

// ── POST /v1/auth/reset-request ───────────────────────────────────────────────

// ResetRequest handles POST /v1/auth/reset-request.
// Always returns 200 regardless of whether the e-mail exists (prevents user enumeration).
func ResetRequest(us AuthUserStore, rcs ResetCodeStore, m Mailer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
			writeError(w, http.StatusBadRequest, "invalid_body", "email is required")
			return
		}

		u, err := us.GetByEmail(r.Context(), req.Email)
		if err != nil {
			// User not found — return 200 to prevent enumeration (RN-04).
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
			return
		}

		code, err := rcs.CreateResetCode(r.Context(), u.ID)
		if errors.Is(err, store.ErrRateLimited) {
			writeError(w, http.StatusTooManyRequests, "rate_limited",
				"please wait at least 60 seconds before requesting another code")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not generate reset code")
			return
		}

		if err := m.SendResetCode(u.Email, code); err != nil {
			// Log but do not surface SMTP errors to the client.
			_ = err
		}

		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

// ── POST /v1/auth/reset-verify ────────────────────────────────────────────────

// ResetVerify handles POST /v1/auth/reset-verify.
// Returns a short-lived reset_token (JWT with scope="reset", TTL 10 min) on success.
func ResetVerify(us AuthUserStore, rcs ResetCodeStore, cfg AuthConfig) http.HandlerFunc {
	const resetTokenTTL = 10 * time.Minute

	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Email string `json:"email"`
			Code  string `json:"code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body", "invalid JSON body")
			return
		}
		if req.Email == "" || req.Code == "" {
			writeError(w, http.StatusBadRequest, "missing_fields", "email and code are required")
			return
		}

		u, err := us.GetByEmail(r.Context(), req.Email)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_code", "invalid or expired code")
			return
		}

		if err := rcs.VerifyResetCode(r.Context(), u.ID, req.Code); err != nil {
			switch {
			case errors.Is(err, store.ErrCodeExpired):
				writeError(w, http.StatusBadRequest, "code_expired", "code has expired")
			case errors.Is(err, store.ErrMaxAttempts):
				writeError(w, http.StatusBadRequest, "max_attempts", "maximum attempts exceeded, code invalidated")
			default:
				writeError(w, http.StatusBadRequest, "invalid_code", "invalid or expired code")
			}
			return
		}

		resetToken, err := auth.Sign(auth.Claims{
			Sub:   u.ID,
			Email: u.Email,
			Scope: "reset",
		}, cfg.JWTSecret, resetTokenTTL)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not issue reset token")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ok": true,
			"data": map[string]any{
				"reset_token": resetToken,
				"expires_in":  int(resetTokenTTL.Seconds()),
			},
		})
	}
}

// ── POST /v1/auth/reset-confirm ───────────────────────────────────────────────

// ResetConfirm handles POST /v1/auth/reset-confirm.
// Accepts a reset_token (scope="reset") + new_password, updates the password
// and returns a full session JWT so the user is immediately logged in.
func ResetConfirm(us AuthUserStore, cfg AuthConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ResetToken  string `json:"reset_token"`
			NewPassword string `json:"new_password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body", "invalid JSON body")
			return
		}
		if req.ResetToken == "" || req.NewPassword == "" {
			writeError(w, http.StatusBadRequest, "missing_fields", "reset_token and new_password are required")
			return
		}

		claims, err := auth.Verify(req.ResetToken, cfg.JWTSecret)
		if errors.Is(err, auth.ErrTokenExpired) {
			writeError(w, http.StatusUnauthorized, "token_expired", "reset token has expired")
			return
		}
		if err != nil || claims.Scope != "reset" {
			writeError(w, http.StatusUnauthorized, "invalid_token", "invalid reset token")
			return
		}

		if err := us.ResetPassword(r.Context(), claims.Sub, req.NewPassword); err != nil {
			if errors.Is(err, store.ErrWeakPassword) {
				writeError(w, http.StatusBadRequest, "weak_password", err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, "internal_error", "could not update password")
			return
		}

		// Fetch fresh user data and issue a full session token.
		u, err := us.GetByID(r.Context(), claims.Sub)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not load user")
			return
		}
		sessionToken, err := auth.Sign(auth.Claims{
			Sub:            u.ID,
			Email:          u.Email,
			Name:           u.Name,
			Role:           u.Role,
			ForceChangePwd: u.ForceChangePwd,
		}, cfg.JWTSecret, cfg.TokenTTL)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not issue session token")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ok": true,
			"data": loginResponse{
				Token:          sessionToken,
				ExpiresAt:      time.Now().UTC().Add(cfg.TokenTTL),
				UserID:         u.ID,
				Email:          u.Email,
				Name:           u.Name,
				Role:           u.Role,
				ForceChangePwd: u.ForceChangePwd,
			},
		})
	}
}

// ── POST /v1/auth/change-password ─────────────────────────────────────────────

type changePwdRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// ChangePassword handles POST /v1/auth/change-password.
// Requires a valid JWT — the user ID is taken from the token claims in the request context.
func ChangePassword(us AuthUserStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}

		var req changePwdRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body", "invalid JSON body")
			return
		}
		if req.CurrentPassword == "" || req.NewPassword == "" {
			writeError(w, http.StatusBadRequest, "missing_fields", "current_password and new_password are required")
			return
		}

		err := us.ChangePassword(r.Context(), claims.Sub, req.CurrentPassword, req.NewPassword)
		if errors.Is(err, store.ErrWrongPassword) {
			writeError(w, http.StatusUnauthorized, "wrong_password", "current password is incorrect")
			return
		}
		if errors.Is(err, store.ErrSamePassword) {
			writeError(w, http.StatusBadRequest, "same_password", "new password must differ from the current one")
			return
		}
		if errors.Is(err, store.ErrWeakPassword) {
			writeError(w, http.StatusBadRequest, "weak_password", err.Error())
			return
		}
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "user not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not change password")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}
