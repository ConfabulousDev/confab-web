package analytics

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/anthropic"
	"github.com/ConfabulousDev/confab-web/internal/storage"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// StaleSession represents a session that needs analytics precomputation.
type StaleSession struct {
	SessionID  string
	UserID     int64
	ExternalID string
	TotalLines int64
}

// PrecomputeConfig holds configuration for the precomputer.
type PrecomputeConfig struct {
	SmartRecapEnabled  bool
	AnthropicAPIKey    string
	SmartRecapModel    string
	SmartRecapQuota    int
	StalenessMinutes   int
	LockTimeoutSeconds int
}

// Precomputer handles background analytics precomputation.
type Precomputer struct {
	db             *sql.DB
	store          *storage.S3Storage
	analyticsStore *Store
	config         PrecomputeConfig
}

// NewPrecomputer creates a new Precomputer.
func NewPrecomputer(db *sql.DB, store *storage.S3Storage, analyticsStore *Store, config PrecomputeConfig) *Precomputer {
	return &Precomputer{
		db:             db,
		store:          store,
		analyticsStore: analyticsStore,
		config:         config,
	}
}

// FindStaleSessions returns sessions where any card is stale (outdated version or
// line count mismatch). Sessions are ordered by most recently synced first.
func (p *Precomputer) FindStaleSessions(ctx context.Context, limit int) ([]StaleSession, error) {
	ctx, span := tracer.Start(ctx, "precompute.find_stale_sessions",
		trace.WithAttributes(attribute.Int("limit", limit)))
	defer span.End()

	// Query finds sessions where ANY card is stale (missing, wrong version, or wrong line count).
	// This mirrors Cards.AllValid() in cards.go which checks all 7 cards independently.
	// A card is valid if: exists AND version matches AND up_to_line matches total lines.
	query := `
		WITH session_lines AS (
			SELECT session_id, SUM(last_synced_line) as total_lines
			FROM sync_files
			WHERE file_type IN ('transcript', 'agent')
			GROUP BY session_id
			HAVING SUM(last_synced_line) > 0
		)
		SELECT sl.session_id, s.user_id, s.external_id, sl.total_lines
		FROM session_lines sl
		JOIN sessions s ON sl.session_id = s.id
		LEFT JOIN session_card_tokens tc ON sl.session_id = tc.session_id
		LEFT JOIN session_card_session sc ON sl.session_id = sc.session_id
		LEFT JOIN session_card_tools tl ON sl.session_id = tl.session_id
		LEFT JOIN session_card_code_activity ca ON sl.session_id = ca.session_id
		LEFT JOIN session_card_conversation cv ON sl.session_id = cv.session_id
		LEFT JOIN session_card_agents_skills as_card ON sl.session_id = as_card.session_id
		LEFT JOIN session_card_redactions rd ON sl.session_id = rd.session_id
		WHERE s.session_type = 'Claude Code'
		  AND NOT (
			-- All cards must be valid (exist with correct version and line count)
			tc.session_id IS NOT NULL AND tc.version = $1 AND tc.up_to_line = sl.total_lines
			AND sc.session_id IS NOT NULL AND sc.version = $2 AND sc.up_to_line = sl.total_lines
			AND tl.session_id IS NOT NULL AND tl.version = $3 AND tl.up_to_line = sl.total_lines
			AND ca.session_id IS NOT NULL AND ca.version = $4 AND ca.up_to_line = sl.total_lines
			AND cv.session_id IS NOT NULL AND cv.version = $5 AND cv.up_to_line = sl.total_lines
			AND as_card.session_id IS NOT NULL AND as_card.version = $6 AND as_card.up_to_line = sl.total_lines
			AND rd.session_id IS NOT NULL AND rd.version = $7 AND rd.up_to_line = sl.total_lines
		  )
		ORDER BY s.last_sync_at DESC NULLS LAST
		LIMIT $8
	`

	rows, err := p.db.QueryContext(ctx, query,
		TokensCardVersion,
		SessionCardVersion,
		ToolsCardVersion,
		CodeActivityCardVersion,
		ConversationCardVersion,
		AgentsAndSkillsCardVersion,
		RedactionsCardVersion,
		limit,
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	defer rows.Close()

	var sessions []StaleSession
	for rows.Next() {
		var s StaleSession
		if err := rows.Scan(&s.SessionID, &s.UserID, &s.ExternalID, &s.TotalLines); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}
		sessions = append(sessions, s)
	}

	if err := rows.Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetAttributes(attribute.Int("sessions.found", len(sessions)))
	return sessions, nil
}

