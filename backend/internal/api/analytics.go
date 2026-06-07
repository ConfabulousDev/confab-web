package api

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/db"
	dbsession "github.com/ConfabulousDev/confab-web/internal/db/session"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/recapquota"
	"github.com/ConfabulousDev/confab-web/internal/storage"
	"github.com/go-chi/chi/v5"
)

// Smart recap configuration constants
const (
	defaultSmartRecapLockTimeoutSecs = 60
)

// SmartRecapConfig holds configuration for the smart recap feature.
type SmartRecapConfig struct {
	Enabled             bool
	APIKey              string
	Model               string
	QuotaLimit          int
	LockTimeoutSeconds  int
	MaxOutputTokens     int    // 0 means use DefaultMaxOutputTokens
	MaxTranscriptTokens int    // 0 means use DefaultMaxTranscriptTokens
	BaseURL             string // Custom base URL for the Anthropic API (for testing)
}

// loadSmartRecapConfig loads smart recap configuration from environment variables.
// All env vars are required for the feature to be enabled.
func loadSmartRecapConfig() SmartRecapConfig {
	config := SmartRecapConfig{
		Enabled:            os.Getenv("SMART_RECAP_ENABLED") == "true",
		APIKey:             os.Getenv("ANTHROPIC_API_KEY"),
		Model:              os.Getenv("SMART_RECAP_MODEL"),
		LockTimeoutSeconds: defaultSmartRecapLockTimeoutSecs,
	}

	// Parse quota limit: positive integer = cap, 0 or omitted = unlimited
	if quotaStr := os.Getenv("SMART_RECAP_QUOTA_LIMIT"); quotaStr != "" {
		quota, err := strconv.Atoi(quotaStr)
		if err != nil || quota < 0 {
			logger.Fatal("invalid SMART_RECAP_QUOTA_LIMIT", "value", quotaStr)
		}
		config.QuotaLimit = quota
	}

	// Parse max output tokens
	if tokStr := os.Getenv("SMART_RECAP_MAX_OUTPUT_TOKENS"); tokStr != "" {
		if tok, err := strconv.Atoi(tokStr); err == nil && tok > 0 {
			config.MaxOutputTokens = tok
		}
	}

	// Parse max transcript tokens
	if tokStr := os.Getenv("SMART_RECAP_MAX_TRANSCRIPT_TOKENS"); tokStr != "" {
		if tok, err := strconv.Atoi(tokStr); err == nil && tok > 0 {
			config.MaxTranscriptTokens = tok
		}
	}

	// Test-only: override the Anthropic API base URL (for mock servers in integration tests)
	config.BaseURL = os.Getenv("TEST_SMART_RECAP_BASE_URL")

	// Disable if required config is missing (quota=0 means unlimited, not disabled)
	if config.APIKey == "" || config.Model == "" {
		config.Enabled = false
	}

	return config
}

// QuotaEnabled returns true if a per-user quota cap is configured (QuotaLimit > 0).
// When false, usage is still tracked but no cap is enforced.
func (c SmartRecapConfig) QuotaEnabled() bool {
	return c.QuotaLimit > 0
}

// generatorConfig returns the analytics.SmartRecapGeneratorConfig derived from this config.
func (c SmartRecapConfig) generatorConfig() analytics.SmartRecapGeneratorConfig {
	return analytics.SmartRecapGeneratorConfig{
		APIKey:              c.APIKey,
		Model:               c.Model,
		MaxOutputTokens:     c.MaxOutputTokens,
		MaxTranscriptTokens: c.MaxTranscriptTokens,
		BaseURL:             c.BaseURL,
	}
}

// totalTranscriptAndAgentLines sums LastSyncedLine across transcript + agent
// files in a session. Used as the cache-validation line count: when a session
// grows (new lines synced), cached cards become stale at the same count.
func totalTranscriptAndAgentLines(files []db.SyncFileDetail) int64 {
	var total int64
	for _, f := range files {
		if f.FileType == "transcript" || f.FileType == "agent" {
			total += int64(f.LastSyncedLine)
		}
	}
	return total
}

