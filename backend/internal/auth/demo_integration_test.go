package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/db/dbauth"
	dbuser "github.com/ConfabulousDev/confab-web/internal/db/user"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// CF-483 DB integration tests for BootstrapDemoIdentity and the
// shared-session row maintenance.

const demoCSRFSecret = "test-csrf-secret-key-32-bytes!!"

func TestBootstrapDemoIdentity_FreshDatabase(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	ctx := context.Background()
	email := "demo@confabulous.dev"

	if err := auth.BootstrapDemoIdentity(ctx, env.DB, email, demoCSRFSecret); err != nil {
		t.Fatalf("BootstrapDemoIdentity: %v", err)
	}

	// Demo user exists with the documented shape.
	userStore := &dbuser.Store{DB: env.DB}
	authStore := &dbauth.Store{DB: env.DB}
	u, err := authStore.GetUserByEmail(ctx, email)
	if err != nil {
		t.Fatalf("GetUserByEmail after bootstrap: %v", err)
	}
	if !u.ReadOnly {
		t.Errorf("demo user read_only = false, want true")
	}
	if u.Name == nil || *u.Name != "Demo" {
		got := "<nil>"
		if u.Name != nil {
			got = *u.Name
		}
		t.Errorf("demo user name = %q, want %q", got, "Demo")
	}
	isAdmin, err := authStore.IsUserAdmin(ctx, u.ID)
	if err != nil {
		t.Fatalf("IsUserAdmin: %v", err)
	}
	if isAdmin {
		t.Errorf("demo user is_admin = true, want false")
	}

	// Exactly one web_sessions row exists for the demo user with the
	// HMAC-derived ID.
	expectedID := auth.DemoSessionCookieID(demoCSRFSecret, email)
	if expectedID == "" {
		t.Fatal("DemoSessionCookieID returned empty for non-empty email")
	}
	var count int
	if err := env.DB.Conn().QueryRowContext(ctx,
		`SELECT count(*) FROM web_sessions WHERE user_id = $1`, u.ID,
	).Scan(&count); err != nil {
		t.Fatalf("count web_sessions: %v", err)
	}
	if count != 1 {
		t.Errorf("web_sessions row count = %d, want 1", count)
	}
	var actualID string
	if err := env.DB.Conn().QueryRowContext(ctx,
		`SELECT id FROM web_sessions WHERE user_id = $1`, u.ID,
	).Scan(&actualID); err != nil {
		t.Fatalf("read session id: %v", err)
	}
	if actualID != expectedID {
		t.Errorf("session id = %q, want %q", actualID, expectedID)
	}

	// silence unused-import warning when stubs return zero values
	_ = userStore
}

func TestBootstrapDemoIdentity_IdempotentReRun(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	ctx := context.Background()
	email := "demo@confabulous.dev"

	for i := 0; i < 3; i++ {
		if err := auth.BootstrapDemoIdentity(ctx, env.DB, email, demoCSRFSecret); err != nil {
			t.Fatalf("BootstrapDemoIdentity iteration %d: %v", i, err)
		}
	}

	// Still exactly one demo user and exactly one demo session row.
	var userCount int
	if err := env.DB.Conn().QueryRowContext(ctx,
		`SELECT count(*) FROM users WHERE email = $1`, email,
	).Scan(&userCount); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if userCount != 1 {
		t.Errorf("user count after 3 bootstraps = %d, want 1", userCount)
	}
	var sessionCount int
	if err := env.DB.Conn().QueryRowContext(ctx,
		`SELECT count(*) FROM web_sessions ws JOIN users u ON ws.user_id = u.id WHERE u.email = $1`, email,
	).Scan(&sessionCount); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if sessionCount != 1 {
		t.Errorf("session count after 3 bootstraps = %d, want 1", sessionCount)
	}
}

