package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	pkgsync "github.com/santaclaude2025/confab/pkg/sync"
)

// mockBackend tracks requests and provides configurable responses
type mockBackend struct {
	t              *testing.T
	initRequests   []pkgsync.SyncInitRequest
	chunkRequests  []pkgsync.SyncChunkRequest
	initResponse   *pkgsync.SyncInitResponse
	initError      bool
	chunkError     bool
	requestCount   int32
	failUntilCount int32 // fail requests until this count is reached
}

func newMockBackend(t *testing.T) *mockBackend {
	return &mockBackend{
		t: t,
		initResponse: &pkgsync.SyncInitResponse{
			SessionID: "test-session-id",
			Files:     make(map[string]pkgsync.SyncFileState),
		},
	}
}

func (m *mockBackend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	count := atomic.AddInt32(&m.requestCount, 1)

	// Simulate failures until failUntilCount
	if m.failUntilCount > 0 && count <= m.failUntilCount {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Service Unavailable"))
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch r.URL.Path {
	case "/api/v1/sync/init":
		if m.initError {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "init failed"})
			return
		}

		var req pkgsync.SyncInitRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.t.Errorf("Failed to decode init request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		m.initRequests = append(m.initRequests, req)
		json.NewEncoder(w).Encode(m.initResponse)

	case "/api/v1/sync/chunk":
		if m.chunkError {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "chunk failed"})
			return
		}

		var req pkgsync.SyncChunkRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.t.Errorf("Failed to decode chunk request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		m.chunkRequests = append(m.chunkRequests, req)

		// Return last synced line as first + len(lines) - 1
		lastLine := req.FirstLine + len(req.Lines) - 1
		json.NewEncoder(w).Encode(pkgsync.SyncChunkResponse{
			LastSyncedLine: lastLine,
		})

	default:
		m.t.Errorf("Unexpected request to %s", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}
}

// setupTestEnv creates a temporary environment for daemon testing
func setupTestEnv(t *testing.T, serverURL string) (tmpDir string, transcriptPath string) {
	tmpDir = t.TempDir()

	// Set up config
	confabDir := filepath.Join(tmpDir, ".confab")
	os.MkdirAll(confabDir, 0755)
	configPath := filepath.Join(confabDir, "config.json")
	configJSON := fmt.Sprintf(`{"backend_url":"%s","api_key":"test-api-key-12345678"}`, serverURL)
	os.WriteFile(configPath, []byte(configJSON), 0600)
	t.Setenv("CONFAB_CONFIG_PATH", configPath)
	t.Setenv("HOME", tmpDir)

	// Create transcript directory
	transcriptDir := filepath.Join(tmpDir, "sessions")
	os.MkdirAll(transcriptDir, 0755)
	transcriptPath = filepath.Join(transcriptDir, "transcript.jsonl")

	return tmpDir, transcriptPath
}

// TestDaemonSyncCycle tests a full init + sync cycle with mock server
func TestDaemonSyncCycle(t *testing.T) {
	mock := newMockBackend(t)
	server := httptest.NewServer(mock)
	defer server.Close()

	tmpDir, transcriptPath := setupTestEnv(t, server.URL)

	// Create transcript with some content
	transcriptContent := `{"type":"system","message":"hello"}
{"type":"user","message":"world"}
{"type":"assistant","message":"response"}
`
	os.WriteFile(transcriptPath, []byte(transcriptContent), 0644)

	// Create and run daemon
	d := New(Config{
		ExternalID:     "test-external-id",
		TranscriptPath: transcriptPath,
		CWD:            tmpDir,
		SyncInterval:   50 * time.Millisecond, // Fast for testing
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Run daemon in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	// Wait for at least one sync cycle
	time.Sleep(200 * time.Millisecond)
	cancel()

	// Wait for daemon to exit
	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
		t.Fatal("Daemon did not exit")
	}

	// Verify init was called
	if len(mock.initRequests) == 0 {
		t.Fatal("Expected init request, got none")
	}
	initReq := mock.initRequests[0]
	if initReq.ExternalID != "test-external-id" {
		t.Errorf("Expected external_id 'test-external-id', got %q", initReq.ExternalID)
	}
	if initReq.TranscriptPath != transcriptPath {
		t.Errorf("Expected transcript_path %q, got %q", transcriptPath, initReq.TranscriptPath)
	}

	// Verify chunk was uploaded
	if len(mock.chunkRequests) == 0 {
		t.Fatal("Expected chunk request, got none")
	}
	chunkReq := mock.chunkRequests[0]
	if chunkReq.SessionID != "test-session-id" {
		t.Errorf("Expected session_id 'test-session-id', got %q", chunkReq.SessionID)
	}
	if chunkReq.FileType != "transcript" {
		t.Errorf("Expected file_type 'transcript', got %q", chunkReq.FileType)
	}
	if len(chunkReq.Lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(chunkReq.Lines))
	}
	if chunkReq.FirstLine != 1 {
		t.Errorf("Expected first_line 1, got %d", chunkReq.FirstLine)
	}
}

