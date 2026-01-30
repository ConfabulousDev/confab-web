package email

import (
	"context"
	"testing"
	"time"
)

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
		mock := NewMockService()
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
		mock := NewMockService()
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
		mock := NewMockService()
		service := NewRateLimitedService(mock, 5)

		// Check if we can send 3 emails (should succeed)
		err := service.CheckRateLimit(1, 3)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Check if we can send 6 emails (should fail)
		err = service.CheckRateLimit(1, 6)
		if err != ErrRateLimitExceeded {
			t.Errorf("expected ErrRateLimitExceeded, got %v", err)
		}
	})
}

func TestMockService(t *testing.T) {
	t.Run("records sent emails", func(t *testing.T) {
		mock := NewMockService()

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
		mock := NewMockService()
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
		mock := NewMockService()
		mock.ShouldFail = true
		mock.SentEmails = append(mock.SentEmails, ShareInvitationParams{})

		mock.Reset()

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

		if !contains(result, "Alice") {
			t.Error("expected sharer name in template")
		}
		if !contains(result, "alice@example.com") {
			t.Error("expected sharer email in template")
		}
		if !contains(result, "My Test Session") {
			t.Error("expected session title in template")
		}
		if !contains(result, "https://example.com/share/abc123") {
			t.Error("expected share URL in template")
		}
		if !contains(result, "https://example.com/unsubscribe") {
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

		if !contains(result, "December 25, 2025") {
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

		if !contains(result, "Untitled Session") {
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

		if !contains(result, "<!DOCTYPE html>") {
			t.Error("expected DOCTYPE in HTML template")
		}
		if !contains(result, "Alice") {
			t.Error("expected sharer name in HTML template")
		}
		if !contains(result, "My Test Session") {
			t.Error("expected session title in HTML template")
		}
		if !contains(result, "https://example.com/share/abc123") {
			t.Error("expected share URL in HTML template")
		}
		if !contains(result, "https://example.com/unsubscribe") {
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

		if !contains(result, "December 25, 2025") {
			t.Error("expected expiration date in HTML template")
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
