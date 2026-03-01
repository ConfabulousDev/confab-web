package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/anthropic"
	"github.com/ConfabulousDev/confab-web/internal/recapquota"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// =============================================================================
// Smart Recap Cache Behavior Tests
//
// These tests verify that the analytics endpoint returns cached smart recaps
// without checking staleness or triggering regeneration.
// =============================================================================

func TestSmartRecap_ReturnsCachedCardWithoutStalenessCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Set up smart recap config (with fake API key - we won't actually call the API)
	os.Setenv("SMART_RECAP_ENABLED", "true")
	os.Setenv("ANTHROPIC_API_KEY", "test-api-key-not-used")
	os.Setenv("SMART_RECAP_MODEL", "claude-haiku-4-5-20251101")
	os.Setenv("SMART_RECAP_QUOTA_LIMIT", "20")
	defer func() {
		os.Unsetenv("SMART_RECAP_ENABLED")
		os.Unsetenv("ANTHROPIC_API_KEY")
		os.Unsetenv("SMART_RECAP_MODEL")
		os.Unsetenv("SMART_RECAP_QUOTA_LIMIT")
	}()

	env := testutil.SetupTestEnvironment(t)

	t.Run("returns cached smart recap without regeneration even when outdated", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "stale@test.com", "Stale Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-stale")

		// Upload JSONL content (1 line to make the card outdated)
		jsonlContent := `{"type":"assistant","message":{"id":"msg_1","type":"message","model":"claude-sonnet-4","role":"assistant","content":[{"type":"text","text":"Hello!"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z","parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
`
		testutil.UploadTestChunk(t, env, user.ID, "test-session-stale", "transcript.jsonl", 1, 1, []byte(jsonlContent))
		testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 1)

		// Pre-populate an "outdated" smart recap card:
		// - UpToLine: 0 (different from current line count of 1)
		// This makes the card outdated (has new lines)
		store := analytics.NewStore(env.DB.Conn())
		staleCard := &analytics.SmartRecapCardRecord{
			SessionID:                 sessionID,
			Version:                   analytics.SmartRecapCardVersion,
			ComputedAt:                time.Now().UTC().Add(-30 * time.Minute),
			UpToLine:                  0, // Different from current line count of 1 (outdated)
			Recap:                     "This is an OUTDATED cached recap that should be returned as-is",
			WentWell:                  []analytics.AnnotatedItem{{Text: "Cached item 1"}},
			WentBad:                   []analytics.AnnotatedItem{{Text: "Cached bad item"}},
			HumanSuggestions:          []analytics.AnnotatedItem{},
			EnvironmentSuggestions:    []analytics.AnnotatedItem{},
			DefaultContextSuggestions: []analytics.AnnotatedItem{},
			ModelUsed:                 "claude-haiku-4-5-20251101",
			InputTokens:               500,
			OutputTokens:              100,
		}
		err := store.UpsertSmartRecapCard(context.Background(), staleCard)
		if err != nil {
			t.Fatalf("Failed to insert stale smart recap card: %v", err)
		}

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		// Make request to analytics endpoint
		resp, err := client.Get(fmt.Sprintf("/api/v1/sessions/%s/analytics", sessionID))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		// Parse response as raw map to check for is_stale field
		var rawResult map[string]interface{}
		testutil.ParseJSON(t, resp, &rawResult)

		// Get the cards map
		cards, ok := rawResult["cards"].(map[string]interface{})
		if !ok {
			t.Fatal("expected cards map in response")
		}

		// Get smart_recap card
		smartRecap, ok := cards["smart_recap"].(map[string]interface{})
		if !ok {
			t.Fatal("expected smart_recap card in response")
		}

		// Verify the OUTDATED cached content was returned (not regenerated)
		recap, _ := smartRecap["recap"].(string)
		if recap != staleCard.Recap {
			t.Errorf("expected outdated recap %q, got %q", staleCard.Recap, recap)
		}

		// Verify is_stale field is NOT present in the response
		if _, hasIsStale := smartRecap["is_stale"]; hasIsStale {
			t.Error("response should NOT contain is_stale field")
		}

		// Verify the card was NOT regenerated (check DB - UpToLine should still be 0)
		retrievedCard, err := store.GetSmartRecapCard(context.Background(), sessionID)
		if err != nil {
			t.Fatalf("Failed to get smart recap card: %v", err)
		}
		if retrievedCard.UpToLine != 0 {
			t.Errorf("card should NOT have been regenerated, UpToLine = %d, want 0", retrievedCard.UpToLine)
		}
		if retrievedCard.Recap != staleCard.Recap {
			t.Error("card content should NOT have changed")
		}
	})

	t.Run("returns cached smart recap for non-owner viewers without regeneration", func(t *testing.T) {
		env.CleanDB(t)

		owner := testutil.CreateTestUser(t, env, "owner@test.com", "Owner User")
		viewer := testutil.CreateTestUser(t, env, "viewer@test.com", "Viewer User")
		viewerToken := testutil.CreateTestWebSessionWithToken(t, env, viewer.ID)
		sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session-shared")

		// Create a public share so viewer can access
		testutil.CreateTestShare(t, env, sessionID, true, nil, nil)

		// Upload JSONL content
		jsonlContent := `{"type":"assistant","message":{"id":"msg_1","type":"message","model":"claude-sonnet-4","role":"assistant","content":[{"type":"text","text":"Hello!"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z","parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
`
		testutil.UploadTestChunk(t, env, owner.ID, "test-session-shared", "transcript.jsonl", 1, 1, []byte(jsonlContent))
		testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 1)

		// Pre-populate smart recap card (stale)
		store := analytics.NewStore(env.DB.Conn())
		cachedCard := &analytics.SmartRecapCardRecord{
			SessionID:                 sessionID,
			Version:                   analytics.SmartRecapCardVersion,
			ComputedAt:                time.Now().UTC().Add(-30 * time.Minute),
			UpToLine:                  0,
			Recap:                     "Cached recap for shared session",
			WentWell:                  []analytics.AnnotatedItem{},
			WentBad:                   []analytics.AnnotatedItem{},
			HumanSuggestions:          []analytics.AnnotatedItem{},
			EnvironmentSuggestions:    []analytics.AnnotatedItem{},
			DefaultContextSuggestions: []analytics.AnnotatedItem{},
			ModelUsed:                 "claude-haiku-4-5-20251101",
			InputTokens:               500,
			OutputTokens:              100,
		}
		err := store.UpsertSmartRecapCard(context.Background(), cachedCard)
		if err != nil {
			t.Fatalf("Failed to insert smart recap card: %v", err)
		}

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(viewerToken)

		resp, err := client.Get(fmt.Sprintf("/api/v1/sessions/%s/analytics", sessionID))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var rawResult map[string]interface{}
		testutil.ParseJSON(t, resp, &rawResult)

		cards := rawResult["cards"].(map[string]interface{})
		smartRecap := cards["smart_recap"].(map[string]interface{})

		// Verify cached content returned
		if smartRecap["recap"] != cachedCard.Recap {
			t.Error("expected cached recap to be returned for viewer")
		}

		// Verify no is_stale field
		if _, hasIsStale := smartRecap["is_stale"]; hasIsStale {
			t.Error("response should NOT contain is_stale field")
		}

		// Verify quota is NOT included for non-owner
		if rawResult["smart_recap_quota"] != nil {
			t.Error("quota should NOT be included for non-owner viewer")
		}
	})

	t.Run("includes quota info only for session owner", func(t *testing.T) {
		env.CleanDB(t)

		owner := testutil.CreateTestUser(t, env, "quotaowner@test.com", "Quota Owner")
		ownerToken := testutil.CreateTestWebSessionWithToken(t, env, owner.ID)
		sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session-quota")

		// Upload JSONL content
		jsonlContent := `{"type":"assistant","message":{"id":"msg_1","type":"message","model":"claude-sonnet-4","role":"assistant","content":[{"type":"text","text":"Hello!"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z","parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
`
		testutil.UploadTestChunk(t, env, owner.ID, "test-session-quota", "transcript.jsonl", 1, 1, []byte(jsonlContent))
		testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 1)

		// Pre-populate smart recap card
		store := analytics.NewStore(env.DB.Conn())
		cachedCard := &analytics.SmartRecapCardRecord{
			SessionID:                 sessionID,
			Version:                   analytics.SmartRecapCardVersion,
			ComputedAt:                time.Now().UTC(),
			UpToLine:                  1,
			Recap:                     "Cached recap",
			WentWell:                  []analytics.AnnotatedItem{},
			WentBad:                   []analytics.AnnotatedItem{},
			HumanSuggestions:          []analytics.AnnotatedItem{},
			EnvironmentSuggestions:    []analytics.AnnotatedItem{},
			DefaultContextSuggestions: []analytics.AnnotatedItem{},
			ModelUsed:                 "claude-haiku-4-5-20251101",
			InputTokens:               500,
			OutputTokens:              100,
		}
		err := store.UpsertSmartRecapCard(context.Background(), cachedCard)
		if err != nil {
			t.Fatalf("Failed to insert smart recap card: %v", err)
		}

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(ownerToken)

		resp, err := client.Get(fmt.Sprintf("/api/v1/sessions/%s/analytics", sessionID))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var rawResult map[string]interface{}
		testutil.ParseJSON(t, resp, &rawResult)

		// Verify quota IS included for owner
		quota, hasQuota := rawResult["smart_recap_quota"].(map[string]interface{})
		if !hasQuota {
			t.Fatal("expected smart_recap_quota for owner")
		}

		// Verify quota structure
		if _, hasUsed := quota["used"]; !hasUsed {
			t.Error("quota should have 'used' field")
		}
		if _, hasLimit := quota["limit"]; !hasLimit {
			t.Error("quota should have 'limit' field")
		}
		if _, hasExceeded := quota["exceeded"]; !hasExceeded {
			t.Error("quota should have 'exceeded' field")
		}
	})

	t.Run("smart recap response structure has correct fields", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "structure@test.com", "Structure Test")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-structure")

		// Upload JSONL content
		jsonlContent := `{"type":"assistant","message":{"id":"msg_1","type":"message","model":"claude-sonnet-4","role":"assistant","content":[{"type":"text","text":"Hello!"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z","parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
`
		testutil.UploadTestChunk(t, env, user.ID, "test-session-structure", "transcript.jsonl", 1, 1, []byte(jsonlContent))
		testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 1)

		// Pre-populate smart recap card with all fields
		store := analytics.NewStore(env.DB.Conn())
		cachedCard := &analytics.SmartRecapCardRecord{
			SessionID:                 sessionID,
			Version:                   analytics.SmartRecapCardVersion,
			ComputedAt:                time.Now().UTC(),
			UpToLine:                  1,
			Recap:                     "Test recap content",
			WentWell:                  []analytics.AnnotatedItem{{Text: "Good 1"}, {Text: "Good 2"}},
			WentBad:                   []analytics.AnnotatedItem{{Text: "Bad 1"}},
			HumanSuggestions:          []analytics.AnnotatedItem{{Text: "Human suggestion"}},
			EnvironmentSuggestions:    []analytics.AnnotatedItem{{Text: "Env suggestion"}},
			DefaultContextSuggestions: []analytics.AnnotatedItem{{Text: "Context suggestion"}},
			ModelUsed:                 "claude-haiku-4-5-20251101",
			InputTokens:               500,
			OutputTokens:              100,
		}
		err := store.UpsertSmartRecapCard(context.Background(), cachedCard)
		if err != nil {
			t.Fatalf("Failed to insert smart recap card: %v", err)
		}

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get(fmt.Sprintf("/api/v1/sessions/%s/analytics", sessionID))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var rawResult map[string]interface{}
		testutil.ParseJSON(t, resp, &rawResult)

		cards := rawResult["cards"].(map[string]interface{})
		smartRecap := cards["smart_recap"].(map[string]interface{})

		// Verify expected fields are present
		expectedFields := []string{
			"recap",
			"went_well",
			"went_bad",
			"human_suggestions",
			"environment_suggestions",
			"default_context_suggestions",
			"computed_at",
			"model_used",
		}
		for _, field := range expectedFields {
			if _, ok := smartRecap[field]; !ok {
				t.Errorf("expected field %q in smart_recap response", field)
			}
		}

		// Verify is_stale is NOT present
		if _, hasIsStale := smartRecap["is_stale"]; hasIsStale {
			t.Error("is_stale field should NOT be present in response")
		}

		// Verify content matches
		if smartRecap["recap"] != cachedCard.Recap {
			t.Errorf("recap = %q, want %q", smartRecap["recap"], cachedCard.Recap)
		}
		if smartRecap["model_used"] != cachedCard.ModelUsed {
			t.Errorf("model_used = %q, want %q", smartRecap["model_used"], cachedCard.ModelUsed)
		}
	})
}

