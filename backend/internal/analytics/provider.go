package analytics

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/storage"
)

// SessionProvider is the per-provider analytics contract. Implementations
// register at init time via RegisterProvider; callers dispatch through
// ProviderFor.
type SessionProvider interface {
	// Parse loads session data into a provider-specific Rollout, or returns
	// (nil, nil) when the session has no transcript yet.
	Parse(ctx context.Context, input ParseInput) (Rollout, error)
	ComputeCards(ctx context.Context, rollout Rollout) *ComputeResult
	SearchText(ctx context.Context, rollout Rollout) string
	PrepareTranscript(ctx context.Context, rollout Rollout) (xml string, idMap map[int]string, err error)
	// ClearMessageIDs reports whether smart-recap items should drop message
	// IDs (providers without stable frontend anchors).
	ClearMessageIDs() bool
	// DisplayName returns the human-facing label (e.g. "Claude Code",
	// "Codex"); concatenated with " session" / " transcript" by callers.
	DisplayName() string
}

// ParseInput carries the dependencies a provider needs to load raw transcript
// data. Stateless providers can register once at init time.
type ParseInput struct {
	DB         *sql.DB
	Store      *storage.S3Storage
	SessionID  string
	UserID     int64
	Provider   string
	ExternalID string
	// CreatedAt is the session's first_seen timestamp, used for date-aware pricing
	// (e.g. Sonnet 5 introductory rates through Aug 31, 2026). Zero value is safe
	// and routes to the introductory tier (before Sep 1 2026).
	CreatedAt  time.Time
}

// Rollout is a provider-specific parsed session representation.
type Rollout interface{}

var providerRegistry = map[string]SessionProvider{}

// RegisterProvider registers p under a canonical name plus optional aliases.
// Nil providers, empty names, and duplicate names all panic — registrations
// happen at init time.
func RegisterProvider(p SessionProvider, canonical string, aliases ...string) {
	if p == nil {
		panic("analytics: cannot register nil SessionProvider")
	}
	for _, name := range append([]string{canonical}, aliases...) {
		if name == "" {
			panic("analytics: cannot register empty provider name")
		}
		if _, exists := providerRegistry[name]; exists {
			panic(fmt.Sprintf("analytics: provider %q already registered", name))
		}
		providerRegistry[name] = p
	}
}

// ProviderFor returns the provider registered for name (canonical or alias).
func ProviderFor(name string) (SessionProvider, error) {
	p, ok := providerRegistry[name]
	if !ok {
		return nil, fmt.Errorf("unsupported provider for analytics: %q", name)
	}
	return p, nil
}
