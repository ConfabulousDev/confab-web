package api

import (
	"net/http"
	"strings"

	"github.com/ConfabulousDev/confab-web/internal/logger"
)

// validateContentType middleware ensures POST/PUT/PATCH requests have proper Content-Type
func validateContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only validate for requests with a body
		method := r.Method
		if method == "POST" || method == "PUT" || method == "PATCH" {
			log := logger.Ctx(r.Context())
			contentType := r.Header.Get("Content-Type")

			// Content-Type must be present
			if contentType == "" {
				log.Info("Request missing Content-Type header", "method", method, "path", r.URL.Path)
				http.Error(w, "Content-Type header required", http.StatusUnsupportedMediaType)
				return
			}

			// Extract media type (ignore charset and other parameters)
			// e.g., "application/json; charset=utf-8" → "application/json"
			mediaType := contentType
			if idx := strings.Index(contentType, ";"); idx != -1 {
				mediaType = strings.TrimSpace(contentType[:idx])
			}

			// Must be application/json
			if mediaType != "application/json" {
				log.Info("Request with invalid Content-Type", "method", method, "path", r.URL.Path, "content_type", mediaType)
				http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
