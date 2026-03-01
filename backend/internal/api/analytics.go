package api

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/recapquota"
	"github.com/ConfabulousDev/confab-web/internal/storage"
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

// classifiedFiles holds the transcript and agent files from a session,
// along with the total line count used for cache validation.
type classifiedFiles struct {
	transcript *db.SyncFileDetail
	agents     []db.SyncFileDetail
	lineCount  int64
}

// classifySessionFiles separates session files into transcript and agent files
// and computes the total line count. Returns nil if no transcript file exists.
func classifySessionFiles(files []db.SyncFileDetail) *classifiedFiles {
	var result classifiedFiles
	for i := range files {
		switch files[i].FileType {
		case "transcript":
			result.transcript = &files[i]
		case "agent":
			result.agents = append(result.agents, files[i])
		}
	}
	if result.transcript == nil {
		return nil
	}
	result.lineCount = int64(result.transcript.LastSyncedLine)
	for _, af := range result.agents {
		result.lineCount += int64(af.LastSyncedLine)
	}
	return &result
}

// downloadAndBuildFileCollection downloads the transcript and agent chunks from storage
// and assembles them into a FileCollection. Returns nil on failure (errors are logged).
func downloadAndBuildFileCollection(
	ctx context.Context,
	store *storage.S3Storage,
	files *classifiedFiles,
	sessionUserID int64,
	externalID string,
	log *slog.Logger,
) *analytics.FileCollection {
	storageCtx, storageCancel := context.WithTimeout(ctx, StorageTimeout)
	defer storageCancel()

	mainContent, err := store.DownloadAndMergeChunks(storageCtx, sessionUserID, externalID, files.transcript.FileName)
	if err != nil || mainContent == nil {
		log.Error("Failed to download transcript", "error", err)
		return nil
	}

	agentContents := make(map[string][]byte)
	for _, af := range files.agents {
		agentID := analytics.ExtractAgentID(af.FileName)
		if agentID == "" {
			continue
		}
		content, err := store.DownloadAndMergeChunks(storageCtx, sessionUserID, externalID, af.FileName)
		if err != nil {
			log.Warn("Failed to download agent file", "error", err, "file", af.FileName)
			continue
		}
		if content != nil {
			agentContents[agentID] = content
		}
	}

	fc, err := analytics.NewFileCollectionWithAgents(mainContent, agentContents)
	if err != nil {
		log.Error("Failed to parse transcript", "error", err)
		return nil
	}
	return fc
}

