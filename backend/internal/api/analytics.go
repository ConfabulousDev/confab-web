package api

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/anthropic"
	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/storage"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var analyticsTracer = otel.Tracer("confab/api/analytics")

// Smart recap configuration constants
const (
	defaultSmartRecapLockTimeoutSecs = 60
)

// SmartRecapConfig holds configuration for the smart recap feature.
type SmartRecapConfig struct {
	Enabled            bool
	APIKey             string
	Model              string
	QuotaLimit         int
	StalenessMinutes   int
	LockTimeoutSeconds int
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

	// Parse quota limit (required)
	if quotaStr := os.Getenv("SMART_RECAP_QUOTA_LIMIT"); quotaStr != "" {
		if quota, err := strconv.Atoi(quotaStr); err == nil && quota > 0 {
			config.QuotaLimit = quota
		}
	}

	// Parse staleness minutes (required)
	if stalenessStr := os.Getenv("SMART_RECAP_STALENESS_MINUTES"); stalenessStr != "" {
		if staleness, err := strconv.Atoi(stalenessStr); err == nil && staleness > 0 {
			config.StalenessMinutes = staleness
		}
	}

	// Disable if any required config is missing
	if config.APIKey == "" || config.Model == "" || config.QuotaLimit == 0 || config.StalenessMinutes == 0 {
		config.Enabled = false
	}

	return config
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

		// Collect transcript and agent files
		var mainFile *db.SyncFileDetail
		var agentFiles []db.SyncFileDetail
		for i := range session.Files {
			switch session.Files[i].FileType {
			case "transcript":
				mainFile = &session.Files[i]
			case "agent":
				agentFiles = append(agentFiles, session.Files[i])
			}
		}
		if mainFile == nil {
			// No transcript file - return empty analytics
			respondJSON(w, http.StatusOK, &analytics.AnalyticsResponse{})
			return
		}

		// Current state for cache validation (sum of all file line counts)
		totalLineCount := int64(mainFile.LastSyncedLine)
		for _, af := range agentFiles {
			totalLineCount += int64(af.LastSyncedLine)
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
					isOwner := result.AccessInfo.AccessType == db.SessionAccessOwner
					handleSmartRecap(
						r.Context(),
						database,
						analyticsStore,
						store,
						smartRecapConfig,
						sessionID,
						sessionUserID,
						externalID,
						totalLineCount,
						nil, // transcript not downloaded yet
						response.Cards, // pass cached card stats for LLM context
						response,
						log,
						isOwner,
					)
				}
			}

			// Include suggested session title if available
			var suggestedTitle sql.NullString
			titleQuery := `SELECT suggested_session_title FROM sessions WHERE id = $1`
			if err := database.Conn().QueryRowContext(dbCtx, titleQuery, sessionID).Scan(&suggestedTitle); err == nil && suggestedTitle.Valid {
				response.SuggestedSessionTitle = &suggestedTitle.String
			}

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

		storageCtx, storageCancel := context.WithTimeout(r.Context(), StorageTimeout)
		defer storageCancel()

		// Download main transcript
		mainContent, err := downloadAndMergeFile(storageCtx, store, sessionUserID, externalID, mainFile.FileName)
		if err != nil {
			respondStorageError(w, err, "Failed to download transcript")
			return
		}
		if mainContent == nil {
			// No chunks - return empty analytics
			respondJSON(w, http.StatusOK, &analytics.AnalyticsResponse{})
			return
		}

		// Download agent files
		agentContents := make(map[string][]byte)
		for _, af := range agentFiles {
			agentID := extractAgentID(af.FileName)
			if agentID == "" {
				continue
			}
			content, err := downloadAndMergeFile(storageCtx, store, sessionUserID, externalID, af.FileName)
			if err != nil {
				// Log but continue - graceful degradation
				log.Warn("Failed to download agent file", "error", err, "file", af.FileName)
				continue
			}
			if content != nil {
				agentContents[agentID] = content
			}
		}

		// Build FileCollection with agents
		fc, err := analytics.NewFileCollectionWithAgents(mainContent, agentContents)
		if err != nil {
			log.Error("Failed to parse transcript", "error", err, "session_id", sessionID)
			respondError(w, http.StatusInternalServerError, "Failed to process session data")
			return
		}

		// Compute analytics from FileCollection
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
			isOwner := result.AccessInfo.AccessType == db.SessionAccessOwner
			handleSmartRecap(
				r.Context(),
				database,
				analyticsStore,
				store,
				smartRecapConfig,
				sessionID,
				sessionUserID,
				externalID,
				totalLineCount,
				fc,
				response.Cards, // pass computed card stats for LLM context
				response,
				log,
				isOwner,
			)
		}

		// Include suggested session title if available
		var suggestedTitle sql.NullString
		titleQuery := `SELECT suggested_session_title FROM sessions WHERE id = $1`
		if err := database.Conn().QueryRowContext(dbCtx, titleQuery, sessionID).Scan(&suggestedTitle); err == nil && suggestedTitle.Valid {
			response.SuggestedSessionTitle = &suggestedTitle.String
		}

		respondJSON(w, http.StatusOK, response)
	}
}

