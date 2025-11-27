package api

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/gorilla/csrf"
	"github.com/santaclaude2025/confab/backend/internal/auth"
	"github.com/santaclaude2025/confab/backend/internal/db"
	"github.com/santaclaude2025/confab/backend/internal/email"
	"github.com/santaclaude2025/confab/backend/internal/ratelimit"
	"github.com/santaclaude2025/confab/backend/internal/storage"
)

// Operation timeout constants
const (
	// DatabaseTimeout is the maximum duration for database operations
	// Prevents slow queries from holding connections indefinitely
	DatabaseTimeout = 5 * time.Second

	// StorageTimeout is the maximum duration for storage operations (uploads/downloads)
	// Longer timeout to accommodate large file transfers (up to 50MB per file)
	StorageTimeout = 30 * time.Second
)

// Server holds dependencies for API handlers
type Server struct {
	db                *db.DB
	storage           *storage.S3Storage
	oauthConfig       auth.OAuthConfig
	emailService      *email.RateLimitedService // Email service for share invitations (may be nil)
	frontendURL       string                    // Base URL for the frontend (for building session URLs)
	globalLimiter     ratelimit.RateLimiter     // Global rate limiter for all requests
	authLimiter       ratelimit.RateLimiter     // Stricter limiter for auth endpoints
	uploadLimiter     ratelimit.RateLimiter     // Stricter limiter for uploads
	validationLimiter ratelimit.RateLimiter     // Moderate limiter for API key validation
}

// NewServer creates a new API server
func NewServer(database *db.DB, store *storage.S3Storage, oauthConfig auth.OAuthConfig, emailService *email.RateLimitedService) *Server {
	return &Server{
		db:           database,
		storage:      store,
		oauthConfig:  oauthConfig,
		emailService: emailService,
		frontendURL:  os.Getenv("FRONTEND_URL"),
		// Global rate limiter: 100 requests per second, burst of 200
		// Generous limit to allow normal usage while preventing DoS
		globalLimiter: ratelimit.NewInMemoryRateLimiter(100, 200),
		// Auth endpoints: 60 requests per minute = 1 req/sec, burst of 30
		// Reasonable limit to prevent brute force while allowing normal dev usage
		authLimiter: ratelimit.NewInMemoryRateLimiter(1, 30),
		// Upload endpoints: 1000 requests per hour = 0.278 req/sec, burst of 200
		// Keyed by user ID (not IP) to allow backfill of many sessions
		uploadLimiter: ratelimit.NewInMemoryRateLimiter(0.278, 200),
		// Validation endpoint: 30 requests per minute = 0.5 req/sec, burst of 10
		// Moderate limit for CLI validation checks while preventing abuse
		validationLimiter: ratelimit.NewInMemoryRateLimiter(0.5, 10),
	}
}

// parseAllowedOrigins parses ALLOWED_ORIGINS env var into CORS and CSRF formats
// Returns (corsOrigins, csrfTrustedOrigins)
// CORS needs full URLs like "https://example.com"
// CSRF needs just host:port like "example.com:443"
func parseAllowedOrigins() ([]string, []string) {
	var allowedOrigins []string
	var trustedOrigins []string

	originsEnv := os.Getenv("ALLOWED_ORIGINS")
	for _, origin := range strings.Split(originsEnv, ",") {
		if trimmed := strings.TrimSpace(origin); trimmed != "" {
			allowedOrigins = append(allowedOrigins, trimmed)
			// Extract host for CSRF TrustedOrigins (expects "host:port" not "http://host:port")
			host := strings.TrimPrefix(strings.TrimPrefix(trimmed, "https://"), "http://")
			trustedOrigins = append(trustedOrigins, host)
		}
	}

	return allowedOrigins, trustedOrigins
}

