package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// =============================================================================
// HandleGetSession Unified Access Tests (CF-132: Canonical Session URLs)
// =============================================================================

// TestHandleGetSession_OwnerAccess tests that owners can access their sessions
func TestHandleGetSession_OwnerAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions/"+sessionID, nil, owner.ID)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sessionID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler := HandleGetSession(env.DB)
	handler(w, req)

	testutil.AssertStatus(t, w, http.StatusOK)

	var session db.SessionDetail
	testutil.ParseJSONResponse(t, w, &session)

	if session.ID != sessionID {
		t.Errorf("expected session ID %s, got %s", sessionID, session.ID)
	}
	if session.IsOwner == nil || !*session.IsOwner {
		t.Error("expected IsOwner = true for owner access")
	}
}

// TestHandleGetSession_PublicShareAccess_Unauthenticated tests unauthenticated access via public share
func TestHandleGetSession_PublicShareAccess_Unauthenticated(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Create public share
	shareToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, sessionID, shareToken, true, nil, nil)

	// Unauthenticated request (no user ID in context)
	req := httptest.NewRequest("GET", "/api/v1/sessions/"+sessionID, nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sessionID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler := HandleGetSession(env.DB)
	handler(w, req)

	testutil.AssertStatus(t, w, http.StatusOK)

	var session db.SessionDetail
	testutil.ParseJSONResponse(t, w, &session)

	if session.ID != sessionID {
		t.Errorf("expected session ID %s, got %s", sessionID, session.ID)
	}
	if session.IsOwner == nil || *session.IsOwner {
		t.Error("expected IsOwner = false for public share access")
	}
	// Hostname/username should be hidden for shared access
	if session.Hostname != nil {
		t.Error("expected Hostname = nil for shared access")
	}
}

// TestHandleGetSession_PublicShareAccess_Authenticated tests authenticated non-owner access via public share
func TestHandleGetSession_PublicShareAccess_Authenticated(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	viewer := testutil.CreateTestUser(t, env, "viewer@example.com", "Viewer")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Create public share
	shareToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, sessionID, shareToken, true, nil, nil)

	req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions/"+sessionID, nil, viewer.ID)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sessionID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler := HandleGetSession(env.DB)
	handler(w, req)

	testutil.AssertStatus(t, w, http.StatusOK)

	var session db.SessionDetail
	testutil.ParseJSONResponse(t, w, &session)

	if session.ID != sessionID {
		t.Errorf("expected session ID %s, got %s", sessionID, session.ID)
	}
	if session.IsOwner == nil || *session.IsOwner {
		t.Error("expected IsOwner = false for non-owner access")
	}
}

// TestHandleGetSession_SystemShareAccess tests authenticated user access via system share
func TestHandleGetSession_SystemShareAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	viewer := testutil.CreateTestUser(t, env, "viewer@example.com", "Viewer")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Create system share
	shareToken := testutil.GenerateShareToken()
	testutil.CreateTestSystemShare(t, env, sessionID, shareToken, nil)

	req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions/"+sessionID, nil, viewer.ID)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sessionID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler := HandleGetSession(env.DB)
	handler(w, req)

	testutil.AssertStatus(t, w, http.StatusOK)

	var session db.SessionDetail
	testutil.ParseJSONResponse(t, w, &session)

	if session.ID != sessionID {
		t.Errorf("expected session ID %s, got %s", sessionID, session.ID)
	}
	if session.IsOwner == nil || *session.IsOwner {
		t.Error("expected IsOwner = false for system share access")
	}
}

// TestHandleGetSession_RecipientShareAccess tests recipient access via private share
func TestHandleGetSession_RecipientShareAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	recipient := testutil.CreateTestUser(t, env, "recipient@example.com", "Recipient")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Create private share with recipient
	shareToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, sessionID, shareToken, false, nil, []string{"recipient@example.com"})

	req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions/"+sessionID, nil, recipient.ID)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sessionID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler := HandleGetSession(env.DB)
	handler(w, req)

	testutil.AssertStatus(t, w, http.StatusOK)

	var session db.SessionDetail
	testutil.ParseJSONResponse(t, w, &session)

	if session.ID != sessionID {
		t.Errorf("expected session ID %s, got %s", sessionID, session.ID)
	}
	if session.IsOwner == nil || *session.IsOwner {
		t.Error("expected IsOwner = false for recipient access")
	}
}

