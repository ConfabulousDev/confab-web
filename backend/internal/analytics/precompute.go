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

// StalenessThresholds holds configuration for determining when a session is stale enough
// to recompute. This allows polling frequently while only recomputing sessions that
// meet percentage-based staleness criteria.
type StalenessThresholds struct {
	// ThresholdPct is the percentage threshold (e.g., 0.20 for 20%)
	ThresholdPct float64
	// BaseMinLines is the minimum line gap floor
	BaseMinLines int64
	// BaseMinTime is the minimum time gap floor
	BaseMinTime time.Duration
	// MinInitialLines is the minimum lines before first compute
	MinInitialLines int64
	// MinSessionAge is the catch-all: compute after this session age even if below MinInitialLines
	MinSessionAge time.Duration
}

// DefaultRegularCardsThresholds returns sensible defaults for regular cards (cheap to compute).
func DefaultRegularCardsThresholds() StalenessThresholds {
	return StalenessThresholds{
		ThresholdPct:    0.20, // 20%
		BaseMinLines:    5,
		BaseMinTime:     3 * time.Minute,
		MinInitialLines: 10,
		MinSessionAge:   10 * time.Minute,
	}
}

// DefaultSmartRecapThresholds returns sensible defaults for smart recap (expensive LLM call).
func DefaultSmartRecapThresholds() StalenessThresholds {
	return StalenessThresholds{
		ThresholdPct:    0.20, // 20%
		BaseMinLines:    50,
		BaseMinTime:     15 * time.Minute,
		MinInitialLines: 10,
		MinSessionAge:   10 * time.Minute,
	}
}