// providerParseInput builds the analytics.ParseInput passed to provider
// methods. Centralized so all dispatch sites populate it identically.
func providerParseInput(database *db.DB, store *storage.S3Storage, sessionID string, sessionUserID int64, sessionProvider, externalID string) analytics.ParseInput {
	return analytics.ParseInput{
		DB:         database.Conn(),
		Store:      store,
		SessionID:  sessionID,
		UserID:     sessionUserID,
		Provider:   sessionProvider,
		ExternalID: externalID,
	}
}

// providerTranscriptForRecap walks the provider through Parse →
// PrepareTranscript and returns the XML + idMap for the smart-recap LLM.
// Returns ("", nil) on any error or empty session.
func providerTranscriptForRecap(ctx context.Context, database *db.DB, store *storage.S3Storage, sessionID string, sessionUserID int64, sessionProvider, externalID string, log *slog.Logger) (string, map[int]string) {
	sp, err := analytics.ProviderFor(sessionProvider)
	if err != nil {
		log.Error("provider lookup failed for smart recap transcript", "error", err, "session_id", sessionID)
		return "", nil
	}
	rollout, err := sp.Parse(ctx, providerParseInput(database, store, sessionID, sessionUserID, sessionProvider, externalID))
	if err != nil {
		log.Error("Failed to parse session for smart recap", "error", err, "session_id", sessionID)
		return "", nil
	}
	if rollout == nil {
		return "", nil
	}
	transcript, idMap, err := sp.PrepareTranscript(ctx, rollout)
	if err != nil {
		log.Error("Failed to prepare transcript for smart recap", "error", err, "session_id", sessionID)
		return "", nil
	}
	return transcript, idMap
}

// providerClearMessageIDs reports whether the smart recap card should clear
// per-item MessageIDs. Unregistered providers default to false (preserves
// historical Claude behavior).
func providerClearMessageIDs(provider string) bool {
	sp, err := analytics.ProviderFor(provider)
	if err != nil {
		return false
	}
	return sp.ClearMessageIDs()
}