// TestHandleGetSession_NoAccess_Authenticated tests 404 for authenticated user with no access
func TestHandleGetSession_NoAccess_Authenticated(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	stranger := testutil.CreateTestUser(t, env, "stranger@example.com", "Stranger")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")
	// No shares created

	req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions/"+sessionID, nil, stranger.ID)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sessionID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler := HandleGetSession(env.DB)
	handler(w, req)

	// Should return 404 (not found) to not reveal session existence
	testutil.AssertStatus(t, w, http.StatusNotFound)
}

// TestHandleGetSession_NoAccess_Unauthenticated tests 404 for unauthenticated user with no public share
func TestHandleGetSession_NoAccess_Unauthenticated(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")
	// No shares created

	// Unauthenticated request
	req := httptest.NewRequest("GET", "/api/v1/sessions/"+sessionID, nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sessionID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler := HandleGetSession(env.DB)
	handler(w, req)

	// Should return 404 (not found) to not reveal session existence
	testutil.AssertStatus(t, w, http.StatusNotFound)
}

// TestHandleGetSession_SystemShareRequiresAuth tests that system shares require authentication
func TestHandleGetSession_SystemShareRequiresAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Create system share (no public share)
	shareToken := testutil.GenerateShareToken()
	testutil.CreateTestSystemShare(t, env, sessionID, shareToken, nil)

	// Unauthenticated request
	req := httptest.NewRequest("GET", "/api/v1/sessions/"+sessionID, nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sessionID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler := HandleGetSession(env.DB)
	handler(w, req)

	// Should return 404 (system shares require auth, so unauthenticated user gets no access)
	testutil.AssertStatus(t, w, http.StatusNotFound)
}

// TestHandleGetSession_PrivateShareRequiresAuth tests that private shares require authentication
func TestHandleGetSession_PrivateShareRequiresAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	testutil.CreateTestUser(t, env, "recipient@example.com", "Recipient")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Create private share
	shareToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, sessionID, shareToken, false, nil, []string{"recipient@example.com"})

	// Unauthenticated request
	req := httptest.NewRequest("GET", "/api/v1/sessions/"+sessionID, nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sessionID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler := HandleGetSession(env.DB)
	handler(w, req)

	// Should return 404 (private shares require auth)
	testutil.AssertStatus(t, w, http.StatusNotFound)
}

// TestHandleGetSession_InactiveOwnerBlocksAccess tests that deactivated owner blocks all access
func TestHandleGetSession_InactiveOwnerBlocksAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Create public share
	shareToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, sessionID, shareToken, true, nil, nil)

	// Deactivate owner
	err := env.DB.UpdateUserStatus(context.Background(), owner.ID, "inactive")
	if err != nil {
		t.Fatalf("failed to deactivate owner: %v", err)
	}

	// Try to access the session
	req := httptest.NewRequest("GET", "/api/v1/sessions/"+sessionID, nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sessionID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler := HandleGetSession(env.DB)
	handler(w, req)

	// Should return 403 (forbidden due to inactive owner)
	testutil.AssertStatus(t, w, http.StatusForbidden)
}

// TestHandleGetSession_SessionNotFound tests 404 for non-existent session
func TestHandleGetSession_SessionNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	viewer := testutil.CreateTestUser(t, env, "viewer@example.com", "Viewer")

	req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions/00000000-0000-0000-0000-000000000000", nil, viewer.ID)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "00000000-0000-0000-0000-000000000000")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler := HandleGetSession(env.DB)
	handler(w, req)

	testutil.AssertStatus(t, w, http.StatusNotFound)
}

