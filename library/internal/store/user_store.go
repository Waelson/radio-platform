package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/oklog/ulid/v2"
	"golang.org/x/crypto/bcrypt"
)

// ErrWrongPassword is returned when a password does not match.
var ErrWrongPassword = errors.New("wrong password")

// ErrConflict is returned when a unique constraint is violated (e.g. duplicate e-mail).
var ErrConflict = errors.New("conflict")

// ErrWeakPassword is returned when a new password does not meet complexity rules.
var ErrWeakPassword = errors.New("password must be at least 8 characters and contain one uppercase letter and one number")

// ErrSamePassword is returned when the new password equals the current one.
var ErrSamePassword = errors.New("new password must differ from the current password")

// User holds the user record fields.
type User struct {
	ID             string
	Email          string
	Name           string
	PasswordHash   string
	Role           string
	ForceChangePwd bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// UserInput carries the fields needed to create a new user.
type UserInput struct {
	Email          string
	Name           string
	Password       string
	Role           string
	ForceChangePwd bool
}

// UserStore manages user records in SQLite.
type UserStore struct {
	db *sql.DB
}

// NewUserStore creates a UserStore backed by db.
func NewUserStore(db *sql.DB) *UserStore {
	return &UserStore{db: db}
}

// Create hashes the password and inserts a new user.
func (s *UserStore) Create(ctx context.Context, in UserInput) (User, error) {
	if err := validatePasswordStrength(in.Password); err != nil {
		return User{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, fmt.Errorf("user_store: hash password: %w", err)
	}
	role := in.Role
	if role == "" {
		role = "operator"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	id := ulid.Make().String()

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO users (id, email, name, password_hash, role, force_change_pwd, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, in.Email, in.Name, string(hash), role, boolToInt(in.ForceChangePwd), now, now,
	)
	if err != nil {
		if isUniqueConstraint(err) {
			return User{}, fmt.Errorf("user_store: create: %w: email already in use", ErrConflict)
		}
		return User{}, fmt.Errorf("user_store: create: %w", err)
	}
	return s.GetByID(ctx, id)
}

// GetByEmail returns the user with the given e-mail address.
func (s *UserStore) GetByEmail(ctx context.Context, email string) (User, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, email, name, password_hash, role, force_change_pwd, created_at, updated_at
		FROM users WHERE email = ?`, email)
	return scanUser(row)
}

// GetByID returns the user with the given ID.
func (s *UserStore) GetByID(ctx context.Context, id string) (User, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, email, name, password_hash, role, force_change_pwd, created_at, updated_at
		FROM users WHERE id = ?`, id)
	return scanUser(row)
}

// Authenticate verifies email+password and returns the user on success.
// Returns ErrNotFound if the e-mail is unknown, ErrWrongPassword on mismatch.
func (s *UserStore) Authenticate(ctx context.Context, email, password string) (User, error) {
	u, err := s.GetByEmail(ctx, email)
	if err != nil {
		return User{}, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return User{}, ErrWrongPassword
	}
	return u, nil
}

// ChangePassword verifies currentPwd, validates newPwd strength, then updates the hash.
func (s *UserStore) ChangePassword(ctx context.Context, userID, currentPwd, newPwd string) error {
	u, err := s.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(currentPwd)); err != nil {
		return ErrWrongPassword
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(newPwd)); err == nil {
		return ErrSamePassword
	}
	if err := validatePasswordStrength(newPwd); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("user_store: hash new password: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.ExecContext(ctx,
		`UPDATE users SET password_hash = ?, force_change_pwd = 0, updated_at = ? WHERE id = ?`,
		string(hash), now, userID,
	)
	return err
}

// SetForceChange sets or clears the force_change_pwd flag for a user.
func (s *UserStore) SetForceChange(ctx context.Context, userID string, force bool) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET force_change_pwd = ?, updated_at = ? WHERE id = ?`,
		boolToInt(force), now, userID,
	)
	return err
}

// ResetPassword validates strength, hashes newPwd and stores it, clearing force_change_pwd.
// Used after a successful e-mail reset flow (no current password required).
func (s *UserStore) ResetPassword(ctx context.Context, userID, newPwd string) error {
	if err := validatePasswordStrength(newPwd); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("user_store: hash password: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.ExecContext(ctx,
		`UPDATE users SET password_hash = ?, force_change_pwd = 0, updated_at = ? WHERE id = ?`,
		string(hash), now, userID,
	)
	return err
}

// SetPasswordHash directly sets a pre-hashed password (used by admin reset).
func (s *UserStore) SetPasswordHash(ctx context.Context, userID, hash string, forceChange bool) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET password_hash = ?, force_change_pwd = ?, updated_at = ? WHERE id = ?`,
		hash, boolToInt(forceChange), now, userID,
	)
	return err
}

// CountAdmins returns the number of users with role 'admin'.
func (s *UserStore) CountAdmins(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&n)
	return n, err
}

// Exists returns true if at least one user exists in the database.
func (s *UserStore) Exists(ctx context.Context) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n > 0, err
}

// ── helpers ───────────────────────────────────────────────────────────────────

func scanUser(row *sql.Row) (User, error) {
	var u User
	var forceInt int
	var createdStr, updatedStr string
	err := row.Scan(
		&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.Role,
		&forceInt, &createdStr, &updatedStr,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("user_store: scan: %w", err)
	}
	u.ForceChangePwd = forceInt != 0
	u.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	u.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)
	return u, nil
}

// isUniqueConstraint reports whether err is a SQLite UNIQUE constraint violation.
func isUniqueConstraint(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// validatePasswordStrength enforces: ≥8 chars, ≥1 uppercase, ≥1 digit.
func validatePasswordStrength(pwd string) error {
	if len(pwd) < 8 {
		return ErrWeakPassword
	}
	var hasUpper, hasDigit bool
	for _, r := range pwd {
		if unicode.IsUpper(r) {
			hasUpper = true
		}
		if unicode.IsDigit(r) {
			hasDigit = true
		}
	}
	if !hasUpper || !hasDigit {
		return ErrWeakPassword
	}
	return nil
}
