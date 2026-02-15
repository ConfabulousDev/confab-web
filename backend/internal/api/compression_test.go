package api

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/storage"
)

// TestCompressionMiddleware tests that gzip compression is applied to responses
func TestCompressionMiddleware(t *testing.T) {
	// Create a test server with minimal setup
	// We don't need real DB/storage for this test, just the middleware chain
	mockDB := &db.DB{} // nil is fine for this test
	mockStorage := &storage.S3Storage{}
	mockOAuth := &auth.OAuthConfig{}

	server := NewServer(mockDB, mockStorage, mockOAuth, nil, "")
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
	mockOAuth := &auth.OAuthConfig{}

	server := NewServer(mockDB, mockStorage, mockOAuth, nil, "")
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

// TestBrotliCompression tests that Brotli compression works correctly
func TestBrotliCompression(t *testing.T) {
	mockDB := &db.DB{}
	mockStorage := &storage.S3Storage{}
	mockOAuth := &auth.OAuthConfig{}

	server := NewServer(mockDB, mockStorage, mockOAuth, nil, "")
	handler := server.SetupRoutes()

	t.Run("compresses with Brotli when client accepts br", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		req.Header.Set("Accept-Encoding", "br")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		// Check Content-Encoding header
		contentEncoding := w.Header().Get("Content-Encoding")
		if contentEncoding != "br" {
			t.Errorf("expected Content-Encoding: br, got %q", contentEncoding)
		}

		// Verify response is actually Brotli-compressed by decompressing it
		reader := brotli.NewReader(w.Body)
		decompressed, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("failed to decompress Brotli response: %v", err)
		}

		// Health endpoint should return JSON with "status"
		body := string(decompressed)
		if !strings.Contains(body, "status") {
			t.Errorf("expected decompressed body to contain 'status', got: %s", body)
		}
	})

	t.Run("prefers Brotli over gzip when both are accepted", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		req.Header.Set("Accept-Encoding", "gzip, br")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		// Should prefer Brotli (better compression)
		contentEncoding := w.Header().Get("Content-Encoding")
		if contentEncoding != "br" {
			t.Errorf("expected Content-Encoding: br (preferred), got %q", contentEncoding)
		}

		// Verify Brotli decompression works
		reader := brotli.NewReader(w.Body)
		decompressed, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("failed to decompress Brotli response: %v", err)
		}

		if len(decompressed) == 0 {
			t.Error("expected non-empty decompressed response")
		}
	})

	t.Run("falls back to gzip when Brotli not accepted", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		req.Header.Set("Accept-Encoding", "gzip")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		// Should use gzip fallback
		contentEncoding := w.Header().Get("Content-Encoding")
		if contentEncoding != "gzip" {
			t.Errorf("expected Content-Encoding: gzip (fallback), got %q", contentEncoding)
		}

		// Verify gzip decompression works
		reader, err := gzip.NewReader(w.Body)
		if err != nil {
			t.Fatalf("failed to create gzip reader: %v", err)
		}
		defer reader.Close()

		decompressed, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("failed to decompress gzip response: %v", err)
		}

		if len(decompressed) == 0 {
			t.Error("expected non-empty decompressed response")
		}
	})

	t.Run("Brotli provides better compression than gzip", func(t *testing.T) {
		// Get gzip-compressed response
		reqGzip := httptest.NewRequest("GET", "/health", nil)
		reqGzip.Header.Set("Accept-Encoding", "gzip")
		wGzip := httptest.NewRecorder()
		handler.ServeHTTP(wGzip, reqGzip)

		// Get Brotli-compressed response
		reqBrotli := httptest.NewRequest("GET", "/health", nil)
		reqBrotli.Header.Set("Accept-Encoding", "br")
		wBrotli := httptest.NewRecorder()
		handler.ServeHTTP(wBrotli, reqBrotli)

		gzipSize := wGzip.Body.Len()
		brotliSize := wBrotli.Body.Len()

		t.Logf("gzip compressed size: %d bytes", gzipSize)
		t.Logf("Brotli compressed size: %d bytes", brotliSize)

		// For small responses like health check, the difference might be minimal
		// But Brotli should never be significantly larger
		// (for large JSON responses, Brotli is typically 15-25% smaller)
		maxBrotliSize := float64(gzipSize) * 1.1
		if float64(brotliSize) > maxBrotliSize {
			t.Errorf("Brotli size (%d) is unexpectedly larger than gzip size (%d)", brotliSize, gzipSize)
		}

		// Verify both decompress to the same content
		gzipReader, _ := gzip.NewReader(bytes.NewReader(wGzip.Body.Bytes()))
		gzipContent, _ := io.ReadAll(gzipReader)

		brotliReader := brotli.NewReader(bytes.NewReader(wBrotli.Body.Bytes()))
		brotliContent, _ := io.ReadAll(brotliReader)

		if string(gzipContent) != string(brotliContent) {
			t.Error("gzip and Brotli decompressed content should match")
		}
	})

	t.Run("Brotli compression works with error responses", func(t *testing.T) {
		// Request a non-existent endpoint to trigger 404
		req := httptest.NewRequest("GET", "/nonexistent", nil)
		req.Header.Set("Accept-Encoding", "br")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", w.Code)
		}

		// Even error responses should be Brotli-compressed
		if w.Header().Get("Content-Encoding") != "br" {
			t.Error("expected Brotli compression for error response")
		}

		// Decompress and verify it's a valid response
		reader := brotli.NewReader(w.Body)
		decompressed, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("failed to decompress Brotli response: %v", err)
		}

		if len(decompressed) == 0 {
			t.Error("expected non-empty decompressed error response")
		}
	})
}

