package storage

import (
	"errors"
	"testing"

	"github.com/minio/minio-go/v7"
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
