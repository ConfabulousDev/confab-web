package analytics

import (
	"fmt"
	"strings"
)

// PrepareTranscript converts the file collection into an XML format suitable for LLM analysis.
// Returns the XML string and a mapping from sequential integer IDs to message UUIDs.
func PrepareTranscript(fc *FileCollection) (string, map[int]string) {
	return PrepareTranscriptFromFiles(fc.AllFiles())
}

// PrepareTranscriptFromFiles converts transcript files into XML format for LLM analysis.
// Tool IDs are file-local, so each file is processed in a single pass: build toolNameMap
// while formatting lines. This allows incremental processing without holding all files in memory.
func PrepareTranscriptFromFiles(files []*TranscriptFile) (string, map[int]string) {
	config := DefaultFormatConfig()
	var sb strings.Builder
	idMap := make(map[int]string)
	counter := 0

	sb.WriteString("<transcript>\n")
	for _, file := range files {
		// Build tool name map for this file and format lines in a single pass.
		// First pass: collect tool names (needed for tool_result resolution).
		toolNameMap := buildToolNameMap(file)

		// Second pass: format lines.
		for _, line := range file.Lines {
			formatted, newCounter := formatLine(line, toolNameMap, counter, idMap, config)
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

// buildToolNameMap builds a tool_use_id -> tool_name map for a single file.
func buildToolNameMap(file *TranscriptFile) map[string]string {
	m := make(map[string]string)
	for _, line := range file.Lines {
		if line.IsAssistantMessage() {
			for _, tool := range line.GetToolUses() {
				if tool.ID != "" {
					if tool.Name == "Skill" {
						if skillName, ok := tool.Input["skill"].(string); ok && skillName != "" {
							m[tool.ID] = skillName
							continue
						}
					}
					m[tool.ID] = tool.Name
				}
			}
		}
	}
	return m
}

// TranscriptBuilder accumulates transcript XML incrementally across multiple files.
// Use ProcessFile for each transcript file, then call Finish to get the result.
type TranscriptBuilder struct {
	sb      strings.Builder
	idMap   map[int]string
	counter int
	files   int          // number of files processed
	config  FormatConfig // truncation limits
}

// NewTranscriptBuilder creates a TranscriptBuilder with the given format config.
func NewTranscriptBuilder(config FormatConfig) *TranscriptBuilder {
	return &TranscriptBuilder{config: config}
}

// ProcessFile adds all lines from a transcript file to the builder.
func (b *TranscriptBuilder) ProcessFile(file *TranscriptFile) {
	if b.files == 0 {
		b.idMap = make(map[int]string)
		b.sb.WriteString("<transcript>\n")
	}
	b.files++

	toolNameMap := buildToolNameMap(file)
	for _, line := range file.Lines {
		formatted, newCounter := formatLine(line, toolNameMap, b.counter, b.idMap, b.config)
		b.counter = newCounter
		if formatted != "" {
			b.sb.WriteString(formatted)
			b.sb.WriteString("\n")
		}
	}
}

// Finish closes the transcript tag and returns the XML string + ID map.
func (b *TranscriptBuilder) Finish() (string, map[int]string) {
	if b.files == 0 {
		return "", nil
	}
	b.sb.WriteString("</transcript>")
	return b.sb.String(), b.idMap
}

// formatLine converts a transcript line to XML format for the LLM.
// Returns the formatted string and the updated counter.
func formatLine(line *TranscriptLine, toolNameMap map[string]string, counter int, idMap map[int]string, config FormatConfig) (string, int) {
	switch line.Type {
	case "user":
		return formatUserLine(line, toolNameMap, counter, idMap, config)
	case "assistant":
		return formatAssistantLine(line, counter, idMap, config)
	default:
		return "", counter
	}
}

// formatUserLine formats a user message for the LLM in XML format.
// Returns the formatted string and the updated counter.
func formatUserLine(line *TranscriptLine, toolNameMap map[string]string, counter int, idMap map[int]string, config FormatConfig) (string, int) {
	// Check for skill expansion messages first (isMeta: true with sourceToolUseID)
	if line.IsSkillExpansionMessage() {
		content := getStringContent(line)
		if content != "" {
			counter++
			if line.UUID != "" {
				idMap[counter] = line.UUID
			}
			content = config.truncate(content, config.MaxSkillChars)
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
			content = config.truncate(content, config.MaxUserChars)
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
func formatAssistantLine(line *TranscriptLine, counter int, idMap map[int]string, config FormatConfig) (string, int) {
	if !line.IsAssistantMessage() {
		return "", counter
	}

	var innerParts []string

	// Get thinking content (shown by default in UI)
	thinkingContent := getAssistantThinkingContent(line)
	if thinkingContent != "" {
		thinkingContent = config.truncate(thinkingContent, config.MaxThinkingChars)
		innerParts = append(innerParts, fmt.Sprintf("<thinking>%s</thinking>", thinkingContent))
	}

	// Get text content
	textContent := getAssistantTextContent(line)
	if textContent != "" {
		textContent = config.truncate(textContent, config.MaxAssistantChars)
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