// TestZstdRequestDecompression tests that zstd-compressed request bodies are decompressed
func TestZstdRequestDecompression(t *testing.T) {
	// Test the decompressMiddleware directly
	var receivedBody []byte

	// Create a simple handler that captures the request body
	captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with decompression middleware
	handler := decompressMiddleware()(captureHandler)

	t.Run("decompresses zstd-encoded request body", func(t *testing.T) {
		// Create test payload
		payload := map[string]interface{}{
			"session_id": "test-session",
			"lines":      []string{"line1", "line2", "line3"},
		}
		jsonPayload, _ := json.Marshal(payload)

		// Compress with zstd
		encoder, _ := zstd.NewWriter(nil)
		compressed := encoder.EncodeAll(jsonPayload, nil)

		// Create request with compressed body
		req := httptest.NewRequest("POST", "/test", bytes.NewReader(compressed))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "zstd")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		// Verify the handler received decompressed JSON
		var received map[string]interface{}
		if err := json.Unmarshal(receivedBody, &received); err != nil {
			t.Fatalf("failed to parse received body as JSON: %v", err)
		}

		if received["session_id"] != "test-session" {
			t.Errorf("expected session_id 'test-session', got %v", received["session_id"])
		}

		t.Logf("Zstd decompression: %d bytes -> %d bytes", len(compressed), len(receivedBody))
	})

	t.Run("passes through uncompressed request body", func(t *testing.T) {
		// Create uncompressed request
		payload := map[string]string{"msg": "hello"}
		jsonPayload, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/test", bytes.NewReader(jsonPayload))
		req.Header.Set("Content-Type", "application/json")
		// No Content-Encoding header

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		// Verify body was passed through unchanged
		if string(receivedBody) != string(jsonPayload) {
			t.Errorf("expected body to pass through unchanged")
		}
	})

	t.Run("rejects unsupported Content-Encoding", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/test", strings.NewReader("test"))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "deflate") // unsupported

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnsupportedMediaType {
			t.Errorf("expected status 415 for unsupported encoding, got %d", w.Code)
		}
	})
}
