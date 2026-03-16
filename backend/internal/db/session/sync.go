package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/ConfabulousDev/confab-web/internal/db"
)

// FindOrCreateSyncSession finds an existing session by external_id or creates a new one
func (s *Store) FindOrCreateSyncSession(ctx context.Context, userID int64, params db.SyncSessionParams) (sessionID string, files map[string]db.SyncFileState, err error) {
	ctx, span := tracer.Start(ctx, "db.find_or_create_sync_session",
		trace.WithAttributes(
			attribute.Int64("user.id", userID),
			attribute.String("session.external_id", params.ExternalID),
		))
	defer span.End()

	selectQuery := `SELECT id FROM sessions WHERE user_id = $1 AND external_id = $2 AND session_type = 'Claude Code'`

	err = s.conn().QueryRowContext(ctx, selectQuery, userID, params.ExternalID).Scan(&sessionID)
	if err == nil {
		span.SetAttributes(attribute.Bool("session.created", false))
		if err := s.updateSessionMetadata(ctx, sessionID, params); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return "", nil, fmt.Errorf("failed to update session metadata: %w", err)
		}
		s.upsertFilterLookups(ctx, params.GitInfo)
		sid, files, err := s.getSyncFilesForSession(ctx, sessionID)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return sid, files, err
	}
	if err != sql.ErrNoRows {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", nil, fmt.Errorf("failed to find session: %w", err)
	}

	sessionID = uuid.New().String()
	insertQuery := `
		INSERT INTO sessions (id, user_id, external_id, first_seen, session_type, cwd, transcript_path, git_info, hostname, username, last_sync_at)
		VALUES ($1, $2, $3, NOW(), 'Claude Code', $4, $5, $6, NULLIF($7, ''), NULLIF($8, ''), NOW())
	`
	_, err = s.conn().ExecContext(ctx, insertQuery, sessionID, userID, params.ExternalID, params.CWD, params.TranscriptPath, params.GitInfo, params.Hostname, params.Username)
	if err == nil {
		span.SetAttributes(attribute.Bool("session.created", true))
		s.upsertFilterLookups(ctx, params.GitInfo)
		return sessionID, make(map[string]db.SyncFileState), nil
	}

	if db.IsUniqueViolation(err) {
		span.SetAttributes(attribute.Bool("session.race_condition", true))
		err = s.conn().QueryRowContext(ctx, selectQuery, userID, params.ExternalID).Scan(&sessionID)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return "", nil, fmt.Errorf("failed to find session after conflict: %w", err)
		}
		if err := s.updateSessionMetadata(ctx, sessionID, params); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return "", nil, fmt.Errorf("failed to update session metadata: %w", err)
		}
		sid, files, err := s.getSyncFilesForSession(ctx, sessionID)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return sid, files, err
	}

	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	return "", nil, fmt.Errorf("failed to create session: %w", err)
}

func (s *Store) updateSessionMetadata(ctx context.Context, sessionID string, params db.SyncSessionParams) error {
	query := `
		UPDATE sessions
		SET cwd = COALESCE($2, cwd),
		    transcript_path = COALESCE($3, transcript_path),
		    git_info = COALESCE($4, git_info),
		    hostname = COALESCE(NULLIF($5, ''), hostname),
		    username = COALESCE(NULLIF($6, ''), username),
		    last_sync_at = NOW()
		WHERE id = $1
	`
	_, err := s.conn().ExecContext(ctx, query, sessionID, params.CWD, params.TranscriptPath, params.GitInfo, params.Hostname, params.Username)
	return err
}

