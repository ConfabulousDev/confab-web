package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/storage"
)

// getCapabilities drives handleCapabilities through a recorder and returns the
// recorder plus the decoded response. Mirrors version_test.go's convention for
// stateless public handlers.
func getCapabilities(t *testing.T, s *Server) (*httptest.ResponseRecorder, capabilitiesResponse) {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/v1/capabilities", nil)
	rr := httptest.NewRecorder()
	s.handleCapabilities(rr, req)

	var resp capabilitiesResponse
	if err := json.NewDecoder(strings.NewReader(rr.Body.String())).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v (body: %s)", err, rr.Body.String())
	}
	return rr, resp
}

func TestHandleCapabilities(t *testing.T) {
	t.Run("advertises workflow_files and workflow_journal as true", func(t *testing.T) {
		s := &Server{}
		rr, resp := getCapabilities(t, s)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
		if !resp.WorkflowFiles {
			t.Error("workflow_files must be true on a build that supports nested workflow agent files")
		}
		if !resp.WorkflowJournal {
			t.Error("workflow_journal must be true on a build that supports the workflow_journal file_type")
		}
	})

	t.Run("advertises opencode_subagents as true (CF-539)", func(t *testing.T) {
		s := &Server{}
		rr, resp := getCapabilities(t, s)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
		if !resp.OpencodeSubagents {
			t.Error("opencode_subagents must be true on a build that ingests OpenCode subagent JSONL files (file_type='agent')")
		}
	})

	t.Run("response is application/json", func(t *testing.T) {
		s := &Server{}
		rr, _ := getCapabilities(t, s)

		if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
	})

	t.Run("body carries both capability keys", func(t *testing.T) {
		s := &Server{}
		rr, _ := getCapabilities(t, s)

		body := rr.Body.String()
		for _, key := range []string{`"workflow_files"`, `"workflow_journal"`, `"opencode_subagents"`} {
			if !strings.Contains(body, key) {
				t.Errorf("response body missing %s; got: %s", key, body)
			}
		}
	})

	t.Run("does not depend on db, storage, or update checker", func(t *testing.T) {
		// All external dependencies nil: the signal is static, so the endpoint
		// must answer with no panic.
		s := &Server{db: nil, storage: nil, updateChecker: nil}
		rr, resp := getCapabilities(t, s)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 with all deps nil, got %d", rr.Code)
		}
		if !resp.WorkflowFiles || !resp.WorkflowJournal {
			t.Error("capabilities must be reported independent of any external dependency")
		}
	})

	// Wire-level: prove the route is registered at /api/v1/capabilities in
	// SetupRoutes and reachable with no auth header. The direct handler tests
	// above can't catch a missing/misplaced route registration.
	t.Run("route is registered under /api/v1/capabilities and needs no auth", func(t *testing.T) {
		srv := NewServer(&db.DB{}, &storage.S3Storage{}, &auth.OAuthConfig{}, nil, BuildInfo{})
		handler := srv.SetupRoutes()

		req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("GET /api/v1/capabilities through full router (no auth) = %d, want 200", rr.Code)
		}
		var resp capabilitiesResponse
		if err := json.NewDecoder(strings.NewReader(rr.Body.String())).Decode(&resp); err != nil {
			t.Fatalf("failed to decode router response: %v (body: %s)", err, rr.Body.String())
		}
		if !resp.WorkflowFiles || !resp.WorkflowJournal {
			t.Error("workflow capabilities must be true through the router")
		}
	})
}
