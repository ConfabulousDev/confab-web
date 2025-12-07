package api

import (
	"bytes"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/ConfabulousDev/confab-web/internal/logger"
)

// maxDebugBodySize is the maximum size of request/response bodies to log
// Larger bodies are truncated to avoid log bloat
const maxDebugBodySize = 10 * 1024 // 10KB

// debugLoggingMiddleware logs full request and response bodies when debug logging is enabled
// This should be placed after decompression middleware so we log the decompressed content
func debugLoggingMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip if debug logging is not enabled
			if !logger.IsDebug() {
				next.ServeHTTP(w, r)
				return
			}

			requestID := middleware.GetReqID(r.Context())

			// Read and log request body
			if r.Body != nil && r.ContentLength != 0 {
				// Read FULL body so we can restore it completely for downstream handlers
				fullBody, _ := io.ReadAll(r.Body)
				// Restore FULL body for downstream handlers
				r.Body = io.NopCloser(bytes.NewReader(fullBody))

				// Only log a truncated version to avoid log bloat
				logBody := fullBody
				truncated := len(fullBody) > maxDebugBodySize
				if truncated {
					logBody = fullBody[:maxDebugBodySize]
				}

				logger.Debug("request body",
					"request_id", requestID,
					"method", r.Method,
					"path", r.URL.Path,
					"body", string(logBody),
					"truncated", truncated,
				)
			}

			// Wrap response writer to capture response body
			ww := &responseCapture{
				ResponseWriter: w,
				body:           &bytes.Buffer{},
				maxSize:        maxDebugBodySize,
			}

			next.ServeHTTP(ww, r)

			// Log response body
			responseBody := ww.body.String()
			truncated := ww.truncated

			logger.Debug("response body",
				"request_id", requestID,
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.status,
				"body", responseBody,
				"truncated", truncated,
			)
		})
	}
}

// responseCapture wraps http.ResponseWriter to capture the response body
type responseCapture struct {
	http.ResponseWriter
	body      *bytes.Buffer
	status    int
	maxSize   int
	truncated bool
}

func (w *responseCapture) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseCapture) Write(b []byte) (int, error) {
	// Capture up to maxSize bytes
	if !w.truncated && w.body.Len() < w.maxSize {
		remaining := w.maxSize - w.body.Len()
		if len(b) <= remaining {
			w.body.Write(b)
		} else {
			w.body.Write(b[:remaining])
			w.truncated = true
		}
	} else if w.body.Len() >= w.maxSize {
		w.truncated = true
	}

	return w.ResponseWriter.Write(b)
}
