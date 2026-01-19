package analytics

import (
	"context"
	"database/sql"
	"time"

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
	LockTimeoutSeconds int
}

// Precomputer handles background analytics precomputation.
type Precomputer struct {
	db                  *sql.DB
	store               *storage.S3Storage
	analyticsStore      *Store
	config              PrecomputeConfig
	smartRecapGenerator *SmartRecapGenerator
}

// precomputeDB implements SmartRecapDB using raw SQL queries.
// This allows the precomputer to use the shared SmartRecapGenerator
// without depending on the db package.
type precomputeDB struct {
	db *sql.DB
}

func (p *precomputeDB) IncrementSmartRecapQuota(ctx context.Context, userID int64) error {
	query := `
		INSERT INTO smart_recap_quota (user_id, compute_count, last_compute_at)
		VALUES ($1, 1, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			compute_count = smart_recap_quota.compute_count + 1,
			last_compute_at = NOW()
	`
	_, err := p.db.ExecContext(ctx, query, userID)
	return err
}

func (p *precomputeDB) UpdateSessionSuggestedTitle(ctx context.Context, sessionID string, title string) error {
	query := `UPDATE sessions SET suggested_session_title = $1 WHERE id = $2`
	_, err := p.db.ExecContext(ctx, query, title, sessionID)
	return err
}