// TestHandleGetSession_OwnerHostnameUsernameVisible tests that owners see hostname/username
func TestHandleGetSession_OwnerHostnameUsernameVisible(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Add hostname/username to session
	_, err := env.DB.Exec(env.Ctx,
		"UPDATE sessions SET hostname = 'test-host', username = 'test-user' WHERE id = $1",
		sessionID)
	if err != nil {
		t.Fatalf("failed to update session: %v", err)
	}

	req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions/"+sessionID, nil, owner.ID)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sessionID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler := HandleGetSession(env.DB)
	handler(w, req)

	testutil.AssertStatus(t, w, http.StatusOK)

	var session db.SessionDetail
	testutil.ParseJSONResponse(t, w, &session)

	if session.Hostname == nil || *session.Hostname != "test-host" {
		t.Errorf("expected Hostname = 'test-host', got %v", session.Hostname)
	}
	if session.Username == nil || *session.Username != "test-user" {
		t.Errorf("expected Username = 'test-user', got %v", session.Username)
	}
}

// TestHandleGetSession_SharedAccessHostnameUsernameHidden tests that shared access hides hostname/username
func TestHandleGetSession_SharedAccessHostnameUsernameHidden(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	viewer := testutil.CreateTestUser(t, env, "viewer@example.com", "Viewer")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Add hostname/username to session
	_, err := env.DB.Exec(env.Ctx,
		"UPDATE sessions SET hostname = 'test-host', username = 'test-user' WHERE id = $1",
		sessionID)
	if err != nil {
		t.Fatalf("failed to update session: %v", err)
	}

	// Create public share
	shareToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, sessionID, shareToken, true, nil, nil)

	req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions/"+sessionID, nil, viewer.ID)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sessionID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler := HandleGetSession(env.DB)
	handler(w, req)

	testutil.AssertStatus(t, w, http.StatusOK)

	var session db.SessionDetail
	testutil.ParseJSONResponse(t, w, &session)

	if session.Hostname != nil {
		t.Error("expected Hostname = nil for shared access")
	}
	if session.Username != nil {
		t.Error("expected Username = nil for shared access")
	}
}

// =============================================================================
// Security Policy Tests - Comprehensive Edge Cases
// =============================================================================

// TestHandleGetSession_InvalidUUID tests 404 for malformed UUID
func TestHandleGetSession_InvalidUUID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Try various invalid UUIDs (avoid spaces as they break HTTP request parsing)
	invalidIDs := []string{
		"not-a-uuid",
		"12345",
		"",
		"../../../etc/passwd",
		"';DROP_TABLE_sessions;--",
	}

	for _, invalidID := range invalidIDs {
		t.Run("InvalidID_"+invalidID, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/sessions/"+invalidID, nil)

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", invalidID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			w := httptest.NewRecorder()
			handler := HandleGetSession(env.DB)
			handler(w, req)

			// All invalid IDs should return 404 or 400, never 500
			if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
				t.Errorf("expected 404 or 400 for invalid UUID %q, got %d", invalidID, w.Code)
			}
		})
	}
}

// TestHandleGetSession_ExpiredPublicShare tests that expired public shares deny access
func TestHandleGetSession_ExpiredPublicShare(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Create public share that's already expired
	shareToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, sessionID, shareToken, true, nil, nil)

	// Expire the share
	_, err := env.DB.Exec(env.Ctx,
		"UPDATE session_shares SET expires_at = NOW() - INTERVAL '1 hour' WHERE share_token = $1",
		shareToken)
	if err != nil {
		t.Fatalf("failed to expire share: %v", err)
	}

	// Unauthenticated request
	req := httptest.NewRequest("GET", "/api/v1/sessions/"+sessionID, nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sessionID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler := HandleGetSession(env.DB)
	handler(w, req)

	// Expired share = no access = 404
	testutil.AssertStatus(t, w, http.StatusNotFound)
}

