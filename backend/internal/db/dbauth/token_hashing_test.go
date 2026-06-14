package dbauth_test

import (
	"context"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/db/dbauth"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// TestWebSessionStoredHashed asserts a web session is stored under sha256(id),
// never the raw cookie value, while lookups by the raw value still round-trip
// (40hj — at-rest token disclosure).
func TestWebSessionStoredHashed(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)
	store := &dbauth.Store{DB: env.DB}
	ctx := context.Background()

	user := testutil.CreateTestUser(t, env, "sess@example.com", "Sess")
	const raw = "raw-web-session-cookie-value-xyz"
	if err := store.CreateWebSession(ctx, raw, user.ID, time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatalf("CreateWebSession: %v", err)
	}

	// The stored primary key must be the hash, NOT the raw cookie value.
	var storedID string
	if err := env.DB.QueryRow(ctx, `SELECT id FROM web_sessions WHERE user_id = $1`, user.ID).Scan(&storedID); err != nil {
		t.Fatalf("read stored id: %v", err)
	}
	if storedID == raw {
		t.Fatal("web_sessions.id stored the raw cookie value (must be hashed)")
	}
	if storedID != db.HashToken(raw) {
		t.Errorf("web_sessions.id = %q, want sha256(raw) = %q", storedID, db.HashToken(raw))
	}

	// Lookup by the raw value round-trips; a wrong value does not.
	if _, err := store.GetWebSession(ctx, raw); err != nil {
		t.Errorf("GetWebSession(raw) should succeed, got %v", err)
	}
	if _, err := store.GetWebSession(ctx, "some-other-value"); err == nil {
		t.Error("GetWebSession(wrong) should fail")
	}

	// Delete by the raw value removes the row.
	if err := store.DeleteWebSession(ctx, raw); err != nil {
		t.Fatalf("DeleteWebSession: %v", err)
	}
	if _, err := store.GetWebSession(ctx, raw); err == nil {
		t.Error("GetWebSession after delete should fail")
	}
}

// TestDeviceCodeStoredHashed asserts device_code is stored hashed (lookups by
// the raw device code round-trip) while user_code stays plaintext (per 40hj D4
// — its defense is the 8epk throttle, not at-rest hashing).
func TestDeviceCodeStoredHashed(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)
	store := &dbauth.Store{DB: env.DB}
	ctx := context.Background()

	const rawDevice = "raw-device-code-高entropy-abc123"
	const userCode = "ABCD-2345"
	if err := store.CreateDeviceCode(ctx, rawDevice, userCode, "cli", time.Now().UTC().Add(5*time.Minute)); err != nil {
		t.Fatalf("CreateDeviceCode: %v", err)
	}

	var storedDevice, storedUser string
	if err := env.DB.QueryRow(ctx,
		`SELECT device_code, user_code FROM device_codes WHERE user_code = $1`, userCode).Scan(&storedDevice, &storedUser); err != nil {
		t.Fatalf("read stored device code: %v", err)
	}
	if storedDevice == rawDevice {
		t.Fatal("device_codes.device_code stored raw (must be hashed)")
	}
	if storedDevice != db.HashToken(rawDevice) {
		t.Errorf("device_code = %q, want sha256(raw)", storedDevice)
	}
	// user_code stays plaintext (D4).
	if storedUser != userCode {
		t.Errorf("user_code should stay plaintext, got %q want %q", storedUser, userCode)
	}

	// Lookup + delete by the RAW device code round-trip.
	if _, err := store.GetDeviceCodeByDeviceCode(ctx, rawDevice); err != nil {
		t.Errorf("GetDeviceCodeByDeviceCode(raw) should succeed, got %v", err)
	}
	if err := store.DeleteDeviceCode(ctx, rawDevice); err != nil {
		t.Fatalf("DeleteDeviceCode: %v", err)
	}
	if _, err := store.GetDeviceCodeByDeviceCode(ctx, rawDevice); err == nil {
		t.Error("GetDeviceCodeByDeviceCode after delete should fail")
	}
}
