package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/logger"
)

// mockService is a mock implementation for testing
type mockService struct {
	SentEmails []ShareInvitationParams
	ShouldFail bool
	FailError  error
}

func newMockService() *mockService {
	return &mockService{
		SentEmails: []ShareInvitationParams{},
	}
}

func (m *mockService) SendShareInvitation(ctx context.Context, params ShareInvitationParams) error {
	if m.ShouldFail {
		if m.FailError != nil {
			return m.FailError
		}
		return fmt.Errorf("mock email service failure")
	}
	m.SentEmails = append(m.SentEmails, params)
	return nil
}

func (m *mockService) reset() {
	m.SentEmails = []ShareInvitationParams{}
	m.ShouldFail = false
	m.FailError = nil
}

func TestRateLimiter(t *testing.T) {
	t.Run("allows requests under limit", func(t *testing.T) {
		limiter := NewEmailRateLimiter()
		userID := int64(1)

		// Should allow up to the limit
		for i := 0; i < 5; i++ {
			// First check if allowed
			if !limiter.Allow(userID, 5) {
				t.Errorf("expected request %d to be allowed", i+1)
			}
			// Then record (simulating the RateLimitedService behavior)
			limiter.Record(userID)
		}
	})

	t.Run("denies requests over limit", func(t *testing.T) {
		limiter := NewEmailRateLimiter()
		userID := int64(1)

		// Fill up the limit
		for i := 0; i < 5; i++ {
			limiter.Record(userID)
		}

		// Next request should be denied
		if limiter.Allow(userID, 5) {
			t.Error("expected request to be denied after reaching limit")
		}
	})

	t.Run("AllowN checks capacity without recording", func(t *testing.T) {
		limiter := NewEmailRateLimiter()
		userID := int64(1)

		// Record 3 emails
		for i := 0; i < 3; i++ {
			limiter.Record(userID)
		}

		// Should allow 2 more (limit is 5)
		if !limiter.AllowN(userID, 5, 2) {
			t.Error("expected AllowN(2) to succeed with 2 capacity remaining")
		}

		// Should not allow 3 more
		if limiter.AllowN(userID, 5, 3) {
			t.Error("expected AllowN(3) to fail with only 2 capacity remaining")
		}

		// The records should not have changed (AllowN doesn't record)
		if !limiter.AllowN(userID, 5, 2) {
			t.Error("AllowN should not have modified the record count")
		}
	})

	t.Run("different users have separate limits", func(t *testing.T) {
		limiter := NewEmailRateLimiter()
		user1 := int64(1)
		user2 := int64(2)

		// Fill up user1's limit
		for i := 0; i < 5; i++ {
			limiter.Record(user1)
		}

		// User1 should be denied
		if limiter.Allow(user1, 5) {
			t.Error("expected user1 to be denied")
		}

		// User2 should still be allowed
		if !limiter.Allow(user2, 5) {
			t.Error("expected user2 to be allowed")
		}
	})
}

func TestRateLimitedService(t *testing.T) {
	t.Run("sends email when under rate limit", func(t *testing.T) {
		mock := newMockService()
		service := NewRateLimitedService(mock, 10)

		params := ShareInvitationParams{
			ToEmail:      "test@example.com",
			SharerName:   "Alice",
			SharerEmail:  "alice@example.com",
			SessionTitle: "Test Session",
			ShareURL:     "https://example.com/share/abc123",
		}

		err := service.SendShareInvitation(context.Background(), 1, params)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(mock.SentEmails) != 1 {
			t.Errorf("expected 1 email sent, got %d", len(mock.SentEmails))
		}
	})

	t.Run("returns error when rate limited", func(t *testing.T) {
		mock := newMockService()
		service := NewRateLimitedService(mock, 2)

		params := ShareInvitationParams{
			ToEmail:      "test@example.com",
			SharerName:   "Alice",
			SharerEmail:  "alice@example.com",
			SessionTitle: "Test Session",
			ShareURL:     "https://example.com/share/abc123",
		}

		// Send 2 emails (at the limit)
		for i := 0; i < 2; i++ {
			err := service.SendShareInvitation(context.Background(), 1, params)
			if err != nil {
				t.Errorf("unexpected error on email %d: %v", i+1, err)
			}
		}

		// Third email should fail
		err := service.SendShareInvitation(context.Background(), 1, params)
		if err != ErrRateLimitExceeded {
			t.Errorf("expected ErrRateLimitExceeded, got %v", err)
		}

		// Only 2 emails should have been sent
		if len(mock.SentEmails) != 2 {
			t.Errorf("expected 2 emails sent, got %d", len(mock.SentEmails))
		}
	})

	t.Run("CheckRateLimit returns error when limit exceeded", func(t *testing.T) {
		mock := newMockService()
		service := NewRateLimitedService(mock, 5)

		// Check if we can send 3 emails (should succeed)
		err := service.checkRateLimit(1, 3)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Check if we can send 6 emails (should fail)
		err = service.checkRateLimit(1, 6)
		if err != ErrRateLimitExceeded {
			t.Errorf("expected ErrRateLimitExceeded, got %v", err)
		}
	})
}