// TestHandleGetSession_ExpiredSystemShare tests that expired system shares deny access
func TestHandleGetSession_ExpiredSystemShare(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	viewer := testutil.CreateTestUser(t, env, "viewer@example.com", "Viewer")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Create system share
	shareToken := testutil.GenerateShareToken()
	testutil.CreateTestSystemShare(t, env, sessionID, shareToken, nil)

	// Expire the share
	_, err := env.DB.Exec(env.Ctx,
		"UPDATE session_shares SET expires_at = NOW() - INTERVAL '1 hour' WHERE share_token = $1",
		shareToken)
	if err != nil {
		t.Fatalf("failed to expire share: %v", err)
	}

	req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions/"+sessionID, nil, viewer.ID)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sessionID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler := HandleGetSession(env.DB)
	handler(w, req)

	// Expired share = no access = 404
	testutil.AssertStatus(t, w, http.StatusNotFound)
}

// TestHandleGetSession_ExpiredRecipientShare tests that expired recipient shares deny access
func TestHandleGetSession_ExpiredRecipientShare(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	recipient := testutil.CreateTestUser(t, env, "recipient@example.com", "Recipient")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Create private share with recipient
	shareToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, sessionID, shareToken, false, nil, []string{"recipient@example.com"})

	// Expire the share
	_, err := env.DB.Exec(env.Ctx,
		"UPDATE session_shares SET expires_at = NOW() - INTERVAL '1 hour' WHERE share_token = $1",
		shareToken)
	if err != nil {
		t.Fatalf("failed to expire share: %v", err)
	}

	req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions/"+sessionID, nil, recipient.ID)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sessionID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler := HandleGetSession(env.DB)
	handler(w, req)

	// Expired share = no access = 404
	testutil.AssertStatus(t, w, http.StatusNotFound)
}

// TestHandleGetSession_WrongRecipient tests that wrong user can't access private share
func TestHandleGetSession_WrongRecipient(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	intendedRecipient := testutil.CreateTestUser(t, env, "intended@example.com", "Intended")
	wrongUser := testutil.CreateTestUser(t, env, "wrong@example.com", "Wrong")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Create private share for intended recipient only
	shareToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, sessionID, shareToken, false, nil, []string{"intended@example.com"})

	// Wrong user tries to access
	req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions/"+sessionID, nil, wrongUser.ID)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sessionID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler := HandleGetSession(env.DB)
	handler(w, req)

	// Wrong user = no access = 404
	testutil.AssertStatus(t, w, http.StatusNotFound)

	// Verify intended recipient CAN access
	req2 := testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions/"+sessionID, nil, intendedRecipient.ID)
	rctx2 := chi.NewRouteContext()
	rctx2.URLParams.Add("id", sessionID)
	req2 = req2.WithContext(context.WithValue(req2.Context(), chi.RouteCtxKey, rctx2))

	w2 := httptest.NewRecorder()
	handler(w2, req2)
	testutil.AssertStatus(t, w2, http.StatusOK)
}

// TestHandleGetSession_Precedence_OwnerOverRecipient tests owner access takes precedence
func TestHandleGetSession_Precedence_OwnerOverRecipient(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Add hostname/username
	_, err := env.DB.Exec(env.Ctx,
		"UPDATE sessions SET hostname = 'secret-host', username = 'secret-user' WHERE id = $1",
		sessionID)
	if err != nil {
		t.Fatalf("failed to update session: %v", err)
	}

	// Owner is also a recipient (weird but possible)
	shareToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, sessionID, shareToken, false, nil, []string{"owner@example.com"})

	req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions/"+sessionID, nil, owner.ID)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sessionID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler := HandleGetSession(env.DB)
	handler(w, req)

	testutil.AssertStatus(t, w, http.StatusOK)

	var session db.SessionDetail
	testutil.ParseJSONResponse(t, w, &session)

	// Owner should see hostname/username (owner access, not recipient access)
	if session.IsOwner == nil || !*session.IsOwner {
		t.Error("expected IsOwner = true (owner access takes precedence)")
	}
	if session.Hostname == nil || *session.Hostname != "secret-host" {
		t.Error("expected hostname to be visible for owner")
	}
}