// HandleGetSessionAnalytics returns computed analytics for a session.
// Uses the same canonical access model as HandleGetSession (CF-132):
// - Owner access: authenticated user who owns the session
// - Public share: anyone (no auth required)
// - System share: any authenticated user
// - Recipient share: authenticated user who is a share recipient
//
// Analytics are cached in the database and recomputed when stale. CF-403
// unified the dispatch: all provider-specific behavior is reached through
// analytics.ProviderFor + the SessionProvider interface.
func HandleGetSessionAnalytics(database *db.DB, store *storage.S3Storage) http.HandlerFunc {
	analyticsStore := analytics.NewStore(database.Conn())
	sessionStore := &dbsession.Store{DB: database}
	smartRecapConfig := loadSmartRecapConfig()
	smartRecapGenerator := analytics.NewSmartRecapGenerator(analyticsStore, database, smartRecapConfig.generatorConfig())

	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Ctx(r.Context())

		sessionID := chi.URLParam(r, "id")
		if sessionID == "" {
			respondError(w, http.StatusBadRequest, "Invalid session ID")
			return
		}

		dbCtx, dbCancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer dbCancel()

		result := RequireCanonicalRead(dbCtx, w, database, sessionID)
		if result == nil {
			return
		}

		session := result.Session

		// Empty session (no transcript file yet) → empty response.
		totalLineCount := totalTranscriptAndAgentLines(session.Files)
		if totalLineCount == 0 {
			respondJSON(w, http.StatusOK, &analytics.AnalyticsResponse{})
			return
		}

		// Parse optional as_of_line query parameter for conditional requests
		// If client already has analytics up to the current line count, return 304
		if asOfLineStr := r.URL.Query().Get("as_of_line"); asOfLineStr != "" {
			asOfLine, err := strconv.ParseInt(asOfLineStr, 10, 64)
			if err != nil {
				respondError(w, http.StatusBadRequest, "as_of_line must be a valid integer")
				return
			}
			if asOfLine < 0 {
				respondError(w, http.StatusBadRequest, "as_of_line must be non-negative")
				return
			}
			if asOfLine >= totalLineCount {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}

		// Check if we have valid cached cards
		cached, err := analyticsStore.GetCards(dbCtx, sessionID)
		if err != nil {
			log.Error("Failed to get cached cards", "error", err, "session_id", sessionID)
			// Continue to compute fresh analytics
		}

		if cached.AllValid(totalLineCount) {
			// Cache hit - return cached data
			response := cached.ToResponse()

			// Handle smart recap (if enabled) even for cached responses
			if smartRecapConfig.Enabled {
				sessionUserID, externalID, sessionProvider, err := sessionStore.GetSessionOwnerExternalIDAndProvider(dbCtx, sessionID)
				if err == nil {
					attachOrGenerateSmartRecap(r.Context(), &smartRecapContext{
						database:        database,
						analyticsStore:  analyticsStore,
						store:           store,
						config:          smartRecapConfig,
						generator:       smartRecapGenerator,
						sessionID:       sessionID,
						sessionUserID:   sessionUserID,
						sessionProvider: sessionProvider,
						externalID:      externalID,
						lineCount:       totalLineCount,
						cardStats:       response.Cards,
						response:        response,
						log:             log,
						isOwner:         result.AccessInfo.AccessType == db.SessionAccessOwner,
						clearMessageIDs: providerClearMessageIDs(sessionProvider),
					})
				}
			}

			attachSuggestedTitle(database, sessionID, response)
			respondJSON(w, http.StatusOK, response)
			return
		}

		// Cache miss or stale — recompute via the provider registry.
		sessionUserID, externalID, sessionProvider, err := sessionStore.GetSessionOwnerExternalIDAndProvider(dbCtx, sessionID)
		if err != nil {
			log.Error("Failed to get session info", "error", err, "session_id", sessionID)
			respondError(w, http.StatusInternalServerError, "Failed to get session info")
			return
		}

		sp, err := analytics.ProviderFor(sessionProvider)
		if err != nil {
			log.Error("provider lookup failed for analytics", "error", err, "session_id", sessionID, "provider", sessionProvider)
			respondJSON(w, http.StatusOK, &analytics.AnalyticsResponse{})
			return
		}

		rollout, err := sp.Parse(r.Context(), providerParseInput(database, store, sessionID, sessionUserID, sessionProvider, externalID))
		if err != nil {
			log.Error("Failed to parse session for analytics", "error", err, "session_id", sessionID)
			respondJSON(w, http.StatusOK, &analytics.AnalyticsResponse{})
			return
		}
		if rollout == nil {
			respondJSON(w, http.StatusOK, &analytics.AnalyticsResponse{})
			return
		}

		// Enrich the ctx logger so any unknown-model pricing warning emitted deep
		// in the compute path is traceable to this session.
		computeCtx := logger.WithLogger(r.Context(), log.With("session_id", sessionID, "provider", sessionProvider))
		computed := sp.ComputeCards(computeCtx, rollout)
		if computed.ValidationErrorCount > 0 {
			log.Warn("Transcript validation errors detected",
				"session_id", sessionID,
				"validation_error_count", computed.ValidationErrorCount,
			)
		}

		// Convert to Cards and cache
		cards := computed.ToCards(sessionID, totalLineCount)
		if err := analyticsStore.UpsertCards(dbCtx, cards); err != nil {
			log.Error("Failed to cache cards", "error", err, "session_id", sessionID)
		}

		response := cards.ToResponse()
		response.ValidationErrorCount = computed.ValidationErrorCount

		// Smart recap. The rollout is reused for PrepareTranscript so
		// providers with lazy-materialize caches (Claude, Codex) don't
		// re-download agent / subagent files.
		if smartRecapConfig.Enabled {
			transcript, idMap, terr := sp.PrepareTranscript(r.Context(), rollout)
			if terr != nil {
				log.Error("Failed to prepare transcript for smart recap", "error", terr, "session_id", sessionID)
			}
			attachOrGenerateSmartRecap(r.Context(), &smartRecapContext{
				database:        database,
				analyticsStore:  analyticsStore,
				store:           store,
				config:          smartRecapConfig,
				generator:       smartRecapGenerator,
				sessionID:       sessionID,
				sessionUserID:   sessionUserID,
				sessionProvider: sessionProvider,
				externalID:      externalID,
				lineCount:       totalLineCount,
				transcript:      transcript,
				idMap:           idMap,
				cardStats:       response.Cards,
				response:        response,
				log:             log,
				isOwner:         result.AccessInfo.AccessType == db.SessionAccessOwner,
				clearMessageIDs: sp.ClearMessageIDs(),
			})
		}

		attachSuggestedTitle(database, sessionID, response)
		respondJSON(w, http.StatusOK, response)
	}
}

