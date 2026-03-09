// ABOUTME: Database CRUD operations for the learnings table.
// ABOUTME: Provides Create, List, Get, Update, Delete, and CountByStatus for learning artifacts.
package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/ConfabulousDev/confab-web/internal/models"
)

// ErrLearningNotFound is returned when a learning is not found or not owned by the user.
var ErrLearningNotFound = fmt.Errorf("learning not found")

// LearningFilters contains optional filters for listing learnings.
type LearningFilters struct {
	Status *models.LearningStatus // Filter by status enum
	Source *models.LearningSource // Filter by source enum
	Tags   []string               // Overlap filter: any of these tags
	Query  *string                // Full-text search against search_vector
	Limit  int                    // Page size (default 50)
	Offset int                    // Pagination offset
}

// CreateLearning inserts a new learning and returns the full row.
func (db *DB) CreateLearning(ctx context.Context, userID int64, req *models.CreateLearningRequest) (*models.Learning, error) {
	ctx, span := tracer.Start(ctx, "db.create_learning",
		trace.WithAttributes(attribute.Int64("user.id", userID)))
	defer span.End()

	// Marshal transcript_range to JSONB (nullable)
	var transcriptRangeJSON []byte
	if req.TranscriptRange != nil {
		var err error
		transcriptRangeJSON, err = json.Marshal(req.TranscriptRange)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to marshal transcript_range")
			return nil, fmt.Errorf("failed to marshal transcript_range: %w", err)
		}
	}

	// Normalize nil slices to empty — PostgreSQL NOT NULL columns reject NULL arrays.
	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}
	sessionIDs := req.SessionIDs
	if sessionIDs == nil {
		sessionIDs = []string{}
	}

	query := `
		INSERT INTO learnings (user_id, title, body, tags, source, session_ids, transcript_range)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, user_id, title, body, tags, status, source, session_ids,
		          transcript_range, confluence_page_id, exported_at, created_at, updated_at
	`

	var l models.Learning
	var tagsArr pq.StringArray
	var sessionIDsArr pq.StringArray
	var transcriptRangeRaw []byte

	err := db.conn.QueryRowContext(ctx, query,
		userID,
		req.Title,
		req.Body,
		pq.Array(tags),
		string(req.Source),
		pq.Array(sessionIDs),
		transcriptRangeJSON,
	).Scan(
		&l.ID, &l.UserID, &l.Title, &l.Body,
		&tagsArr, &l.Status, &l.Source, &sessionIDsArr,
		&transcriptRangeRaw, &l.ConfluencePageID, &l.ExportedAt,
		&l.CreatedAt, &l.UpdatedAt,
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create learning")
		return nil, fmt.Errorf("failed to create learning: %w", err)
	}

	l.Tags = []string(tagsArr)
	l.SessionIDs = []string(sessionIDsArr)
	if transcriptRangeRaw != nil {
		var tr interface{}
		if err := json.Unmarshal(transcriptRangeRaw, &tr); err != nil {
			// Log but don't fail — transcript_range is supplementary data
			span.RecordError(err)
		} else {
			l.TranscriptRange = tr
		}
	}

	span.SetAttributes(attribute.String("learning.id", l.ID))
	return &l, nil
}

