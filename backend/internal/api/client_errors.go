package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/logger"
)

// Maximum number of errors accepted per request
const maxClientErrors = 50

// clientErrorDetail represents one validation issue (e.g., wrong type at a JSON path)
type clientErrorDetail struct {
	Path     string `json:"path"`
	Message  string `json:"message"`
	Expected string `json:"expected,omitempty"`
	Received string `json:"received,omitempty"`
}

// clientErrorItem groups validation issues for a single transcript line
type clientErrorItem struct {
	Line           int                 `json:"line"`
	MessageType    string              `json:"message_type,omitempty"`
	Details        []clientErrorDetail `json:"details"`
	RawJSONPreview string              `json:"raw_json_preview,omitempty"`
}

// clientErrorContext provides additional context about where the error occurred
type clientErrorContext struct {
	URL       string `json:"url,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
}

// clientErrorReport is the request body for POST /api/v1/client-errors
type clientErrorReport struct {
	Category  string             `json:"category"`
	SessionID string             `json:"session_id,omitempty"`
	Errors    []clientErrorItem  `json:"errors"`
	Context   *clientErrorContext `json:"context,omitempty"`
}

// HandleReportClientErrors accepts client-side error reports and logs them server-side
// for observability. Currently used for transcript validation errors (schema drift detection).
func HandleReportClientErrors() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Ctx(r.Context())

		userID, _ := auth.GetUserID(r.Context())

		var req clientErrorReport
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		if req.Category == "" {
			respondError(w, http.StatusBadRequest, "category is required")
			return
		}

		if len(req.Errors) == 0 {
			respondError(w, http.StatusBadRequest, "errors must not be empty")
			return
		}

		if len(req.Errors) > maxClientErrors {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("too many errors (max %d)", maxClientErrors))
			return
		}

		for i, e := range req.Errors {
			if len(e.Details) == 0 {
				respondError(w, http.StatusBadRequest, fmt.Sprintf("errors[%d].details must not be empty", i))
				return
			}
		}

		// Extract context fields (default to empty strings when context is nil)
		var pageURL, userAgent string
		if req.Context != nil {
			pageURL = req.Context.URL
			userAgent = req.Context.UserAgent
		}

		// Log each error detail individually for easy grep/alerting
		for _, e := range req.Errors {
			rawPreview := e.RawJSONPreview
			if len(rawPreview) > 500 {
				rawPreview = rawPreview[:500]
			}

			for _, d := range e.Details {
				log.Warn("client error detail",
					"category", req.Category,
					"session_id", req.SessionID,
					"line", e.Line,
					"message_type", e.MessageType,
					"error_path", d.Path,
					"error_message", d.Message,
					"expected", d.Expected,
					"received", d.Received,
					"raw_preview", rawPreview,
					"viewer_user_id", userID,
					"url", pageURL,
					"user_agent", userAgent,
				)
			}
		}

		// Log summary
		log.Warn("client errors reported",
			"count", len(req.Errors),
			"category", req.Category,
			"session_id", req.SessionID,
			"viewer_user_id", userID,
		)

		respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
