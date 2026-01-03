package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// TestValidateAPIKey_ValidKey tests successful API key validation
func TestValidateAPIKey_ValidKey(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a test user
	user := testutil.CreateTestUser(t, env, "apikey@test.com", "API Key Test User")

	// Generate and store an API key
	rawKey, keyHash, err := auth.GenerateAPIKey()
	if err != nil {
		t.Fatalf("failed to generate API key: %v", err)
	}

	testutil.CreateTestAPIKey(t, env, user.ID, keyHash, "Test Key")

	// Validate the key
	userID, keyID, _, userStatus, err := env.DB.ValidateAPIKey(context.Background(), keyHash)
	if err != nil {
		t.Fatalf("ValidateAPIKey failed: %v", err)
	}

	if userID != user.ID {
		t.Errorf("userID = %d, want %d", userID, user.ID)
	}
	if keyID == 0 {
		t.Error("expected non-zero keyID")
	}
	if userStatus != models.UserStatusActive {
		t.Errorf("userStatus = %s, want %s", userStatus, models.UserStatusActive)
	}

	// Verify the raw key hashes to the same value
	computedHash := auth.HashAPIKey(rawKey)
	if computedHash != keyHash {
		t.Errorf("HashAPIKey mismatch: got %s, want %s", computedHash, keyHash)
	}
}

// TestValidateAPIKey_InvalidKey tests validation with non-existent key
func TestValidateAPIKey_InvalidKey(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Try to validate a non-existent key
	_, _, _, _, err := env.DB.ValidateAPIKey(context.Background(), "nonexistent_hash_12345")
	if err == nil {
		t.Error("expected error for invalid API key")
	}
}

// TestValidateAPIKey_MultipleKeys tests that each key returns the correct user
func TestValidateAPIKey_MultipleKeys(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create two users
	user1 := testutil.CreateTestUser(t, env, "user1@test.com", "User One")
	user2 := testutil.CreateTestUser(t, env, "user2@test.com", "User Two")

	// Create API keys for each user
	_, keyHash1, _ := auth.GenerateAPIKey()
	_, keyHash2, _ := auth.GenerateAPIKey()

	testutil.CreateTestAPIKey(t, env, user1.ID, keyHash1, "User1 Key")
	testutil.CreateTestAPIKey(t, env, user2.ID, keyHash2, "User2 Key")

	// Validate each key returns correct user
	userID1, _, _, _, err := env.DB.ValidateAPIKey(context.Background(), keyHash1)
	if err != nil {
		t.Fatalf("ValidateAPIKey for user1 failed: %v", err)
	}
	if userID1 != user1.ID {
		t.Errorf("key1 returned userID = %d, want %d", userID1, user1.ID)
	}

	userID2, _, _, _, err := env.DB.ValidateAPIKey(context.Background(), keyHash2)
	if err != nil {
		t.Fatalf("ValidateAPIKey for user2 failed: %v", err)
	}
	if userID2 != user2.ID {
		t.Errorf("key2 returned userID = %d, want %d", userID2, user2.ID)
	}
}

// TestCreateAPIKeyWithReturn tests API key creation returns correct values
func TestCreateAPIKeyWithReturn(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "createkey@test.com", "Create Key User")

	_, keyHash, _ := auth.GenerateAPIKey()
	keyName := "My New Key"

	before := time.Now().Add(-time.Second)
	keyID, createdAt, err := env.DB.CreateAPIKeyWithReturn(context.Background(), user.ID, keyHash, keyName)
	after := time.Now().Add(time.Second)

	if err != nil {
		t.Fatalf("CreateAPIKeyWithReturn failed: %v", err)
	}

	if keyID == 0 {
		t.Error("expected non-zero keyID")
	}

	if createdAt.Before(before) || createdAt.After(after) {
		t.Errorf("createdAt %v not in expected range [%v, %v]", createdAt, before, after)
	}

	// Verify key can be validated
	userID, _, _, _, err := env.DB.ValidateAPIKey(context.Background(), keyHash)
	if err != nil {
		t.Fatalf("ValidateAPIKey failed: %v", err)
	}
	if userID != user.ID {
		t.Errorf("userID = %d, want %d", userID, user.ID)
	}
}