// HandleGetSessionAnalytics returns computed analytics for a session.
// Uses the same canonical access model as HandleGetSession (CF-132):
// - Owner access: authenticated user who owns the session
// - Public share: anyone (no auth required)
// - System share: any authenticated user
// - Recipient share: authenticated user who is a share recipient
//
// Analytics are cached in the database and recomputed when stale.
func HandleGetSessionAnalytics(database *db.DB, store *storage.S3Storage) http.HandlerFunc {
	analyticsStore := analytics.NewStore(database.Conn())
	smartRecapConfig := loadSmartRecapConfig()
	smartRecapGenerator := analytics.NewSmartRecapGenerator(analyticsStore, database.Conn(), smartRecapConfig.generatorConfig())

	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Ctx(r.Context())

		// Get session ID from URL (UUID)
		sessionID := chi.URLParam(r, "id")
		if sessionID == "" {
			respondError(w, http.StatusBadRequest, "Invalid session ID")
			return
		}

		// Create context with timeout for database operation
		dbCtx, dbCancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer dbCancel()

		// Check canonical access (CF-132 unified access model)
		result, err := CheckCanonicalAccess(dbCtx, database, sessionID)
		if RespondCanonicalAccessError(dbCtx, w, err, sessionID) {
			return
		}

		// Handle no access - check AuthMayHelp to decide 401 vs 404
		if result.AccessInfo.AccessType == db.SessionAccessNone {
			if result.AccessInfo.AuthMayHelp {
				respondError(w, http.StatusUnauthorized, "Sign in to view this session")
				return
			}
			respondError(w, http.StatusNotFound, "Session not found")
			return
		}

		session := result.Session

		// Classify session files and compute total line count
		files := classifySessionFiles(session.Files)
		if files == nil {
			respondJSON(w, http.StatusOK, &analytics.AnalyticsResponse{})
			return
		}
		totalLineCount := files.lineCount

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
			// Client already has analytics up to or past current line count - no new data
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
				// Get session owner ID for quota lookup
				sessionUserID, externalID, err := database.GetSessionOwnerAndExternalID(dbCtx, sessionID)
				if err == nil {
					attachOrGenerateSmartRecap(r.Context(), &smartRecapContext{
						database:       database,
						analyticsStore: analyticsStore,
						store:          store,
						config:         smartRecapConfig,
						generator:      smartRecapGenerator,
						sessionID:      sessionID,
						sessionUserID:  sessionUserID,
						externalID:     externalID,
						lineCount:      totalLineCount,
						cardStats:      response.Cards,
						response:       response,
						log:            log,
						isOwner:        result.AccessInfo.AccessType == db.SessionAccessOwner,
					})
				}
			}

			// Include suggested session title if available
			attachSuggestedTitle(database, sessionID, response)

			respondJSON(w, http.StatusOK, response)
			return
		}

		// Cache miss or stale - need to recompute
		// Get the session's user_id and external_id for S3 path
		sessionUserID, externalID, err := database.GetSessionOwnerAndExternalID(dbCtx, sessionID)
		if err != nil {
			log.Error("Failed to get session info", "error", err, "session_id", sessionID)
			respondError(w, http.StatusInternalServerError, "Failed to get session info")
			return
		}

		// Download transcript and agent files
		fc := downloadAndBuildFileCollection(r.Context(), store, files, sessionUserID, externalID, log)
		if fc == nil {
			respondJSON(w, http.StatusOK, &analytics.AnalyticsResponse{})
			return
		}

		// Compute analytics from FileCollection
		storageCtx, storageCancel := context.WithTimeout(r.Context(), StorageTimeout)
		defer storageCancel()
		computed, err := analytics.ComputeFromFileCollection(storageCtx, fc)
		if err != nil {
			log.Error("Failed to compute analytics", "error", err, "session_id", sessionID)
			respondError(w, http.StatusInternalServerError, "Failed to compute analytics")
			return
		}

		// Log validation errors if any
		if computed.ValidationErrorCount > 0 {
			log.Warn("Transcript validation errors detected",
				"session_id", sessionID,
				"validation_error_count", computed.ValidationErrorCount,
			)
		}

		// Convert to Cards and cache
		cards := computed.ToCards(sessionID, totalLineCount)

		// Store in cache (errors logged but not returned - we can still return computed result)
		if err := analyticsStore.UpsertCards(dbCtx, cards); err != nil {
			log.Error("Failed to cache cards", "error", err, "session_id", sessionID)
		}

		// Build response with validation error count
		response := cards.ToResponse()
		response.ValidationErrorCount = computed.ValidationErrorCount

		// Handle smart recap (if enabled)
		if smartRecapConfig.Enabled {
			attachOrGenerateSmartRecap(r.Context(), &smartRecapContext{
				database:       database,
				analyticsStore: analyticsStore,
				store:          store,
				config:         smartRecapConfig,
				generator:      smartRecapGenerator,
				sessionID:      sessionID,
				sessionUserID:  sessionUserID,
				externalID:     externalID,
				lineCount:      totalLineCount,
				fc:             fc,
				cardStats:      response.Cards,
				response:       response,
				log:            log,
				isOwner:        result.AccessInfo.AccessType == db.SessionAccessOwner,
			})
		}

		// Include suggested session title if available
		attachSuggestedTitle(database, sessionID, response)

		respondJSON(w, http.StatusOK, response)
	}
}

// smartRecapContext groups the parameters needed for smart recap attachment/generation.
type smartRecapContext struct {
	database       *db.DB
	analyticsStore *analytics.Store
	store          *storage.S3Storage
	config         SmartRecapConfig
	generator      *analytics.SmartRecapGenerator
	sessionID      string
	sessionUserID  int64
	externalID     string
	lineCount      int64
	fc             *analytics.FileCollection      // nil if transcript not yet downloaded
	cardStats      map[string]interface{}          // computed card data for LLM context
	response       *analytics.AnalyticsResponse
	log            *slog.Logger
	isOwner        bool
}

