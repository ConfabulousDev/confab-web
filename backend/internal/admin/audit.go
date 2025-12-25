package admin

import (
	"context"
	"net/http"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/logger"
)

// AdminAction represents the type of admin action performed
type AdminAction string

const (
	ActionUserDeactivate    AdminAction = "user.deactivate"
	ActionUserActivate      AdminAction = "user.activate"
	ActionUserDelete        AdminAction = "user.delete"
	ActionSystemShareCreate AdminAction = "system_share.create"
)

// AuditLog logs an admin action with full context for security audit trail.
// All admin actions should be logged through this function.
func AuditLog(ctx context.Context, database *db.DB, action AdminAction, details map[string]interface{}) {
	log := logger.Ctx(ctx)

	// Extract admin user ID from context (set by RequireSession middleware)
	adminUserID, ok := auth.GetUserID(ctx)
	if !ok {
		// This shouldn't happen since admin routes require session auth,
		// but log anyway with unknown admin
		log.Warn("Admin action without authenticated user",
			"action", string(action),
			"details", details)
		return
	}

	// Get admin email for readable logs
	var adminEmail string
	adminUser, err := database.GetUserByID(ctx, adminUserID)
	if err != nil {
		adminEmail = "unknown"
		log.Warn("Failed to get admin user for audit log", "error", err, "admin_user_id", adminUserID)
	} else {
		adminEmail = adminUser.Email
	}

	// Build log arguments: always include admin identity and action
	logArgs := []interface{}{
		"audit", true, // marker for filtering audit logs
		"action", string(action),
		"admin_user_id", adminUserID,
		"admin_email", adminEmail,
	}

	// Add all details
	for k, v := range details {
		logArgs = append(logArgs, k, v)
	}

	log.Info("ADMIN_AUDIT", logArgs...)
}

// AuditLogFromRequest is a convenience wrapper that extracts context from request
func AuditLogFromRequest(r *http.Request, database *db.DB, action AdminAction, details map[string]interface{}) {
	AuditLog(r.Context(), database, action, details)
}
