package analytics

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/anthropic"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/db/dbadminsettings"
	"github.com/ConfabulousDev/confab-web/internal/recapquota"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// SmartRecapGeneratorConfig holds configuration for the smart recap generator.
type SmartRecapGeneratorConfig struct {
	APIKey              string
	Model               string
	GenerationTimeout   time.Duration
	MaxOutputTokens     int    // 0 means use DefaultMaxOutputTokens
	MaxTranscriptTokens int    // 0 means use DefaultMaxTranscriptTokens
	BaseURL             string // Custom base URL for the Anthropic API (for testing)
}

// SmartRecapGenerator handles the full smart recap generation flow.
// It coordinates lock acquisition, LLM generation, and persistence.
type SmartRecapGenerator struct {
	store         *Store
	db            *sql.DB
	settingsStore *dbadminsettings.Store
	config        SmartRecapGeneratorConfig
}

// NewSmartRecapGenerator creates a new generator with the given dependencies.
func NewSmartRecapGenerator(store *Store, database *db.DB, config SmartRecapGeneratorConfig) *SmartRecapGenerator {
	// Default timeout if not specified
	if config.GenerationTimeout == 0 {
		config.GenerationTimeout = 30 * time.Second
	}
	return &SmartRecapGenerator{
		store:         store,
		db:            database.Conn(),
		settingsStore: &dbadminsettings.Store{DB: database},
		config:        config,
	}
}

// resolveSystemPrompt fetches the custom instructions from admin_settings (if any)
// and assembles the full system prompt. Returns a fully assembled prompt string.
func (g *SmartRecapGenerator) resolveSystemPrompt(ctx context.Context) string {
	setting, err := g.settingsStore.Get(ctx, "smart_recap_system_prompt")
	if err != nil {
		slog.WarnContext(ctx, "failed to fetch custom smart recap prompt, using default", "error", err)
	}
	if err != nil || setting == nil {
		return BuildSmartRecapSystemPrompt(nil)
	}
	return BuildSmartRecapSystemPrompt(&setting.Value)
}

// GenerateInput contains all the information needed to generate a smart recap.
// Provide either FileCollection (convenience) or Transcript+IDMap (streaming).
// If Transcript is set, FileCollection is not used for transcript preparation.
type GenerateInput struct {
	SessionID      string
	UserID         int64
	LineCount      int64
	FileCollection *FileCollection // used when all files are in memory
	Transcript     string          // pre-built XML transcript (streaming path)
	IDMap          map[int]string  // sequential ID -> UUID map (streaming path)
	CardStats      map[string]interface{}
}

// GenerateResult contains the result of a generation attempt.
type GenerateResult struct {
	Card           *SmartRecapCardRecord
	SuggestedTitle string // Title from LLM, empty if not generated
	Skipped        bool   // True if generation was skipped (lock held, etc.)
	Error          error
}

// Generate creates a smart recap for the given session.
// It handles lock acquisition, LLM generation, saving, title update, and quota increment.
// If the lock cannot be acquired, it returns Skipped=true without an error.
// The caller is responsible for checking staleness and quota before calling this.
// If skipQuota is true, the quota increment is skipped (used for admin-triggered regeneration).
func (g *SmartRecapGenerator) Generate(ctx context.Context, input GenerateInput, lockTimeoutSeconds int, skipQuota bool) *GenerateResult {
	return g.generate(ctx, input, lockTimeoutSeconds, skipQuota, false)
}

// GenerateWithMessageIDClearing generates a smart recap and clears annotated
// item message IDs before saving. Use this for providers that lack stable
// frontend anchors.
func (g *SmartRecapGenerator) GenerateWithMessageIDClearing(ctx context.Context, input GenerateInput, lockTimeoutSeconds int, skipQuota bool) *GenerateResult {
	return g.generate(ctx, input, lockTimeoutSeconds, skipQuota, true)
}

