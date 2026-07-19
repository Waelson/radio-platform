package store_test

import (
	"context"
	"testing"

	"github.com/Waelson/radio-library-service/internal/store"
)

func newUserStore(t *testing.T) *store.UserStore {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return store.NewUserStore(db)
}

func defaultInput(email string) store.UserInput {
	return store.UserInput{
		Email:    email,
		Name:     "Test User",
		Password: "Secure1234",
		Role:     "operator",
	}
}

func TestCreateUser_Success(t *testing.T) {
	s := newUserStore(t)
	u, err := s.Create(context.Background(), defaultInput("user@test.com"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Email != "user@test.com" {
		t.Errorf("email mismatch: got %q", u.Email)
	}
	if u.ID == "" {
		t.Error("ID must not be empty")
	}
}

func TestCreateUser_DuplicateEmail_Error(t *testing.T) {
	s := newUserStore(t)
	ctx := context.Background()
	in := defaultInput("dup@test.com")
	if _, err := s.Create(ctx, in); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := s.Create(ctx, in)
	if err == nil {
		t.Fatal("expected error for duplicate email, got nil")
	}
}

func TestCreateUser_WeakPassword_Error(t *testing.T) {
	s := newUserStore(t)
	in := defaultInput("weak@test.com")
	in.Password = "short"
	_, err := s.Create(context.Background(), in)
	if err == nil {
		t.Fatal("expected weak password error")
	}
}

func TestAuthenticateUser_CorrectPassword_ReturnsUser(t *testing.T) {
	s := newUserStore(t)
	ctx := context.Background()
	in := defaultInput("auth@test.com")
	if _, err := s.Create(ctx, in); err != nil {
		t.Fatalf("create: %v", err)
	}
	u, err := s.Authenticate(ctx, in.Email, in.Password)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Email != in.Email {
		t.Errorf("email mismatch: got %q", u.Email)
	}
}

func TestAuthenticateUser_WrongPassword_Error(t *testing.T) {
	s := newUserStore(t)
	ctx := context.Background()
	in := defaultInput("wrongpwd@test.com")
	if _, err := s.Create(ctx, in); err != nil {
		t.Fatalf("create: %v", err)
	}
	_, err := s.Authenticate(ctx, in.Email, "WrongPass1")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
}

func TestAuthenticateUser_UserNotFound_Error(t *testing.T) {
	s := newUserStore(t)
	_, err := s.Authenticate(context.Background(), "ghost@test.com", "Secure1234")
	if err == nil {
		t.Fatal("expected ErrNotFound")
	}
}

func TestForceChangePwd_SetOnCreate_WhenAdminSetsDefault(t *testing.T) {
	s := newUserStore(t)
	ctx := context.Background()
	in := defaultInput("forced@test.com")
	in.ForceChangePwd = true
	u, err := s.Create(ctx, in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !u.ForceChangePwd {
		t.Error("expected force_change_pwd=true")
	}
}

func TestChangePassword_Success(t *testing.T) {
	s := newUserStore(t)
	ctx := context.Background()
	in := defaultInput("change@test.com")
	u, err := s.Create(ctx, in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	err = s.ChangePassword(ctx, u.ID, in.Password, "NewPass456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Old password no longer works.
	if _, err := s.Authenticate(ctx, in.Email, in.Password); err == nil {
		t.Error("old password should no longer work")
	}
	// New password works.
	if _, err := s.Authenticate(ctx, in.Email, "NewPass456"); err != nil {
		t.Errorf("new password should work: %v", err)
	}
}

func TestChangePassword_SameAsCurrentPassword_Error(t *testing.T) {
	s := newUserStore(t)
	ctx := context.Background()
	in := defaultInput("same@test.com")
	u, err := s.Create(ctx, in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	err = s.ChangePassword(ctx, u.ID, in.Password, in.Password)
	if err == nil {
		t.Fatal("expected same-password error")
	}
}

func TestChangePassword_WeakPassword_Error(t *testing.T) {
	s := newUserStore(t)
	ctx := context.Background()
	in := defaultInput("weaknew@test.com")
	u, err := s.Create(ctx, in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	err = s.ChangePassword(ctx, u.ID, in.Password, "abc")
	if err == nil {
		t.Fatal("expected weak-password error")
	}
}
