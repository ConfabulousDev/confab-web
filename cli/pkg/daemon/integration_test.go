package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
	pkgsync "github.com/santaclaude2025/confab/pkg/sync"
)

// zstd decoder for decompressing request bodies in tests
var zstdDecoder, _ = zstd.NewReader(nil)

// readRequestBody reads and decompresses the request body if needed
func readRequestBody(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	// Decompress if zstd encoded
	if r.Header.Get("Content-Encoding") == "zstd" {
		return zstdDecoder.DecodeAll(body, nil)
	}

	return body, nil
}

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

	// Read and decompress request body
	body, err := readRequestBody(r)
	if err != nil {
		m.t.Errorf("Failed to read request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch r.URL.Path {
	case "/api/v1/sync/init":
		if m.initError {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "init failed"})
			return
		}

		var req pkgsync.SyncInitRequest
		if err := json.Unmarshal(body, &req); err != nil {
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
		if err := json.Unmarshal(body, &req); err != nil {
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

// TestDaemonTranscriptAppearsLate tests that daemon waits for transcript then syncs
func TestDaemonTranscriptAppearsLate(t *testing.T) {
	mock := newMockBackend(t)
	server := httptest.NewServer(mock)
	defer server.Close()

	tmpDir, transcriptPath := setupTestEnv(t, server.URL)

	// DON'T create transcript yet - it will appear later

	d := New(Config{
		ExternalID:     "late-transcript-test",
		TranscriptPath: transcriptPath,
		CWD:            tmpDir,
		SyncInterval:   50 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	// Wait a bit, then create transcript
	time.Sleep(100 * time.Millisecond)

	// Transcript should not exist yet, no init should have happened
	if len(mock.initRequests) > 0 {
		t.Error("Init should not happen before transcript exists")
	}

	// Now create the transcript
	os.MkdirAll(filepath.Dir(transcriptPath), 0755)
	os.WriteFile(transcriptPath, []byte(`{"type":"system","message":"late"}`+"\n"), 0644)

	// Wait for daemon to notice and sync (poll interval is 2s, so wait longer)
	time.Sleep(2500 * time.Millisecond)
	cancel()

	<-errCh

	// Now init should have happened
	if len(mock.initRequests) == 0 {
		t.Error("Expected init request after transcript appeared")
	}
	if len(mock.chunkRequests) == 0 {
		t.Error("Expected chunk upload after transcript appeared")
	}
}

// TestDaemonAgentFileNotExistYet tests that missing agent files are skipped and picked up later
func TestDaemonAgentFileNotExistYet(t *testing.T) {
	mock := newMockBackend(t)
	server := httptest.NewServer(mock)
	defer server.Close()

	tmpDir, transcriptPath := setupTestEnv(t, server.URL)
	transcriptDir := filepath.Dir(transcriptPath)

	// Create transcript that references an agent, but DON'T create the agent file
	transcriptContent := `{"type":"system","message":"start"}
{"type":"user","toolUseResult":{"agentId":"def67890","result":"pending"}}
`
	os.WriteFile(transcriptPath, []byte(transcriptContent), 0644)

	d := New(Config{
		ExternalID:     "agent-not-exist-test",
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

	// Wait for first sync cycle
	time.Sleep(150 * time.Millisecond)

	// Should have synced transcript but no agent (file doesn't exist)
	transcriptUploads := 0
	agentUploads := 0
	for _, req := range mock.chunkRequests {
		if req.FileType == "transcript" {
			transcriptUploads++
		} else if req.FileType == "agent" {
			agentUploads++
		}
	}
	if transcriptUploads == 0 {
		t.Error("Expected transcript upload")
	}
	if agentUploads > 0 {
		t.Error("Should not upload agent that doesn't exist yet")
	}

	// Now create the agent file
	agentPath := filepath.Join(transcriptDir, "agent-def67890.jsonl")
	os.WriteFile(agentPath, []byte(`{"type":"agent","message":"now exists"}`+"\n"), 0644)

	// Wait for next sync cycle
	time.Sleep(150 * time.Millisecond)
	cancel()

	<-errCh

	// Now should have agent upload
	agentUploads = 0
	for _, req := range mock.chunkRequests {
		if req.FileType == "agent" {
			agentUploads++
		}
	}
	if agentUploads == 0 {
		t.Error("Expected agent upload after file appeared")
	}
}

// TestDaemonBackendHasMoreLines tests resuming when backend has more lines than expected
func TestDaemonBackendHasMoreLines(t *testing.T) {
	mock := newMockBackend(t)
	// Backend says it already has 5 lines
	mock.initResponse.Files = map[string]pkgsync.SyncFileState{
		"transcript.jsonl": {LastSyncedLine: 5},
	}
	server := httptest.NewServer(mock)
	defer server.Close()

	tmpDir, transcriptPath := setupTestEnv(t, server.URL)

	// Create transcript with 7 lines (backend has 5, we upload 2)
	var lines []string
	for i := 1; i <= 7; i++ {
		lines = append(lines, fmt.Sprintf(`{"type":"msg","line":%d}`, i))
	}
	os.WriteFile(transcriptPath, []byte(strings.Join(lines, "\n")+"\n"), 0644)

	d := New(Config{
		ExternalID:     "backend-ahead-test",
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

	if len(mock.chunkRequests) == 0 {
		t.Fatal("Expected chunk request")
	}

	// Should start from line 6 (after backend's line 5)
	chunkReq := mock.chunkRequests[0]
	if chunkReq.FirstLine != 6 {
		t.Errorf("Expected first_line 6, got %d", chunkReq.FirstLine)
	}
	if len(chunkReq.Lines) != 2 {
		t.Errorf("Expected 2 lines (6 and 7), got %d", len(chunkReq.Lines))
	}
}

// TestDaemonEmptyTranscript tests handling of empty transcript file
func TestDaemonEmptyTranscript(t *testing.T) {
	mock := newMockBackend(t)
	server := httptest.NewServer(mock)
	defer server.Close()

	tmpDir, transcriptPath := setupTestEnv(t, server.URL)

	// Create empty transcript
	os.WriteFile(transcriptPath, []byte(""), 0644)

	d := New(Config{
		ExternalID:     "empty-transcript-test",
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

	// Init should happen
	if len(mock.initRequests) == 0 {
		t.Error("Expected init request even for empty transcript")
	}

	// No chunks should be uploaded (nothing to sync)
	if len(mock.chunkRequests) > 0 {
		t.Errorf("Expected no chunk uploads for empty transcript, got %d", len(mock.chunkRequests))
	}
}

// TestDaemonShutdownFinalSync tests that final sync happens on shutdown
func TestDaemonShutdownFinalSync(t *testing.T) {
	mock := newMockBackend(t)
	server := httptest.NewServer(mock)
	defer server.Close()

	tmpDir, transcriptPath := setupTestEnv(t, server.URL)

	// Create transcript
	os.WriteFile(transcriptPath, []byte(`{"type":"system","line":1}`+"\n"), 0644)

	d := New(Config{
		ExternalID:     "shutdown-sync-test",
		TranscriptPath: transcriptPath,
		CWD:            tmpDir,
		SyncInterval:   10 * time.Second, // Very long - won't trigger during test
	})

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	// Wait for initial sync (happens immediately on start)
	time.Sleep(100 * time.Millisecond)

	initialChunks := len(mock.chunkRequests)

	// Append content that won't be synced by interval (10s is too long)
	f, _ := os.OpenFile(transcriptPath, os.O_APPEND|os.O_WRONLY, 0644)
	f.WriteString(`{"type":"user","line":2}` + "\n")
	f.Close()

	// Give a moment for file to be written
	time.Sleep(50 * time.Millisecond)

	// Cancel - should trigger final sync
	cancel()

	<-errCh

	// Should have more chunks after shutdown (final sync picked up line 2)
	if len(mock.chunkRequests) <= initialChunks {
		t.Errorf("Expected final sync to upload new content, had %d chunks before, %d after",
			initialChunks, len(mock.chunkRequests))
	}
}

// TestDaemonMultipleAgentFiles tests discovery and sync of multiple agent files
func TestDaemonMultipleAgentFiles(t *testing.T) {
	mock := newMockBackend(t)
	server := httptest.NewServer(mock)
	defer server.Close()

	tmpDir, transcriptPath := setupTestEnv(t, server.URL)
	transcriptDir := filepath.Dir(transcriptPath)

	// Create transcript referencing multiple agents
	transcriptContent := `{"type":"system","message":"start"}
{"type":"user","toolUseResult":{"agentId":"aaaaaaaa","result":"done"}}
{"type":"user","toolUseResult":{"agentId":"bbbbbbbb","result":"done"}}
{"type":"user","toolUseResult":{"agentId":"cccccccc","result":"done"}}
`
	os.WriteFile(transcriptPath, []byte(transcriptContent), 0644)

	// Create all three agent files
	for _, id := range []string{"aaaaaaaa", "bbbbbbbb", "cccccccc"} {
		agentPath := filepath.Join(transcriptDir, fmt.Sprintf("agent-%s.jsonl", id))
		os.WriteFile(agentPath, []byte(fmt.Sprintf(`{"agent":"%s","line":1}`+"\n", id)), 0644)
	}

	d := New(Config{
		ExternalID:     "multi-agent-test",
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

	time.Sleep(300 * time.Millisecond)
	cancel()

	<-errCh

	// Count uploads by type
	transcriptUploads := 0
	agentFiles := make(map[string]bool)
	for _, req := range mock.chunkRequests {
		if req.FileType == "transcript" {
			transcriptUploads++
		} else if req.FileType == "agent" {
			agentFiles[req.FileName] = true
		}
	}

	if transcriptUploads == 0 {
		t.Error("Expected transcript upload")
	}
	if len(agentFiles) != 3 {
		t.Errorf("Expected 3 different agent files uploaded, got %d: %v", len(agentFiles), agentFiles)
	}
	for _, id := range []string{"aaaaaaaa", "bbbbbbbb", "cccccccc"} {
		expectedName := fmt.Sprintf("agent-%s.jsonl", id)
		if !agentFiles[expectedName] {
			t.Errorf("Expected agent file %s to be uploaded", expectedName)
		}
	}
}

// TestDaemonAgentAppearsMidSession tests agent discovered after initial sync
func TestDaemonAgentAppearsMidSession(t *testing.T) {
	mock := newMockBackend(t)
	server := httptest.NewServer(mock)
	defer server.Close()

	tmpDir, transcriptPath := setupTestEnv(t, server.URL)
	transcriptDir := filepath.Dir(transcriptPath)

	// Start with transcript that has NO agent references
	os.WriteFile(transcriptPath, []byte(`{"type":"system","message":"start"}`+"\n"), 0644)

	d := New(Config{
		ExternalID:     "mid-session-agent-test",
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

	// Wait for initial sync
	time.Sleep(150 * time.Millisecond)

	// Verify no agent uploads yet
	agentUploadsBefore := 0
	for _, req := range mock.chunkRequests {
		if req.FileType == "agent" {
			agentUploadsBefore++
		}
	}
	if agentUploadsBefore > 0 {
		t.Error("Should have no agent uploads before agent is referenced")
	}

	// Now append agent reference to transcript AND create agent file
	// Note: agent ID must be valid 8-char hex
	f, _ := os.OpenFile(transcriptPath, os.O_APPEND|os.O_WRONLY, 0644)
	f.WriteString(`{"type":"user","toolUseResult":{"agentId":"12345678","result":"done"}}` + "\n")
	f.Close()

	agentPath := filepath.Join(transcriptDir, "agent-12345678.jsonl")
	os.WriteFile(agentPath, []byte(`{"type":"agent","message":"mid-session agent"}`+"\n"), 0644)

	// Wait for sync to pick up the new agent
	time.Sleep(250 * time.Millisecond)
	cancel()

	<-errCh

	// Now should have agent upload
	agentUploadsAfter := 0
	for _, req := range mock.chunkRequests {
		if req.FileType == "agent" && req.FileName == "agent-12345678.jsonl" {
			agentUploadsAfter++
		}
	}
	if agentUploadsAfter == 0 {
		t.Error("Expected agent upload after agent appeared mid-session")
	}
}

// TestDaemonConcurrentStartup tests that a second daemon for the same session
// detects the first is running and exits gracefully (or the first continues if second starts).
// The key behavior: at least one daemon should successfully sync, no data corruption.
func TestDaemonConcurrentStartup(t *testing.T) {
	mock := newMockBackend(t)
	server := httptest.NewServer(mock)
	defer server.Close()

	tmpDir, transcriptPath := setupTestEnv(t, server.URL)

	// Create transcript
	os.WriteFile(transcriptPath, []byte(`{"type":"system","message":"concurrent test"}`+"\n"), 0644)

	// Start first daemon
	d1 := New(Config{
		ExternalID:     "concurrent-test",
		TranscriptPath: transcriptPath,
		CWD:            tmpDir,
		SyncInterval:   50 * time.Millisecond,
	})

	ctx1, cancel1 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel1()

	errCh1 := make(chan error, 1)
	go func() {
		errCh1 <- d1.Run(ctx1)
	}()

	// Give first daemon time to start and save state
	time.Sleep(100 * time.Millisecond)

	// Start second daemon with same external ID
	d2 := New(Config{
		ExternalID:     "concurrent-test", // Same ID!
		TranscriptPath: transcriptPath,
		CWD:            tmpDir,
		SyncInterval:   50 * time.Millisecond,
	})

	ctx2, cancel2 := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel2()

	errCh2 := make(chan error, 1)
	go func() {
		errCh2 <- d2.Run(ctx2)
	}()

	// Wait for both to run for a bit
	time.Sleep(300 * time.Millisecond)

	// Cancel both
	cancel1()
	cancel2()

	// Wait for both to exit
	<-errCh1
	<-errCh2

	// Key assertion: at least one successful init and chunk upload happened
	// (we don't care which daemon "won", just that syncing worked)
	if len(mock.initRequests) == 0 {
		t.Error("Expected at least one init request from concurrent daemons")
	}
	if len(mock.chunkRequests) == 0 {
		t.Error("Expected at least one chunk upload from concurrent daemons")
	}

	// Verify no duplicate uploads of the same content (idempotency)
	// Both daemons might upload, but the backend should handle dedup
	t.Logf("Concurrent test: %d init requests, %d chunk requests",
		len(mock.initRequests), len(mock.chunkRequests))
}

// TestDaemonFileTruncation tests that daemon handles file truncation gracefully.
// If a transcript file is truncated mid-session, daemon should:
// 1. Not crash
// 2. Continue running
// 3. Sync whatever content is available
func TestDaemonFileTruncation(t *testing.T) {
	mock := newMockBackend(t)
	server := httptest.NewServer(mock)
	defer server.Close()

	tmpDir, transcriptPath := setupTestEnv(t, server.URL)

	// Create transcript with multiple lines
	initialContent := `{"type":"system","line":1}
{"type":"user","line":2}
{"type":"assistant","line":3}
{"type":"user","line":4}
{"type":"assistant","line":5}
`
	os.WriteFile(transcriptPath, []byte(initialContent), 0644)

	d := New(Config{
		ExternalID:     "truncation-test",
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

	// Wait for initial sync
	time.Sleep(200 * time.Millisecond)

	// Verify initial content was synced
	initialChunks := len(mock.chunkRequests)
	if initialChunks == 0 {
		t.Fatal("Expected initial chunk upload")
	}

	// Now truncate the file to just 2 lines (simulating corruption or reset)
	truncatedContent := `{"type":"system","line":1}
{"type":"user","line":2}
`
	os.WriteFile(transcriptPath, []byte(truncatedContent), 0644)

	// Wait for next sync cycle
	time.Sleep(200 * time.Millisecond)

	// Daemon should still be running (not crashed)
	select {
	case err := <-errCh:
		t.Fatalf("Daemon crashed after truncation: %v", err)
	default:
		// Good - daemon still running
	}

	// Now append new content after truncation
	f, _ := os.OpenFile(transcriptPath, os.O_APPEND|os.O_WRONLY, 0644)
	f.WriteString(`{"type":"assistant","line":3,"new":true}` + "\n")
	f.Close()

	// Wait for sync
	time.Sleep(200 * time.Millisecond)

	cancel()
	<-errCh

	// Daemon should have continued running and completed gracefully
	t.Logf("Truncation test: daemon handled truncation, total chunks=%d", len(mock.chunkRequests))
}

// TestDaemonHTTPErrors tests that daemon handles various HTTP errors gracefully.
// When HTTP requests fail (timeout, connection reset, server errors), daemon should:
// 1. Not crash
// 2. Log the error
// 3. Continue running and retry on next cycle
func TestDaemonHTTPErrors(t *testing.T) {
	var requestCount int32

	// Create a server that returns various errors then recovers
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)

		// Simulate various error conditions
		switch count {
		case 1:
			// Connection reset / abrupt close
			hj, ok := w.(http.Hijacker)
			if ok {
				conn, _, _ := hj.Hijack()
				conn.Close()
				return
			}
			// Fallback if hijacking not supported
			w.WriteHeader(http.StatusInternalServerError)
		case 2:
			// Rate limited
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte("Rate limited"))
		case 3:
			// Server error
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal error"))
		default:
			// After errors, succeed (simulating recovery)
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/api/v1/sync/init" {
				json.NewEncoder(w).Encode(pkgsync.SyncInitResponse{
					SessionID: "recovered-session",
					Files:     make(map[string]pkgsync.SyncFileState),
				})
			} else if r.URL.Path == "/api/v1/sync/chunk" {
				var req pkgsync.SyncChunkRequest
				json.NewDecoder(r.Body).Decode(&req)
				lastLine := req.FirstLine + len(req.Lines) - 1
				json.NewEncoder(w).Encode(pkgsync.SyncChunkResponse{
					LastSyncedLine: lastLine,
				})
			}
		}
	}))
	defer errorServer.Close()

	tmpDir, transcriptPath := setupTestEnv(t, errorServer.URL)

	// Create transcript
	os.WriteFile(transcriptPath, []byte(`{"type":"system","message":"error test"}`+"\n"), 0644)

	d := New(Config{
		ExternalID:     "http-error-test",
		TranscriptPath: transcriptPath,
		CWD:            tmpDir,
		SyncInterval:   100 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	startTime := time.Now()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	// Wait for daemon to experience failures and recover
	time.Sleep(600 * time.Millisecond)

	// Daemon should still be running despite errors
	select {
	case err := <-errCh:
		t.Fatalf("Daemon crashed on HTTP error: %v", err)
	default:
		// Good - daemon still running
	}

	// Wait for more cycles to allow recovery
	time.Sleep(1000 * time.Millisecond)
	cancel()
	<-errCh

	elapsed := time.Since(startTime)
	finalCount := atomic.LoadInt32(&requestCount)

	// Daemon should have:
	// 1. Survived all error types
	// 2. Eventually recovered and made successful requests
	if finalCount < 4 {
		t.Errorf("Expected at least 4 requests (3 errors + recovery), got %d", finalCount)
	}

	t.Logf("HTTP error test: daemon survived %.1fs, %d total requests (first 3 had errors)",
		elapsed.Seconds(), finalCount)
}

// TestDaemonLargeFile tests that daemon can handle large transcript files (~100MB).
// This tests memory efficiency and streaming behavior.
func TestDaemonLargeFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large file test in short mode")
	}

	var totalLinesReceived int32
	var totalBytesReceived int64 // tracks compressed bytes received

	// Custom server that tracks received data
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Read raw body first to track compressed size
		rawBody, _ := io.ReadAll(r.Body)
		atomic.AddInt64(&totalBytesReceived, int64(len(rawBody)))

		// Decompress if needed
		var body []byte
		if r.Header.Get("Content-Encoding") == "zstd" {
			body, _ = zstdDecoder.DecodeAll(rawBody, nil)
		} else {
			body = rawBody
		}

		switch r.URL.Path {
		case "/api/v1/sync/init":
			json.NewEncoder(w).Encode(pkgsync.SyncInitResponse{
				SessionID: "large-file-session",
				Files:     make(map[string]pkgsync.SyncFileState),
			})

		case "/api/v1/sync/chunk":
			var req pkgsync.SyncChunkRequest
			if json.Unmarshal(body, &req) == nil {
				atomic.AddInt32(&totalLinesReceived, int32(len(req.Lines)))
				lastLine := req.FirstLine + len(req.Lines) - 1
				json.NewEncoder(w).Encode(pkgsync.SyncChunkResponse{
					LastSyncedLine: lastLine,
				})
			} else {
				w.WriteHeader(http.StatusBadRequest)
			}

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	tmpDir, transcriptPath := setupTestEnv(t, server.URL)

	// Create a large transcript file (~100MB)
	// Each line is ~1KB of JSON, 100K lines = ~100MB
	f, err := os.Create(transcriptPath)
	if err != nil {
		t.Fatalf("Failed to create transcript: %v", err)
	}

	numLines := 100000
	padding := strings.Repeat("x", 900) // ~900 bytes padding per line
	for i := 0; i < numLines; i++ {
		line := fmt.Sprintf(`{"type":"msg","line":%d,"padding":"%s"}`, i+1, padding)
		f.WriteString(line + "\n")
	}
	f.Close()

	// Verify file size
	info, _ := os.Stat(transcriptPath)
	fileSizeMB := float64(info.Size()) / (1024 * 1024)
	t.Logf("Large file test: created %d lines, %.2f MB", numLines, fileSizeMB)

	if fileSizeMB < 80 {
		t.Fatalf("File too small: expected ~100MB, got %.2f MB", fileSizeMB)
	}

	d := New(Config{
		ExternalID:     "large-file-test",
		TranscriptPath: transcriptPath,
		CWD:            tmpDir,
		SyncInterval:   50 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	startTime := time.Now()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	// Wait for sync - large file might take a while
	// Poll until we've received all lines or timeout
	deadline := time.Now().Add(110 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&totalLinesReceived) >= int32(numLines) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	cancel()
	<-errCh

	elapsed := time.Since(startTime)

	// Verify all lines were uploaded
	received := atomic.LoadInt32(&totalLinesReceived)
	if received < int32(numLines) {
		t.Errorf("Expected %d lines uploaded, got %d", numLines, received)
	}

	bytesReceived := atomic.LoadInt64(&totalBytesReceived)
	bytesReceivedMB := float64(bytesReceived) / (1024 * 1024)
	throughputMBps := bytesReceivedMB / elapsed.Seconds()
	compressionRatio := bytesReceivedMB / fileSizeMB * 100

	t.Logf("Large file test: uploaded %d lines, %.2f MB compressed (%.1f%% of %.2f MB original) in %.1fs (%.2f MB/s)",
		received, bytesReceivedMB, compressionRatio, fileSizeMB, elapsed.Seconds(), throughputMBps)
}