func TestMockService(t *testing.T) {
	t.Run("records sent emails", func(t *testing.T) {
		mock := newMockService()

		params1 := ShareInvitationParams{
			ToEmail:     "user1@example.com",
			SharerName:  "Alice",
			SharerEmail: "alice@example.com",
		}
		params2 := ShareInvitationParams{
			ToEmail:     "user2@example.com",
			SharerName:  "Bob",
			SharerEmail: "bob@example.com",
		}

		mock.SendShareInvitation(context.Background(), params1)
		mock.SendShareInvitation(context.Background(), params2)

		if len(mock.SentEmails) != 2 {
			t.Errorf("expected 2 emails, got %d", len(mock.SentEmails))
		}
		if mock.SentEmails[0].ToEmail != "user1@example.com" {
			t.Errorf("expected first email to user1, got %s", mock.SentEmails[0].ToEmail)
		}
		if mock.SentEmails[1].ToEmail != "user2@example.com" {
			t.Errorf("expected second email to user2, got %s", mock.SentEmails[1].ToEmail)
		}
	})

	t.Run("fails when ShouldFail is set", func(t *testing.T) {
		mock := newMockService()
		mock.ShouldFail = true

		params := ShareInvitationParams{
			ToEmail: "test@example.com",
		}

		err := mock.SendShareInvitation(context.Background(), params)
		if err == nil {
			t.Error("expected error when ShouldFail is true")
		}
	})

	t.Run("Reset clears state", func(t *testing.T) {
		mock := newMockService()
		mock.ShouldFail = true
		mock.SentEmails = append(mock.SentEmails, ShareInvitationParams{})

		mock.reset()

		if mock.ShouldFail {
			t.Error("ShouldFail should be false after Reset")
		}
		if len(mock.SentEmails) != 0 {
			t.Error("SentEmails should be empty after Reset")
		}
	})
}

func TestRenderTextTemplate(t *testing.T) {
	frontendURL := "https://example.com"

	t.Run("renders basic template", func(t *testing.T) {
		params := ShareInvitationParams{
			ToEmail:      "test@example.com",
			SharerName:   "Alice",
			SharerEmail:  "alice@example.com",
			SessionTitle: "My Test Session",
			ShareURL:     "https://example.com/share/abc123",
		}

		result := renderTextTemplate(params, frontendURL)

		if !strings.Contains(result, "Alice") {
			t.Error("expected sharer name in template")
		}
		if !strings.Contains(result, "alice@example.com") {
			t.Error("expected sharer email in template")
		}
		if !strings.Contains(result, "My Test Session") {
			t.Error("expected session title in template")
		}
		if !strings.Contains(result, "https://example.com/share/abc123") {
			t.Error("expected share URL in template")
		}
		if !strings.Contains(result, "https://example.com/unsubscribe") {
			t.Error("expected unsubscribe URL in template")
		}
	})

	t.Run("includes expiration when set", func(t *testing.T) {
		expires := time.Date(2025, 12, 25, 0, 0, 0, 0, time.UTC)
		params := ShareInvitationParams{
			ToEmail:      "test@example.com",
			SharerName:   "Alice",
			SharerEmail:  "alice@example.com",
			SessionTitle: "Test",
			ShareURL:     "https://example.com",
			ExpiresAt:    &expires,
		}

		result := renderTextTemplate(params, frontendURL)

		if !strings.Contains(result, "December 25, 2025") {
			t.Error("expected expiration date in template")
		}
	})

	t.Run("uses Untitled Session when title is empty", func(t *testing.T) {
		params := ShareInvitationParams{
			ToEmail:      "test@example.com",
			SharerName:   "Alice",
			SharerEmail:  "alice@example.com",
			SessionTitle: "",
			ShareURL:     "https://example.com",
		}

		result := renderTextTemplate(params, frontendURL)

		if !strings.Contains(result, "Untitled Session") {
			t.Error("expected 'Untitled Session' when title is empty")
		}
	})
}

