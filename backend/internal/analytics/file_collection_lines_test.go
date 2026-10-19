package analytics

import (
	"strconv"
	"strings"
	"testing"
)

// Line-splitting and oversized-line contract for parseTranscriptFile (e9qd):
// one line over the cap must never abort the whole transcript.

func mustParseTranscript(t *testing.T, content string) *TranscriptFile {
	t.Helper()
	tf, err := parseTranscriptFile([]byte(content), "")
	if err != nil {
		t.Fatalf("parseTranscriptFile returned error %v; one line must never abort the whole transcript", err)
	}
	return tf
}

func TestParseTranscriptFile_LineBeyondOldScannerCap_Parses(t *testing.T) {
	big := makeUserMessage("u1", "2025-01-01T00:00:00Z", strings.Repeat("a", 11*1024*1024)) // over the old 10 MB cap
	tf := mustParseTranscript(t, big+"\n"+makeUserMessage("u2", "2025-01-01T00:00:01Z", "after")+"\n")
	if len(tf.ValidationErrors) != 0 {
		t.Errorf("ValidationErrors = %+v, want none for an 11 MB line under the cap", tf.ValidationErrors)
	}
	if len(tf.Lines) != 2 {
		t.Errorf("Lines = %d, want 2", len(tf.Lines))
	}
}

func TestParseTranscriptFile_LineOverCap_SkippedAsLineValidationError(t *testing.T) {
	first := makeUserMessage("u1", "2025-01-01T00:00:00Z", "before")
	// Pad a valid message out to one byte over the cap.
	overhead := len(makeUserMessage("u2", "2025-01-01T00:00:01Z", ""))
	over := makeUserMessage("u2", "2025-01-01T00:00:01Z", strings.Repeat("a", maxLineBytes+1-overhead))
	if len(over) != maxLineBytes+1 {
		t.Fatalf("built line of %d bytes, want %d", len(over), maxLineBytes+1)
	}
	last := makeUserMessage("u3", "2025-01-01T00:00:02Z", "after")

	tf := mustParseTranscript(t, first+"\n"+over+"\n"+last)

	if len(tf.ValidationErrors) != 1 {
		t.Fatalf("ValidationErrors = %d, want exactly 1 for the oversized line", len(tf.ValidationErrors))
	}
	ve := tf.ValidationErrors[0]
	if ve.Line != 2 {
		t.Errorf("LineValidationError.Line = %d, want 2", ve.Line)
	}
	if len(ve.RawJSON) > 200 || !strings.HasPrefix(ve.RawJSON, over[:50]) {
		t.Errorf("RawJSON = %q (len %d), want the line truncated to 200 chars", ve.RawJSON, len(ve.RawJSON))
	}
	wantMsg := "line exceeds 32 MiB cap (" + strconv.Itoa(maxLineBytes+1) + " bytes); skipped"
	if len(ve.Errors) != 1 || ve.Errors[0].Path != "(root)" || ve.Errors[0].Message != wantMsg {
		t.Errorf("Errors = %+v, want one (root) error %q", ve.Errors, wantMsg)
	}
	if len(tf.Lines) != 2 {
		t.Errorf("Lines = %d, want 2 (lines around the oversized one still parse)", len(tf.Lines))
	}
	if tf.TotalLines != 3 {
		t.Errorf("TotalLines = %d, want 3", tf.TotalLines)
	}
}

func TestParseTranscriptFile_LineExactlyAtCap_Kept(t *testing.T) {
	overhead := len(makeUserMessage("u1", "2025-01-01T00:00:00Z", ""))
	at := makeUserMessage("u1", "2025-01-01T00:00:00Z", strings.Repeat("a", maxLineBytes-overhead))
	if len(at) != maxLineBytes {
		t.Fatalf("built line of %d bytes, want %d", len(at), maxLineBytes)
	}
	tf := mustParseTranscript(t, at)
	if len(tf.ValidationErrors) != 0 || len(tf.Lines) != 1 {
		t.Errorf("ValidationErrors=%d Lines=%d, want 0 and 1 for a line exactly at the cap", len(tf.ValidationErrors), len(tf.Lines))
	}
}

