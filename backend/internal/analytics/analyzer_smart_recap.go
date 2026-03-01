package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/anthropic"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	// DefaultMaxOutputTokens is the default maximum number of output tokens for the recap.
	DefaultMaxOutputTokens = 1000

	// DefaultMaxTranscriptTokens is the default approximate maximum input size (characters / 4 as rough estimate).
	DefaultMaxTranscriptTokens = 50000
)

// AnnotatedItem represents a list item with optional message reference.
// Supports backwards-compatible unmarshaling: accepts both plain strings (legacy)
// and objects with text + optional message_id (new format).
type AnnotatedItem struct {
	Text      string `json:"text"`
	MessageID string `json:"message_id,omitempty"`
}

// UnmarshalJSON implements custom unmarshaling for AnnotatedItem.
// Accepts both "string" (legacy) and {"text":"...", "message_id":"..."} (new).
func (a *AnnotatedItem) UnmarshalJSON(data []byte) error {
	// Try string first (legacy format)
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		a.Text = s
		return nil
	}

	// Try object format
	type annotatedItemRaw struct {
		Text      string      `json:"text"`
		MessageID interface{} `json:"message_id,omitempty"`
	}
	var raw annotatedItemRaw
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	a.Text = raw.Text

	// message_id from LLM can be an integer or string
	switch v := raw.MessageID.(type) {
	case float64:
		a.MessageID = strconv.Itoa(int(v))
	case string:
		a.MessageID = v
	default:
		a.MessageID = ""
	}
	return nil
}

// SmartRecapResult contains the parsed LLM response.
type SmartRecapResult struct {
	SuggestedSessionTitle     string          `json:"suggested_session_title"`
	Recap                     string          `json:"recap"`
	WentWell                  []AnnotatedItem `json:"went_well"`
	WentBad                   []AnnotatedItem `json:"went_bad"`
	HumanSuggestions          []AnnotatedItem `json:"human_suggestions"`
	EnvironmentSuggestions    []AnnotatedItem `json:"environment_suggestions"`
	DefaultContextSuggestions []AnnotatedItem `json:"default_context_suggestions"`

	// Metadata from LLM response
	InputTokens      int
	OutputTokens     int
	GenerationTimeMs int
}

// SmartRecapAnalyzer generates AI-powered session recaps using Claude Haiku.
type SmartRecapAnalyzer struct {
	client             *anthropic.Client
	model              string
	maxOutputTokens    int
	maxTranscriptChars int
}

// SmartRecapAnalyzerConfig holds tunable parameters for the analyzer.
type SmartRecapAnalyzerConfig struct {
	MaxOutputTokens    int // 0 means use DefaultMaxOutputTokens
	MaxTranscriptTokens int // 0 means use DefaultMaxTranscriptTokens
}

// NewSmartRecapAnalyzer creates a new analyzer with the given Anthropic client.
func NewSmartRecapAnalyzer(client *anthropic.Client, model string, cfg SmartRecapAnalyzerConfig) *SmartRecapAnalyzer {
	maxOutput := cfg.MaxOutputTokens
	if maxOutput <= 0 {
		maxOutput = DefaultMaxOutputTokens
	}
	maxTranscriptTokens := cfg.MaxTranscriptTokens
	if maxTranscriptTokens <= 0 {
		maxTranscriptTokens = DefaultMaxTranscriptTokens
	}
	return &SmartRecapAnalyzer{
		client:             client,
		model:              model,
		maxOutputTokens:    maxOutput,
		maxTranscriptChars: maxTranscriptTokens * 4,
	}
}

