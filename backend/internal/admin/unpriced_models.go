package admin

import (
	"context"
	"net/http"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/httputil"
	"github.com/ConfabulousDev/confab-web/internal/logger"
)

// UnpricedModelJSON is one model family seen in stored session data whose family
// is absent from the active pricing table. LastSeen is the most recent tokens_v2
// recompute time across the matching sessions — a proxy for "last seen", not a
// true ingestion time (label it as such in the UI).
type UnpricedModelJSON struct {
	Provider     string `json:"provider"`
	Family       string `json:"family"`
	SessionCount int    `json:"session_count"`
	LastSeen     string `json:"last_seen"`
}

// UnpricedModelsResponse is the response for GET /api/v1/admin/unpriced-models.
type UnpricedModelsResponse struct {
	Models []UnpricedModelJSON `json:"models"`
}

// HandleUnpricedModels serves the pricing-gap surface (axk2): model families
// present in stored session data but missing from the active pricing table, so a
// newly-released unpriced model is visible on the admin page within minutes
// instead of only as a backend WARN someone has to grep for. Read-only; gated by
// the super-admin middleware on the /admin route group.
func (h *Handlers) HandleUnpricedModels(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	rows, err := h.analyticsStore.UnpricedModels(ctx)
	if err != nil {
		logger.Ctx(r.Context()).Error("Failed to compute unpriced models", "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to compute unpriced models")
		return
	}

	out := make([]UnpricedModelJSON, 0, len(rows))
	for _, m := range rows {
		out = append(out, UnpricedModelJSON{
			Provider:     m.Provider,
			Family:       m.Family,
			SessionCount: m.SessionCount,
			LastSeen:     m.LastSeen.Format(time.RFC3339),
		})
	}

	httputil.RespondJSON(w, http.StatusOK, UnpricedModelsResponse{Models: out})
}
