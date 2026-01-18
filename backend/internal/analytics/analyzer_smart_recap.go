package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/anthropic"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	// MaxOutputTokens is the maximum number of output tokens for the recap.
	MaxOutputTokens = 1400

	// MaxTranscriptTokens is the approximate maximum input size (characters / 4 as rough estimate).
	MaxTranscriptTokens = 100000

	// MaxTranscriptChars is the max characters to send (rough approximation of tokens).
	MaxTranscriptChars = MaxTranscriptTokens * 4
)

// SmartRecapResult contains the parsed LLM response.
type SmartRecapResult struct {
	Recap                     string   `json:"recap"`
	WentWell                  []string `json:"went_well"`
	WentBad                   []string `json:"went_bad"`
	HumanSuggestions          []string `json:"human_suggestions"`
	EnvironmentSuggestions    []string `json:"environment_suggestions"`
	DefaultContextSuggestions []string `json:"default_context_suggestions"`

	// Metadata from LLM response
	InputTokens      int
	OutputTokens     int
	GenerationTimeMs int
}

// SmartRecapAnalyzer generates AI-powered session recaps using Claude Haiku.
type SmartRecapAnalyzer struct {
	client *anthropic.Client
	model  string
}

// NewSmartRecapAnalyzer creates a new analyzer with the given Anthropic client.
func NewSmartRecapAnalyzer(client *anthropic.Client, model string) *SmartRecapAnalyzer {
	return &SmartRecapAnalyzer{
		client: client,
		model:  model,
	}
}