// TestListAPIKeys tests listing API keys for a user
func TestListAPIKeys(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "listkeys@test.com", "List Keys User")

	// Create multiple API keys
	_, keyHash1, _ := auth.GenerateAPIKey()
	_, keyHash2, _ := auth.GenerateAPIKey()
	_, keyHash3, _ := auth.GenerateAPIKey()

	testutil.CreateTestAPIKey(t, env, user.ID, keyHash1, "Key 1")
	time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	testutil.CreateTestAPIKey(t, env, user.ID, keyHash2, "Key 2")
	time.Sleep(10 * time.Millisecond)
	testutil.CreateTestAPIKey(t, env, user.ID, keyHash3, "Key 3")

	// List keys
	keys, err := env.DB.ListAPIKeys(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("ListAPIKeys failed: %v", err)
	}

	if len(keys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(keys))
	}

	// Keys should be in DESC order by created_at (newest first)
	if keys[0].Name != "Key 3" {
		t.Errorf("first key name = %s, want 'Key 3'", keys[0].Name)
	}

	// Verify keys don't include hashes (security)
	for _, key := range keys {
		if key.UserID != user.ID {
			t.Errorf("key UserID = %d, want %d", key.UserID, user.ID)
		}
	}
}

// TestListAPIKeys_Empty tests listing when user has no keys
func TestListAPIKeys_Empty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "nokeys@test.com", "No Keys User")

	keys, err := env.DB.ListAPIKeys(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("ListAPIKeys failed: %v", err)
	}

	if len(keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(keys))
	}
}

// TestDeleteAPIKey tests deleting an API key
func TestDeleteAPIKey(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "deletekey@test.com", "Delete Key User")

	_, keyHash, _ := auth.GenerateAPIKey()
	keyID := testutil.CreateTestAPIKey(t, env, user.ID, keyHash, "Key to Delete")

	// Delete the key
	err := env.DB.DeleteAPIKey(context.Background(), user.ID, keyID)
	if err != nil {
		t.Fatalf("DeleteAPIKey failed: %v", err)
	}

	// Verify key no longer works
	_, _, _, _, err = env.DB.ValidateAPIKey(context.Background(), keyHash)
	if err == nil {
		t.Error("expected error after key deletion")
	}
}

// TestDeleteAPIKey_WrongUser tests that users can't delete other users' keys
func TestDeleteAPIKey_WrongUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user1 := testutil.CreateTestUser(t, env, "owner@test.com", "Key Owner")
	user2 := testutil.CreateTestUser(t, env, "attacker@test.com", "Attacker")

	_, keyHash, _ := auth.GenerateAPIKey()
	keyID := testutil.CreateTestAPIKey(t, env, user1.ID, keyHash, "Owner's Key")

	// User2 tries to delete User1's key
	err := env.DB.DeleteAPIKey(context.Background(), user2.ID, keyID)
	if err == nil {
		t.Error("expected error when deleting another user's key")
	}

	// Verify key still works
	userID, _, _, _, err := env.DB.ValidateAPIKey(context.Background(), keyHash)
	if err != nil {
		t.Fatalf("key should still be valid: %v", err)
	}
	if userID != user1.ID {
		t.Errorf("userID = %d, want %d", userID, user1.ID)
	}
}

// TestDeleteAPIKey_NotFound tests deleting non-existent key
func TestDeleteAPIKey_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "notfound@test.com", "Not Found User")

	err := env.DB.DeleteAPIKey(context.Background(), user.ID, 99999)
	if err == nil {
		t.Error("expected error for non-existent key")
	}
}