// attachOrGenerateSmartRecap adds smart recap to the analytics response.
// - If a cached smart recap exists: return it (regardless of staleness)
// - If no smart recap exists: generate synchronously (first-time only)
// Staleness-based regeneration is handled by background worker and manual regenerate endpoint.
// If fc is nil, the transcript will be downloaded synchronously when generation is needed.
// isOwner controls whether quota info is included in the response (private to owner).
// cardStats contains the computed analytics cards to include in the LLM prompt for context.
// If smart recap generation fails, an error is added to response.CardErrors for graceful degradation.
func attachOrGenerateSmartRecap(ctx context.Context, sc *smartRecapContext) {
	// Helper to add smart recap error to response for graceful degradation
	addCardError := func(errMsg string) {
		if sc.response.CardErrors == nil {
			sc.response.CardErrors = make(map[string]string)
		}
		sc.response.CardErrors["smart_recap"] = errMsg
	}

	dbCtx, cancel := context.WithTimeout(ctx, DatabaseTimeout)
	defer cancel()

	// Get current smart recap card
	smartCard, err := sc.analyticsStore.GetSmartRecapCard(dbCtx, sc.sessionID)
	if err != nil {
		sc.log.Error("Failed to get smart recap card", "error", err, "session_id", sc.sessionID)
		addCardError("Failed to load smart recap")
		return
	}

	// Get quota info - needed for generation decisions, but only expose to owner.
	// Quota is tracked against the session owner, not the viewer.
	// GetOrCreate atomically resets count if month is stale.
	quota, err := recapquota.GetOrCreate(dbCtx, sc.database.Conn(), sc.sessionUserID)
	if err != nil {
		sc.log.Error("Failed to get smart recap quota", "error", err, "user_id", sc.sessionUserID)
	} else if sc.config.QuotaEnabled() && sc.isOwner {
		// Only include quota in response when capped and viewer is the session owner
		sc.response.SmartRecapQuota = &analytics.SmartRecapQuotaInfo{
			Used:     quota.ComputeCount,
			Limit:    sc.config.QuotaLimit,
			Exceeded: quota.ComputeCount >= sc.config.QuotaLimit,
		}
	}

	// If we have a cached card with valid version, return it (no regeneration, worker handles updates)
	if smartCard.HasValidVersion() {
		addSmartRecapToResponse(sc.response, smartCard)
		return
	}

	// No valid card exists - generate first-time if quota allows

	// Check quota (skip when unlimited)
	if sc.config.QuotaEnabled() && quota != nil && quota.ComputeCount >= sc.config.QuotaLimit {
		// Quota exceeded - return whatever cached data we have (even if invalid version)
		if smartCard != nil {
			addSmartRecapToResponse(sc.response, smartCard)
		} else {
			// No card data at all -- tell the frontend why
			reason := "unavailable"
			if sc.isOwner {
				reason = "quota_exceeded"
			}
			sc.response.SmartRecapMissingReason = &reason
		}
		return
	}

	// Check if another process is already generating (lock held)
	if smartCard != nil && !smartCard.CanAcquireLock(sc.config.LockTimeoutSeconds) {
		// Lock held by another request - graceful degradation, skip smart recap
		return
	}

	// Download transcript if not already available
	transcriptFC := sc.fc
	if transcriptFC == nil {
		transcriptFC = downloadTranscriptForSmartRecap(ctx, sc.database, sc.store, sc.sessionID, sc.sessionUserID, sc.externalID, sc.log)
		if transcriptFC == nil {
			addCardError("Failed to download transcript for smart recap")
			return
		}
	}

	// Generate synchronously (first-time generation)
	genResult := sc.generator.Generate(ctx, analytics.GenerateInput{
		SessionID:      sc.sessionID,
		UserID:         sc.sessionUserID,
		LineCount:      sc.lineCount,
		FileCollection: transcriptFC,
		CardStats:      sc.cardStats,
	}, sc.config.LockTimeoutSeconds)
	if genResult.Error != nil {
		sc.log.Error("Failed to generate smart recap", "error", genResult.Error, "session_id", sc.sessionID)
		addCardError("Failed to generate smart recap")
		return
	}
	if genResult.Skipped {
		// Lock held by another request - graceful degradation
		return
	}
	addSmartRecapToResponse(sc.response, genResult.Card)
	// Set the title directly from the LLM result to avoid a separate DB round-trip.
	// This is more reliable than re-querying via attachSuggestedTitle, which can fail
	// if the request context is near its deadline after a long LLM generation.
	if genResult.SuggestedTitle != "" {
		sc.response.SuggestedSessionTitle = &genResult.SuggestedTitle
	}
}

