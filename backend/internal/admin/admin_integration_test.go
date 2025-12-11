package admin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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

	// Check that response is HTML and contains all users
	body := w.Body.String()
	if !strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("expected HTML response")
	}
	if !strings.Contains(body, "user1@example.com") {
		t.Error("expected user1@example.com in response")
	}
	if !strings.Contains(body, "user2@example.com") {
		t.Error("expected user2@example.com in response")
	}
	if !strings.Contains(body, "admin@example.com") {
		t.Error("expected admin@example.com in response")
	}
	if !strings.Contains(body, "3 users") {
		t.Error("expected '3 users' count in response")
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

	// Should redirect on success
	if w.Code != http.StatusSeeOther {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusSeeOther, w.Code, w.Body.String())
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

	// Should redirect on success
	if w.Code != http.StatusSeeOther {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusSeeOther, w.Code, w.Body.String())
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

	// Should redirect on success
	if w.Code != http.StatusSeeOther {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusSeeOther, w.Code, w.Body.String())
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

	// Should redirect to error page
	if w.Code != http.StatusSeeOther {
		t.Errorf("expected status %d, got %d", http.StatusSeeOther, w.Code)
	}
	location := w.Header().Get("Location")
	if !strings.Contains(location, "error=") {
		t.Errorf("expected redirect with error param, got: %s", location)
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

	// Should redirect with error because user doesn't exist
	if w.Code != http.StatusSeeOther {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusSeeOther, w.Code, w.Body.String())
	}
	location := w.Header().Get("Location")
	if !strings.Contains(location, "error=") {
		t.Errorf("expected redirect with error param, got: %s", location)
	}
}
