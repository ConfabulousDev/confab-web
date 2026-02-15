package analytics

import (
	"context"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/anthropic"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// SmartRecapDB defines the database operations needed for smart recap generation.
// This interface allows the analytics package to remain decoupled from the db package.
type SmartRecapDB interface {
	// IncrementSmartRecapQuota increments the compute count for the user.
	IncrementSmartRecapQuota(ctx context.Context, userID int64) error
	// UpdateSessionSuggestedTitle updates the suggested title for a session.
	UpdateSessionSuggestedTitle(ctx context.Context, sessionID string, title string) error
}

// SmartRecapGeneratorConfig holds configuration for the smart recap generator.
type SmartRecapGeneratorConfig struct {
	APIKey              string
	Model               string
	GenerationTimeout   time.Duration
	MaxOutputTokens     int // 0 means use DefaultMaxOutputTokens
	MaxTranscriptTokens int // 0 means use DefaultMaxTranscriptTokens
}

// SmartRecapGenerator handles the full smart recap generation flow.
// It coordinates lock acquisition, LLM generation, and persistence.
type SmartRecapGenerator struct {
	store  *Store
	db     SmartRecapDB
	config SmartRecapGeneratorConfig
}

// NewSmartRecapGenerator creates a new generator with the given dependencies.
func NewSmartRecapGenerator(store *Store, db SmartRecapDB, config SmartRecapGeneratorConfig) *SmartRecapGenerator {
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
	Card    *SmartRecapCardRecord
	Skipped bool   // True if generation was skipped (lock held, etc.)
	Error   error
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
	client := anthropic.NewClient(g.config.APIKey)
	analyzer := NewSmartRecapAnalyzer(client, g.config.Model, SmartRecapAnalyzerConfig{
		MaxOutputTokens:    g.config.MaxOutputTokens,
		MaxTranscriptTokens: g.config.MaxTranscriptTokens,
	})

	genCtx, genCancel := context.WithTimeout(ctx, g.config.GenerationTimeout)
	result, err := analyzer.Analyze(genCtx, input.FileCollection, input.CardStats)
	genCancel()

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

	// Save the card (this also clears the lock via upsert)
	// Use background context to ensure save completes even if request was canceled
	saveCtx, saveCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer saveCancel()

	if err := g.store.UpsertSmartRecapCard(saveCtx, card); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		_ = g.store.ClearSmartRecapLock(saveCtx, input.SessionID)
		return &GenerateResult{Error: err}
	}

	// Update suggested title if generated
	if result.SuggestedSessionTitle != "" {
		if err := g.db.UpdateSessionSuggestedTitle(saveCtx, input.SessionID, result.SuggestedSessionTitle); err != nil {
			// Log but don't fail - the main operation succeeded
			span.SetAttributes(attribute.String("title.update.error", err.Error()))
		}
	}

	// Increment quota
	if err := g.db.IncrementSmartRecapQuota(saveCtx, input.UserID); err != nil {
		// Log but don't fail - the main operation succeeded
		span.SetAttributes(attribute.String("quota.increment.error", err.Error()))
	}

	span.SetAttributes(
		attribute.Int("llm.tokens.input", result.InputTokens),
		attribute.Int("llm.tokens.output", result.OutputTokens),
		attribute.Int("generation.time_ms", result.GenerationTimeMs),
	)

	return &GenerateResult{Card: card}
}
