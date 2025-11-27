package http

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/santaclaude2025/confab/pkg/config"
)

func TestClient_CompressionThreshold(t *testing.T) {
	var receivedContentEncoding string
	var receivedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentEncoding = r.Header.Get("Content-Encoding")
		receivedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := NewClient(&config.UploadConfig{
		BackendURL: server.URL,
		APIKey:     "test-key",
	}, 0)

	t.Run("small payload not compressed", func(t *testing.T) {
		// Small payload (< 1KB)
		smallPayload := map[string]string{"msg": "hello"}
		var resp struct{ Ok bool }

		err := client.Post("/test", smallPayload, &resp)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}

		if receivedContentEncoding != "" {
			t.Errorf("expected no Content-Encoding for small payload, got %q", receivedContentEncoding)
		}

		// Verify it's valid JSON (not compressed)
		var decoded map[string]string
		if err := json.Unmarshal(receivedBody, &decoded); err != nil {
			t.Errorf("small payload should be uncompressed JSON: %v", err)
		}
	})

	t.Run("large payload compressed with zstd", func(t *testing.T) {
		// Large payload (> 1KB)
		largePayload := map[string]string{
			"msg": string(make([]byte, 2000)), // 2KB of data
		}
		var resp struct{ Ok bool }

		err := client.Post("/test", largePayload, &resp)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}

		if receivedContentEncoding != "zstd" {
			t.Errorf("expected Content-Encoding 'zstd' for large payload, got %q", receivedContentEncoding)
		}

		// Verify it's valid zstd (decompress and check JSON)
		decoder, _ := zstd.NewReader(nil)
		decompressed, err := decoder.DecodeAll(receivedBody, nil)
		if err != nil {
			t.Fatalf("failed to decompress zstd: %v", err)
		}

		var decoded map[string]string
		if err := json.Unmarshal(decompressed, &decoded); err != nil {
			t.Errorf("decompressed payload should be valid JSON: %v", err)
		}

		// Verify compression actually reduced size
		if len(receivedBody) >= len(decompressed) {
			t.Errorf("compression didn't reduce size: compressed=%d, original=%d",
				len(receivedBody), len(decompressed))
		}

		t.Logf("Compression: %d -> %d bytes (%.1f%% reduction)",
			len(decompressed), len(receivedBody),
			100*(1-float64(len(receivedBody))/float64(len(decompressed))))
	})
}

func TestClient_CompressionRatio(t *testing.T) {
	var receivedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := NewClient(&config.UploadConfig{
		BackendURL: server.URL,
		APIKey:     "test-key",
	}, 0)

	// Simulate realistic transcript chunk (repetitive JSON structures)
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = `{"type":"assistant","message":"This is a typical message with some repeated content and structure"}`
	}
	payload := map[string]interface{}{
		"session_id": "test-session",
		"file_name":  "transcript.jsonl",
		"file_type":  "transcript",
		"first_line": 1,
		"lines":      lines,
	}

	originalJSON, _ := json.Marshal(payload)
	var resp struct{ Ok bool }

	err := client.Post("/test", payload, &resp)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	ratio := float64(len(receivedBody)) / float64(len(originalJSON)) * 100
	t.Logf("Realistic transcript compression: %d -> %d bytes (%.1f%% of original)",
		len(originalJSON), len(receivedBody), ratio)

	// Expect at least 50% reduction for repetitive JSON
	if ratio > 50 {
		t.Errorf("expected at least 50%% compression, got %.1f%%", ratio)
	}
}
