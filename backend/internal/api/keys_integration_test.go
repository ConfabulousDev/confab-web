package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/santaclaude2025/confab/backend/internal/testutil"
	"github.com/santaclaude2025/confab/backend/internal/models"
)

// TestHandleCreateAPIKey_Integration tests API key creation with real database
func TestHandleCreateAPIKey_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)

	t.Run("creates API key successfully", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

		reqBody := CreateAPIKeyRequest{
			Name: "My Test Key",
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/keys", reqBody, user.ID)

		w := httptest.NewRecorder()
		handler := HandleCreateAPIKey(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		var resp CreateAPIKeyResponse
		testutil.ParseJSONResponse(t, w, &resp)

		if resp.ID == 0 {
			t.Error("expected non-zero key ID")
		}
		if resp.Name != "My Test Key" {
			t.Errorf("expected name 'My Test Key', got %s", resp.Name)
		}
		if resp.Key == "" {
			t.Error("expected non-empty API key")
		}
		// API keys are "cfb_" (4 chars) + 40 chars base64 = 44 chars total
		if len(resp.Key) != 44 {
			t.Errorf("expected API key length 44, got %d", len(resp.Key))
		}
		if !strings.HasPrefix(resp.Key, "cfb_") {
			t.Errorf("expected API key to start with 'cfb_', got %s", resp.Key[:4])
		}
		if resp.CreatedAt == "" {
			t.Error("expected non-empty created_at timestamp")
		}

		// Verify key exists in database
		var count int
		row := env.DB.QueryRow(env.Ctx,
			"SELECT COUNT(*) FROM api_keys WHERE id = $1 AND user_id = $2 AND name = $3",
			resp.ID, user.ID, "My Test Key")
		if err := row.Scan(&count); err != nil {
			t.Fatalf("failed to query api_keys: %v", err)
		}
		if count != 1 {
			t.Errorf("expected 1 API key in database, got %d", count)
		}
	})

	t.Run("creates API key with default name when not provided", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

		reqBody := CreateAPIKeyRequest{
			Name: "", // Empty - should default to "API Key"
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/keys", reqBody, user.ID)

		w := httptest.NewRecorder()
		handler := HandleCreateAPIKey(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		var resp CreateAPIKeyResponse
		testutil.ParseJSONResponse(t, w, &resp)

		if resp.Name != "API Key" {
			t.Errorf("expected default name 'API Key', got %s", resp.Name)
		}
	})

	t.Run("returns 401 for unauthenticated request", func(t *testing.T) {
		env.CleanDB(t)

		reqBody := CreateAPIKeyRequest{
			Name: "Test Key",
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/keys", reqBody, 0)
		req = req.WithContext(context.Background())

		w := httptest.NewRecorder()
		handler := HandleCreateAPIKey(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusUnauthorized)
	})
}

// TestHandleListAPIKeys_Integration tests listing API keys with real database
func TestHandleListAPIKeys_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)

	t.Run("lists all API keys for user", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

		// Create two API keys
		testutil.CreateTestAPIKey(t, env, user.ID, "hash1", "Key One")
		testutil.CreateTestAPIKey(t, env, user.ID, "hash2", "Key Two")

		req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/keys", nil, user.ID)

		w := httptest.NewRecorder()
		handler := HandleListAPIKeys(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		var keys []models.APIKey
		testutil.ParseJSONResponse(t, w, &keys)

		if len(keys) != 2 {
			t.Errorf("expected 2 keys, got %d", len(keys))
		}

		// Verify names
		names := make(map[string]bool)
		for _, key := range keys {
			names[key.Name] = true
			// Verify key_hash is NOT returned (security)
			if key.KeyHash != "" {
				t.Error("key_hash should not be returned in list response")
			}
		}

		if !names["Key One"] {
			t.Error("expected 'Key One' in response")
		}
		if !names["Key Two"] {
			t.Error("expected 'Key Two' in response")
		}
	})

	t.Run("returns empty array when user has no keys", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

		req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/keys", nil, user.ID)

		w := httptest.NewRecorder()
		handler := HandleListAPIKeys(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		var keys []models.APIKey
		testutil.ParseJSONResponse(t, w, &keys)

		if len(keys) != 0 {
			t.Errorf("expected 0 keys, got %d", len(keys))
		}
	})

	t.Run("only returns keys belonging to authenticated user", func(t *testing.T) {
		env.CleanDB(t)

		user1 := testutil.CreateTestUser(t, env, "user1@example.com", "User One")
		user2 := testutil.CreateTestUser(t, env, "user2@example.com", "User Two")

		// Create keys for both users
		testutil.CreateTestAPIKey(t, env, user1.ID, "hash1", "User1 Key")
		testutil.CreateTestAPIKey(t, env, user2.ID, "hash2", "User2 Key")

		// Request as user1
		req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/keys", nil, user1.ID)

		w := httptest.NewRecorder()
		handler := HandleListAPIKeys(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		var keys []models.APIKey
		testutil.ParseJSONResponse(t, w, &keys)

		if len(keys) != 1 {
			t.Errorf("expected 1 key for user1, got %d", len(keys))
		}

		if keys[0].Name != "User1 Key" {
			t.Errorf("expected 'User1 Key', got %s", keys[0].Name)
		}
	})

	t.Run("returns 401 for unauthenticated request", func(t *testing.T) {
		env.CleanDB(t)

		req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/keys", nil, 0)
		req = req.WithContext(context.Background())

		w := httptest.NewRecorder()
		handler := HandleListAPIKeys(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusUnauthorized)
	})
}

// TestHandleDeleteAPIKey_Integration tests deleting API keys with real database
func TestHandleDeleteAPIKey_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)

	t.Run("deletes API key successfully", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

		keyID := testutil.CreateTestAPIKey(t, env, user.ID, "hash1", "Key to Delete")

		req := testutil.AuthenticatedRequest(t, "DELETE", "/api/v1/keys/"+strconv.FormatInt(keyID, 10), nil, user.ID)

		// Add URL parameter
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", strconv.FormatInt(keyID, 10))
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleDeleteAPIKey(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusNoContent)

		// Verify key was deleted
		var count int
		row := env.DB.QueryRow(env.Ctx,
			"SELECT COUNT(*) FROM api_keys WHERE id = $1",
			keyID)
		if err := row.Scan(&count); err != nil {
			t.Fatalf("failed to query api_keys: %v", err)
		}
		if count != 0 {
			t.Errorf("expected 0 keys after deletion, got %d", count)
		}
	})

	t.Run("returns 404 when deleting non-existent key", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

		nonExistentID := int64(99999)

		req := testutil.AuthenticatedRequest(t, "DELETE", "/api/v1/keys/"+strconv.FormatInt(nonExistentID, 10), nil, user.ID)

		// Add URL parameter
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", strconv.FormatInt(nonExistentID, 10))
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleDeleteAPIKey(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusNotFound)
	})

	t.Run("prevents deleting another user's key", func(t *testing.T) {
		env.CleanDB(t)

		user1 := testutil.CreateTestUser(t, env, "user1@example.com", "User One")
		user2 := testutil.CreateTestUser(t, env, "user2@example.com", "User Two")

		// Create key for user2
		keyID := testutil.CreateTestAPIKey(t, env, user2.ID, "hash1", "User2 Key")

		// Try to delete as user1
		req := testutil.AuthenticatedRequest(t, "DELETE", "/api/v1/keys/"+strconv.FormatInt(keyID, 10), nil, user1.ID)

		// Add URL parameter
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", strconv.FormatInt(keyID, 10))
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleDeleteAPIKey(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusNotFound)

		// Verify key still exists
		var count int
		row := env.DB.QueryRow(env.Ctx,
			"SELECT COUNT(*) FROM api_keys WHERE id = $1",
			keyID)
		if err := row.Scan(&count); err != nil {
			t.Fatalf("failed to query api_keys: %v", err)
		}
		if count != 1 {
			t.Error("key should still exist after unauthorized delete attempt")
		}
	})

	t.Run("returns 400 for invalid key ID", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

		req := testutil.AuthenticatedRequest(t, "DELETE", "/api/v1/keys/invalid", nil, user.ID)

		// Add invalid URL parameter
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "invalid")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleDeleteAPIKey(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusBadRequest)

		var resp map[string]string
		testutil.ParseJSONResponse(t, w, &resp)

		if !strings.Contains(resp["error"], "Invalid key ID") {
			t.Errorf("expected error about invalid ID, got: %s", resp["error"])
		}
	})

	t.Run("returns 401 for unauthenticated request", func(t *testing.T) {
		env.CleanDB(t)

		req := testutil.AuthenticatedRequest(t, "DELETE", "/api/v1/keys/123", nil, 0)
		req = req.WithContext(context.Background())

		// Add URL parameter
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "123")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleDeleteAPIKey(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusUnauthorized)
	})
}
