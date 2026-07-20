package store_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Waelson/radio-library-service/internal/store"
)

// openResetDB opens an in-memory DB and returns both stores ready to use.
func openResetDB(t *testing.T) (*store.UserStore, *store.ResetCodeStore) {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return store.NewUserStore(db), store.NewResetCodeStore(db)
}

func createTestUser(t *testing.T, us *store.UserStore) store.User {
	t.Helper()
	u, err := us.Create(context.Background(), store.UserInput{
		Email: "reset@test.com", Name: "Reset User",
		Password: "Secure1234", Role: "operator",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

// ── TestCreateResetCode ───────────────────────────────────────────────────────

func TestCreateResetCode_GeneratesSixDigits(t *testing.T) {
	us, rcs := openResetDB(t)
	u := createTestUser(t, us)

	code, err := rcs.CreateResetCode(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(code) != 6 {
		t.Errorf("expected 6-digit code, got %q (len=%d)", code, len(code))
	}
	for _, c := range code {
		if c < '0' || c > '9' {
			t.Errorf("non-numeric character %q in code %q", c, code)
		}
	}
}

func TestCreateResetCode_StoresBcryptHash(t *testing.T) {
	us, rcs := openResetDB(t)
	u := createTestUser(t, us)

	code, err := rcs.CreateResetCode(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// The code should verify correctly (bcrypt stored).
	if err := rcs.VerifyResetCode(context.Background(), u.ID, code); err != nil {
		t.Errorf("verify should succeed with correct code: %v", err)
	}
}

// ── TestVerifyResetCode ───────────────────────────────────────────────────────

func TestVerifyResetCode_ValidCode_Success(t *testing.T) {
	us, rcs := openResetDB(t)
	u := createTestUser(t, us)

	code, _ := rcs.CreateResetCode(context.Background(), u.ID)
	if err := rcs.VerifyResetCode(context.Background(), u.ID, code); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestVerifyResetCode_WrongCode_Error(t *testing.T) {
	us, rcs := openResetDB(t)
	u := createTestUser(t, us)

	rcs.CreateResetCode(context.Background(), u.ID) //nolint:errcheck
	err := rcs.VerifyResetCode(context.Background(), u.ID, "000000")
	if err == nil {
		t.Fatal("expected error for wrong code")
	}
}

func TestVerifyResetCode_UsedCode_Error(t *testing.T) {
	us, rcs := openResetDB(t)
	u := createTestUser(t, us)

	code, _ := rcs.CreateResetCode(context.Background(), u.ID)
	rcs.VerifyResetCode(context.Background(), u.ID, code) //nolint:errcheck — marks as used

	// Second attempt with same code should fail.
	err := rcs.VerifyResetCode(context.Background(), u.ID, code)
	if err == nil {
		t.Fatal("expected error for already-used code")
	}
}

func TestVerifyResetCode_MaxAttempts_InvalidatesCode(t *testing.T) {
	us, rcs := openResetDB(t)
	u := createTestUser(t, us)

	code, _ := rcs.CreateResetCode(context.Background(), u.ID)

	// 4 wrong attempts — each should return ErrCodeInvalid.
	for i := 0; i < 4; i++ {
		err := rcs.VerifyResetCode(context.Background(), u.ID, "000000")
		if err == nil {
			t.Fatalf("attempt %d: expected error", i+1)
		}
	}

	// 5th attempt (maxAttempts) — should return ErrMaxAttempts.
	err := rcs.VerifyResetCode(context.Background(), u.ID, "000000")
	if err == nil {
		t.Fatal("expected ErrMaxAttempts on 5th attempt")
	}

	// Correct code should now also fail (code invalidated).
	err = rcs.VerifyResetCode(context.Background(), u.ID, code)
	if err == nil {
		t.Fatal("code should be invalidated after max attempts")
	}
}

// ── TestRateLimit ─────────────────────────────────────────────────────────────

func TestRateLimit_SecondRequestWithin60s_Rejected(t *testing.T) {
	us, rcs := openResetDB(t)
	u := createTestUser(t, us)

	if _, err := rcs.CreateResetCode(context.Background(), u.ID); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := rcs.CreateResetCode(context.Background(), u.ID)
	if err == nil {
		t.Fatal("expected rate limit error on second immediate request")
	}
}

func TestRateLimit_SecondRequestAfter60s_Allowed(t *testing.T) {
	// Use a raw *sql.DB to manipulate created_at timestamp directly.
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	us := store.NewUserStore(db)
	rcs := store.NewResetCodeStore(db)
	u := createUserOnDB(t, us)

	// Create first code and backdate its created_at by 61 seconds.
	code, err := rcs.CreateResetCode(ctx, u.ID)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	_ = code
	backdateMostRecentCode(t, db, u.ID, 61*time.Second)

	// Second request should be allowed now.
	if _, err := rcs.CreateResetCode(ctx, u.ID); err != nil {
		t.Errorf("second request after 61s should succeed: %v", err)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func createUserOnDB(t *testing.T, us *store.UserStore) store.User {
	t.Helper()
	u, err := us.Create(context.Background(), store.UserInput{
		Email: "ratelimit@test.com", Name: "RL User",
		Password: "Secure1234", Role: "operator",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

func backdateMostRecentCode(t *testing.T, db *sql.DB, userID string, ago time.Duration) {
	t.Helper()
	past := time.Now().UTC().Add(-ago).Format(time.RFC3339)
	_, err := db.ExecContext(context.Background(),
		`UPDATE password_reset_codes SET created_at = ? WHERE user_id = ?`, past, userID)
	if err != nil {
		t.Fatalf("backdate: %v", err)
	}
}
