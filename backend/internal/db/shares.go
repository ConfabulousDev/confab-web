package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// CreateShare creates a new share link for a session (by UUID primary key)
// isPublic: true for public shares (anyone with link), false for recipient-only shares
// recipientEmails: email addresses to grant access (ignored if isPublic)
func (db *DB) CreateShare(ctx context.Context, sessionID string, userID int64, isPublic bool, expiresAt *time.Time, recipientEmails []string) (*SessionShare, error) {
	ctx, span := tracer.Start(ctx, "db.create_share",
		trace.WithAttributes(
			attribute.String("session.id", sessionID),
			attribute.Int64("user.id", userID),
			attribute.Bool("share.is_public", isPublic),
		))
	defer span.End()

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Verify session exists for this user and get external_id for display
	var externalID string
	err = tx.QueryRowContext(ctx,
		`SELECT external_id FROM sessions WHERE id = $1 AND user_id = $2`,
		sessionID, userID).Scan(&externalID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrSessionNotFound
		}
		if isInvalidUUIDError(err) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to verify session: %w", err)
	}

	// Insert share
	query := `INSERT INTO session_shares (session_id, expires_at)
	          VALUES ($1, $2)
	          RETURNING id, created_at`

	var share SessionShare
	share.SessionID = sessionID
	share.ExternalID = externalID
	share.IsPublic = isPublic
	share.ExpiresAt = expiresAt

	err = tx.QueryRowContext(ctx, query, sessionID, expiresAt).Scan(&share.ID, &share.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create share: %w", err)
	}

	// For public shares, insert into session_share_public
	if isPublic {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO session_share_public (share_id) VALUES ($1)`,
			share.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to create public share: %w", err)
		}
	}

	// For recipient shares, batch insert recipients with user_id lookup
	if !isPublic && len(recipientEmails) > 0 {
		// Batch lookup: get all existing user_ids for the recipient emails
		emailToUserID := make(map[string]int64)
		placeholders := make([]string, len(recipientEmails))
		args := make([]interface{}, len(recipientEmails))
		for i, email := range recipientEmails {
			placeholders[i] = fmt.Sprintf("LOWER($%d)", i+1)
			args[i] = email
		}

		lookupQuery := fmt.Sprintf(
			`SELECT id, LOWER(email) FROM users WHERE LOWER(email) IN (%s)`,
			strings.Join(placeholders, ", "))
		rows, err := tx.QueryContext(ctx, lookupQuery, args...)
		if err != nil {
			return nil, fmt.Errorf("failed to lookup recipient users: %w", err)
		}
		for rows.Next() {
			var uid int64
			var email string
			if err := rows.Scan(&uid, &email); err != nil {
				rows.Close()
				return nil, fmt.Errorf("failed to scan recipient user: %w", err)
			}
			emailToUserID[email] = uid
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("error iterating recipient users: %w", err)
		}

		// Batch insert: build multi-row INSERT for all recipients
		insertPlaceholders := make([]string, len(recipientEmails))
		insertArgs := make([]interface{}, 0, len(recipientEmails)*3)
		for i, email := range recipientEmails {
			var recipientUserID *int64
			if uid, ok := emailToUserID[strings.ToLower(email)]; ok {
				recipientUserID = &uid
			}
			base := i*3 + 1
			insertPlaceholders[i] = fmt.Sprintf("($%d, $%d, $%d)", base, base+1, base+2)
			insertArgs = append(insertArgs, share.ID, email, recipientUserID)
		}

		insertQuery := fmt.Sprintf(
			`INSERT INTO session_share_recipients (share_id, email, user_id) VALUES %s`,
			strings.Join(insertPlaceholders, ", "))
		_, err = tx.ExecContext(ctx, insertQuery, insertArgs...)
		if err != nil {
			return nil, fmt.Errorf("failed to create recipients: %w", err)
		}

		share.Recipients = recipientEmails
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}

	return &share, nil
}

// CreateSystemShare creates a system-wide share for a session (admin only, no ownership check)
// System shares are accessible to any authenticated user
func (db *DB) CreateSystemShare(ctx context.Context, sessionID string, expiresAt *time.Time) (*SessionShare, error) {
	ctx, span := tracer.Start(ctx, "db.create_system_share",
		trace.WithAttributes(attribute.String("session.id", sessionID)))
	defer span.End()

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get session external_id (no ownership check - admin operation)
	var externalID string
	err = tx.QueryRowContext(ctx,
		`SELECT external_id FROM sessions WHERE id = $1`,
		sessionID).Scan(&externalID)
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

	// Insert share
	var share SessionShare
	share.SessionID = sessionID
	share.ExternalID = externalID
	share.IsPublic = false // System shares are not public (require auth)
	share.ExpiresAt = expiresAt

	err = tx.QueryRowContext(ctx,
		`INSERT INTO session_shares (session_id, expires_at)
		 VALUES ($1, $2)
		 RETURNING id, created_at`,
		sessionID, expiresAt).Scan(&share.ID, &share.CreatedAt)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to create share: %w", err)
	}

	// Insert into session_share_system
	_, err = tx.ExecContext(ctx,
		`INSERT INTO session_share_system (share_id) VALUES ($1)`,
		share.ID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to create system share: %w", err)
	}

	if err = tx.Commit(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to commit: %w", err)
	}

	span.SetAttributes(attribute.Int64("share.id", share.ID))
	return &share, nil
}

// ListShares returns all shares for a session (by UUID primary key)
func (db *DB) ListShares(ctx context.Context, sessionID string, userID int64) ([]SessionShare, error) {
	ctx, span := tracer.Start(ctx, "db.list_shares",
		trace.WithAttributes(
			attribute.String("session.id", sessionID),
			attribute.Int64("user.id", userID),
		))
	defer span.End()

	// Verify session exists for this user and get external_id for display
	var externalID string
	err := db.conn.QueryRowContext(ctx,
		`SELECT external_id FROM sessions WHERE id = $1 AND user_id = $2`,
		sessionID, userID).Scan(&externalID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrSessionNotFound
		}
		if isInvalidUUIDError(err) {
			return nil, ErrSessionNotFound
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to verify session: %w", err)
	}

	// Get shares with public status
	query := `SELECT ss.id, ss.session_id, ss.expires_at, ss.created_at, ss.last_accessed_at,
	                 (ssp.share_id IS NOT NULL) as is_public
	          FROM session_shares ss
	          LEFT JOIN session_share_public ssp ON ss.id = ssp.share_id
	          WHERE ss.session_id = $1
	          ORDER BY ss.created_at DESC`

	rows, err := db.conn.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to list shares: %w", err)
	}
	defer rows.Close()

	shares := make([]SessionShare, 0)
	for rows.Next() {
		var share SessionShare
		err := rows.Scan(&share.ID, &share.SessionID,
			&share.ExpiresAt, &share.CreatedAt, &share.LastAccessedAt, &share.IsPublic)
		if err != nil {
			return nil, fmt.Errorf("failed to scan share: %w", err)
		}
		share.ExternalID = externalID // Set from parent query

		// Get recipients for non-public shares
		if !share.IsPublic {
			emails, err := db.loadShareRecipients(ctx, share.ID)
			if err != nil {
				return nil, err
			}
			share.Recipients = emails
		}

		shares = append(shares, share)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating shares: %w", err)
	}

	return shares, nil
}

// ListAllUserShares returns all shares for a user across all sessions
func (db *DB) ListAllUserShares(ctx context.Context, userID int64) ([]ShareWithSessionInfo, error) {
	ctx, span := tracer.Start(ctx, "db.list_all_user_shares",
		trace.WithAttributes(attribute.Int64("user.id", userID)))
	defer span.End()

	// Get all shares for the user with session info and public status
	query := `
		SELECT
			ss.id, ss.session_id, s.external_id,
			(ssp.share_id IS NOT NULL) as is_public,
			ss.expires_at, ss.created_at, ss.last_accessed_at,
			s.summary, s.first_user_message
		FROM session_shares ss
		JOIN sessions s ON ss.session_id = s.id
		LEFT JOIN session_share_public ssp ON ss.id = ssp.share_id
		WHERE s.user_id = $1
		ORDER BY ss.created_at DESC
	`

	rows, err := db.conn.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list shares: %w", err)
	}
	defer rows.Close()

	shares := make([]ShareWithSessionInfo, 0)
	for rows.Next() {
		var share ShareWithSessionInfo
		err := rows.Scan(
			&share.ID, &share.SessionID, &share.ExternalID,
			&share.IsPublic, &share.ExpiresAt, &share.CreatedAt, &share.LastAccessedAt,
			&share.SessionSummary, &share.SessionFirstUserMessage,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan share: %w", err)
		}

		// Get recipients for non-public shares
		if !share.IsPublic {
			emails, err := db.loadShareRecipients(ctx, share.ID)
			if err != nil {
				return nil, err
			}
			share.Recipients = emails
		}

		shares = append(shares, share)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating shares: %w", err)
	}

	return shares, nil
}

// RevokeShare deletes a share by ID
func (db *DB) RevokeShare(ctx context.Context, shareID int64, userID int64) error {
	ctx, span := tracer.Start(ctx, "db.revoke_share",
		trace.WithAttributes(
			attribute.Int64("share.id", shareID),
			attribute.Int64("user.id", userID),
		))
	defer span.End()

	// Verify ownership via session and delete
	result, err := db.conn.ExecContext(ctx,
		`DELETE FROM session_shares ss
		 USING sessions s
		 WHERE ss.session_id = s.id AND ss.id = $1 AND s.user_id = $2`,
		shareID, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to revoke share: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		// Could be either not found or unauthorized - keeping combined error for security
		return ErrUnauthorized
	}

	return nil
}

// loadShareRecipients loads the recipient emails for a share
func (db *DB) loadShareRecipients(ctx context.Context, shareID int64) ([]string, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT email FROM session_share_recipients WHERE share_id = $1 ORDER BY email`,
		shareID)
	if err != nil {
		return nil, fmt.Errorf("failed to get recipients: %w", err)
	}
	defer rows.Close()

	var emails []string
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return nil, fmt.Errorf("failed to scan email: %w", err)
		}
		emails = append(emails, email)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating emails: %w", err)
	}

	return emails, nil
}