func TestRenderHTMLTemplate(t *testing.T) {
	frontendURL := "https://example.com"

	t.Run("renders valid HTML", func(t *testing.T) {
		params := ShareInvitationParams{
			ToEmail:      "test@example.com",
			SharerName:   "Alice",
			SharerEmail:  "alice@example.com",
			SessionTitle: "My Test Session",
			ShareURL:     "https://example.com/share/abc123",
		}

		result, err := renderHTMLTemplate(params, frontendURL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(result, "<!DOCTYPE html>") {
			t.Error("expected DOCTYPE in HTML template")
		}
		if !strings.Contains(result, "Alice") {
			t.Error("expected sharer name in HTML template")
		}
		if !strings.Contains(result, "My Test Session") {
			t.Error("expected session title in HTML template")
		}
		if !strings.Contains(result, "https://example.com/share/abc123") {
			t.Error("expected share URL in HTML template")
		}
		if !strings.Contains(result, "https://example.com/unsubscribe") {
			t.Error("expected unsubscribe URL in HTML template")
		}
	})

	t.Run("includes expiration when set", func(t *testing.T) {
		expires := time.Date(2025, 12, 25, 0, 0, 0, 0, time.UTC)
		params := ShareInvitationParams{
			ToEmail:      "test@example.com",
			SharerName:   "Alice",
			SharerEmail:  "alice@example.com",
			SessionTitle: "Test",
			ShareURL:     "https://example.com",
			ExpiresAt:    &expires,
		}

		result, err := renderHTMLTemplate(params, frontendURL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(result, "December 25, 2025") {
			t.Error("expected expiration date in HTML template")
		}
	})
}

// providerWordingCases is the locked test matrix for CF-353. Each row asserts
// that the rendered email (subject, plaintext body, HTML body) contains the
// brand phrase matching params.Provider and that no other brand phrase leaks
// in. The empty / unknown rows pin the neutral fallback wording.
//
// Use the longer phrases "Claude Code session" / "Codex session" — not just
// the brand names — as discriminators so substring matches can't false-match
// a future "Claude Codex" or similar.
var providerWordingCases = []struct {
	name     string
	provider string
	wants    []string // substrings that MUST appear
	forbids  []string // substrings that MUST NOT appear
}{
	{
		name:     "claude-code canonical",
		provider: "claude-code",
		wants:    []string{"Claude Code session transcript"},
		forbids:  []string{"Codex session", "shared a session transcript with you"},
	},
	{
		name:     "codex canonical",
		provider: "codex",
		wants:    []string{"Codex session transcript"},
		forbids:  []string{"Claude Code session", "shared a session transcript with you"},
	},
	{
		name:     "empty falls back to neutral",
		provider: "",
		wants:    []string{"shared a session transcript with you"},
		forbids:  []string{"Claude Code session", "Codex session"},
	},
	{
		name:     "unknown future provider falls back to neutral",
		provider: "windsurf",
		wants:    []string{"shared a session transcript with you"},
		forbids:  []string{"Claude Code session", "Codex session"},
	},
}

func TestRenderTextTemplate_ProviderAware(t *testing.T) {
	frontendURL := "https://example.com"
	for _, tc := range providerWordingCases {
		t.Run(tc.name, func(t *testing.T) {
			params := ShareInvitationParams{
				ToEmail:      "recipient@example.com",
				SharerName:   "Alice",
				SharerEmail:  "alice@example.com",
				SessionTitle: "Some Session",
				ShareURL:     "https://example.com/share/abc",
				Provider:     tc.provider,
				ShareID:      "share-" + tc.name,
			}
			body := renderTextTemplate(params, frontendURL)
			for _, want := range tc.wants {
				if !strings.Contains(body, want) {
					t.Errorf("plaintext body missing required phrase %q for provider=%q\nbody:\n%s",
						want, tc.provider, body)
				}
			}
			for _, forbid := range tc.forbids {
				if strings.Contains(body, forbid) {
					t.Errorf("plaintext body leaked forbidden phrase %q for provider=%q\nbody:\n%s",
						forbid, tc.provider, body)
				}
			}
		})
	}
}

func TestRenderHTMLTemplate_ProviderAware(t *testing.T) {
	frontendURL := "https://example.com"
	for _, tc := range providerWordingCases {
		t.Run(tc.name, func(t *testing.T) {
			params := ShareInvitationParams{
				ToEmail:      "recipient@example.com",
				SharerName:   "Alice",
				SharerEmail:  "alice@example.com",
				SessionTitle: "Some Session",
				ShareURL:     "https://example.com/share/abc",
				Provider:     tc.provider,
				ShareID:      "share-" + tc.name,
			}
			body, err := renderHTMLTemplate(params, frontendURL)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, want := range tc.wants {
				if !strings.Contains(body, want) {
					t.Errorf("HTML body missing required phrase %q for provider=%q", want, tc.provider)
				}
			}
			for _, forbid := range tc.forbids {
				if strings.Contains(body, forbid) {
					t.Errorf("HTML body leaked forbidden phrase %q for provider=%q", forbid, tc.provider)
				}
			}
		})
	}
}

func TestComposeSubject_ProviderAware(t *testing.T) {
	for _, tc := range providerWordingCases {
		t.Run(tc.name, func(t *testing.T) {
			params := ShareInvitationParams{
				SharerName: "Alice",
				ToEmail:    "recipient@example.com",
				Provider:   tc.provider,
				ShareID:    "share-" + tc.name,
			}
			subject := composeSubject(context.Background(), params)
			for _, want := range tc.wants {
				// The subject reuses the same phrase as the body for known
				// providers ("Claude Code session transcript") and for the
				// neutral fallback ("shared a session transcript with you").
				if !strings.Contains(subject, want) {
					t.Errorf("subject missing required phrase %q for provider=%q\nsubject: %s",
						want, tc.provider, subject)
				}
			}
			for _, forbid := range tc.forbids {
				if strings.Contains(subject, forbid) {
					t.Errorf("subject leaked forbidden phrase %q for provider=%q\nsubject: %s",
						forbid, tc.provider, subject)
				}
			}
		})
	}
}

func TestComposeSubject_NormalizesLegacyClaudeCode(t *testing.T) {
	// Defense in depth: even if a caller forgets to call
	// models.NormalizeProvider, the legacy display form "Claude Code" must not
	// produce a different brand phrase or trigger the unknown-provider log.
	params := ShareInvitationParams{
		SharerName: "Alice",
		ToEmail:    "recipient@example.com",
		Provider:   "Claude Code", // legacy display form
		ShareID:    "share-legacy",
	}
	subject := composeSubject(context.Background(), params)
	if !strings.Contains(subject, "Claude Code session transcript") {
		t.Errorf("legacy provider %q must produce Claude Code wording, got: %s", "Claude Code", subject)
	}
	if strings.Contains(subject, "shared a session transcript with you") {
		t.Errorf("legacy provider %q must not fall back to neutral wording: %s", "Claude Code", subject)
	}
}

// captureLogs runs fn with a slog logger that writes to a buffer, then returns
// the captured JSON log lines parsed into maps. Routes via logger.WithLogger
// so any code path calling logger.Ctx(ctx) picks up our buffer.
func captureLogs(t *testing.T, fn func(ctx context.Context)) []map[string]any {
	t.Helper()
	var buf bytes.Buffer
	h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	ctx := logger.WithLogger(context.Background(), slog.New(h))
	fn(ctx)
	var records []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("log line not JSON: %q (%v)", line, err)
		}
		records = append(records, rec)
	}
	return records
}

