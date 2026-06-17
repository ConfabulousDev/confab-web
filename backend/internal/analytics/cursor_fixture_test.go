package analytics

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// cursorFixturePath locates the sanitized Cursor wire-format fixture committed
// for downstream tickets (gevp parser/analytics, 18n2 frontend service tests).
// It must be readable without touching ~/.cursor — the acceptance criterion of
// fy5q. The fixture documents Cursor's agent-transcript JSONL line shapes:
//
//	{"role":..., "message":{"content":[...]}}        (conversation rows)
//	{"type":"turn_ended","status":"success"}         (turn marker)
//	{"type":"turn_ended","status":"error","error":..}(turn marker, error)
//
// Crucially this is NOT Claude Code JSONL: conversation rows carry no top-level
// type/uuid/timestamp/usage/model fields.
var cursorFixturePath = filepath.Join("testdata", "cursor", "main.jsonl")

// cursorLine mirrors the union of the two Cursor line shapes for parsing checks.
type cursorLine struct {
	// Conversation rows.
	Role    string `json:"role,omitempty"`
	Message *struct {
		Content []cursorBlock `json:"content"`
	} `json:"message,omitempty"`

	// turn_ended marker rows.
	Type   string `json:"type,omitempty"`
	Status string `json:"status,omitempty"`
	Error  string `json:"error,omitempty"`
}

type cursorBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

// eachCursorLine invokes fn for every non-empty JSONL line in the fixture,
// passing the 1-based line number and the trimmed raw bytes. It fails the test
// if the fixture is unreadable or the scan errors.
func eachCursorLine(t *testing.T, fn func(n int, raw []byte)) {
	t.Helper()
	content, err := os.ReadFile(cursorFixturePath)
	if err != nil {
		t.Fatalf("read cursor fixture: %v", err)
	}
	sc := bufio.NewScanner(bytes.NewReader(content))
	sc.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	for n := 1; sc.Scan(); n++ {
		raw := bytes.TrimSpace(sc.Bytes())
		if len(raw) == 0 {
			continue
		}
		fn(n, raw)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan cursor fixture: %v", err)
	}
}

// readCursorFixture loads every JSONL line from the fixture, failing the test if
// any line is not a valid JSON object. Returns the parsed lines in file order.
func readCursorFixture(t *testing.T) []cursorLine {
	t.Helper()
	var lines []cursorLine
	eachCursorLine(t, func(n int, raw []byte) {
		var line cursorLine
		if err := json.Unmarshal(raw, &line); err != nil {
			t.Fatalf("line %d is not a valid JSON object: %v", n, err)
		}
		lines = append(lines, line)
	})
	if len(lines) == 0 {
		t.Fatal("cursor fixture is empty")
	}
	return lines
}

// TestCursorFixtureParses confirms the committed fixture is valid JSONL and is
// readable from the package directory (no ~/.cursor access).
func TestCursorFixtureParses(t *testing.T) {
	lines := readCursorFixture(t)
	if len(lines) < 4 {
		t.Fatalf("expected a fixture with structural variety, got %d lines", len(lines))
	}
}

// TestCursorFixtureHasConversationVariety verifies the fixture exercises both
// user and assistant conversation rows, and that at least one assistant row
// mixes a text block with tool_use blocks.
func TestCursorFixtureHasConversationVariety(t *testing.T) {
	lines := readCursorFixture(t)

	var sawUser, sawAssistant, sawMixedAssistant bool
	for _, l := range lines {
		switch l.Role {
		case "user":
			sawUser = true
			if l.Message == nil || len(l.Message.Content) == 0 {
				t.Error("user row must carry message.content blocks")
			}
			for _, b := range l.Message.Content {
				if b.Type != "text" {
					t.Errorf("user rows are text-only; saw block type %q", b.Type)
				}
			}
		case "assistant":
			sawAssistant = true
			if l.Message == nil {
				t.Fatal("assistant row missing message")
			}
			var hasText, hasTool bool
			for _, b := range l.Message.Content {
				switch b.Type {
				case "text":
					hasText = true
				case "tool_use":
					hasTool = true
				}
			}
			if !hasText {
				t.Error("every assistant row must contain >=1 text block")
			}
			if hasText && hasTool {
				sawMixedAssistant = true
			}
		}
	}

	if !sawUser {
		t.Error("fixture must contain at least one user row")
	}
	if !sawAssistant {
		t.Error("fixture must contain at least one assistant row")
	}
	if !sawMixedAssistant {
		t.Error("fixture must contain an assistant row mixing text + tool_use")
	}
}