// PrecomputeSession computes all analytics cards for a session (including smart recap).
func (p *Precomputer) PrecomputeSession(ctx context.Context, session StaleSession) error {
	ctx, span := tracer.Start(ctx, "precompute.session",
		trace.WithAttributes(
			attribute.String("session.id", session.SessionID),
			attribute.Int64("session.user_id", session.UserID),
			attribute.Int64("session.total_lines", session.TotalLines),
		))
	defer span.End()

	// Download transcript and agent files
	fc, err := p.downloadTranscript(ctx, session)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	if fc == nil {
		// No transcript data - nothing to compute
		span.SetAttributes(attribute.Bool("session.empty", true))
		return nil
	}

	// Compute standard cards
	computed, err := ComputeFromFileCollection(ctx, fc)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	// Convert to Cards and upsert
	cards := computed.ToCards(session.SessionID, session.TotalLines)
	if err := p.analyticsStore.UpsertCards(ctx, cards); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	// Handle smart recap (with staleness check and quota)
	if p.config.SmartRecapEnabled {
		if err := p.precomputeSmartRecap(ctx, session, fc, cards.ToResponse().Cards); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return err
		}
	}

	span.SetAttributes(attribute.Bool("session.computed", true))
	return nil
}

// downloadTranscript downloads the transcript and agent files for a session.
func (p *Precomputer) downloadTranscript(ctx context.Context, session StaleSession) (*FileCollection, error) {
	ctx, span := tracer.Start(ctx, "precompute.download_transcript",
		trace.WithAttributes(attribute.String("session.id", session.SessionID)))
	defer span.End()

	// Get sync files for this session
	filesQuery := `
		SELECT file_name, file_type, last_synced_line
		FROM sync_files
		WHERE session_id = $1 AND file_type IN ('transcript', 'agent')
	`
	rows, err := p.db.QueryContext(ctx, filesQuery, session.SessionID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	defer rows.Close()

	type syncFile struct {
		FileName       string
		FileType       string
		LastSyncedLine int
	}
	var files []syncFile
	for rows.Next() {
		var f syncFile
		if err := rows.Scan(&f.FileName, &f.FileType, &f.LastSyncedLine); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}
		files = append(files, f)
	}
	if err := rows.Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	// Find main transcript and agent files
	var mainFile *syncFile
	var agentFiles []syncFile
	for i := range files {
		switch files[i].FileType {
		case "transcript":
			mainFile = &files[i]
		case "agent":
			agentFiles = append(agentFiles, files[i])
		}
	}

	if mainFile == nil {
		return nil, nil
	}

	// Download main transcript
	mainContent, err := p.downloadAndMergeFile(ctx, session.UserID, session.ExternalID, mainFile.FileName)
	if err != nil || mainContent == nil {
		return nil, err
	}

	// Download agent files
	agentContents := make(map[string][]byte)
	for _, af := range agentFiles {
		agentID := extractAgentID(af.FileName)
		if agentID == "" {
			continue
		}
		content, err := p.downloadAndMergeFile(ctx, session.UserID, session.ExternalID, af.FileName)
		if err != nil {
			// Log but continue - graceful degradation
			continue
		}
		if content != nil {
			agentContents[agentID] = content
		}
	}

	// Build FileCollection
	fc, err := NewFileCollectionWithAgents(mainContent, agentContents)
	if err != nil {
		return nil, err
	}

	return fc, nil
}

// downloadAndMergeFile downloads and merges all chunks for a file.
func (p *Precomputer) downloadAndMergeFile(ctx context.Context, userID int64, externalID, fileName string) ([]byte, error) {
	chunkKeys, err := p.store.ListChunks(ctx, userID, externalID, fileName)
	if err != nil {
		return nil, err
	}
	if len(chunkKeys) == 0 {
		return nil, nil
	}

	// Download all chunks
	var chunks [][]byte
	for _, key := range chunkKeys {
		data, err := p.store.Download(ctx, key)
		if err != nil {
			return nil, err
		}
		chunks = append(chunks, data)
	}

	// Merge chunks
	return mergeChunks(chunks)
}

// mergeChunks merges multiple byte slices into one.
func mergeChunks(chunks [][]byte) ([]byte, error) {
	if len(chunks) == 0 {
		return nil, nil
	}

	// Calculate total size
	totalSize := 0
	for _, chunk := range chunks {
		totalSize += len(chunk)
	}

	// Allocate and copy
	result := make([]byte, 0, totalSize)
	for _, chunk := range chunks {
		result = append(result, chunk...)
	}

	return result, nil
}