// Analyze generates a smart recap for the given transcript.
func (a *SmartRecapAnalyzer) Analyze(ctx context.Context, fc *FileCollection) (*SmartRecapResult, error) {
	ctx, span := tracer.Start(ctx, "analytics.smart_recap.analyze",
		trace.WithAttributes(attribute.String("llm.model", a.model)))
	defer span.End()

	// Prepare the transcript for the LLM
	transcript := PrepareTranscript(fc)
	if transcript == "" {
		err := fmt.Errorf("no content to analyze")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	// Track transcript size
	transcriptLen := len(transcript)
	truncated := false

	// Truncate if too long
	if transcriptLen > MaxTranscriptChars {
		transcript = transcript[:MaxTranscriptChars] + "\n\n[Transcript truncated due to length]"
		truncated = true
	}

	span.SetAttributes(
		attribute.Int("transcript.chars", transcriptLen),
		attribute.Bool("transcript.truncated", truncated),
	)

	start := time.Now()

	// Create the request with low temperature for consistent output
	temperature := 0.0
	resp, err := a.client.CreateMessage(ctx, &anthropic.MessagesRequest{
		Model:       a.model,
		MaxTokens:   MaxOutputTokens,
		Temperature: &temperature,
		System:      smartRecapSystemPrompt,
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

	// Parse the response
	result, err := parseSmartRecapResponse(resp.GetTextContent())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	result.InputTokens = resp.Usage.InputTokens
	result.OutputTokens = resp.Usage.OutputTokens
	result.GenerationTimeMs = generationTimeMs

	// Record final metrics
	span.SetAttributes(
		attribute.Int("llm.tokens.input", result.InputTokens),
		attribute.Int("llm.tokens.output", result.OutputTokens),
		attribute.Int("generation.time_ms", generationTimeMs),
	)

	return result, nil
}

// PrepareTranscript converts the file collection into an XML format suitable for LLM analysis.
func PrepareTranscript(fc *FileCollection) string {
	var sb strings.Builder

	// Build a map of tool_use_id -> tool_name for resolving tool results
	toolNameMap := make(map[string]string)
	for _, file := range fc.AllFiles() {
		for _, line := range file.Lines {
			if line.IsAssistantMessage() {
				for _, tool := range line.GetToolUses() {
					if tool.ID != "" {
						toolNameMap[tool.ID] = tool.Name
					}
				}
			}
		}
	}

	sb.WriteString("<transcript>\n")
	for _, file := range fc.AllFiles() {
		for _, line := range file.Lines {
			formatted := formatLine(line, toolNameMap)
			if formatted != "" {
				sb.WriteString(formatted)
				sb.WriteString("\n")
			}
		}
	}
	sb.WriteString("</transcript>")

	return sb.String()
}

// formatLine converts a transcript line to XML format for the LLM.
func formatLine(line *TranscriptLine, toolNameMap map[string]string) string {
	switch line.Type {
	case "user":
		return formatUserLine(line, toolNameMap)
	case "assistant":
		return formatAssistantLine(line)
	default:
		return ""
	}
}

// formatUserLine formats a user message for the LLM in XML format.
func formatUserLine(line *TranscriptLine, toolNameMap map[string]string) string {
	if line.IsHumanMessage() {
		content := getStringContent(line)
		if content != "" {
			// Truncate very long user messages
			if len(content) > 2000 {
				content = content[:2000] + "... [truncated]"
			}
			return fmt.Sprintf("<user>\n%s\n</user>", content)
		}
	}

	// Tool results - note results with tool names
	if line.IsToolResultMessage() {
		blocks := getToolResultBlocks(line, toolNameMap)
		if len(blocks) > 0 {
			var results []string
			for _, block := range blocks {
				status := "success"
				if block.isError {
					status = "error"
				}
				results = append(results, fmt.Sprintf("  <result tool=\"%s\" status=\"%s\"/>", block.toolName, status))
			}
			return fmt.Sprintf("<tool_results>\n%s\n</tool_results>", strings.Join(results, "\n"))
		}
	}

	return ""
}

// formatAssistantLine formats an assistant message for the LLM in XML format.
func formatAssistantLine(line *TranscriptLine) string {
	if !line.IsAssistantMessage() {
		return ""
	}

	var innerParts []string

	// Get text content
	textContent := getAssistantTextContent(line)
	if textContent != "" {
		// Truncate very long responses
		if len(textContent) > 3000 {
			textContent = textContent[:3000] + "... [truncated]"
		}
		innerParts = append(innerParts, textContent)
	}

	// Get tool uses (just names, not full input)
	toolUses := line.GetToolUses()
	if len(toolUses) > 0 {
		var tools []string
		for _, tool := range toolUses {
			tools = append(tools, tool.Name)
		}
		innerParts = append(innerParts, fmt.Sprintf("<tools_called>%s</tools_called>", strings.Join(tools, ", ")))
	}

	if len(innerParts) > 0 {
		return fmt.Sprintf("<assistant>\n%s\n</assistant>", strings.Join(innerParts, "\n"))
	}

	return ""
}

// getStringContent extracts string content from a user message.
func getStringContent(line *TranscriptLine) string {
	if line.Message == nil || line.Message.Content == nil {
		return ""
	}
	if s, ok := line.Message.Content.(string); ok {
		return s
	}
	return ""
}

// getAssistantTextContent extracts text content from an assistant message.
func getAssistantTextContent(line *TranscriptLine) string {
	if line.Message == nil || line.Message.Content == nil {
		return ""
	}

	// String content
	if s, ok := line.Message.Content.(string); ok {
		return s
	}

	// Array content - extract text blocks
	blocks := line.GetContentBlocks()
	var texts []string
	for _, block := range blocks {
		if block.Type == "text" {
			// Get text from the raw content
			if contentArray, ok := line.Message.Content.([]interface{}); ok {
				for _, item := range contentArray {
					if blockMap, ok := item.(map[string]interface{}); ok {
						if blockMap["type"] == "text" {
							if text, ok := blockMap["text"].(string); ok {
								texts = append(texts, text)
							}
						}
					}
				}
			}
		}
	}

	return strings.Join(texts, "\n")
}

type toolResultBlock struct {
	toolName string
	isError  bool
}

// getToolResultBlocks extracts tool result information from a user message.
func getToolResultBlocks(line *TranscriptLine, toolNameMap map[string]string) []toolResultBlock {
	if line.Message == nil || line.Message.Content == nil {
		return nil
	}

	contentArray, ok := line.Message.Content.([]interface{})
	if !ok {
		return nil
	}

	var blocks []toolResultBlock
	for _, item := range contentArray {
		blockMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if blockMap["type"] != "tool_result" {
			continue
		}

		block := toolResultBlock{}
		if isErr, ok := blockMap["is_error"].(bool); ok {
			block.isError = isErr
		}

		// Resolve tool name from tool_use_id
		block.toolName = "unknown"
		if toolUseID, ok := blockMap["tool_use_id"].(string); ok {
			if name, exists := toolNameMap[toolUseID]; exists {
				block.toolName = name
			}
		}
		blocks = append(blocks, block)
	}

	return blocks
}

// parseSmartRecapResponse parses the JSON response from the LLM.
func parseSmartRecapResponse(content string) (*SmartRecapResult, error) {
	// Try to extract JSON from the response (in case there's extra text)
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")
	if jsonStart == -1 || jsonEnd == -1 || jsonEnd < jsonStart {
		return nil, fmt.Errorf("no JSON found in response")
	}

	jsonContent := content[jsonStart : jsonEnd+1]

	var result SmartRecapResult
	if err := json.Unmarshal([]byte(jsonContent), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Validate and limit array sizes
	if len(result.WentWell) > 3 {
		result.WentWell = result.WentWell[:3]
	}
	if len(result.WentBad) > 3 {
		result.WentBad = result.WentBad[:3]
	}
	if len(result.HumanSuggestions) > 3 {
		result.HumanSuggestions = result.HumanSuggestions[:3]
	}
	if len(result.EnvironmentSuggestions) > 3 {
		result.EnvironmentSuggestions = result.EnvironmentSuggestions[:3]
	}
	if len(result.DefaultContextSuggestions) > 3 {
		result.DefaultContextSuggestions = result.DefaultContextSuggestions[:3]
	}

	// Ensure arrays are not nil (for JSON serialization)
	if result.WentWell == nil {
		result.WentWell = []string{}
	}
	if result.WentBad == nil {
		result.WentBad = []string{}
	}
	if result.HumanSuggestions == nil {
		result.HumanSuggestions = []string{}
	}
	if result.EnvironmentSuggestions == nil {
		result.EnvironmentSuggestions = []string{}
	}
	if result.DefaultContextSuggestions == nil {
		result.DefaultContextSuggestions = []string{}
	}

	return &result, nil
}

const smartRecapSystemPrompt = `You are a highly expert software engineer with decades of experience working in the software industry. You have become highly proficient in using Claude Code for software engineering tasks. You have an in-depth understanding of software engineering best practices in general, and you know how to marry such understanding in the new world of Claude Code assisted engineering. You are a great communicator who explains complex concepts in simple terms and in an approachable tone.

You are analyzing a Claude Code session transcript provided in XML format. The transcript contains:
- <user> tags for human messages
- <assistant> tags for Claude's responses (may include <tools_called> listing tool names)
- <tool_results> tags showing which tools succeeded or failed

Provide a high-signal analysis.

Output ONLY valid JSON with these fields:
- recap: Short 2-3 sentence recap of what occurred
- went_well: Up to 3 things that went well (omit or use empty array if none are clearly valid)
- went_bad: Up to 3 things that did not go well (omit or use empty array if none are clearly valid)
- human_suggestions: Up to 3 human technique improvements (e.g., "provide more context in initial prompts")
- environment_suggestions: Up to 3 environment improvements (e.g., "speed up test suite")
- default_context_suggestions: Up to 3 CLAUDE.md/system context improvements

Guidelines:
- Keep lists very high signal. Better to omit an item than show something low-confidence.
- Suggestions should be concise and actionable. Don't prefix with "suggest" - they're already suggestions.
- Focus on what would actually improve future sessions.
- Output ONLY the JSON object, no additional text.

Example output:
{
  "recap": "User implemented a dark mode feature, iterating on CSS variables and component updates. Tests were added and all passed.",
  "went_well": ["Clear initial requirements", "Good iteration on feedback"],
  "went_bad": ["Multiple rounds needed to fix CSS specificity issues"],
  "human_suggestions": ["Include browser compatibility requirements upfront"],
  "environment_suggestions": [],
  "default_context_suggestions": ["Add project's CSS architecture patterns to CLAUDE.md"]
}`
