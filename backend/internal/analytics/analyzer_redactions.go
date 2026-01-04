package analytics

import (
	"regexp"
)

// redactionPattern matches [REDACTED:TYPE] markers in strings.
// TYPE is captured in group 1 (must start with uppercase letter, then uppercase letters, digits, and underscores).
var redactionPattern = regexp.MustCompile(`\[REDACTED:([A-Z][A-Z0-9_]*)\]`)

// RedactionsResult contains redaction counts by type.
type RedactionsResult struct {
	TotalRedactions int
	RedactionCounts map[string]int // Type -> count (e.g., "GITHUB_TOKEN" -> 5)
}

// RedactionsAnalyzer extracts redaction counts from transcripts.
// It recursively walks the JSON structure of each line to find all
// [REDACTED:TYPE] markers in string values.
//
// Memory note: This analyzer uses TranscriptLine.RawData which stores the full
// parsed JSON alongside the typed struct, roughly doubling memory per line.
// If memory becomes an issue, consider a two-phase approach: run raw-bytes
// analyzers first, then parse into structs and discard raw bytes before
// running struct-based analyzers.
type RedactionsAnalyzer struct{}

// Analyze processes the file collection and returns redaction counts.
func (a *RedactionsAnalyzer) Analyze(fc *FileCollection) (*RedactionsResult, error) {
	result := &RedactionsResult{
		RedactionCounts: make(map[string]int),
	}

	// Process all files - main and agents
	for _, file := range fc.AllFiles() {
		for _, line := range file.Lines {
			if line.RawData != nil {
				a.walkValue(line.RawData, result)
			}
		}
	}

	return result, nil
}

// walkValue recursively walks a JSON value and counts redaction markers in strings.
// This mirrors the CLI's redactValueWithFieldContext pattern.
func (a *RedactionsAnalyzer) walkValue(v interface{}, result *RedactionsResult) {
	switch val := v.(type) {
	case string:
		a.countRedactionsInString(val, result)
	case map[string]interface{}:
		for _, v := range val {
			a.walkValue(v, result)
		}
	case []interface{}:
		for _, v := range val {
			a.walkValue(v, result)
		}
	// Numbers, bools, nil - nothing to scan
	}
}

// countRedactionsInString finds all [REDACTED:TYPE] markers in a string
// and updates the counts.
func (a *RedactionsAnalyzer) countRedactionsInString(s string, result *RedactionsResult) {
	matches := redactionPattern.FindAllStringSubmatch(s, -1)
	for _, match := range matches {
		if len(match) >= 2 {
			redactionType := match[1]
			// Skip "TYPE" - it's a documentation placeholder, not a real category
			if redactionType == "TYPE" {
				continue
			}
			result.RedactionCounts[redactionType]++
			result.TotalRedactions++
		}
	}
}
