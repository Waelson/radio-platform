package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
	"golang.org/x/crypto/bcrypt"
)

const (
	resetCodeTTL       = 15 * time.Minute
	resetRateLimit     = 60 * time.Second
	maxResetAttempts   = 5
)

// Sentinel errors for reset code operations.
var (
	ErrRateLimited = errors.New("rate limited: please wait before requesting another code")
	ErrCodeExpired = errors.New("reset code has expired")
	ErrCodeInvalid = errors.New("reset code is invalid")
	ErrMaxAttempts = errors.New("maximum attempts exceeded, code has been invalidated")
)

// ResetCodeStore manages password reset codes in SQLite.
type ResetCodeStore struct {
	db *sql.DB
}

// NewResetCodeStore creates a ResetCodeStore backed by db.
func NewResetCodeStore(db *sql.DB) *ResetCodeStore {
	return &ResetCodeStore{db: db}
}

// CreateResetCode generates a random 6-digit code, stores its bcrypt hash
// and returns the plaintext code for delivery by e-mail.
// Returns ErrRateLimited if a code was already created for this user within the last 60 s.
func (s *ResetCodeStore) CreateResetCode(ctx context.Context, userID string) (string, error) {
	// Server-side rate limit: check the most recent code's created_at.
	var lastCreated sql.NullString
	if err := s.db.QueryRowContext(ctx,
		`SELECT MAX(created_at) FROM password_reset_codes WHERE user_id = ?`, userID,
	).Scan(&lastCreated); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("reset_code_store: rate limit check: %w", err)
	}
	if lastCreated.Valid && lastCreated.String != "" {
		t, _ := time.Parse(time.RFC3339, lastCreated.String)
		if time.Since(t) < resetRateLimit {
			return "", ErrRateLimited
		}
	}

	code, err := generate6Digits()
	if err != nil {
		return "", fmt.Errorf("reset_code_store: generate code: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("reset_code_store: hash code: %w", err)
	}

	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO password_reset_codes (id, user_id, code_hash, attempts, expires_at, used, created_at)
		VALUES (?, ?, ?, 0, ?, 0, ?)`,
		ulid.Make().String(), userID, string(hash),
		now.Add(resetCodeTTL).Format(time.RFC3339),
		now.Format(time.RFC3339),
	)
	if err != nil {
		return "", fmt.Errorf("reset_code_store: insert: %w", err)
	}
	return code, nil
}

// VerifyResetCode validates the plaintext code against the most recent
// unused, non-expired record for the given user.
// On success the code is marked as used and nil is returned.
// On failure a typed error is returned (ErrCodeInvalid, ErrCodeExpired, ErrMaxAttempts).
func (s *ResetCodeStore) VerifyResetCode(ctx context.Context, userID, plainCode string) error {
	var id, codeHash, expiresStr string
	var attempts, usedInt int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, code_hash, attempts, expires_at, used
		FROM password_reset_codes
		WHERE user_id = ? AND used = 0
		ORDER BY created_at DESC
		LIMIT 1`, userID,
	).Scan(&id, &codeHash, &attempts, &expiresStr, &usedInt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrCodeInvalid
	}
	if err != nil {
		return fmt.Errorf("reset_code_store: query: %w", err)
	}

	expiresAt, _ := time.Parse(time.RFC3339, expiresStr)
	if time.Now().After(expiresAt) {
		return ErrCodeExpired
	}

	newAttempts := attempts + 1

	if newAttempts >= maxResetAttempts {
		// Invalidate before returning so the code cannot be retried.
		_, _ = s.db.ExecContext(ctx,
			`UPDATE password_reset_codes SET used = 1, attempts = ? WHERE id = ?`,
			newAttempts, id,
		)
		// Always run the hash comparison to prevent timing differences that
		// could reveal whether the code was correct.
		bcrypt.CompareHashAndPassword([]byte(codeHash), []byte(plainCode)) //nolint:errcheck
		return ErrMaxAttempts
	}

	// Increment attempt counter before checking the hash.
	if _, err := s.db.ExecContext(ctx,
		`UPDATE password_reset_codes SET attempts = ? WHERE id = ?`, newAttempts, id,
	); err != nil {
		return fmt.Errorf("reset_code_store: update attempts: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(codeHash), []byte(plainCode)); err != nil {
		return ErrCodeInvalid
	}

	// Mark as used.
	_, err = s.db.ExecContext(ctx,
		`UPDATE password_reset_codes SET used = 1 WHERE id = ?`, id,
	)
	return err
}

// ── helpers ───────────────────────────────────────────────────────────────────

// generate6Digits returns a random 6-digit numeric string (000000–999999)
// using crypto/rand to avoid modulo bias.
func generate6Digits() (string, error) {
	digits := make([]byte, 6)
	for i := range digits {
		var b [1]byte
		for {
			if _, err := rand.Read(b[:]); err != nil {
				return "", err
			}
			// Reject values ≥ 250 to avoid bias (250 is the largest multiple of 10 ≤ 255).
			if b[0] < 250 {
				digits[i] = '0' + (b[0] % 10)
				break
			}
		}
	}
	return string(digits), nil
}
