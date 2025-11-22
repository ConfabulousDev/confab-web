package redactor

import (
	"fmt"
	"regexp"
	"strings"
)

// Redactor handles redaction of sensitive data
type Redactor struct {
	patterns []compiledPattern
}

// compiledPattern represents a compiled regex pattern with metadata
type compiledPattern struct {
	regex        *regexp.Regexp
	patternType  string
	captureGroup int
}

// NewRedactor creates a new Redactor from a config
func NewRedactor(config Config) (*Redactor, error) {
	patterns := make([]compiledPattern, 0, len(config.Patterns))

	for _, p := range config.Patterns {
		regex, err := regexp.Compile(p.Pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile pattern '%s': %w", p.Name, err)
		}

		patterns = append(patterns, compiledPattern{
			regex:        regex,
			patternType:  p.Type,
			captureGroup: p.CaptureGroup,
		})
	}

	return &Redactor{
		patterns: patterns,
	}, nil
}

// Redact redacts sensitive data from a string
func (r *Redactor) Redact(input string) string {
	result := input

	for _, p := range r.patterns {
		if p.captureGroup > 0 {
			// Partial redaction using capture group
			result = r.redactCaptureGroup(result, p)
		} else {
			// Full match redaction
			result = r.redactFullMatch(result, p)
		}
	}

	return result
}

// RedactBytes redacts sensitive data from a byte slice
func (r *Redactor) RedactBytes(input []byte) []byte {
	return []byte(r.Redact(string(input)))
}

// redactFullMatch replaces the entire match with a redaction marker
func (r *Redactor) redactFullMatch(input string, p compiledPattern) string {
	marker := fmt.Sprintf("[REDACTED:%s]", strings.ToUpper(p.patternType))
	return p.regex.ReplaceAllString(input, marker)
}

// redactCaptureGroup replaces only the specified capture group
func (r *Redactor) redactCaptureGroup(input string, p compiledPattern) string {
	marker := fmt.Sprintf("[REDACTED:%s]", strings.ToUpper(p.patternType))

	return p.regex.ReplaceAllStringFunc(input, func(match string) string {
		submatches := p.regex.FindStringSubmatch(match)
		if len(submatches) <= p.captureGroup {
			// If capture group doesn't exist, return original match
			return match
		}

		// Replace the capture group with redaction marker
		result := match
		capturedText := submatches[p.captureGroup]

		// Find the position of the captured group in the match
		// and replace it with the marker
		idx := strings.Index(match, capturedText)
		if idx != -1 {
			result = match[:idx] + marker + match[idx+len(capturedText):]
		}

		return result
	})
}
