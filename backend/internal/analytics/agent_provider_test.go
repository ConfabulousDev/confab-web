package analytics

import (
	"context"
	"fmt"
	"io"
	"testing"
)

func TestNewAgentProvider_AllSucceed(t *testing.T) {
	agentJsonl := makeAssistantMessage("aa1", "2025-01-01T00:00:01Z", "claude-haiku-3", 50, 25, []map[string]interface{}{
		makeTextBlock("Agent"),
	}) + "\n"

	agents := []AgentFileInfo{
		{FileName: "agent-abc.jsonl", AgentID: "abc"},
		{FileName: "agent-def.jsonl", AgentID: "def"},
	}

	download := func(_ context.Context, fileName string) ([]byte, error) {
		return []byte(agentJsonl), nil
	}

	provider := NewAgentProvider(agents, download, 0)
	ctx := context.Background()

	tf1, err := provider(ctx)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if tf1.AgentID != "abc" {
		t.Errorf("first agent ID = %q, want %q", tf1.AgentID, "abc")
	}

	tf2, err := provider(ctx)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if tf2.AgentID != "def" {
		t.Errorf("second agent ID = %q, want %q", tf2.AgentID, "def")
	}

	_, err = provider(ctx)
	if err != io.EOF {
		t.Errorf("third call: got %v, want io.EOF", err)
	}
}

func TestNewAgentProvider_DownloadError_Skipped(t *testing.T) {
	agentJsonl := makeAssistantMessage("aa1", "2025-01-01T00:00:01Z", "claude-haiku-3", 50, 25, []map[string]interface{}{
		makeTextBlock("Agent"),
	}) + "\n"

	agents := []AgentFileInfo{
		{FileName: "agent-bad.jsonl", AgentID: "bad"},
		{FileName: "agent-good.jsonl", AgentID: "good"},
	}

	download := func(_ context.Context, fileName string) ([]byte, error) {
		if fileName == "agent-bad.jsonl" {
			return nil, fmt.Errorf("network error")
		}
		return []byte(agentJsonl), nil
	}

	provider := NewAgentProvider(agents, download, 0)
	ctx := context.Background()

	// Should skip "bad" and return "good"
	tf, err := provider(ctx)
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if tf.AgentID != "good" {
		t.Errorf("agent ID = %q, want %q", tf.AgentID, "good")
	}

	_, err = provider(ctx)
	if err != io.EOF {
		t.Errorf("expected io.EOF, got %v", err)
	}
}

func TestNewAgentProvider_EmptyContent_Skipped(t *testing.T) {
	agentJsonl := makeAssistantMessage("aa1", "2025-01-01T00:00:01Z", "claude-haiku-3", 50, 25, []map[string]interface{}{
		makeTextBlock("Agent"),
	}) + "\n"

	agents := []AgentFileInfo{
		{FileName: "agent-empty.jsonl", AgentID: "empty"},
		{FileName: "agent-good.jsonl", AgentID: "good"},
	}

	download := func(_ context.Context, fileName string) ([]byte, error) {
		if fileName == "agent-empty.jsonl" {
			return nil, nil // empty
		}
		return []byte(agentJsonl), nil
	}

	provider := NewAgentProvider(agents, download, 0)
	ctx := context.Background()

	tf, err := provider(ctx)
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if tf.AgentID != "good" {
		t.Errorf("agent ID = %q, want %q", tf.AgentID, "good")
	}
}

func TestNewAgentProvider_Cap(t *testing.T) {
	agentJsonl := makeAssistantMessage("aa1", "2025-01-01T00:00:01Z", "claude-haiku-3", 50, 25, []map[string]interface{}{
		makeTextBlock("Agent"),
	}) + "\n"

	agents := []AgentFileInfo{
		{FileName: "agent-1.jsonl", AgentID: "1"},
		{FileName: "agent-2.jsonl", AgentID: "2"},
		{FileName: "agent-3.jsonl", AgentID: "3"},
	}

	download := func(_ context.Context, fileName string) ([]byte, error) {
		return []byte(agentJsonl), nil
	}

	provider := NewAgentProvider(agents, download, 2)
	ctx := context.Background()

	tf1, err := provider(ctx)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if tf1.AgentID != "1" {
		t.Errorf("first ID = %q, want %q", tf1.AgentID, "1")
	}

	tf2, err := provider(ctx)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if tf2.AgentID != "2" {
		t.Errorf("second ID = %q, want %q", tf2.AgentID, "2")
	}

	// Cap reached — should return EOF, not download 3rd
	_, err = provider(ctx)
	if err != io.EOF {
		t.Errorf("third call: got %v, want io.EOF", err)
	}
}

func TestNewAgentProvider_NoAgents(t *testing.T) {
	provider := NewAgentProvider(nil, nil, 0)
	_, err := provider(context.Background())
	if err != io.EOF {
		t.Errorf("got %v, want io.EOF", err)
	}
}
