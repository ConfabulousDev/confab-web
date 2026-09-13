package analytics

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// TestSmartRecapLiveCompare runs one real Claude Code transcript through both
// LLM vendors and logs the recaps side by side. It makes live, billed API calls
// and is opt-in only: it skips unless SMART_RECAP_LIVE_COMPARE=1,
// ANTHROPIC_API_KEY, OPENAI_API_KEY, and SMART_RECAP_COMPARE_JSONL are all set.
// CI sets none of these. It is also the live check that the OpenAI model accepts
// reasoning effort "none" and the strict schema.
//
// Models default to claude-haiku-4-5-20251001 and gpt-5.6-luna; override with
// SMART_RECAP_COMPARE_ANTHROPIC_MODEL / SMART_RECAP_COMPARE_OPENAI_MODEL.
func TestSmartRecapLiveCompare(t *testing.T) {
	if testing.Short() {
		t.Skip("live comparison makes real API calls")
	}
	if os.Getenv("SMART_RECAP_LIVE_COMPARE") != "1" {
		t.Skip("set SMART_RECAP_LIVE_COMPARE=1 to run the live smart recap vendor comparison")
	}
	for _, k := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "SMART_RECAP_COMPARE_JSONL"} {
		if os.Getenv(k) == "" {
			t.Skipf("set %s to run the live smart recap vendor comparison", k)
		}
	}

	path := os.Getenv("SMART_RECAP_COMPARE_JSONL")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	fc, err := NewFileCollection(data)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	vendors := []struct {
		provider, keyVar, modelVar, defaultModel string
	}{
		{LLMProviderAnthropic, "ANTHROPIC_API_KEY", "SMART_RECAP_COMPARE_ANTHROPIC_MODEL", "claude-haiku-4-5-20251001"},
		{LLMProviderOpenAI, "OPENAI_API_KEY", "SMART_RECAP_COMPARE_OPENAI_MODEL", "gpt-5.6-luna"},
	}
	for _, v := range vendors {
		t.Run(v.provider, func(t *testing.T) {
			model := os.Getenv(v.modelVar)
			if model == "" {
				model = v.defaultModel
			}
			llm, err := newRecapLLM(v.provider, os.Getenv(v.keyVar), "")
			if err != nil {
				t.Fatalf("newRecapLLM: %v", err)
			}
			analyzer := NewSmartRecapAnalyzer(llm, model, SmartRecapAnalyzerConfig{})

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			result, err := analyzer.Analyze(ctx, GenerateInput{FileCollection: fc}, nil)
			if err != nil {
				t.Fatalf("%s (%s) failed: %v", v.provider, model, err)
			}

			pretty, _ := json.MarshalIndent(result, "", "  ")
			t.Logf("%s (%s): input_tokens=%d output_tokens=%d generation_ms=%d\n%s",
				v.provider, model, result.InputTokens, result.OutputTokens, result.GenerationTimeMs, pretty)
		})
	}
}
