package redactor

import (
	"bufio"
	"bytes"
	"encoding/json"
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
	fieldRegex   *regexp.Regexp // nil means apply to all string values
	patternType  string
	captureGroup int
}

// NewRedactor creates a new Redactor from a config
func NewRedactor(config Config) (*Redactor, error) {
	patterns := make([]compiledPattern, 0, len(config.Patterns))

	for _, p := range config.Patterns {
		cp := compiledPattern{
			patternType:  p.Type,
			captureGroup: p.CaptureGroup,
		}

		// Compile value pattern if provided
		if p.Pattern != "" {
			regex, err := regexp.Compile(p.Pattern)
			if err != nil {
				return nil, fmt.Errorf("failed to compile pattern '%s': %w", p.Name, err)
			}
			cp.regex = regex
		}

		// Compile field pattern if provided
		if p.FieldPattern != "" {
			fieldRegex, err := regexp.Compile(p.FieldPattern)
			if err != nil {
				return nil, fmt.Errorf("failed to compile field pattern '%s': %w", p.Name, err)
			}
			cp.fieldRegex = fieldRegex
		}

		// Validate: must have at least one of Pattern or FieldPattern
		if cp.regex == nil && cp.fieldRegex == nil {
			return nil, fmt.Errorf("pattern '%s' must have either pattern or field_pattern", p.Name)
		}

		patterns = append(patterns, cp)
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

// RedactJSONL redacts sensitive data from JSONL content by parsing each line,
// recursively redacting string values, and re-serializing. This ensures JSON
// structure is never corrupted by redaction patterns.
func (r *Redactor) RedactJSONL(input []byte) []byte {
	var result bytes.Buffer
	scanner := bufio.NewScanner(bytes.NewReader(input))

	// Handle large lines (transcripts can have big content blocks)
	const maxLineSize = 10 * 1024 * 1024 // 10MB
	scanner.Buffer(make([]byte, 64*1024), maxLineSize)

	first := true
	for scanner.Scan() {
		line := scanner.Bytes()

		// Preserve empty lines as-is
		if len(bytes.TrimSpace(line)) == 0 {
			if !first {
				result.WriteByte('\n')
			}
			first = false
			continue
		}

		// Parse JSON
		var data interface{}
		if err := json.Unmarshal(line, &data); err != nil {
			// If parsing fails, fall back to text-based redaction
			if !first {
				result.WriteByte('\n')
			}
			result.Write([]byte(r.Redact(string(line))))
			first = false
			continue
		}

		// Recursively redact string values
		redacted := r.redactValue(data)

		// Re-serialize
		output, err := json.Marshal(redacted)
		if err != nil {
			// Shouldn't happen, but fall back to original if it does
			if !first {
				result.WriteByte('\n')
			}
			result.Write(line)
			first = false
			continue
		}

		if !first {
			result.WriteByte('\n')
		}
		result.Write(output)
		first = false
	}

	return result.Bytes()
}

// RedactJSONLine redacts a single JSON line, parsing it and applying redaction
// to string values only. Returns the redacted JSON. If the input is not valid
// JSON, falls back to text-based redaction.
func (r *Redactor) RedactJSONLine(line string) string {
	// Try to parse as JSON
	var data interface{}
	if err := json.Unmarshal([]byte(line), &data); err != nil {
		// Not valid JSON, fall back to text-based redaction (value patterns only)
		return r.redactTextValuePatternsOnly(line)
	}

	// Recursively redact string values
	redacted := r.redactValueWithFieldContext(data, "")

	// Re-serialize
	output, err := json.Marshal(redacted)
	if err != nil {
		// Shouldn't happen, but fall back to original if it does
		return line
	}

	return string(output)
}

// redactTextValuePatternsOnly applies only value-based patterns (no field patterns)
// to plain text. This is the safe fallback for non-JSON content.
func (r *Redactor) redactTextValuePatternsOnly(input string) string {
	result := input
	for _, p := range r.patterns {
		// Skip field-based patterns for text mode
		if p.fieldRegex != nil {
			continue
		}
		if p.regex == nil {
			continue
		}
		if p.captureGroup > 0 {
			result = r.redactCaptureGroup(result, p)
		} else {
			result = r.redactFullMatch(result, p)
		}
	}
	return result
}

// redactValueWithFieldContext recursively redacts string values in a JSON structure,
// tracking the current field name for field-based pattern matching.
func (r *Redactor) redactValueWithFieldContext(v interface{}, fieldName string) interface{} {
	switch val := v.(type) {
	case string:
		return r.redactStringValue(val, fieldName)
	case map[string]interface{}:
		result := make(map[string]interface{}, len(val))
		for k, v := range val {
			result[k] = r.redactValueWithFieldContext(v, k)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, v := range val {
			// Array elements inherit parent field name for field-based matching
			result[i] = r.redactValueWithFieldContext(v, fieldName)
		}
		return result
	default:
		// Numbers, bools, null - return as-is
		return val
	}
}

// redactStringValue applies redaction patterns to a string value, considering
// both value-based and field-based patterns.
func (r *Redactor) redactStringValue(value, fieldName string) string {
	result := value

	for _, p := range r.patterns {
		if p.fieldRegex != nil {
			// Field-based pattern: only apply if field name matches
			if fieldName == "" || !p.fieldRegex.MatchString(fieldName) {
				continue
			}
			// Field matches - redact the value
			if p.regex != nil {
				// Apply value regex to matching field
				if p.captureGroup > 0 {
					result = r.redactCaptureGroup(result, p)
				} else {
					result = r.redactFullMatch(result, p)
				}
			} else {
				// No value regex - redact entire value
				result = fmt.Sprintf("[REDACTED:%s]", strings.ToUpper(p.patternType))
			}
		} else if p.regex != nil {
			// Value-based pattern: apply to all string values
			if p.captureGroup > 0 {
				result = r.redactCaptureGroup(result, p)
			} else {
				result = r.redactFullMatch(result, p)
			}
		}
	}

	return result
}

// redactValue recursively redacts string values in a JSON structure
// Deprecated: Use redactValueWithFieldContext for field-aware redaction
func (r *Redactor) redactValue(v interface{}) interface{} {
	return r.redactValueWithFieldContext(v, "")
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