// precomputeSmartRecap handles smart recap generation with staleness and quota checks.
// Returns an error if smart recap generation fails. Returns nil if skipped (not stale, quota exceeded, lock held).
func (p *Precomputer) precomputeSmartRecap(ctx context.Context, session StaleSession, fc *FileCollection, cardStats map[string]interface{}) error {
	ctx, span := tracer.Start(ctx, "precompute.smart_recap",
		trace.WithAttributes(attribute.String("session.id", session.SessionID)))
	defer span.End()

	// Get current smart recap card
	smartCard, err := p.analyticsStore.GetSmartRecapCard(ctx, session.SessionID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	// Check if we need to regenerate
	// Only regenerate if: (1) no valid card OR (2) stale (new lines + time threshold)
	if smartCard != nil && smartCard.IsValid() && !smartCard.IsStale(session.TotalLines, p.config.StalenessMinutes) {
		span.SetAttributes(attribute.Bool("smart_recap.skipped", true), attribute.String("reason", "not_stale"))
		return nil
	}

	// Check quota for session owner
	quotaQuery := `
		SELECT compute_count FROM smart_recap_quota WHERE user_id = $1
	`
	var computeCount int
	err = p.db.QueryRowContext(ctx, quotaQuery, session.UserID).Scan(&computeCount)
	if err != nil && err != sql.ErrNoRows {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	if computeCount >= p.config.SmartRecapQuota {
		span.SetAttributes(attribute.Bool("smart_recap.skipped", true), attribute.String("reason", "quota_exceeded"))
		return nil
	}

	// Try to acquire lock
	acquired, err := p.analyticsStore.AcquireSmartRecapLock(ctx, session.SessionID, p.config.LockTimeoutSeconds)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	if !acquired {
		span.SetAttributes(attribute.Bool("smart_recap.skipped", true), attribute.String("reason", "lock_held"))
		return nil
	}

	// Generate smart recap
	client := anthropic.NewClient(p.config.AnthropicAPIKey)
	analyzer := NewSmartRecapAnalyzer(client, p.config.SmartRecapModel)

	genCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	result, err := analyzer.Analyze(genCtx, fc, cardStats)
	cancel()

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		// Clear lock on error
		_ = p.analyticsStore.ClearSmartRecapLock(ctx, session.SessionID)
		return err
	}

	// Save result (clears lock via upsert)
	card := &SmartRecapCardRecord{
		SessionID:                 session.SessionID,
		Version:                   SmartRecapCardVersion,
		ComputedAt:                time.Now().UTC(),
		UpToLine:                  session.TotalLines,
		Recap:                     result.Recap,
		WentWell:                  result.WentWell,
		WentBad:                   result.WentBad,
		HumanSuggestions:          result.HumanSuggestions,
		EnvironmentSuggestions:    result.EnvironmentSuggestions,
		DefaultContextSuggestions: result.DefaultContextSuggestions,
		ModelUsed:                 p.config.SmartRecapModel,
		InputTokens:               result.InputTokens,
		OutputTokens:              result.OutputTokens,
		GenerationTimeMs:          &result.GenerationTimeMs,
	}

	if err := p.analyticsStore.UpsertSmartRecapCard(ctx, card); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		// Clear lock on error
		_ = p.analyticsStore.ClearSmartRecapLock(ctx, session.SessionID)
		return err
	}

	// Update session with suggested title
	if result.SuggestedSessionTitle != "" {
		titleQuery := `UPDATE sessions SET suggested_session_title = $1 WHERE id = $2`
		_, _ = p.db.ExecContext(ctx, titleQuery, result.SuggestedSessionTitle, session.SessionID)
	}

	// Increment quota
	quotaIncrementQuery := `
		INSERT INTO smart_recap_quota (user_id, compute_count, last_compute_at)
		VALUES ($1, 1, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			compute_count = smart_recap_quota.compute_count + 1,
			last_compute_at = NOW()
	`
	_, _ = p.db.ExecContext(ctx, quotaIncrementQuery, session.UserID)

	span.SetAttributes(
		attribute.Bool("smart_recap.generated", true),
		attribute.Int("llm.tokens.input", result.InputTokens),
		attribute.Int("llm.tokens.output", result.OutputTokens),
	)
	return nil
}

// extractAgentID extracts the agent ID from a filename like "agent-{id}.jsonl".
func extractAgentID(fileName string) string {
	if !strings.HasPrefix(fileName, "agent-") || !strings.HasSuffix(fileName, ".jsonl") {
		return ""
	}
	return strings.TrimSuffix(strings.TrimPrefix(fileName, "agent-"), ".jsonl")
}
