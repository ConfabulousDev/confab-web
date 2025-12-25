package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/logger"
)

// CanonicalAccessResult contains the result of checking canonical session access.
// This is used by endpoints that support the unified access model (CF-132).
type CanonicalAccessResult struct {
	// ViewerUserID is the authenticated user's ID, or nil if unauthenticated
	ViewerUserID *int64
	// AccessInfo contains the access type and related metadata
	AccessInfo *db.SessionAccessInfo
	// Session is the session detail (only set if access was granted and session was fetched)
	Session *db.SessionDetail
}

// CheckCanonicalAccess checks session access using the unified canonical access model.
// This implements the common access control pattern used by session and sync endpoints.
//
// IMPORTANT: Routes using this function should apply auth.OptionalAuth middleware
// to set the user ID in context if authenticated.
//
// Access is determined by checking in order:
//  1. Owner - user owns the session (full access)
//  2. Recipient - user is named in a private share
//  3. System - any authenticated user via system share
//  4. Public - anyone via public share
//  5. None - no access
//
// Returns the access result. Callers should check:
//   - err != nil: Database error occurred
//   - result.AccessInfo.AccessType == db.SessionAccessNone: No access (check AuthMayHelp for 401 vs 404)
//   - result.Session != nil: Access granted, session details available
func CheckCanonicalAccess(
	ctx context.Context,
	database *db.DB,
	sessionID string,
) (*CanonicalAccessResult, error) {
	result := &CanonicalAccessResult{}

	// Step 1: Extract viewer identity from context (set by OptionalAuth middleware)
	if userID, ok := auth.GetUserID(ctx); ok {
		result.ViewerUserID = &userID
	}

	// Step 2: Determine access type based on ownership and shares
	accessInfo, err := database.GetSessionAccessType(ctx, sessionID, result.ViewerUserID)
	if err != nil {
		return nil, err
	}
	result.AccessInfo = accessInfo

	// Step 3: If no access, return early (caller decides 401 vs 404 based on AuthMayHelp)
	if accessInfo.AccessType == db.SessionAccessNone {
		return result, nil
	}

	// Step 4: Get session with privacy filtering based on access type
	session, err := database.GetSessionDetailWithAccess(ctx, sessionID, result.ViewerUserID, accessInfo)
	if err != nil {
		return nil, err
	}
	result.Session = session

	return result, nil
}

// RespondCanonicalAccessError writes the appropriate HTTP error response for access control failures.
// This handles the common error cases: not found, owner inactive, and internal errors.
// Returns true if an error response was written, false if no error occurred.
func RespondCanonicalAccessError(ctx context.Context, w http.ResponseWriter, err error, sessionID string) bool {
	if err == nil {
		return false
	}

	switch {
	case errors.Is(err, db.ErrSessionNotFound):
		respondError(w, http.StatusNotFound, "Session not found")
	case errors.Is(err, db.ErrOwnerInactive):
		respondError(w, http.StatusForbidden, "This session is no longer available")
	default:
		log := logger.Ctx(ctx)
		log.Error("Failed to check session access", "error", err, "session_id", sessionID)
		respondError(w, http.StatusInternalServerError, "Failed to get session")
	}
	return true
}