func (g *SmartRecapGenerator) generate(ctx context.Context, input GenerateInput, lockTimeoutSeconds int, skipQuota bool, clearIDs bool) *GenerateResult {
	ctx, span := tracer.Start(ctx, "smart_recap.generate",
		trace.WithAttributes(
			attribute.String("session.id", input.SessionID),
			attribute.Int64("session.line_count", input.LineCount),
			attribute.String("llm.model", g.config.Model),
		))
	defer span.End()

	// Try to acquire the lock
	acquired, err := g.store.AcquireSmartRecapLock(ctx, input.SessionID, lockTimeoutSeconds)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return &GenerateResult{Error: err}
	}
	if !acquired {
		span.SetAttributes(attribute.Bool("lock.skipped", true))
		return &GenerateResult{Skipped: true}
	}

	// Resolve the system prompt: check admin_settings for a custom instructions override,
	// then assemble the full prompt. The generator owns all prompt resolution logic.
	systemPrompt := g.resolveSystemPrompt(ctx)

	// Create the analyzer and generate
	var clientOpts []anthropic.ClientOption
	if g.config.BaseURL != "" {
		clientOpts = append(clientOpts, anthropic.WithBaseURL(g.config.BaseURL))
	}
	client := anthropic.NewClient(g.config.APIKey, clientOpts...)
	analyzer := NewSmartRecapAnalyzer(client, g.config.Model, SmartRecapAnalyzerConfig{
		MaxOutputTokens:     g.config.MaxOutputTokens,
		MaxTranscriptTokens: g.config.MaxTranscriptTokens,
		SystemPrompt:        systemPrompt,
	})

	genCtx, genCancel := context.WithTimeout(ctx, g.config.GenerationTimeout)
	defer genCancel()
	result, err := analyzer.Analyze(genCtx, input, input.CardStats)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		// Clear the lock so another request can try
		// Use background context to ensure cleanup happens even if request was canceled
		clearCtx, clearCancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = g.store.ClearSmartRecapLock(clearCtx, input.SessionID)
		clearCancel()
		return &GenerateResult{Error: err}
	}

	// Codex sessions have no stable per-message id. Zero MessageID on every
	// annotated item so the frontend's SmartRecapCard short-circuit renders
	// items as plain text instead of broken anchor links.
	if clearIDs {
		clearMessageIDs(result)
	}

	// Build the card record
	card := &SmartRecapCardRecord{
		SessionID:                 input.SessionID,
		Version:                   SmartRecapCardVersion,
		ComputedAt:                time.Now().UTC(),
		UpToLine:                  input.LineCount,
		Recap:                     result.Recap,
		WentWell:                  result.WentWell,
		WentBad:                   result.WentBad,
		HumanSuggestions:          result.HumanSuggestions,
		EnvironmentSuggestions:    result.EnvironmentSuggestions,
		DefaultContextSuggestions: result.DefaultContextSuggestions,
		ModelUsed:                 g.config.Model,
		InputTokens:               result.InputTokens,
		OutputTokens:              result.OutputTokens,
		GenerationTimeMs:          &result.GenerationTimeMs,
	}

	// Use background context to ensure operations complete even if request was canceled
	saveCtx, saveCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer saveCancel()

	// Increment quota BEFORE saving the card.
	// If we can't track usage, we must not produce the recap.
	// Admin-triggered regeneration (skipQuota=true) bypasses this.
	if !skipQuota {
		if err := recapquota.Increment(saveCtx, g.db, input.UserID); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "quota increment failed: "+err.Error())
			_ = g.store.ClearSmartRecapLock(saveCtx, input.SessionID)
			return &GenerateResult{Error: fmt.Errorf("failed to increment quota: %w", err)}
		}
	}

	// Save the card (this also clears the lock via upsert)
	if err := g.store.UpsertSmartRecapCard(saveCtx, card); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		_ = g.store.ClearSmartRecapLock(saveCtx, input.SessionID)
		return &GenerateResult{Error: err}
	}

	// Update suggested title if generated
	if result.SuggestedSessionTitle != "" {
		_, err := g.db.ExecContext(saveCtx, `UPDATE sessions SET suggested_session_title = $1 WHERE id = $2`,
			result.SuggestedSessionTitle, input.SessionID)
		if err != nil {
			// Log but don't fail - the main operation succeeded
			span.SetAttributes(attribute.String("title.update.error", err.Error()))
		}
	}

	span.SetAttributes(
		attribute.Int("llm.tokens.input", result.InputTokens),
		attribute.Int("llm.tokens.output", result.OutputTokens),
		attribute.Int("generation.time_ms", result.GenerationTimeMs),
	)

	return &GenerateResult{Card: card, SuggestedTitle: result.SuggestedSessionTitle}
}

// clearMessageIDs zeroes every AnnotatedItem.MessageID across all bucket
// slices on a SmartRecapResult. Used by the Codex precompute path because
// Codex messages don't have stable ids the frontend can anchor on.
func clearMessageIDs(r *SmartRecapResult) {
	buckets := [][]AnnotatedItem{
		r.WentWell,
		r.WentBad,
		r.HumanSuggestions,
		r.EnvironmentSuggestions,
		r.DefaultContextSuggestions,
	}
	for _, items := range buckets {
		for i := range items {
			items[i].MessageID = ""
		}
	}
}
