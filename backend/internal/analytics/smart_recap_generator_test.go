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

// setupGeneratorTest creates the shared test fixture: test environment, mock Anthropic
// server, user, session, analytics store, and generator. Caller provides email and
// externalID to keep tests independent.
func setupGeneratorTest(t *testing.T, email, externalID string) *generatorTestFixture {
	t.Helper()

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	mockServer := newMockAnthropicServer(t)
	t.Cleanup(mockServer.Close)

	user := testutil.CreateTestUser(t, env, email, "Test User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)
	conn := env.DB.Conn()
	store := analytics.NewStore(conn)

	generator := analytics.NewSmartRecapGenerator(store, conn, analytics.SmartRecapGeneratorConfig{
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
	return f.generator.Generate(context.Background(), input, 60)
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
