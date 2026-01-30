package admin

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/csrf"
	"golang.org/x/crypto/bcrypt"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/storage"
	"github.com/ConfabulousDev/confab-web/internal/validation"
)

const (
	// DatabaseTimeout is the maximum duration for database operations
	DatabaseTimeout = 5 * time.Second

	// AdminPathPrefix is the admin route prefix
	// This must match the route in server.go
	AdminPathPrefix = "/admin"
)

// Handlers holds dependencies for admin handlers
type Handlers struct {
	DB                  *db.DB
	Storage             *storage.S3Storage
	PasswordAuthEnabled bool
}

// NewHandlers creates admin handlers with dependencies
func NewHandlers(database *db.DB, store *storage.S3Storage, passwordAuthEnabled bool) *Handlers {
	return &Handlers{
		DB:                  database,
		Storage:             store,
		PasswordAuthEnabled: passwordAuthEnabled,
	}
}

// HandleListUsers renders the admin user list page
// NOTE: Inline HTML is intentional for admin pages - these are internal tools that
// rarely change. Keeping them inline avoids external template file dependencies and
// simplifies deployment. This is acceptable for low-churn admin UI pages.
func (h *Handlers) HandleListUsers(w http.ResponseWriter, r *http.Request) {
	log := logger.Ctx(r.Context())

	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	users, err := h.DB.ListAllUsers(ctx)
	if err != nil {
		log.Error("Failed to list users", "error", err)
		http.Error(w, "Failed to list users", http.StatusInternalServerError)
		return
	}

	// Get smart recap stats per user
	recapStats, err := h.DB.ListUserSmartRecapStats(ctx)
	if err != nil {
		log.Error("Failed to list smart recap stats", "error", err)
		http.Error(w, "Failed to load smart recap stats", http.StatusInternalServerError)
		return
	}

	// Build a map for quick lookup by user ID
	recapStatsByUser := make(map[int64]db.UserSmartRecapStats)
	for _, stat := range recapStats {
		recapStatsByUser[stat.UserID] = stat
	}

	// Get totals for summary display
	recapTotals, err := h.DB.GetSmartRecapTotals(ctx)
	if err != nil {
		log.Error("Failed to get smart recap totals", "error", err)
		http.Error(w, "Failed to load smart recap totals", http.StatusInternalServerError)
		return
	}

	// Get CSRF token for forms
	csrfToken := csrf.Token(r)

	// Check for flash message from query params
	message := r.URL.Query().Get("message")
	errorMsg := r.URL.Query().Get("error")

	// Build user rows HTML
	var userRows string
	for _, user := range users {
		statusClass := "status-active"
		statusText := "Active"
		if user.Status == models.UserStatusInactive {
			statusClass = "status-inactive"
			statusText = "Inactive"
		}

		name := "-"
		if user.Name != nil {
			name = html.EscapeString(*user.Name)
		}

		lastAPIKeyUsed := "-"
		if user.LastAPIKeyUsed != nil {
			lastAPIKeyUsed = user.LastAPIKeyUsed.Format("Jan 2, 2006 15:04")
		}

		lastLoggedIn := "-"
		if user.LastLoggedIn != nil {
			lastLoggedIn = user.LastLoggedIn.Format("Jan 2, 2006 15:04")
		}

		// Get recap stats for this user (default to 0 if not found)
		recapStat := recapStatsByUser[user.ID]

		deleteAction := fmt.Sprintf("%s/users/%d/delete", AdminPathPrefix, user.ID)

		userRows += fmt.Sprintf(`
			<tr>
				<td>%d</td>
				<td>%s</td>
				<td>%s</td>
				<td><span class="%s">%s</span></td>
				<td>%d</td>
				<td>%d</td>
				<td>%d</td>
				<td>%s</td>
				<td>%s</td>
				<td>%s</td>
				<td class="actions">
					%s
					<form method="POST" action="%s" class="inline-form" onsubmit="return confirm('PERMANENTLY DELETE user %s and all their data? This cannot be undone!');">
						<input type="hidden" name="gorilla.csrf.Token" value="%s">
						<button type="submit" class="btn btn-danger">Delete</button>
					</form>
				</td>
			</tr>`,
			user.ID,
			html.EscapeString(user.Email),
			name,
			statusClass,
			statusText,
			user.SessionCount,
			recapStat.SessionsWithCache,
			recapStat.ComputationsThisMonth,
			lastAPIKeyUsed,
			lastLoggedIn,
			user.CreatedAt.Format("Jan 2, 2006"),
			h.buildStatusToggleForm(user.User, csrfToken),
			deleteAction,
			html.EscapeString(user.Email),
			csrfToken,
		)
	}

	// Get current month name for display
	currentMonth := time.Now().Format("January 2006")

	// Calculate total sessions across all users
	var totalSessions int
	for _, user := range users {
		totalSessions += user.SessionCount
	}

	// Build flash message HTML
	var flashHTML string
	if message != "" {
		flashHTML = fmt.Sprintf(`<div class="flash flash-success">%s</div>`, html.EscapeString(message))
	}
	if errorMsg != "" {
		flashHTML = fmt.Sprintf(`<div class="flash flash-error">%s</div>`, html.EscapeString(errorMsg))
	}

	// Build "New User" button if password auth is enabled
	var newUserButtonHTML string
	if h.PasswordAuthEnabled {
		newUserButtonHTML = fmt.Sprintf(`<a href="%s/users/new" class="btn btn-primary">New User</a>`, AdminPathPrefix)
	}

	htmlPage := `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Admin - User Management</title>
    <style>
        * { box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            margin: 0;
            padding: 2rem;
            background: #fafafa;
            color: #1a1a1a;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
        }
        h1 {
            margin: 0 0 0.5rem 0;
            font-size: 1.5rem;
            font-weight: 600;
        }
        .subtitle {
            color: #666;
            margin: 0 0 1.5rem 0;
            font-size: 0.875rem;
        }
        .flash {
            padding: 0.75rem 1rem;
            border-radius: 4px;
            margin-bottom: 1rem;
            font-size: 0.875rem;
        }
        .flash-success {
            background: #d4edda;
            color: #155724;
            border: 1px solid #c3e6cb;
        }
        .flash-error {
            background: #f8d7da;
            color: #721c24;
            border: 1px solid #f5c6cb;
        }
        table {
            width: 100%;
            border-collapse: collapse;
            background: #fff;
            border-radius: 6px;
            overflow: hidden;
            box-shadow: 0 1px 3px rgba(0,0,0,0.08);
        }
        th, td {
            padding: 0.75rem 1rem;
            text-align: left;
            border-bottom: 1px solid #eee;
        }
        th {
            background: #f8f8f8;
            font-weight: 600;
            font-size: 0.75rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            color: #666;
        }
        td {
            font-size: 0.875rem;
        }
        tr:last-child td {
            border-bottom: none;
        }
        tr:hover {
            background: #fafafa;
        }
        .status-active {
            color: #28a745;
            font-weight: 500;
        }
        .status-inactive {
            color: #dc3545;
            font-weight: 500;
        }
        .actions {
            display: flex;
            gap: 0.5rem;
            align-items: center;
        }
        .inline-form {
            display: inline;
            margin: 0;
        }
        .btn {
            padding: 0.375rem 0.75rem;
            border: 1px solid #e5e5e5;
            border-radius: 4px;
            font-size: 0.75rem;
            font-weight: 500;
            cursor: pointer;
            transition: all 0.15s ease;
            background: #fff;
        }
        .btn:hover {
            border-color: #ccc;
        }
        .btn-primary {
            background: #007bff;
            color: #fff;
            border-color: #007bff;
        }
        .btn-primary:hover {
            background: #0056b3;
            border-color: #0056b3;
        }
        .btn-warning {
            background: #ffc107;
            color: #212529;
            border-color: #ffc107;
        }
        .btn-warning:hover {
            background: #e0a800;
            border-color: #e0a800;
        }
        .btn-danger {
            background: #dc3545;
            color: #fff;
            border-color: #dc3545;
        }
        .btn-danger:hover {
            background: #c82333;
            border-color: #c82333;
        }
        .user-count {
            font-size: 0.875rem;
            color: #666;
            margin-bottom: 1rem;
        }
        .stats-summary {
            display: flex;
            gap: 1.5rem;
            margin-bottom: 1.5rem;
            flex-wrap: wrap;
        }
        .stat-card {
            background: #fff;
            border-radius: 6px;
            padding: 1rem 1.5rem;
            box-shadow: 0 1px 3px rgba(0,0,0,0.08);
            min-width: 180px;
        }
        .stat-card .label {
            font-size: 0.75rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            color: #666;
            margin-bottom: 0.25rem;
        }
        .stat-card .value {
            font-size: 1.5rem;
            font-weight: 600;
            color: #1a1a1a;
        }
        .header-row {
            display: flex;
            justify-content: space-between;
            align-items: flex-start;
            margin-bottom: 1.5rem;
        }
        .header-row h1 {
            margin-bottom: 0.5rem;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header-row">
            <div>
                <h1>User Management</h1>
                <p class="subtitle">Manage user accounts - deactivate, reactivate, or permanently delete users</p>
            </div>
            ` + newUserButtonHTML + `
        </div>
        ` + flashHTML + `
        <div class="stats-summary">
            <div class="stat-card">
                <div class="label">Total Sessions</div>
                <div class="value">` + fmt.Sprintf("%d", totalSessions) + `</div>
            </div>
            <div class="stat-card">
                <div class="label">Non-Empty Sessions</div>
                <div class="value">` + fmt.Sprintf("%d", recapTotals.TotalNonEmptySessions) + `</div>
            </div>
            <div class="stat-card">
                <div class="label">Sessions with Recap Cache</div>
                <div class="value">` + fmt.Sprintf("%d", recapTotals.TotalSessionsWithCache) + `</div>
            </div>
            <div class="stat-card">
                <div class="label">Recaps This Month (` + currentMonth + `)</div>
                <div class="value">` + fmt.Sprintf("%d", recapTotals.TotalComputationsThisMonth) + `</div>
            </div>
        </div>
        <p class="user-count">` + fmt.Sprintf("%d users", len(users)) + `</p>
        <table>
            <thead>
                <tr>
                    <th>ID</th>
                    <th>Email</th>
                    <th>Name</th>
                    <th>Status</th>
                    <th>Sessions</th>
                    <th>Recap Cache</th>
                    <th>Recaps This Month</th>
                    <th>Last API Key Used</th>
                    <th>Last Logged In</th>
                    <th>Created</th>
                    <th>Actions</th>
                </tr>
            </thead>
            <tbody>
                ` + userRows + `
            </tbody>
        </table>
    </div>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(htmlPage))
}

// buildStatusToggleForm creates the activate/deactivate form based on user status
func (h *Handlers) buildStatusToggleForm(user models.User, csrfToken string) string {
	if user.Status == models.UserStatusActive {
		return fmt.Sprintf(`
			<form method="POST" action="%s/users/%d/deactivate" class="inline-form" onsubmit="return confirm('Deactivate user %s? They will lose all access.');">
				<input type="hidden" name="gorilla.csrf.Token" value="%s">
				<button type="submit" class="btn btn-warning">Deactivate</button>
			</form>`,
			AdminPathPrefix,
			user.ID,
			html.EscapeString(user.Email),
			csrfToken,
		)
	}
	return fmt.Sprintf(`
		<form method="POST" action="%s/users/%d/activate" class="inline-form">
			<input type="hidden" name="gorilla.csrf.Token" value="%s">
			<button type="submit" class="btn btn-primary">Activate</button>
		</form>`,
		AdminPathPrefix,
		user.ID,
		csrfToken,
	)
}

// HandleDeactivateUser sets a user's status to inactive
func (h *Handlers) HandleDeactivateUser(w http.ResponseWriter, r *http.Request) {
	log := logger.Ctx(r.Context())

	userID, err := parseUserID(r)
	if err != nil {
		http.Redirect(w, r, AdminPathPrefix+"/users?error=Invalid+user+ID", http.StatusSeeOther)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	// Get target user email for audit log before modifying
	var targetEmail string
	if targetUser, err := h.DB.GetUserByID(ctx, userID); err == nil {
		targetEmail = targetUser.Email
	}

	if err := h.DB.UpdateUserStatus(ctx, userID, models.UserStatusInactive); err != nil {
		log.Error("Failed to deactivate user", "error", err, "user_id", userID)
		http.Redirect(w, r, AdminPathPrefix+"/users?error=Failed+to+deactivate+user", http.StatusSeeOther)
		return
	}

	// Audit log with full context
	AuditLogFromRequest(r, h.DB, ActionUserDeactivate, map[string]interface{}{
		"target_user_id":    userID,
		"target_user_email": targetEmail,
	})

	http.Redirect(w, r, fmt.Sprintf(AdminPathPrefix+"/users?message=User+%d+deactivated", userID), http.StatusSeeOther)
}

// HandleActivateUser sets a user's status to active
func (h *Handlers) HandleActivateUser(w http.ResponseWriter, r *http.Request) {
	log := logger.Ctx(r.Context())

	userID, err := parseUserID(r)
	if err != nil {
		http.Redirect(w, r, AdminPathPrefix+"/users?error=Invalid+user+ID", http.StatusSeeOther)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	// Get target user email for audit log before modifying
	var targetEmail string
	if targetUser, err := h.DB.GetUserByID(ctx, userID); err == nil {
		targetEmail = targetUser.Email
	}

	if err := h.DB.UpdateUserStatus(ctx, userID, models.UserStatusActive); err != nil {
		log.Error("Failed to activate user", "error", err, "user_id", userID)
		http.Redirect(w, r, AdminPathPrefix+"/users?error=Failed+to+activate+user", http.StatusSeeOther)
		return
	}

	// Audit log with full context
	AuditLogFromRequest(r, h.DB, ActionUserActivate, map[string]interface{}{
		"target_user_id":    userID,
		"target_user_email": targetEmail,
	})

	http.Redirect(w, r, fmt.Sprintf(AdminPathPrefix+"/users?message=User+%d+activated", userID), http.StatusSeeOther)
}

// HandleDeleteUser permanently deletes a user and all their data
func (h *Handlers) HandleDeleteUser(w http.ResponseWriter, r *http.Request) {
	log := logger.Ctx(r.Context())

	userID, err := parseUserID(r)
	if err != nil {
		http.Redirect(w, r, AdminPathPrefix+"/users?error=Invalid+user+ID", http.StatusSeeOther)
		return
	}

	// Use a longer timeout for deletion since it involves S3 operations
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	// Get target user email for audit log BEFORE deletion
	var targetEmail string
	if targetUser, err := h.DB.GetUserByID(ctx, userID); err == nil {
		targetEmail = targetUser.Email
	}

	// Step 1: Get all session IDs for S3 cleanup
	sessionIDs, err := h.DB.GetUserSessionIDs(ctx, userID)
	if err != nil {
		log.Error("Failed to get user sessions for deletion", "error", err, "user_id", userID)
		http.Redirect(w, r, AdminPathPrefix+"/users?error=Failed+to+get+user+sessions", http.StatusSeeOther)
		return
	}

	// Step 2: Delete S3 objects for each session (fail-fast: S3 before DB)
	for _, sessionID := range sessionIDs {
		if err := h.Storage.DeleteAllSessionChunks(ctx, userID, sessionID); err != nil {
			log.Error("Failed to delete S3 objects for session", "error", err, "user_id", userID, "session_id", sessionID)
			http.Redirect(w, r, AdminPathPrefix+"/users?error=Failed+to+delete+storage", http.StatusSeeOther)
			return
		}
	}

	// Step 3: Delete user from database (CASCADE handles related records)
	if err := h.DB.DeleteUser(ctx, userID); err != nil {
		log.Error("Failed to delete user from database", "error", err, "user_id", userID)
		http.Redirect(w, r, AdminPathPrefix+"/users?error=Failed+to+delete+user", http.StatusSeeOther)
		return
	}

	// Audit log with full context - this is a destructive action
	AuditLogFromRequest(r, h.DB, ActionUserDelete, map[string]interface{}{
		"target_user_id":    userID,
		"target_user_email": targetEmail,
		"sessions_deleted":  len(sessionIDs),
	})

	http.Redirect(w, r, fmt.Sprintf(AdminPathPrefix+"/users?message=User+%d+permanently+deleted", userID), http.StatusSeeOther)
}

// parseUserID extracts and validates the user ID from the URL path
func parseUserID(r *http.Request) (int64, error) {
	idStr := chi.URLParam(r, "id")
	return strconv.ParseInt(idStr, 10, 64)
}

// HandleSystemSharePage renders the system share creation page
func (h *Handlers) HandleSystemSharePage(w http.ResponseWriter, r *http.Request) {
	csrfToken := csrf.Token(r)

	// Check for flash messages
	message := r.URL.Query().Get("message")
	errorMsg := r.URL.Query().Get("error")
	shareURL := r.URL.Query().Get("share_url")

	var flashHTML string
	if message != "" {
		flashHTML = fmt.Sprintf(`<div class="flash flash-success">%s</div>`, html.EscapeString(message))
	}
	if errorMsg != "" {
		flashHTML = fmt.Sprintf(`<div class="flash flash-error">%s</div>`, html.EscapeString(errorMsg))
	}

	var shareResultHTML string
	if shareURL != "" {
		shareResultHTML = fmt.Sprintf(`
			<div class="share-result">
				<h3>System Share Created</h3>
				<p>Share URL (accessible to all authenticated users):</p>
				<div class="share-url-container">
					<input type="text" readonly value="%s" id="shareUrl" class="share-url-input">
					<button type="button" onclick="copyShareUrl()" class="btn btn-primary">Copy</button>
				</div>
			</div>`, html.EscapeString(shareURL))
	}

	htmlPage := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Admin - System Shares</title>
    <style>
        * { box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            margin: 0;
            padding: 2rem;
            background: #fafafa;
            color: #1a1a1a;
        }
        .container {
            max-width: 800px;
            margin: 0 auto;
        }
        h1 {
            margin: 0 0 0.5rem 0;
            font-size: 1.5rem;
            font-weight: 600;
        }
        .subtitle {
            color: #666;
            margin: 0 0 1.5rem 0;
            font-size: 0.875rem;
        }
        .flash {
            padding: 0.75rem 1rem;
            border-radius: 4px;
            margin-bottom: 1rem;
            font-size: 0.875rem;
        }
        .flash-success {
            background: #d4edda;
            color: #155724;
            border: 1px solid #c3e6cb;
        }
        .flash-error {
            background: #f8d7da;
            color: #721c24;
            border: 1px solid #f5c6cb;
        }
        .card {
            background: #fff;
            border-radius: 6px;
            padding: 1.5rem;
            box-shadow: 0 1px 3px rgba(0,0,0,0.08);
            margin-bottom: 1.5rem;
        }
        .card h2 {
            margin: 0 0 1rem 0;
            font-size: 1.125rem;
            font-weight: 600;
        }
        .form-group {
            margin-bottom: 1rem;
        }
        label {
            display: block;
            font-weight: 500;
            margin-bottom: 0.5rem;
            font-size: 0.875rem;
        }
        input[type="text"] {
            width: 100%%;
            padding: 0.5rem 0.75rem;
            border: 1px solid #ddd;
            border-radius: 4px;
            font-size: 0.875rem;
        }
        input[type="text"]:focus {
            outline: none;
            border-color: #007bff;
            box-shadow: 0 0 0 2px rgba(0,123,255,0.1);
        }
        .help-text {
            font-size: 0.75rem;
            color: #666;
            margin-top: 0.25rem;
        }
        .btn {
            padding: 0.5rem 1rem;
            border: 1px solid #e5e5e5;
            border-radius: 4px;
            font-size: 0.875rem;
            font-weight: 500;
            cursor: pointer;
            transition: all 0.15s ease;
            background: #fff;
        }
        .btn-primary {
            background: #007bff;
            color: #fff;
            border-color: #007bff;
        }
        .btn-primary:hover {
            background: #0056b3;
            border-color: #0056b3;
        }
        .share-result {
            background: #e8f5e9;
            border: 1px solid #c8e6c9;
            border-radius: 6px;
            padding: 1rem;
            margin-bottom: 1.5rem;
        }
        .share-result h3 {
            margin: 0 0 0.5rem 0;
            font-size: 1rem;
            color: #2e7d32;
        }
        .share-result p {
            margin: 0 0 0.75rem 0;
            font-size: 0.875rem;
        }
        .share-url-container {
            display: flex;
            gap: 0.5rem;
        }
        .share-url-input {
            flex: 1;
            font-family: monospace;
            font-size: 0.8125rem !important;
        }
        .nav-link {
            display: inline-block;
            margin-bottom: 1rem;
            color: #007bff;
            text-decoration: none;
            font-size: 0.875rem;
        }
        .nav-link:hover {
            text-decoration: underline;
        }
    </style>
    <script>
        function copyShareUrl() {
            const input = document.getElementById('shareUrl');
            input.select();
            document.execCommand('copy');
            alert('Copied to clipboard!');
        }
    </script>
</head>
<body>
    <div class="container">
        <a href="%s/users" class="nav-link">← Back to User Management</a>
        <h1>System Shares</h1>
        <p class="subtitle">Create shares accessible to all authenticated users (current and future)</p>
        %s
        %s
        <div class="card">
            <h2>Create System Share</h2>
            <form method="POST" action="%s/system-shares">
                <input type="hidden" name="gorilla.csrf.Token" value="%s">
                <div class="form-group">
                    <label for="sessionId">Session ID (UUID)</label>
                    <input type="text" id="sessionId" name="session_id" placeholder="e.g., 550e8400-e29b-41d4-a716-446655440000" required>
                    <p class="help-text">The internal UUID of the session (not the external_id). Find this in the database or session detail URL.</p>
                </div>
                <button type="submit" class="btn btn-primary">Create System Share</button>
            </form>
        </div>
    </div>
</body>
</html>`,
		AdminPathPrefix,
		flashHTML,
		shareResultHTML,
		AdminPathPrefix,
		csrfToken,
	)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(htmlPage))
}

