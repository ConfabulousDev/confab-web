package admin

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/csrf"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/storage"
)

const (
	// DatabaseTimeout is the maximum duration for database operations
	DatabaseTimeout = 5 * time.Second
)

// Handlers holds dependencies for admin handlers
type Handlers struct {
	DB      *db.DB
	Storage *storage.S3Storage
}

// NewHandlers creates admin handlers with dependencies
func NewHandlers(database *db.DB, store *storage.S3Storage) *Handlers {
	return &Handlers{
		DB:      database,
		Storage: store,
	}
}

// HandleListUsers renders the admin user list page
func (h *Handlers) HandleListUsers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	users, err := h.DB.ListAllUsers(ctx)
	if err != nil {
		logger.Error("Failed to list users", "error", err)
		http.Error(w, "Failed to list users", http.StatusInternalServerError)
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

		userRows += fmt.Sprintf(`
			<tr>
				<td>%d</td>
				<td>%s</td>
				<td>%s</td>
				<td><span class="%s">%s</span></td>
				<td>%s</td>
				<td class="actions">
					%s
					<form method="POST" action="/admin/users/%d/delete" class="inline-form" onsubmit="return confirm('PERMANENTLY DELETE user %s and all their data? This cannot be undone!');">
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
			user.CreatedAt.Format("Jan 2, 2006"),
			h.buildStatusToggleForm(user, csrfToken),
			user.ID,
			html.EscapeString(user.Email),
			csrfToken,
		)
	}

	// Build flash message HTML
	var flashHTML string
	if message != "" {
		flashHTML = fmt.Sprintf(`<div class="flash flash-success">%s</div>`, html.EscapeString(message))
	}
	if errorMsg != "" {
		flashHTML = fmt.Sprintf(`<div class="flash flash-error">%s</div>`, html.EscapeString(errorMsg))
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
    </style>
</head>
<body>
    <div class="container">
        <h1>User Management</h1>
        <p class="subtitle">Manage user accounts - deactivate, reactivate, or permanently delete users</p>
        ` + flashHTML + `
        <p class="user-count">` + fmt.Sprintf("%d users", len(users)) + `</p>
        <table>
            <thead>
                <tr>
                    <th>ID</th>
                    <th>Email</th>
                    <th>Name</th>
                    <th>Status</th>
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
			<form method="POST" action="/admin/users/%d/deactivate" class="inline-form" onsubmit="return confirm('Deactivate user %s? They will lose all access.');">
				<input type="hidden" name="gorilla.csrf.Token" value="%s">
				<button type="submit" class="btn btn-warning">Deactivate</button>
			</form>`,
			user.ID,
			html.EscapeString(user.Email),
			csrfToken,
		)
	}
	return fmt.Sprintf(`
		<form method="POST" action="/admin/users/%d/activate" class="inline-form">
			<input type="hidden" name="gorilla.csrf.Token" value="%s">
			<button type="submit" class="btn btn-primary">Activate</button>
		</form>`,
		user.ID,
		csrfToken,
	)
}

// HandleDeactivateUser sets a user's status to inactive
func (h *Handlers) HandleDeactivateUser(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserID(r)
	if err != nil {
		http.Redirect(w, r, "/admin/users?error=Invalid+user+ID", http.StatusSeeOther)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	if err := h.DB.UpdateUserStatus(ctx, userID, models.UserStatusInactive); err != nil {
		logger.Error("Failed to deactivate user", "error", err, "user_id", userID)
		http.Redirect(w, r, "/admin/users?error=Failed+to+deactivate+user", http.StatusSeeOther)
		return
	}

	logger.Info("User deactivated", "user_id", userID)
	http.Redirect(w, r, fmt.Sprintf("/admin/users?message=User+%d+deactivated", userID), http.StatusSeeOther)
}

// HandleActivateUser sets a user's status to active
func (h *Handlers) HandleActivateUser(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserID(r)
	if err != nil {
		http.Redirect(w, r, "/admin/users?error=Invalid+user+ID", http.StatusSeeOther)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	if err := h.DB.UpdateUserStatus(ctx, userID, models.UserStatusActive); err != nil {
		logger.Error("Failed to activate user", "error", err, "user_id", userID)
		http.Redirect(w, r, "/admin/users?error=Failed+to+activate+user", http.StatusSeeOther)
		return
	}

	logger.Info("User activated", "user_id", userID)
	http.Redirect(w, r, fmt.Sprintf("/admin/users?message=User+%d+activated", userID), http.StatusSeeOther)
}

// HandleDeleteUser permanently deletes a user and all their data
func (h *Handlers) HandleDeleteUser(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserID(r)
	if err != nil {
		http.Redirect(w, r, "/admin/users?error=Invalid+user+ID", http.StatusSeeOther)
		return
	}

	// Use a longer timeout for deletion since it involves S3 operations
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	// Step 1: Get all session IDs for S3 cleanup
	sessionIDs, err := h.DB.GetUserSessionIDs(ctx, userID)
	if err != nil {
		logger.Error("Failed to get user sessions for deletion", "error", err, "user_id", userID)
		http.Redirect(w, r, "/admin/users?error=Failed+to+get+user+sessions", http.StatusSeeOther)
		return
	}

	// Step 2: Delete S3 objects for each session (fail-fast: S3 before DB)
	for _, sessionID := range sessionIDs {
		if err := h.Storage.DeleteAllSessionChunks(ctx, userID, sessionID); err != nil {
			logger.Error("Failed to delete S3 objects for session", "error", err, "user_id", userID, "session_id", sessionID)
			http.Redirect(w, r, "/admin/users?error=Failed+to+delete+storage", http.StatusSeeOther)
			return
		}
	}

	// Step 3: Delete user from database (CASCADE handles related records)
	if err := h.DB.DeleteUser(ctx, userID); err != nil {
		logger.Error("Failed to delete user from database", "error", err, "user_id", userID)
		http.Redirect(w, r, "/admin/users?error=Failed+to+delete+user", http.StatusSeeOther)
		return
	}

	logger.Info("User permanently deleted", "user_id", userID, "sessions_deleted", len(sessionIDs))
	http.Redirect(w, r, fmt.Sprintf("/admin/users?message=User+%d+permanently+deleted", userID), http.StatusSeeOther)
}

// parseUserID extracts and validates the user ID from the URL path
func parseUserID(r *http.Request) (int64, error) {
	idStr := chi.URLParam(r, "id")
	return strconv.ParseInt(idStr, 10, 64)
}
