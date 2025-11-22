package redactor

// Config represents the redaction configuration
type Config struct {
	Patterns []Pattern `json:"patterns"`
}

// Pattern represents a single redaction pattern
type Pattern struct {
	Name         string `json:"name"`
	Pattern      string `json:"pattern"`
	Type         string `json:"type"`
	CaptureGroup int    `json:"capture_group,omitempty"`
}
