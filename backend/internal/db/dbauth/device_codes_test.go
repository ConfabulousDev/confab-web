package dbauth_test

import (
	"context"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/db/dbauth"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// makeDeviceCode creates a 64-character device code for testing
// (CHAR(64) in DB pads with spaces, so we need exactly 64 chars)
func makeDeviceCode(base string) string {
	const length = 64
	if len(base) >= length {
		return base[:length]
	}
	// Pad with zeros to reach 64 chars
	padding := make([]byte, length-len(base))
	for i := range padding {
		padding[i] = '0'
	}
	return base + string(padding)
}

// TestCreateDeviceCode tests creating a device code
func TestCreateDeviceCode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbauth.Store{DB: env.DB}

	deviceCode := makeDeviceCode("create_device_code")
	userCode := "ABCD-1234"
	keyName := "My CLI"
	expiresAt := time.Now().UTC().Add(15 * time.Minute)

	err := store.CreateDeviceCode(context.Background(), deviceCode, userCode, keyName, expiresAt)
	if err != nil {
		t.Fatalf("CreateDeviceCode failed: %v", err)
	}

	// Verify it can be retrieved
	dc, err := store.GetDeviceCodeByDeviceCode(context.Background(), deviceCode)
	if err != nil {
		t.Fatalf("GetDeviceCodeByDeviceCode failed: %v", err)
	}

	if dc.DeviceCode != deviceCode {
		t.Errorf("DeviceCode = %q, want %q", dc.DeviceCode, deviceCode)
	}
	if dc.UserCode != userCode {
		t.Errorf("UserCode = %q, want %q", dc.UserCode, userCode)
	}
	if dc.KeyName != keyName {
		t.Errorf("KeyName = %s, want %s", dc.KeyName, keyName)
	}
	if dc.UserID != nil {
		t.Errorf("UserID should be nil before authorization")
	}
	if dc.AuthorizedAt != nil {
		t.Errorf("AuthorizedAt should be nil before authorization")
	}
}

// TestGetDeviceCodeByUserCode tests retrieving by user code (for web verification)
func TestGetDeviceCodeByUserCode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbauth.Store{DB: env.DB}

	deviceCode := makeDeviceCode("user_lookup_device_code")
	userCode := "WXYZ-5678"
	expiresAt := time.Now().UTC().Add(15 * time.Minute)

	testutil.CreateTestDeviceCode(t, env, deviceCode, userCode, "Test Key", expiresAt)

	dc, err := store.GetDeviceCodeByUserCode(context.Background(), userCode)
	if err != nil {
		t.Fatalf("GetDeviceCodeByUserCode failed: %v", err)
	}

	if dc.DeviceCode != deviceCode {
		t.Errorf("DeviceCode = %q, want %q", dc.DeviceCode, deviceCode)
	}
}

// TestGetDeviceCodeByUserCode_Expired tests that expired codes are rejected
func TestGetDeviceCodeByUserCode_Expired(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbauth.Store{DB: env.DB}

	deviceCode := makeDeviceCode("expired_device_code")
	userCode := "EXPR-1234"
	expiresAt := time.Now().UTC().Add(-time.Hour) // Expired 1 hour ago

	testutil.CreateTestDeviceCode(t, env, deviceCode, userCode, "Expired Key", expiresAt)

	_, err := store.GetDeviceCodeByUserCode(context.Background(), userCode)
	if err == nil {
		t.Error("expected error for expired device code")
	}
	if err != db.ErrDeviceCodeNotFound {
		t.Errorf("expected ErrDeviceCodeNotFound, got %v", err)
	}
}

// TestGetDeviceCodeByUserCode_NotFound tests non-existent code
func TestGetDeviceCodeByUserCode_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbauth.Store{DB: env.DB}

	_, err := store.GetDeviceCodeByUserCode(context.Background(), "NONEXIST-")
	if err == nil {
		t.Error("expected error for non-existent code")
	}
	if err != db.ErrDeviceCodeNotFound {
		t.Errorf("expected ErrDeviceCodeNotFound, got %v", err)
	}
}

