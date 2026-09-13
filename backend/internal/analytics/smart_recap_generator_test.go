package analytics_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/anthropic"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/recapquota"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// mockAnthropicResponse returns a valid Anthropic API response with a complete
// smart recap JSON object (structured outputs; no prefill).
func mockAnthropicResponse() anthropic.MessagesResponse {
	return anthropic.MessagesResponse{
		ID:         "msg_test",
		Type:       "message",
		Role:       "assistant",
		StopReason: "end_turn",
		Content: []anthropic.ContentBlock{
			{
				Type: "text",
				Text: `{"suggested_session_title": "Test Session", "recap": "Test recap content.", "went_well": [{"text": "Good thing", "message_id": null}], "went_bad": [], "human_suggestions": [], "environment_suggestions": [], "default_context_suggestions": []}`,
			},
		},
		Usage: anthropic.Usage{
			InputTokens:  100,
			OutputTokens: 50,
		},
	}
}

// newMockAnthropicServer creates an HTTP test server that returns a valid smart recap response.
func newMockAnthropicServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := mockAnthropicResponse()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode mock response: %v", err)
		}
	}))
}

// makeTestFileCollection creates a minimal FileCollection with one user message.
func makeTestFileCollection(t *testing.T) *analytics.FileCollection {
	t.Helper()
	jsonl := `{"type":"user","message":{"role":"user","content":"Hello world"},"uuid":"u1","timestamp":"2025-01-01T00:00:00Z","parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
`
	fc, err := analytics.NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("failed to create FileCollection: %v", err)
	}
	return fc
}

// generatorTestFixture bundles the common dependencies for SmartRecapGenerator tests.
type generatorTestFixture struct {
	env        *testutil.TestEnvironment
	mockServer *httptest.Server
	conn       *sql.DB
	store      *analytics.Store
	generator  *analytics.SmartRecapGenerator
	user       *models.User
	sessionID  string
}

// newMockOpenAIServer creates an HTTP test server that answers the Responses API
// with the given status and output_text (empty text → no message item).
func newMockOpenAIServer(t *testing.T, status, text string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		output := []map[string]any{}
		if text != "" {
			output = append(output, map[string]any{
				"type":    "message",
				"content": []map[string]any{{"type": "output_text", "text": text}},
			})
		}
		body := map[string]any{
			"id":     "resp_test",
			"status": status,
			"output": output,
			"usage":  map[string]int{"input_tokens": 150, "output_tokens": 60},
		}
		if status == "incomplete" {
			body["incomplete_details"] = map[string]string{"reason": "max_output_tokens"}
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(body); err != nil {
			t.Errorf("failed to encode mock response: %v", err)
		}
	}))
}

// setupGeneratorTest creates the shared test fixture: test environment, mock Anthropic
// server, user, session, analytics store, and generator. Caller provides email and
// externalID to keep tests independent.
func setupGeneratorTest(t *testing.T, email, externalID string) *generatorTestFixture {
	t.Helper()
	mockServer := newMockAnthropicServer(t)
	t.Cleanup(mockServer.Close)
	return setupGeneratorTestWithLLM(t, email, externalID, analytics.LLMProviderAnthropic, mockServer)
}

// setupGeneratorTestWithLLM is setupGeneratorTest with an explicit LLM vendor and mock server.
func setupGeneratorTestWithLLM(t *testing.T, email, externalID, provider string, mockServer *httptest.Server) *generatorTestFixture {
	t.Helper()

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, email, "Test User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)
	conn := env.DB.Conn()
	store := analytics.NewStore(conn)

	generator := analytics.NewSmartRecapGenerator(store, env.DB, analytics.SmartRecapGeneratorConfig{
		Provider:          provider,
		APIKey:            "test-key",
		Model:             "test-model",
		GenerationTimeout: 10 * time.Second,
		BaseURL:           mockServer.URL,
	})

	return &generatorTestFixture{
		env:        env,
		mockServer: mockServer,
		conn:       conn,
		store:      store,
		generator:  generator,
		user:       user,
		sessionID:  sessionID,
	}
}

// generateWithDefaults runs Generate with a standard single-line FileCollection and default settings.
func (f *generatorTestFixture) generateWithDefaults(t *testing.T) *analytics.GenerateResult {
	t.Helper()
	fc := makeTestFileCollection(t)
	input := analytics.GenerateInput{
		SessionID:      f.sessionID,
		UserID:         f.user.ID,
		LineCount:      1,
		FileCollection: fc,
	}
	return f.generator.Generate(context.Background(), input, 60, false)
}

// requireSuccessfulGeneration asserts that the result has no error, returns a card,
// and verifies the card was persisted to the database with the expected recap text.
func (f *generatorTestFixture) requireSuccessfulGeneration(t *testing.T, result *analytics.GenerateResult) {
	t.Helper()
	if result.Error != nil {
		t.Fatalf("expected no error, got: %v", result.Error)
	}
	if result.Card == nil {
		t.Fatal("expected card to be returned")
	}
	if result.Card.Recap != "Test recap content." {
		t.Errorf("recap = %q, want %q", result.Card.Recap, "Test recap content.")
	}

	// Verify card was saved to the database
	card, err := f.store.GetSmartRecapCard(context.Background(), f.sessionID)
	if err != nil {
		t.Fatalf("GetSmartRecapCard failed: %v", err)
	}
	if card == nil {
		t.Fatal("card should be saved after successful generation")
	}
	if card.Recap != "Test recap content." {
		t.Errorf("saved recap = %q, want %q", card.Recap, "Test recap content.")
	}
}