// PrecomputeConfig holds configuration for the precomputer.
type PrecomputeConfig struct {
	SmartRecapEnabled  bool
	AnthropicAPIKey    string
	SmartRecapModel    string
	SmartRecapQuota    int
	LockTimeoutSeconds int

	// Staleness thresholds for each bucket
	RegularCardsThresholds StalenessThresholds
	SmartRecapThresholds   StalenessThresholds
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

// FindStaleSessions returns sessions where any card is stale based on configurable
// staleness thresholds. The algorithm prioritizes:
// 1. New sessions (no cards) with enough content or old enough
// 2. Version mismatches (always recompute)
// 3. Line gap or time gap exceeds threshold
//
// Sessions are ordered by: new sessions → version mismatch → largest line gap → last_sync_at
func (p *Precomputer) FindStaleSessions(ctx context.Context, limit int) ([]StaleSession, error) {
	ctx, span := tracer.Start(ctx, "precompute.find_stale_sessions",
		trace.WithAttributes(attribute.Int("limit", limit)))
	defer span.End()

	th := p.config.RegularCardsThresholds

	// Query implements the staleness algorithm:
	// 1. New sessions (any card NULL) with enough content OR old enough session
	// 2. Version mismatches (always trigger recompute)
	// 3. Percentage-based threshold: line_gap >= MAX(base_min_lines, up_to_line * pct)
	//    OR time_gap >= MAX(base_min_time, prior_duration * pct)
	//
	// min_up_to_line = minimum up_to_line across all existing cards (most stale point)
	// line_gap = total_lines - min_up_to_line
	// prior_duration = min_computed_at - first_seen (time covered by existing cards)
	// time_gap = NOW() - min_computed_at
	query := `
		WITH session_lines AS (
			SELECT session_id, SUM(last_synced_line) as total_lines
			FROM sync_files
			WHERE file_type IN ('transcript', 'agent')
			GROUP BY session_id
			HAVING SUM(last_synced_line) > 0
		),
		card_status AS (
			SELECT
				sl.session_id,
				s.user_id,
				s.external_id,
				sl.total_lines,
				s.first_seen,
				-- Check if ALL cards exist (not missing)
				CASE WHEN tc.session_id IS NOT NULL AND sc.session_id IS NOT NULL AND tl.session_id IS NOT NULL
				     AND ca.session_id IS NOT NULL AND cv.session_id IS NOT NULL AND as_card.session_id IS NOT NULL
				     AND rd.session_id IS NOT NULL
				THEN TRUE ELSE FALSE END AS all_cards_exist,
				-- Check if any existing card has wrong version (only meaningful when all cards exist)
				CASE WHEN (tc.session_id IS NOT NULL AND tc.version != $1)
				     OR (sc.session_id IS NOT NULL AND sc.version != $2)
				     OR (tl.session_id IS NOT NULL AND tl.version != $3)
				     OR (ca.session_id IS NOT NULL AND ca.version != $4)
				     OR (cv.session_id IS NOT NULL AND cv.version != $5)
				     OR (as_card.session_id IS NOT NULL AND as_card.version != $6)
				     OR (rd.session_id IS NOT NULL AND rd.version != $7)
				THEN TRUE ELSE FALSE END AS has_version_mismatch,
				-- Minimum up_to_line across all cards (most stale point)
				LEAST(
					COALESCE(tc.up_to_line, 0), COALESCE(sc.up_to_line, 0),
					COALESCE(tl.up_to_line, 0), COALESCE(ca.up_to_line, 0),
					COALESCE(cv.up_to_line, 0), COALESCE(as_card.up_to_line, 0),
					COALESCE(rd.up_to_line, 0)
				) AS min_up_to_line,
				-- Oldest computed_at across all cards (earliest computation)
				LEAST(
					COALESCE(tc.computed_at, NOW()), COALESCE(sc.computed_at, NOW()),
					COALESCE(tl.computed_at, NOW()), COALESCE(ca.computed_at, NOW()),
					COALESCE(cv.computed_at, NOW()), COALESCE(as_card.computed_at, NOW()),
					COALESCE(rd.computed_at, NOW())
				) AS min_computed_at,
				s.last_sync_at
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
		),
		stale_sessions AS (
			SELECT
				cs.*,
				-- Calculate line gap
				cs.total_lines - cs.min_up_to_line AS line_gap,
				-- Calculate time gap in seconds
				EXTRACT(EPOCH FROM (NOW() - cs.min_computed_at)) AS time_gap_secs,
				-- Calculate prior duration in seconds (time covered by cache)
				EXTRACT(EPOCH FROM (cs.min_computed_at - cs.first_seen)) AS prior_duration_secs,
				-- Calculate session age in seconds
				EXTRACT(EPOCH FROM (NOW() - cs.first_seen)) AS session_age_secs,
				-- Line threshold = MAX(base_min_lines, up_to_line * pct)
				GREATEST($8::bigint, (cs.min_up_to_line::float8 * $9::float8)::bigint) AS line_threshold,
				-- Staleness category for ordering (1=new, 2=version mismatch, 3=threshold met)
				CASE
					WHEN cs.all_cards_exist = FALSE THEN 1
					WHEN cs.has_version_mismatch = TRUE THEN 2
					ELSE 3
				END AS staleness_category
			FROM card_status cs
		)
		SELECT session_id, user_id, external_id, total_lines
		FROM stale_sessions
		WHERE
			-- Case 1: New session (missing cards) with enough content OR old enough
			(all_cards_exist = FALSE AND (
				total_lines >= $11  -- min_initial_lines
				OR session_age_secs >= $12  -- min_session_age in seconds
			))
			-- Case 2: Version mismatch - always recompute
			OR (all_cards_exist = TRUE AND has_version_mismatch = TRUE)
			-- Case 3: Existing cards with line_gap > 0 that meet threshold
			OR (all_cards_exist = TRUE AND has_version_mismatch = FALSE AND line_gap > 0 AND (
				-- Line gap meets threshold
				line_gap >= line_threshold
				-- OR time gap meets threshold: MAX(base_min_time, prior_duration * pct)
				OR time_gap_secs >= GREATEST($10::float8, prior_duration_secs * $9::float8)
			))
		ORDER BY
			staleness_category,           -- New sessions first, then version mismatches, then threshold
			line_gap DESC NULLS LAST,     -- Largest line gap within category
			last_sync_at DESC NULLS LAST  -- Most recently synced as tie-breaker
		LIMIT $13
	`

	rows, err := p.db.QueryContext(ctx, query,
		TokensCardVersion,           // $1
		SessionCardVersion,          // $2
		ToolsCardVersion,            // $3
		CodeActivityCardVersion,     // $4
		ConversationCardVersion,     // $5
		AgentsAndSkillsCardVersion,  // $6
		RedactionsCardVersion,       // $7
		th.BaseMinLines,             // $8
		th.ThresholdPct,             // $9
		th.BaseMinTime.Seconds(),    // $10
		th.MinInitialLines,          // $11
		th.MinSessionAge.Seconds(),  // $12
		limit,                       // $13
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

// PrecomputeRegularCards computes only the regular analytics cards for a session.
// Smart recap is handled separately via PrecomputeSmartRecapOnly with its own staleness thresholds.
func (p *Precomputer) PrecomputeRegularCards(ctx context.Context, session StaleSession) error {
	ctx, span := tracer.Start(ctx, "precompute.regular_cards",
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
// Smart recap is stale based on configurable staleness thresholds:
// 1. Missing smart recap with enough content or old enough session
// 2. Version mismatch (always recompute)
// 3. Line gap or time gap exceeds threshold
//
// This complements FindStaleSessions which finds sessions with stale regular cards.
func (p *Precomputer) FindStaleSmartRecapSessions(ctx context.Context, limit int) ([]StaleSession, error) {
	ctx, span := tracer.Start(ctx, "precompute.find_stale_smart_recap_sessions",
		trace.WithAttributes(attribute.Int("limit", limit)))
	defer span.End()

	if !p.config.SmartRecapEnabled {
		span.SetAttributes(attribute.Bool("smart_recap.disabled", true))
		return nil, nil
	}

	th := p.config.SmartRecapThresholds

	// Query implements the staleness algorithm for smart recap:
	// 1. All regular cards must be valid (up-to-date)
	// 2. Smart recap is stale if:
	//    - Missing with enough content OR old enough session
	//    - Version mismatch
	//    - Line gap or time gap meets threshold
	query := `
		WITH session_lines AS (
			SELECT session_id, SUM(last_synced_line) as total_lines
			FROM sync_files
			WHERE file_type IN ('transcript', 'agent')
			GROUP BY session_id
			HAVING SUM(last_synced_line) > 0
		),
		recap_status AS (
			SELECT
				sl.session_id,
				s.user_id,
				s.external_id,
				sl.total_lines,
				s.first_seen,
				s.last_sync_at,
				-- Smart recap card status
				sr.session_id IS NULL AS is_missing,
				CASE WHEN sr.session_id IS NOT NULL AND sr.version != $8 THEN TRUE ELSE FALSE END AS has_version_mismatch,
				COALESCE(sr.up_to_line, 0) AS up_to_line,
				sr.computed_at,
				-- Calculate line gap
				sl.total_lines - COALESCE(sr.up_to_line, 0) AS line_gap,
				-- Calculate time gap in seconds (only if card exists)
				CASE WHEN sr.computed_at IS NOT NULL
					THEN EXTRACT(EPOCH FROM (NOW() - sr.computed_at))
					ELSE 0
				END AS time_gap_secs,
				-- Calculate prior duration in seconds (time covered by cache)
				CASE WHEN sr.computed_at IS NOT NULL AND s.first_seen IS NOT NULL
					THEN EXTRACT(EPOCH FROM (sr.computed_at - s.first_seen))
					ELSE 0
				END AS prior_duration_secs,
				-- Calculate session age in seconds
				EXTRACT(EPOCH FROM (NOW() - s.first_seen)) AS session_age_secs,
				-- Line threshold = MAX(base_min_lines, up_to_line * pct)
				GREATEST($9::bigint, (COALESCE(sr.up_to_line, 0)::float8 * $10::float8)::bigint) AS line_threshold,
				-- Staleness category for ordering (1=new, 2=version mismatch, 3=threshold met)
				CASE
					WHEN sr.session_id IS NULL THEN 1
					WHEN sr.version != $8 THEN 2
					ELSE 3
				END AS staleness_category
			FROM session_lines sl
			JOIN sessions s ON sl.session_id = s.id
			-- All regular cards must be valid
			JOIN session_card_tokens tc ON sl.session_id = tc.session_id
				AND tc.version = $1 AND tc.up_to_line = sl.total_lines
			JOIN session_card_session sc ON sl.session_id = sc.session_id
				AND sc.version = $2 AND sc.up_to_line = sl.total_lines
			JOIN session_card_tools tl ON sl.session_id = tl.session_id
				AND tl.version = $3 AND tl.up_to_line = sl.total_lines
			JOIN session_card_code_activity ca ON sl.session_id = ca.session_id
				AND ca.version = $4 AND ca.up_to_line = sl.total_lines
			JOIN session_card_conversation cv ON sl.session_id = cv.session_id
				AND cv.version = $5 AND cv.up_to_line = sl.total_lines
			JOIN session_card_agents_and_skills as_card ON sl.session_id = as_card.session_id
				AND as_card.version = $6 AND as_card.up_to_line = sl.total_lines
			JOIN session_card_redactions rd ON sl.session_id = rd.session_id
				AND rd.version = $7 AND rd.up_to_line = sl.total_lines
			LEFT JOIN session_card_smart_recap sr ON sl.session_id = sr.session_id
			WHERE s.session_type = 'Claude Code'
		)
		SELECT session_id, user_id, external_id, total_lines
		FROM recap_status
		WHERE
			-- Case 1: Missing smart recap with enough content OR old enough
			(is_missing = TRUE AND (
				total_lines >= $12::bigint  -- min_initial_lines
				OR session_age_secs >= $13::float8  -- min_session_age in seconds
			))
			-- Case 2: Version mismatch - always recompute
			OR (is_missing = FALSE AND has_version_mismatch = TRUE)
			-- Case 3: Existing card with line_gap > 0 that meets threshold
			OR (is_missing = FALSE AND has_version_mismatch = FALSE AND line_gap > 0 AND (
				-- Line gap meets threshold
				line_gap >= line_threshold
				-- OR time gap meets threshold: MAX(base_min_time, prior_duration * pct)
				OR time_gap_secs >= GREATEST($11::float8, prior_duration_secs * $10::float8)
			))
		ORDER BY
			staleness_category,           -- Missing first, then version mismatches, then threshold
			line_gap DESC NULLS LAST,     -- Largest line gap within category
			last_sync_at DESC NULLS LAST  -- Most recently synced as tie-breaker
		LIMIT $14
	`

	rows, err := p.db.QueryContext(ctx, query,
		TokensCardVersion,             // $1
		SessionCardVersion,            // $2
		ToolsCardVersion,              // $3
		CodeActivityCardVersion,       // $4
		ConversationCardVersion,       // $5
		AgentsAndSkillsCardVersion,    // $6
		RedactionsCardVersion,         // $7
		SmartRecapCardVersion,         // $8
		th.BaseMinLines,               // $9
		th.ThresholdPct,               // $10
		th.BaseMinTime.Seconds(),      // $11
		th.MinInitialLines,            // $12
		th.MinSessionAge.Seconds(),    // $13
		limit,                         // $14
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
