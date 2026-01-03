package api

import (
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// SpanEnricher is a middleware that enriches the current span with request metadata.
// Adds CLI version, OS, and architecture when the request comes from the Confab CLI.
func SpanEnricher(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse CLI user agent and enrich span if present
		if ua := r.Header.Get("User-Agent"); ua != "" {
			if cli := ParseCLIUserAgent(ua); cli != nil {
				span := trace.SpanFromContext(r.Context())
				span.SetAttributes(
					attribute.String("cli.version", cli.Version),
					attribute.String("cli.os", cli.OS),
					attribute.String("cli.arch", cli.Arch),
				)
			}
		}

		next.ServeHTTP(w, r)
	})
}
