package logger

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
)

// captureLogOutput redirects log output through a pipe so tests can read the
// emitted JSON lines.
func captureLogOutput(t *testing.T, f func()) []byte {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	t.Cleanup(SetOutputForTest(w))

	f()

	if err := w.Close(); err != nil {
		t.Fatalf("close pipe: %v", err)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return buf.Bytes()
}

func TestInfoEmitsJSONLine(t *testing.T) {
	out := captureLogOutput(t, func() {
		Info("hello", "user", "alice", "count", 3)
	})

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d: %q", len(lines), string(out))
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &decoded); err != nil {
		t.Fatalf("not valid JSON: %v (line=%q)", err, lines[0])
	}
	if decoded["msg"] != "hello" {
		t.Errorf("msg = %v, want hello", decoded["msg"])
	}
	if decoded["user"] != "alice" {
		t.Errorf("user = %v, want alice", decoded["user"])
	}
	if decoded["level"] != "INFO" {
		t.Errorf("level = %v, want INFO", decoded["level"])
	}
}

func TestWarnAndErrorLevels(t *testing.T) {
	out := captureLogOutput(t, func() {
		Warn("careful")
		Error("oops")
	})
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), string(out))
	}
	for i, want := range []string{"WARN", "ERROR"} {
		var decoded map[string]any
		if err := json.Unmarshal([]byte(lines[i]), &decoded); err != nil {
			t.Fatalf("line %d not valid JSON: %v", i, err)
		}
		if decoded["level"] != want {
			t.Errorf("line %d level = %v, want %s", i, decoded["level"], want)
		}
	}
}

func TestIsDebugAndSetDebugForTest(t *testing.T) {
	cleanup := SetDebugForTest(true)
	if !IsDebug() {
		t.Error("IsDebug() = false after SetDebugForTest(true)")
	}
	cleanup()

	cleanup = SetDebugForTest(false)
	if IsDebug() {
		t.Error("IsDebug() = true after SetDebugForTest(false)")
	}
	cleanup()
}

func TestSetOutputForTestRestoresOriginalHandler(t *testing.T) {
	// Smoke test: applying and reverting SetOutputForTest should leave Info
	// callable without panic.
	r, w, _ := os.Pipe()
	restore := SetOutputForTest(w)
	Info("redirected")
	restore()
	if err := w.Close(); err != nil {
		t.Fatalf("close pipe: %v", err)
	}
	_, _ = io.ReadAll(r) // drain

	// After restore, Info should still work without panicking.
	Info("after-restore")
}

// TestSlogDefaultSync verifies that the package keeps slog.Default in sync —
// SetOutputForTest swaps both the package log and slog.Default.
func TestSlogDefaultSync(t *testing.T) {
	r, w, _ := os.Pipe()
	restore := SetOutputForTest(w)
	t.Cleanup(func() {
		restore()
		_ = w.Close()
		_, _ = io.ReadAll(r)
	})

	slog.Default().Info("via-slog-default")
	_ = w.Close()
	out, _ := io.ReadAll(r)
	if !strings.Contains(string(out), "via-slog-default") {
		t.Errorf("slog.Default did not pick up SetOutputForTest: %q", out)
	}
}