// ListLearnings returns learnings for a user with optional filters.
func (db *DB) ListLearnings(ctx context.Context, userID int64, filters *LearningFilters) ([]models.Learning, error) {
	ctx, span := tracer.Start(ctx, "db.list_learnings",
		trace.WithAttributes(attribute.Int64("user.id", userID)))
	defer span.End()

	pb := newParamBuilder(userID)
	var whereClauses []string
	whereClauses = append(whereClauses, "l.user_id = $1")

	if filters != nil {
		if filters.Status != nil {
			p := pb.add(string(*filters.Status))
			whereClauses = append(whereClauses, fmt.Sprintf("l.status = %s", p))
		}
		if filters.Source != nil {
			p := pb.add(string(*filters.Source))
			whereClauses = append(whereClauses, fmt.Sprintf("l.source = %s", p))
		}
		if len(filters.Tags) > 0 {
			p := pb.addArray(filters.Tags)
			whereClauses = append(whereClauses, fmt.Sprintf("l.tags && %s", p))
		}
		if filters.Query != nil && *filters.Query != "" {
			p := pb.add(*filters.Query)
			whereClauses = append(whereClauses, fmt.Sprintf("l.search_vector @@ plainto_tsquery('english', %s)", p))
		}
	}

	limit := 50
	offset := 0
	if filters != nil {
		if filters.Limit > 0 {
			limit = filters.Limit
		}
		if filters.Offset > 0 {
			offset = filters.Offset
		}
	}

	query := fmt.Sprintf(`
		SELECT l.id, l.user_id, l.title, l.body, l.tags, l.status, l.source,
		       l.session_ids, l.transcript_range, l.confluence_page_id,
		       l.exported_at, l.created_at, l.updated_at,
		       u.email AS owner_email
		FROM learnings l
		JOIN users u ON l.user_id = u.id
		WHERE %s
		ORDER BY l.created_at DESC
		LIMIT %d OFFSET %d
	`, strings.Join(whereClauses, " AND "), limit, offset)

	rows, err := db.conn.QueryContext(ctx, query, pb.args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to list learnings")
		return nil, fmt.Errorf("failed to list learnings: %w", err)
	}
	defer rows.Close()

	learnings := make([]models.Learning, 0)
	for rows.Next() {
		var l models.Learning
		var tagsArr pq.StringArray
		var sessionIDsArr pq.StringArray
		var transcriptRangeRaw []byte

		err := rows.Scan(
			&l.ID, &l.UserID, &l.Title, &l.Body,
			&tagsArr, &l.Status, &l.Source, &sessionIDsArr,
			&transcriptRangeRaw, &l.ConfluencePageID, &l.ExportedAt,
			&l.CreatedAt, &l.UpdatedAt, &l.OwnerEmail,
		)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to scan learning")
			return nil, fmt.Errorf("failed to scan learning: %w", err)
		}

		l.Tags = []string(tagsArr)
		l.SessionIDs = []string(sessionIDsArr)
		if transcriptRangeRaw != nil {
			var tr interface{}
			if err := json.Unmarshal(transcriptRangeRaw, &tr); err == nil {
				l.TranscriptRange = tr
			}
		}

		learnings = append(learnings, l)
	}

	if err := rows.Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "error iterating learnings")
		return nil, fmt.Errorf("error iterating learnings: %w", err)
	}

	return learnings, nil
}

// GetLearning returns a single learning by ID, scoped to the given user.
func (db *DB) GetLearning(ctx context.Context, learningID string, userID int64) (*models.Learning, error) {
	ctx, span := tracer.Start(ctx, "db.get_learning",
		trace.WithAttributes(
			attribute.String("learning.id", learningID),
			attribute.Int64("user.id", userID),
		))
	defer span.End()

	query := `
		SELECT l.id, l.user_id, l.title, l.body, l.tags, l.status, l.source,
		       l.session_ids, l.transcript_range, l.confluence_page_id,
		       l.exported_at, l.created_at, l.updated_at,
		       u.email AS owner_email
		FROM learnings l
		JOIN users u ON l.user_id = u.id
		WHERE l.id = $1 AND l.user_id = $2
	`

	var l models.Learning
	var tagsArr pq.StringArray
	var sessionIDsArr pq.StringArray
	var transcriptRangeRaw []byte

	err := db.conn.QueryRowContext(ctx, query, learningID, userID).Scan(
		&l.ID, &l.UserID, &l.Title, &l.Body,
		&tagsArr, &l.Status, &l.Source, &sessionIDsArr,
		&transcriptRangeRaw, &l.ConfluencePageID, &l.ExportedAt,
		&l.CreatedAt, &l.UpdatedAt, &l.OwnerEmail,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrLearningNotFound
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get learning")
		return nil, fmt.Errorf("failed to get learning: %w", err)
	}

	l.Tags = []string(tagsArr)
	l.SessionIDs = []string(sessionIDsArr)
	if transcriptRangeRaw != nil {
		var tr interface{}
		if err := json.Unmarshal(transcriptRangeRaw, &tr); err != nil {
			// Log but don't fail — transcript_range is supplementary data
			span.RecordError(err)
		} else {
			l.TranscriptRange = tr
		}
	}

	return &l, nil
}

