package auth

import (
	"context"
	"strings"
	"testing"
)

// Test API key generation - authentication security critical
func TestGenerateAPIKey(t *testing.T) {
	t.Run("generates key with cfb_ prefix", func(t *testing.T) {
		rawKey, _, err := GenerateAPIKey()
		if err != nil {
			t.Fatalf("GenerateAPIKey failed: %v", err)
		}

		if !strings.HasPrefix(rawKey, "cfb_") {
			t.Errorf("expected key to start with 'cfb_', got: %s", rawKey[:10])
		}
	})

	t.Run("generates key of expected length", func(t *testing.T) {
		rawKey, _, err := GenerateAPIKey()
		if err != nil {
			t.Fatalf("GenerateAPIKey failed: %v", err)
		}

		// cfb_ (4 chars) + 40 base64 chars = 44 total
		expectedLen := 44
		if len(rawKey) != expectedLen {
			t.Errorf("expected key length %d, got %d", expectedLen, len(rawKey))
		}
	})

	t.Run("generates different keys each time", func(t *testing.T) {
		key1, _, err := GenerateAPIKey()
		if err != nil {
			t.Fatalf("GenerateAPIKey failed: %v", err)
		}

		key2, _, err := GenerateAPIKey()
		if err != nil {
			t.Fatalf("GenerateAPIKey failed: %v", err)
		}

		if key1 == key2 {
			t.Error("generated identical keys - randomness failure")
		}
	})

	t.Run("generates valid hash", func(t *testing.T) {
		_, hash, err := GenerateAPIKey()
		if err != nil {
			t.Fatalf("GenerateAPIKey failed: %v", err)
		}

		// SHA-256 hash should be 64 hex characters
		if len(hash) != 64 {
			t.Errorf("expected hash length 64, got %d", len(hash))
		}

		// Check all characters are valid hex
		for _, c := range hash {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("hash contains non-hex character: %c", c)
				break
			}
		}
	})

	t.Run("raw key and hash are different", func(t *testing.T) {
		rawKey, hash, err := GenerateAPIKey()
		if err != nil {
			t.Fatalf("GenerateAPIKey failed: %v", err)
		}

		if rawKey == hash {
			t.Error("raw key and hash should be different")
		}
	})

	t.Run("generates different hashes for different keys", func(t *testing.T) {
		_, hash1, err := GenerateAPIKey()
		if err != nil {
			t.Fatalf("GenerateAPIKey failed: %v", err)
		}

		_, hash2, err := GenerateAPIKey()
		if err != nil {
			t.Fatalf("GenerateAPIKey failed: %v", err)
		}

		if hash1 == hash2 {
			t.Error("different keys produced identical hashes")
		}
	})
}

// Test hashing consistency - critical for authentication
func TestHashAPIKey(t *testing.T) {
	t.Run("produces consistent hash for same input", func(t *testing.T) {
		key := "cfb_test_key_12345"
		hash1 := HashAPIKey(key)
		hash2 := HashAPIKey(key)

		if hash1 != hash2 {
			t.Errorf("same input produced different hashes: %s vs %s", hash1, hash2)
		}
	})

	t.Run("produces different hash for different input", func(t *testing.T) {
		key1 := "cfb_test_key_1"
		key2 := "cfb_test_key_2"

		hash1 := HashAPIKey(key1)
		hash2 := HashAPIKey(key2)

		if hash1 == hash2 {
			t.Error("different inputs produced same hash")
		}
	})

	t.Run("produces valid hex hash", func(t *testing.T) {
		key := "cfb_test_key"
		hash := HashAPIKey(key)

		// SHA-256 should produce 64 hex characters
		if len(hash) != 64 {
			t.Errorf("expected hash length 64, got %d", len(hash))
		}

		for _, c := range hash {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("hash contains non-hex character: %c", c)
				break
			}
		}
	})

	t.Run("hash changes with small input change", func(t *testing.T) {
		// This tests avalanche property of hash function
		key1 := "cfb_test_key_a"
		key2 := "cfb_test_key_b"

		hash1 := HashAPIKey(key1)
		hash2 := HashAPIKey(key2)

		// Even one character difference should produce completely different hash
		if hash1 == hash2 {
			t.Error("similar keys produced identical hashes")
		}

		// Count different characters (should be many due to avalanche effect)
		different := 0
		for i := 0; i < len(hash1); i++ {
			if hash1[i] != hash2[i] {
				different++
			}
		}

		// Expect at least 25% of characters to be different (avalanche property)
		if different < len(hash1)/4 {
			t.Errorf("only %d/%d characters different - poor avalanche effect", different, len(hash1))
		}
	})
}

// Test GetUserID context extraction
func TestGetUserID(t *testing.T) {
	t.Run("extracts user ID from context", func(t *testing.T) {
		ctx := SetUserIDForTest(context.Background(), 12345)
		userID, ok := GetUserID(ctx)

		if !ok {
			t.Fatal("expected ok=true, got false")
		}
		if userID != 12345 {
			t.Errorf("expected userID=12345, got %d", userID)
		}
	})

	t.Run("returns false for missing user ID", func(t *testing.T) {
		// Empty context
		ctx := context.Background()
		_, ok := GetUserID(ctx)

		if ok {
			t.Error("expected ok=false for empty context")
		}
	})

	t.Run("returns false for wrong type in context", func(t *testing.T) {
		// Wrong type in context
		ctx := context.WithValue(context.Background(), userIDContextKey, "not-an-int")
		_, ok := GetUserID(ctx)

		if ok {
			t.Error("expected ok=false for wrong type")
		}
	})
}
