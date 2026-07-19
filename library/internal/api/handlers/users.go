package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/Waelson/radio-library-service/internal/store"
	"golang.org/x/crypto/bcrypt"
)

// AdminUserStore is the subset of UserStore required by admin user management handlers.
type AdminUserStore interface {
	GetByID(ctx context.Context, id string) (store.User, error)
	SetPasswordHash(ctx context.Context, userID, hash string, forceChange bool) error
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
