package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// =============================================================================
// Config resolution (SMART_RECAP_LLM_PROVIDER)
// =============================================================================

func TestResolveSmartRecapLLMConfig(t *testing.T) {
	tests := []struct {
		name        string
		env         map[string]string
		wantErr     bool
		wantVendor  string
		wantKey     string
		wantMissing string
	}{
		{
			name:       "unset provider defaults to anthropic",
			env:        map[string]string{"ANTHROPIC_API_KEY": "ak", "OPENAI_API_KEY": "ok", "SMART_RECAP_MODEL": "claude-haiku-4-5"},
			wantVendor: LLMProviderAnthropic,
			wantKey:    "ak",
		},
		{
			name:       "openai uses OPENAI_API_KEY, not ANTHROPIC_API_KEY",
			env:        map[string]string{"SMART_RECAP_LLM_PROVIDER": "openai", "ANTHROPIC_API_KEY": "ak", "OPENAI_API_KEY": "ok", "SMART_RECAP_MODEL": "gpt-5.6-luna"},
			wantVendor: LLMProviderOpenAI,
			wantKey:    "ok",
		},
		{
			name:       "surrounding whitespace is trimmed",
			env:        map[string]string{"SMART_RECAP_LLM_PROVIDER": "  openai \n", "OPENAI_API_KEY": "ok", "SMART_RECAP_MODEL": "gpt-5.6-luna"},
			wantVendor: LLMProviderOpenAI,
			wantKey:    "ok",
		},
		{
			name:       "explicit anthropic",
			env:        map[string]string{"SMART_RECAP_LLM_PROVIDER": "anthropic", "ANTHROPIC_API_KEY": "ak", "SMART_RECAP_MODEL": "m"},
			wantVendor: LLMProviderAnthropic,
			wantKey:    "ak",
		},
		{
			name:        "openai with only ANTHROPIC_API_KEY reports OPENAI_API_KEY missing",
			env:         map[string]string{"SMART_RECAP_LLM_PROVIDER": "openai", "ANTHROPIC_API_KEY": "ak", "SMART_RECAP_MODEL": "gpt-5.6-luna"},
			wantVendor:  LLMProviderOpenAI,
			wantMissing: "OPENAI_API_KEY",
		},
		{
			name:        "anthropic without key reports ANTHROPIC_API_KEY missing",
			env:         map[string]string{"OPENAI_API_KEY": "ok", "SMART_RECAP_MODEL": "m"},
			wantVendor:  LLMProviderAnthropic,
			wantMissing: "ANTHROPIC_API_KEY",
		},
		{
			name:        "missing model is reported",
			env:         map[string]string{"ANTHROPIC_API_KEY": "ak"},
			wantVendor:  LLMProviderAnthropic,
			wantKey:     "ak",
			wantMissing: "SMART_RECAP_MODEL",
		},
		{
			name:        "missing key is reported before missing model",
			env:         map[string]string{},
			wantVendor:  LLMProviderAnthropic,
			wantMissing: "ANTHROPIC_API_KEY",
		},
		{name: "unknown provider is an error", env: map[string]string{"SMART_RECAP_LLM_PROVIDER": "gemini"}, wantErr: true},
		{name: "provider match is case-sensitive", env: map[string]string{"SMART_RECAP_LLM_PROVIDER": "OpenAI"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getenv := func(k string) string { return tt.env[k] }
			cfg, err := ResolveSmartRecapLLMConfig(getenv)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got config %+v", cfg)
				}
				if !strings.Contains(err.Error(), "SMART_RECAP_LLM_PROVIDER") {
					t.Errorf("error should name SMART_RECAP_LLM_PROVIDER, got %q", err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Provider != tt.wantVendor {
				t.Errorf("Provider = %q, want %q", cfg.Provider, tt.wantVendor)
			}
			if cfg.APIKey != tt.wantKey {
				t.Errorf("APIKey = %q, want %q", cfg.APIKey, tt.wantKey)
			}
			if cfg.Model != tt.env["SMART_RECAP_MODEL"] {
				t.Errorf("Model = %q, want %q", cfg.Model, tt.env["SMART_RECAP_MODEL"])
			}
			if cfg.MissingVar != tt.wantMissing {
				t.Errorf("MissingVar = %q, want %q", cfg.MissingVar, tt.wantMissing)
			}
		})
	}
}

// =============================================================================
// Vendor switch
// =============================================================================

func TestNewRecapLLM_RejectsUnknownProvider(t *testing.T) {
	for _, p := range []string{"", "gemini", "OpenAI"} {
		if _, err := newRecapLLM(p, "k", ""); err == nil {
			t.Errorf("newRecapLLM(%q) should error", p)
		}
	}
	for _, p := range []string{LLMProviderAnthropic, LLMProviderOpenAI} {
		if llm, err := newRecapLLM(p, "k", ""); err != nil || llm == nil {
			t.Errorf("newRecapLLM(%q) = %v, %v; want a client", p, llm, err)
		}
	}
}

// =============================================================================
// Anthropic adapter: wire request must stay byte-for-byte what it was
// =============================================================================

func TestAnthropicRecapLLM_SendsUnchangedPrefillRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("path = %s, want /v1/messages", r.URL.Path)
		}
		if r.Header.Get("X-API-Key") != "ak" {
			t.Errorf("X-API-Key = %q", r.Header.Get("X-API-Key"))
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		want := map[string]any{
			"model":       "claude-haiku-4-5-20251001",
			"max_tokens":  float64(1000),
			"temperature": 0.25,
			"system":      "SYSTEM",
			"messages": []any{
				map[string]any{"role": "user", "content": "USER"},
				map[string]any{"role": "assistant", "content": "{"},
			},
		}
		if !reflect.DeepEqual(body, want) {
			t.Errorf("anthropic request body changed:\n got: %v\nwant: %v", body, want)
		}
		_, _ = w.Write([]byte(`{"id":"msg","type":"message","role":"assistant","content":[{"type":"text","text":"\"recap\": \"hi\"}"}],"usage":{"input_tokens":11,"output_tokens":7}}`))
	}))
	defer server.Close()

	llm, err := newRecapLLM(LLMProviderAnthropic, "ak", server.URL)
	if err != nil {
		t.Fatalf("newRecapLLM: %v", err)
	}
	resp, err := llm.Generate(context.Background(), recapLLMRequest{
		Model: "claude-haiku-4-5-20251001", System: "SYSTEM", User: "USER", MaxOutputTokens: 1000,
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp.Text != `{"recap": "hi"}` {
		t.Errorf("Text = %q, want prefill-prepended JSON", resp.Text)
	}
	if resp.InputTokens != 11 || resp.OutputTokens != 7 {
		t.Errorf("tokens = %d/%d, want 11/7", resp.InputTokens, resp.OutputTokens)
	}
}

// =============================================================================
// OpenAI adapter
// =============================================================================

func TestOpenAIRecapLLM_SendsStrictSchemaRequestWithoutTemperature(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Errorf("path = %s, want /v1/responses", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer ok" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body["model"] != "gpt-5.6-luna" || body["instructions"] != "SYSTEM" || body["input"] != "USER" {
			t.Errorf("model/instructions/input = %v/%v/%v", body["model"], body["instructions"], body["input"])
		}
		if body["max_output_tokens"] != float64(1000) {
			t.Errorf("max_output_tokens = %v", body["max_output_tokens"])
		}
		if !reflect.DeepEqual(body["reasoning"], map[string]any{"effort": "none"}) {
			t.Errorf("reasoning = %v, want effort none", body["reasoning"])
		}
		if v, ok := body["store"]; !ok || v != false {
			t.Errorf("store = %v (present=%v), want explicit false", v, ok)
		}
		for _, k := range []string{"temperature", "top_p"} {
			if _, ok := body[k]; ok {
				t.Errorf("request must not include %q", k)
			}
		}
		format := body["text"].(map[string]any)["format"].(map[string]any)
		if format["type"] != "json_schema" || format["name"] != "smart_recap" || format["strict"] != true {
			t.Errorf("text.format = %v", format)
		}
		var wantSchema any
		if err := json.Unmarshal([]byte(smartRecapJSONSchema), &wantSchema); err != nil {
			t.Fatalf("smartRecapJSONSchema is not valid JSON: %v", err)
		}
		if !reflect.DeepEqual(format["schema"], wantSchema) {
			t.Errorf("schema sent does not match smartRecapJSONSchema")
		}
		_, _ = w.Write([]byte(`{"id":"r","status":"completed","output":[{"type":"reasoning"},{"type":"message","content":[{"type":"output_text","text":"{\"recap\":"},{"type":"output_text","text":"\"hi\"}"}]}],"usage":{"input_tokens":21,"output_tokens":9}}`))
	}))
	defer server.Close()

	llm, err := newRecapLLM(LLMProviderOpenAI, "ok", server.URL)
	if err != nil {
		t.Fatalf("newRecapLLM: %v", err)
	}
	resp, err := llm.Generate(context.Background(), recapLLMRequest{
		Model: "gpt-5.6-luna", System: "SYSTEM", User: "USER", MaxOutputTokens: 1000,
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp.Text != `{"recap":"hi"}` {
		t.Errorf("Text = %q", resp.Text)
	}
	if resp.InputTokens != 21 || resp.OutputTokens != 9 {
		t.Errorf("tokens = %d/%d, want 21/9", resp.InputTokens, resp.OutputTokens)
	}
}

func TestOpenAIRecapLLM_TreatsUnusableResponsesAsFailures(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		wantInErr string
	}{
		{
			name:      "incomplete status names the reason",
			body:      `{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output":[{"type":"message","content":[{"type":"output_text","text":"{\"recap\":"}]}]}`,
			wantInErr: "max_output_tokens",
		},
		{
			name:      "refusal includes the refusal text",
			body:      `{"status":"completed","output":[{"type":"message","content":[{"type":"refusal","refusal":"I can't help with that"}]}]}`,
			wantInErr: "I can't help with that",
		},
		{
			name:      "no output items",
			body:      `{"status":"completed","output":[]}`,
			wantInErr: "no output",
		},
		{
			name:      "only reasoning, no message text",
			body:      `{"status":"completed","output":[{"type":"reasoning"},{"type":"message","content":[{"type":"output_text","text":""}]}]}`,
			wantInErr: "no output",
		},
		{
			name:      "HTTP error",
			status:    http.StatusBadRequest,
			body:      `{"error":{"message":"bad effort","type":"invalid_request_error","code":null}}`,
			wantInErr: "bad effort",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.status != 0 {
					w.WriteHeader(tt.status)
				}
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			llm, err := newRecapLLM(LLMProviderOpenAI, "ok", server.URL)
			if err != nil {
				t.Fatalf("newRecapLLM: %v", err)
			}
			_, err = llm.Generate(context.Background(), recapLLMRequest{Model: "m", System: "s", User: "u", MaxOutputTokens: 10})
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tt.wantInErr) {
				t.Errorf("error %q should contain %q", err.Error(), tt.wantInErr)
			}
		})
	}
}

