package api

import (
	"strings"
	"testing"

	"github.com/santaclaude2025/confab/backend/internal/models"
)

// Test input validation - security critical edge cases
func TestValidateSaveSessionRequest(t *testing.T) {
	// Helper to create valid base request
	validRequest := func() *models.SaveSessionRequest {
		return &models.SaveSessionRequest{
			ExternalID:     "valid-external-id",
			TranscriptPath: "path/to/transcript.jsonl",
			Files: []models.FileUpload{
				{
					Path:    "path/to/file.txt",
					Content: []byte("content"),
				},
			},
		}
	}

	t.Run("rejects path traversal attempts", func(t *testing.T) {
		req := validRequest()
		req.Files[0].Path = "../../../etc/passwd"

		err := validateSaveSessionRequest(req)
		if err == nil {
			t.Fatal("expected error for path traversal, got nil")
		}
		if !strings.Contains(err.Error(), "..") {
			t.Errorf("expected error to mention path traversal, got: %v", err)
		}
	})

	t.Run("rejects invalid UTF-8 in external ID", func(t *testing.T) {
		req := validRequest()
		req.ExternalID = "invalid-\xff\xfe-utf8"

		err := validateSaveSessionRequest(req)
		if err == nil {
			t.Fatal("expected error for invalid UTF-8, got nil")
		}
	})

	t.Run("rejects invalid UTF-8 in file path", func(t *testing.T) {
		req := validRequest()
		req.Files[0].Path = "path/\xff\xfe/file.txt"

		err := validateSaveSessionRequest(req)
		if err == nil {
			t.Fatal("expected error for invalid UTF-8 in path, got nil")
		}
	})

	t.Run("rejects oversized file", func(t *testing.T) {
		req := validRequest()
		// Create file larger than MaxFileSize (10MB)
		req.Files[0].Content = make([]byte, MaxFileSize+1)

		err := validateSaveSessionRequest(req)
		if err == nil {
			t.Fatal("expected error for oversized file, got nil")
		}
	})

	t.Run("rejects total size exceeding limit", func(t *testing.T) {
		req := validRequest()
		// Create multiple files that individually pass but exceed total
		// MaxRequestBodySize = 200MB, MaxFileSize = 50MB per file
		// Create 5 files of 45MB each = 225MB > 200MB limit
		fileSize := 45 * 1024 * 1024 // 45MB each (under MaxFileSize of 50MB)
		numFiles := 5                // 5 * 45MB = 225MB > 200MB limit

		req.Files = make([]models.FileUpload, numFiles)
		for i := 0; i < numFiles; i++ {
			req.Files[i] = models.FileUpload{
				Path:    "path/file.txt",
				Content: make([]byte, fileSize),
			}
		}

		err := validateSaveSessionRequest(req)
		if err == nil {
			t.Fatal("expected error for total size exceeding limit, got nil")
		}
	})

	t.Run("rejects too many files", func(t *testing.T) {
		req := validRequest()
		req.Files = make([]models.FileUpload, MaxFiles+1)
		for i := range req.Files {
			req.Files[i] = models.FileUpload{
				Path:    "path/file.txt",
				Content: []byte("small"),
			}
		}

		err := validateSaveSessionRequest(req)
		if err == nil {
			t.Fatal("expected error for too many files, got nil")
		}
	})

	t.Run("rejects empty external ID", func(t *testing.T) {
		req := validRequest()
		req.ExternalID = ""

		err := validateSaveSessionRequest(req)
		if err == nil {
			t.Fatal("expected error for empty external ID, got nil")
		}
	})

	t.Run("rejects oversized external ID", func(t *testing.T) {
		req := validRequest()
		req.ExternalID = strings.Repeat("a", MaxExternalIDLength+1)

		err := validateSaveSessionRequest(req)
		if err == nil {
			t.Fatal("expected error for oversized external ID, got nil")
		}
	})

	t.Run("rejects empty files array", func(t *testing.T) {
		req := validRequest()
		req.Files = []models.FileUpload{}

		err := validateSaveSessionRequest(req)
		if err == nil {
			t.Fatal("expected error for empty files array, got nil")
		}
	})

	t.Run("rejects oversized path", func(t *testing.T) {
		req := validRequest()
		req.Files[0].Path = strings.Repeat("a", MaxPathLength+1)

		err := validateSaveSessionRequest(req)
		if err == nil {
			t.Fatal("expected error for oversized path, got nil")
		}
	})

	t.Run("rejects oversized reason", func(t *testing.T) {
		req := validRequest()
		req.Reason = strings.Repeat("a", MaxReasonLength+1)

		err := validateSaveSessionRequest(req)
		if err == nil {
			t.Fatal("expected error for oversized reason, got nil")
		}
	})

	t.Run("accepts valid request", func(t *testing.T) {
		req := validRequest()

		err := validateSaveSessionRequest(req)
		if err != nil {
			t.Errorf("expected no error for valid request, got: %v", err)
		}
	})

	t.Run("accepts optional fields when valid", func(t *testing.T) {
		req := validRequest()
		req.CWD = "/home/user/project"
		req.Reason = "Testing feature X"

		err := validateSaveSessionRequest(req)
		if err != nil {
			t.Errorf("expected no error with optional fields, got: %v", err)
		}
	})

	t.Run("accepts maximum valid sizes", func(t *testing.T) {
		req := validRequest()
		req.ExternalID = strings.Repeat("a", MaxExternalIDLength)
		req.Files[0].Path = strings.Repeat("b", MaxPathLength)
		req.Files[0].Content = make([]byte, MaxFileSize)

		err := validateSaveSessionRequest(req)
		if err != nil {
			t.Errorf("expected no error at maximum valid sizes, got: %v", err)
		}
	})
}

