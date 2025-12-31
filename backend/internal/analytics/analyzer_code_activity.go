package analytics

import (
	"path/filepath"
	"strings"
)

// CodeActivityResult contains code activity metrics.
type CodeActivityResult struct {
	FilesRead         int
	FilesModified     int
	LinesAdded        int
	LinesRemoved      int
	SearchCount       int
	LanguageBreakdown map[string]int
}

// CodeActivityAnalyzer extracts code activity metrics from transcripts.
// It tracks file operations from Read, Write, Edit, Glob, and Grep tools.
// It processes all files (main + agents) to get complete activity.
type CodeActivityAnalyzer struct{}

// Analyze processes the file collection and returns code activity metrics.
func (a *CodeActivityAnalyzer) Analyze(fc *FileCollection) (*CodeActivityResult, error) {
	// Use maps to track unique files
	filesRead := make(map[string]bool)
	filesModified := make(map[string]bool)
	extensions := make(map[string]int)
	var linesAdded, linesRemoved, searchCount int

	// Process all files - main and agents
	for _, file := range fc.AllFiles() {
		a.processFile(file, filesRead, filesModified, extensions, &linesAdded, &linesRemoved, &searchCount)
	}

	// Build language breakdown with cleaned extensions
	languageBreakdown := make(map[string]int)
	for ext, count := range extensions {
		cleanExt := strings.TrimPrefix(ext, ".")
		if cleanExt != "" {
			languageBreakdown[cleanExt] = count
		}
	}

	return &CodeActivityResult{
		FilesRead:         len(filesRead),
		FilesModified:     len(filesModified),
		LinesAdded:        linesAdded,
		LinesRemoved:      linesRemoved,
		SearchCount:       searchCount,
		LanguageBreakdown: languageBreakdown,
	}, nil
}

// processFile processes a single transcript file for code activity metrics.
func (a *CodeActivityAnalyzer) processFile(
	file *TranscriptFile,
	filesRead map[string]bool,
	filesModified map[string]bool,
	extensions map[string]int,
	linesAdded *int,
	linesRemoved *int,
	searchCount *int,
) {
	for _, line := range file.Lines {
		if !line.IsAssistantMessage() {
			continue
		}

		for _, tool := range line.GetToolUses() {
			switch tool.Name {
			case "Read":
				if path := getFilePath(tool.Input); path != "" {
					filesRead[path] = true
					trackExtension(path, extensions)
				}

			case "Write":
				if path := getFilePath(tool.Input); path != "" {
					filesModified[path] = true
					trackExtension(path, extensions)
					// Count lines in new content
					if content, ok := tool.Input["content"].(string); ok {
						*linesAdded += countLines(content)
					}
				}

			case "Edit":
				if path := getFilePath(tool.Input); path != "" {
					filesModified[path] = true
					trackExtension(path, extensions)
					// Count old lines as removed, new lines as added
					oldStr, _ := tool.Input["old_string"].(string)
					newStr, _ := tool.Input["new_string"].(string)
					*linesRemoved += countLines(oldStr)
					*linesAdded += countLines(newStr)
				}

			case "Glob", "Grep":
				*searchCount++
			}
		}
	}
}

// getFilePath extracts the file_path from tool input.
func getFilePath(input map[string]interface{}) string {
	if input == nil {
		return ""
	}
	if path, ok := input["file_path"].(string); ok {
		return path
	}
	return ""
}

// trackExtension records the file extension for language breakdown.
func trackExtension(path string, extensions map[string]int) {
	ext := filepath.Ext(path)
	if ext != "" {
		extensions[ext]++
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
