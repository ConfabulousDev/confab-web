package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/ConfabulousDev/confab-web/internal/models"
)

// GetSessionAccessType determines how a user can access a session.
// Checks in order of specificity: owner, recipient, system, public.
// Returns the access type and the share ID (if applicable).
// viewerUserID can be nil for unauthenticated users.
func (db *DB) GetSessionAccessType(ctx context.Context, sessionID string, viewerUserID *int64) (*SessionAccessInfo, error) {
	ctx, span := tracer.Start(ctx, "db.get_session_access_type",
		trace.WithAttributes(attribute.String("session.id", sessionID)))
	defer span.End()

	if viewerUserID != nil {
		span.SetAttributes(attribute.Int64("user.id", *viewerUserID))
	}

	// First, check if session exists and get owner
	var ownerUserID int64
	err := db.conn.QueryRowContext(ctx,
		`SELECT user_id FROM sessions WHERE id = $1`, sessionID).Scan(&ownerUserID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrSessionNotFound
		}
		if isInvalidUUIDError(err) {
			return nil, ErrSessionNotFound
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Check if viewer is the owner (most specific)
	if viewerUserID != nil && *viewerUserID == ownerUserID {
		span.SetAttributes(attribute.String("access.type", "owner"))
		return &SessionAccessInfo{AccessType: SessionAccessOwner}, nil
	}

	// Combined query checks all share types in one round-trip.
	// Priority: recipient (1) > system (2) > public (3)
	// Also computes auth_may_help: true if unauthenticated and non-public shares exist
	var accessType string
	var shareID int64
	var authMayHelp bool

	err = db.conn.QueryRowContext(ctx, `
		SELECT
			CASE
				WHEN ssr.user_id IS NOT NULL THEN 'recipient'
				WHEN sss.share_id IS NOT NULL AND $2::bigint IS NOT NULL THEN 'system'
				WHEN ssp.share_id IS NOT NULL THEN 'public'
				ELSE 'none'
			END as access_type,
			ss.id as share_id,
			($2::bigint IS NULL AND ssp.share_id IS NULL) as auth_may_help
		FROM session_shares ss
		LEFT JOIN session_share_recipients ssr ON ss.id = ssr.share_id AND ssr.user_id = $2
		LEFT JOIN session_share_system sss ON ss.id = sss.share_id
		LEFT JOIN session_share_public ssp ON ss.id = ssp.share_id
		WHERE ss.session_id = $1
		  AND (ss.expires_at IS NULL OR ss.expires_at > NOW())
		ORDER BY
			CASE
				WHEN ssr.user_id IS NOT NULL THEN 1
				WHEN sss.share_id IS NOT NULL AND $2::bigint IS NOT NULL THEN 2
				WHEN ssp.share_id IS NOT NULL THEN 3
				ELSE 4
			END
		LIMIT 1
	`, sessionID, viewerUserID).Scan(&accessType, &shareID, &authMayHelp)

	if err == sql.ErrNoRows {
		// No shares exist for this session
		span.SetAttributes(attribute.String("access.type", "none"))
		return &SessionAccessInfo{AccessType: SessionAccessNone, AuthMayHelp: false}, nil
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to check share access: %w", err)
	}

	span.SetAttributes(attribute.String("access.type", accessType))

	switch accessType {
	case "recipient":
		return &SessionAccessInfo{AccessType: SessionAccessRecipient, ShareID: &shareID}, nil
	case "system":
		return &SessionAccessInfo{AccessType: SessionAccessSystem, ShareID: &shareID}, nil
	case "public":
		return &SessionAccessInfo{AccessType: SessionAccessPublic, ShareID: &shareID}, nil
	default:
		// "none" - has shares but viewer has no access
		return &SessionAccessInfo{AccessType: SessionAccessNone, AuthMayHelp: authMayHelp}, nil
	}
}

// GetSessionDetailWithAccess returns session details for any user with access.
// Unlike GetSessionDetail, this works for shared access (not just owners).
// Hostname and username are only returned for owners.
// Updates last_accessed_at on the share if accessed via share.
func (db *DB) GetSessionDetailWithAccess(ctx context.Context, sessionID string, viewerUserID *int64, accessInfo *SessionAccessInfo) (*SessionDetail, error) {
	ctx, span := tracer.Start(ctx, "db.get_session_detail_with_access",
		trace.WithAttributes(
			attribute.String("session.id", sessionID),
			attribute.String("access.type", string(accessInfo.AccessType)),
		))
	defer span.End()
	if viewerUserID != nil {
		span.SetAttributes(attribute.Int64("viewer.user_id", *viewerUserID))
	}

	// Check owner's status to block access if deactivated
	var session SessionDetail
	var gitInfoBytes []byte
	var ownerStatus models.UserStatus
	var hostname, username *string

	sessionQuery := `
		SELECT s.id, s.external_id, s.custom_title, s.summary, s.first_user_message, s.first_seen, s.cwd, s.transcript_path, s.git_info, s.last_sync_at, s.hostname, s.username, u.status
		FROM sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.id = $1
	`
	err := db.conn.QueryRowContext(ctx, sessionQuery, sessionID).Scan(
		&session.ID,
		&session.ExternalID,
		&session.CustomTitle,
		&session.Summary,
		&session.FirstUserMessage,
		&session.FirstSeen,
		&session.CWD,
		&session.TranscriptPath,
		&gitInfoBytes,
		&session.LastSyncAt,
		&hostname,
		&username,
		&ownerStatus,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrSessionNotFound
		}
		if isInvalidUUIDError(err) {
			return nil, ErrSessionNotFound
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Check if session owner is deactivated
	if ownerStatus == models.UserStatusInactive {
		return nil, ErrOwnerInactive
	}

	// Only include hostname/username for owners
	if accessInfo.AccessType == SessionAccessOwner {
		session.Hostname = hostname
		session.Username = username
	}

	// Set IsOwner flag
	isOwner := accessInfo.AccessType == SessionAccessOwner
	session.IsOwner = &isOwner

	// Unmarshal git_info and load sync files
	if err := db.unmarshalSessionGitInfo(&session, gitInfoBytes); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	if err := db.loadSessionSyncFiles(ctx, &session); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	// Update last_accessed_at on the share if accessed via share
	// Non-critical analytics update; ignore errors to not fail the main operation
	if accessInfo.ShareID != nil {
		_, _ = db.conn.ExecContext(ctx,
			`UPDATE session_shares SET last_accessed_at = NOW() WHERE id = $1`,
			*accessInfo.ShareID)
	}

	return &session, nil
}

// unmarshalSessionGitInfoForAccess unmarshals git_info JSONB if present
// This is a package-local helper that mirrors the one in sessions.go
func unmarshalSessionGitInfoForAccess(session *SessionDetail, gitInfoBytes []byte) error {
	if len(gitInfoBytes) > 0 {
		if err := json.Unmarshal(gitInfoBytes, &session.GitInfo); err != nil {
			return fmt.Errorf("failed to unmarshal git_info: %w", err)
		}
	}
	return nil
}