// TestDaemonRetryOnBackendError tests that daemon retries when backend is unavailable
func TestDaemonRetryOnBackendError(t *testing.T) {
	mock := newMockBackend(t)
	mock.failUntilCount = 2 // Fail first 2 requests
	server := httptest.NewServer(mock)
	defer server.Close()

	tmpDir, transcriptPath := setupTestEnv(t, server.URL)

	// Create transcript
	os.WriteFile(transcriptPath, []byte(`{"type":"system"}`+"\n"), 0644)

	d := New(Config{
		ExternalID:     "retry-test",
		TranscriptPath: transcriptPath,
		CWD:            tmpDir,
		SyncInterval:   100 * time.Millisecond, // Needs to be long enough to trigger retries
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	// Wait for retries - need enough time for multiple sync intervals
	time.Sleep(800 * time.Millisecond)
	cancel()

	<-errCh

	// Should have had multiple attempts to init endpoint
	totalRequests := atomic.LoadInt32(&mock.requestCount)
	if totalRequests < 3 {
		t.Errorf("Expected at least 3 requests (2 failures + 1 success), got %d", totalRequests)
	}

	// Eventually should have succeeded with init
	if len(mock.initRequests) == 0 {
		t.Error("Expected at least one successful init request after retries")
	}
}

// TestDaemonAgentDiscovery tests that daemon discovers and uploads agent files
func TestDaemonAgentDiscovery(t *testing.T) {
	mock := newMockBackend(t)
	server := httptest.NewServer(mock)
	defer server.Close()

	tmpDir, transcriptPath := setupTestEnv(t, server.URL)
	transcriptDir := filepath.Dir(transcriptPath)

	// Create transcript that references an agent
	transcriptContent := `{"type":"system","message":"start"}
{"type":"user","toolUseResult":{"agentId":"abc12345","result":"done"}}
`
	os.WriteFile(transcriptPath, []byte(transcriptContent), 0644)

	// Create the agent file
	agentPath := filepath.Join(transcriptDir, "agent-abc12345.jsonl")
	agentContent := `{"type":"agent","message":"agent line 1"}
{"type":"agent","message":"agent line 2"}
`
	os.WriteFile(agentPath, []byte(agentContent), 0644)

	d := New(Config{
		ExternalID:     "agent-discovery-test",
		TranscriptPath: transcriptPath,
		CWD:            tmpDir,
		SyncInterval:   50 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	// Wait for sync
	time.Sleep(300 * time.Millisecond)
	cancel()

	<-errCh

	// Verify both transcript and agent were uploaded
	if len(mock.chunkRequests) < 2 {
		t.Fatalf("Expected at least 2 chunk requests (transcript + agent), got %d", len(mock.chunkRequests))
	}

	// Find transcript and agent uploads
	var transcriptChunk, agentChunk *pkgsync.SyncChunkRequest
	for i := range mock.chunkRequests {
		req := &mock.chunkRequests[i]
		if req.FileType == "transcript" {
			transcriptChunk = req
		} else if req.FileType == "agent" {
			agentChunk = req
		}
	}

	if transcriptChunk == nil {
		t.Error("Expected transcript chunk upload")
	}
	if agentChunk == nil {
		t.Error("Expected agent chunk upload")
	} else {
		if agentChunk.FileName != "agent-abc12345.jsonl" {
			t.Errorf("Expected agent file name 'agent-abc12345.jsonl', got %q", agentChunk.FileName)
		}
		if len(agentChunk.Lines) != 2 {
			t.Errorf("Expected 2 agent lines, got %d", len(agentChunk.Lines))
		}
	}
}

// TestDaemonIncrementalSync tests that daemon only uploads new lines
func TestDaemonIncrementalSync(t *testing.T) {
	mock := newMockBackend(t)
	// Simulate backend already has first 2 lines
	mock.initResponse.Files = map[string]pkgsync.SyncFileState{
		"transcript.jsonl": {LastSyncedLine: 2},
	}
	server := httptest.NewServer(mock)
	defer server.Close()

	tmpDir, transcriptPath := setupTestEnv(t, server.URL)

	// Create transcript with 4 lines (backend has 2, we should upload 2)
	transcriptContent := `{"type":"system","line":1}
{"type":"user","line":2}
{"type":"assistant","line":3}
{"type":"user","line":4}
`
	os.WriteFile(transcriptPath, []byte(transcriptContent), 0644)

	d := New(Config{
		ExternalID:     "incremental-test",
		TranscriptPath: transcriptPath,
		CWD:            tmpDir,
		SyncInterval:   50 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	time.Sleep(200 * time.Millisecond)
	cancel()

	<-errCh

	// Verify only new lines were uploaded
	if len(mock.chunkRequests) == 0 {
		t.Fatal("Expected chunk request, got none")
	}

	chunkReq := mock.chunkRequests[0]
	if chunkReq.FirstLine != 3 {
		t.Errorf("Expected first_line 3 (after synced line 2), got %d", chunkReq.FirstLine)
	}
	if len(chunkReq.Lines) != 2 {
		t.Errorf("Expected 2 new lines, got %d", len(chunkReq.Lines))
	}
}

// TestDaemonMultipleSyncCycles tests that daemon continues syncing new content
func TestDaemonMultipleSyncCycles(t *testing.T) {
	mock := newMockBackend(t)
	server := httptest.NewServer(mock)
	defer server.Close()

	tmpDir, transcriptPath := setupTestEnv(t, server.URL)

	// Start with initial content
	os.WriteFile(transcriptPath, []byte(`{"type":"system","line":1}`+"\n"), 0644)

	d := New(Config{
		ExternalID:     "multi-cycle-test",
		TranscriptPath: transcriptPath,
		CWD:            tmpDir,
		SyncInterval:   100 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	// Wait for first sync
	time.Sleep(150 * time.Millisecond)

	// Append more content
	f, _ := os.OpenFile(transcriptPath, os.O_APPEND|os.O_WRONLY, 0644)
	f.WriteString(`{"type":"user","line":2}` + "\n")
	f.WriteString(`{"type":"assistant","line":3}` + "\n")
	f.Close()

	// Wait for second sync
	time.Sleep(200 * time.Millisecond)
	cancel()

	<-errCh

	// Should have multiple chunk uploads
	if len(mock.chunkRequests) < 2 {
		t.Errorf("Expected at least 2 chunk uploads (initial + appended), got %d", len(mock.chunkRequests))
	}

	// First chunk should be line 1
	if mock.chunkRequests[0].FirstLine != 1 {
		t.Errorf("First chunk should start at line 1, got %d", mock.chunkRequests[0].FirstLine)
	}

	// Second chunk should be lines 2-3
	if len(mock.chunkRequests) >= 2 {
		secondChunk := mock.chunkRequests[1]
		if secondChunk.FirstLine != 2 {
			t.Errorf("Second chunk should start at line 2, got %d", secondChunk.FirstLine)
		}
	}
}