// SetupRoutes configures HTTP routes
func (s *Server) SetupRoutes() http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(securityHeadersMiddleware())

	// Compression middleware: Brotli (preferred) + gzip (fallback)
	// Brotli provides 15-25% better compression than gzip for JSON
	// Serves Brotli to modern clients (95%+), gzip to legacy clients
	compressor := middleware.NewCompressor(5) // gzip level 5 (baseline)
	compressor.SetEncoder("br", func(w io.Writer, level int) io.Writer {
		// Brotli level 5: balanced speed/compression, ~85% size reduction vs gzip's ~80%
		return brotli.NewWriterLevel(w, 5)
	})
	r.Use(compressor.Handler)

	r.Use(ratelimit.Middleware(s.globalLimiter)) // Global rate limiting

	// CORS configuration - CRITICAL SECURITY FIX
	// Note: ALLOWED_ORIGINS is validated at startup in main.go
	allowedOrigins, trustedOrigins := parseAllowedOrigins()
	log.Printf("CORS allowed origins: %v, CSRF trusted origins: %v", allowedOrigins, trustedOrigins)

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
	// Note: CSRF_SECRET_KEY is validated at startup in main.go
	csrfSecretKey := os.Getenv("CSRF_SECRET_KEY")

	// Only enforce CSRF on session-based routes (not API key routes)
	// This is important because CLI uses API keys, not sessions
	csrfMiddleware := csrf.Protect(
		[]byte(csrfSecretKey),
		csrf.Secure(os.Getenv("INSECURE_DEV_MODE") != "true"), // Secure by default (HTTPS-only, set INSECURE_DEV_MODE=true to disable)
		csrf.SameSite(csrf.SameSiteLaxMode),                   // Lax mode for OAuth compatibility
		csrf.Path("/"),
		csrf.TrustedOrigins(trustedOrigins), // Trust the frontend origin(s)
		csrf.ErrorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Debug: log all relevant info
			log.Printf("CSRF validation failed for %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
			log.Printf("  X-CSRF-Token header: %s", r.Header.Get("X-CSRF-Token"))
			log.Printf("  Cookie header: %s", r.Header.Get("Cookie"))
			log.Printf("  Origin: %s", r.Header.Get("Origin"))
			log.Printf("  Referer: %s", r.Header.Get("Referer"))
			respondError(w, http.StatusForbidden, "CSRF token validation failed")
		})),
	)

	// Health check (no additional rate limiting needed)
	r.Get("/health", s.handleHealth)

	// Root endpoint - only serve API info if not serving static frontend
	staticDir := os.Getenv("STATIC_FILES_DIR")
	if staticDir == "" {
		r.Get("/", s.handleRoot)
	}

	// OAuth routes (public) - Apply stricter auth rate limiting
	// Login selector (choose provider)
	r.Get("/auth/login", ratelimit.HandlerFunc(s.authLimiter, auth.HandleLoginSelector()))
	// GitHub
	r.Get("/auth/github/login", ratelimit.HandlerFunc(s.authLimiter, auth.HandleGitHubLogin(s.oauthConfig)))
	r.Get("/auth/github/callback", ratelimit.HandlerFunc(s.authLimiter, auth.HandleGitHubCallback(s.oauthConfig, s.db)))
	// Google
	r.Get("/auth/google/login", ratelimit.HandlerFunc(s.authLimiter, auth.HandleGoogleLogin(s.oauthConfig)))
	r.Get("/auth/google/callback", ratelimit.HandlerFunc(s.authLimiter, auth.HandleGoogleCallback(s.oauthConfig, s.db)))
	// Logout
	r.Get("/auth/logout", ratelimit.HandlerFunc(s.authLimiter, auth.HandleLogout(s.db)))

	// CLI authorize (requires web session) - Apply auth rate limiting
	r.Get("/auth/cli/authorize", ratelimit.HandlerFunc(s.authLimiter, auth.HandleCLIAuthorize(s.db)))

	// Device code flow (for CLI on headless/remote machines)
	backendURL := os.Getenv("BACKEND_URL")
	if backendURL == "" {
		backendURL = "http://localhost:8080" // Default for local dev
	}
	r.Post("/auth/device/code", ratelimit.HandlerFunc(s.authLimiter, auth.HandleDeviceCode(s.db, backendURL)))
	r.Post("/auth/device/token", ratelimit.HandlerFunc(s.authLimiter, auth.HandleDeviceToken(s.db)))
	r.Get("/auth/device", auth.HandleDevicePage(s.db))
	r.Post("/auth/device/verify", ratelimit.HandlerFunc(s.authLimiter, auth.HandleDeviceVerify(s.db)))

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Validate Content-Type for POST/PUT/PATCH requests
		r.Use(validateContentType)

		// Protected routes require API key authentication (for CLI)
		// No CSRF protection for API key routes (CLI doesn't use cookies)
		r.Group(func(r chi.Router) {
			r.Use(auth.Middleware(s.db))

			// API key validation endpoint with rate limiting to prevent abuse
			r.Get("/auth/validate", ratelimit.HandlerFunc(s.validationLimiter, s.handleValidateAPIKey))

			// Check which sessions exist (for backfill deduplication)
			r.Post("/sessions/check", HandleCheckSessions(s))

			// Upload endpoints with user-based rate limiting
			r.Group(func(r chi.Router) {
				// Rate limit by user ID (not IP) to allow backfill of many sessions
				r.Use(ratelimit.MiddlewareWithKey(s.uploadLimiter, ratelimit.UserKeyFunc(auth.GetUserIDContextKey())))
				// Decompress zstd-compressed request bodies
				r.Use(decompressMiddleware())
				r.Post("/sessions/save", s.handleSaveSession)
			})

			// Incremental sync endpoints (for daemon-based uploads)
			r.Post("/sync/init", s.handleSyncInit)
			r.Post("/sync/chunk", s.handleSyncChunk)
			r.Get("/sync/file", s.handleSyncFileRead)
		})

		// Protected routes for web dashboard (require web session)
		// CSRF protection applied here to prevent forged requests
		r.Group(func(r chi.Router) {
			r.Use(csrfMiddleware)
			r.Use(auth.SessionMiddleware(s.db))

			// CSRF token endpoint - must be inside CSRF middleware to set cookie
			r.Get("/csrf-token", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-CSRF-Token", csrf.Token(r))
				respondJSON(w, http.StatusOK, map[string]string{
					"csrf_token": csrf.Token(r),
				})
			})
			r.Get("/me", s.handleGetMe)
			r.Get("/usage/weekly", s.handleGetWeeklyUsage)

			// API key management
			r.Post("/keys", HandleCreateAPIKey(s.db))
			r.Get("/keys", HandleListAPIKeys(s.db))
			r.Delete("/keys/{id}", HandleDeleteAPIKey(s.db))

			// Session viewing
			r.Get("/sessions", HandleListSessions(s.db))
			r.Get("/sessions/{id}", HandleGetSession(s.db))

			// Session deletion
			r.Delete("/sessions/{id}", HandleDeleteSessionOrRun(s.db, s.storage))
			r.Delete("/runs/{runId}", HandleDeleteRun(s.db, s.storage))

			// File content
			r.Get("/runs/{runId}/files/{fileId}/content", HandleGetFileContent(s.db, s.storage))

			// Session sharing
			// Note: FRONTEND_URL is validated at startup in main.go
			frontendURL := os.Getenv("FRONTEND_URL")
			r.Post("/sessions/{id}/share", HandleCreateShare(s.db, frontendURL, s.emailService))
			r.Get("/sessions/{id}/shares", HandleListShares(s.db))
			r.Get("/shares", HandleListAllUserShares(s.db))
			r.Delete("/shares/{shareToken}", HandleRevokeShare(s.db))
		})

		// Public shared session access (no auth for public, optional auth for private)
		r.Get("/sessions/{id}/shared/{shareToken}", HandleGetSharedSession(s.db))
		r.Get("/sessions/{id}/shared/{shareToken}/files/{fileId}/content", HandleGetSharedFileContent(s.db, s.storage))
	})

	// Static file serving (production mode when frontend is bundled with backend)
	if staticDir != "" {
		log.Printf("Serving static files from: %s", staticDir)
		// Serve static assets (JS, CSS, images, etc.)
		r.Get("/*", s.serveSPA(staticDir))
	}

	return r
}

