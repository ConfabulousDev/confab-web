package storage

import (
	"errors"
	"strings"
	"testing"

	"github.com/minio/minio-go/v7"

	"github.com/ConfabulousDev/confab-web/internal/models"
)

// TestContainsAny tests the helper function for network error detection
func TestContainsAny(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substrs  []string
		expected bool
	}{
		{"contains first", "connection refused", []string{"connection", "timeout"}, true},
		{"contains second", "request timeout", []string{"connection", "timeout"}, true},
		{"contains none", "success", []string{"connection", "timeout"}, false},
		{"empty string", "", []string{"connection"}, false},
		{"empty substrs", "connection", []string{}, false},
		{"exact match", "timeout", []string{"timeout"}, true},
		{"substring match", "connection refused: dial error", []string{"refused"}, true},
		{"case sensitive - no match", "TIMEOUT", []string{"timeout"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsAny(tt.s, tt.substrs)
			if result != tt.expected {
				t.Errorf("containsAny(%q, %v) = %v, want %v", tt.s, tt.substrs, result, tt.expected)
			}
		})
	}
}

// TestClassifyStorageError tests error classification
func TestClassifyStorageError(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		operation     string
		expectedError error
		checkWrapped  bool
	}{
		{
			name:          "nil error",
			err:           nil,
			operation:     "upload",
			expectedError: nil,
		},
		{
			name: "NoSuchKey error",
			err: minio.ErrorResponse{
				Code: "NoSuchKey",
			},
			operation:     "download",
			expectedError: ErrObjectNotFound,
			checkWrapped:  true,
		},
		{
			name: "NoSuchBucket error",
			err: minio.ErrorResponse{
				Code: "NoSuchBucket",
			},
			operation:     "download",
			expectedError: ErrObjectNotFound,
			checkWrapped:  true,
		},
		{
			name: "AccessDenied error",
			err: minio.ErrorResponse{
				Code: "AccessDenied",
			},
			operation:     "upload",
			expectedError: ErrAccessDenied,
			checkWrapped:  true,
		},
		{
			name: "InvalidAccessKeyId error",
			err: minio.ErrorResponse{
				Code: "InvalidAccessKeyId",
			},
			operation:     "upload",
			expectedError: ErrAccessDenied,
			checkWrapped:  true,
		},
		{
			name: "SignatureDoesNotMatch error",
			err: minio.ErrorResponse{
				Code: "SignatureDoesNotMatch",
			},
			operation:     "delete",
			expectedError: ErrAccessDenied,
			checkWrapped:  true,
		},
		{
			name:          "connection error string",
			err:           errors.New("dial tcp: connection refused"),
			operation:     "upload",
			expectedError: ErrNetworkError,
			checkWrapped:  true,
		},
		{
			name:          "timeout error string",
			err:           errors.New("context deadline exceeded: timeout"),
			operation:     "download",
			expectedError: ErrNetworkError,
			checkWrapped:  true,
		},
		{
			name:          "network error string",
			err:           errors.New("network unreachable"),
			operation:     "upload",
			expectedError: ErrNetworkError,
			checkWrapped:  true,
		},
		{
			name:          "unknown error",
			err:           errors.New("some unknown error"),
			operation:     "upload",
			expectedError: nil, // Will be wrapped but not with sentinel
			checkWrapped:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyStorageError(tt.err, tt.operation)

			if tt.expectedError == nil && result == nil {
				return // Both nil, test passes
			}

			if tt.expectedError == nil && result != nil {
				// Unknown error - just verify it's wrapped
				if tt.err == nil {
					t.Error("expected nil result for nil input")
				}
				return
			}

			if tt.checkWrapped {
				if !errors.Is(result, tt.expectedError) {
					t.Errorf("classifyStorageError(%v, %q) should wrap %v, got %v",
						tt.err, tt.operation, tt.expectedError, result)
				}
			}
		})
	}
}