func (s *Store) getSyncFilesForSession(ctx context.Context, sessionID string) (string, map[string]db.SyncFileState, error) {
	files := make(map[string]db.SyncFileState)
	filesQuery := `SELECT file_name, file_type, last_synced_line FROM sync_files WHERE session_id = $1`
	rows, err := s.conn().QueryContext(ctx, filesQuery, sessionID)
	if err != nil {
		return "", nil, fmt.Errorf("failed to query sync files: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var state db.SyncFileState
		if err := rows.Scan(&state.FileName, &state.FileType, &state.LastSyncedLine); err != nil {
			return "", nil, fmt.Errorf("failed to scan sync file: %w", err)
		}
		files[state.FileName] = state
	}

	if err := rows.Err(); err != nil {
		return "", nil, fmt.Errorf("error iterating sync files: %w", err)
	}

	return sessionID, files, nil
}

// UpdateSyncFileState updates the high-water mark for a file's sync state
func (s *Store) UpdateSyncFileState(ctx context.Context, sessionID, fileName, fileType string, lastSyncedLine int, lastMessageAt *time.Time, summary, firstUserMessage *string, gitInfo json.RawMessage) error {
	ctx, span := tracer.Start(ctx, "db.update_sync_file_state",
		trace.WithAttributes(
			attribute.String("session.id", sessionID),
			attribute.String("file.name", fileName),
			attribute.String("file.type", fileType),
			attribute.Int("sync.last_line", lastSyncedLine),
		))
	defer span.End()

	tx, err := s.conn().BeginTx(ctx, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	syncQuery := `
		INSERT INTO sync_files (session_id, file_name, file_type, last_synced_line, chunk_count, updated_at)
		VALUES ($1, $2, $3, $4, 1, NOW())
		ON CONFLICT (session_id, file_name) DO UPDATE SET
			last_synced_line = $4,
			chunk_count = COALESCE(sync_files.chunk_count, 0) + 1,
			updated_at = NOW()
	`
	_, err = tx.ExecContext(ctx, syncQuery, sessionID, fileName, fileType, lastSyncedLine)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to update sync file state: %w", err)
	}

	sessionQuery := `UPDATE sessions SET last_sync_at = NOW()`
	args := []interface{}{sessionID}
	argIdx := 2

	if lastMessageAt != nil {
		sessionQuery += fmt.Sprintf(", last_message_at = CASE WHEN last_message_at IS NULL OR last_message_at < $%d THEN $%d ELSE last_message_at END", argIdx, argIdx)
		args = append(args, lastMessageAt)
		argIdx++
	}
	if summary != nil {
		sessionQuery += fmt.Sprintf(", summary = $%d", argIdx)
		args = append(args, *summary)
		argIdx++
	}
	if firstUserMessage != nil {
		sessionQuery += fmt.Sprintf(", first_user_message = COALESCE(first_user_message, $%d)", argIdx)
		args = append(args, *firstUserMessage)
		argIdx++
	}
	if len(gitInfo) > 0 {
		sessionQuery += fmt.Sprintf(", git_info = $%d", argIdx)
		args = append(args, gitInfo)
	}
	sessionQuery += " WHERE id = $1"
	_, err = tx.ExecContext(ctx, sessionQuery, args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to update session metadata: %w", err)
	}

	if err = tx.Commit(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to commit: %w", err)
	}

	if len(gitInfo) > 0 {
		s.upsertFilterLookups(ctx, gitInfo)
	}

	return nil
}

// GetSyncFileState retrieves the sync state for a specific file
func (s *Store) GetSyncFileState(ctx context.Context, sessionID, fileName string) (*db.SyncFileState, error) {
	ctx, span := tracer.Start(ctx, "db.get_sync_file_state",
		trace.WithAttributes(
			attribute.String("session.id", sessionID),
			attribute.String("file.name", fileName),
		))
	defer span.End()

	query := `SELECT file_name, file_type, last_synced_line, chunk_count FROM sync_files WHERE session_id = $1 AND file_name = $2`
	var state db.SyncFileState
	err := s.conn().QueryRowContext(ctx, query, sessionID, fileName).Scan(&state.FileName, &state.FileType, &state.LastSyncedLine, &state.ChunkCount)
	if err == sql.ErrNoRows {
		return nil, db.ErrFileNotFound
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to get sync file state: %w", err)
	}
	span.SetAttributes(attribute.Int("sync.last_line", state.LastSyncedLine))
	return &state, nil
}

// UpdateSyncFileChunkCount sets the chunk_count for a file (used for self-healing on read)
func (s *Store) UpdateSyncFileChunkCount(ctx context.Context, sessionID, fileName string, chunkCount int) error {
	ctx, span := tracer.Start(ctx, "db.update_sync_file_chunk_count",
		trace.WithAttributes(
			attribute.String("session.id", sessionID),
			attribute.String("file.name", fileName),
			attribute.Int("chunk.count", chunkCount),
		))
	defer span.End()

	query := `UPDATE sync_files SET chunk_count = $3, updated_at = NOW() WHERE session_id = $1 AND file_name = $2`
	_, err := s.conn().ExecContext(ctx, query, sessionID, fileName, chunkCount)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to update chunk count: %w", err)
	}
	return nil
}

func (s *Store) upsertFilterLookups(ctx context.Context, gitInfo json.RawMessage) {
	if len(gitInfo) == 0 {
		return
	}
	var info struct {
		RepoURL string `json:"repo_url"`
		Branch  string `json:"branch"`
	}
	if err := json.Unmarshal(gitInfo, &info); err != nil {
		return
	}
	if info.RepoURL != "" {
		repo := db.ExtractRepoName(info.RepoURL)
		if repo != nil && *repo != "" {
			s.conn().ExecContext(ctx, "INSERT INTO session_repos (repo_name) VALUES ($1) ON CONFLICT DO NOTHING", *repo)
		}
	}
	if info.Branch != "" {
		s.conn().ExecContext(ctx, "INSERT INTO session_branches (branch_name) VALUES ($1) ON CONFLICT DO NOTHING", info.Branch)
	}
}
