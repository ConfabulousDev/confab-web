package analytics

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ConfabulousDev/confab-web/internal/storage"
)

// SessionProvider is the contract every provider's analytics implementation
// satisfies. Implementations register at init time via RegisterProvider; the
// precompute worker dispatches through ProviderFor.
type SessionProvider interface {
	// Parse downloads and normalizes session data into a provider-specific
	// Rollout. It returns a nil rollout when the session has no transcript yet.
	Parse(ctx context.Context, input ParseInput) (Rollout, error)

	// ComputeCards maps the Rollout onto the canonical ComputeResult shape.
	ComputeCards(ctx context.Context, rollout Rollout) *ComputeResult

	// SearchText returns the Weight C transcript text for the search index.
	SearchText(ctx context.Context, rollout Rollout) string

	// PrepareTranscript builds the XML transcript and id map for smart recap.
	PrepareTranscript(ctx context.Context, rollout Rollout) (xml string, idMap map[int]string, err error)

	// ClearMessageIDs reports whether smart recap items should drop message IDs
	// because the provider lacks stable frontend anchors.
	ClearMessageIDs() bool
}

// ParseInput contains session metadata plus the dependencies providers need to
// load raw transcript data. Provider implementations stay stateless and can be
// registered once at package init time.
type ParseInput struct {
	DB         *sql.DB
	Store      *storage.S3Storage
	SessionID  string
	UserID     int64
	Provider   string
	ExternalID string
}

// Rollout is a provider-specific parsed session representation.
type Rollout interface{}

var providerRegistry = map[string]SessionProvider{}

// RegisterProvider registers p for a canonical provider name and optional
// aliases. Duplicate names panic because registrations happen at init time and
// duplicate ownership is a programmer error.
func RegisterProvider(p SessionProvider, canonical string, aliases ...string) {
	if p == nil {
		panic("analytics: cannot register nil SessionProvider")
	}
	registerProviderName(p, canonical)
	for _, alias := range aliases {
		registerProviderName(p, alias)
	}
}

func registerProviderName(p SessionProvider, name string) {
	if name == "" {
		panic("analytics: cannot register empty provider name")
	}
	if _, exists := providerRegistry[name]; exists {
		panic(fmt.Sprintf("analytics: provider %q already registered", name))
	}
	providerRegistry[name] = p
}

// ProviderFor returns the provider registered for name. Name may be canonical
// or a registered legacy alias.
func ProviderFor(name string) (SessionProvider, error) {
	p, ok := providerRegistry[name]
	if !ok {
		return nil, fmt.Errorf("unsupported provider for analytics: %q", name)
	}
	return p, nil
}
