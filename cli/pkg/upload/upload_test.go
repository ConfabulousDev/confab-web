package upload

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/santaclaude2025/confab/pkg/config"
	"github.com/santaclaude2025/confab/pkg/types"
)

func TestReadFilesForUpload(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create test files
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.json")
	os.WriteFile(file1, []byte("test content 1"), 0644)
	os.WriteFile(file2, []byte(`{"key": "value"}`), 0644)

	tests := []struct {
		name    string
		files   []types.SessionFile
		wantErr bool
		wantLen int
	}{
		{
			name: "read multiple files",
			files: []types.SessionFile{
				{Path: file1, Type: "transcript", SizeBytes: 14},
				{Path: file2, Type: "todo", SizeBytes: 16},
			},
			wantErr: false,
			wantLen: 2,
		},
		{
			name: "empty file list",
			files: []types.SessionFile{},
			wantErr: false,
			wantLen: 0,
		},
		{
			name: "nonexistent file",
			files: []types.SessionFile{
				{Path: filepath.Join(tmpDir, "nonexistent.txt"), Type: "transcript", SizeBytes: 0},
			},
			wantErr: true,
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uploads, err := ReadFilesForUpload(tt.files)

			if (err != nil) != tt.wantErr {
				t.Errorf("ReadFilesForUpload() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(uploads) != tt.wantLen {
					t.Errorf("ReadFilesForUpload() returned %d files, want %d", len(uploads), tt.wantLen)
				}

				// Verify content for successful reads
				if tt.wantLen > 0 {
					if string(uploads[0].Content) != "test content 1" {
						t.Errorf("First file content = %s, want 'test content 1'", string(uploads[0].Content))
					}
				}
			}
		})
	}
}

func TestSendSessionRequest_Success(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		if r.Header.Get("Content-Encoding") != "zstd" {
			t.Errorf("Expected Content-Encoding: zstd, got %s", r.Header.Get("Content-Encoding"))
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("Expected Authorization header with test-api-key")
		}

		// Read and decompress body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("Failed to read request body: %v", err)
		}

		// Decompress
		decoder, err := zstd.NewReader(bytes.NewReader(body))
		if err != nil {
			t.Fatalf("Failed to create zstd decoder: %v", err)
		}
		defer decoder.Close()

		decompressed, err := io.ReadAll(decoder)
		if err != nil {
			t.Fatalf("Failed to decompress: %v", err)
		}

		// Verify it's valid JSON
		var req SaveSessionRequest
		if err := json.Unmarshal(decompressed, &req); err != nil {
			t.Fatalf("Failed to unmarshal request: %v", err)
		}

		if req.SessionID != "test-session-123" {
			t.Errorf("Expected SessionID test-session-123, got %s", req.SessionID)
		}

		// Send success response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(SaveSessionResponse{
			Success:   true,
			SessionID: "test-session-123",
			RunID:     42,
		})
	}))
	defer server.Close()

	cfg := &config.UploadConfig{
		BackendURL: server.URL,
		APIKey:     "test-api-key",
	}

	request := &SaveSessionRequest{
		SessionID:      "test-session-123",
		TranscriptPath: "/test/path",
		CWD:            "/test",
		Reason:         "test",
		Files: []FileUpload{
			{Path: "/test/file", Type: "transcript", SizeBytes: 100, Content: []byte("test")},
		},
	}

	err := SendSessionRequest(cfg, request)
	if err != nil {
		t.Fatalf("SendSessionRequest() failed: %v", err)
	}
}

func TestSendSessionRequest_HTTPError(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Invalid request"))
	}))
	defer server.Close()

	cfg := &config.UploadConfig{
		BackendURL: server.URL,
		APIKey:     "test-api-key",
	}

	request := &SaveSessionRequest{
		SessionID: "test-session-123",
		Files:     []FileUpload{},
	}

	err := SendSessionRequest(cfg, request)
	if err == nil {
		t.Fatal("Expected error for HTTP 400, got nil")
	}

	if !contains(err.Error(), "upload failed with status 400") {
		t.Errorf("Expected error about status 400, got: %v", err)
	}
}

func TestSendSessionRequest_ServerRejectsUpload(t *testing.T) {
	// Server returns 200 but success=false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(SaveSessionResponse{
			Success: false,
			Message: "Session already exists",
		})
	}))
	defer server.Close()

	cfg := &config.UploadConfig{
		BackendURL: server.URL,
		APIKey:     "test-api-key",
	}

	request := &SaveSessionRequest{
		SessionID: "test-session-123",
		Files:     []FileUpload{},
	}

	err := SendSessionRequest(cfg, request)
	if err == nil {
		t.Fatal("Expected error for success=false response, got nil")
	}

	if !contains(err.Error(), "Session already exists") {
		t.Errorf("Expected error message about session exists, got: %v", err)
	}
}

func TestSendSessionRequest_CompressionWorks(t *testing.T) {
	var receivedSize int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedSize = int64(len(body))

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(SaveSessionResponse{
			Success:   true,
			SessionID: "test",
		})
	}))
	defer server.Close()

	cfg := &config.UploadConfig{
		BackendURL: server.URL,
		APIKey:     "test-key",
	}

	// Create a large payload with repetitive data (compresses well)
	largeContent := bytes.Repeat([]byte("test data "), 1000)
	request := &SaveSessionRequest{
		SessionID: "test",
		Files: []FileUpload{
			{Path: "/test", Type: "transcript", SizeBytes: int64(len(largeContent)), Content: largeContent},
		},
	}

	err := SendSessionRequest(cfg, request)
	if err != nil {
		t.Fatalf("SendSessionRequest() failed: %v", err)
	}

	// Verify compression worked (compressed size should be much smaller)
	// Uncompressed JSON would be >10KB, compressed should be <1KB
	if receivedSize > 5000 {
		t.Errorf("Compression ineffective: received %d bytes, expected much less", receivedSize)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || bytes.Contains([]byte(s), []byte(substr))))
}