// TestHandleGetSession_Precedence_RecipientOverSystem tests recipient access takes precedence over system
func TestHandleGetSession_Precedence_RecipientOverSystem(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	recipient := testutil.CreateTestUser(t, env, "recipient@example.com", "Recipient")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Create both system share and recipient share
	systemToken := testutil.GenerateShareToken()
	testutil.CreateTestSystemShare(t, env, sessionID, systemToken, nil)

	recipientToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, sessionID, recipientToken, false, nil, []string{"recipient@example.com"})

	req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions/"+sessionID, nil, recipient.ID)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sessionID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler := HandleGetSession(env.DB)
	handler(w, req)

	testutil.AssertStatus(t, w, http.StatusOK)

	// Access granted - recipient share takes precedence
	// (We can't directly verify which share was used, but access should work)
	var session db.SessionDetail
	testutil.ParseJSONResponse(t, w, &session)
	if session.ID != sessionID {
		t.Errorf("expected session ID %s, got %s", sessionID, session.ID)
	}
}

// TestHandleGetSession_Precedence_SystemOverPublic tests system access takes precedence over public
func TestHandleGetSession_Precedence_SystemOverPublic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	viewer := testutil.CreateTestUser(t, env, "viewer@example.com", "Viewer")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Create both public share and system share
	publicToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, sessionID, publicToken, true, nil, nil)

	systemToken := testutil.GenerateShareToken()
	testutil.CreateTestSystemShare(t, env, sessionID, systemToken, nil)

	req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions/"+sessionID, nil, viewer.ID)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sessionID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler := HandleGetSession(env.DB)
	handler(w, req)

	testutil.AssertStatus(t, w, http.StatusOK)

	// Access granted via system share (more specific than public)
	var session db.SessionDetail
	testutil.ParseJSONResponse(t, w, &session)
	if session.ID != sessionID {
		t.Errorf("expected session ID %s, got %s", sessionID, session.ID)
	}
}

// TestHandleGetSession_AllSharesExpired tests no access when all shares are expired
func TestHandleGetSession_AllSharesExpired(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	viewer := testutil.CreateTestUser(t, env, "viewer@example.com", "Viewer")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Create multiple shares
	publicToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, sessionID, publicToken, true, nil, nil)

	systemToken := testutil.GenerateShareToken()
	testutil.CreateTestSystemShare(t, env, sessionID, systemToken, nil)

	recipientToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, sessionID, recipientToken, false, nil, []string{"viewer@example.com"})

	// Expire ALL shares
	_, err := env.DB.Exec(env.Ctx,
		"UPDATE session_shares SET expires_at = NOW() - INTERVAL '1 hour' WHERE session_id = $1",
		sessionID)
	if err != nil {
		t.Fatalf("failed to expire shares: %v", err)
	}

	req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions/"+sessionID, nil, viewer.ID)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sessionID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler := HandleGetSession(env.DB)
	handler(w, req)

	// All shares expired = no access = 404
	testutil.AssertStatus(t, w, http.StatusNotFound)
}

// TestHandleGetSession_RecipientHostnameUsernameHidden tests recipient can't see hostname/username
func TestHandleGetSession_RecipientHostnameUsernameHidden(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	recipient := testutil.CreateTestUser(t, env, "recipient@example.com", "Recipient")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Add hostname/username
	_, err := env.DB.Exec(env.Ctx,
		"UPDATE sessions SET hostname = 'secret-host', username = 'secret-user' WHERE id = $1",
		sessionID)
	if err != nil {
		t.Fatalf("failed to update session: %v", err)
	}

	// Create private share
	shareToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, sessionID, shareToken, false, nil, []string{"recipient@example.com"})

	req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions/"+sessionID, nil, recipient.ID)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sessionID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler := HandleGetSession(env.DB)
	handler(w, req)

	testutil.AssertStatus(t, w, http.StatusOK)

	var session db.SessionDetail
	testutil.ParseJSONResponse(t, w, &session)

	// Recipient should NOT see hostname/username
	if session.Hostname != nil {
		t.Errorf("PRIVACY VIOLATION: hostname exposed to recipient: %s", *session.Hostname)
	}
	if session.Username != nil {
		t.Errorf("PRIVACY VIOLATION: username exposed to recipient: %s", *session.Username)
	}
}

