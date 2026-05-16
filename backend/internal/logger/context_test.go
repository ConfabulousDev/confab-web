package logger

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
)

func TestCtx_ReturnsDefaultWhenAbsent(t *testing.T) {
	got := Ctx(context.Background())
	if got == nil {
		t.Fatal("Ctx returned nil")
	}
	// Should equal slog.Default
	if got != slog.Default() {
		t.Error("Ctx should fall back to slog.Default when no logger in context")
	}
}

func TestWithLoggerAndCtx_RoundTrip(t *testing.T) {
	enriched := slog.Default().With("user_id", int64(42))
	ctx := WithLogger(context.Background(), enriched)
	got := Ctx(ctx)
	if got != enriched {
		t.Error("Ctx did not return the logger set by WithLogger")
	}
}

func TestMiddleware_InjectsRequestScopedLogger(t *testing.T) {
	var captured *slog.Logger

	// Use chi's RequestID middleware so the request has a req_id we can find.
	final := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = Ctx(r.Context())
	})

	stack := middleware.RequestID(Middleware(final))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/foo", nil)
	stack.ServeHTTP(rec, req)

	if captured == nil {
		t.Fatal("logger was not captured")
	}
	// Should not be the bare default — Middleware adds req_id.
	if captured == slog.Default() {
		t.Error("Middleware did not enrich logger with req_id")
	}
}

func TestMiddleware_WorksWithoutRequestID(t *testing.T) {
	var captured *slog.Logger
	stack := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = Ctx(r.Context())
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/foo", nil)
	stack.ServeHTTP(rec, req)

	if captured == nil {
		t.Fatal("logger was not captured")
	}
	// Without RequestID middleware, the default (unenriched) logger is injected.
}
