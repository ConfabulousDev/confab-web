package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db/dbadmincardinvalidations"
	"github.com/ConfabulousDev/confab-web/internal/httputil"
	"github.com/ConfabulousDev/confab-web/internal/logger"
)

const (
	// invalidateTimeout caps the whole POST /admin/cards/invalidate call, including
	// all chunked batches. Long enough for moderately large windows but bounded so
	// a runaway request can't hold a connection forever.
	invalidateTimeout = 5 * time.Minute

	maxReasonLen = 500
)

// InvalidateCardsRequest is the body of POST /api/v1/admin/cards/invalidate.
// start_date/end_date are ISO-8601 timestamps with explicit timezone (Z or ±hh:mm).
// Missing timezone is rejected with 400.
type InvalidateCardsRequest struct {
	StartDate string   `json:"start_date"`
	EndDate   string   `json:"end_date,omitempty"`
	CardTypes []string `json:"card_types"`
	Reason    string   `json:"reason"`
	DryRun    *bool    `json:"dry_run,omitempty"` // defaults to true when nil
}

// InvalidateCardsResponse is returned by POST /api/v1/admin/cards/invalidate.
// On dry-run, Executed is false.
// On execute success, Executed=true and the Executed-suffix fields match non-suffix counts.
// On partial failure, Error is populated and the Executed-suffix fields report progress.
type InvalidateCardsResponse struct {
	CorrelationID            string         `json:"correlation_id"`
	AffectedSessions         int            `json:"affected_sessions"`
	AffectedCards            map[string]int `json:"affected_cards"`
	Executed                 bool           `json:"executed"`
	CompletedBatches         *int           `json:"completed_batches,omitempty"`
	AffectedSessionsExecuted *int           `json:"affected_sessions_executed,omitempty"`
	Error                    string         `json:"error,omitempty"`
}

// CardInvalidationRow is a single row returned by GET /api/v1/admin/cards/invalidations.
type CardInvalidationRow struct {
	ID            int64    `json:"id"`
	SessionID     string   `json:"session_id"`
	AdminUserID   int64    `json:"admin_user_id"`
	AdminEmail    string   `json:"admin_email,omitempty"`
	InvalidatedAt string   `json:"invalidated_at"`
	CardTypes     []string `json:"card_types"`
	CorrelationID string   `json:"correlation_id"`
	Reason        string   `json:"reason"`
}

// CardInvalidationsListResponse is returned by GET /api/v1/admin/cards/invalidations.
type CardInvalidationsListResponse struct {
	Rows []CardInvalidationRow `json:"rows"`
}

// CardTypesResponse is the GET /admin/cards/types payload — the canonical list of
// invalidatable card table names.
type CardTypesResponse struct {
	CardTypes []string `json:"card_types"`
}

// HandleGetCardTypes serves analytics.AllCardTableNames so the admin UI's
// invalidation checkboxes are sourced from the backend source of truth and can't
// drift from a hand-maintained frontend copy (vd31). Inbound invalidation
// requests are still validated against the same list in parseInvalidateCardsRequest.
func (h *Handlers) HandleGetCardTypes(w http.ResponseWriter, _ *http.Request) {
	httputil.RespondJSON(w, http.StatusOK, CardTypesResponse{CardTypes: analytics.AllCardTableNames})
}

// parseInvalidateCardsRequest validates the request body and produces the
// store-level CountRequest + dry_run + reason. Returns a user-facing error message
// on 400-worthy failures.
func parseInvalidateCardsRequest(req *InvalidateCardsRequest) (dbadmincardinvalidations.CountRequest, bool, string, error) {
	var out dbadmincardinvalidations.CountRequest

	if strings.TrimSpace(req.StartDate) == "" {
		return out, false, "", errors.New("start_date is required")
	}
	start, err := parseStrictTimestamp(req.StartDate)
	if err != nil {
		return out, false, "", err
	}
	out.StartDate = start

	if strings.TrimSpace(req.EndDate) != "" {
		end, err := parseStrictTimestamp(req.EndDate)
		if err != nil {
			return out, false, "", err
		}
		if !end.After(start) {
			return out, false, "", errors.New("end_date must be after start_date")
		}
		out.EndDate = &end
	}

	if len(req.CardTypes) == 0 {
		return out, false, "", errors.New("card_types must be non-empty")
	}
	for _, ct := range req.CardTypes {
		if !analytics.IsKnownCardTableName(ct) {
			return out, false, "", errors.New("unknown card_type: " + ct)
		}
	}
	out.CardTypes = req.CardTypes

	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return out, false, "", errors.New("reason is required")
	}
	if len(reason) > maxReasonLen {
		return out, false, "", errors.New("reason too long (max 500 chars)")
	}

	dryRun := true
	if req.DryRun != nil {
		dryRun = *req.DryRun
	}
	return out, dryRun, reason, nil
}

// parseStrictTimestamp requires an ISO-8601 timestamp with an explicit timezone
// offset (Z or ±hh:mm). `time.RFC3339` rejects bare-naive timestamps.
func parseStrictTimestamp(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, errors.New("timestamp must include timezone (Z or ±hh:mm): " + s)
	}
	return t, nil
}

