// ABOUTME: AI-powered learning extractor that analyzes session transcripts to find reusable insights.
// ABOUTME: Uses the Anthropic API to identify gotchas, debugging techniques, and architecture decisions.
package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/anthropic"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	// DefaultLearningMaxOutputTokens is the default maximum output tokens for learning extraction.
	DefaultLearningMaxOutputTokens = 2000

	// DefaultLearningMaxTranscriptChars is the default maximum transcript length in characters.
	DefaultLearningMaxTranscriptChars = 200000
)

// LearningCandidate represents a single reusable insight extracted from a session transcript.
type LearningCandidate struct {
	Title string   `json:"title"`
	Body  string   `json:"body"`
	Tags  []string `json:"tags"`
}

// LearningExtractorConfig holds tunable parameters for the learning extractor.
type LearningExtractorConfig struct {
	MaxOutputTokens    int // 0 means use DefaultLearningMaxOutputTokens
	MaxTranscriptChars int // 0 means use DefaultLearningMaxTranscriptChars
}

// LearningExtractor analyzes session transcripts to extract reusable learnings using the Anthropic API.
type LearningExtractor struct {
	client             *anthropic.Client
	model              string
	maxOutputTokens    int
	maxTranscriptChars int
}

// NewLearningExtractor creates a new extractor with the given Anthropic client.
func NewLearningExtractor(client *anthropic.Client, model string, cfg LearningExtractorConfig) *LearningExtractor {
	maxOutput := cfg.MaxOutputTokens
	if maxOutput <= 0 {
		maxOutput = DefaultLearningMaxOutputTokens
	}
	maxChars := cfg.MaxTranscriptChars
	if maxChars <= 0 {
		maxChars = DefaultLearningMaxTranscriptChars
	}
	return &LearningExtractor{
		client:             client,
		model:              model,
		maxOutputTokens:    maxOutput,
		maxTranscriptChars: maxChars,
	}
}

// Extract analyzes the transcript and returns reusable learning candidates.
func (e *LearningExtractor) Extract(ctx context.Context, transcript string, sessionID string, userID int64) ([]LearningCandidate, error) {
	ctx, span := tracer.Start(ctx, "analytics.learning_extractor.extract",
		trace.WithAttributes(
			attribute.String("llm.model", e.model),
			attribute.String("session.id", sessionID),
			attribute.Int64("user.id", userID),
		))
	defer span.End()

	if transcript == "" {
		err := fmt.Errorf("no transcript content to analyze")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	// Truncate transcript if too long
	truncated := false
	if len(transcript) > e.maxTranscriptChars {
		transcript = transcript[:e.maxTranscriptChars] + "\n\n[Transcript truncated due to length]"
		truncated = true
	}

	span.SetAttributes(
		attribute.Int("transcript.chars", len(transcript)),
		attribute.Bool("transcript.truncated", truncated),
	)

	start := time.Now()

	// Use low temperature for consistent extraction
	temperature := 0.2
	resp, err := e.client.CreateMessage(ctx, &anthropic.MessagesRequest{
		Model:       e.model,
		MaxTokens:   e.maxOutputTokens,
		Temperature: &temperature,
		System:      learningExtractionSystemPrompt,
		Messages: []anthropic.Message{
			{Role: "user", Content: transcript},
		},
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}

	generationTimeMs := int(time.Since(start).Milliseconds())

	// Parse the JSON array response
	llmContent := resp.GetTextContent()
	candidates, err := parseLearningResponse(llmContent)
	if err != nil {
		slog.Error("learning extraction parse failed",
			"error", err,
			"model", e.model,
			"session_id", sessionID,
			"response_length", len(llmContent),
			"raw_response", llmContent,
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	span.SetAttributes(
		attribute.Int("llm.tokens.input", resp.Usage.InputTokens),
		attribute.Int("llm.tokens.output", resp.Usage.OutputTokens),
		attribute.Int("generation.time_ms", generationTimeMs),
		attribute.Int("learnings.count", len(candidates)),
	)

	return candidates, nil
}

// parseLearningResponse parses the JSON array of learning candidates from the LLM response.
func parseLearningResponse(content string) ([]LearningCandidate, error) {
	// Trim whitespace and find the JSON array
	content = findJSONArray(content)
	if content == "" {
		return nil, fmt.Errorf("no JSON array found in response")
	}

	var candidates []LearningCandidate
	if err := json.Unmarshal([]byte(content), &candidates); err != nil {
		return nil, fmt.Errorf("failed to parse JSON array: %w", err)
	}

	// Ensure non-nil tags on each candidate
	for i := range candidates {
		if candidates[i].Tags == nil {
			candidates[i].Tags = []string{}
		}
		// Truncate title if too long
		if len(candidates[i].Title) > 100 {
			candidates[i].Title = candidates[i].Title[:100]
		}
	}

	return candidates, nil
}

// findJSONArray extracts the outermost JSON array from a string that may contain surrounding text.
func findJSONArray(s string) string {
	// Find first '[' and last ']'
	start := -1
	for i, c := range s {
		if c == '[' {
			start = i
			break
		}
	}
	if start == -1 {
		return ""
	}

	end := -1
	for i := len(s) - 1; i >= start; i-- {
		if s[i] == ']' {
			end = i
			break
		}
	}
	if end == -1 {
		return ""
	}

	return s[start : end+1]
}

const learningExtractionSystemPrompt = `You are analyzing a Claude Code session transcript to extract reusable learnings for a team knowledge base.

Extract insights that would help team members avoid repeating mistakes, understand gotchas, or reuse solutions. Focus on:
- Configuration gotchas (e.g., proxy settings, certificate issues)
- Debugging techniques that worked
- Architecture decisions and their reasoning
- Tool-specific tips or workarounds
- Environment-specific knowledge (networking, permissions, etc.)

Output ONLY valid JSON: an array of objects, each with:
- "title": Short descriptive title (max 100 chars)
- "body": Detailed explanation in markdown (2-5 sentences)
- "tags": Array of lowercase tags (e.g., ["openshift", "networking", "proxy"])

If no reusable learnings are found, return an empty array: []

Example output:
[
  {
    "title": "Enterprise proxy blocks OCP signature verification",
    "body": "When upgrading OpenShift behind a corporate proxy, the signature verification step may fail because the proxy intercepts HTTPS traffic. The workaround is to manually inject the release signature as a configmap in the openshift-config-managed namespace.",
    "tags": ["openshift", "proxy", "upgrade"]
  }
]`
