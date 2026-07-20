package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Waelson/radio-library-service/internal/store"
	"golang.org/x/crypto/bcrypt"
)

// AdminUserStore is the subset of UserStore required by admin user management handlers.
type AdminUserStore interface {
	Create(ctx context.Context, in store.UserInput) (store.User, error)
	GetByID(ctx context.Context, id string) (store.User, error)
	SetPasswordHash(ctx context.Context, userID, hash string, forceChange bool) error
}

// CreateUser handles POST /v1/users.
// Only admins may create new users.
func CreateUser(us AdminUserStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok || claims.Role != "admin" {
			writeError(w, http.StatusForbidden, "forbidden", "admin role required")
			return
		}

		var body struct {
			Email         string `json:"email"`
			Name          string `json:"name"`
			Password      string `json:"password"`
			Role          string `json:"role"`
			ForceChangePwd bool  `json:"force_change_pwd"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
			return
		}
		if body.Email == "" || body.Name == "" || body.Password == "" {
			writeError(w, http.StatusBadRequest, "bad_request", "email, name and password are required")
			return
		}
		if body.Role != "" && body.Role != "admin" && body.Role != "operator" {
			writeError(w, http.StatusBadRequest, "bad_request", "role must be 'admin' or 'operator'")
			return
		}

		u, err := us.Create(r.Context(), store.UserInput{
			Email:          body.Email,
			Name:           body.Name,
			Password:       body.Password,
			Role:           body.Role,
			ForceChangePwd: body.ForceChangePwd,
		})
		if errors.Is(err, store.ErrConflict) {
			writeError(w, http.StatusConflict, "conflict", "email already in use")
			return
		}
		if errors.Is(err, store.ErrWeakPassword) {
			writeError(w, http.StatusBadRequest, "weak_password", err.Error())
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"ok": true,
			"data": map[string]any{
				"id":               u.ID,
				"email":            u.Email,
				"name":             u.Name,
				"role":             u.Role,
				"force_change_pwd": u.ForceChangePwd,
				"created_at":       u.CreatedAt,
			},
		})
	}
}

// AdminResetPassword handles POST /v1/users/{id}/reset-password.
// Sets the user's password to defaultPwd and forces a password change on next login.
// Requires the caller to have role "admin" (enforced via JWT middleware + this handler).
func AdminResetPassword(us AdminUserStore, defaultPwd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Verify caller is admin.
		claims, ok := ClaimsFromContext(r.Context())
		if !ok || claims.Role != "admin" {
			writeError(w, http.StatusForbidden, "forbidden", "admin role required")
			return
		}

		id := r.PathValue("id")
		if _, err := us.GetByID(r.Context(), id); errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "user not found")
			return
		} else if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(defaultPwd), bcrypt.DefaultCost)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not hash password")
			return
		}

		if err := us.SetPasswordHash(r.Context(), id, string(hash), true); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not reset password")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}