// handleHealth returns server health status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

// handleValidateAPIKey validates the API key and returns user info
func (s *Server) handleValidateAPIKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get authenticated user ID (already validated by auth.Middleware)
	userID, ok := auth.GetUserID(ctx)
	if !ok {
		respondError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	// Get user details
	user, err := s.db.GetUserByID(ctx, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get user details")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"valid":   true,
		"user_id": user.ID,
		"email":   user.Email,
		"name":    user.Name,
	})
}

// handleRoot returns API info
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"service": "confab-backend",
		"version": "v1",
	})
}

// serveSPA serves the static frontend files with SPA fallback
func (s *Server) serveSPA(staticDir string) http.HandlerFunc {
	fileServer := http.FileServer(http.Dir(staticDir))
	// Clean staticDir once during initialization for security check
	cleanStaticDir := filepath.Clean(staticDir)

	return func(w http.ResponseWriter, r *http.Request) {
		// Clean the requested path to prevent path traversal attacks
		// This resolves .. sequences and removes redundant separators
		requestPath := filepath.Clean(r.URL.Path)

		// Join with staticDir to get the full path
		fullPath := filepath.Join(cleanStaticDir, requestPath)

		// CRITICAL SECURITY CHECK: Ensure resolved path is still under staticDir
		// This prevents path traversal attacks like /../../../etc/passwd
		if !strings.HasPrefix(fullPath, cleanStaticDir) {
			// Path escapes static directory, serve index.html instead
			http.ServeFile(w, r, filepath.Join(cleanStaticDir, "index.html"))
			return
		}

		// Check if file exists
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			// File doesn't exist, serve index.html for SPA routing
			http.ServeFile(w, r, filepath.Join(cleanStaticDir, "index.html"))
			return
		}

		// File exists, serve it
		fileServer.ServeHTTP(w, r)
	}
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

