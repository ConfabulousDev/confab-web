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
type CodeActivityAnalyzer struct {
	filesRead    map[string]bool
	filesModified map[string]bool
	extensions   map[string]int
	linesAdded   int
	linesRemoved int
	searchCount  int
}

// ProcessFile accumulates code activity from a single file.
func (a *CodeActivityAnalyzer) ProcessFile(file *TranscriptFile, isMain bool) {
	if isMain {
		a.filesRead = make(map[string]bool)
		a.filesModified = make(map[string]bool)
		a.extensions = make(map[string]int)
	}

	for _, line := range file.Lines {
		if !line.IsAssistantMessage() {
			continue
		}

		for _, tool := range line.GetToolUses() {
			switch tool.Name {
			case "Read":
				if path := getFilePath(tool.Input); path != "" {
					a.filesRead[path] = true
					trackExtension(path, a.extensions)
				}

			case "Write":
				if path := getFilePath(tool.Input); path != "" {
					a.filesModified[path] = true
					trackExtension(path, a.extensions)
					if content, ok := tool.Input["content"].(string); ok {
						a.linesAdded += countLines(content)
					}
				}

			case "Edit":
				if path := getFilePath(tool.Input); path != "" {
					a.filesModified[path] = true
					trackExtension(path, a.extensions)
					oldStr, _ := tool.Input["old_string"].(string)
					newStr, _ := tool.Input["new_string"].(string)
					a.linesRemoved += countLines(oldStr)
					a.linesAdded += countLines(newStr)
				}

			case "Glob", "Grep":
				a.searchCount++
			}
		}
	}
}

// Finalize builds the final result.
func (a *CodeActivityAnalyzer) Finalize(hasAgentFile func(string) bool) {}

// Result returns the accumulated code activity metrics.
func (a *CodeActivityAnalyzer) Result() *CodeActivityResult {
	languageBreakdown := make(map[string]int)
	for ext, count := range a.extensions {
		cleanExt := strings.TrimPrefix(ext, ".")
		if cleanExt != "" {
			languageBreakdown[cleanExt] = count
		}
	}

	return &CodeActivityResult{
		FilesRead:         len(a.filesRead),
		FilesModified:     len(a.filesModified),
		LinesAdded:        a.linesAdded,
		LinesRemoved:      a.linesRemoved,
		SearchCount:       a.searchCount,
		LanguageBreakdown: languageBreakdown,
	}
}

// Analyze processes the file collection and returns code activity metrics.
func (a *CodeActivityAnalyzer) Analyze(fc *FileCollection) (*CodeActivityResult, error) {
	a.ProcessFile(fc.Main, true)
	for _, agent := range fc.Agents {
		a.ProcessFile(agent, false)
	}
	a.Finalize(fc.HasAgentFile)
	return a.Result(), nil
}

// getFilePath extracts the file_path from tool input.
func getFilePath(input map[string]interface{}) string {
	path, _ := input["file_path"].(string)
	return path
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