// smartRecapContext groups the parameters needed for smart recap attachment/generation.
type smartRecapContext struct {
	database        *db.DB
	analyticsStore  *analytics.Store
	store           *storage.S3Storage
	config          SmartRecapConfig
	generator       *analytics.SmartRecapGenerator
	sessionID       string
	sessionUserID   int64
	sessionProvider string
	externalID      string
	lineCount       int64
	transcript      string                 // pre-built XML transcript (empty if not yet built)
	idMap           map[int]string         // sequential ID -> UUID map for transcript
	cardStats       map[string]interface{} // computed card data for LLM context
	response        *analytics.AnalyticsResponse
	log             *slog.Logger
	isOwner         bool
	// clearMessageIDs zeroes out the per-item MessageID in the generated recap.
	// Set from SessionProvider.ClearMessageIDs() — providers without stable
	// frontend anchors (Codex) request this so the SmartRecapCard renders
	// items as plain text instead of broken deep-links.
	clearMessageIDs bool
}

// attachOrGenerateSmartRecap adds smart recap to the analytics response.
// - If a cached smart recap exists: return it (regardless of staleness)
// - If no smart recap exists: generate synchronously (first-time only)
// Staleness-based regeneration is handled by background worker and manual regenerate endpoint.
// If transcript is empty, the transcript will be downloaded and built via streaming when generation is needed.
// isOwner controls whether quota info is included in the response (private to owner).
// cardStats contains the computed analytics cards to include in the LLM prompt for context.
// If smart recap generation fails, an error is added to response.CardErrors for graceful degradation.
func attachOrGenerateSmartRecap(ctx context.Context, sc *smartRecapContext) {
	addCardError := func(errMsg string) {
		if sc.response.CardErrors == nil {
			sc.response.CardErrors = make(map[string]string)
		}
		sc.response.CardErrors["smart_recap"] = errMsg
	}

	dbCtx, cancel := context.WithTimeout(ctx, DatabaseTimeout)
	defer cancel()

	smartCard, err := sc.analyticsStore.GetSmartRecapCard(dbCtx, sc.sessionID)
	if err != nil {
		sc.log.Error("Failed to get smart recap card", "error", err, "session_id", sc.sessionID)
		addCardError("Failed to load smart recap")
		return
	}

	// Get quota info — needed for generation decisions, but only expose to owner.
	// Quota is tracked against the session owner, not the viewer.
	// GetOrCreate atomically resets count if month is stale.
	quota, err := recapquota.GetOrCreate(dbCtx, sc.database.Conn(), sc.sessionUserID)
	if err != nil {
		sc.log.Error("Failed to get smart recap quota", "error", err, "user_id", sc.sessionUserID)
	} else if sc.config.QuotaEnabled() && sc.isOwner {
		sc.response.SmartRecapQuota = &analytics.SmartRecapQuotaInfo{
			Used:     quota.ComputeCount,
			Limit:    sc.config.QuotaLimit,
			Exceeded: quota.ComputeCount >= sc.config.QuotaLimit,
		}
	}

	if smartCard.HasValidVersion() {
		addSmartRecapToResponse(sc.response, smartCard)
		return
	}

	// No valid card exists — generate first-time if quota allows.
	if sc.config.QuotaEnabled() && quota != nil && quota.ComputeCount >= sc.config.QuotaLimit {
		if smartCard != nil {
			addSmartRecapToResponse(sc.response, smartCard)
		} else {
			reason := "unavailable"
			if sc.isOwner {
				reason = "quota_exceeded"
			}
			sc.response.SmartRecapMissingReason = &reason
		}
		return
	}

	if smartCard != nil && !smartCard.CanAcquireLock(sc.config.LockTimeoutSeconds) {
		return
	}

	transcript := sc.transcript
	idMap := sc.idMap
	if transcript == "" {
		transcript, idMap = providerTranscriptForRecap(ctx, sc.database, sc.store, sc.sessionID, sc.sessionUserID, sc.sessionProvider, sc.externalID, sc.log)
		if transcript == "" {
			addCardError("Failed to download transcript for smart recap")
			return
		}
	}

	input := analytics.GenerateInput{
		SessionID:  sc.sessionID,
		UserID:     sc.sessionUserID,
		LineCount:  sc.lineCount,
		Transcript: transcript,
		IDMap:      idMap,
		CardStats:  sc.cardStats,
	}
	var genResult *analytics.GenerateResult
	if sc.clearMessageIDs {
		genResult = sc.generator.GenerateWithMessageIDClearing(ctx, input, sc.config.LockTimeoutSeconds, false)
	} else {
		genResult = sc.generator.Generate(ctx, input, sc.config.LockTimeoutSeconds, false)
	}
	if genResult.Error != nil {
		sc.log.Error("Failed to generate smart recap", "error", genResult.Error, "session_id", sc.sessionID)
		addCardError("Failed to generate smart recap")
		return
	}
	if genResult.Skipped {
		return
	}
	addSmartRecapToResponse(sc.response, genResult.Card)
	if genResult.SuggestedTitle != "" {
		sc.response.SuggestedSessionTitle = &genResult.SuggestedTitle
	}
}

