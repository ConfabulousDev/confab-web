package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santaclaude2025/confab/pkg/config"
	"github.com/santaclaude2025/confab/pkg/types"
)

// TestSaveFromHook_ValidInput tests the happy path of the SessionEnd hook
func TestSaveFromHook_ValidInput(t *testing.T) {
	// Setup: Create temp directory with transcript file
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "session-123.jsonl")

	// Create a simple transcript file
	transcriptContent := `{"type":"user","message":"test"}`
	if err := os.WriteFile(transcriptPath, []byte(transcriptContent), 0644); err != nil {
		t.Fatalf("Failed to create transcript: %v", err)
	}

	// Setup: Mock HTTP server for upload endpoint
	uploadCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sessions/save" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		uploadCalled = true

		// Return success response
		response := map[string]interface{}{
			"success":    true,
			"session_id": "session-123",
			"run_id":     1,
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Setup: Configure upload with mock server
	cfg := &config.UploadConfig{
		BackendURL: server.URL,
		APIKey:     "test-api-key-1234567890",
	}
	if err := config.SaveUploadConfig(cfg); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Setup: Prepare hook input JSON
	hookInput := types.HookInput{
		SessionID:      "session-123",
		TranscriptPath: transcriptPath,
		CWD:            tmpDir,
		Reason:         "user_exit",
	}
	hookInputJSON, _ := json.Marshal(hookInput)

	// Setup: Capture stdin, stdout, stderr
	oldStdin := os.Stdin
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	defer func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	// Create pipes for stdin, stdout, stderr
	stdinReader, stdinWriter, _ := os.Pipe()
	stdoutReader, stdoutWriter, _ := os.Pipe()
	stderrReader, stderrWriter, _ := os.Pipe()

	os.Stdin = stdinReader
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter

	// Write hook input to stdin
	go func() {
		stdinWriter.Write(hookInputJSON)
		stdinWriter.Close()
	}()

	// Capture stdout in goroutine
	stdoutChan := make(chan string)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, stdoutReader)
		stdoutChan <- buf.String()
	}()

	// Capture stderr in goroutine
	stderrChan := make(chan string)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, stderrReader)
		stderrChan <- buf.String()
	}()

	// Execute: Run saveFromHook
	err := saveFromHook()

	// Close writers to unblock readers
	stdoutWriter.Close()
	stderrWriter.Close()

	// Get captured output
	stdoutOutput := <-stdoutChan
	stderrOutput := <-stderrChan

	// Verify: No error returned (even if upload fails, it returns nil)
	if err != nil {
		t.Errorf("Expected nil error, got: %v", err)
	}

	// Verify: Valid hook response written to stdout
	var hookResponse types.HookResponse
	if err := json.Unmarshal([]byte(stdoutOutput), &hookResponse); err != nil {
		t.Fatalf("Failed to parse hook response: %v\nOutput: %s", err, stdoutOutput)
	}

	if !hookResponse.Continue {
		t.Error("Expected hook response Continue=true")
	}

	// Verify: Upload was called
	if !uploadCalled {
		t.Error("Expected upload to be called")
	}

	// Verify: Stderr contains expected messages
	if !strings.Contains(stderrOutput, "Confab: Capture Session") {
		t.Error("Expected 'Confab: Capture Session' in stderr")
	}
	if !strings.Contains(stderrOutput, "Session ID: session-123") {
		t.Error("Expected session ID in stderr")
	}
}

// TestSaveFromHook_InvalidJSON tests graceful handling of invalid input
func TestSaveFromHook_InvalidJSON(t *testing.T) {
	// Setup: Capture stdin, stdout, stderr
	oldStdin := os.Stdin
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	defer func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	stdinReader, stdinWriter, _ := os.Pipe()
	stdoutReader, stdoutWriter, _ := os.Pipe()
	stderrReader, stderrWriter, _ := os.Pipe()

	os.Stdin = stdinReader
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter

	// Write invalid JSON to stdin
	go func() {
		stdinWriter.Write([]byte("not valid json"))
		stdinWriter.Close()
	}()

	// Capture stdout
	stdoutChan := make(chan string)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, stdoutReader)
		stdoutChan <- buf.String()
	}()

	// Capture stderr
	stderrChan := make(chan string)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, stderrReader)
		stderrChan <- buf.String()
	}()

	// Execute: Run saveFromHook
	err := saveFromHook()

	// Close writers
	stdoutWriter.Close()
	stderrWriter.Close()

	// Get captured output
	stdoutOutput := <-stdoutChan
	stderrOutput := <-stderrChan

	// Verify: Returns nil (graceful failure)
	if err != nil {
		t.Errorf("Expected nil error for graceful failure, got: %v", err)
	}

	// Verify: Valid hook response still written to stdout
	var hookResponse types.HookResponse
	if err := json.Unmarshal([]byte(stdoutOutput), &hookResponse); err != nil {
		t.Fatalf("Failed to parse hook response: %v\nOutput: %s", err, stdoutOutput)
	}

	if !hookResponse.Continue {
		t.Error("Expected hook response Continue=true even on error")
	}

	// Verify: Error message in stderr
	if !strings.Contains(stderrOutput, "Error reading hook input") {
		t.Errorf("Expected error message in stderr, got: %s", stderrOutput)
	}
}

