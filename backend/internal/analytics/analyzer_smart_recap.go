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
	SuggestedSessionTitle     string   `json:"suggested_session_title"`
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

// Analyze generates a smart recap for the given transcript and analytics stats.
// cardStats contains the computed analytics cards (tokens, session, conversation, etc.)
// which are included in the prompt for additional context.
func (a *SmartRecapAnalyzer) Analyze(ctx context.Context, fc *FileCollection, cardStats map[string]interface{}) (*SmartRecapResult, error) {
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
	if contentLen > MaxTranscriptChars {
		// Truncate transcript portion, keep stats
		maxTranscript := MaxTranscriptChars - len(statsSection) - 100 // leave room for truncation message
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

	// Create the request with low temperature for consistent output
	temperature := 0.0
	resp, err := a.client.CreateMessage(ctx, &anthropic.MessagesRequest{
		Model:       a.model,
		MaxTokens:   MaxOutputTokens,
		Temperature: &temperature,
		System:      smartRecapSystemPrompt,
		Messages: []anthropic.Message{
			{Role: "user", Content: userContent},
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
			totalInputTokens := tokens.CacheRead + tokens.Input
			if totalInputTokens > 0 {
				cacheHitRate := float64(tokens.CacheRead) / float64(totalInputTokens) * 100
				sb.WriteString(fmt.Sprintf("    <cache_hit_rate_percent>%.1f</cache_hit_rate_percent>\n", cacheHitRate))
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
		if conv.AvgUserThinkingMs != nil && *conv.AvgUserThinkingMs > 0 {
			sb.WriteString(fmt.Sprintf("    <avg_user_response_seconds>%.1f</avg_user_response_seconds>\n", float64(*conv.AvgUserThinkingMs)/1000))
		}
		if conv.AssistantUtilization != nil {
			sb.WriteString(fmt.Sprintf("    <assistant_utilization_percent>%.1f</assistant_utilization_percent>\n", *conv.AssistantUtilization*100))
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
	// Check for skill expansion messages first (isMeta: true with sourceToolUseID)
	if line.IsSkillExpansionMessage() {
		content := getStringContent(line)
		if content != "" {
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
				return fmt.Sprintf("<skill name=\"%s\">\n%s\n</skill>", skillName, content)
			}
			return fmt.Sprintf("<skill>\n%s\n</skill>", content)
		}
		return ""
	}

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

You are analyzing a Claude Code session. The input contains:

1. <transcript> - The conversation in XML format:
   - <user> tags for human messages (prompts from the user)
   - <skill> tags for skill expansions (instructions injected when skills like /commit are invoked)
   - <assistant> tags for Claude's responses, which may include:
     - <thinking> for Claude's reasoning process
     - <tools_called> listing tool names used
   - <tool_results> tags showing which tools succeeded or failed

2. <session_stats> - Computed analytics metrics (if available):
   - Token usage, costs, and cache hit rates
   - Session duration and compaction count
   - Conversation turn count and user response latencies
   - Code activity (files created/modified, lines added/removed)
   - Tool usage and error rates
   - Agent and skill invocations

Provide a high-signal analysis. Look for interesting patterns in both the transcript AND the stats.

Output ONLY valid JSON with these fields:
- suggested_session_title: Concise, descriptive title for this session (max 100 chars). Focus on the main task or outcome. Examples: "Add dark mode toggle to settings", "Debug OAuth login redirect loop", "Refactor API validation middleware"
- recap: Short 2-3 sentence recap of what occurred. If stats show notable patterns (e.g., very long user latencies suggesting distraction, high cache hit rate showing efficiency, many tool errors), mention them briefly.
- went_well: Up to 3 things that went well (omit or use empty array if none are clearly valid)
- went_bad: Up to 3 things that did not go well (omit or use empty array if none are clearly valid)
- human_suggestions: Up to 3 human technique improvements (e.g., "provide more context in initial prompts")
- environment_suggestions: Up to 3 environment improvements (e.g., "speed up test suite")
- default_context_suggestions: Up to 3 CLAUDE.md/system context improvements

Guidelines:
- Keep lists very high signal. Better to omit an item than show something low-confidence.
- Suggestions should be concise and actionable. Don't prefix with "suggest" - they're already suggestions.
- Focus on what would actually improve future sessions.
- Note interesting stat patterns: high cache utilization is good, long user latencies may indicate confusion, high tool error rates suggest issues.
- Output ONLY the JSON object, no additional text.

Example output:
{
  "suggested_session_title": "Implement dark mode toggle feature",
  "recap": "User implemented a dark mode feature with 85% cache hit rate showing efficient context reuse. Tests were added and all passed after minor iteration.",
  "went_well": ["Clear initial requirements", "High cache utilization", "Good iteration on feedback"],
  "went_bad": ["Multiple rounds needed to fix CSS specificity issues"],
  "human_suggestions": ["Include browser compatibility requirements upfront"],
  "environment_suggestions": [],
  "default_context_suggestions": ["Add project's CSS architecture patterns to CLAUDE.md"]
}`
