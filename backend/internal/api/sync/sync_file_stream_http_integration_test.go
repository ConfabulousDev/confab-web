package sync_test

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"

	"github.com/ConfabulousDev/confab-web/internal/api"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// =============================================================================
// GET /api/v1/sessions/{id}/sync/file - streamed response body (x4ry)
//
// The handler streams merged lines to the (compressed) response writer and
// flushes about every 1MB instead of materializing the merged transcript.
// These tests pin the wire bytes across encodings and flush intervals.
// =============================================================================

// readDecodedBody reads the response body, decoding it per Content-Encoding.
// The request must set Accept-Encoding explicitly so Go's transport does not
// decode transparently.
func readDecodedBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	var r io.Reader = resp.Body
	switch enc := resp.Header.Get("Content-Encoding"); enc {
	case "br":
		r = brotli.NewReader(resp.Body)
	case "gzip":
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			t.Fatalf("gzip.NewReader: %v", err)
		}
		defer gz.Close()
		r = gz
	case "":
	default:
		t.Fatalf("unexpected Content-Encoding %q", enc)
	}
	body, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("reading %q body: %v", resp.Header.Get("Content-Encoding"), err)
	}
	return body
}

// uploadLines posts lines 1..len(lines) through the sync API in chunks of
// chunkSize lines and returns the expected merged body.
func uploadLines(t *testing.T, client *testutil.TestClient, sessionID string, lines []string, chunkSize int) string {
	t.Helper()
	for start := 0; start < len(lines); start += chunkSize {
		end := min(start+chunkSize, len(lines))
		resp, err := client.Post("/api/v1/sync/chunk", api.SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: start + 1,
			Lines:     lines[start:end],
		})
		if err != nil {
			t.Fatalf("upload chunk: %v", err)
		}
		testutil.RequireStatus(t, resp, http.StatusOK)
		resp.Body.Close()
	}
	return strings.Join(lines, "\n") + "\n"
}

func numberedLines(n int, pad int) []string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf(`{"line":%d,"pad":"%s"}`, i+1, strings.Repeat(fmt.Sprintf("%07d", i*7919), pad))
	}
	return lines
}

func TestSyncFileRead_Streaming_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	setup := func(t *testing.T, externalID string) (*testutil.TestClient, string) {
		env.CleanDB(t)
		user := testutil.CreateTestUser(t, env, "stream@example.com", "Stream User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)
		ts := setupTestServerWithEnv(t, env)
		return testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken), sessionID
	}

	get := func(t *testing.T, client *testutil.TestClient, path, encoding string) *http.Response {
		t.Helper()
		resp, err := client.RequestWithHeaders(http.MethodGet, path, nil, map[string]string{"Accept-Encoding": encoding})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		t.Cleanup(func() { resp.Body.Close() })
		testutil.RequireStatus(t, resp, http.StatusOK)
		return resp
	}

	t.Run("full read returns the merged bytes exactly", func(t *testing.T) {
		client, sessionID := setup(t, "stream-full")
		want := uploadLines(t, client, sessionID, numberedLines(7, 1), 3)

		resp := get(t, client, "/api/v1/sessions/"+sessionID+"/sync/file?file_name=transcript.jsonl", "identity")
		if got := string(readDecodedBody(t, resp)); got != want {
			t.Errorf("full read body = %q, want %q", got, want)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "text/plain; charset=utf-8" {
			t.Errorf("Content-Type = %q, want text/plain; charset=utf-8", ct)
		}
	})

	t.Run("line_offset returns newline-terminated lines after the offset", func(t *testing.T) {
		client, sessionID := setup(t, "stream-offset")
		lines := numberedLines(7, 1)
		uploadLines(t, client, sessionID, lines, 3)

		resp := get(t, client, "/api/v1/sessions/"+sessionID+"/sync/file?file_name=transcript.jsonl&line_offset=2", "identity")
		want := strings.Join(lines[2:], "\n") + "\n"
		if got := string(readDecodedBody(t, resp)); got != want {
			t.Errorf("line_offset=2 body = %q, want %q", got, want)
		}
	})

	t.Run("multi-megabyte transcript decodes intact for every encoding", func(t *testing.T) {
		client, sessionID := setup(t, "stream-large")
		// ~3.4MB: spans several 1MB flush intervals.
		lines := numberedLines(1200, 400)
		want := uploadLines(t, client, sessionID, lines, 400)
		if len(want) < 3<<20 {
			t.Fatalf("fixture is %d bytes, want more than 3MB", len(want))
		}

		for _, enc := range []string{"br", "gzip", "identity"} {
			t.Run(enc, func(t *testing.T) {
				resp := get(t, client, "/api/v1/sessions/"+sessionID+"/sync/file?file_name=transcript.jsonl", enc)
				wantEnc := enc
				if enc == "identity" {
					wantEnc = ""
				}
				if got := resp.Header.Get("Content-Encoding"); got != wantEnc {
					t.Errorf("Content-Encoding = %q, want %q", got, wantEnc)
				}
				got := readDecodedBody(t, resp)
				if string(got) != want {
					t.Errorf("%s body: got %d bytes, want %d identical bytes", enc, len(got), len(want))
				}
			})
		}

		resp := get(t, client, "/api/v1/sessions/"+sessionID+"/sync/file?file_name=transcript.jsonl&line_offset=1000", "br")
		wantTail := strings.Join(lines[1000:], "\n") + "\n"
		if got := string(readDecodedBody(t, resp)); got != wantTail {
			t.Errorf("br line_offset=1000 body: got %d bytes, want %d identical bytes", len(got), len(wantTail))
		}
	})
}