// TestSmartRecap_ErrorPropagation verifies that smart recap errors are propagated
// to the card_errors field for graceful frontend degradation.
func TestSmartRecap_ErrorPropagation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Set up smart recap config (with fake API key - generation will fail)
	os.Setenv("SMART_RECAP_ENABLED", "true")
	os.Setenv("ANTHROPIC_API_KEY", "invalid-api-key-that-will-fail")
	os.Setenv("SMART_RECAP_MODEL", "claude-haiku-4-5-20251101")
	os.Setenv("SMART_RECAP_QUOTA_LIMIT", "20")
	defer func() {
		os.Unsetenv("SMART_RECAP_ENABLED")
		os.Unsetenv("ANTHROPIC_API_KEY")
		os.Unsetenv("SMART_RECAP_MODEL")
		os.Unsetenv("SMART_RECAP_QUOTA_LIMIT")
	}()

	env := testutil.SetupTestEnvironment(t)

	t.Run("returns card_errors when smart recap generation fails", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "errtest@test.com", "Error Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-error")

		// Upload JSONL content (required to trigger smart recap generation attempt)
		jsonlContent := `{"type":"assistant","message":{"id":"msg_1","type":"message","model":"claude-sonnet-4","role":"assistant","content":[{"type":"text","text":"Hello!"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z","parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
`
		testutil.UploadTestChunk(t, env, user.ID, "test-session-error", "transcript.jsonl", 1, 1, []byte(jsonlContent))
		testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 1)

		// Note: We do NOT insert a cached smart recap card, so generation will be attempted
		// The generation will fail because the API key is invalid

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		// Make request to analytics endpoint
		resp, err := client.Get(fmt.Sprintf("/api/v1/sessions/%s/analytics", sessionID))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		// Request should succeed (analytics endpoint returns partial results)
		testutil.RequireStatus(t, resp, http.StatusOK)

		var rawResult map[string]interface{}
		testutil.ParseJSON(t, resp, &rawResult)

		// Other cards should still be present
		cards, ok := rawResult["cards"].(map[string]interface{})
		if !ok {
			t.Fatal("expected cards map in response")
		}
		if _, hasTokens := cards["tokens"]; !hasTokens {
			t.Error("expected tokens card to still be present")
		}

		// smart_recap should NOT be in cards (generation failed)
		if _, hasSmartRecap := cards["smart_recap"]; hasSmartRecap {
			t.Error("smart_recap should NOT be in cards when generation failed")
		}

		// card_errors should contain smart_recap error
		cardErrors, hasErrors := rawResult["card_errors"].(map[string]interface{})
		if !hasErrors {
			t.Fatal("expected card_errors in response when smart recap fails")
		}
		smartRecapError, hasSmartRecapError := cardErrors["smart_recap"].(string)
		if !hasSmartRecapError {
			t.Fatal("expected smart_recap key in card_errors")
		}
		if smartRecapError == "" {
			t.Error("smart_recap error message should not be empty")
		}
		t.Logf("Smart recap error message: %s", smartRecapError)
	})
}