// UpdateLearning dynamically updates a learning's fields and returns the updated row.
// NOTE: OwnerEmail is NOT populated in the returned Learning because the UPDATE
// does not join the users table. Callers needing OwnerEmail should call GetLearning afterward.
func (db *DB) UpdateLearning(ctx context.Context, learningID string, userID int64, req *models.UpdateLearningRequest) (*models.Learning, error) {
	ctx, span := tracer.Start(ctx, "db.update_learning",
		trace.WithAttributes(
			attribute.String("learning.id", learningID),
			attribute.Int64("user.id", userID),
		))
	defer span.End()

	// Build SET clauses dynamically
	setClauses := []string{"updated_at = NOW()"}
	args := []interface{}{learningID, userID}
	nextIdx := 3

	if req.Title != nil {
		setClauses = append(setClauses, fmt.Sprintf("title = $%d", nextIdx))
		args = append(args, *req.Title)
		nextIdx++
	}
	if req.Body != nil {
		setClauses = append(setClauses, fmt.Sprintf("body = $%d", nextIdx))
		args = append(args, *req.Body)
		nextIdx++
	}
	if req.Tags != nil {
		setClauses = append(setClauses, fmt.Sprintf("tags = $%d", nextIdx))
		args = append(args, pq.Array(req.Tags))
		nextIdx++
	}
	if req.Status != nil {
		setClauses = append(setClauses, fmt.Sprintf("status = $%d", nextIdx))
		args = append(args, string(*req.Status))
		nextIdx++

		// If status is "exported", set exported_at
		if *req.Status == models.LearningStatusExported {
			now := time.Now()
			setClauses = append(setClauses, fmt.Sprintf("exported_at = $%d", nextIdx))
			args = append(args, now)
			nextIdx++
		}
	}
	if req.ConfluencePageID != nil {
		setClauses = append(setClauses, fmt.Sprintf("confluence_page_id = $%d", nextIdx))
		args = append(args, *req.ConfluencePageID)
		nextIdx++
	}

	query := fmt.Sprintf(`
		UPDATE learnings
		SET %s
		WHERE id = $1 AND user_id = $2
		RETURNING id, user_id, title, body, tags, status, source, session_ids,
		          transcript_range, confluence_page_id, exported_at, created_at, updated_at
	`, strings.Join(setClauses, ", "))

	var l models.Learning
	var tagsArr pq.StringArray
	var sessionIDsArr pq.StringArray
	var transcriptRangeRaw []byte

	err := db.conn.QueryRowContext(ctx, query, args...).Scan(
		&l.ID, &l.UserID, &l.Title, &l.Body,
		&tagsArr, &l.Status, &l.Source, &sessionIDsArr,
		&transcriptRangeRaw, &l.ConfluencePageID, &l.ExportedAt,
		&l.CreatedAt, &l.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrLearningNotFound
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update learning")
		return nil, fmt.Errorf("failed to update learning: %w", err)
	}

	l.Tags = []string(tagsArr)
	l.SessionIDs = []string(sessionIDsArr)
	if transcriptRangeRaw != nil {
		var tr interface{}
		if err := json.Unmarshal(transcriptRangeRaw, &tr); err != nil {
			// Log but don't fail — transcript_range is supplementary data
			span.RecordError(err)
		} else {
			l.TranscriptRange = tr
		}
	}

	return &l, nil
}

// DeleteLearning removes a learning by ID, scoped to the given user.
func (db *DB) DeleteLearning(ctx context.Context, learningID string, userID int64) error {
	ctx, span := tracer.Start(ctx, "db.delete_learning",
		trace.WithAttributes(
			attribute.String("learning.id", learningID),
			attribute.Int64("user.id", userID),
		))
	defer span.End()

	result, err := db.conn.ExecContext(ctx,
		`DELETE FROM learnings WHERE id = $1 AND user_id = $2`,
		learningID, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to delete learning")
		return fmt.Errorf("failed to delete learning: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrLearningNotFound
	}

	return nil
}

// CountLearningsByStatus returns a map of status -> count for the given user.
func (db *DB) CountLearningsByStatus(ctx context.Context, userID int64) (map[string]int, error) {
	ctx, span := tracer.Start(ctx, "db.count_learnings_by_status",
		trace.WithAttributes(attribute.Int64("user.id", userID)))
	defer span.End()

	query := `
		SELECT status::text, COUNT(*)
		FROM learnings
		WHERE user_id = $1
		GROUP BY status
	`

	rows, err := db.conn.QueryContext(ctx, query, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to count learnings by status")
		return nil, fmt.Errorf("failed to count learnings by status: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to scan count")
			return nil, fmt.Errorf("failed to scan learning status count: %w", err)
		}
		counts[status] = count
	}

	if err := rows.Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "error iterating counts")
		return nil, fmt.Errorf("error iterating learning status counts: %w", err)
	}

	return counts, nil
}