// TestGetDeviceCodeByDeviceCode tests retrieval by device code (for CLI polling)
func TestGetDeviceCodeByDeviceCode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbauth.Store{DB: env.DB}

	deviceCode := makeDeviceCode("poll_device_code")
	userCode := "POLL-9999"
	expiresAt := time.Now().UTC().Add(15 * time.Minute)

	testutil.CreateTestDeviceCode(t, env, deviceCode, userCode, "Poll Key", expiresAt)

	dc, err := store.GetDeviceCodeByDeviceCode(context.Background(), deviceCode)
	if err != nil {
		t.Fatalf("GetDeviceCodeByDeviceCode failed: %v", err)
	}

	if dc.UserCode != userCode {
		t.Errorf("UserCode = %s, want %s", dc.UserCode, userCode)
	}
}

// TestGetDeviceCodeByDeviceCode_NotFound tests non-existent device code
func TestGetDeviceCodeByDeviceCode_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbauth.Store{DB: env.DB}

	_, err := store.GetDeviceCodeByDeviceCode(context.Background(), makeDeviceCode("nonexistent"))
	if err == nil {
		t.Error("expected error for non-existent code")
	}
	if err != db.ErrDeviceCodeNotFound {
		t.Errorf("expected ErrDeviceCodeNotFound, got %v", err)
	}
}

// TestAuthorizeDeviceCode tests authorizing a device code
func TestAuthorizeDeviceCode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbauth.Store{DB: env.DB}

	user := testutil.CreateTestUser(t, env, "authorize@test.com", "Authorize User")

	deviceCode := makeDeviceCode("auth_device_code")
	userCode := "AUTH-1234"
	expiresAt := time.Now().UTC().Add(15 * time.Minute)

	testutil.CreateTestDeviceCode(t, env, deviceCode, userCode, "Auth Key", expiresAt)

	// Authorize the code
	err := store.AuthorizeDeviceCode(context.Background(), userCode, user.ID)
	if err != nil {
		t.Fatalf("AuthorizeDeviceCode failed: %v", err)
	}

	// Verify authorization
	dc, err := store.GetDeviceCodeByDeviceCode(context.Background(), deviceCode)
	if err != nil {
		t.Fatalf("GetDeviceCodeByDeviceCode failed: %v", err)
	}

	if dc.UserID == nil {
		t.Error("UserID should be set after authorization")
	} else if *dc.UserID != user.ID {
		t.Errorf("UserID = %d, want %d", *dc.UserID, user.ID)
	}

	if dc.AuthorizedAt == nil {
		t.Error("AuthorizedAt should be set after authorization")
	}
}

// TestAuthorizeDeviceCode_Expired tests authorizing an expired code
func TestAuthorizeDeviceCode_Expired(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbauth.Store{DB: env.DB}

	user := testutil.CreateTestUser(t, env, "expired_auth@test.com", "Expired Auth User")

	deviceCode := makeDeviceCode("expired_auth_device_code")
	userCode := "EXPD-1234"
	expiresAt := time.Now().UTC().Add(-time.Hour) // Expired

	testutil.CreateTestDeviceCode(t, env, deviceCode, userCode, "Expired Key", expiresAt)

	err := store.AuthorizeDeviceCode(context.Background(), userCode, user.ID)
	if err == nil {
		t.Error("expected error for expired device code")
	}
	if err != db.ErrDeviceCodeNotFound {
		t.Errorf("expected ErrDeviceCodeNotFound, got %v", err)
	}
}

// TestAuthorizeDeviceCode_AlreadyAuthorized tests re-authorizing a code
func TestAuthorizeDeviceCode_AlreadyAuthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbauth.Store{DB: env.DB}

	user1 := testutil.CreateTestUser(t, env, "user1@test.com", "User One")
	user2 := testutil.CreateTestUser(t, env, "user2@test.com", "User Two")

	deviceCode := makeDeviceCode("already_auth_device_code")
	userCode := "ALRD-1234"
	expiresAt := time.Now().UTC().Add(15 * time.Minute)

	testutil.CreateTestDeviceCode(t, env, deviceCode, userCode, "Auth Key", expiresAt)

	// First authorization
	err := store.AuthorizeDeviceCode(context.Background(), userCode, user1.ID)
	if err != nil {
		t.Fatalf("first AuthorizeDeviceCode failed: %v", err)
	}

	// Second authorization should fail
	err = store.AuthorizeDeviceCode(context.Background(), userCode, user2.ID)
	if err == nil {
		t.Error("expected error for already authorized code")
	}
	if err != db.ErrDeviceCodeNotFound {
		t.Errorf("expected ErrDeviceCodeNotFound, got %v", err)
	}

	// Verify original user still authorized
	dc, _ := store.GetDeviceCodeByDeviceCode(context.Background(), deviceCode)
	if dc.UserID == nil || *dc.UserID != user1.ID {
		t.Error("original authorization should be preserved")
	}
}

