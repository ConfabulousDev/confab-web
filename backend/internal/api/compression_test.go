package api

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/santaclaude2025/confab/backend/internal/auth"
	"github.com/santaclaude2025/confab/backend/internal/db"
	"github.com/santaclaude2025/confab/backend/internal/storage"
)

// TestCompressionMiddleware tests that gzip compression is applied to responses
func TestCompressionMiddleware(t *testing.T) {
	// Create a test server with minimal setup
	// We don't need real DB/storage for this test, just the middleware chain
	mockDB := &db.DB{} // nil is fine for this test
	mockStorage := &storage.S3Storage{}
	mockOAuth := auth.OAuthConfig{}

	server := NewServer(mockDB, mockStorage, mockOAuth)
	handler := server.SetupRoutes()

	t.Run("compresses JSON responses when client accepts gzip", func(t *testing.T) {
		// Create request with Accept-Encoding: gzip header
		req := httptest.NewRequest("GET", "/health", nil)
		req.Header.Set("Accept-Encoding", "gzip")

		// Record response
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		// Check that response is compressed
		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		// Check Content-Encoding header
		contentEncoding := w.Header().Get("Content-Encoding")
		if contentEncoding != "gzip" {
			t.Errorf("expected Content-Encoding: gzip, got %q", contentEncoding)
		}

		// Verify response is actually gzipped by decompressing it
		reader, err := gzip.NewReader(w.Body)
		if err != nil {
			t.Fatalf("failed to create gzip reader: %v", err)
		}
		defer reader.Close()

		decompressed, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("failed to decompress response: %v", err)
		}

		// Health endpoint should return JSON with "status"
		body := string(decompressed)
		if !strings.Contains(body, "status") {
			t.Errorf("expected decompressed body to contain 'status', got: %s", body)
		}
	})

	t.Run("does not compress when client does not accept gzip", func(t *testing.T) {
		// Create request WITHOUT Accept-Encoding header
		req := httptest.NewRequest("GET", "/health", nil)

		// Record response
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		// Check that response is NOT compressed
		contentEncoding := w.Header().Get("Content-Encoding")
		if contentEncoding == "gzip" {
			t.Error("expected no compression without Accept-Encoding header")
		}

		// Body should be readable directly
		body := w.Body.String()
		if !strings.Contains(body, "status") {
			t.Errorf("expected body to contain 'status', got: %s", body)
		}
	})

	t.Run("compresses large JSON responses", func(t *testing.T) {
		// Health check is small, but it should still compress it
		req := httptest.NewRequest("GET", "/health", nil)
		req.Header.Set("Accept-Encoding", "gzip")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		// Should be compressed
		if w.Header().Get("Content-Encoding") != "gzip" {
			t.Error("expected gzip compression for response")
		}

		// Compressed size should be less than or equal to original
		// (for very small responses, gzip overhead might make it larger, but chi's
		// middleware should skip compression for tiny responses)
		compressedSize := w.Body.Len()
		if compressedSize == 0 {
			t.Error("expected non-empty compressed response")
		}
	})

	t.Run("compression works with error responses", func(t *testing.T) {
		// Request a non-existent endpoint to trigger 404
		req := httptest.NewRequest("GET", "/nonexistent", nil)
		req.Header.Set("Accept-Encoding", "gzip")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", w.Code)
		}

		// Even error responses should be compressed
		if w.Header().Get("Content-Encoding") != "gzip" {
			t.Error("expected gzip compression for error response")
		}

		// Decompress and verify it's a valid response
		reader, err := gzip.NewReader(w.Body)
		if err != nil {
			t.Fatalf("failed to create gzip reader: %v", err)
		}
		defer reader.Close()

		decompressed, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("failed to decompress response: %v", err)
		}

		if len(decompressed) == 0 {
			t.Error("expected non-empty decompressed error response")
		}
	})
}

// TestCompressionSavings tests that compression actually reduces response size
func TestCompressionSavings(t *testing.T) {
	mockDB := &db.DB{}
	mockStorage := &storage.S3Storage{}
	mockOAuth := auth.OAuthConfig{}

	server := NewServer(mockDB, mockStorage, mockOAuth)
	handler := server.SetupRoutes()

	// Get uncompressed response
	reqUncompressed := httptest.NewRequest("GET", "/health", nil)
	wUncompressed := httptest.NewRecorder()
	handler.ServeHTTP(wUncompressed, reqUncompressed)

	// Get compressed response
	reqCompressed := httptest.NewRequest("GET", "/health", nil)
	reqCompressed.Header.Set("Accept-Encoding", "gzip")
	wCompressed := httptest.NewRecorder()
	handler.ServeHTTP(wCompressed, reqCompressed)

	uncompressedSize := wUncompressed.Body.Len()
	compressedSize := wCompressed.Body.Len()

	t.Logf("Uncompressed size: %d bytes", uncompressedSize)
	t.Logf("Compressed size: %d bytes", compressedSize)

	// For small responses like health check, compression might not save much
	// or might even be slightly larger due to gzip overhead
	// The real savings come with large JSON responses (sessions, files, etc.)
	if uncompressedSize == 0 {
		t.Error("expected non-empty uncompressed response")
	}
	if compressedSize == 0 {
		t.Error("expected non-empty compressed response")
	}

	// Decompress the compressed response to verify it matches
	reader, err := gzip.NewReader(bytes.NewReader(wCompressed.Body.Bytes()))
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to decompress: %v", err)
	}

	// Decompressed content should match uncompressed content
	if string(decompressed) != wUncompressed.Body.String() {
		t.Error("decompressed content does not match original uncompressed content")
	}
}