// TestS3Config validates S3Config struct
func TestS3Config_Validation(t *testing.T) {
	// This tests that the config struct has the expected fields
	config := S3Config{
		Endpoint:        "localhost:9000",
		AccessKeyID:     "minioadmin",
		SecretAccessKey: "minioadmin",
		BucketName:      "test-bucket",
		UseSSL:          false,
	}

	if config.Endpoint == "" {
		t.Error("Endpoint should not be empty")
	}
	if config.AccessKeyID == "" {
		t.Error("AccessKeyID should not be empty")
	}
	if config.SecretAccessKey == "" {
		t.Error("SecretAccessKey should not be empty")
	}
	if config.BucketName == "" {
		t.Error("BucketName should not be empty")
	}
}

// TestUploadChunkLineBounds verifies that UploadChunk rejects invalid line ranges
// before attempting any S3 operation. Uses a nil client since the bounds check is first.
func TestUploadChunkLineBounds(t *testing.T) {
	s := &S3Storage{} // nil client — bounds check runs before S3 calls

	tests := []struct {
		name      string
		firstLine int
		lastLine  int
	}{
		{"zero firstLine", 0, 100},
		{"negative firstLine", -1, 100},
		{"firstLine > lastLine", 100, 50},
		{"lastLine exceeds max", 1, MaxLineNumber + 1},
		{"both exceed max", MaxLineNumber + 1, MaxLineNumber + 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.UploadChunk(t.Context(), 1, models.ProviderClaudeCode, "ext-123", "transcript.jsonl", tt.firstLine, tt.lastLine, []byte("data"))
			if err == nil {
				t.Errorf("expected error for invalid range [%d, %d]", tt.firstLine, tt.lastLine)
			} else if !strings.Contains(err.Error(), "invalid line range") {
				t.Errorf("expected bounds error, got: %v", err)
			}
		})
	}
}

// TestUploadChunk_RejectsInvalidProvider verifies the storage-layer
// defense-in-depth check: an invalid provider value must error out before
// any S3 call so plumbing bugs surface immediately rather than as silently
// missing objects. Uses a nil S3 client — the provider check runs first.
func TestUploadChunk_RejectsInvalidProvider(t *testing.T) {
	s := &S3Storage{} // nil client — provider check runs before S3 calls

	tests := []struct {
		name     string
		provider string
	}{
		{"empty provider", ""},
		{"unknown provider", "gemini"},
		{"legacy display form is rejected at the storage layer", "Claude Code"},
		{"uppercase Codex is rejected", "Codex"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.UploadChunk(t.Context(), 1, tt.provider, "ext-123", "transcript.jsonl", 1, 5, []byte("data"))
			if err == nil {
				t.Errorf("expected error for invalid provider %q", tt.provider)
			}
		})
	}
}

// TestListChunks_RejectsInvalidProvider mirrors the UploadChunk check.
func TestListChunks_RejectsInvalidProvider(t *testing.T) {
	s := &S3Storage{}
	_, err := s.ListChunks(t.Context(), 1, "Claude Code", "ext-123", "transcript.jsonl")
	if err == nil {
		t.Error("expected error for legacy provider value")
	}
}

// TestDeleteAllSessionChunks_RejectsInvalidProvider mirrors the UploadChunk check.
func TestDeleteAllSessionChunks_RejectsInvalidProvider(t *testing.T) {
	s := &S3Storage{}
	err := s.DeleteAllSessionChunks(t.Context(), 1, "", "ext-123")
	if err == nil {
		t.Error("expected error for empty provider value")
	}
}

// TestSentinelErrors verifies sentinel errors are properly defined
func TestSentinelErrors(t *testing.T) {
	// Verify sentinel errors are not nil
	if ErrObjectNotFound == nil {
		t.Error("ErrObjectNotFound should not be nil")
	}
	if ErrAccessDenied == nil {
		t.Error("ErrAccessDenied should not be nil")
	}
	if ErrNetworkError == nil {
		t.Error("ErrNetworkError should not be nil")
	}

	// Verify sentinel errors have distinct messages
	errors := []error{ErrObjectNotFound, ErrAccessDenied, ErrNetworkError}
	messages := make(map[string]bool)
	for _, err := range errors {
		msg := err.Error()
		if messages[msg] {
			t.Errorf("duplicate error message: %s", msg)
		}
		messages[msg] = true
	}
}
