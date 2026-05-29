package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/pricingsource"
)

func pricingDoc(updated time.Time, gpt5Input float64) pricingsource.Document {
	return pricingsource.Document{
		SchemaVersion: 0,
		UpdatedAt:     updated,
		Pricing: map[string]map[string]pricingsource.Rate{
			"codex": {"gpt-5": {Input: gpt5Input, Output: 10, CacheWrite: 0, CacheRead: 0.125}},
		},
	}
}

func TestHandlePricingServesEffectiveDocument(t *testing.T) {
	emb := pricingDoc(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), 1.25)
	// Empty URL ⇒ disabled ⇒ serves the embedded floor.
	s := &Server{pricingSource: pricingsource.NewSource(emb, "", time.Hour)}

	req := httptest.NewRequest("GET", "/api/v1/pricing", nil)
	rr := httptest.NewRecorder()
	s.handlePricing(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if cc := rr.Header().Get("Cache-Control"); cc != "public, max-age=3600" {
		t.Errorf("Cache-Control = %q, want %q", cc, "public, max-age=3600")
	}

	var got pricingsource.Document
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.SchemaVersion != 0 {
		t.Errorf("schema_version = %d, want 0", got.SchemaVersion)
	}
	if v := got.Pricing["codex"]["gpt-5"].Input; v != 1.25 {
		t.Errorf("served gpt-5 input = %v, want embedded 1.25", v)
	}
}

func TestHandlePricingReflectsNewerRemote(t *testing.T) {
	emb := pricingDoc(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), 1.25)
	remote := pricingDoc(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), 99)
	body, _ := json.Marshal(remote)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer srv.Close()

	s := &Server{pricingSource: pricingsource.NewSource(emb, srv.URL, time.Hour)}

	req := httptest.NewRequest("GET", "/api/v1/pricing", nil)
	rr := httptest.NewRecorder()
	s.handlePricing(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var got pricingsource.Document
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if v := got.Pricing["codex"]["gpt-5"].Input; v != 99 {
		t.Errorf("served gpt-5 input = %v, want newer remote 99", v)
	}
}