// TotalLines and validation line numbers must match what the old
// bufio.Scanner produced for each input shape.
func TestParseTranscriptFile_LineSplittingParity(t *testing.T) {
	a := makeUserMessage("u1", "2025-01-01T00:00:00Z", "one")
	b := makeUserMessage("u2", "2025-01-01T00:00:01Z", "two")
	cases := []struct {
		name          string
		content       string
		wantTotal     int
		wantLines     int
		wantInvalidAt int // 0 = no validation error expected
		wantRawNoCRLF bool
	}{
		{name: "empty", content: "", wantTotal: 0},
		{name: "LF trailing newline", content: a + "\n" + b + "\n", wantTotal: 2, wantLines: 2},
		{name: "no trailing newline", content: a + "\n" + b, wantTotal: 2, wantLines: 2},
		{name: "CRLF", content: a + "\r\n" + b + "\r\n", wantTotal: 2, wantLines: 2},
		{name: "blank lines counted", content: a + "\n\n  \n" + b + "\n", wantTotal: 4, wantLines: 2},
		{name: "trailing blank line counted", content: a + "\n\n", wantTotal: 2, wantLines: 1},
		{name: "invalid line number after blanks", content: a + "\n\nnot json\r\n" + b, wantTotal: 4, wantLines: 2, wantInvalidAt: 3, wantRawNoCRLF: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tf := mustParseTranscript(t, c.content)
			if tf.TotalLines != c.wantTotal {
				t.Errorf("TotalLines = %d, want %d", tf.TotalLines, c.wantTotal)
			}
			if len(tf.Lines) != c.wantLines {
				t.Errorf("Lines = %d, want %d", len(tf.Lines), c.wantLines)
			}
			if c.wantInvalidAt == 0 {
				if len(tf.ValidationErrors) != 0 {
					t.Errorf("ValidationErrors = %+v, want none", tf.ValidationErrors)
				}
				return
			}
			if len(tf.ValidationErrors) != 1 || tf.ValidationErrors[0].Line != c.wantInvalidAt {
				t.Fatalf("ValidationErrors = %+v, want one at line %d", tf.ValidationErrors, c.wantInvalidAt)
			}
			if c.wantRawNoCRLF && tf.ValidationErrors[0].RawJSON != "not json" {
				t.Errorf("RawJSON = %q, want %q (line terminator stripped)", tf.ValidationErrors[0].RawJSON, "not json")
			}
		})
	}
}

func TestParseApplyPatch_LineBeyondOldScannerCap_CountsWholePatch(t *testing.T) {
	envelope := "*** Begin Patch\n*** Update File: a.go\n+" + strings.Repeat("x", 4*1024*1024+1) +
		"\n+b\n+c\n-d\n*** End Patch"
	langs := map[string]int{}
	files, added, removed := parseApplyPatch(envelope, langs)
	if files != 1 || added != 3 || removed != 1 {
		t.Errorf("parseApplyPatch = files %d, added %d, removed %d; want 1, 3, 1 (an over-4 MiB patch line must not stop counting)", files, added, removed)
	}
	if langs["go"] != 1 {
		t.Errorf("langs = %v, want go:1", langs)
	}
}

func TestParseApplyPatch_CRLFEnvelope_SameAsLF(t *testing.T) {
	lf := "*** Begin Patch\n*** Add File: a.go\n+x\n+y\n*** Delete File: b.py\n*** End Patch\n"
	crlf := strings.ReplaceAll(lf, "\n", "\r\n")
	lfLangs, crlfLangs := map[string]int{}, map[string]int{}
	f1, a1, r1 := parseApplyPatch(lf, lfLangs)
	f2, a2, r2 := parseApplyPatch(crlf, crlfLangs)
	if f1 != 2 || a1 != 2 || r1 != 0 {
		t.Fatalf("LF parseApplyPatch = %d,%d,%d; want 2,2,0", f1, a1, r1)
	}
	if f2 != f1 || a2 != a1 || r2 != r1 {
		t.Errorf("CRLF parseApplyPatch = %d,%d,%d; want %d,%d,%d", f2, a2, r2, f1, a1, r1)
	}
	if crlfLangs["go"] != 1 || crlfLangs["python"] != 1 {
		t.Errorf("CRLF langs = %v, want go:1 python:1 (no trailing \\r in paths)", crlfLangs)
	}
}

func TestParseApplyPatch_EmptyEnvelope_ZeroCounts(t *testing.T) {
	if f, a, r := parseApplyPatch("", nil); f != 0 || a != 0 || r != 0 {
		t.Errorf("parseApplyPatch(\"\") = %d,%d,%d; want 0,0,0", f, a, r)
	}
}