// =============================================================================
// Schema drift guard: smartRecapJSONSchema vs Go structs
// =============================================================================

func jsonTagNames(t reflect.Type) []string {
	var names []string
	for field := range t.Fields() {
		tag := field.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		names = append(names, strings.Split(tag, ",")[0])
	}
	sort.Strings(names)
	return names
}

func schemaPropKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedStrings(v any) []string {
	arr, _ := v.([]any)
	out := make([]string, 0, len(arr))
	for _, s := range arr {
		out = append(out, s.(string))
	}
	sort.Strings(out)
	return out
}

// assertStrictObjects walks every object schema and checks OpenAI strict-mode rules.
func assertStrictObjects(t *testing.T, path string, node map[string]any) {
	t.Helper()
	if node["type"] == "object" {
		if node["additionalProperties"] != false {
			t.Errorf("%s: additionalProperties must be false", path)
		}
		props, _ := node["properties"].(map[string]any)
		if !reflect.DeepEqual(schemaPropKeys(props), sortedStrings(node["required"])) {
			t.Errorf("%s: required %v must list every property %v", path, sortedStrings(node["required"]), schemaPropKeys(props))
		}
		for name, p := range props {
			assertStrictObjects(t, path+"."+name, p.(map[string]any))
		}
	}
	if items, ok := node["items"].(map[string]any); ok {
		assertStrictObjects(t, path+"[]", items)
	}
}

