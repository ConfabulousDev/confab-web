package api

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// countingFlushRecorder records how often the handler flushed.
type countingFlushRecorder struct {
	*httptest.ResponseRecorder
	flushes int
}

func (r *countingFlushRecorder) Flush() { r.flushes++ }

// noFlushWriter is a ResponseWriter that does not support flushing.
type noFlushWriter struct {
	header http.Header
	body   bytes.Buffer
}

func (w *noFlushWriter) Header() http.Header         { return w.header }
func (w *noFlushWriter) Write(p []byte) (int, error) { return w.body.Write(p) }
func (w *noFlushWriter) WriteHeader(int)             {}

// failingWriter is a ResponseWriter whose writes fail (client went away).
type failingWriter struct{ noFlushWriter }

var errClientGone = errors.New("client gone")

func (w *failingWriter) Write([]byte) (int, error) { return 0, errClientGone }

func TestFlushWriter_FlushesAboutEveryMegabyte(t *testing.T) {
	rec := &countingFlushRecorder{ResponseRecorder: httptest.NewRecorder()}
	fw := newFlushWriter(rec)

	if _, err := fw.Write(make([]byte, syncFileFlushBytes-1)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if rec.flushes != 0 {
		t.Errorf("flushed %d times before %d bytes were written, want 0", rec.flushes, syncFileFlushBytes)
	}

	if _, err := fw.Write([]byte{'x'}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if rec.flushes != 1 {
		t.Errorf("flushed %d times after exactly %d bytes, want 1", rec.flushes, syncFileFlushBytes)
	}

	// The byte count resets after each flush: another 1.5MB in 64KB writes
	// crosses the threshold once more, not on every write.
	piece := make([]byte, 64<<10)
	for range 24 {
		if _, err := fw.Write(piece); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	if rec.flushes != 2 {
		t.Errorf("flushed %d times after %d more bytes, want 2", rec.flushes, 24*len(piece))
	}
	if got, want := rec.Body.Len(), syncFileFlushBytes+24*len(piece); got != want {
		t.Errorf("body length = %d, want %d (every byte passed through)", got, want)
	}
}

func TestFlushWriter_IgnoresWritersWithoutFlush(t *testing.T) {
	w := &noFlushWriter{header: http.Header{}}
	fw := newFlushWriter(w)
	data := make([]byte, 2*syncFileFlushBytes)
	n, err := fw.Write(data)
	if err != nil {
		t.Fatalf("Write to a non-flushing writer returned %v, want nil", err)
	}
	if n != len(data) || w.body.Len() != len(data) {
		t.Errorf("wrote %d bytes (body %d), want %d", n, w.body.Len(), len(data))
	}
}

func TestFlushWriter_PropagatesWriteErrors(t *testing.T) {
	fw := newFlushWriter(&failingWriter{noFlushWriter{header: http.Header{}}})
	if _, err := fw.Write([]byte("line\n")); !errors.Is(err, errClientGone) {
		t.Errorf("Write error = %v, want the underlying writer's error", err)
	}
}
