package api

import (
	"io"
	"net/http"
	"strings"

	"github.com/klauspost/compress/zstd"
)

// maxDecompressedBody bounds the size of any decompressed request body produced
// by decompressMiddleware. Per-route withMaxBody wrappers (MaxBodyXL = 16MB for
// sync chunks) also enforce a limit, but binding decompressed output here
// prevents a zstd bomb (small compressed payload, huge decompressed output)
// from being read into memory if a future route forgets the per-route wrapper.
const maxDecompressedBody = MaxBodyXL

// decompressMiddleware handles decompression of request bodies based on Content-Encoding header
// Supports: zstd
// Falls back to uncompressed if no Content-Encoding header (backward compatible)
func decompressMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			encoding := r.Header.Get("Content-Encoding")

			// No compression, pass through
			if encoding == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Handle zstd compression
			if strings.EqualFold(encoding, "zstd") {
				decoder, err := zstd.NewReader(r.Body)
				if err != nil {
					respondError(w, http.StatusBadRequest, "Failed to create zstd decoder")
					return
				}
				defer decoder.Close()

				// Bound decompressed output (defense against zstd bombs).
				r.Body = http.MaxBytesReader(w, io.NopCloser(decoder), maxDecompressedBody)

				// Remove Content-Encoding header so downstream handlers see uncompressed data
				r.Header.Del("Content-Encoding")

				// Update Content-Length to unknown since decompressed size differs
				r.Header.Del("Content-Length")
				r.ContentLength = -1

				next.ServeHTTP(w, r)
				return
			}

			// Unsupported encoding
			respondError(w, http.StatusUnsupportedMediaType,
				"Unsupported Content-Encoding: "+encoding)
		})
	}
}
