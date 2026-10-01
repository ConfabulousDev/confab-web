package api

import (
	"errors"
	"io"
	"net/http"
)

// syncFileFlushBytes is how much uncompressed output a streamed transcript
// response writes between flushes. The global Brotli encoder buffers roughly
// 3x its input until flushed; flushing every 1MB caps that buffer for
// multi-MB transcripts without changing the compressed size.
const syncFileFlushBytes = 1 << 20

// flushWriter writes through to an http.ResponseWriter and flushes it (and so
// the response compressor) after about every syncFileFlushBytes bytes.
type flushWriter struct {
	w       http.ResponseWriter
	rc      *http.ResponseController
	pending int
}

func newFlushWriter(w http.ResponseWriter) io.Writer {
	return &flushWriter{w: w, rc: http.NewResponseController(w)}
}

func (f *flushWriter) Write(p []byte) (int, error) {
	n, err := f.w.Write(p)
	f.pending += n
	if err != nil || f.pending < syncFileFlushBytes {
		return n, err
	}
	f.pending = 0
	if err := f.rc.Flush(); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return n, err
	}
	return n, nil
}
