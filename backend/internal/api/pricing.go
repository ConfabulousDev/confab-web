package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ConfabulousDev/confab-web/internal/logger"
)

// handlePricing serves the effective model price table (embedded floor, or a
// freshest valid remote pull). Public and unauthenticated: the frontend reads
// it at bootstrap, and downstream self-hosted backends pull it from
// confabulous.dev. Unlike most responses (respondJSON marks no-store), this is
// cacheable for the refresh interval so an edge/CDN can absorb fan-out.
func (s *Server) handlePricing(w http.ResponseWriter, r *http.Request) {
	doc := s.pricingSource.Effective(r.Context())

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", int(s.pricingSource.RefreshInterval().Seconds())))
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(doc); err != nil {
		logger.Warn("failed to encode pricing response", "error", err)
	}
}
