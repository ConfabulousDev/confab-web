package admin_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/ConfabulousDev/confab-web/internal/admin"
	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// TestAdminMiddleware_SuperAdmin tests that super admins can access admin routes
func TestAdminMiddleware_SuperAdmin(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create an admin user
	adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin User")

	// Set this user as super admin
	os.Setenv("SUPER_ADMIN_EMAILS", "admin@example.com")
	defer os.Unsetenv("SUPER_ADMIN_EMAILS")

	// Create a handler that just returns 200 OK
	handler := admin.Middleware(env.DB)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	// Create request with admin user context
	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	ctx := context.WithValue(req.Context(), auth.GetUserIDContextKey(), adminUser.ID)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

// TestAdminMiddleware_NonAdmin tests that non-admins are rejected
func TestAdminMiddleware_NonAdmin(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a regular user
	regularUser := testutil.CreateTestUser(t, env, "user@example.com", "Regular User")

	// Set a different user as super admin
	os.Setenv("SUPER_ADMIN_EMAILS", "admin@example.com")
	defer os.Unsetenv("SUPER_ADMIN_EMAILS")

	// Create a handler that just returns 200 OK
	handler := admin.Middleware(env.DB)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	// Create request with regular user context
	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	ctx := context.WithValue(req.Context(), auth.GetUserIDContextKey(), regularUser.ID)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

// TestAdminMiddleware_NoAuth tests that unauthenticated requests are rejected
func TestAdminMiddleware_NoAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)

	handler := admin.Middleware(env.DB)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	// Create request without user context
	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// TestHandleListUsers tests listing all users
func TestHandleListUsers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create some test users
	testutil.CreateTestUser(t, env, "user1@example.com", "User One")
	testutil.CreateTestUser(t, env, "user2@example.com", "User Two")
	testutil.CreateTestUser(t, env, "admin@example.com", "Admin User")

	handlers := admin.NewHandlers(env.DB, env.Storage)

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	w := httptest.NewRecorder()

	handlers.HandleListUsers(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp struct {
		Users []models.User `json:"users"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Users) != 3 {
		t.Errorf("expected 3 users, got %d", len(resp.Users))
	}
}

// TestHandleDeactivateUser tests deactivating a user
func TestHandleDeactivateUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a user to deactivate
	user := testutil.CreateTestUser(t, env, "user@example.com", "Test User")

	handlers := admin.NewHandlers(env.DB, env.Storage)

	// Create request with chi URL param
	req := httptest.NewRequest(http.MethodPost, "/admin/users/1/deactivate", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handlers.HandleDeactivateUser(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// Verify user is now inactive
	updatedUser, err := env.DB.GetUserByID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}
	if updatedUser.Status != models.UserStatusInactive {
		t.Errorf("expected status %s, got %s", models.UserStatusInactive, updatedUser.Status)
	}
}

// TestHandleActivateUser tests reactivating a user
func TestHandleActivateUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a user and deactivate them
	user := testutil.CreateTestUser(t, env, "user@example.com", "Test User")
	if err := env.DB.UpdateUserStatus(context.Background(), user.ID, models.UserStatusInactive); err != nil {
		t.Fatalf("failed to deactivate user: %v", err)
	}

	handlers := admin.NewHandlers(env.DB, env.Storage)

	// Create request with chi URL param
	req := httptest.NewRequest(http.MethodPost, "/admin/users/1/activate", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handlers.HandleActivateUser(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// Verify user is now active
	updatedUser, err := env.DB.GetUserByID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}
	if updatedUser.Status != models.UserStatusActive {
		t.Errorf("expected status %s, got %s", models.UserStatusActive, updatedUser.Status)
	}
}

// TestHandleDeleteUser tests permanently deleting a user
func TestHandleDeleteUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a user with sessions
	user := testutil.CreateTestUser(t, env, "user@example.com", "Test User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-1")

	// Upload some test data to S3 for this session
	testContent := []byte("test transcript content")
	if _, err := env.Storage.UploadChunk(context.Background(), user.ID, sessionID, "transcript.jsonl", 1, 10, testContent); err != nil {
		t.Fatalf("failed to upload test content: %v", err)
	}

	handlers := admin.NewHandlers(env.DB, env.Storage)

	// Create request with chi URL param
	req := httptest.NewRequest(http.MethodPost, "/admin/users/1/delete", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handlers.HandleDeleteUser(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// Verify user no longer exists
	_, err := env.DB.GetUserByID(context.Background(), user.ID)
	if err == nil {
		t.Error("expected error when getting deleted user")
	}

	// Verify S3 objects are deleted (ListChunks should return empty)
	chunks, err := env.Storage.ListChunks(context.Background(), user.ID, sessionID, "transcript.jsonl")
	if err != nil {
		t.Fatalf("failed to list chunks: %v", err)
	}
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks after deletion, got %d", len(chunks))
	}
}

// TestHandleDeleteUser_InvalidID tests deleting with invalid user ID
func TestHandleDeleteUser_InvalidID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)

	handlers := admin.NewHandlers(env.DB, env.Storage)

	// Create request with invalid user ID
	req := httptest.NewRequest(http.MethodPost, "/admin/users/invalid/delete", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handlers.HandleDeleteUser(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestHandleDeleteUser_NonExistentUser tests deleting a user that doesn't exist
func TestHandleDeleteUser_NonExistentUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	handlers := admin.NewHandlers(env.DB, env.Storage)

	// Create request with non-existent user ID
	req := httptest.NewRequest(http.MethodPost, "/admin/users/99999/delete", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "99999")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handlers.HandleDeleteUser(w, req)

	// Should fail because user doesn't exist (no rows affected)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusInternalServerError, w.Code, w.Body.String())
	}
}