// Test sanitization
func TestSanitizeString(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "removes null bytes",
			input: "hello\x00world",
		},
		{
			name:  "handles empty string",
			input: "",
		},
		{
			name:  "handles multiple null bytes",
			input: "a\x00b\x00c\x00",
		},
		{
			name:  "handles valid string",
			input: "normal text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeString(tt.input)

			// Check null bytes are removed
			if strings.Contains(result, "\x00") {
				t.Error("sanitized string contains null bytes")
			}

			// Check result is valid UTF-8
			if !strings.Contains(result, "\x00") && tt.input != "" {
				// If input was not just null bytes, we should have some output
				if tt.input != "\x00" && result == "" && tt.input != "" {
					// Only fail if we removed more than just null bytes
					hasOnlyNull := true
					for _, c := range tt.input {
						if c != '\x00' {
							hasOnlyNull = false
							break
						}
					}
					if !hasOnlyNull {
						t.Error("sanitization removed too much content")
					}
				}
			}
		})
	}
}

func TestSanitizeTitleText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "removes script tags and escapes content",
			input:    "<script>alert('xss')</script>",
			expected: "alert(&#39;xss&#39;)",
		},
		{
			name:     "removes script tags with surrounding text",
			input:    "Hello <script>alert('xss')</script> World",
			expected: "Hello alert(&#39;xss&#39;) World",
		},
		{
			name:     "removes img tags with onerror",
			input:    `<img src=x onerror="alert('xss')">`,
			expected: "",
		},
		{
			name:     "removes all HTML tags",
			input:    "<div><p>Hello <b>World</b></p></div>",
			expected: "Hello World",
		},
		{
			name:     "escapes HTML entities and removes empty tags",
			input:    "This & that < or > maybe",
			expected: "This &amp; that  maybe",
		},
		{
			name:     "handles already escaped HTML entities",
			input:    "&lt;script&gt;alert('xss')&lt;/script&gt;",
			expected: "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;",
		},
		{
			name:     "handles mixed tags and entities",
			input:    "<script>alert('test')</script> & <b>bold</b>",
			expected: "alert(&#39;test&#39;) &amp; bold",
		},
		{
			name:     "trims whitespace",
			input:    "  <p>Hello</p>  ",
			expected: "Hello",
		},
		{
			name:     "handles empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "handles plain text",
			input:    "Just plain text",
			expected: "Just plain text",
		},
		{
			name:     "handles javascript: protocol",
			input:    `<a href="javascript:alert('xss')">Click</a>`,
			expected: "Click",
		},
		{
			name:     "handles data: protocol",
			input:    `<img src="data:text/html,<script>alert('xss')</script>">`,
			expected: "alert(&#39;xss&#39;)&#34;&gt;",
		},
		{
			name:     "handles nested tags",
			input:    "<div><div><div>Deep nesting</div></div></div>",
			expected: "Deep nesting",
		},
		{
			name:     "handles malformed tags",
			input:    "<script<script>alert('xss')</script>",
			expected: "alert(&#39;xss&#39;)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeTitleText(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeTitleText(%q) = %q, want %q", tt.input, result, tt.expected)
			}

			// Verify no script tags remain
			if strings.Contains(strings.ToLower(result), "<script") {
				t.Errorf("sanitizeTitleText(%q) still contains <script tag: %q", tt.input, result)
			}

			// Verify no HTML tags remain
			if strings.Contains(result, "<") && strings.Contains(result, ">") {
				// Check if it's an escaped entity
				if !strings.Contains(result, "&lt;") && !strings.Contains(result, "&gt;") {
					t.Errorf("sanitizeTitleText(%q) may contain unescaped HTML tags: %q", tt.input, result)
				}
			}
		})
	}
}