func TestHumanProviderLabel_UnknownProviderLogsError(t *testing.T) {
	cases := []struct {
		name     string
		provider string
	}{
		{"empty", ""},
		{"unknown future provider", "windsurf"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			records := captureLogs(t, func(ctx context.Context) {
				_ = humanProviderLabel(ctx, tc.provider, "share-xyz", "recipient@example.com")
			})
			var matched bool
			for _, rec := range records {
				if rec["level"] != "ERROR" {
					continue
				}
				if rec["provider"] != tc.provider {
					continue
				}
				if rec["share_id"] != "share-xyz" {
					continue
				}
				if rec["to_email"] != "recipient@example.com" {
					continue
				}
				matched = true
				break
			}
			if !matched {
				t.Errorf("expected an ERROR log with provider=%q share_id=share-xyz to_email=recipient@example.com; got records=%v",
					tc.provider, records)
			}
		})
	}
}

func TestHumanProviderLabel_KnownProvidersDoNotLog(t *testing.T) {
	cases := []struct {
		name     string
		provider string
	}{
		{"claude-code", "claude-code"},
		{"codex", "codex"},
		{"legacy Claude Code display form", "Claude Code"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			records := captureLogs(t, func(ctx context.Context) {
				_ = humanProviderLabel(ctx, tc.provider, "share-xyz", "recipient@example.com")
			})
			for _, rec := range records {
				if rec["level"] == "ERROR" {
					t.Errorf("known provider %q should not emit ERROR log; got %v", tc.provider, rec)
				}
			}
		})
	}
}