// attachSuggestedTitle fetches and attaches the suggested session title to the response.
// Uses context.Background() to ensure the query succeeds even if the request context
// is near its deadline (e.g., after a long smart recap generation).
// Skips the query if the title is already set on the response (e.g., from fresh generation).
func attachSuggestedTitle(database *db.DB, sessionID string, response *analytics.AnalyticsResponse) {
	// Skip if title was already set directly (e.g., from freshly generated smart recap)
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
	smartRecapConfig := loadSmartRecapConfig()
	smartRecapGenerator := analytics.NewSmartRecapGenerator(analyticsStore, database.Conn(), smartRecapConfig.generatorConfig())

	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Ctx(r.Context())

		// Feature must be enabled
		if !smartRecapConfig.Enabled {
			respondError(w, http.StatusNotFound, "Smart recap not available")
			return
		}

		// Get session ID from URL
		sessionID := chi.URLParam(r, "id")
		if sessionID == "" {
			respondError(w, http.StatusBadRequest, "Invalid session ID")
			return
		}

		// Get authenticated user (RequireSession middleware ensures this exists)
		userID, ok := auth.GetUserID(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		dbCtx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer cancel()

		// Get session and verify ownership
		sessionUserID, externalID, err := database.GetSessionOwnerAndExternalID(dbCtx, sessionID)
		if err != nil {
			log.Error("Failed to get session", "error", err, "session_id", sessionID)
			respondError(w, http.StatusNotFound, "Session not found")
			return
		}

		if sessionUserID != userID {
			respondError(w, http.StatusForbidden, "Only the session owner can regenerate the recap")
			return
		}

		// Get session files and compute total line count
		session, err := database.GetSessionDetail(dbCtx, sessionID, userID)
		if err != nil {
			log.Error("Failed to get session detail", "error", err, "session_id", sessionID)
			respondError(w, http.StatusInternalServerError, "Failed to get session")
			return
		}

		files := classifySessionFiles(session.Files)
		if files == nil {
			respondError(w, http.StatusBadRequest, "No transcript available")
			return
		}
		totalLineCount := files.lineCount

		// Check quota (GetOrCreate atomically resets count if month is stale)
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

		// Check if generation is already in progress (lock check) - return 409 Conflict
		smartCard, err := analyticsStore.GetSmartRecapCard(dbCtx, sessionID)
		if err != nil {
			log.Error("Failed to get smart recap card", "error", err, "session_id", sessionID)
		}
		if smartCard != nil && !smartCard.CanAcquireLock(smartRecapConfig.LockTimeoutSeconds) {
			respondError(w, http.StatusConflict, "Generation already in progress")
			return
		}

		// Get cached cards for stats context
		cached, err := analyticsStore.GetCards(dbCtx, sessionID)
		if err != nil {
			log.Error("Failed to get cached cards", "error", err, "session_id", sessionID)
		}
		var cardStats map[string]interface{}
		if cached != nil {
			cardStats = cached.ToResponse().Cards
		}

		// Download transcript synchronously
		fc := downloadTranscriptForSmartRecap(r.Context(), database, store, sessionID, sessionUserID, externalID, log)
		if fc == nil {
			respondError(w, http.StatusInternalServerError, "Failed to download transcript")
			return
		}

		// Generate synchronously (this acquires the lock internally)
		genResult := smartRecapGenerator.Generate(r.Context(), analytics.GenerateInput{
			SessionID:      sessionID,
			UserID:         sessionUserID,
			LineCount:      totalLineCount,
			FileCollection: fc,
			CardStats:      cardStats,
		}, smartRecapConfig.LockTimeoutSeconds)
		if genResult.Error != nil {
			log.Error("Failed to generate smart recap", "error", genResult.Error, "session_id", sessionID)
			respondError(w, http.StatusInternalServerError, "Failed to generate smart recap")
			return
		}
		if genResult.Skipped {
			respondError(w, http.StatusConflict, "Generation already in progress")
			return
		}

		// Return the generated card using the shared helper
		response := &analytics.AnalyticsResponse{
			Cards: make(map[string]interface{}),
		}
		if smartRecapConfig.QuotaEnabled() {
			response.SmartRecapQuota = &analytics.SmartRecapQuotaInfo{
				Used:     quota.ComputeCount + 1, // Increment since we just generated
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

// downloadTranscriptForSmartRecap downloads the transcript files and creates a FileCollection.
func downloadTranscriptForSmartRecap(
	ctx context.Context,
	database *db.DB,
	store *storage.S3Storage,
	sessionID string,
	sessionUserID int64,
	externalID string,
	log *slog.Logger,
) *analytics.FileCollection {
	dbCtx, cancel := context.WithTimeout(ctx, DatabaseTimeout)
	defer cancel()

	session, err := database.GetSessionDetail(dbCtx, sessionID, sessionUserID)
	if err != nil {
		log.Error("Failed to get session for smart recap", "error", err, "session_id", sessionID)
		return nil
	}

	files := classifySessionFiles(session.Files)
	if files == nil {
		return nil
	}

	return downloadAndBuildFileCollection(ctx, store, files, sessionUserID, externalID, log)
}