func TestSmartRecapJSONSchema_MatchesResultStructs(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal([]byte(smartRecapJSONSchema), &schema); err != nil {
		t.Fatalf("smartRecapJSONSchema is not valid JSON: %v", err)
	}
	assertStrictObjects(t, "$", schema)

	props := schema["properties"].(map[string]any)
	if got, want := schemaPropKeys(props), jsonTagNames(reflect.TypeFor[SmartRecapResult]()); !reflect.DeepEqual(got, want) {
		t.Errorf("top-level properties = %v, want SmartRecapResult json tags %v", got, want)
	}

	itemTags := jsonTagNames(reflect.TypeFor[AnnotatedItem]())
	resultType := reflect.TypeFor[SmartRecapResult]()
	for f := range resultType.Fields() {
		f := f
		name := strings.Split(f.Tag.Get("json"), ",")[0]
		if name == "" {
			continue
		}
		prop := props[name].(map[string]any)
		switch f.Type {
		case reflect.TypeFor[string]():
			if prop["type"] != "string" {
				t.Errorf("%s: type = %v, want string", name, prop["type"])
			}
		case reflect.TypeFor[[]AnnotatedItem]():
			if prop["type"] != "array" {
				t.Errorf("%s: type = %v, want array", name, prop["type"])
				continue
			}
			items := prop["items"].(map[string]any)
			itemProps := items["properties"].(map[string]any)
			if got := schemaPropKeys(itemProps); !reflect.DeepEqual(got, itemTags) {
				t.Errorf("%s items properties = %v, want AnnotatedItem tags %v", name, got, itemTags)
			}
			if !reflect.DeepEqual(itemProps["message_id"].(map[string]any)["type"], []any{"integer", "null"}) {
				t.Errorf("%s.message_id must be nullable integer", name)
			}
		default:
			t.Errorf("%s: unhandled Go type %v — extend the schema and this test", name, f.Type)
		}
	}
}