// TestSaveFromHook_MissingTranscript tests handling of missing transcript file
func TestSaveFromHook_MissingTranscript(t *testing.T) {
	tmpDir := t.TempDir()
	missingPath := filepath.Join(tmpDir, "nonexistent.jsonl")

	// Setup: Prepare hook input with missing transcript
	hookInput := types.HookInput{
		SessionID:      "session-456",
		TranscriptPath: missingPath,
		CWD:            tmpDir,
		Reason:         "user_exit",
	}
	hookInputJSON, _ := json.Marshal(hookInput)

	// Setup: Capture stdin, stdout, stderr
	oldStdin := os.Stdin
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	defer func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	stdinReader, stdinWriter, _ := os.Pipe()
	stdoutReader, stdoutWriter, _ := os.Pipe()
	stderrReader, stderrWriter, _ := os.Pipe()

	os.Stdin = stdinReader
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter

	go func() {
		stdinWriter.Write(hookInputJSON)
		stdinWriter.Close()
	}()

	stdoutChan := make(chan string)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, stdoutReader)
		stdoutChan <- buf.String()
	}()

	stderrChan := make(chan string)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, stderrReader)
		stderrChan <- buf.String()
	}()

	// Execute
	err := saveFromHook()

	stdoutWriter.Close()
	stderrWriter.Close()

	stdoutOutput := <-stdoutChan
	stderrOutput := <-stderrChan

	// Verify: Returns nil (graceful failure)
	if err != nil {
		t.Errorf("Expected nil error, got: %v", err)
	}

	// Verify: Valid hook response
	var hookResponse types.HookResponse
	if err := json.Unmarshal([]byte(stdoutOutput), &hookResponse); err != nil {
		t.Fatalf("Failed to parse hook response: %v", err)
	}

	if !hookResponse.Continue {
		t.Error("Expected Continue=true")
	}

	// Verify: Error about discovering files in stderr
	if !strings.Contains(stderrOutput, "Error discovering files") {
		t.Errorf("Expected discovery error in stderr, got: %s", stderrOutput)
	}
}

// TestSaveFromHook_UploadFailure tests handling when upload fails
func TestSaveFromHook_UploadFailure(t *testing.T) {
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "session-789.jsonl")

	// Create transcript
	if err := os.WriteFile(transcriptPath, []byte(`{"type":"test"}`), 0644); err != nil {
		t.Fatalf("Failed to create transcript: %v", err)
	}

	// Setup: Mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Internal server error")
	}))
	defer server.Close()

	// Configure upload
	cfg := &config.UploadConfig{
		BackendURL: server.URL,
		APIKey:     "test-key-1234567890",
	}
	if err := config.SaveUploadConfig(cfg); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Prepare input
	hookInput := types.HookInput{
		SessionID:      "session-789",
		TranscriptPath: transcriptPath,
		CWD:            tmpDir,
		Reason:         "user_exit",
	}
	hookInputJSON, _ := json.Marshal(hookInput)

	// Capture I/O
	oldStdin := os.Stdin
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	defer func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	stdinReader, stdinWriter, _ := os.Pipe()
	stdoutReader, stdoutWriter, _ := os.Pipe()
	stderrReader, stderrWriter, _ := os.Pipe()

	os.Stdin = stdinReader
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter

	go func() {
		stdinWriter.Write(hookInputJSON)
		stdinWriter.Close()
	}()

	stdoutChan := make(chan string)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, stdoutReader)
		stdoutChan <- buf.String()
	}()

	stderrChan := make(chan string)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, stderrReader)
		stderrChan <- buf.String()
	}()

	// Execute
	err := saveFromHook()

	stdoutWriter.Close()
	stderrWriter.Close()

	stdoutOutput := <-stdoutChan
	stderrOutput := <-stderrChan

	// Verify: Still returns nil (graceful)
	if err != nil {
		t.Errorf("Expected nil error, got: %v", err)
	}

	// Verify: Hook response valid
	var hookResponse types.HookResponse
	if err := json.Unmarshal([]byte(stdoutOutput), &hookResponse); err != nil {
		t.Fatalf("Failed to parse hook response: %v", err)
	}

	if !hookResponse.Continue {
		t.Error("Expected Continue=true even on upload failure")
	}

	// Verify: Error message about upload failure
	if !strings.Contains(stderrOutput, "Cloud upload failed") {
		t.Errorf("Expected upload failure message in stderr, got: %s", stderrOutput)
	}
}
