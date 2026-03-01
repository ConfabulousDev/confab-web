package analytics

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/anthropic"
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
	MaxOutputTokens     int // 0 means use DefaultMaxOutputTokens
	MaxTranscriptTokens int // 0 means use DefaultMaxTranscriptTokens
	BaseURL             string // Custom base URL for the Anthropic API (for testing)
}

// SmartRecapGenerator handles the full smart recap generation flow.
// It coordinates lock acquisition, LLM generation, and persistence.
type SmartRecapGenerator struct {
	store  *Store
	db     *sql.DB
	config SmartRecapGeneratorConfig
}

// NewSmartRecapGenerator creates a new generator with the given dependencies.
func NewSmartRecapGenerator(store *Store, db *sql.DB, config SmartRecapGeneratorConfig) *SmartRecapGenerator {
	// Default timeout if not specified
	if config.GenerationTimeout == 0 {
		config.GenerationTimeout = 30 * time.Second
	}
	return &SmartRecapGenerator{
		store:  store,
		db:     db,
		config: config,
	}
}

// GenerateInput contains all the information needed to generate a smart recap.
type GenerateInput struct {
	SessionID    string
	UserID       int64
	LineCount    int64
	FileCollection *FileCollection
	CardStats    map[string]interface{}
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
func (g *SmartRecapGenerator) Generate(ctx context.Context, input GenerateInput, lockTimeoutSeconds int) *GenerateResult {
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

	// Create the analyzer and generate
	var clientOpts []anthropic.ClientOption
	if g.config.BaseURL != "" {
		clientOpts = append(clientOpts, anthropic.WithBaseURL(g.config.BaseURL))
	}
	client := anthropic.NewClient(g.config.APIKey, clientOpts...)
	analyzer := NewSmartRecapAnalyzer(client, g.config.Model, SmartRecapAnalyzerConfig{
		MaxOutputTokens:    g.config.MaxOutputTokens,
		MaxTranscriptTokens: g.config.MaxTranscriptTokens,
	})

	genCtx, genCancel := context.WithTimeout(ctx, g.config.GenerationTimeout)
	defer genCancel()
	result, err := analyzer.Analyze(genCtx, input.FileCollection, input.CardStats)

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
	if err := recapquota.Increment(saveCtx, g.db, input.UserID); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "quota increment failed: "+err.Error())
		_ = g.store.ClearSmartRecapLock(saveCtx, input.SessionID)
		return &GenerateResult{Error: fmt.Errorf("failed to increment quota: %w", err)}
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
