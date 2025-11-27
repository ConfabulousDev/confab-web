package redactor

// Config represents the redaction configuration
type Config struct {
	Patterns []Pattern `json:"patterns"`
}

// Pattern represents a single redaction pattern.
//
// There are two modes of operation:
//
// 1. Value-based (FieldPattern empty): The Pattern regex is applied to all string
//    values in the JSON. Use this for secrets with distinctive formats that can
//    appear anywhere (API keys, tokens, etc.).
//
// 2. Field-based (FieldPattern set): Only values of fields whose names match
//    FieldPattern are redacted. The Pattern regex (if set) is applied to the
//    field value; if Pattern is empty, the entire value is redacted.
type Pattern struct {
	Name         string `json:"name"`
	Pattern      string `json:"pattern,omitempty"`
	Type         string `json:"type"`
	CaptureGroup int    `json:"capture_group,omitempty"`
	// FieldPattern is a regex that matches JSON field names. When set, only
	// values of matching fields are considered for redaction.
	FieldPattern string `json:"field_pattern,omitempty"`
}