// TestAuthorizeDeviceCode_NotFound tests authorizing non-existent code
func TestAuthorizeDeviceCode_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbauth.Store{DB: env.DB}

	user := testutil.CreateTestUser(t, env, "notfound@test.com", "Not Found User")

	err := store.AuthorizeDeviceCode(context.Background(), "NONEXI-ST", user.ID)
	if err == nil {
		t.Error("expected error for non-existent code")
	}
	if err != db.ErrDeviceCodeNotFound {
		t.Errorf("expected ErrDeviceCodeNotFound, got %v", err)
	}
}

// TestDeleteDeviceCode tests deleting a device code
func TestDeleteDeviceCode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbauth.Store{DB: env.DB}

	deviceCode := makeDeviceCode("delete_device_code")
	userCode := "DELT-1234"
	expiresAt := time.Now().UTC().Add(15 * time.Minute)

	testutil.CreateTestDeviceCode(t, env, deviceCode, userCode, "Delete Key", expiresAt)

	// Delete the code
	err := store.DeleteDeviceCode(context.Background(), deviceCode)
	if err != nil {
		t.Fatalf("DeleteDeviceCode failed: %v", err)
	}

	// Verify it's gone
	_, err = store.GetDeviceCodeByDeviceCode(context.Background(), deviceCode)
	if err != db.ErrDeviceCodeNotFound {
		t.Errorf("expected ErrDeviceCodeNotFound after delete, got %v", err)
	}
}

// TestDeviceCodeFlow_CompleteScenario tests the full device code authorization flow
func TestDeviceCodeFlow_CompleteScenario(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbauth.Store{DB: env.DB}

	user := testutil.CreateTestUser(t, env, "flow@test.com", "Flow User")

	// Step 1: CLI creates device code
	deviceCode := makeDeviceCode("flow_device_code")
	userCode := "FLOW-1234"
	keyName := "Flow CLI Key"
	expiresAt := time.Now().UTC().Add(15 * time.Minute)

	err := store.CreateDeviceCode(context.Background(), deviceCode, userCode, keyName, expiresAt)
	if err != nil {
		t.Fatalf("Step 1 - CreateDeviceCode failed: %v", err)
	}

	// Step 2: CLI polls - should get "authorization_pending"
	dc, err := store.GetDeviceCodeByDeviceCode(context.Background(), deviceCode)
	if err != nil {
		t.Fatalf("Step 2 - GetDeviceCodeByDeviceCode failed: %v", err)
	}
	if dc.AuthorizedAt != nil {
		t.Error("Step 2 - should not be authorized yet")
	}

	// Step 3: User enters code on web - web looks up by user code
	_, err = store.GetDeviceCodeByUserCode(context.Background(), userCode)
	if err != nil {
		t.Fatalf("Step 3 - GetDeviceCodeByUserCode failed: %v", err)
	}

	// Step 4: User authorizes
	err = store.AuthorizeDeviceCode(context.Background(), userCode, user.ID)
	if err != nil {
		t.Fatalf("Step 4 - AuthorizeDeviceCode failed: %v", err)
	}

	// Step 5: CLI polls again - should now be authorized
	dc, err = store.GetDeviceCodeByDeviceCode(context.Background(), deviceCode)
	if err != nil {
		t.Fatalf("Step 5 - GetDeviceCodeByDeviceCode failed: %v", err)
	}
	if dc.AuthorizedAt == nil {
		t.Error("Step 5 - should be authorized now")
	}
	if dc.UserID == nil || *dc.UserID != user.ID {
		t.Errorf("Step 5 - UserID = %v, want %d", dc.UserID, user.ID)
	}
	if dc.KeyName != keyName {
		t.Errorf("Step 5 - KeyName = %s, want %s", dc.KeyName, keyName)
	}

	// Step 6: CLI deletes device code after exchanging for API key
	err = store.DeleteDeviceCode(context.Background(), deviceCode)
	if err != nil {
		t.Fatalf("Step 6 - DeleteDeviceCode failed: %v", err)
	}

	// Verify device code is gone
	_, err = store.GetDeviceCodeByDeviceCode(context.Background(), deviceCode)
	if err != db.ErrDeviceCodeNotFound {
		t.Error("Step 6 - device code should be deleted after exchange")
	}
}