// handleSmartRecap handles smart recap computation for the analytics response.
// Any viewer (owner, shared, or public) can trigger generation - quota is charged to session owner.
// Generation is synchronous - the request blocks until the LLM completes (~60-90s for initial generation).
// If fc is nil, the transcript will be downloaded synchronously when generation is needed.
// isOwner controls whether quota info is included in the response (private to owner).
// cardStats contains the computed analytics cards to include in the LLM prompt for context.
func handleSmartRecap(
	ctx context.Context,
	database *db.DB,
	analyticsStore *analytics.Store,
	store *storage.S3Storage,
	config SmartRecapConfig,
	sessionID string,
	sessionUserID int64,
	externalID string,
	lineCount int64,
	fc *analytics.FileCollection, // nil if transcript not yet downloaded
	cardStats map[string]interface{}, // computed card data for LLM context
	response *analytics.AnalyticsResponse,
	log *slog.Logger,
	isOwner bool,
) {
	dbCtx, cancel := context.WithTimeout(ctx, DatabaseTimeout)
	defer cancel()

	// Get current smart recap card
	smartCard, err := analyticsStore.GetSmartRecapCard(dbCtx, sessionID)
	if err != nil {
		log.Error("Failed to get smart recap card", "error", err, "session_id", sessionID)
		return
	}

	// Get quota info - needed for generation decisions, but only expose to owner
	// Quota is tracked against the session owner, not the viewer
	// Reset quota if needed (start of new month)
	_, _ = database.ResetSmartRecapQuotaIfNeeded(dbCtx, sessionUserID)

	quota, err := database.GetOrCreateSmartRecapQuota(dbCtx, sessionUserID)
	var quotaInfo *analytics.SmartRecapQuotaInfo
	if err != nil {
		log.Error("Failed to get smart recap quota", "error", err, "user_id", sessionUserID)
	} else {
		quotaInfo = &analytics.SmartRecapQuotaInfo{
			Used:     quota.ComputeCount,
			Limit:    config.QuotaLimit,
			Exceeded: quota.ComputeCount >= config.QuotaLimit,
		}
		// Only include quota in response for session owner (private info)
		if isOwner {
			response.SmartRecapQuota = quotaInfo
		}
	}

	// Helper to generate synchronously, downloading transcript if needed
	// Returns the generated card or nil if generation failed/skipped
	generateSync := func() *analytics.SmartRecapCardRecord {
		// Download transcript if not already available
		transcriptFC := fc
		if transcriptFC == nil {
			transcriptFC = downloadTranscriptForSmartRecap(ctx, database, store, sessionID, sessionUserID, externalID, log)
			if transcriptFC == nil {
				return nil
			}
		}
		return generateSmartRecapSync(ctx, database, analyticsStore, config, sessionID, sessionUserID, lineCount, transcriptFC, cardStats, log)
	}

	// If we have a valid cached card, return it
	if smartCard != nil && smartCard.IsValid() {
		isStale := smartCard.IsStale(lineCount, config.StalenessMinutes)

		// Check if we should regenerate (stale + quota OK + can acquire lock)
		if isStale && quotaInfo != nil && !quotaInfo.Exceeded && smartCard.CanAcquireLock(config.LockTimeoutSeconds) {
			// Try to generate synchronously
			if newCard := generateSync(); newCard != nil {
				addSmartRecapToResponse(response, newCard, false)
				return
			}
			// Generation failed, fall through to return cached stale data
		}

		addSmartRecapToResponse(response, smartCard, isStale)
		return
	}

	// No valid card exists - any viewer can trigger generation
	// Quota is charged to session owner

	// Check quota
	if quotaInfo != nil && quotaInfo.Exceeded {
		// Quota exceeded - return whatever cached data we have (even if stale)
		if smartCard != nil {
			addSmartRecapToResponse(response, smartCard, true)
		}
		return
	}

	// Check if another process is already generating (lock held)
	if smartCard != nil && !smartCard.CanAcquireLock(config.LockTimeoutSeconds) {
		// Lock held by another request - return stale data if available, otherwise nothing
		// (graceful degradation - don't fail, just skip smart recap)
		return
	}

	// Generate synchronously
	if newCard := generateSync(); newCard != nil {
		addSmartRecapToResponse(response, newCard, false)
	}
}

