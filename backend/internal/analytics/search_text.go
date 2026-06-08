package analytics

import (
	"strings"
	"unicode/utf8"
)

// searchTextBuilder accumulates newline-separated strings up to maxBytes
// (inclusive of separators), truncating the final write on a UTF-8 codepoint
// boundary. Once full, subsequent writes are dropped. Used by both Codex's
// and OpenCode's search-index extractors so the byte-cap + UTF-8-safe
// truncation lives in one place.
type searchTextBuilder struct {
	b        strings.Builder
	maxBytes int
	used     int
	full     bool
}

func newSearchTextBuilder(maxBytes int) *searchTextBuilder {
	return &searchTextBuilder{maxBytes: maxBytes}
}

// Add appends text after a single '\n' separator (skipped before the first
// chunk). If the addition would exceed the cap, it truncates the tail on a
// UTF-8 rune boundary and marks the builder full.
func (s *searchTextBuilder) Add(text string) {
	if s.full || text == "" {
		return
	}
	sep := 0
	if s.b.Len() > 0 {
		sep = 1
	}
	if s.used+sep+len(text) > s.maxBytes {
		remaining := s.maxBytes - s.used
		if remaining > 1 && s.b.Len() > 0 {
			s.b.WriteByte('\n')
			remaining--
		}
		if remaining > 0 && remaining < len(text) {
			for remaining > 0 && !utf8.RuneStart(text[remaining]) {
				remaining--
			}
			s.b.WriteString(text[:remaining])
		} else if remaining > 0 {
			s.b.WriteString(text)
		}
		s.full = true
		return
	}
	if sep > 0 {
		s.b.WriteByte('\n')
		s.used += sep
	}
	s.b.WriteString(text)
	s.used += len(text)
}

func (s *searchTextBuilder) String() string {
	return s.b.String()
}