func TestSmartRecap_CacheHitVsMissPath(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Set up smart recap config
	os.Setenv("SMART_RECAP_ENABLED", "true")
	os.Setenv("ANTHROPIC_API_KEY", "test-api-key-not-used")
	os.Setenv("SMART_RECAP_MODEL", "claude-haiku-4-5-20251101")
	os.Setenv("SMART_RECAP_QUOTA_LIMIT", "20")
	defer func() {
		os.Unsetenv("SMART_RECAP_ENABLED")
		os.Unsetenv("ANTHROPIC_API_KEY")
		os.Unsetenv("SMART_RECAP_MODEL")
		os.Unsetenv("SMART_RECAP_QUOTA_LIMIT")
	}()

	env := testutil.SetupTestEnvironment(t)

	t.Run("cache hit path returns cached card immediately", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "cachehit@test.com", "Cache Hit User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-cachehit")

		// Upload JSONL content
		jsonlContent := `{"type":"assistant","message":{"id":"msg_1","type":"message","model":"claude-sonnet-4","role":"assistant","content":[{"type":"text","text":"Hello!"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z","parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
`
		testutil.UploadTestChunk(t, env, user.ID, "test-session-cachehit", "transcript.jsonl", 1, 1, []byte(jsonlContent))
		testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 1)

		// First, trigger card computation by making a request (this caches the regular analytics cards)
		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp1, err := client.Get(fmt.Sprintf("/api/v1/sessions/%s/analytics", sessionID))
		if err != nil {
			t.Fatalf("request 1 failed: %v", err)
		}
		resp1.Body.Close()

		// Now insert a smart recap card (simulating background worker having generated it)
		store := analytics.NewStore(env.DB.Conn())
		cachedCard := &analytics.SmartRecapCardRecord{
			SessionID:                 sessionID,
			Version:                   analytics.SmartRecapCardVersion,
			ComputedAt:                time.Now().UTC(),
			UpToLine:                  1,
			Recap:                     "Pre-cached recap from background worker",
			WentWell:                  []analytics.AnnotatedItem{},
			WentBad:                   []analytics.AnnotatedItem{},
			HumanSuggestions:          []analytics.AnnotatedItem{},
			EnvironmentSuggestions:    []analytics.AnnotatedItem{},
			DefaultContextSuggestions: []analytics.AnnotatedItem{},
			ModelUsed:                 "claude-haiku-4-5-20251101",
			InputTokens:               500,
			OutputTokens:              100,
		}
		err = store.UpsertSmartRecapCard(context.Background(), cachedCard)
		if err != nil {
			t.Fatalf("Failed to insert smart recap card: %v", err)
		}

		// Second request should hit cache and return the pre-cached card
		resp2, err := client.Get(fmt.Sprintf("/api/v1/sessions/%s/analytics", sessionID))
		if err != nil {
			t.Fatalf("request 2 failed: %v", err)
		}
		defer resp2.Body.Close()

		testutil.RequireStatus(t, resp2, http.StatusOK)

		var rawResult map[string]interface{}
		testutil.ParseJSON(t, resp2, &rawResult)

		cards := rawResult["cards"].(map[string]interface{})
		smartRecap, ok := cards["smart_recap"].(map[string]interface{})
		if !ok {
			t.Fatal("expected smart_recap card in response")
		}

		// Verify the cached content was returned
		if smartRecap["recap"] != cachedCard.Recap {
			t.Errorf("expected cached recap, got %q", smartRecap["recap"])
		}
	})
}

