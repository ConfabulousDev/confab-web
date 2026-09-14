package email

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// These tests point resendEmailsURL at an httptest server. The var is
// package-global, so they must not call t.Parallel().

func overrideResendURL(t *testing.T, value string) {
	t.Helper()
	orig := resendEmailsURL
	resendEmailsURL = value
	t.Cleanup(func() { resendEmailsURL = orig })
}

func sampleInvitation() ShareInvitationParams {
	return ShareInvitationParams{
		ToEmail:      "recipient@example.com",
		SharerName:   "Alice",
		SharerEmail:  "alice@example.com",
		SessionTitle: "Refactor the parser",
		ShareURL:     "https://confab.test/sessions/abc",
		Provider:     "claude-code",
		ShareID:      "share-1",
	}
}

func TestNewResendService_SetsFieldsAndTimeout(t *testing.T) {
	s := NewResendService("re_key", "noreply@confab.test", "Confab", "https://confab.test")
	if s.apiKey != "re_key" || s.fromAddress != "noreply@confab.test" || s.fromName != "Confab" || s.frontendURL != "https://confab.test" {
		t.Errorf("unexpected fields: %+v", s)
	}
	if s.httpClient == nil {
		t.Fatal("httpClient must be set")
	}
	if s.httpClient.Timeout != 10*time.Second {
		t.Errorf("httpClient.Timeout = %v, want 10s", s.httpClient.Timeout)
	}
}

func TestResendSendShareInvitation_PostsAuthenticatedJSON(t *testing.T) {
	var (
		gotMethod, gotAuth, gotContentType string
		gotBody                            resendRequest
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &gotBody); err != nil {
			t.Errorf("request body is not JSON: %v (%s)", err, raw)
		}
		_, _ = w.Write([]byte(`{"id":"email-1"}`))
	}))
	t.Cleanup(srv.Close)
	overrideResendURL(t, srv.URL)

	s := NewResendService("re_secret", "noreply@confab.test", "Confab", "https://confab.test")
	if err := s.SendShareInvitation(context.Background(), sampleInvitation()); err != nil {
		t.Fatalf("SendShareInvitation: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if gotAuth != "Bearer re_secret" {
		t.Errorf("Authorization = %q, want Bearer re_secret", gotAuth)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}
	if gotBody.From != "Confab <noreply@confab.test>" {
		t.Errorf("from = %q, want Confab <noreply@confab.test>", gotBody.From)
	}
	if len(gotBody.To) != 1 || gotBody.To[0] != "recipient@example.com" {
		t.Errorf("to = %v, want [recipient@example.com]", gotBody.To)
	}
	if gotBody.Subject != "Alice shared a Claude Code session transcript with you" {
		t.Errorf("subject = %q", gotBody.Subject)
	}
	for name, body := range map[string]string{"html": gotBody.HTML, "text": gotBody.Text} {
		if !strings.Contains(body, "https://confab.test/sessions/abc") {
			t.Errorf("%s body missing share URL", name)
		}
		if !strings.Contains(body, "https://confab.test/unsubscribe") {
			t.Errorf("%s body missing unsubscribe link", name)
		}
	}
}

func TestResendSendShareInvitation_ErrorStatusReturnsError(t *testing.T) {
	for _, status := range []int{http.StatusUnprocessableEntity, http.StatusTooManyRequests, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"name":"validation_error","message":"bad from"}`))
			}))
			t.Cleanup(srv.Close)
			overrideResendURL(t, srv.URL)

			s := NewResendService("k", "noreply@confab.test", "Confab", "https://confab.test")
			err := s.SendShareInvitation(context.Background(), sampleInvitation())
			if err == nil {
				t.Fatal("expected error for Resend error status")
			}
			if want := "status " + strconv.Itoa(status); !strings.Contains(err.Error(), want) {
				t.Errorf("error = %v, want it to mention %q", err, want)
			}
		})
	}

	t.Run("error status with non-JSON body still errors", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`<html>bad gateway</html>`))
		}))
		t.Cleanup(srv.Close)
		overrideResendURL(t, srv.URL)

		s := NewResendService("k", "noreply@confab.test", "Confab", "https://confab.test")
		if err := s.SendShareInvitation(context.Background(), sampleInvitation()); err == nil {
			t.Fatal("expected error for 502")
		}
	})
}

func TestResendSendShareInvitation_TransportErrorReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	closedURL := srv.URL
	srv.Close()
	overrideResendURL(t, closedURL)

	s := NewResendService("k", "noreply@confab.test", "Confab", "https://confab.test")
	err := s.SendShareInvitation(context.Background(), sampleInvitation())
	if err == nil {
		t.Fatal("expected transport error")
	}
	if !strings.Contains(err.Error(), "failed to send request") {
		t.Errorf("error = %v, want it to mention failed to send request", err)
	}
}

func TestResendSendShareInvitation_CanceledContextReturnsError(t *testing.T) {
	var hit atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit.Store(true)
	}))
	t.Cleanup(srv.Close)
	overrideResendURL(t, srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := NewResendService("k", "noreply@confab.test", "Confab", "https://confab.test")
	if err := s.SendShareInvitation(ctx, sampleInvitation()); err == nil {
		t.Fatal("expected error for canceled context")
	}
	if hit.Load() {
		t.Error("request must not reach Resend when the context is already canceled")
	}
}