// attachSuggestedTitle fetches and attaches the suggested session title to the response.
// Uses context.Background() to ensure the query succeeds even if the request context
// is near its deadline (e.g., after a long smart recap generation).
// Skips the query if the title is already set on the response (e.g., from fresh generation).
func attachSuggestedTitle(database *db.DB, sessionID string, response *analytics.AnalyticsResponse) {
	if response.SuggestedSessionTitle != nil {
		return
	}
	titleCtx, titleCancel := context.WithTimeout(context.Background(), DatabaseTimeout)
	defer titleCancel()
	var suggestedTitle sql.NullString
	if err := database.Conn().QueryRowContext(titleCtx,
		`SELECT suggested_session_title FROM sessions WHERE id = $1`, sessionID,
	).Scan(&suggestedTitle); err == nil && suggestedTitle.Valid {
		response.SuggestedSessionTitle = &suggestedTitle.String
	}
}

// addSmartRecapToResponse adds the smart recap card data to the response.
func addSmartRecapToResponse(response *analytics.AnalyticsResponse, card *analytics.SmartRecapCardRecord) {
	response.Cards["smart_recap"] = analytics.SmartRecapCardData{
		Recap:                     card.Recap,
		WentWell:                  card.WentWell,
		WentBad:                   card.WentBad,
		HumanSuggestions:          card.HumanSuggestions,
		EnvironmentSuggestions:    card.EnvironmentSuggestions,
		DefaultContextSuggestions: card.DefaultContextSuggestions,
		ComputedAt:                card.ComputedAt.Format(time.RFC3339),
		ModelUsed:                 card.ModelUsed,
	}
}

