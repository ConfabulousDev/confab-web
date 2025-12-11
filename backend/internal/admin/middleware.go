package admin

import (
	"net/http"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
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
			user, err := database.GetUserByID(r.Context(), userID)
			if err != nil {
				http.Error(w, "Failed to get user", http.StatusInternalServerError)
				return
			}

			// Check if user is a super admin
			if !IsSuperAdmin(user.Email) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
