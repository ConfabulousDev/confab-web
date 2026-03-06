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
type RedactionsAnalyzer struct {
	result RedactionsResult
}

// ProcessFile accumulates redaction counts from a single file.
func (a *RedactionsAnalyzer) ProcessFile(file *TranscriptFile, isMain bool) {
	if isMain {
		a.result.RedactionCounts = make(map[string]int)
	}

	for _, line := range file.Lines {
		if line.RawData != nil {
			a.walkValue(line.RawData)
		}
	}
}

// Finalize is a no-op for redactions.
func (a *RedactionsAnalyzer) Finalize(hasAgentFile func(string) bool) {}

// Result returns the accumulated redaction metrics.
func (a *RedactionsAnalyzer) Result() *RedactionsResult {
	return &a.result
}

// Analyze processes the file collection and returns redaction counts.
func (a *RedactionsAnalyzer) Analyze(fc *FileCollection) (*RedactionsResult, error) {
	a.ProcessFile(fc.Main, true)
	for _, agent := range fc.Agents {
		a.ProcessFile(agent, false)
	}
	a.Finalize(fc.HasAgentFile)
	return a.Result(), nil
}

// walkValue recursively walks a JSON value and counts redaction markers in strings.
func (a *RedactionsAnalyzer) walkValue(v interface{}) {
	switch val := v.(type) {
	case string:
		a.countRedactionsInString(val)
	case map[string]interface{}:
		for _, v := range val {
			a.walkValue(v)
		}
	case []interface{}:
		for _, v := range val {
			a.walkValue(v)
		}
	}
}

// countRedactionsInString finds all [REDACTED:TYPE] markers in a string.
func (a *RedactionsAnalyzer) countRedactionsInString(s string) {
	matches := redactionPattern.FindAllStringSubmatch(s, -1)
	for _, match := range matches {
		if len(match) >= 2 {
			redactionType := match[1]
			if redactionType == "TYPE" {
				continue
			}
			a.result.RedactionCounts[redactionType]++
			a.result.TotalRedactions++
		}
	}
}