// addSmartRecapToResponse adds the smart recap card data to the response.
func addSmartRecapToResponse(response *analytics.AnalyticsResponse, card *analytics.SmartRecapCardRecord, isStale bool) {
	response.Cards["smart_recap"] = analytics.SmartRecapCardData{
		Recap:                     card.Recap,
		WentWell:                  card.WentWell,
		WentBad:                   card.WentBad,
		HumanSuggestions:          card.HumanSuggestions,
		EnvironmentSuggestions:    card.EnvironmentSuggestions,
		DefaultContextSuggestions: card.DefaultContextSuggestions,
		ComputedAt:                card.ComputedAt.Format(time.RFC3339),
		IsStale:                   isStale,
		ModelUsed:                 card.ModelUsed,
	}
}

// generateSmartRecapSync generates the smart recap synchronously using the LLM and saves it.
// Returns the generated card on success, or nil on failure.
// The lock is acquired at the start and released (via upsert) on success or cleared on failure.
// cardStats contains the computed analytics cards to include in the LLM prompt.
func generateSmartRecapSync(
	ctx context.Context,
	database *db.DB,
	analyticsStore *analytics.Store,
	config SmartRecapConfig,
	sessionID string,
	sessionUserID int64,
	lineCount int64,
	fc *analytics.FileCollection,
	cardStats map[string]interface{},
	log *slog.Logger,
) *analytics.SmartRecapCardRecord {
	// Try to acquire the lock
	dbCtx, cancel := context.WithTimeout(ctx, DatabaseTimeout)
	acquired, err := analyticsStore.AcquireSmartRecapLock(dbCtx, sessionID, config.LockTimeoutSeconds)
	cancel()
	if err != nil {
		log.Error("Failed to acquire smart recap lock", "error", err, "session_id", sessionID)
		return nil
	}
	if !acquired {
		// Lock held by another request
		return nil
	}

	// Start tracing span for generation
	ctx, span := analyticsTracer.Start(ctx, "api.smart_recap.generate",
		trace.WithAttributes(
			attribute.String("session.id", sessionID),
			attribute.Int64("session.line_count", lineCount),
			attribute.String("llm.model", config.Model),
		))
	defer span.End()

	// Create Anthropic client
	client := anthropic.NewClient(config.APIKey)
	analyzer := analytics.NewSmartRecapAnalyzer(client, config.Model)

	// Generate the recap (with timeout)
	genCtx, genCancel := context.WithTimeout(ctx, 30*time.Second)
	defer genCancel()

	result, err := analyzer.Analyze(genCtx, fc, cardStats)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.Error("Failed to generate smart recap", "error", err, "session_id", sessionID)
		// Clear the lock so another request can try
		// Use Background context to ensure cleanup happens even if request was canceled
		clearCtx, clearCancel := context.WithTimeout(context.Background(), DatabaseTimeout)
		defer clearCancel()
		_ = analyticsStore.ClearSmartRecapLock(clearCtx, sessionID)
		return nil
	}

	// Record token usage on the span
	span.SetAttributes(
		attribute.Int("llm.tokens.input", result.InputTokens),
		attribute.Int("llm.tokens.output", result.OutputTokens),
		attribute.Int("generation.time_ms", result.GenerationTimeMs),
	)

	// Save the result (this also clears the lock via upsert)
	// Use Background context to ensure save completes even if request was canceled
	saveCtx, saveCancel := context.WithTimeout(context.Background(), DatabaseTimeout)
	defer saveCancel()

	card := &analytics.SmartRecapCardRecord{
		SessionID:                 sessionID,
		Version:                   analytics.SmartRecapCardVersion,
		ComputedAt:                time.Now().UTC(),
		UpToLine:                  lineCount,
		Recap:                     result.Recap,
		WentWell:                  result.WentWell,
		WentBad:                   result.WentBad,
		HumanSuggestions:          result.HumanSuggestions,
		EnvironmentSuggestions:    result.EnvironmentSuggestions,
		DefaultContextSuggestions: result.DefaultContextSuggestions,
		ModelUsed:                 config.Model,
		InputTokens:               result.InputTokens,
		OutputTokens:              result.OutputTokens,
		GenerationTimeMs:          &result.GenerationTimeMs,
	}

	if err := analyticsStore.UpsertSmartRecapCard(saveCtx, card); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.Error("Failed to save smart recap card", "error", err, "session_id", sessionID)
		// Clear the lock so another request can try
		_ = analyticsStore.ClearSmartRecapLock(saveCtx, sessionID)
		return nil
	}

	// Update session with suggested title
	if result.SuggestedSessionTitle != "" {
		if err := database.UpdateSessionSuggestedTitle(saveCtx, sessionID, result.SuggestedSessionTitle); err != nil {
			// Log but don't fail - the main operation succeeded
			log.Error("Failed to update suggested title", "error", err, "session_id", sessionID)
		}
	}

	// Increment quota
	if err := database.IncrementSmartRecapQuota(saveCtx, sessionUserID); err != nil {
		span.RecordError(err)
		// Don't set error status for quota increment failure - the main operation succeeded
		log.Error("Failed to increment smart recap quota", "error", err, "user_id", sessionUserID)
	}

	log.Info("Smart recap generated",
		"session_id", sessionID,
		"input_tokens", result.InputTokens,
		"output_tokens", result.OutputTokens,
		"generation_time_ms", result.GenerationTimeMs,
	)

	return card
}