// HandleRegenerateSmartRecap forces regeneration of the smart recap for a session.
// This endpoint is owner-only and bypasses the staleness check.
// Generation is synchronous - the request blocks until the LLM completes.
// Returns 409 Conflict if generation is already in progress (lock held).
func HandleRegenerateSmartRecap(database *db.DB, store *storage.S3Storage) http.HandlerFunc {
	analyticsStore := analytics.NewStore(database.Conn())
	sessionStore := &dbsession.Store{DB: database}
	smartRecapConfig := loadSmartRecapConfig()
	smartRecapGenerator := analytics.NewSmartRecapGenerator(analyticsStore, database, smartRecapConfig.generatorConfig())

	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Ctx(r.Context())

		if !smartRecapConfig.Enabled {
			respondError(w, http.StatusNotFound, "Smart recap not available")
			return
		}

		sessionID := chi.URLParam(r, "id")
		if sessionID == "" {
			respondError(w, http.StatusBadRequest, "Invalid session ID")
			return
		}

		userID, ok := requireUserID(w, r)
		if !ok {
			return
		}

		dbCtx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer cancel()

		sessionUserID, externalID, sessionProvider, err := sessionStore.GetSessionOwnerExternalIDAndProvider(dbCtx, sessionID)
		if err != nil {
			log.Error("Failed to get session", "error", err, "session_id", sessionID)
			respondError(w, http.StatusNotFound, "Session not found")
			return
		}

		if sessionUserID != userID {
			respondError(w, http.StatusForbidden, "Only the session owner can regenerate the recap")
			return
		}

		session, err := sessionStore.GetSessionDetail(dbCtx, sessionID, userID)
		if err != nil {
			log.Error("Failed to get session detail", "error", err, "session_id", sessionID)
			respondError(w, http.StatusInternalServerError, "Failed to get session")
			return
		}

		totalLineCount := totalTranscriptAndAgentLines(session.Files)
		if totalLineCount == 0 {
			respondError(w, http.StatusBadRequest, "No transcript available")
			return
		}

		// Check quota
		quota, err := recapquota.GetOrCreate(dbCtx, database.Conn(), userID)
		if err != nil {
			log.Error("Failed to get quota", "error", err, "user_id", userID)
			respondError(w, http.StatusInternalServerError, "Failed to check quota")
			return
		}

		if smartRecapConfig.QuotaEnabled() && quota.ComputeCount >= smartRecapConfig.QuotaLimit {
			respondError(w, http.StatusForbidden, "Recap generation limit reached")
			return
		}

		smartCard, err := analyticsStore.GetSmartRecapCard(dbCtx, sessionID)
		if err != nil {
			log.Error("Failed to get smart recap card", "error", err, "session_id", sessionID)
		}
		if smartCard != nil && !smartCard.CanAcquireLock(smartRecapConfig.LockTimeoutSeconds) {
			respondError(w, http.StatusConflict, "Generation already in progress")
			return
		}

		cached, err := analyticsStore.GetCards(dbCtx, sessionID)
		if err != nil {
			log.Error("Failed to get cached cards", "error", err, "session_id", sessionID)
		}
		var cardStats map[string]interface{}
		if cached != nil {
			cardStats = cached.ToResponse().Cards
		}

		sp, err := analytics.ProviderFor(sessionProvider)
		if err != nil {
			log.Error("provider lookup failed for smart recap regenerate", "error", err, "session_id", sessionID, "provider", sessionProvider)
			respondError(w, http.StatusInternalServerError, "unsupported provider")
			return
		}

		transcript, idMap := providerTranscriptForRecap(r.Context(), database, store, sessionID, sessionUserID, sessionProvider, externalID, log)
		if transcript == "" {
			respondError(w, http.StatusInternalServerError, "Failed to download transcript")
			return
		}

		input := analytics.GenerateInput{
			SessionID:  sessionID,
			UserID:     sessionUserID,
			LineCount:  totalLineCount,
			Transcript: transcript,
			IDMap:      idMap,
			CardStats:  cardStats,
		}
		var genResult *analytics.GenerateResult
		if sp.ClearMessageIDs() {
			genResult = smartRecapGenerator.GenerateWithMessageIDClearing(r.Context(), input, smartRecapConfig.LockTimeoutSeconds, false)
		} else {
			genResult = smartRecapGenerator.Generate(r.Context(), input, smartRecapConfig.LockTimeoutSeconds, false)
		}
		if genResult.Error != nil {
			log.Error("Failed to generate smart recap", "error", genResult.Error, "session_id", sessionID)
			respondError(w, http.StatusInternalServerError, "Failed to generate smart recap")
			return
		}
		if genResult.Skipped {
			respondError(w, http.StatusConflict, "Generation already in progress")
			return
		}

		response := &analytics.AnalyticsResponse{
			Cards: make(map[string]interface{}),
		}
		if smartRecapConfig.QuotaEnabled() {
			response.SmartRecapQuota = &analytics.SmartRecapQuotaInfo{
				Used:     quota.ComputeCount + 1,
				Limit:    smartRecapConfig.QuotaLimit,
				Exceeded: quota.ComputeCount+1 >= smartRecapConfig.QuotaLimit,
			}
		}
		addSmartRecapToResponse(response, genResult.Card)
		if genResult.SuggestedTitle != "" {
			response.SuggestedSessionTitle = &genResult.SuggestedTitle
		}
		respondJSON(w, http.StatusOK, response)
	}
}
