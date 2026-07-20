// Package middleware provides HTTP middleware for the Library Service API.
package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Waelson/radio-library-service/internal/api/handlers"
	"github.com/Waelson/radio-library-service/internal/auth"
)

// RequireAuth returns middleware that validates a Bearer JWT on every request.
// On success the parsed Claims are stored in the request context via handlers.WithClaims.
// On failure it returns 401 and does not call the next handler.
func RequireAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearerToken(r)
			if token == "" {
				writeError(w, http.StatusUnauthorized, "missing_token", "Authorization header required")
				return
			}
			claims, err := auth.Verify(token, secret)
			if errors.Is(err, auth.ErrTokenExpired) {
				writeError(w, http.StatusUnauthorized, "token_expired", "token has expired")
				return
			}
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid_token", "token is invalid")
				return
			}
			ctx := handlers.WithClaims(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// bearerToken extracts the token from "Authorization: Bearer <token>".
func bearerToken(r *http.Request) string {
	v := r.Header.Get("Authorization")
	if !strings.HasPrefix(v, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(v, "Bearer ")
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Minimal JSON — avoids importing the api package (would create a cycle).
	w.Write([]byte(`{"ok":false,"error":"` + code + `","message":"` + message + `"}`)) //nolint:errcheck
}
