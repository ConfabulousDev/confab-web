package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/storage"
)

// getVersion drives handleVersion through a recorder and returns the recorder
// plus the decoded response. Mirrors the direct-httptest convention used by
// auth_config_test.go / pricing_test.go for stateless public handlers.
func getVersion(t *testing.T, s *Server) (*httptest.ResponseRecorder, versionResponse) {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/v1/version", nil)
	rr := httptest.NewRecorder()
	s.handleVersion(rr, req)

	var resp versionResponse
	if err := json.NewDecoder(strings.NewReader(rr.Body.String())).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v (body: %s)", err, rr.Body.String())
	}
	return rr, resp
}

func TestHandleVersion(t *testing.T) {
	t.Run("returns 200 with version and go_version", func(t *testing.T) {
		s := &Server{buildInfo: BuildInfo{Version: "v0.4.3"}}
		rr, resp := getVersion(t, s)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
		if resp.Version != "v0.4.3" {
			t.Errorf("version = %q, want %q", resp.Version, "v0.4.3")
		}
		if resp.GoVersion != runtime.Version() {
			t.Errorf("go_version = %q, want %q", resp.GoVersion, runtime.Version())
		}
	})

	t.Run("response is application/json", func(t *testing.T) {
		s := &Server{buildInfo: BuildInfo{Version: "v0.4.3"}}
		rr, _ := getVersion(t, s)

		if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
	})

	t.Run("body carries version and go_version keys", func(t *testing.T) {
		s := &Server{buildInfo: BuildInfo{Version: "v0.4.3"}}
		rr, _ := getVersion(t, s)

		body := rr.Body.String()
		for _, key := range []string{`"version"`, `"go_version"`} {
			if !strings.Contains(body, key) {
				t.Errorf("response body missing %s; got: %s", key, body)
			}
		}
	})

	t.Run("dev builds report dev placeholder for empty version", func(t *testing.T) {
		s := &Server{buildInfo: BuildInfo{Version: ""}}
		rr, resp := getVersion(t, s)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
		if resp.Version != "dev" {
			t.Errorf("version = %q, want %q for empty build version", resp.Version, "dev")
		}
		if resp.Version == "" {
			t.Error("version must never be empty")
		}
	})

	t.Run("does not depend on db, storage, or update checker", func(t *testing.T) {
		// All external dependencies nil: the endpoint must answer from build
		// info + runtime alone, with no panic (AC #2).
		s := &Server{
			db:            nil,
			storage:       nil,
			updateChecker: nil,
			buildInfo:     BuildInfo{Version: "v0.4.3"},
		}
		rr, resp := getVersion(t, s)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 with all deps nil, got %d", rr.Code)
		}
		if resp.GoVersion == "" {
			t.Error("go_version must be populated from runtime.Version()")
		}
	})

	t.Run("commit and build_time are echoed verbatim when set", func(t *testing.T) {
		const fullSHA = "224ede8a1b2c3d4e5f60718293a4b5c6d7e8f901"
		const buildTime = "2026-05-29T21:00:00Z"
		s := &Server{buildInfo: BuildInfo{
			Version:   "v0.4.3",
			Commit:    fullSHA,
			BuildTime: buildTime,
		}}
		rr, resp := getVersion(t, s)

		if resp.Commit != fullSHA {
			t.Errorf("commit = %q, want full SHA %q (no truncation)", resp.Commit, fullSHA)
		}
		if resp.BuildTime != buildTime {
			t.Errorf("build_time = %q, want %q", resp.BuildTime, buildTime)
		}
		body := rr.Body.String()
		if !strings.Contains(body, `"commit":"`+fullSHA+`"`) {
			t.Errorf("body missing full commit; got: %s", body)
		}
	})

	t.Run("commit and build_time are omitted from the wire when empty", func(t *testing.T) {
		s := &Server{buildInfo: BuildInfo{Version: "dev"}}
		rr, _ := getVersion(t, s)

		body := rr.Body.String()
		if strings.Contains(body, "commit") {
			t.Errorf("commit should be omitted when empty; got: %s", body)
		}
		if strings.Contains(body, "build_time") {
			t.Errorf("build_time should be omitted when empty; got: %s", body)
		}
	})

	// Wire-level: prove the route is actually registered at /api/v1/version in
	// SetupRoutes and reachable with no auth header (AC #1, AC #4). The direct
	// handler tests above can't catch a missing/misplaced route registration.
	t.Run("route is registered under /api/v1/version and needs no auth", func(t *testing.T) {
		srv := NewServer(&db.DB{}, &storage.S3Storage{}, &auth.OAuthConfig{}, nil, BuildInfo{Version: "v9.9.9"})
		handler := srv.SetupRoutes()

		req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("GET /api/v1/version through full router (no auth) = %d, want 200", rr.Code)
		}
		var resp versionResponse
		if err := json.NewDecoder(strings.NewReader(rr.Body.String())).Decode(&resp); err != nil {
			t.Fatalf("failed to decode router response: %v (body: %s)", err, rr.Body.String())
		}
		if resp.Version != "v9.9.9" {
			t.Errorf("version = %q, want %q through router", resp.Version, "v9.9.9")
		}
		if resp.GoVersion == "" {
			t.Error("go_version must be populated through the router")
		}
	})
}
