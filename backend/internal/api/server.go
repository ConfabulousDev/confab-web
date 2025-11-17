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
	"github.com/santaclaude2025/confab/backend/internal/ratelimit"
	"github.com/santaclaude2025/confab/backend/internal/storage"
)

// Server holds dependencies for API handlers
type Server struct {
	db              *db.DB
	storage         *storage.S3Storage
	oauthConfig     auth.OAuthConfig
	globalLimiter   ratelimit.RateLimiter // Global rate limiter for all requests
	authLimiter     ratelimit.RateLimiter // Stricter limiter for auth endpoints
	uploadLimiter   ratelimit.RateLimiter // Stricter limiter for uploads
}

// NewServer creates a new API server
func NewServer(database *db.DB, store *storage.S3Storage, oauthConfig auth.OAuthConfig) *Server {
	return &Server{
		db:          database,
		storage:     store,
		oauthConfig: oauthConfig,
		// Global rate limiter: 100 requests per second, burst of 200
		// Generous limit to allow normal usage while preventing DoS
		globalLimiter: ratelimit.NewInMemoryRateLimiter(100, 200),
		// Auth endpoints: 10 requests per minute = 0.167 req/sec, burst of 5
		// Stricter to prevent brute force attacks on OAuth flow
		authLimiter: ratelimit.NewInMemoryRateLimiter(0.167, 5),
		// Upload endpoints: 20 requests per hour = 0.0056 req/sec, burst of 5
		// Very strict to prevent storage abuse
		uploadLimiter: ratelimit.NewInMemoryRateLimiter(0.0056, 5),
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
	r.Use(securityHeaders)
	r.Use(ratelimit.Middleware(s.globalLimiter)) // Global rate limiting

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

	// Health check (no additional rate limiting needed)
	r.Get("/health", s.handleHealth)
	r.Get("/", s.handleRoot)

	// OAuth routes (public) - Apply stricter auth rate limiting
	r.Get("/auth/github/login", ratelimit.HandlerFunc(s.authLimiter, auth.HandleGitHubLogin(s.oauthConfig)))
	r.Get("/auth/github/callback", ratelimit.HandlerFunc(s.authLimiter, auth.HandleGitHubCallback(s.oauthConfig, s.db)))
	r.Get("/auth/logout", ratelimit.HandlerFunc(s.authLimiter, auth.HandleLogout(s.db)))

	// CLI authorize (requires web session) - Apply auth rate limiting
	r.Get("/auth/cli/authorize", ratelimit.HandlerFunc(s.authLimiter, auth.HandleCLIAuthorize(s.db)))

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
			// Apply upload rate limiting to prevent storage abuse
			r.Use(ratelimit.Middleware(s.uploadLimiter))
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

// securityHeaders adds security headers to all responses
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Content-Security-Policy: Prevents XSS attacks
		// - default-src 'self': Only load resources from same origin
		// - script-src 'self': Only execute scripts from same origin
		// - style-src 'self' 'unsafe-inline': Allow same-origin styles + inline styles (needed for some frameworks)
		// - img-src 'self' data: https:: Allow images from same origin, data URIs, and HTTPS
		// - font-src 'self': Only load fonts from same origin
		// - connect-src 'self': Only connect to same origin for AJAX/WebSocket
		// - frame-ancestors 'none': Prevent embedding in iframes (defense in depth with X-Frame-Options)
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'")

		// X-Frame-Options: Prevents clickjacking attacks
		// DENY: Page cannot be embedded in any iframe
		w.Header().Set("X-Frame-Options", "DENY")

		// X-Content-Type-Options: Prevents MIME sniffing attacks
		// nosniff: Browser must respect Content-Type header, not try to guess
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Strict-Transport-Security (HSTS): Forces HTTPS
		// max-age=31536000: Remember for 1 year
		// includeSubDomains: Apply to all subdomains
		// Only set in production (when cookies are secure)
		if os.Getenv("INSECURE_DEV_MODE") != "true" {
			w.Header().Set("Strict-Transport-Security",
				"max-age=31536000; includeSubDomains")
		}

		// Referrer-Policy: Controls referrer information leakage
		// strict-origin-when-cross-origin: Send full URL for same-origin, only origin for cross-origin
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// X-Permitted-Cross-Domain-Policies: Restricts Flash/PDF cross-domain access
		w.Header().Set("X-Permitted-Cross-Domain-Policies", "none")

		next.ServeHTTP(w, r)
	})
}