// TestCursorFixtureToolCoverage verifies the fixture includes the load-bearing
// tools downstream cards key on — StrReplace (Cursor's file-EDIT tool), plus
// Read/Shell/Write — and that every tool_use block carries name + input with no
// id, and uses `path` (not `file_path`) for file tools.
func TestCursorFixtureToolCoverage(t *testing.T) {
	lines := readCursorFixture(t)

	seen := map[string]bool{}
	for _, l := range lines {
		if l.Role != "assistant" || l.Message == nil {
			continue
		}
		for _, b := range l.Message.Content {
			if b.Type != "tool_use" {
				continue
			}
			if b.Name == "" {
				t.Error("tool_use block missing name")
			}
			if len(b.Input) == 0 {
				t.Errorf("tool_use %q missing input", b.Name)
			}
			seen[b.Name] = true

			// File tools must key on `path`, not `file_path`.
			switch b.Name {
			case "Read", "Write", "StrReplace", "Delete":
				var in map[string]json.RawMessage
				if err := json.Unmarshal(b.Input, &in); err != nil {
					t.Fatalf("tool %q input not an object: %v", b.Name, err)
				}
				if _, ok := in["path"]; !ok {
					t.Errorf("file tool %q must use `path` key, got keys %v", b.Name, keys(in))
				}
				if _, bad := in["file_path"]; bad {
					t.Errorf("file tool %q must not use `file_path`", b.Name)
				}
			}
		}
	}

	for _, want := range []string{"StrReplace", "Read", "Shell", "Write"} {
		if !seen[want] {
			t.Errorf("fixture must include a %q tool_use", want)
		}
	}
}

// TestCursorFixtureTurnMarkers verifies the fixture includes both a successful
// and an error turn_ended marker, and that error rows carry an `error` string.
func TestCursorFixtureTurnMarkers(t *testing.T) {
	lines := readCursorFixture(t)

	var sawSuccess, sawError bool
	for _, l := range lines {
		if l.Type != "turn_ended" {
			continue
		}
		switch l.Status {
		case "success":
			sawSuccess = true
		case "error":
			sawError = true
			if l.Error == "" {
				t.Error("turn_ended error row must carry an `error` message")
			}
		default:
			t.Errorf("unexpected turn_ended status %q", l.Status)
		}
	}

	if !sawSuccess {
		t.Error("fixture must include a turn_ended success row")
	}
	if !sawError {
		t.Error("fixture must include a turn_ended error row")
	}
}

// TestCursorFixtureNotClaudeEnvelope guards the doc's central claim: Cursor
// conversation rows are NOT Claude Code JSONL. They must not carry the
// top-level type/uuid/timestamp/usage/model fields that Claude's TranscriptLine
// (parser.go) keys on, and tool_use blocks carry no id.
func TestCursorFixtureNotClaudeEnvelope(t *testing.T) {
	eachCursorLine(t, func(n int, raw []byte) {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			t.Fatalf("line %d invalid JSON: %v", n, err)
		}

		// Only conversation rows (role present) are constrained here.
		if _, isConvo := obj["role"]; !isConvo {
			return
		}

		// Conversation rows must not carry Claude top-level fields.
		for _, claudeField := range []string{"type", "uuid", "timestamp", "usage", "model"} {
			if _, ok := obj[claudeField]; ok {
				t.Errorf("line %d: conversation row must not carry Claude-style top-level %q", n, claudeField)
			}
		}

		// tool_use blocks must carry only type/name/input — no id.
		var conv struct {
			Message struct {
				Content []map[string]json.RawMessage `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal(raw, &conv); err != nil {
			t.Fatalf("line %d: re-decode content: %v", n, err)
		}
		for _, b := range conv.Message.Content {
			var typ string
			if err := json.Unmarshal(b["type"], &typ); err != nil {
				continue
			}
			if typ == "tool_use" {
				if _, ok := b["id"]; ok {
					t.Errorf("line %d: Cursor tool_use blocks carry no id", n)
				}
			}
		}
	})
}

func keys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
