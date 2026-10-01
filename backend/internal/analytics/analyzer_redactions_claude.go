package analytics

import "regexp"

// redactionPattern matches [REDACTED:TYPE] markers in strings.
// TYPE is captured in group 1 (must start with uppercase letter, then uppercase letters, digits, and underscores).
var redactionPattern = regexp.MustCompile(`\[REDACTED:([A-Z][A-Z0-9_]*)\]`)

// RedactionsResult contains redaction counts by type.
type RedactionsResult struct {
	TotalRedactions int
	RedactionCounts map[string]int // Type -> count (e.g., "GITHUB_TOKEN" -> 5)
}

// RedactionsAnalyzer sums the [REDACTED:TYPE] marker counts of each file.
// Counting happens at parse time (TranscriptFile.RedactionCounts, filled by
// parseTranscriptFile from the line's transient validation map), so parsed
// lines never retain a generic copy of their JSON.
type RedactionsAnalyzer struct {
	result RedactionsResult
}

// ProcessFile accumulates redaction counts from a single file.
func (a *RedactionsAnalyzer) ProcessFile(file *TranscriptFile, isMain bool) {
	if isMain {
		a.result.RedactionCounts = make(map[string]int)
	}

	for redactionType, n := range file.RedactionCounts {
		a.result.RedactionCounts[redactionType] += n
		a.result.TotalRedactions += n
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

// countRedactionsInValue recursively walks a decoded JSON value and adds every
// [REDACTED:TYPE] marker found in string values (not object keys) to counts.
// The literal placeholder [REDACTED:TYPE] is skipped. counts is allocated on
// the first marker found.
func countRedactionsInValue(v any, counts *map[string]int) {
	switch val := v.(type) {
	case string:
		countRedactionsInString(val, counts)
	case map[string]any:
		for _, elem := range val {
			countRedactionsInValue(elem, counts)
		}
	case []any:
		for _, elem := range val {
			countRedactionsInValue(elem, counts)
		}
	}
}

// countRedactionsInString adds the [REDACTED:TYPE] markers in s to counts.
func countRedactionsInString(s string, counts *map[string]int) {
	for _, match := range redactionPattern.FindAllStringSubmatch(s, -1) {
		redactionType := match[1]
		if redactionType == "TYPE" {
			continue
		}
		if *counts == nil {
			*counts = make(map[string]int)
		}
		(*counts)[redactionType]++
	}
}
