package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/httputil"
	"github.com/ConfabulousDev/confab-web/internal/logger"
)

// PrecomputeFailureJSON is one session quarantined from a precompute bucket
// after analytics.QuarantineAfterAttempts consecutive worker failures (6kbe).
type PrecomputeFailureJSON struct {
	SessionID     string `json:"session_id"`
	UserID        int64  `json:"user_id"`
	ExternalID    string `json:"external_id"`
	Provider      string `json:"provider"`
	Bucket        string `json:"bucket"`
	Attempts      int    `json:"attempts"`
	LastError     string `json:"last_error"`
	QuarantinedAt string `json:"quarantined_at"`
}

// PrecomputeFailuresResponse is the response for GET /api/v1/admin/precompute-failures.
type PrecomputeFailuresResponse struct {
	Failures []PrecomputeFailureJSON `json:"failures"`
}

// ReleasePrecomputeFailureRequest is the body for POST
// /api/v1/admin/precompute-failures/release.
type ReleasePrecomputeFailureRequest struct {
	SessionID string `json:"session_id"`
	Bucket    string `json:"bucket"`
}

// ReleasePrecomputeFailuresResponse is the response for both release endpoints.
type ReleasePrecomputeFailuresResponse struct {
	Released int `json:"released"`
}

// HandleListPrecomputeFailures lists sessions the worker has quarantined from a
// precompute bucket, newest first.
func (h *Handlers) HandleListPrecomputeFailures(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	rows, err := h.analyticsStore.ListQuarantined(ctx)
	if err != nil {
		logger.Ctx(r.Context()).Error("Failed to list precompute failures", "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to list precompute failures")
		return
	}

	out := make([]PrecomputeFailureJSON, 0, len(rows))
	for _, q := range rows {
		out = append(out, PrecomputeFailureJSON{
			SessionID:     q.SessionID,
			UserID:        q.UserID,
			ExternalID:    q.ExternalID,
			Provider:      q.Provider,
			Bucket:        q.Bucket,
			Attempts:      q.Attempts,
			LastError:     q.LastError,
			QuarantinedAt: q.QuarantinedAt.UTC().Format(time.RFC3339),
		})
	}
	httputil.RespondJSON(w, http.StatusOK, PrecomputeFailuresResponse{Failures: out})
}

// HandleReleasePrecomputeFailure releases one (session, bucket) quarantine so
// the worker retries it on its next cycle. Audited.
func (h *Handlers) HandleReleasePrecomputeFailure(w http.ResponseWriter, r *http.Request) {
	var req ReleasePrecomputeFailureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if _, err := uuid.Parse(req.SessionID); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "session_id must be a UUID")
		return
	}
	if !analytics.IsPrecomputeBucket(req.Bucket) {
		httputil.RespondError(w, http.StatusBadRequest, "Unknown bucket")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	released, err := h.analyticsStore.ReleaseQuarantined(ctx, req.SessionID, req.Bucket)
	if err != nil {
		logger.Ctx(r.Context()).Error("Failed to release precompute failure", "error", err,
			"session_id", req.SessionID, "bucket", req.Bucket)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to release precompute failure")
		return
	}

	AuditLogFromRequest(r, h.DB, ActionPrecomputeRelease, map[string]any{
		"session_id": req.SessionID,
		"bucket":     req.Bucket,
		"released":   released,
	})
	httputil.RespondJSON(w, http.StatusOK, ReleasePrecomputeFailuresResponse{Released: released})
}

// HandleReleaseAllPrecomputeFailures releases every quarantine, for recovery
// after an outage quarantined many sessions at once. Audited.
func (h *Handlers) HandleReleaseAllPrecomputeFailures(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	released, err := h.analyticsStore.ReleaseAllQuarantined(ctx)
	if err != nil {
		logger.Ctx(r.Context()).Error("Failed to release all precompute failures", "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to release precompute failures")
		return
	}

	AuditLogFromRequest(r, h.DB, ActionPrecomputeReleaseAll, map[string]any{
		"released": released,
	})
	httputil.RespondJSON(w, http.StatusOK, ReleasePrecomputeFailuresResponse{Released: released})
}
