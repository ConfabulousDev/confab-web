package api

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/gorilla/csrf"
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

	// CORS configuration - CRITICAL SECURITY FIX
	// Get allowed origins from environment (comma-separated list)
	allowedOrigins := []string{}
	originsEnv := os.Getenv("ALLOWED_ORIGINS")
	if originsEnv != "" {
		// Split by comma and trim whitespace
		for _, origin := range strings.Split(originsEnv, ",") {
			trimmed := strings.TrimSpace(origin)
			if trimmed != "" {
				allowedOrigins = append(allowedOrigins, trimmed)
			}
		}
	}

	// Fallback to FRONTEND_URL for development
	if len(allowedOrigins) == 0 {
		frontendURL := os.Getenv("FRONTEND_URL")
		if frontendURL == "" {
			frontendURL = "http://localhost:5173"
		}
		allowedOrigins = []string{frontendURL}
	}

	r.Use(cors.Handler(cors.Options{
		// AllowedOrigins: Only requests from these domains are allowed
		AllowedOrigins: allowedOrigins,
		// AllowedMethods: HTTP methods that can be used
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		// AllowedHeaders: Headers that can be sent by the client
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		// ExposedHeaders: Headers that can be accessed by the client
		ExposedHeaders: []string{"Link", "X-CSRF-Token"},
		// AllowCredentials: Allow cookies and auth headers
		AllowCredentials: true,
		// MaxAge: How long the browser can cache CORS responses (5 minutes)
		MaxAge: 300,
	}))

	// CSRF protection - CRITICAL SECURITY FIX
	// Protects against Cross-Site Request Forgery attacks
	csrfSecretKey := os.Getenv("CSRF_SECRET_KEY")
	if csrfSecretKey == "" {
		// Generate a warning but use a default for development
		log.Println("WARNING: CSRF_SECRET_KEY not set, using default (INSECURE for production)")
		csrfSecretKey = "development-secret-key-minimum-32-characters-long-change-me"
	}

	// Only enforce CSRF on session-based routes (not API key routes)
	// This is important because CLI uses API keys, not sessions
	csrfMiddleware := csrf.Protect(
		[]byte(csrfSecretKey),
		csrf.Secure(os.Getenv("INSECURE_DEV_MODE") != "true"), // Secure by default (HTTPS-only, set INSECURE_DEV_MODE=true to disable)
		csrf.SameSite(csrf.SameSiteLaxMode),                   // Lax mode for OAuth compatibility
		csrf.Path("/"),
		csrf.ErrorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("CSRF validation failed for %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
			respondError(w, http.StatusForbidden, "CSRF token validation failed")
		})),
	)

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
		// CSRF token endpoint - publicly accessible to get token for subsequent requests
		r.Get("/csrf-token", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-CSRF-Token", csrf.Token(r))
			respondJSON(w, http.StatusOK, map[string]string{
				"csrf_token": csrf.Token(r),
			})
		})

		// Protected routes require API key authentication (for CLI)
		// No CSRF protection for API key routes (CLI doesn't use cookies)
		r.Group(func(r chi.Router) {
			r.Use(auth.Middleware(s.db))
			r.Post("/sessions/save", s.handleSaveSession)
		})

		// Protected routes for web dashboard (require web session)
		// CSRF protection applied here to prevent forged requests
		r.Group(func(r chi.Router) {
			r.Use(csrfMiddleware)
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