// TestSmartRecapGenerator_NoQuotaRowSucceeds verifies that generation succeeds
// even when no quota row exists for the user. Increment() now UPSERTs, so the
// quota row is created automatically with count=1. This was the original bug:
// the worker path used GetCount() (doesn't create quota row), and Increment()
// failed for users without a row.
func TestSmartRecapGenerator_NoQuotaRowSucceeds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	f := setupGeneratorTest(t, "noquota@test.com", "test-session-noquota")

	// Do NOT call recapquota.GetOrCreate -- no quota row exists.
	// Increment() should UPSERT and create the row automatically.
	result := f.generateWithDefaults(t)

	f.requireSuccessfulGeneration(t, result)

	// Verify quota was auto-created with count=1
	count, err := recapquota.GetCount(context.Background(), f.conn, f.user.ID)
	if err != nil {
		t.Fatalf("GetCount failed: %v", err)
	}
	if count != 1 {
		t.Errorf("quota count = %d, want 1", count)
	}
}

func TestSmartRecapGenerator_QuotaIncrementSuccessAllowsGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	f := setupGeneratorTest(t, "quotaok@test.com", "test-session-quotaok")

	// Create quota row first (normal flow)
	_, err := recapquota.GetOrCreate(context.Background(), f.conn, f.user.ID)
	if err != nil {
		t.Fatalf("GetOrCreate failed: %v", err)
	}

	result := f.generateWithDefaults(t)

	f.requireSuccessfulGeneration(t, result)

	// Verify quota was incremented
	count, err := recapquota.GetCount(context.Background(), f.conn, f.user.ID)
	if err != nil {
		t.Fatalf("GetCount failed: %v", err)
	}
	if count != 1 {
		t.Errorf("quota count = %d, want 1", count)
	}
}

func TestSmartRecapGenerator_AnthropicCardRecordsVendor(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	f := setupGeneratorTest(t, "anthropicvendor@test.com", "test-session-anthropic-vendor")
	result := f.generateWithDefaults(t)
	f.requireSuccessfulGeneration(t, result)

	card, err := f.store.GetSmartRecapCard(context.Background(), f.sessionID)
	if err != nil {
		t.Fatalf("GetSmartRecapCard failed: %v", err)
	}
	if card.LLMProvider != analytics.LLMProviderAnthropic {
		t.Errorf("saved LLMProvider = %q, want anthropic", card.LLMProvider)
	}
}

func TestSmartRecapGenerator_OpenAIProviderPersistsVendorAndUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	mock := newMockOpenAIServer(t, "completed",
		`{"suggested_session_title":"Test Session","recap":"Test recap content.","went_well":[{"text":"Good thing","message_id":null}],"went_bad":[],"human_suggestions":[],"environment_suggestions":[],"default_context_suggestions":[]}`)
	t.Cleanup(mock.Close)
	f := setupGeneratorTestWithLLM(t, "openaivendor@test.com", "test-session-openai-vendor", analytics.LLMProviderOpenAI, mock)

	result := f.generateWithDefaults(t)
	f.requireSuccessfulGeneration(t, result)

	if result.Card.LLMProvider != analytics.LLMProviderOpenAI {
		t.Errorf("returned LLMProvider = %q, want openai", result.Card.LLMProvider)
	}
	card, err := f.store.GetSmartRecapCard(context.Background(), f.sessionID)
	if err != nil {
		t.Fatalf("GetSmartRecapCard failed: %v", err)
	}
	if card.LLMProvider != analytics.LLMProviderOpenAI {
		t.Errorf("saved LLMProvider = %q, want openai", card.LLMProvider)
	}
	if card.InputTokens != 150 || card.OutputTokens != 60 {
		t.Errorf("saved tokens = %d/%d, want 150/60 from OpenAI usage", card.InputTokens, card.OutputTokens)
	}
	if result.SuggestedTitle != "Test Session" {
		t.Errorf("SuggestedTitle = %q", result.SuggestedTitle)
	}
}

func TestSmartRecapGenerator_OpenAIIncompleteFailsWithoutCardOrQuota(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	mock := newMockOpenAIServer(t, "incomplete", `{"recap":"cut off`)
	t.Cleanup(mock.Close)
	f := setupGeneratorTestWithLLM(t, "openaiincomplete@test.com", "test-session-openai-incomplete", analytics.LLMProviderOpenAI, mock)

	result := f.generateWithDefaults(t)

	if result.Error == nil {
		t.Fatal("expected an error for an incomplete OpenAI response")
	}
	if result.Card != nil {
		t.Error("no card should be returned on failure")
	}
	count, err := recapquota.GetCount(context.Background(), f.conn, f.user.ID)
	if err != nil {
		t.Fatalf("GetCount failed: %v", err)
	}
	if count != 0 {
		t.Errorf("quota count = %d, want 0 (failed generations are free)", count)
	}
	acquired, err := f.store.AcquireSmartRecapLock(context.Background(), f.sessionID, 60)
	if err != nil {
		t.Fatalf("AcquireSmartRecapLock: %v", err)
	}
	if !acquired {
		t.Error("lock should have been cleared after the failed generation")
	}
}

func TestSmartRecapGenerator_ReturnsSuggestedTitle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	f := setupGeneratorTest(t, "title@test.com", "test-session-title")

	result := f.generateWithDefaults(t)

	if result.Error != nil {
		t.Fatalf("expected no error, got: %v", result.Error)
	}
	// The mock response includes "suggested_session_title": "Test Session"
	if result.SuggestedTitle != "Test Session" {
		t.Errorf("SuggestedTitle = %q, want %q", result.SuggestedTitle, "Test Session")
	}
}
