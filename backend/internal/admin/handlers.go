package admin

import (
	"time"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/db/dbadmincardinvalidations"
	"github.com/ConfabulousDev/confab-web/internal/db/dbadminsettings"
	"github.com/ConfabulousDev/confab-web/internal/storage"
)

const (
	// DatabaseTimeout is the maximum duration for database operations
	DatabaseTimeout = 5 * time.Second
)

// Handlers holds dependencies for admin handlers
type Handlers struct {
	DB                     *db.DB
	Storage                *storage.S3Storage
	FrontendURL            string
	AllowedEmailDomains    []string
	SharesEnabled          bool
	settingsStore          *dbadminsettings.Store
	analyticsStore         *analytics.Store
	cardInvalidationsStore *dbadmincardinvalidations.Store
}

// NewHandlers creates admin handlers with dependencies
func NewHandlers(database *db.DB, store *storage.S3Storage, frontendURL string, allowedDomains []string, sharesEnabled bool) *Handlers {
	return &Handlers{
		DB:                     database,
		Storage:                store,
		FrontendURL:            frontendURL,
		AllowedEmailDomains:    allowedDomains,
		SharesEnabled:          sharesEnabled,
		settingsStore:          &dbadminsettings.Store{DB: database},
		analyticsStore:         analytics.NewStore(database.Conn()),
		cardInvalidationsStore: &dbadmincardinvalidations.Store{DB: database},
	}
}