// NewPrecomputer creates a new Precomputer.
func NewPrecomputer(db *sql.DB, store *storage.S3Storage, analyticsStore *Store, config PrecomputeConfig) *Precomputer {
	p := &Precomputer{
		db:             db,
		store:          store,
		analyticsStore: analyticsStore,
		config:         config,
	}

	// Create the shared smart recap generator if enabled
	if config.SmartRecapEnabled {
		p.smartRecapGenerator = NewSmartRecapGenerator(
			analyticsStore,
			&precomputeDB{db: db},
			SmartRecapGeneratorConfig{
				APIKey:            config.AnthropicAPIKey,
				Model:             config.SmartRecapModel,
				GenerationTimeout: 60 * time.Second,
			},
		)
	}

	return p
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
		LEFT JOIN session_card_agents_and_skills as_card ON sl.session_id = as_card.session_id
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

	// Download main transcript using the robust chunk merge from storage package
	mainContent, err := p.store.DownloadAndMergeChunks(ctx, session.UserID, session.ExternalID, mainFile.FileName)
	if err != nil || mainContent == nil {
		return nil, err
	}

	// Download agent files
	agentContents := make(map[string][]byte)
	for _, af := range agentFiles {
		agentID := ExtractAgentID(af.FileName)
		if agentID == "" {
			continue
		}
		content, err := p.store.DownloadAndMergeChunks(ctx, session.UserID, session.ExternalID, af.FileName)
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

// precomputeSmartRecap handles smart recap generation with line count and quota checks.
// Returns an error if smart recap generation fails. Returns nil if skipped (up-to-date, quota exceeded, lock held).
func (p *Precomputer) precomputeSmartRecap(ctx context.Context, session StaleSession, fc *FileCollection, cardStats map[string]interface{}) error {
	ctx, span := tracer.Start(ctx, "precompute.smart_recap",
		trace.WithAttributes(attribute.String("session.id", session.SessionID)))
	defer span.End()

	// Get current smart recap card to check if up-to-date
	smartCard, err := p.analyticsStore.GetSmartRecapCard(ctx, session.SessionID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	// Check if we need to regenerate - skip if card is up-to-date
	if smartCard.IsUpToDate(session.TotalLines) {
		span.SetAttributes(attribute.Bool("smart_recap.skipped", true), attribute.String("reason", "up_to_date"))
		return nil
	}

	// Check quota for session owner
	quotaQuery := `SELECT compute_count FROM smart_recap_quota WHERE user_id = $1`
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

	// Use the shared generator for the actual generation (handles lock, LLM call, save, quota increment)
	result := p.smartRecapGenerator.Generate(ctx, GenerateInput{
		SessionID:      session.SessionID,
		UserID:         session.UserID,
		LineCount:      session.TotalLines,
		FileCollection: fc,
		CardStats:      cardStats,
	}, p.config.LockTimeoutSeconds)

	if result.Skipped {
		span.SetAttributes(attribute.Bool("smart_recap.skipped", true), attribute.String("reason", "lock_held"))
		return nil
	}
	if result.Error != nil {
		span.RecordError(result.Error)
		span.SetStatus(codes.Error, result.Error.Error())
		return result.Error
	}

	span.SetAttributes(
		attribute.Bool("smart_recap.generated", true),
		attribute.Int("llm.tokens.input", result.Card.InputTokens),
		attribute.Int("llm.tokens.output", result.Card.OutputTokens),
	)
	return nil
}

// FindStaleSmartRecapSessions returns sessions where smart recap is stale but regular cards are up-to-date.
// Smart recap is stale if: missing OR wrong version OR has new lines (up_to_line < total_lines).
// This complements FindStaleSessions which finds sessions with stale regular cards.
func (p *Precomputer) FindStaleSmartRecapSessions(ctx context.Context, limit int) ([]StaleSession, error) {
	ctx, span := tracer.Start(ctx, "precompute.find_stale_smart_recap_sessions",
		trace.WithAttributes(attribute.Int("limit", limit)))
	defer span.End()

	if !p.config.SmartRecapEnabled {
		span.SetAttributes(attribute.Bool("smart_recap.disabled", true))
		return nil, nil
	}

	// Query finds sessions where:
	// 1. All regular cards are valid (up-to-date)
	// 2. Smart recap is stale (missing, wrong version, or has new lines)
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
		LEFT JOIN session_card_agents_and_skills as_card ON sl.session_id = as_card.session_id
		LEFT JOIN session_card_redactions rd ON sl.session_id = rd.session_id
		LEFT JOIN session_card_smart_recap sr ON sl.session_id = sr.session_id
		WHERE s.session_type = 'Claude Code'
		  -- All regular cards must be valid
		  AND (
			tc.session_id IS NOT NULL AND tc.version = $1 AND tc.up_to_line = sl.total_lines
			AND sc.session_id IS NOT NULL AND sc.version = $2 AND sc.up_to_line = sl.total_lines
			AND tl.session_id IS NOT NULL AND tl.version = $3 AND tl.up_to_line = sl.total_lines
			AND ca.session_id IS NOT NULL AND ca.version = $4 AND ca.up_to_line = sl.total_lines
			AND cv.session_id IS NOT NULL AND cv.version = $5 AND cv.up_to_line = sl.total_lines
			AND as_card.session_id IS NOT NULL AND as_card.version = $6 AND as_card.up_to_line = sl.total_lines
			AND rd.session_id IS NOT NULL AND rd.version = $7 AND rd.up_to_line = sl.total_lines
		  )
		  -- Smart recap must be stale (missing, wrong version, or has new lines)
		  AND NOT (
			sr.session_id IS NOT NULL
			AND sr.version = $8
			AND sr.up_to_line >= sl.total_lines
		  )
		ORDER BY s.last_sync_at DESC NULLS LAST
		LIMIT $9
	`

	rows, err := p.db.QueryContext(ctx, query,
		TokensCardVersion,
		SessionCardVersion,
		ToolsCardVersion,
		CodeActivityCardVersion,
		ConversationCardVersion,
		AgentsAndSkillsCardVersion,
		RedactionsCardVersion,
		SmartRecapCardVersion,
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

// PrecomputeSmartRecapOnly computes only the smart recap for a session.
// Use this when regular cards are already up-to-date but smart recap is stale.
// It downloads the transcript, fetches existing card stats from DB, and generates smart recap.
func (p *Precomputer) PrecomputeSmartRecapOnly(ctx context.Context, session StaleSession) error {
	ctx, span := tracer.Start(ctx, "precompute.smart_recap_only",
		trace.WithAttributes(
			attribute.String("session.id", session.SessionID),
			attribute.Int64("session.user_id", session.UserID),
			attribute.Int64("session.total_lines", session.TotalLines),
		))
	defer span.End()

	if !p.config.SmartRecapEnabled {
		span.SetAttributes(attribute.Bool("smart_recap.disabled", true))
		return nil
	}

	// Download transcript (needed for smart recap generation)
	fc, err := p.downloadTranscript(ctx, session)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	if fc == nil {
		span.SetAttributes(attribute.Bool("session.empty", true))
		return nil
	}

	// Fetch existing card stats from DB (regular cards are already up-to-date)
	cards, err := p.analyticsStore.GetCards(ctx, session.SessionID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	// Convert to card stats map for smart recap
	var cardStats map[string]interface{}
	if cards != nil {
		cardStats = cards.ToResponse().Cards
	}

	// Generate smart recap (handles staleness check, quota, lock, etc.)
	if err := p.precomputeSmartRecap(ctx, session, fc, cardStats); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetAttributes(attribute.Bool("session.computed", true))
	return nil
}