// Analyze generates a smart recap for the given transcript and analytics stats.
// cardStats contains the computed analytics cards (tokens, session, conversation, etc.)
// which are included in the prompt for additional context.
func (a *SmartRecapAnalyzer) Analyze(ctx context.Context, fc *FileCollection, cardStats map[string]interface{}) (*SmartRecapResult, error) {
	ctx, span := tracer.Start(ctx, "analytics.smart_recap.analyze",
		trace.WithAttributes(attribute.String("llm.model", a.model)))
	defer span.End()

	// Prepare the transcript for the LLM
	transcript, idMap := PrepareTranscript(fc)
	if transcript == "" {
		err := fmt.Errorf("no content to analyze")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	// Prepare the stats section
	statsSection := PrepareStats(cardStats)

	// Combine transcript and stats
	userContent := transcript
	if statsSection != "" {
		userContent = transcript + "\n\n" + statsSection
	}

	// Track content size
	contentLen := len(userContent)
	truncated := false

	// Truncate if too long (prioritize transcript, stats are at the end)
	if contentLen > a.maxTranscriptChars {
		// Truncate transcript portion, keep stats
		maxTranscript := a.maxTranscriptChars - len(statsSection) - 100 // leave room for truncation message
		if maxTranscript > 0 && len(transcript) > maxTranscript {
			transcript = transcript[:maxTranscript] + "\n\n[Transcript truncated due to length]"
			userContent = transcript + "\n\n" + statsSection
		}
		truncated = true
	}

	span.SetAttributes(
		attribute.Int("content.chars", contentLen),
		attribute.Bool("content.truncated", truncated),
		attribute.Bool("stats.included", statsSection != ""),
	)

	start := time.Now()

	// Create the request with low temperature for mostly consistent output
	// 0.25 allows slight variation on regeneration while staying focused
	temperature := 0.25
	resp, err := a.client.CreateMessage(ctx, &anthropic.MessagesRequest{
		Model:       a.model,
		MaxTokens:   a.maxOutputTokens,
		Temperature: &temperature,
		System:      smartRecapSystemPrompt,
		Messages: []anthropic.Message{
			{Role: "user", Content: userContent},
			// Prefill assistant response with "{" to force JSON output.
			// This prevents the model from role-playing as Claude Code when
			// analyzing transcripts that contain tool calls.
			{Role: "assistant", Content: "{"},
		},
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}

	generationTimeMs := int(time.Since(start).Milliseconds())

	// Parse the response - prepend "{" since we used prefill and the API
	// returns only the continuation after the prefilled content
	llmContent := "{" + resp.GetTextContent()
	result, err := parseSmartRecapResponse(llmContent)
	if err != nil {
		// Log the raw LLM response for debugging parse failures
		slog.Error("smart recap parse failed",
			"error", err,
			"model", a.model,
			"response_length", len(llmContent),
			"raw_response", llmContent,
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	// Translate integer message_ids from LLM response to real UUIDs
	resolveMessageIDs(result, idMap)

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
// Returns the XML string and a mapping from sequential integer IDs to message UUIDs.
func PrepareTranscript(fc *FileCollection) (string, map[int]string) {
	var sb strings.Builder
	idMap := make(map[int]string) // sequential integer -> message UUID
	counter := 0

	// Build a map of tool_use_id -> tool_name for resolving tool results and skill expansions
	// For Skill tool uses, we store the skill name (from input.skill) instead of "Skill"
	toolNameMap := make(map[string]string)
	for _, file := range fc.AllFiles() {
		for _, line := range file.Lines {
			if line.IsAssistantMessage() {
				for _, tool := range line.GetToolUses() {
					if tool.ID != "" {
						// For Skill tools, extract the actual skill name from input
						if tool.Name == "Skill" {
							if skillName, ok := tool.Input["skill"].(string); ok && skillName != "" {
								toolNameMap[tool.ID] = skillName
								continue
							}
						}
						toolNameMap[tool.ID] = tool.Name
					}
				}
			}
		}
	}

	sb.WriteString("<transcript>\n")
	for _, file := range fc.AllFiles() {
		for _, line := range file.Lines {
			formatted, newCounter := formatLine(line, toolNameMap, counter, idMap)
			counter = newCounter
			if formatted != "" {
				sb.WriteString(formatted)
				sb.WriteString("\n")
			}
		}
	}
	sb.WriteString("</transcript>")

	return sb.String(), idMap
}

// PrepareStats formats the computed analytics cards as XML for the LLM.
// This provides additional context about session metrics for pattern detection.
func PrepareStats(cardStats map[string]interface{}) string {
	if len(cardStats) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<session_stats>\n")

	// Tokens card
	if tokens, ok := cardStats["tokens"].(TokensCardData); ok {
		sb.WriteString("  <tokens>\n")
		sb.WriteString(fmt.Sprintf("    <input>%d</input>\n", tokens.Input))
		sb.WriteString(fmt.Sprintf("    <output>%d</output>\n", tokens.Output))
		if tokens.EstimatedUSD != "" && tokens.EstimatedUSD != "0.00" {
			sb.WriteString(fmt.Sprintf("    <cost_usd>%s</cost_usd>\n", tokens.EstimatedUSD))
		}
		if tokens.CacheRead > 0 || tokens.CacheCreation > 0 {
			// Cache hit rate = CacheRead / (CacheRead + Input)
			// This represents the fraction of input tokens that came from cache
			// Only note when below 95% - high cache hit rates are normal for Claude Code
			totalInputTokens := tokens.CacheRead + tokens.Input
			if totalInputTokens > 0 {
				cacheHitRate := float64(tokens.CacheRead) / float64(totalInputTokens) * 100
				if cacheHitRate < 95 {
					sb.WriteString(fmt.Sprintf("    <cache_hit_rate_percent>%.1f</cache_hit_rate_percent>\n", cacheHitRate))
				}
			}
		}
		sb.WriteString("  </tokens>\n")
	}

	// Session card
	if session, ok := cardStats["session"].(SessionCardData); ok {
		sb.WriteString("  <session>\n")
		if session.DurationMs != nil && *session.DurationMs > 0 {
			sb.WriteString(fmt.Sprintf("    <duration_minutes>%.1f</duration_minutes>\n", float64(*session.DurationMs)/60000))
		}
		totalCompactions := session.CompactionAuto + session.CompactionManual
		if totalCompactions > 0 {
			sb.WriteString(fmt.Sprintf("    <compactions>%d</compactions>\n", totalCompactions))
		}
		sb.WriteString("  </session>\n")
	}

	// Conversation card
	if conv, ok := cardStats["conversation"].(ConversationCardData); ok {
		sb.WriteString("  <conversation>\n")
		sb.WriteString(fmt.Sprintf("    <user_turns>%d</user_turns>\n", conv.UserTurns))
		sb.WriteString(fmt.Sprintf("    <assistant_turns>%d</assistant_turns>\n", conv.AssistantTurns))
		// Only include avg user response time if > 5 minutes (indicates real breaks, not normal thinking)
		if conv.AvgUserThinkingMs != nil && *conv.AvgUserThinkingMs > 300000 {
			sb.WriteString(fmt.Sprintf("    <avg_user_response_minutes>%.1f</avg_user_response_minutes>\n", float64(*conv.AvgUserThinkingMs)/60000))
		}
		if conv.AssistantUtilizationPct != nil {
			sb.WriteString(fmt.Sprintf("    <assistant_utilization_percent>%.1f</assistant_utilization_percent>\n", *conv.AssistantUtilizationPct))
		}
		sb.WriteString("  </conversation>\n")
	}

	// Code Activity card
	if code, ok := cardStats["code_activity"].(CodeActivityCardData); ok {
		if code.FilesRead > 0 || code.FilesModified > 0 {
			sb.WriteString("  <code_activity>\n")
			if code.FilesRead > 0 {
				sb.WriteString(fmt.Sprintf("    <files_read>%d</files_read>\n", code.FilesRead))
			}
			if code.FilesModified > 0 {
				sb.WriteString(fmt.Sprintf("    <files_modified>%d</files_modified>\n", code.FilesModified))
			}
			if code.LinesAdded > 0 {
				sb.WriteString(fmt.Sprintf("    <lines_added>%d</lines_added>\n", code.LinesAdded))
			}
			if code.LinesRemoved > 0 {
				sb.WriteString(fmt.Sprintf("    <lines_removed>%d</lines_removed>\n", code.LinesRemoved))
			}
			sb.WriteString("  </code_activity>\n")
		}
	}

	// Tools card
	if tools, ok := cardStats["tools"].(ToolsCardData); ok {
		if tools.TotalCalls > 0 {
			sb.WriteString("  <tools>\n")
			sb.WriteString(fmt.Sprintf("    <total_calls>%d</total_calls>\n", tools.TotalCalls))
			if tools.ErrorCount > 0 {
				errorRate := float64(tools.ErrorCount) / float64(tools.TotalCalls) * 100
				sb.WriteString(fmt.Sprintf("    <error_rate_percent>%.1f</error_rate_percent>\n", errorRate))
			}
			sb.WriteString("  </tools>\n")
		}
	}

	// Agents and Skills card
	if as, ok := cardStats["agents_and_skills"].(AgentsAndSkillsCardData); ok {
		if as.AgentInvocations > 0 || as.SkillInvocations > 0 {
			sb.WriteString("  <agents_and_skills>\n")
			if as.AgentInvocations > 0 {
				sb.WriteString(fmt.Sprintf("    <agent_invocations>%d</agent_invocations>\n", as.AgentInvocations))
			}
			if as.SkillInvocations > 0 {
				sb.WriteString(fmt.Sprintf("    <skill_invocations>%d</skill_invocations>\n", as.SkillInvocations))
			}
			sb.WriteString("  </agents_and_skills>\n")
		}
	}

	// Redactions card
	if redact, ok := cardStats["redactions"].(RedactionsCardData); ok {
		if redact.TotalRedactions > 0 {
			sb.WriteString("  <redactions>\n")
			sb.WriteString(fmt.Sprintf("    <total>%d</total>\n", redact.TotalRedactions))
			sb.WriteString("  </redactions>\n")
		}
	}

	sb.WriteString("</session_stats>")

	return sb.String()
}

// formatLine converts a transcript line to XML format for the LLM.
// Returns the formatted string and the updated counter.
func formatLine(line *TranscriptLine, toolNameMap map[string]string, counter int, idMap map[int]string) (string, int) {
	switch line.Type {
	case "user":
		return formatUserLine(line, toolNameMap, counter, idMap)
	case "assistant":
		return formatAssistantLine(line, counter, idMap)
	default:
		return "", counter
	}
}

// formatUserLine formats a user message for the LLM in XML format.
// Returns the formatted string and the updated counter.
func formatUserLine(line *TranscriptLine, toolNameMap map[string]string, counter int, idMap map[int]string) (string, int) {
	// Check for skill expansion messages first (isMeta: true with sourceToolUseID)
	if line.IsSkillExpansionMessage() {
		content := getStringContent(line)
		if content != "" {
			counter++
			if line.UUID != "" {
				idMap[counter] = line.UUID
			}
			// Truncate skill content (can be lengthy)
			if len(content) > 1500 {
				content = content[:1500] + "... [truncated]"
			}
			// Get skill name from the linked tool_use if available
			skillName := ""
			if line.SourceToolUseID != "" {
				if name, ok := toolNameMap[line.SourceToolUseID]; ok {
					skillName = name
				}
			}
			if skillName != "" {
				return fmt.Sprintf("<skill id=\"%d\" name=\"%s\">\n%s\n</skill>", counter, skillName, content), counter
			}
			return fmt.Sprintf("<skill id=\"%d\">\n%s\n</skill>", counter, content), counter
		}
		return "", counter
	}

	if line.IsHumanMessage() {
		content := getStringContent(line)
		if content != "" {
			counter++
			if line.UUID != "" {
				idMap[counter] = line.UUID
			}
			// Truncate very long user messages
			if len(content) > 2000 {
				content = content[:2000] + "... [truncated]"
			}
			return fmt.Sprintf("<user id=\"%d\">\n%s\n</user>", counter, content), counter
		}
	}

	// Tool results - note results with tool names
	if line.IsToolResultMessage() {
		blocks := getToolResultBlocks(line, toolNameMap)
		if len(blocks) > 0 {
			counter++
			if line.UUID != "" {
				idMap[counter] = line.UUID
			}
			var results []string
			for _, block := range blocks {
				status := "success"
				if block.isError {
					status = "error"
				}
				results = append(results, fmt.Sprintf("  <result tool=\"%s\" status=\"%s\"/>", block.toolName, status))
			}
			return fmt.Sprintf("<tool_results id=\"%d\">\n%s\n</tool_results>", counter, strings.Join(results, "\n")), counter
		}
	}

	return "", counter
}

// formatAssistantLine formats an assistant message for the LLM in XML format.
// Returns the formatted string and the updated counter.
func formatAssistantLine(line *TranscriptLine, counter int, idMap map[int]string) (string, int) {
	if !line.IsAssistantMessage() {
		return "", counter
	}

	var innerParts []string

	// Get thinking content (shown by default in UI)
	thinkingContent := getAssistantThinkingContent(line)
	if thinkingContent != "" {
		// Truncate thinking (can be very long)
		if len(thinkingContent) > 2000 {
			thinkingContent = thinkingContent[:2000] + "... [truncated]"
		}
		innerParts = append(innerParts, fmt.Sprintf("<thinking>%s</thinking>", thinkingContent))
	}

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
		counter++
		if line.UUID != "" {
			idMap[counter] = line.UUID
		}
		return fmt.Sprintf("<assistant id=\"%d\">\n%s\n</assistant>", counter, strings.Join(innerParts, "\n")), counter
	}

	return "", counter
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
	contentArray, ok := line.Message.Content.([]interface{})
	if !ok {
		return ""
	}

	var texts []string
	for _, item := range contentArray {
		blockMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if blockMap["type"] == "text" {
			if text, ok := blockMap["text"].(string); ok {
				texts = append(texts, text)
			}
		}
	}

	return strings.Join(texts, "\n")
}

// getAssistantThinkingContent extracts thinking content from an assistant message.
func getAssistantThinkingContent(line *TranscriptLine) string {
	if line.Message == nil || line.Message.Content == nil {
		return ""
	}

	contentArray, ok := line.Message.Content.([]interface{})
	if !ok {
		return ""
	}

	var thoughts []string
	for _, item := range contentArray {
		blockMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if blockMap["type"] == "thinking" {
			if thinking, ok := blockMap["thinking"].(string); ok {
				thoughts = append(thoughts, thinking)
			}
		}
	}

	return strings.Join(thoughts, "\n")
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

	// Truncate suggested_session_title if too long
	if len(result.SuggestedSessionTitle) > 100 {
		result.SuggestedSessionTitle = result.SuggestedSessionTitle[:100]
	}

	// Truncate arrays to max items and ensure non-nil for JSON serialization
	result.WentWell = truncateAnnotatedSlice(result.WentWell, 3)
	result.WentBad = truncateAnnotatedSlice(result.WentBad, 3)
	result.HumanSuggestions = truncateAnnotatedSlice(result.HumanSuggestions, 2)
	result.EnvironmentSuggestions = truncateAnnotatedSlice(result.EnvironmentSuggestions, 2)
	result.DefaultContextSuggestions = truncateAnnotatedSlice(result.DefaultContextSuggestions, 2)

	return &result, nil
}

// truncateAnnotatedSlice truncates an AnnotatedItem slice to maxLen and ensures a non-nil result
// for consistent JSON serialization ([] instead of null).
func truncateAnnotatedSlice(s []AnnotatedItem, maxLen int) []AnnotatedItem {
	if s == nil {
		return []AnnotatedItem{}
	}
	if len(s) > maxLen {
		return s[:maxLen]
	}
	return s
}

// resolveMessageIDs translates integer message_id values in the result to real UUIDs
// using the provided mapping. Invalid or missing IDs are cleared (text is kept).
func resolveMessageIDs(result *SmartRecapResult, idMap map[int]string) {
	lists := []*[]AnnotatedItem{
		&result.WentWell,
		&result.WentBad,
		&result.HumanSuggestions,
		&result.EnvironmentSuggestions,
		&result.DefaultContextSuggestions,
	}
	for _, list := range lists {
		for i := range *list {
			item := &(*list)[i]
			if item.MessageID == "" {
				continue
			}
			id, err := strconv.Atoi(item.MessageID)
			if err != nil {
				// Not a valid integer — clear it
				item.MessageID = ""
				continue
			}
			if uuid, ok := idMap[id]; ok {
				item.MessageID = uuid
			} else {
				// Integer not in mapping — clear it
				item.MessageID = ""
			}
		}
	}
}

const smartRecapSystemPrompt = `You are a highly expert software engineer with decades of experience working in the software industry. You have become highly proficient in using Claude Code for software engineering tasks. You have an in-depth understanding of software engineering best practices in general, and you know how to marry such understanding in the new world of Claude Code assisted engineering. You are a great communicator who explains complex concepts in simple terms and in an approachable tone.

You are analyzing a Claude Code session. The input contains:

1. <transcript> - The conversation in XML format:
   - Each element has a sequential integer id attribute for reference (e.g., <user id="1">, <assistant id="2">)
   - <user> tags for human messages (prompts from the user)
   - <skill> tags for skill expansions (instructions injected when skills like /commit are invoked)
   - <assistant> tags for Claude's responses, which may include:
     - <thinking> for Claude's reasoning process
     - <tools_called> listing tool names used
   - <tool_results> tags showing which tools succeeded or failed

2. <session_stats> - Computed analytics metrics (if available):
   - Token usage, costs, and cache hit rates
   - Session duration and compaction count
   - Conversation turn count and assistant utilization percentage
   - Code activity (files created/modified, lines added/removed)
   - Tool usage and error rates
   - Agent and skill invocations

Provide a high-signal analysis. Look for interesting patterns in both the transcript AND the stats.

Output ONLY valid JSON with these fields:
- suggested_session_title: Concise, descriptive title for this session (max 100 chars). Focus on the main task or outcome. Examples: "Add dark mode toggle to settings", "Debug OAuth login redirect loop", "Refactor API validation middleware"
- recap: Short 2-3 sentence recap of what occurred (plain text, no message references). If stats show notable patterns (e.g., high assistant utilization showing good flow, high cache hit rate showing efficiency, many tool errors), mention them briefly.
- went_well: Up to 3 objects of things that went well (omit or use empty array if none are clearly valid). Each item is {"text": "...", "message_id": N} where message_id is the integer id of the transcript element that best illustrates the point. Omit message_id if no specific message is relevant.
- went_bad: Up to 3 objects of things that did not go well (same format as went_well)
- human_suggestions: Up to 2 objects of human technique improvements (same format). Omit or use empty array if nothing stands out.
- environment_suggestions: Up to 2 objects of environment improvements (same format). Omit or use empty array if nothing stands out.
- default_context_suggestions: Up to 2 objects of CLAUDE.md/system context improvements (same format). These should be high-level general practices (e.g., "always run tests before committing"), NOT task-specific details (e.g., "when implementing OAuth, use PKCE flow"). Omit or use empty array if nothing stands out.

Guidelines:
- The session may still be in progress. Do not penalize workflows that appear incomplete or in-progress. Focus on what has happened so far rather than judging whether tasks were "finished."
- Keep lists very high signal. Better to omit an item than show something low-confidence.
- Suggestions should be concise and actionable. Don't prefix with "suggest" - they're already suggestions.
- Focus on what would actually improve future sessions.
- Note interesting stat patterns: high assistant utilization and cache hit rates are positive, high tool error rates suggest issues.
- Output ONLY the JSON object, no additional text.

Example output:
{
  "suggested_session_title": "Implement dark mode toggle feature",
  "recap": "User implemented a dark mode feature with 85% cache hit rate showing efficient context reuse. Tests were added and all passed after minor iteration.",
  "went_well": [{"text": "Clear initial requirements", "message_id": 1}, {"text": "High cache utilization"}, {"text": "Good iteration on feedback", "message_id": 12}],
  "went_bad": [{"text": "Multiple rounds needed to fix CSS specificity issues", "message_id": 5}],
  "human_suggestions": [{"text": "Include browser compatibility requirements upfront"}],
  "environment_suggestions": [],
  "default_context_suggestions": [{"text": "Document preferred testing patterns in CLAUDE.md"}]
}`