// TestSuggestedTitle_IncludedInAnalyticsResponse verifies that suggested_session_title
// from the database is included in the analytics response (the attachSuggestedTitle fallback path).
func TestSuggestedTitle_IncludedInAnalyticsResponse(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)

	t.Run("includes suggested_session_title when present in DB", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "title@test.com", "Title Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-title")

		// Upload JSONL content
		jsonlContent := `{"type":"assistant","message":{"id":"msg_1","type":"message","model":"claude-sonnet-4","role":"assistant","content":[{"type":"text","text":"Hello!"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z","parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
`
		testutil.UploadTestChunk(t, env, user.ID, "test-session-title", "transcript.jsonl", 1, 1, []byte(jsonlContent))
		testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 1)

		// Set suggested title in DB (simulates prior smart recap generation)
		err := env.DB.UpdateSessionSuggestedTitle(context.Background(), sessionID, "AI-generated session title")
		if err != nil {
			t.Fatalf("Failed to set suggested title: %v", err)
		}

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get(fmt.Sprintf("/api/v1/sessions/%s/analytics", sessionID))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var rawResult map[string]interface{}
		testutil.ParseJSON(t, resp, &rawResult)

		// Verify suggested_session_title is present in response
		title, hasTitle := rawResult["suggested_session_title"].(string)
		if !hasTitle {
			t.Fatal("expected suggested_session_title in analytics response")
		}
		if title != "AI-generated session title" {
			t.Errorf("suggested_session_title = %q, want %q", title, "AI-generated session title")
		}
	})

	t.Run("omits suggested_session_title when not set in DB", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "notitle@test.com", "No Title User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-notitle")

		// Upload JSONL content
		jsonlContent := `{"type":"assistant","message":{"id":"msg_1","type":"message","model":"claude-sonnet-4","role":"assistant","content":[{"type":"text","text":"Hello!"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z","parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
`
		testutil.UploadTestChunk(t, env, user.ID, "test-session-notitle", "transcript.jsonl", 1, 1, []byte(jsonlContent))
		testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 1)

		// Do NOT set suggested title

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get(fmt.Sprintf("/api/v1/sessions/%s/analytics", sessionID))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var rawResult map[string]interface{}
		testutil.ParseJSON(t, resp, &rawResult)

		// Verify suggested_session_title is NOT present (omitempty)
		if _, hasTitle := rawResult["suggested_session_title"]; hasTitle {
			t.Error("suggested_session_title should be omitted when not set in DB")
		}
	})

	t.Run("includes suggested_session_title on cache-hit path", func(t *testing.T) {
		env.CleanDB(t)

		// Enable smart recap so the cache-hit path runs through attachOrGenerateSmartRecap
		os.Setenv("SMART_RECAP_ENABLED", "true")
		os.Setenv("ANTHROPIC_API_KEY", "test-api-key-not-used")
		os.Setenv("SMART_RECAP_MODEL", "claude-haiku-4-5-20251101")
		os.Setenv("SMART_RECAP_QUOTA_LIMIT", "20")
		defer func() {
			os.Unsetenv("SMART_RECAP_ENABLED")
			os.Unsetenv("ANTHROPIC_API_KEY")
			os.Unsetenv("SMART_RECAP_MODEL")
			os.Unsetenv("SMART_RECAP_QUOTA_LIMIT")
		}()

		user := testutil.CreateTestUser(t, env, "cachetitle@test.com", "Cache Title User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-cachetitle")

		// Upload JSONL content
		jsonlContent := `{"type":"assistant","message":{"id":"msg_1","type":"message","model":"claude-sonnet-4","role":"assistant","content":[{"type":"text","text":"Hello!"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z","parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
`
		testutil.UploadTestChunk(t, env, user.ID, "test-session-cachetitle", "transcript.jsonl", 1, 1, []byte(jsonlContent))
		testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 1)

		// Pre-populate smart recap card (cache hit path - card is valid/current)
		store := analytics.NewStore(env.DB.Conn())
		cachedCard := &analytics.SmartRecapCardRecord{
			SessionID:                 sessionID,
			Version:                   analytics.SmartRecapCardVersion,
			ComputedAt:                time.Now().UTC(),
			UpToLine:                  1,
			Recap:                     "Cached recap",
			WentWell:                  []analytics.AnnotatedItem{},
			WentBad:                   []analytics.AnnotatedItem{},
			HumanSuggestions:          []analytics.AnnotatedItem{},
			EnvironmentSuggestions:    []analytics.AnnotatedItem{},
			DefaultContextSuggestions: []analytics.AnnotatedItem{},
			ModelUsed:                 "claude-haiku-4-5-20251101",
			InputTokens:               500,
			OutputTokens:              100,
		}
		err := store.UpsertSmartRecapCard(context.Background(), cachedCard)
		if err != nil {
			t.Fatalf("Failed to insert smart recap card: %v", err)
		}

		// Set suggested title in DB (from prior generation)
		err = env.DB.UpdateSessionSuggestedTitle(context.Background(), sessionID, "Title from prior generation")
		if err != nil {
			t.Fatalf("Failed to set suggested title: %v", err)
		}

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get(fmt.Sprintf("/api/v1/sessions/%s/analytics", sessionID))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var rawResult map[string]interface{}
		testutil.ParseJSON(t, resp, &rawResult)

		// Verify smart recap card is present
		cards := rawResult["cards"].(map[string]interface{})
		if _, hasSmartRecap := cards["smart_recap"]; !hasSmartRecap {
			t.Fatal("expected smart_recap card")
		}

		// Verify suggested_session_title is present alongside the smart recap
		title, hasTitle := rawResult["suggested_session_title"].(string)
		if !hasTitle {
			t.Fatal("expected suggested_session_title in analytics response on cache-hit path")
		}
		if title != "Title from prior generation" {
			t.Errorf("suggested_session_title = %q, want %q", title, "Title from prior generation")
		}
	})
}

