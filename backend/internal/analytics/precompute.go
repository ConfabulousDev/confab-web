package analytics

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/recapquota"
	"github.com/ConfabulousDev/confab-web/internal/storage"
	"github.com/lib/pq"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// ErrQuotaExceeded is returned when a user has exceeded their smart recap quota for the month.
var ErrQuotaExceeded = errors.New("smart recap quota exceeded")

// StaleSession represents a session that needs analytics precomputation.
type StaleSession struct {
	SessionID  string
	UserID     int64
	ExternalID string
	// Provider is the canonical session_type ("claude-code" or "codex"),
	// scanned from sessions.session_type and normalized via models.NormalizeProvider.
	// Two consumers:
	//   1. Chunk-storage calls — chunks are read from the provider-scoped S3 prefix.
	//   2. ProviderFor registry lookup — routes to the right per-provider parser.
	Provider   string
	TotalLines int64
	// RegenRequestedAt is non-nil when this session was surfaced due to an
	// admin-triggered bulk regeneration (staleness category 4). When set,
	// the precomputer bypasses quota checks and does not increment quota.
	RegenRequestedAt *time.Time
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
		BaseMinLines:    150,
		BaseMinTime:     30 * time.Minute,
		MinInitialLines: 25,
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

	// LLM token limits (0 means use defaults)
	MaxOutputTokens     int
	MaxTranscriptTokens int

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

// NewPrecomputer creates a new Precomputer.
// The database parameter is optional; if provided, it enables custom prompt
// lookups in the smart recap generator. Pass nil in tests that don't need this.
func NewPrecomputer(rawDB *sql.DB, store *storage.S3Storage, analyticsStore *Store, config PrecomputeConfig, database ...*db.DB) *Precomputer {
	p := &Precomputer{
		db:             rawDB,
		store:          store,
		analyticsStore: analyticsStore,
		config:         config,
	}

	// Create the shared smart recap generator if enabled.
	// Requires the wrapped *db.DB for admin_settings lookups in the generator.
	var wrappedDB *db.DB
	if len(database) > 0 {
		wrappedDB = database[0]
	}
	if config.SmartRecapEnabled && wrappedDB != nil {
		p.smartRecapGenerator = NewSmartRecapGenerator(
			analyticsStore,
			wrappedDB,
			SmartRecapGeneratorConfig{
				APIKey:              config.AnthropicAPIKey,
				Model:               config.SmartRecapModel,
				GenerationTimeout:   60 * time.Second,
				MaxOutputTokens:     config.MaxOutputTokens,
				MaxTranscriptTokens: config.MaxTranscriptTokens,
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
				s.session_type,
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
			-- Provider filter: pq.Array(models.AllowedProviders) is the
			-- permanent allowlist (canonical forms + legacy aliases). See
			-- internal/models/provider.go for the OSS self-hosted aliasing
			-- rationale; TestRegistryCoversAllowedProviders is the guard
			-- against drift between this list and the analytics registry.
			WHERE s.session_type = ANY($14)
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
		SELECT session_id, user_id, external_id, session_type, total_lines
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
		TokensCardVersion,                 // $1
		SessionCardVersion,                // $2
		ToolsCardVersion,                  // $3
		CodeActivityCardVersion,           // $4
		ConversationCardVersion,           // $5
		AgentsAndSkillsCardVersion,        // $6
		RedactionsCardVersion,             // $7
		th.BaseMinLines,                   // $8
		th.ThresholdPct,                   // $9
		th.BaseMinTime.Seconds(),          // $10
		th.MinInitialLines,                // $11
		th.MinSessionAge.Seconds(),        // $12
		limit,                             // $13
		pq.Array(models.AllowedProviders), // $14
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
		var rawProvider string
		if err := rows.Scan(&s.SessionID, &s.UserID, &s.ExternalID, &rawProvider, &s.TotalLines); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}
		s.Provider = models.NormalizeProvider(rawProvider)
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

// PrecomputeRegularCards computes only the regular analytics cards for a
// session. Smart recap is handled separately via PrecomputeSmartRecapOnly with
// its own staleness thresholds.
func (p *Precomputer) PrecomputeRegularCards(ctx context.Context, session StaleSession) error {
	ctx, span := tracer.Start(ctx, "precompute.regular_cards",
		trace.WithAttributes(
			attribute.String("session.id", session.SessionID),
			attribute.String("session.provider", session.Provider),
			attribute.Int64("session.user_id", session.UserID),
			attribute.Int64("session.total_lines", session.TotalLines),
		))
	defer span.End()

	sp, err := ProviderFor(session.Provider)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	rollout, err := sp.Parse(ctx, p.parseInput(session))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	if rollout == nil {
		span.SetAttributes(attribute.Bool("session.empty", true))
		return nil
	}

	computed := sp.ComputeCards(ctx, rollout)
	if computed == nil {
		err := fmt.Errorf("provider %q returned nil compute result", session.Provider)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	if computed.SkippedAgentFiles > 0 {
		span.SetAttributes(attribute.Int("agent_files.skipped", computed.SkippedAgentFiles))
	}

	cards := computed.ToCards(session.SessionID, session.TotalLines)
	if err := p.analyticsStore.UpsertCards(ctx, cards); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetAttributes(attribute.Bool("session.computed", true))
	return nil
}

func (p *Precomputer) parseInput(session StaleSession) ParseInput {
	return ParseInput{
		DB:         p.db,
		Store:      p.store,
		SessionID:  session.SessionID,
		UserID:     session.UserID,
		Provider:   session.Provider,
		ExternalID: session.ExternalID,
	}
}

// precomputeSmartRecap handles smart recap generation with line count and quota checks.
// Returns an error if smart recap generation fails. Returns nil if skipped (up-to-date, quota exceeded, lock held).
func (p *Precomputer) precomputeSmartRecap(ctx context.Context, session StaleSession, input GenerateInput, clearMessageIDs bool) error {
	ctx, span := tracer.Start(ctx, "precompute.smart_recap",
		trace.WithAttributes(attribute.String("session.id", session.SessionID)))
	defer span.End()

	isAdminRegen := session.RegenRequestedAt != nil
	if isAdminRegen {
		span.SetAttributes(attribute.Bool("admin_regen", true))
	}

	// Get current smart recap card to check if up-to-date
	smartCard, err := p.analyticsStore.GetSmartRecapCard(ctx, session.SessionID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	// Check if we need to regenerate - skip if card is up-to-date.
	// For admin-triggered regeneration (category 4), also check computed_at < regen_requested_at
	// since the card may be "up to date" by version+lines but stale by admin request.
	if smartCard.IsUpToDate(session.TotalLines) {
		if !isAdminRegen || !smartCard.ComputedAt.Before(*session.RegenRequestedAt) {
			span.SetAttributes(attribute.Bool("smart_recap.skipped", true), attribute.String("reason", "up_to_date"))
			return nil
		}
	}

	// Quota check: admin-triggered regeneration bypasses quota entirely.
	if !isAdminRegen {
		// Ensure quota record exists and check limit (creates row if missing so
		// the later Increment call in the generator never fails on a missing row).
		quota, err := recapquota.GetOrCreate(ctx, p.db, session.UserID)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return err
		}
		if p.config.SmartRecapQuota > 0 && quota.ComputeCount >= p.config.SmartRecapQuota {
			span.SetAttributes(attribute.Bool("smart_recap.skipped", true), attribute.String("reason", "quota_exceeded"))
			return ErrQuotaExceeded
		}
	}

	// Use the shared generator for the actual generation (handles lock, LLM call, save, quota increment)
	result := p.smartRecapGenerator.generate(ctx, input, p.config.LockTimeoutSeconds, isAdminRegen, clearMessageIDs)

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
	//    - Admin-triggered regeneration (computed_at < regen_requested_at) [category 4]
	// Category 4 (admin regen) bypasses quota checks — the admin explicitly requested it.
	//
	// Per-session admin invalidation (CF-343) also bypasses quota: if there's an unconsumed
	// admin_card_invalidations row covering session_card_smart_recap (invalidated_at > current
	// recap's computed_at, or recap is missing), the session qualifies regardless of quota.
	query := `
		WITH session_lines AS (
			SELECT session_id, SUM(last_synced_line) as total_lines
			FROM sync_files
			WHERE file_type IN ('transcript', 'agent')
			GROUP BY session_id
			HAVING SUM(last_synced_line) > 0
		),
		regen_ts AS (
			-- Read admin-triggered regeneration timestamp from admin_settings.
			-- When an admin clicks "Regenerate All", this row is upserted with NOW().
			-- Cards with computed_at < this timestamp are treated as stale (category 4).
			SELECT value::timestamptz AS requested_at
			FROM admin_settings
			WHERE key = 'smart_recap_regen_requested_at'
		),
		admin_invalidations AS (
			-- Per-session admin invalidations covering smart recap. The MAX captures
			-- the latest invalidation; the bypass clause below treats it as "unconsumed"
			-- if the smart recap is missing or was computed before invalidated_at.
			SELECT session_id, MAX(invalidated_at) AS last_invalidated_at
			FROM admin_card_invalidations
			WHERE 'session_card_smart_recap' = ANY(card_types)
			GROUP BY session_id
		),
		recap_status AS (
			SELECT
				sl.session_id,
				s.user_id,
				s.external_id,
				s.session_type,
				sl.total_lines,
				s.first_seen,
				s.last_sync_at,
				-- Smart recap card status
				sr.session_id IS NULL AS is_missing,
				CASE WHEN sr.session_id IS NOT NULL AND sr.version != $8 THEN TRUE ELSE FALSE END AS has_version_mismatch,
				COALESCE(sr.up_to_line, 0) AS up_to_line,
				sr.computed_at,
				-- Admin regen: card exists and was computed before the regen request
				CASE WHEN sr.session_id IS NOT NULL AND rt.requested_at IS NOT NULL
					AND sr.computed_at < rt.requested_at THEN TRUE ELSE FALSE END AS needs_admin_regen,
				rt.requested_at AS regen_requested_at,
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
				-- Staleness category for ordering (1=new, 2=version mismatch, 3=threshold met, 4=admin regen)
				CASE
					WHEN sr.session_id IS NULL THEN 1
					WHEN sr.version != $8 THEN 2
					WHEN sr.session_id IS NOT NULL AND rt.requested_at IS NOT NULL
						AND sr.computed_at < rt.requested_at THEN 4
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
			LEFT JOIN smart_recap_quota sq ON s.user_id = sq.user_id
				AND sq.quota_month = TO_CHAR(NOW() AT TIME ZONE 'UTC', 'YYYY-MM')
			LEFT JOIN regen_ts rt ON TRUE
			LEFT JOIN admin_invalidations ai ON ai.session_id = sl.session_id
			-- Provider filter: pq.Array(models.AllowedProviders) is the
			-- permanent allowlist (canonical forms + legacy aliases). See
			-- internal/models/provider.go for the OSS self-hosted aliasing
			-- rationale.
			WHERE s.session_type = ANY($16)
				-- Quota check: skip for category 4 (global admin regen) and for
				-- per-session admin invalidations (CF-343). Bypass clauses OR together.
				AND (
					$15::int = 0
					OR COALESCE(sq.compute_count, 0) < $15::int
					-- Global admin regen (category 4):
					OR (sr.session_id IS NOT NULL AND rt.requested_at IS NOT NULL
						AND sr.computed_at < rt.requested_at)
					-- Per-session admin invalidation (CF-343): unconsumed invalidation.
					OR (ai.last_invalidated_at IS NOT NULL
						AND (sr.session_id IS NULL OR sr.computed_at < ai.last_invalidated_at))
				)
		)
		SELECT session_id, user_id, external_id, session_type, total_lines,
			CASE WHEN needs_admin_regen THEN regen_requested_at ELSE NULL END AS regen_requested_at
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
			-- Case 4: Admin-triggered regeneration (computed_at < regen_requested_at)
			OR needs_admin_regen = TRUE
		ORDER BY
			staleness_category,           -- Missing first, then version mismatches, then threshold, then admin regen
			line_gap DESC NULLS LAST,     -- Largest line gap within category
			last_sync_at DESC NULLS LAST  -- Most recently synced as tie-breaker
		LIMIT $14
	`

	rows, err := p.db.QueryContext(ctx, query,
		TokensCardVersion,                 // $1
		SessionCardVersion,                // $2
		ToolsCardVersion,                  // $3
		CodeActivityCardVersion,           // $4
		ConversationCardVersion,           // $5
		AgentsAndSkillsCardVersion,        // $6
		RedactionsCardVersion,             // $7
		SmartRecapCardVersion,             // $8
		th.BaseMinLines,                   // $9
		th.ThresholdPct,                   // $10
		th.BaseMinTime.Seconds(),          // $11
		th.MinInitialLines,                // $12
		th.MinSessionAge.Seconds(),        // $13
		limit,                             // $14
		p.config.SmartRecapQuota,          // $15
		pq.Array(models.AllowedProviders), // $16
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
		var rawProvider string
		if err := rows.Scan(&s.SessionID, &s.UserID, &s.ExternalID, &rawProvider, &s.TotalLines, &s.RegenRequestedAt); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}
		s.Provider = models.NormalizeProvider(rawProvider)
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

// FindStaleSearchIndexSessions returns sessions where the search index is stale
// but all 7 regular cards are up-to-date. A session's search index is stale when:
// 1. No index exists (never indexed)
// 2. Version mismatch (search logic changed)
// 3. Transcript grew (indexed_up_to_line < total_lines)
// 4. Recap changed (recap computed_at > recap_indexed_at, or recap exists but not indexed)
// 5. Metadata changed (MD5 hash mismatch on titles/summary/first_user_message)
func (p *Precomputer) FindStaleSearchIndexSessions(ctx context.Context, limit int) ([]StaleSession, error) {
	ctx, span := tracer.Start(ctx, "precompute.find_stale_search_index_sessions",
		trace.WithAttributes(attribute.Int("limit", limit)))
	defer span.End()

	query := `
		WITH session_lines AS (
			SELECT session_id, SUM(last_synced_line) as total_lines
			FROM sync_files
			WHERE file_type IN ('transcript', 'agent')
			GROUP BY session_id
			HAVING SUM(last_synced_line) > 0
		)
		SELECT sl.session_id, s.user_id, s.external_id, s.session_type, sl.total_lines
		FROM session_lines sl
		JOIN sessions s ON sl.session_id = s.id
		-- All 7 regular cards must be current
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
		LEFT JOIN session_search_index si ON sl.session_id = si.session_id
		LEFT JOIN session_card_smart_recap sr ON sl.session_id = sr.session_id
		-- Provider filter: pq.Array(models.AllowedProviders) is the
		-- permanent allowlist (canonical forms + legacy aliases). See
		-- internal/models/provider.go for the OSS self-hosted aliasing
		-- rationale.
		WHERE s.session_type = ANY($10)
		  AND (
			-- 1. Never indexed
			si.session_id IS NULL
			-- 2. Version mismatch
			OR si.version != $8
			-- 3. Transcript grew
			OR si.indexed_up_to_line < sl.total_lines
			-- 4. Recap changed (recap exists but not yet indexed, or recap recomputed after indexing)
			OR (sr.session_id IS NOT NULL AND (si.recap_indexed_at IS NULL OR sr.computed_at > si.recap_indexed_at))
			-- 5. Metadata changed
			OR si.metadata_hash != MD5(COALESCE(s.custom_title, '') || '|' || COALESCE(s.suggested_session_title, '') || '|' || COALESCE(s.summary, '') || '|' || COALESCE(s.first_user_message, ''))
		  )
		ORDER BY s.last_sync_at DESC NULLS LAST
		LIMIT $9
	`

	rows, err := p.db.QueryContext(ctx, query,
		TokensCardVersion,                 // $1
		SessionCardVersion,                // $2
		ToolsCardVersion,                  // $3
		CodeActivityCardVersion,           // $4
		ConversationCardVersion,           // $5
		AgentsAndSkillsCardVersion,        // $6
		RedactionsCardVersion,             // $7
		SearchIndexVersion,                // $8
		limit,                             // $9
		pq.Array(models.AllowedProviders), // $10
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
		var rawProvider string
		if err := rows.Scan(&s.SessionID, &s.UserID, &s.ExternalID, &rawProvider, &s.TotalLines); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}
		s.Provider = models.NormalizeProvider(rawProvider)
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

// BuildSearchIndexOnly builds the search index for a session.
func (p *Precomputer) BuildSearchIndexOnly(ctx context.Context, session StaleSession) error {
	ctx, span := tracer.Start(ctx, "precompute.build_search_index",
		trace.WithAttributes(
			attribute.String("session.id", session.SessionID),
			attribute.String("session.provider", session.Provider),
			attribute.Int64("session.total_lines", session.TotalLines),
		))
	defer span.End()

	sp, err := ProviderFor(session.Provider)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	rollout, err := sp.Parse(ctx, p.parseInput(session))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	if rollout == nil {
		span.SetAttributes(attribute.Bool("session.empty", true))
		return nil
	}

	content, err := ExtractSearchContentWithUserMessages(ctx, p.db, session.SessionID, sp.SearchText(ctx, rollout))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	recapIndexedAt, err := p.loadRecapIndexedAt(ctx, session.SessionID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	record := &SearchIndexRecord{
		SessionID:       session.SessionID,
		Version:         SearchIndexVersion,
		IndexedUpToLine: session.TotalLines,
		RecapIndexedAt:  recapIndexedAt,
		MetadataHash:    content.MetadataHash,
	}

	if err := p.analyticsStore.UpsertSearchIndex(ctx, record, content); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetAttributes(attribute.Bool("session.indexed", true))
	return nil
}

// loadRecapIndexedAt fetches session_card_smart_recap.computed_at (UTC) for the
// given session, returning nil if the row does not exist. Used when building
// SearchIndexRecord.RecapIndexedAt.
func (p *Precomputer) loadRecapIndexedAt(ctx context.Context, sessionID string) (*time.Time, error) {
	var computedAt sql.NullTime
	err := p.db.QueryRowContext(ctx,
		`SELECT computed_at FROM session_card_smart_recap WHERE session_id = $1`,
		sessionID,
	).Scan(&computedAt)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if !computedAt.Valid {
		return nil, nil
	}
	t := computedAt.Time.UTC()
	return &t, nil
}

// PrecomputeSmartRecapOnly computes only the smart recap for a session.
func (p *Precomputer) PrecomputeSmartRecapOnly(ctx context.Context, session StaleSession) error {
	ctx, span := tracer.Start(ctx, "precompute.smart_recap_only",
		trace.WithAttributes(
			attribute.String("session.id", session.SessionID),
			attribute.String("session.provider", session.Provider),
			attribute.Int64("session.user_id", session.UserID),
			attribute.Int64("session.total_lines", session.TotalLines),
		))
	defer span.End()

	sp, err := ProviderFor(session.Provider)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	if !p.config.SmartRecapEnabled {
		span.SetAttributes(attribute.Bool("smart_recap.disabled", true))
		return nil
	}

	rollout, err := sp.Parse(ctx, p.parseInput(session))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	if rollout == nil {
		span.SetAttributes(attribute.Bool("session.empty", true))
		return nil
	}

	cards, err := p.analyticsStore.GetCards(ctx, session.SessionID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	var cardStats map[string]interface{}
	if cards != nil {
		cardStats = cards.ToResponse().Cards
	}

	transcript, idMap, err := sp.PrepareTranscript(ctx, rollout)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	if err := p.precomputeSmartRecap(ctx, session, GenerateInput{
		SessionID:  session.SessionID,
		UserID:     session.UserID,
		LineCount:  session.TotalLines,
		Transcript: transcript,
		IDMap:      idMap,
		CardStats:  cardStats,
	}, sp.ClearMessageIDs()); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetAttributes(attribute.Bool("session.computed", true))
	return nil
}