// respondStorageError returns an appropriate error response based on the storage error type
func respondStorageError(w http.ResponseWriter, err error, defaultMsg string) {
	if errors.Is(err, storage.ErrObjectNotFound) {
		respondError(w, http.StatusNotFound, "File not found in storage")
		return
	}
	if errors.Is(err, storage.ErrAccessDenied) {
		respondError(w, http.StatusForbidden, "Storage access denied")
		return
	}
	if errors.Is(err, storage.ErrNetworkError) {
		respondError(w, http.StatusServiceUnavailable, "Storage temporarily unavailable")
		return
	}
	// Generic error
	respondError(w, http.StatusInternalServerError, defaultMsg)
}

// securityHeadersMiddleware creates middleware that adds appropriate security headers
func securityHeadersMiddleware() func(http.Handler) http.Handler {
	staticDir := os.Getenv("STATIC_FILES_DIR")
	servingStatic := staticDir != ""

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Content-Security-Policy: Prevents XSS attacks
			// Different policies for static frontend vs API-only mode
			if servingStatic {
				// Relaxed CSP for SPA frontend: allows inline scripts needed for SPA bootstrap
				// - script-src 'self' 'unsafe-inline': Allow inline scripts (React apps may need this)
				// - style-src 'self' 'unsafe-inline': Allow inline styles
				w.Header().Set("Content-Security-Policy",
					"default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'")
			} else {
				// Strict CSP for API-only mode
				// - script-src 'self': Only execute scripts from same origin
				w.Header().Set("Content-Security-Policy",
					"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'")
			}

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
}