// HandleInvalidateCards is the handler for POST /api/v1/admin/cards/invalidate.
func (h *Handlers) HandleInvalidateCards(w http.ResponseWriter, r *http.Request) {
	var req InvalidateCardsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	countReq, dryRun, reason, err := parseInvalidateCardsRequest(&req)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), invalidateTimeout)
	defer cancel()

	if dryRun {
		h.respondDryRun(ctx, w, countReq)
		return
	}

	h.respondExecute(ctx, w, r, countReq, reason)
}

// respondDryRun handles the read-only path: count only, no writes, 200 OK.
func (h *Handlers) respondDryRun(ctx context.Context, w http.ResponseWriter, countReq dbadmincardinvalidations.CountRequest) {
	result, err := h.cardInvalidationsStore.CountAffected(ctx, countReq)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to count affected cards")
		return
	}
	httputil.RespondJSON(w, http.StatusOK, InvalidateCardsResponse{
		CorrelationID:    uuid.New().String(),
		AffectedSessions: result.AffectedSessions,
		AffectedCards:    result.AffectedCards,
		Executed:         false,
	})
}

// respondExecute handles the write path: chunked DELETE + audit INSERT, audit log.
// On partial failure, responds 500 with progress so the admin can re-run.
func (h *Handlers) respondExecute(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	countReq dbadmincardinvalidations.CountRequest,
	reason string,
) {
	adminID, ok := auth.GetUserID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	res, execErr := h.cardInvalidationsStore.Execute(ctx, dbadmincardinvalidations.ExecuteRequest{
		CountRequest: countReq,
		AdminUserID:  adminID,
		Reason:       reason,
	})

	if res != nil {
		AuditLogFromRequest(r, h.DB, ActionCardInvalidate, map[string]interface{}{
			"correlation_id":    res.CorrelationID.String(),
			"card_types":        countReq.CardTypes,
			"start_date":        countReq.StartDate.Format(time.RFC3339),
			"end_date":          formatEndDate(countReq.EndDate),
			"affected_sessions": res.Result.AffectedSessions,
			"completed_batches": res.CompletedBatches,
			"reason":            reason,
			"partial":           execErr != nil,
		})
	}

	if execErr != nil {
		logger.Ctx(r.Context()).Error("Admin card invalidation partial failure", "error", execErr)
		if res == nil {
			httputil.RespondError(w, http.StatusInternalServerError, execErr.Error())
			return
		}
		// Partial failure: some batches committed, later one failed. 500 + progress.
		executed := res.Result.AffectedSessions
		completed := res.CompletedBatches
		resp := executeResponse(res)
		resp.CompletedBatches = &completed
		resp.AffectedSessionsExecuted = &executed
		resp.Error = execErr.Error()
		httputil.RespondJSON(w, http.StatusInternalServerError, resp)
		return
	}

	httputil.RespondJSON(w, http.StatusOK, executeResponse(res))
}

// executeResponse builds the success-shaped response from an ExecuteResult.
// Partial-failure callers extend the result with CompletedBatches / AffectedSessionsExecuted / Error.
func executeResponse(res *dbadmincardinvalidations.ExecuteResult) InvalidateCardsResponse {
	return InvalidateCardsResponse{
		CorrelationID:    res.CorrelationID.String(),
		AffectedSessions: res.Result.AffectedSessions,
		AffectedCards:    res.Result.AffectedCards,
		Executed:         true,
	}
}

func formatEndDate(end *time.Time) string {
	if end == nil {
		return ""
	}
	return end.Format(time.RFC3339)
}

// HandleListCardInvalidations is the handler for GET /api/v1/admin/cards/invalidations.
func (h *Handlers) HandleListCardInvalidations(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	if corrID := r.URL.Query().Get("correlation_id"); corrID != "" {
		u, err := uuid.Parse(corrID)
		if err != nil {
			httputil.RespondError(w, http.StatusBadRequest, "Invalid correlation_id")
			return
		}
		rows, err := h.cardInvalidationsStore.ListByCorrelationID(ctx, u)
		if err != nil {
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to list invalidations")
			return
		}
		httputil.RespondJSON(w, http.StatusOK, CardInvalidationsListResponse{Rows: toAPIRows(rows)})
		return
	}

	rows, err := h.cardInvalidationsStore.ListRecent(ctx, 500)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to list invalidations")
		return
	}
	httputil.RespondJSON(w, http.StatusOK, CardInvalidationsListResponse{Rows: toAPIRows(rows)})
}

func toAPIRows(rows []dbadmincardinvalidations.AuditRow) []CardInvalidationRow {
	out := make([]CardInvalidationRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, CardInvalidationRow{
			ID:            r.ID,
			SessionID:     r.SessionID,
			AdminUserID:   r.AdminUserID,
			AdminEmail:    r.AdminEmail,
			InvalidatedAt: r.InvalidatedAt.Format(time.RFC3339),
			CardTypes:     r.CardTypes,
			CorrelationID: r.CorrelationID.String(),
			Reason:        r.Reason,
		})
	}
	return out
}