// TestHandleGetSession_SystemShareHostnameUsernameHidden tests system share can't see hostname/username
func TestHandleGetSession_SystemShareHostnameUsernameHidden(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	viewer := testutil.CreateTestUser(t, env, "viewer@example.com", "Viewer")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Add hostname/username
	_, err := env.DB.Exec(env.Ctx,
		"UPDATE sessions SET hostname = 'secret-host', username = 'secret-user' WHERE id = $1",
		sessionID)
	if err != nil {
		t.Fatalf("failed to update session: %v", err)
	}

	// Create system share
	shareToken := testutil.GenerateShareToken()
	testutil.CreateTestSystemShare(t, env, sessionID, shareToken, nil)

	req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions/"+sessionID, nil, viewer.ID)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sessionID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler := HandleGetSession(env.DB)
	handler(w, req)

	testutil.AssertStatus(t, w, http.StatusOK)

	var session db.SessionDetail
	testutil.ParseJSONResponse(t, w, &session)

	// System share user should NOT see hostname/username
	if session.Hostname != nil {
		t.Errorf("PRIVACY VIOLATION: hostname exposed via system share: %s", *session.Hostname)
	}
	if session.Username != nil {
		t.Errorf("PRIVACY VIOLATION: username exposed via system share: %s", *session.Username)
	}
}

// TestHandleGetSession_InactiveOwnerBlocksOwnerAccess tests that even owner can't access if deactivated
func TestHandleGetSession_InactiveOwnerBlocksOwnerAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Deactivate owner
	err := env.DB.UpdateUserStatus(context.Background(), owner.ID, "inactive")
	if err != nil {
		t.Fatalf("failed to deactivate owner: %v", err)
	}

	// Owner tries to access their own session
	req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions/"+sessionID, nil, owner.ID)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sessionID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler := HandleGetSession(env.DB)
	handler(w, req)

	// Deactivated owner = forbidden
	testutil.AssertStatus(t, w, http.StatusForbidden)
}

// TestHandleGetSession_InactiveOwnerBlocksAllAccess tests that inactive owner blocks all access types
func TestHandleGetSession_InactiveOwnerBlocksAllAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	recipient := testutil.CreateTestUser(t, env, "recipient@example.com", "Recipient")
	viewer := testutil.CreateTestUser(t, env, "viewer@example.com", "Viewer")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Create all share types
	publicToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, sessionID, publicToken, true, nil, nil)

	systemToken := testutil.GenerateShareToken()
	testutil.CreateTestSystemShare(t, env, sessionID, systemToken, nil)

	recipientToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, sessionID, recipientToken, false, nil, []string{"recipient@example.com"})

	// Deactivate owner
	err := env.DB.UpdateUserStatus(context.Background(), owner.ID, "inactive")
	if err != nil {
		t.Fatalf("failed to deactivate owner: %v", err)
	}

	testCases := []struct {
		name   string
		userID *int64
	}{
		{"Public (unauthenticated)", nil},
		{"System share (authenticated viewer)", &viewer.ID},
		{"Recipient share", &recipient.ID},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var req *http.Request
			if tc.userID != nil {
				req = testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions/"+sessionID, nil, *tc.userID)
			} else {
				req = httptest.NewRequest("GET", "/api/v1/sessions/"+sessionID, nil)
			}

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", sessionID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			w := httptest.NewRecorder()
			handler := HandleGetSession(env.DB)
			handler(w, req)

			// All access types blocked when owner is inactive
			testutil.AssertStatus(t, w, http.StatusForbidden)
		})
	}
}