// =============================================================================
// Analyzer is vendor-agnostic
// =============================================================================

type fakeRecapLLM struct {
	got  recapLLMRequest
	resp recapLLMResponse
	err  error
}

func (f *fakeRecapLLM) Generate(_ context.Context, req recapLLMRequest) (recapLLMResponse, error) {
	f.got = req
	return f.resp, f.err
}

func TestSmartRecapAnalyzer_AnalyzeParsesAnyVendorOutput(t *testing.T) {
	fake := &fakeRecapLLM{resp: recapLLMResponse{
		// Strict-schema output: every field present, message_id null when irrelevant.
		Text:         `{"suggested_session_title":"T","recap":"R","went_well":[{"text":"good","message_id":1},{"text":"meh","message_id":null}],"went_bad":[],"human_suggestions":[],"environment_suggestions":[],"default_context_suggestions":[]}`,
		InputTokens:  300,
		OutputTokens: 40,
	}}
	analyzer := NewSmartRecapAnalyzer(fake, "gpt-5.6-luna", SmartRecapAnalyzerConfig{SystemPrompt: "SYS"})

	result, err := analyzer.Analyze(context.Background(), GenerateInput{
		Transcript: "<transcript>hello</transcript>",
		IDMap:      map[int]string{1: "uuid-1"},
	}, nil)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if fake.got.Model != "gpt-5.6-luna" || fake.got.System != "SYS" || fake.got.MaxOutputTokens != DefaultMaxOutputTokens {
		t.Errorf("request = %+v", fake.got)
	}
	if !strings.Contains(fake.got.User, "<transcript>hello</transcript>") {
		t.Errorf("user content should contain the transcript, got %q", fake.got.User)
	}
	if result.Recap != "R" || result.SuggestedSessionTitle != "T" {
		t.Errorf("result = %+v", result)
	}
	if len(result.WentWell) != 2 || result.WentWell[0].MessageID != "uuid-1" || result.WentWell[1].MessageID != "" {
		t.Errorf("WentWell = %+v, want uuid-1 resolved and null cleared", result.WentWell)
	}
	if result.InputTokens != 300 || result.OutputTokens != 40 {
		t.Errorf("tokens = %d/%d", result.InputTokens, result.OutputTokens)
	}
}

func TestSmartRecapAnalyzer_AnalyzePropagatesLLMError(t *testing.T) {
	boom := errors.New("boom")
	analyzer := NewSmartRecapAnalyzer(&fakeRecapLLM{err: boom}, "m", SmartRecapAnalyzerConfig{})
	_, err := analyzer.Analyze(context.Background(), GenerateInput{Transcript: "x"}, nil)
	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want wrapped boom", err)
	}
}
