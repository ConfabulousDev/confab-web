package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newOIDCFakeServer stands up an httptest server that serves canned
// discovery, token, and userinfo responses. The discovery hit counter is
// returned so caching tests can verify the lazy-discovery contract.
func newOIDCFakeServer(t *testing.T, tokenStatus int, tokenBody string, userStatus int, userBody string) (*httptest.Server, *OIDCEndpoints, *int) {
	t.Helper()
	var (
		srv           *httptest.Server
		discoveryHits int
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		discoveryHits++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 strings.TrimRight(srv.URL, "/"),
			"authorization_endpoint": srv.URL + "/authorize",
			"token_endpoint":         srv.URL + "/token",
			"userinfo_endpoint":      srv.URL + "/userinfo",
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(tokenStatus)
		_, _ = w.Write([]byte(tokenBody))
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(userStatus)
		_, _ = w.Write([]byte(userBody))
	})

	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	endpoints := &OIDCEndpoints{
		AuthorizationEndpoint: srv.URL + "/authorize",
		TokenEndpoint:         srv.URL + "/token",
		UserinfoEndpoint:      srv.URL + "/userinfo",
		Issuer:                strings.TrimRight(srv.URL, "/"),
	}
	return srv, endpoints, &discoveryHits
}

func TestExchangeOIDCCode(t *testing.T) {
	cases := []struct {
		name        string
		tokenBody   string
		wantToken   string
		wantErr     bool
		wantErrText string
	}{
		{
			name:      "success",
			tokenBody: `{"access_token":"abc123"}`,
			wantToken: "abc123",
		},
		{
			name:        "token endpoint error",
			tokenBody:   `{"error":"invalid_grant","error_description":"bad code"}`,
			wantErr:     true,
			wantErrText: "invalid_grant",
		},
		{
			name:        "missing access token",
			tokenBody:   `{}`,
			wantErr:     true,
			wantErrText: "no access token",
		},
		{
			name:      "malformed JSON",
			tokenBody: `<<<not-json>>>`,
			wantErr:   true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, endpoints, _ := newOIDCFakeServer(t,
				http.StatusOK, c.tokenBody,
				http.StatusOK, `{}`,
			)
			config := &OAuthConfig{OIDCClientID: "id", OIDCClientSecret: "sec", OIDCRedirectURL: "http://localhost/cb"}

			token, err := exchangeOIDCCode("code", config, endpoints)
			if c.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if c.wantErrText != "" && !strings.Contains(err.Error(), c.wantErrText) {
					t.Errorf("error should mention %q, got %v", c.wantErrText, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("exchangeOIDCCode: %v", err)
			}
			if token != c.wantToken {
				t.Errorf("token = %q, want %q", token, c.wantToken)
			}
		})
	}
}

func TestGetOIDCUser(t *testing.T) {
	cases := []struct {
		name        string
		userStatus  int
		userBody    string
		wantErr     bool
		wantErrText string
		check       func(t *testing.T, u *OIDCUser)
	}{
		{
			name:       "success",
			userStatus: http.StatusOK,
			userBody:   `{"sub":"u1","email":"alice@example.com","email_verified":true,"name":"Alice"}`,
			check: func(t *testing.T, u *OIDCUser) {
				if u.Sub != "u1" || u.Email != "alice@example.com" {
					t.Errorf("unexpected user: %+v", u)
				}
				if !u.IsEmailVerified() {
					t.Error("expected IsEmailVerified=true for bool true")
				}
			},
		},
		{
			name:        "non-200 returns error",
			userStatus:  http.StatusUnauthorized,
			userBody:    `{}`,
			wantErr:     true,
			wantErrText: "status 401",
		},
		{
			name:        "missing sub",
			userStatus:  http.StatusOK,
			userBody:    `{"email":"a@b.com"}`,
			wantErr:     true,
			wantErrText: "missing sub",
		},
		{
			name:        "missing email",
			userStatus:  http.StatusOK,
			userBody:    `{"sub":"u1"}`,
			wantErr:     true,
			wantErrText: "missing email",
		},
		{
			name:       "malformed JSON",
			userStatus: http.StatusOK,
			userBody:   `not-json`,
			wantErr:    true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, endpoints, _ := newOIDCFakeServer(t,
				http.StatusOK, `{}`,
				c.userStatus, c.userBody,
			)
			user, err := getOIDCUser("token", endpoints)
			if c.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if c.wantErrText != "" && !strings.Contains(err.Error(), c.wantErrText) {
					t.Errorf("error should mention %q, got %v", c.wantErrText, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("getOIDCUser: %v", err)
			}
			if c.check != nil {
				c.check(t, user)
			}
		})
	}
}

// TestGetOIDCEndpoints_CachesAfterFirstDiscovery checks the lazy-discovery
// path: the first call should hit the wire, the second should return the
// cached value without re-fetching.
func TestGetOIDCEndpoints_CachesAfterFirstDiscovery(t *testing.T) {
	srv, _, hits := newOIDCFakeServer(t,
		http.StatusOK, `{}`,
		http.StatusOK, `{}`,
	)
	config := &OAuthConfig{OIDCIssuerURL: srv.URL}

	first, err := config.getOIDCEndpoints()
	if err != nil {
		t.Fatalf("first getOIDCEndpoints: %v", err)
	}
	if first.TokenEndpoint == "" {
		t.Error("TokenEndpoint missing from discovery")
	}
	if *hits != 1 {
		t.Errorf("expected 1 discovery hit after first call, got %d", *hits)
	}

	second, err := config.getOIDCEndpoints()
	if err != nil {
		t.Fatalf("second getOIDCEndpoints: %v", err)
	}
	if second != first {
		t.Error("cached endpoints should be the same pointer")
	}
	if *hits != 1 {
		t.Errorf("cached call must not re-hit discovery; got %d hits", *hits)
	}
}

// TestGetOIDCEndpoints_DoesNotCacheFailure ensures a failed discovery is NOT
// cached, so a transient IdP outage can be retried.
func TestGetOIDCEndpoints_DoesNotCacheFailure(t *testing.T) {
	var attempts int
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			http.Error(w, "down", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 strings.TrimRight(srv.URL, "/"),
			"authorization_endpoint": srv.URL + "/a",
			"token_endpoint":         srv.URL + "/t",
			"userinfo_endpoint":      srv.URL + "/u",
		})
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	config := &OAuthConfig{OIDCIssuerURL: srv.URL}
	if _, err := config.getOIDCEndpoints(); err == nil {
		t.Fatal("expected first attempt to fail")
	}
	got, err := config.getOIDCEndpoints()
	if err != nil {
		t.Fatalf("second attempt should succeed after IdP recovers, got %v", err)
	}
	if got == nil {
		t.Fatal("expected endpoints on retry")
	}
	if attempts != 2 {
		t.Errorf("expected 2 discovery attempts, got %d", attempts)
	}
}