// HandleRegenerateSmartRecap forces regeneration of the smart recap for a session.
// This endpoint is owner-only and bypasses the staleness check.
// Generation is synchronous - the request blocks until the LLM completes.
// Returns 409 Conflict if generation is already in progress (lock held).
func HandleRegenerateSmartRecap(database *db.DB, store *storage.S3Storage) http.HandlerFunc {
	analyticsStore := analytics.NewStore(database.Conn())
	smartRecapConfig := loadSmartRecapConfig()

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

		// Get session for line count
		session, err := database.GetSessionDetail(dbCtx, sessionID, userID)
		if err != nil {
			log.Error("Failed to get session detail", "error", err, "session_id", sessionID)
			respondError(w, http.StatusInternalServerError, "Failed to get session")
			return
		}

		// Calculate total line count
		var totalLineCount int64
		for _, file := range session.Files {
			if file.FileType == "transcript" || file.FileType == "agent" {
				totalLineCount += int64(file.LastSyncedLine)
			}
		}

		// Check quota
		_, _ = database.ResetSmartRecapQuotaIfNeeded(dbCtx, userID)
		quota, err := database.GetOrCreateSmartRecapQuota(dbCtx, userID)
		if err != nil {
			log.Error("Failed to get quota", "error", err, "user_id", userID)
			respondError(w, http.StatusInternalServerError, "Failed to check quota")
			return
		}

		if quota.ComputeCount >= smartRecapConfig.QuotaLimit {
			respondError(w, http.StatusForbidden, "Monthly recap limit reached")
			return
		}

		// Check if generation is already in progress (lock check) - return 409 Conflict
		smartCard, _ := analyticsStore.GetSmartRecapCard(dbCtx, sessionID)
		if smartCard != nil && !smartCard.CanAcquireLock(smartRecapConfig.LockTimeoutSeconds) {
			respondError(w, http.StatusConflict, "Generation already in progress")
			return
		}

		// Get cached cards for stats context
		cached, _ := analyticsStore.GetCards(dbCtx, sessionID)
		cardStats := cached.ToResponse().Cards

		// Download transcript synchronously
		fc := downloadTranscriptForSmartRecap(r.Context(), database, store, sessionID, sessionUserID, externalID, log)
		if fc == nil {
			respondError(w, http.StatusInternalServerError, "Failed to download transcript")
			return
		}

		// Generate synchronously (this acquires the lock internally)
		newCard := generateSmartRecapSync(r.Context(), database, analyticsStore, smartRecapConfig, sessionID, sessionUserID, totalLineCount, fc, cardStats, log)
		if newCard == nil {
			// Could be lock conflict (race) or generation failure
			// Check if it's a lock conflict
			smartCard, _ = analyticsStore.GetSmartRecapCard(dbCtx, sessionID)
			if smartCard != nil && !smartCard.CanAcquireLock(smartRecapConfig.LockTimeoutSeconds) {
				respondError(w, http.StatusConflict, "Generation already in progress")
				return
			}
			respondError(w, http.StatusInternalServerError, "Failed to generate smart recap")
			return
		}

		// Return the generated card
		response := &analytics.AnalyticsResponse{
			Cards: map[string]interface{}{
				"smart_recap": analytics.SmartRecapCardData{
					Recap:                     newCard.Recap,
					WentWell:                  newCard.WentWell,
					WentBad:                   newCard.WentBad,
					HumanSuggestions:          newCard.HumanSuggestions,
					EnvironmentSuggestions:    newCard.EnvironmentSuggestions,
					DefaultContextSuggestions: newCard.DefaultContextSuggestions,
					ComputedAt:                newCard.ComputedAt.Format(time.RFC3339),
					IsStale:                   false,
					ModelUsed:                 newCard.ModelUsed,
				},
			},
			SmartRecapQuota: &analytics.SmartRecapQuotaInfo{
				Used:     quota.ComputeCount + 1, // Increment since we just generated
				Limit:    smartRecapConfig.QuotaLimit,
				Exceeded: quota.ComputeCount+1 >= smartRecapConfig.QuotaLimit,
			},
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

	// Get session files
	session, err := database.GetSessionDetail(dbCtx, sessionID, sessionUserID)
	if err != nil {
		log.Error("Failed to get session for smart recap", "error", err, "session_id", sessionID)
		return nil
	}

	// Find transcript and agent files
	var mainFile *db.SyncFileDetail
	var agentFiles []db.SyncFileDetail
	for i := range session.Files {
		switch session.Files[i].FileType {
		case "transcript":
			mainFile = &session.Files[i]
		case "agent":
			agentFiles = append(agentFiles, session.Files[i])
		}
	}

	if mainFile == nil {
		return nil
	}

	storageCtx, storageCancel := context.WithTimeout(ctx, StorageTimeout)
	defer storageCancel()

	// Download main transcript
	mainContent, err := downloadAndMergeFile(storageCtx, store, sessionUserID, externalID, mainFile.FileName)
	if err != nil || mainContent == nil {
		log.Error("Failed to download transcript for smart recap", "error", err, "session_id", sessionID)
		return nil
	}

	// Download agent files
	agentContents := make(map[string][]byte)
	for _, af := range agentFiles {
		agentID := extractAgentID(af.FileName)
		if agentID == "" {
			continue
		}
		content, err := downloadAndMergeFile(storageCtx, store, sessionUserID, externalID, af.FileName)
		if err != nil {
			continue
		}
		if content != nil {
			agentContents[agentID] = content
		}
	}

	// Build FileCollection
	fc, err := analytics.NewFileCollectionWithAgents(mainContent, agentContents)
	if err != nil {
		log.Error("Failed to parse transcript for smart recap", "error", err, "session_id", sessionID)
		return nil
	}

	return fc
}

// downloadAndMergeFile downloads and merges all chunks for a file.
// Returns nil content if no chunks exist (not an error).
func downloadAndMergeFile(ctx context.Context, store *storage.S3Storage, userID int64, externalID, fileName string) ([]byte, error) {
	chunkKeys, err := store.ListChunks(ctx, userID, externalID, fileName)
	if err != nil {
		return nil, err
	}
	if len(chunkKeys) == 0 {
		return nil, nil
	}

	chunks, err := downloadChunks(ctx, store, chunkKeys)
	if err != nil {
		return nil, err
	}
	if len(chunks) == 0 {
		return nil, nil
	}

	return mergeChunks(chunks)
}

// extractAgentID extracts the agent ID from a filename like "agent-{id}.jsonl".
// Returns empty string if the filename doesn't match the expected pattern.
func extractAgentID(fileName string) string {
	if !strings.HasPrefix(fileName, "agent-") || !strings.HasSuffix(fileName, ".jsonl") {
		return ""
	}
	// Remove "agent-" prefix and ".jsonl" suffix
	return strings.TrimSuffix(strings.TrimPrefix(fileName, "agent-"), ".jsonl")
}
