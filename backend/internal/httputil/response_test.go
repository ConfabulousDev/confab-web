package httputil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRespondJSON_SetsHeadersAndStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	RespondJSON(rec, http.StatusCreated, map[string]string{"hello": "world"})

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}

	var decoded map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if decoded["hello"] != "world" {
		t.Errorf("decoded[hello] = %q, want world", decoded["hello"])
	}
}

func TestRespondJSON_NilData(t *testing.T) {
	rec := httptest.NewRecorder()
	RespondJSON(rec, http.StatusOK, nil)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	// json.NewEncoder writes "null\n" for nil
	if got := rec.Body.String(); got != "null\n" {
		t.Errorf("body = %q, want %q", got, "null\n")
	}
}

func TestRespondJSON_EncodeError(t *testing.T) {
	// channels are not encodable — Encode() returns an error.
	// The function does not surface the error to the caller; it just writes
	// nothing to the body. Headers and status are still set.
	rec := httptest.NewRecorder()
	RespondJSON(rec, http.StatusOK, make(chan int))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
}

func TestRespondError_ShapeAndStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	RespondError(rec, http.StatusBadRequest, "bad input")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	var decoded map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if decoded["error"] != "bad input" {
		t.Errorf("error = %q, want %q", decoded["error"], "bad input")
	}
	if len(decoded) != 1 {
		t.Errorf("body has %d keys, want 1", len(decoded))
	}
}
