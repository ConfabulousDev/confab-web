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
	"time"

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

	_, err := SendSessionRequest(cfg, request)
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

	_, err := SendSessionRequest(cfg, request)
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

	_, err := SendSessionRequest(cfg, request)
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

	_, err := SendSessionRequest(cfg, request)
	if err != nil {
		t.Fatalf("SendSessionRequest() failed: %v", err)
	}

	// Verify compression worked (compressed size should be much smaller)
	// Uncompressed JSON would be >10KB, compressed should be <1KB
	if receivedSize > 5000 {
		t.Errorf("Compression ineffective: received %d bytes, expected much less", receivedSize)
	}
}

func TestExtractLastActivity(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name           string
		transcriptData string
		wantTimestamp  string // RFC3339 format, empty string means nil
		wantErr        bool
	}{
		{
			name: "multiple messages with different timestamps",
			transcriptData: `{"type":"user","message":{"content":"hello"},"timestamp":"2025-01-15T10:00:00Z"}
{"type":"assistant","message":{"content":"hi"},"timestamp":"2025-01-15T10:00:05Z"}
{"type":"user","message":{"content":"bye"},"timestamp":"2025-01-15T10:00:10Z"}`,
			wantTimestamp: "2025-01-15T10:00:10Z",
			wantErr:       false,
		},
		{
			name: "messages with nano precision timestamps",
			transcriptData: `{"type":"user","message":{"content":"test"},"timestamp":"2025-01-15T10:00:00.123456789Z"}
{"type":"assistant","message":{"content":"response"},"timestamp":"2025-01-15T10:00:05.987654321Z"}`,
			wantTimestamp: "2025-01-15T10:00:05.987654321Z",
			wantErr:       false,
		},
		{
			name: "mixed timestamp formats (RFC3339 and RFC3339Nano)",
			transcriptData: `{"type":"user","message":{"content":"a"},"timestamp":"2025-01-15T10:00:00Z"}
{"type":"assistant","message":{"content":"b"},"timestamp":"2025-01-15T10:00:05.123456Z"}
{"type":"summary","summary":"test","timestamp":"2025-01-15T10:00:03Z"}`,
			wantTimestamp: "2025-01-15T10:00:05.123456Z",
			wantErr:       false,
		},
		{
			name: "all message types with timestamps",
			transcriptData: `{"type":"user","message":{"content":"user msg"},"timestamp":"2025-01-15T10:00:00Z"}
{"type":"assistant","message":{"content":"assistant msg"},"timestamp":"2025-01-15T10:00:01Z"}
{"type":"system","message":"system msg","timestamp":"2025-01-15T10:00:02Z"}
{"type":"summary","summary":"summary msg","timestamp":"2025-01-15T10:00:03Z"}
{"type":"error","error":"error msg","timestamp":"2025-01-15T10:00:04Z"}`,
			wantTimestamp: "2025-01-15T10:00:04Z",
			wantErr:       false,
		},
		{
			name: "timestamps out of order",
			transcriptData: `{"type":"user","message":{"content":"first"},"timestamp":"2025-01-15T10:00:10Z"}
{"type":"assistant","message":{"content":"second"},"timestamp":"2025-01-15T10:00:05Z"}
{"type":"user","message":{"content":"third"},"timestamp":"2025-01-15T10:00:15Z"}`,
			wantTimestamp: "2025-01-15T10:00:15Z",
			wantErr:       false,
		},
		{
			name: "single message",
			transcriptData: `{"type":"user","message":{"content":"only one"},"timestamp":"2025-01-15T10:00:00Z"}`,
			wantTimestamp: "2025-01-15T10:00:00Z",
			wantErr:       false,
		},
		{
			name:           "empty file",
			transcriptData: "",
			wantTimestamp:  "",
			wantErr:        false,
		},
		{
			name: "only empty lines",
			transcriptData: `

`,
			wantTimestamp: "",
			wantErr:       false,
		},
		{
			name: "messages without timestamps",
			transcriptData: `{"type":"user","message":{"content":"no timestamp"}}
{"type":"assistant","message":{"content":"also no timestamp"}}`,
			wantTimestamp: "",
			wantErr:       false,
		},
		{
			name: "malformed JSON lines (should skip and continue)",
			transcriptData: `{"type":"user","message":{"content":"valid"},"timestamp":"2025-01-15T10:00:00Z"}
this is not valid json
{"type":"assistant","message":{"content":"also valid"},"timestamp":"2025-01-15T10:00:05Z"}`,
			wantTimestamp: "2025-01-15T10:00:05Z",
			wantErr:       false,
		},
		{
			name: "invalid timestamp format (should skip and continue)",
			transcriptData: `{"type":"user","message":{"content":"bad ts"},"timestamp":"not-a-timestamp"}
{"type":"assistant","message":{"content":"good ts"},"timestamp":"2025-01-15T10:00:05Z"}`,
			wantTimestamp: "2025-01-15T10:00:05Z",
			wantErr:       false,
		},
		{
			name: "mixed valid and invalid entries",
			transcriptData: `{"type":"user","message":{"content":"a"},"timestamp":"2025-01-15T10:00:00Z"}
invalid json line here
{"type":"assistant","message":{"content":"b"}}
{"type":"user","message":{"content":"c"},"timestamp":"bad-format"}
{"type":"assistant","message":{"content":"d"},"timestamp":"2025-01-15T10:00:10Z"}`,
			wantTimestamp: "2025-01-15T10:00:10Z",
			wantErr:       false,
		},
		{
			name: "file-history-snapshot with nested timestamp",
			transcriptData: `{"type":"user","message":{"content":"start"},"timestamp":"2025-01-15T10:00:00Z"}
{"type":"file-history-snapshot","messageId":"abc","isSnapshotUpdate":false,"snapshot":{"messageId":"abc","timestamp":"2025-01-15T10:00:15Z","trackedFileBackups":{"file.go":{"backupFileName":"backup1","version":1,"backupTime":"2025-01-15T10:00:15Z"}}}}
{"type":"assistant","message":{"content":"response"},"timestamp":"2025-01-15T10:00:05Z"}`,
			wantTimestamp: "2025-01-15T10:00:15Z",
			wantErr:       false,
		},
		{
			name: "queue-operation messages with timestamps",
			transcriptData: `{"type":"user","message":{"content":"start"},"timestamp":"2025-01-15T10:00:00Z"}
{"type":"queue-operation","operation":"enqueue","timestamp":"2025-01-15T10:00:08Z","content":"queued","sessionId":"test"}
{"type":"assistant","message":{"content":"response"},"timestamp":"2025-01-15T10:00:05Z"}`,
			wantTimestamp: "2025-01-15T10:00:08Z",
			wantErr:       false,
		},
		{
			name: "all message types including file-history-snapshot and queue-operation",
			transcriptData: `{"type":"user","message":{"content":"user msg"},"timestamp":"2025-01-15T10:00:00Z"}
{"type":"assistant","message":{"content":"assistant msg"},"timestamp":"2025-01-15T10:00:01Z"}
{"type":"system","message":"system msg","timestamp":"2025-01-15T10:00:02Z"}
{"type":"summary","summary":"summary msg","timestamp":"2025-01-15T10:00:03Z"}
{"type":"queue-operation","operation":"enqueue","timestamp":"2025-01-15T10:00:04Z","content":"queued","sessionId":"test"}
{"type":"file-history-snapshot","messageId":"xyz","isSnapshotUpdate":false,"snapshot":{"messageId":"xyz","timestamp":"2025-01-15T10:00:20Z","trackedFileBackups":{}}}`,
			wantTimestamp: "2025-01-15T10:00:20Z",
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test transcript file
			transcriptPath := filepath.Join(tmpDir, tt.name+".jsonl")
			err := os.WriteFile(transcriptPath, []byte(tt.transcriptData), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Call extractLastActivity
			result, err := extractLastActivity(transcriptPath)

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("extractLastActivity() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Check result
			if tt.wantTimestamp == "" {
				if result != nil {
					t.Errorf("extractLastActivity() = %v, want nil", result)
				}
			} else {
				if result == nil {
					t.Errorf("extractLastActivity() = nil, want %s", tt.wantTimestamp)
					return
				}

				// Parse expected timestamp
				var expectedTime time.Time
				expectedTime, err = time.Parse(time.RFC3339Nano, tt.wantTimestamp)
				if err != nil {
					expectedTime, err = time.Parse(time.RFC3339, tt.wantTimestamp)
					if err != nil {
						t.Fatalf("Failed to parse expected timestamp: %v", err)
					}
				}

				// Compare timestamps
				if !result.Equal(expectedTime) {
					t.Errorf("extractLastActivity() = %v, want %v", result, expectedTime)
				}
			}
		})
	}
}

func TestExtractLastActivity_NonexistentFile(t *testing.T) {
	_, err := extractLastActivity("/nonexistent/path/to/file.jsonl")
	if err == nil {
		t.Error("extractLastActivity() with nonexistent file should return error, got nil")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || bytes.Contains([]byte(s), []byte(substr))))
}