// TestSmartRecap_CacheMissTriggersGeneration is an E2E test that verifies the full
// generation path through the API: cache miss → download transcript → generate via
// SmartRecapGenerator → return card + suggested title + increment quota.
func TestSmartRecap_CacheMissTriggersGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Set up a mock Anthropic server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := anthropic.MessagesResponse{
			ID:         "msg_test",
			Type:       "message",
			Role:       "assistant",
			StopReason: "end_turn",
			Content: []anthropic.ContentBlock{
				{
					Type: "text",
					Text: `"suggested_session_title": "Generated Title", "recap": "Generated recap.", "went_well": ["Good"], "went_bad": [], "human_suggestions": [], "environment_suggestions": [], "default_context_suggestions": []}`,
				},
			},
			Usage: anthropic.Usage{
				InputTokens:  200,
				OutputTokens: 80,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	// Configure smart recap with the mock server
	os.Setenv("SMART_RECAP_ENABLED", "true")
	os.Setenv("ANTHROPIC_API_KEY", "test-key")
	os.Setenv("SMART_RECAP_MODEL", "test-model")
	os.Setenv("SMART_RECAP_QUOTA_LIMIT", "20")
	os.Setenv("TEST_SMART_RECAP_BASE_URL", mockServer.URL)
	defer func() {
		os.Unsetenv("SMART_RECAP_ENABLED")
		os.Unsetenv("ANTHROPIC_API_KEY")
		os.Unsetenv("SMART_RECAP_MODEL")
		os.Unsetenv("SMART_RECAP_QUOTA_LIMIT")
		os.Unsetenv("TEST_SMART_RECAP_BASE_URL")
	}()

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "gen@test.com", "Gen User")
	sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
	sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-gen")

	// Upload JSONL content (required for analytics computation and smart recap)
	jsonlContent := `{"type":"assistant","message":{"id":"msg_1","type":"message","model":"claude-sonnet-4","role":"assistant","content":[{"type":"text","text":"Hello!"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z","parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
`
	testutil.UploadTestChunk(t, env, user.ID, "test-session-gen", "transcript.jsonl", 1, 1, []byte(jsonlContent))
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 1)

	// No pre-populated smart recap card → triggers generation

	ts := setupTestServerWithEnv(t, env)
	client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

	resp, err := client.Get(fmt.Sprintf("/api/v1/sessions/%s/analytics", sessionID))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	testutil.RequireStatus(t, resp, http.StatusOK)

	var rawResult map[string]interface{}
	testutil.ParseJSON(t, resp, &rawResult)

	// Verify smart_recap card is present
	cards, ok := rawResult["cards"].(map[string]interface{})
	if !ok {
		t.Fatal("expected cards map in response")
	}
	smartRecap, ok := cards["smart_recap"].(map[string]interface{})
	if !ok {
		t.Fatal("expected smart_recap card in response (generation should have succeeded)")
	}

	// Verify generated content
	if smartRecap["recap"] != "Generated recap." {
		t.Errorf("recap = %q, want %q", smartRecap["recap"], "Generated recap.")
	}

	// Verify suggested_session_title from generation
	title, hasTitle := rawResult["suggested_session_title"].(string)
	if !hasTitle {
		t.Fatal("expected suggested_session_title in response")
	}
	if title != "Generated Title" {
		t.Errorf("suggested_session_title = %q, want %q", title, "Generated Title")
	}

	// Verify quota was incremented
	count, err := recapquota.GetCount(context.Background(), env.DB.Conn(), user.ID)
	if err != nil {
		t.Fatalf("GetCount failed: %v", err)
	}
	if count != 1 {
		t.Errorf("quota count = %d, want 1", count)
	}

	// Verify card was persisted in DB
	store := analytics.NewStore(env.DB.Conn())
	card, err := store.GetSmartRecapCard(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("GetSmartRecapCard failed: %v", err)
	}
	if card == nil {
		t.Fatal("expected smart recap card to be saved in DB")
	}
	if card.Recap != "Generated recap." {
		t.Errorf("saved recap = %q, want %q", card.Recap, "Generated recap.")
	}
}
