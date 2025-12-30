package analytics

import (
	"path/filepath"
	"strings"
)

// CodeActivityCollector extracts code activity metrics from transcript lines.
// It tracks file operations from Read, Write, Edit, Glob, and Grep tools.
type CodeActivityCollector struct {
	filesRead     map[string]bool // unique file paths read
	filesModified map[string]bool // unique file paths written/edited
	LinesAdded    int
	LinesRemoved  int
	SearchCount   int
	extensions    map[string]int // extension -> count (e.g., ".go" -> 5)
}

// NewCodeActivityCollector creates a new code activity collector.
func NewCodeActivityCollector() *CodeActivityCollector {
	return &CodeActivityCollector{
		filesRead:     make(map[string]bool),
		filesModified: make(map[string]bool),
		extensions:    make(map[string]int),
	}
}

// Collect processes a single line for code activity metrics.
func (c *CodeActivityCollector) Collect(line *TranscriptLine, ctx *CollectContext) {
	if !line.IsAssistantMessage() {
		return
	}

	for _, tool := range line.GetToolUses() {
		switch tool.Name {
		case "Read":
			if path := c.getFilePath(tool.Input); path != "" {
				c.filesRead[path] = true
				c.trackExtension(path)
			}

		case "Write":
			if path := c.getFilePath(tool.Input); path != "" {
				c.filesModified[path] = true
				c.trackExtension(path)
				// Count lines in new content
				if content, ok := tool.Input["content"].(string); ok {
					c.LinesAdded += countLines(content)
				}
			}

		case "Edit":
			if path := c.getFilePath(tool.Input); path != "" {
				c.filesModified[path] = true
				c.trackExtension(path)
				// Count old lines as removed, new lines as added (matches GitHub diff behavior)
				oldStr, _ := tool.Input["old_string"].(string)
				newStr, _ := tool.Input["new_string"].(string)
				c.LinesRemoved += countLines(oldStr)
				c.LinesAdded += countLines(newStr)
			}

		case "Glob", "Grep":
			c.SearchCount++
		}
	}
}

// Finalize is called after all lines are processed.
func (c *CodeActivityCollector) Finalize(ctx *CollectContext) {
	// No post-processing needed
}

// FilesRead returns the count of unique files read.
func (c *CodeActivityCollector) FilesRead() int {
	return len(c.filesRead)
}

// FilesModified returns the count of unique files modified.
func (c *CodeActivityCollector) FilesModified() int {
	return len(c.filesModified)
}

// LanguageBreakdown returns the extension breakdown with cleaned keys.
// Extensions are normalized: ".go" -> "go", ".tsx" -> "tsx"
func (c *CodeActivityCollector) LanguageBreakdown() map[string]int {
	result := make(map[string]int)
	for ext, count := range c.extensions {
		// Remove leading dot for cleaner display
		cleanExt := strings.TrimPrefix(ext, ".")
		if cleanExt != "" {
			result[cleanExt] = count
		}
	}
	return result
}

// getFilePath extracts the file_path from tool input.
func (c *CodeActivityCollector) getFilePath(input map[string]interface{}) string {
	if input == nil {
		return ""
	}
	if path, ok := input["file_path"].(string); ok {
		return path
	}
	return ""
}

// trackExtension records the file extension for language breakdown.
func (c *CodeActivityCollector) trackExtension(path string) {
	ext := filepath.Ext(path)
	if ext != "" {
		c.extensions[ext]++
	}
}

// countLines counts the number of lines in a string.
// Empty string returns 0, otherwise count newlines + 1.
// Trailing newlines are ignored (e.g., "hello\n" = 1 line, not 2).
func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(strings.TrimSuffix(s, "\n"), "\n") + 1
}