// HandleCreateSystemShareForm handles form submission for creating system shares
func (h *Handlers) HandleCreateSystemShareForm(w http.ResponseWriter, r *http.Request, frontendURL string) {
	log := logger.Ctx(r.Context())

	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, AdminPathPrefix+"/system-shares?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	sessionID := r.FormValue("session_id")
	if sessionID == "" {
		http.Redirect(w, r, AdminPathPrefix+"/system-shares?error=Session+ID+required", http.StatusSeeOther)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	// Create system share
	share, err := h.DB.CreateSystemShare(ctx, sessionID, nil)
	if err != nil {
		if err == db.ErrSessionNotFound {
			http.Redirect(w, r, AdminPathPrefix+"/system-shares?error=Session+not+found", http.StatusSeeOther)
			return
		}
		log.Error("Failed to create system share", "error", err, "session_id", sessionID)
		http.Redirect(w, r, AdminPathPrefix+"/system-shares?error=Failed+to+create+system+share", http.StatusSeeOther)
		return
	}

	// Canonical URL (CF-132: no token in URL)
	shareURL := frontendURL + "/sessions/" + sessionID

	// Audit log for system share creation
	AuditLogFromRequest(r, h.DB, ActionSystemShareCreate, map[string]interface{}{
		"session_id":  sessionID,
		"share_id":    share.ID,
		"external_id": share.ExternalID,
	})

	// Redirect back with success message and share URL
	redirectURL := fmt.Sprintf("%s/system-shares?message=System+share+created&share_url=%s",
		AdminPathPrefix, url.QueryEscape(shareURL))
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// HandleCreateUserPage renders the user creation page
func (h *Handlers) HandleCreateUserPage(w http.ResponseWriter, r *http.Request) {
	csrfToken := csrf.Token(r)

	// Check for flash messages
	message := r.URL.Query().Get("message")
	errorMsg := r.URL.Query().Get("error")

	var flashHTML string
	if message != "" {
		flashHTML = fmt.Sprintf(`<div class="flash flash-success">%s</div>`, html.EscapeString(message))
	}
	if errorMsg != "" {
		flashHTML = fmt.Sprintf(`<div class="flash flash-error">%s</div>`, html.EscapeString(errorMsg))
	}

	htmlPage := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Admin - Create User</title>
    <style>
        * { box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            margin: 0;
            padding: 2rem;
            background: #fafafa;
            color: #1a1a1a;
        }
        .container {
            max-width: 600px;
            margin: 0 auto;
        }
        h1 {
            margin: 0 0 0.5rem 0;
            font-size: 1.5rem;
            font-weight: 600;
        }
        .subtitle {
            color: #666;
            margin: 0 0 1.5rem 0;
            font-size: 0.875rem;
        }
        .flash {
            padding: 0.75rem 1rem;
            border-radius: 4px;
            margin-bottom: 1rem;
            font-size: 0.875rem;
        }
        .flash-success {
            background: #d4edda;
            color: #155724;
            border: 1px solid #c3e6cb;
        }
        .flash-error {
            background: #f8d7da;
            color: #721c24;
            border: 1px solid #f5c6cb;
        }
        .card {
            background: #fff;
            border-radius: 6px;
            padding: 1.5rem;
            box-shadow: 0 1px 3px rgba(0,0,0,0.08);
            margin-bottom: 1.5rem;
        }
        .form-group {
            margin-bottom: 1rem;
        }
        label {
            display: block;
            font-weight: 500;
            margin-bottom: 0.5rem;
            font-size: 0.875rem;
        }
        input[type="text"], input[type="email"], input[type="password"] {
            width: 100%%;
            padding: 0.5rem 0.75rem;
            border: 1px solid #ddd;
            border-radius: 4px;
            font-size: 0.875rem;
        }
        input:focus {
            outline: none;
            border-color: #007bff;
            box-shadow: 0 0 0 2px rgba(0,123,255,0.1);
        }
        .help-text {
            font-size: 0.75rem;
            color: #666;
            margin-top: 0.25rem;
        }
        .checkbox-group {
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }
        .checkbox-group input {
            width: auto;
        }
        .btn {
            padding: 0.5rem 1rem;
            border: 1px solid #e5e5e5;
            border-radius: 4px;
            font-size: 0.875rem;
            font-weight: 500;
            cursor: pointer;
            transition: all 0.15s ease;
            background: #fff;
        }
        .btn-primary {
            background: #007bff;
            color: #fff;
            border-color: #007bff;
        }
        .btn-primary:hover {
            background: #0056b3;
            border-color: #0056b3;
        }
        .nav-link {
            display: inline-block;
            margin-bottom: 1rem;
            color: #007bff;
            text-decoration: none;
            font-size: 0.875rem;
        }
        .nav-link:hover {
            text-decoration: underline;
        }
    </style>
</head>
<body>
    <div class="container">
        <a href="%s/users" class="nav-link">← Back to User Management</a>
        <h1>Create User</h1>
        <p class="subtitle">Create a new user with password authentication</p>
        %s
        <div class="card">
            <form method="POST" action="%s/users/create">
                <input type="hidden" name="gorilla.csrf.Token" value="%s">
                <div class="form-group">
                    <label for="email">Email</label>
                    <input type="email" id="email" name="email" required>
                </div>
                <div class="form-group">
                    <label for="password">Password</label>
                    <input type="password" id="password" name="password" required minlength="8">
                    <p class="help-text">Minimum 8 characters</p>
                </div>
                <div class="form-group">
                    <div class="checkbox-group">
                        <input type="checkbox" id="is_admin" name="is_admin" value="true">
                        <label for="is_admin" style="margin-bottom: 0;">Admin privileges</label>
                    </div>
                </div>
                <button type="submit" class="btn btn-primary">Create User</button>
            </form>
        </div>
    </div>
</body>
</html>`,
		AdminPathPrefix,
		flashHTML,
		AdminPathPrefix,
		csrfToken,
	)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(htmlPage))
}

// HandleCreateUser creates a new user with password authentication
func (h *Handlers) HandleCreateUser(w http.ResponseWriter, r *http.Request) {
	log := logger.Ctx(r.Context())

	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, AdminPathPrefix+"/users/new?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	password := r.FormValue("password")
	isAdmin := r.FormValue("is_admin") == "true"

	// Validate email
	if !validation.IsValidEmail(email) {
		http.Redirect(w, r, AdminPathPrefix+"/users/new?error=Invalid+email+address", http.StatusSeeOther)
		return
	}

	// Validate password
	if len(password) < 8 {
		http.Redirect(w, r, AdminPathPrefix+"/users/new?error=Password+must+be+at+least+8+characters", http.StatusSeeOther)
		return
	}
	if len(password) > 1024 {
		http.Redirect(w, r, AdminPathPrefix+"/users/new?error=Password+too+long", http.StatusSeeOther)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	// Hash password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		log.Error("Failed to hash password", "error", err)
		http.Redirect(w, r, AdminPathPrefix+"/users/new?error=Failed+to+create+user", http.StatusSeeOther)
		return
	}

	// Create user
	user, err := h.DB.CreatePasswordUser(ctx, email, string(passwordHash), isAdmin)
	if err != nil {
		log.Error("Failed to create user", "error", err, "email", email)
		if strings.Contains(err.Error(), "already exists") {
			http.Redirect(w, r, AdminPathPrefix+"/users/new?error=User+with+this+email+already+exists", http.StatusSeeOther)
		} else {
			http.Redirect(w, r, AdminPathPrefix+"/users/new?error=Failed+to+create+user", http.StatusSeeOther)
		}
		return
	}

	// Audit log
	AuditLogFromRequest(r, h.DB, ActionUserCreate, map[string]interface{}{
		"created_user_id":    user.ID,
		"created_user_email": email,
		"is_admin":           isAdmin,
	})

	http.Redirect(w, r, fmt.Sprintf("%s/users?message=User+%s+created", AdminPathPrefix, url.QueryEscape(email)), http.StatusSeeOther)
}