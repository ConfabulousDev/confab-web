package analytics_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/anthropic"
	"github.com/ConfabulousDev/confab-web/internal/recapquota"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// mockAnthropicResponse returns a valid Anthropic API response with a smart recap JSON.
// The text content omits the leading "{" because the analyzer prefills it.
func mockAnthropicResponse() anthropic.MessagesResponse {
	return anthropic.MessagesResponse{
		ID:         "msg_test",
		Type:       "message",
		Role:       "assistant",
		StopReason: "end_turn",
		Content: []anthropic.ContentBlock{
			{
				Type: "text",
				Text: `"suggested_session_title": "Test Session", "recap": "Test recap content.", "went_well": ["Good thing"], "went_bad": [], "human_suggestions": [], "environment_suggestions": [], "default_context_suggestions": []}`,
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

func TestSmartRecapGenerator_QuotaIncrementFailurePreventsGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	mockServer := newMockAnthropicServer(t)
	defer mockServer.Close()

	user := testutil.CreateTestUser(t, env, "quotafail@test.com", "QuotaFail User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-quotafail")
	ctx := context.Background()
	conn := env.DB.Conn()
	store := analytics.NewStore(conn)

	generator := analytics.NewSmartRecapGenerator(store, conn, analytics.SmartRecapGeneratorConfig{
		APIKey:            "test-key",
		Model:             "test-model",
		GenerationTimeout: 10 * time.Second,
		BaseURL:           mockServer.URL,
	})

	fc := makeTestFileCollection(t)
	input := analytics.GenerateInput{
		SessionID:      sessionID,
		UserID:         user.ID,
		LineCount:      1,
		FileCollection: fc,
		CardStats:      nil,
	}

	// Do NOT call recapquota.GetOrCreate — no quota row exists.
	// This simulates a quota system failure where the row is missing.
	result := generator.Generate(ctx, input, 60)

	// Generation should fail because quota increment failed
	if result.Error == nil {
		t.Fatal("expected error when quota increment fails, got nil")
	}
	if result.Card != nil {
		t.Error("expected no card when quota increment fails")
	}

	// Verify no real card was saved to the database.
	// Note: AcquireSmartRecapLock creates a stub row (version=0, empty recap),
	// so we check that no real card (version > 0) was written.
	card, err := store.GetSmartRecapCard(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSmartRecapCard failed: %v", err)
	}
	if card != nil && card.Version != 0 {
		t.Errorf("real card should NOT be saved when quota increment fails, got version=%d", card.Version)
	}
	if card != nil && card.Recap != "" {
		t.Errorf("card recap should be empty (lock stub), got %q", card.Recap)
	}

	// Verify lock was cleared (another request can acquire it)
	acquired, err := store.AcquireSmartRecapLock(ctx, sessionID, 60)
	if err != nil {
		t.Fatalf("AcquireSmartRecapLock failed: %v", err)
	}
	if !acquired {
		t.Error("lock should be cleared after quota increment failure")
	}
}

func TestSmartRecapGenerator_QuotaIncrementSuccessAllowsGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	mockServer := newMockAnthropicServer(t)
	defer mockServer.Close()

	user := testutil.CreateTestUser(t, env, "quotaok@test.com", "QuotaOK User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-quotaok")
	ctx := context.Background()
	conn := env.DB.Conn()
	store := analytics.NewStore(conn)

	generator := analytics.NewSmartRecapGenerator(store, conn, analytics.SmartRecapGeneratorConfig{
		APIKey:            "test-key",
		Model:             "test-model",
		GenerationTimeout: 10 * time.Second,
		BaseURL:           mockServer.URL,
	})

	// Create quota row first (normal flow)
	_, err := recapquota.GetOrCreate(ctx, conn, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreate failed: %v", err)
	}

	fc := makeTestFileCollection(t)
	input := analytics.GenerateInput{
		SessionID:      sessionID,
		UserID:         user.ID,
		LineCount:      1,
		FileCollection: fc,
		CardStats:      nil,
	}

	result := generator.Generate(ctx, input, 60)

	// Generation should succeed
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
	card, err := store.GetSmartRecapCard(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSmartRecapCard failed: %v", err)
	}
	if card == nil {
		t.Fatal("card should be saved after successful generation")
	}
	if card.Recap != "Test recap content." {
		t.Errorf("saved recap = %q, want %q", card.Recap, "Test recap content.")
	}

	// Verify quota was incremented
	count, err := recapquota.GetCount(ctx, conn, user.ID)
	if err != nil {
		t.Fatalf("GetCount failed: %v", err)
	}
	if count != 1 {
		t.Errorf("quota count = %d, want 1", count)
	}
}
