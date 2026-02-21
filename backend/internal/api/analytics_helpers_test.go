package api

import (
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/db"
)

func TestClassifySessionFiles(t *testing.T) {
	t.Run("returns nil when no files", func(t *testing.T) {
		result := classifySessionFiles(nil)
		if result != nil {
			t.Error("expected nil for empty input")
		}
	})

	t.Run("returns nil when no transcript file", func(t *testing.T) {
		files := []db.SyncFileDetail{
			{FileName: "agent-abc.jsonl", FileType: "agent", LastSyncedLine: 10},
			{FileName: "agent-def.jsonl", FileType: "agent", LastSyncedLine: 20},
		}
		result := classifySessionFiles(files)
		if result != nil {
			t.Error("expected nil when no transcript file")
		}
	})

	t.Run("transcript only", func(t *testing.T) {
		files := []db.SyncFileDetail{
			{FileName: "transcript.jsonl", FileType: "transcript", LastSyncedLine: 42},
		}
		result := classifySessionFiles(files)
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.transcript == nil {
			t.Fatal("expected transcript to be set")
		}
		if result.transcript.FileName != "transcript.jsonl" {
			t.Errorf("transcript file name = %q, want %q", result.transcript.FileName, "transcript.jsonl")
		}
		if len(result.agents) != 0 {
			t.Errorf("expected 0 agents, got %d", len(result.agents))
		}
		if result.lineCount != 42 {
			t.Errorf("lineCount = %d, want 42", result.lineCount)
		}
	})

	t.Run("transcript plus agents sums line counts", func(t *testing.T) {
		files := []db.SyncFileDetail{
			{FileName: "transcript.jsonl", FileType: "transcript", LastSyncedLine: 100},
			{FileName: "agent-abc.jsonl", FileType: "agent", LastSyncedLine: 30},
			{FileName: "agent-def.jsonl", FileType: "agent", LastSyncedLine: 20},
		}
		result := classifySessionFiles(files)
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.transcript == nil {
			t.Fatal("expected transcript to be set")
		}
		if len(result.agents) != 2 {
			t.Errorf("expected 2 agents, got %d", len(result.agents))
		}
		if result.lineCount != 150 {
			t.Errorf("lineCount = %d, want 150 (100+30+20)", result.lineCount)
		}
	})

	t.Run("ignores unknown file types", func(t *testing.T) {
		files := []db.SyncFileDetail{
			{FileName: "transcript.jsonl", FileType: "transcript", LastSyncedLine: 10},
			{FileName: "unknown.jsonl", FileType: "other", LastSyncedLine: 999},
		}
		result := classifySessionFiles(files)
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if len(result.agents) != 0 {
			t.Errorf("expected 0 agents, got %d", len(result.agents))
		}
		if result.lineCount != 10 {
			t.Errorf("lineCount = %d, want 10 (unknown type should be excluded)", result.lineCount)
		}
	})

	t.Run("transcript pointer references original slice element", func(t *testing.T) {
		files := []db.SyncFileDetail{
			{FileName: "transcript.jsonl", FileType: "transcript", LastSyncedLine: 5},
		}
		result := classifySessionFiles(files)
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		// Verify the pointer references the original slice element (not a copy)
		if result.transcript != &files[0] {
			t.Error("transcript should point to the original slice element")
		}
	})
}
