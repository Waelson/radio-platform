package handlers

import (
	"context"

	"github.com/Waelson/radio-library-service/internal/auth"
)

type contextKey int

const claimsKey contextKey = iota

// WithClaims stores the JWT claims in the context.
func WithClaims(ctx context.Context, c auth.Claims) context.Context {
	return context.WithValue(ctx, claimsKey, c)
}

// ClaimsFromContext retrieves the JWT claims stored by WithClaims.
// ok is false when no claims are present.
func ClaimsFromContext(ctx context.Context) (auth.Claims, bool) {
	c, ok := ctx.Value(claimsKey).(auth.Claims)
	return c, ok
}
