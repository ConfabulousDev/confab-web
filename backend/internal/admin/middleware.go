package admin

import (
	"net/http"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/clientip"
	"github.com/ConfabulousDev/confab-web/internal/db"
	dbuser "github.com/ConfabulousDev/confab-web/internal/db/user"
	"github.com/ConfabulousDev/confab-web/internal/logger"
)

// Middleware returns an HTTP middleware that requires super admin authentication.
// It must be used after auth.SessionMiddleware since it expects a user ID in context.
func Middleware(database *db.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user ID from session middleware
			userID, ok := auth.GetUserID(r.Context())
			if !ok {
				http.Error(w, "Not authenticated", http.StatusUnauthorized)
				return
			}

			// Get user email to check admin status
			userStore := &dbuser.Store{DB: database}
			user, err := userStore.GetUserByID(r.Context(), userID)
			if err != nil {
				http.Error(w, "Failed to get user", http.StatusInternalServerError)
				return
			}

			// Admin authorization is the UNION of SUPER_ADMIN_EMAILS (env) and
			// the users.is_admin column, so admins can be managed at runtime
			// without an env edit + restart (5k4v).
			if !IsSuperAdmin(user.Email) && !user.IsAdmin {
				logger.Ctx(r.Context()).Warn("Admin access denied",
					"reason", "not_admin",
					"user_id", userID,
					"email", user.Email,
					"client_ip", clientip.FromRequest(r).Primary,
					"method", r.Method,
					"path", r.URL.Path)
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			logger.Ctx(r.Context()).Info("Admin access granted",
				"user_id", userID,
				"email", user.Email,
				"client_ip", clientip.FromRequest(r).Primary,
				"method", r.Method,
				"path", r.URL.Path)

			next.ServeHTTP(w, r)
		})
	}
}
