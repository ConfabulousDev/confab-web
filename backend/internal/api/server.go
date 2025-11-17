package api

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/santaclaude2025/confab/backend/internal/auth"
	"github.com/santaclaude2025/confab/backend/internal/db"
	"github.com/santaclaude2025/confab/backend/internal/storage"
)

// Server holds dependencies for API handlers
type Server struct {
	db          *db.DB
	storage     *storage.S3Storage
	oauthConfig auth.OAuthConfig
}

// NewServer creates a new API server
func NewServer(database *db.DB, store *storage.S3Storage, oauthConfig auth.OAuthConfig) *Server {
	return &Server{
		db:          database,
		storage:     store,
		oauthConfig: oauthConfig,
	}
}

// SetupRoutes configures HTTP routes
func (s *Server) SetupRoutes() http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)

	// Health check
	r.Get("/health", s.handleHealth)
	r.Get("/", s.handleRoot)

	// OAuth routes (public)
	r.Get("/auth/github/login", auth.HandleGitHubLogin(s.oauthConfig))
	r.Get("/auth/github/callback", auth.HandleGitHubCallback(s.oauthConfig, s.db))
	r.Get("/auth/logout", auth.HandleLogout(s.db))

	// CLI authorize (requires web session)
	r.Get("/auth/cli/authorize", auth.HandleCLIAuthorize(s.db))

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Protected routes require API key authentication (for CLI)
		r.Group(func(r chi.Router) {
			r.Use(auth.Middleware(s.db))
			r.Post("/sessions/save", s.handleSaveSession)
		})

		// Protected routes for web dashboard (require web session)
		r.Group(func(r chi.Router) {
			r.Use(auth.SessionMiddleware(s.db))
			r.Get("/me", s.handleGetMe)

			// API key management
			r.Post("/keys", HandleCreateAPIKey(s.db))
			r.Get("/keys", HandleListAPIKeys(s.db))
			r.Delete("/keys/{id}", HandleDeleteAPIKey(s.db))

			// Session viewing
			r.Get("/sessions", HandleListSessions(s.db))
			r.Get("/sessions/{sessionId}", HandleGetSession(s.db))

			// Session sharing
			frontendURL := os.Getenv("FRONTEND_URL")
			if frontendURL == "" {
				frontendURL = "http://localhost:5173"
			}
			r.Post("/sessions/{sessionId}/share", HandleCreateShare(s.db, frontendURL))
			r.Get("/sessions/{sessionId}/shares", HandleListShares(s.db))
			r.Delete("/shares/{shareToken}", HandleRevokeShare(s.db))
		})

		// Public shared session access (no auth for public, optional auth for private)
		r.Get("/sessions/{sessionId}/shared/{shareToken}", HandleGetSharedSession(s.db))
	})

	return r
}

// handleHealth returns server health status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

// handleRoot returns API info
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"service": "confab-backend",
		"version": "v1",
	})
}

// respondJSON writes a JSON response
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondError writes an error JSON response
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{
		"error": message,
	})
}
