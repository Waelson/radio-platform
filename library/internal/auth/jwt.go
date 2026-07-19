// Package auth provides JWT creation and verification for the Library Service.
// Tokens use HMAC-SHA256 (HS256) — no external dependency required.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrTokenInvalid is returned when the token signature or structure is wrong.
var ErrTokenInvalid = errors.New("token invalid")

// ErrTokenExpired is returned when the token exp claim is in the past.
var ErrTokenExpired = errors.New("token expired")

// Claims carries the user information embedded in a JWT.
type Claims struct {
	Sub            string `json:"sub"`              // user ID
	Email          string `json:"email"`
	Name           string `json:"name"`
	Role           string `json:"role"`
	ForceChangePwd bool   `json:"force_change_pwd"`
	// Scope is empty for regular session tokens.
	// Set to "reset" for short-lived password-reset tokens.
	Scope string `json:"scope,omitempty"`
	Exp   int64  `json:"exp"` // Unix timestamp
	Iat   int64  `json:"iat"` // Unix timestamp
}

var jwtHeader = base64url(mustMarshal(map[string]string{"alg": "HS256", "typ": "JWT"}))

// Sign creates a signed JWT string for the given claims.
// ttl is the token lifetime; Exp is set to now+ttl automatically.
func Sign(claims Claims, secret string, ttl time.Duration) (string, error) {
	now := time.Now().UTC()
	claims.Iat = now.Unix()
	claims.Exp = now.Add(ttl).Unix()

	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("auth: marshal claims: %w", err)
	}
	payload := base64url(payloadJSON)
	sig := sign(jwtHeader+"."+payload, secret)
	return jwtHeader + "." + payload + "." + sig, nil
}

// Verify parses and validates a JWT string.
// Returns ErrTokenInvalid for structural/signature errors, ErrTokenExpired when exp is past.
func Verify(token, secret string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, ErrTokenInvalid
	}
	expected := sign(parts[0]+"."+parts[1], secret)
	if !hmac.Equal([]byte(parts[2]), []byte(expected)) {
		return Claims{}, ErrTokenInvalid
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrTokenInvalid
	}
	var c Claims
	if err := json.Unmarshal(payloadJSON, &c); err != nil {
		return Claims{}, ErrTokenInvalid
	}
	if time.Now().Unix() > c.Exp {
		return Claims{}, ErrTokenExpired
	}
	return c, nil
}

// ── internals ─────────────────────────────────────────────────────────────────

func sign(data, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	return base64url(mac.Sum(nil))
}

func base64url(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

func mustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