// Flipping an existing real user (with a password identity) to demo
// must strip the password row so AuthenticatePassword fails naturally,
// and must reset is_admin to false.
func TestBootstrapDemoIdentity_FlipsExistingRealUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	ctx := context.Background()
	email := "preexisting@confabulous.dev"

	// Provision a real user with a password identity AND admin flag.
	hash, err := auth.HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	authStore := &dbauth.Store{DB: env.DB}
	preU, err := authStore.CreatePasswordUser(ctx, email, hash, true /* isAdmin */)
	if err != nil {
		t.Fatalf("CreatePasswordUser: %v", err)
	}

	// Verify the password works before bootstrap so the test is
	// honest about what it's flipping.
	if _, err := authStore.AuthenticatePassword(ctx, email, "correct-horse-battery-staple"); err != nil {
		t.Fatalf("password should work pre-bootstrap: %v", err)
	}

	// Bootstrap demo with the same email.
	if err := auth.BootstrapDemoIdentity(ctx, env.DB, email, demoCSRFSecret); err != nil {
		t.Fatalf("BootstrapDemoIdentity: %v", err)
	}

	// Same user row (preserved ID, flipped flags).
	postU, err := authStore.GetUserByEmail(ctx, email)
	if err != nil {
		t.Fatalf("GetUserByEmail post-bootstrap: %v", err)
	}
	if postU.ID != preU.ID {
		t.Errorf("user ID changed: pre=%d post=%d (must reuse row)", preU.ID, postU.ID)
	}
	if !postU.ReadOnly {
		t.Error("post-bootstrap read_only = false, want true")
	}
	isAdmin, err := authStore.IsUserAdmin(ctx, postU.ID)
	if err != nil {
		t.Fatalf("IsUserAdmin: %v", err)
	}
	if isAdmin {
		t.Error("post-bootstrap is_admin = true, want false (admin must be revoked)")
	}

	// Password identity row must be gone — login fails naturally.
	if _, err := authStore.AuthenticatePassword(ctx, email, "correct-horse-battery-staple"); err == nil {
		t.Error("password still works after demo bootstrap; identity_passwords row not deleted")
	} else if err != db.ErrInvalidCredentials {
		// ErrAccountLocked is also acceptable in theory but unexpected here.
		t.Errorf("password error = %v, want ErrInvalidCredentials", err)
	}
}

// Bootstrap must prune extra web_sessions for the demo user, keeping
// the table at exactly one demo row. Simulates a real-user-flipped
// scenario where the existing user had active browser sessions.
func TestBootstrapDemoIdentity_PrunesExtraSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	ctx := context.Background()
	email := "preexisting@confabulous.dev"

	hash, _ := auth.HashPassword("placeholder-password-1234")
	authStore := &dbauth.Store{DB: env.DB}
	preU, err := authStore.CreatePasswordUser(ctx, email, hash, false)
	if err != nil {
		t.Fatalf("CreatePasswordUser: %v", err)
	}

	// Two stale browser sessions for this user.
	for _, sid := range []string{"stale-session-1", "stale-session-2"} {
		if err := authStore.CreateWebSession(ctx, sid, preU.ID, time.Now().Add(24*time.Hour)); err != nil {
			t.Fatalf("CreateWebSession(%s): %v", sid, err)
		}
	}

	if err := auth.BootstrapDemoIdentity(ctx, env.DB, email, demoCSRFSecret); err != nil {
		t.Fatalf("BootstrapDemoIdentity: %v", err)
	}

	var count int
	if err := env.DB.Conn().QueryRowContext(ctx,
		`SELECT count(*) FROM web_sessions WHERE user_id = $1`, preU.ID,
	).Scan(&count); err != nil {
		t.Fatalf("count web_sessions: %v", err)
	}
	if count != 1 {
		t.Errorf("web_sessions count = %d, want exactly 1 (pruning failed)", count)
	}
}

// Regression: BootstrapDemoIdentity with empty email is a no-op
// (defensive — main.go also short-circuits, but the function itself
// must not require the demo column to exist or do any DB writes).
func TestBootstrapDemoIdentity_EmptyEmailIsNoOp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	ctx := context.Background()
	if err := auth.BootstrapDemoIdentity(ctx, env.DB, "", demoCSRFSecret); err != nil {
		t.Fatalf("BootstrapDemoIdentity(\"\") returned error: %v", err)
	}
	// No users created.
	var count int
	if err := env.DB.Conn().QueryRowContext(ctx, `SELECT count(*) FROM users`).Scan(&count); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 0 {
		t.Errorf("created %d users with empty demo email; expected zero (no-op)", count)
	}
}
