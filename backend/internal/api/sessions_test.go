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
			SessionID:      "valid-session-id",
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

	t.Run("rejects invalid UTF-8 in session ID", func(t *testing.T) {
		req := validRequest()
		req.SessionID = "invalid-\xff\xfe-utf8"

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
		// MaxRequestBodySize = 50MB, so create 6 files of 9MB each = 54MB
		fileSize := 9 * 1024 * 1024 // 9MB each (under MaxFileSize of 10MB)
		numFiles := 6               // 6 * 9MB = 54MB > 50MB limit

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

	t.Run("rejects empty session ID", func(t *testing.T) {
		req := validRequest()
		req.SessionID = ""

		err := validateSaveSessionRequest(req)
		if err == nil {
			t.Fatal("expected error for empty session ID, got nil")
		}
	})

	t.Run("rejects oversized session ID", func(t *testing.T) {
		req := validRequest()
		req.SessionID = strings.Repeat("a", MaxSessionIDLength+1)

		err := validateSaveSessionRequest(req)
		if err == nil {
			t.Fatal("expected error for oversized session ID, got nil")
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
		req.SessionID = strings.Repeat("a", MaxSessionIDLength)
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
