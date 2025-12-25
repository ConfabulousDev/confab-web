package logger

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

type ctxKey struct{}

// Middleware creates a request-scoped logger with req_id and stores it in context.
// Must be placed after chi's RequestID middleware.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := slog.Default()
		if reqID := middleware.GetReqID(r.Context()); reqID != "" {
			log = log.With("req_id", reqID)
		}
		ctx := context.WithValue(r.Context(), ctxKey{}, log)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Ctx retrieves the request-scoped logger from context.
// Falls back to the default logger if not found.
func Ctx(ctx context.Context) *slog.Logger {
	if log, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok {
		return log
	}
	return slog.Default()
}

// WithLogger stores an enriched logger in context.
// Use this to add fields (like user_id) after initial middleware setup.
func WithLogger(ctx context.Context, log *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, log)
}
