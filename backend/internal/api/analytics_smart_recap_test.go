package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
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
	os.Setenv("SMART_RECAP_STALENESS_MINUTES", "10")
	defer func() {
		os.Unsetenv("SMART_RECAP_ENABLED")
		os.Unsetenv("ANTHROPIC_API_KEY")
		os.Unsetenv("SMART_RECAP_MODEL")
		os.Unsetenv("SMART_RECAP_QUOTA_LIMIT")
		os.Unsetenv("SMART_RECAP_STALENESS_MINUTES")
	}()

	env := testutil.SetupTestEnvironment(t)

	t.Run("returns cached smart recap without regeneration even when stale", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "stale@test.com", "Stale Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-stale")

		// Upload JSONL content (100 lines to make the card "stale")
		jsonlContent := `{"type":"assistant","message":{"id":"msg_1","type":"message","model":"claude-sonnet-4","role":"assistant","content":[{"type":"text","text":"Hello!"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z","parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
`
		testutil.UploadTestChunk(t, env, user.ID, "test-session-stale", "transcript.jsonl", 1, 1, []byte(jsonlContent))
		testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 1)

		// Pre-populate a "stale" smart recap card:
		// - ComputedAt: 30 minutes ago (older than staleness threshold of 10 minutes)
		// - UpToLine: 0 (different from current line count of 1)
		// This combination makes the card "stale" by the old logic
		store := analytics.NewStore(env.DB.Conn())
		staleCard := &analytics.SmartRecapCardRecord{
			SessionID:                 sessionID,
			Version:                   analytics.SmartRecapCardVersion,
			ComputedAt:                time.Now().UTC().Add(-30 * time.Minute), // 30 min ago (stale)
			UpToLine:                  0,                                       // Different from current (stale)
			Recap:                     "This is a STALE cached recap that should be returned as-is",
			WentWell:                  []string{"Cached item 1"},
			WentBad:                   []string{"Cached bad item"},
			HumanSuggestions:          []string{},
			EnvironmentSuggestions:    []string{},
			DefaultContextSuggestions: []string{},
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

		// Verify the STALE cached content was returned (not regenerated)
		recap, _ := smartRecap["recap"].(string)
		if recap != staleCard.Recap {
			t.Errorf("expected stale recap %q, got %q", staleCard.Recap, recap)
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
			WentWell:                  []string{},
			WentBad:                   []string{},
			HumanSuggestions:          []string{},
			EnvironmentSuggestions:    []string{},
			DefaultContextSuggestions: []string{},
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
			WentWell:                  []string{},
			WentBad:                   []string{},
			HumanSuggestions:          []string{},
			EnvironmentSuggestions:    []string{},
			DefaultContextSuggestions: []string{},
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
			WentWell:                  []string{"Good 1", "Good 2"},
			WentBad:                   []string{"Bad 1"},
			HumanSuggestions:          []string{"Human suggestion"},
			EnvironmentSuggestions:    []string{"Env suggestion"},
			DefaultContextSuggestions: []string{"Context suggestion"},
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

func TestSmartRecap_CacheHitVsMissPath(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Set up smart recap config
	os.Setenv("SMART_RECAP_ENABLED", "true")
	os.Setenv("ANTHROPIC_API_KEY", "test-api-key-not-used")
	os.Setenv("SMART_RECAP_MODEL", "claude-haiku-4-5-20251101")
	os.Setenv("SMART_RECAP_QUOTA_LIMIT", "20")
	os.Setenv("SMART_RECAP_STALENESS_MINUTES", "10")
	defer func() {
		os.Unsetenv("SMART_RECAP_ENABLED")
		os.Unsetenv("ANTHROPIC_API_KEY")
		os.Unsetenv("SMART_RECAP_MODEL")
		os.Unsetenv("SMART_RECAP_QUOTA_LIMIT")
		os.Unsetenv("SMART_RECAP_STALENESS_MINUTES")
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
			WentWell:                  []string{},
			WentBad:                   []string{},
			HumanSuggestions:          []string{},
			EnvironmentSuggestions:    []string{},
			DefaultContextSuggestions: []string{},
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
