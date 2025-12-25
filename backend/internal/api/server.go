package api

import (
	"encoding/json"
	"errors"
	"io"
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
	"github.com/ConfabulousDev/confab-web/internal/admin"
	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/clientip"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/email"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/ratelimit"
	"github.com/ConfabulousDev/confab-web/internal/storage"
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

// Request body size limits (t-shirt sizes)
const (
	MaxBodyXS = 2 * 1024        // 2KB - GET/DELETE safety buffer
	MaxBodyS  = 16 * 1024       // 16KB - auth tokens, simple metadata
	MaxBodyM  = 128 * 1024      // 128KB - API keys, shares, session updates
	MaxBodyL  = 2 * 1024 * 1024 // 2MB - batch operations
	MaxBodyXL = 16 * 1024 * 1024 // 16MB - sync chunk uploads
)

// withMaxBody wraps a handler with a request body size limit
func withMaxBody(limit int64, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		h(w, r)
	}
}

// withMaxBodyGraced wraps a handler with a request body size limit,
// with a higher limit for a specific graced user.
// TODO(2026-Q2): Remove this temporary grace logic once user 9 is migrated.
func withMaxBodyGraced(normalLimit, gracedLimit, gracedUserID int64, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := normalLimit
		if userID, ok := auth.GetUserID(r.Context()); ok && userID == gracedUserID {
			limit = gracedLimit
		}
		if limit > 0 {
			r.Body = http.MaxBytesReader(w, r.Body, limit)
		}
		h(w, r)
	}
}

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
		// Upload endpoints: 10000 requests per hour = 2.78 req/sec, burst of 2000
		// Keyed by user ID (not IP) to allow backfill of many sessions
		uploadLimiter: ratelimit.NewInMemoryRateLimiter(2.78, 2000),
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

	// Middleware - order matters!
	// 1. Recoverer: catch panics first
	r.Use(middleware.Recoverer)
	// 2. ClientIP: extract real client IP early (replaces chi's RealIP)
	//    Sets r.RemoteAddr and stores IP info in context for rate limiter, logger
	r.Use(clientip.Middleware)
	// 3. Rate limiter: reject abusive requests early, before expensive work
	//    Intentionally before RequestID - rejected requests don't need IDs allocated
	r.Use(ratelimit.Middleware(s.globalLimiter))
	// 4. RequestID: assign unique ID for request tracing (used by FlyLogger)
	r.Use(middleware.RequestID)
	// 5. Redirects and security headers
	r.Use(wwwRedirectMiddleware())
	r.Use(securityHeadersMiddleware())

	// 6. Compression: Brotli (preferred) + gzip (fallback)
	// Brotli provides 15-25% better compression than gzip for JSON
	// Serves Brotli to modern clients (95%+), gzip to legacy clients
	compressor := middleware.NewCompressor(5) // gzip level 5 (baseline)
	compressor.SetEncoder("br", func(w io.Writer, level int) io.Writer {
		// Brotli level 5: balanced speed/compression, ~85% size reduction vs gzip's ~80%
		return brotli.NewWriterLevel(w, 5)
	})
	r.Use(compressor.Handler)

	// 7. FlyLogger: AFTER compressor so it captures uncompressed response bodies
	r.Use(FlyLogger)

	// CORS configuration - CRITICAL SECURITY FIX
	// Note: ALLOWED_ORIGINS is validated at startup in main.go
	allowedOrigins, trustedOrigins := parseAllowedOrigins()
	logger.Info("CORS configured", "allowed_origins", allowedOrigins, "csrf_trusted_origins", trustedOrigins)

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
		csrf.MaxAge(int((auth.SessionDuration + 24*time.Hour).Seconds())), // Session duration + 1 day buffer
		csrf.TrustedOrigins(trustedOrigins),                               // Trust the frontend origin(s)
		csrf.ErrorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.Warn("CSRF validation failed",
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
				"csrf_token", r.Header.Get("X-CSRF-Token"),
				"origin", r.Header.Get("Origin"),
				"referer", r.Header.Get("Referer"),
			)
			respondError(w, http.StatusForbidden, "CSRF token validation failed")
		})),
	)

	// Health check (no additional rate limiting needed)
	r.Get("/health", withMaxBody(MaxBodyXS, s.handleHealth))

	// Public help pages
	r.Get("/help/delete-account", withMaxBody(MaxBodyXS, s.handleDeleteAccountHelp))

	// Root endpoint - only serve API info if not serving static frontend
	staticDir := os.Getenv("STATIC_FILES_DIR")
	if staticDir == "" {
		r.Get("/", withMaxBody(MaxBodyXS, s.handleRoot))
	}

	// OAuth routes (public) - Apply stricter auth rate limiting
	// Login selector (choose provider)
	r.Get("/auth/login", withMaxBody(MaxBodyXS, ratelimit.HandlerFunc(s.authLimiter, auth.HandleLoginSelector())))
	// GitHub
	r.Get("/auth/github/login", withMaxBody(MaxBodyXS, ratelimit.HandlerFunc(s.authLimiter, auth.HandleGitHubLogin(s.oauthConfig))))
	r.Get("/auth/github/callback", withMaxBody(MaxBodyXS, ratelimit.HandlerFunc(s.authLimiter, auth.HandleGitHubCallback(s.oauthConfig, s.db))))
	// Google
	r.Get("/auth/google/login", withMaxBody(MaxBodyXS, ratelimit.HandlerFunc(s.authLimiter, auth.HandleGoogleLogin(s.oauthConfig))))
	r.Get("/auth/google/callback", withMaxBody(MaxBodyXS, ratelimit.HandlerFunc(s.authLimiter, auth.HandleGoogleCallback(s.oauthConfig, s.db))))
	// Logout
	r.Get("/auth/logout", withMaxBody(MaxBodyXS, ratelimit.HandlerFunc(s.authLimiter, auth.HandleLogout(s.db))))

	// CLI authorize (requires web session) - Apply auth rate limiting
	r.Get("/auth/cli/authorize", withMaxBody(MaxBodyXS, ratelimit.HandlerFunc(s.authLimiter, auth.HandleCLIAuthorize(s.db))))

	// Device code flow (for CLI on headless/remote machines)
	backendURL := os.Getenv("BACKEND_URL")
	if backendURL == "" {
		backendURL = "http://localhost:8080" // Default for local dev
	}
	r.Post("/auth/device/code", withMaxBody(MaxBodyS, ratelimit.HandlerFunc(s.authLimiter, auth.HandleDeviceCode(s.db, backendURL))))
	r.Post("/auth/device/token", withMaxBody(MaxBodyS, ratelimit.HandlerFunc(s.authLimiter, auth.HandleDeviceToken(s.db))))
	r.Get("/auth/device", withMaxBody(MaxBodyXS, auth.HandleDevicePage(s.db)))
	r.Post("/auth/device/verify", withMaxBody(MaxBodyS, ratelimit.HandlerFunc(s.authLimiter, auth.HandleDeviceVerify(s.db))))

	// Admin routes (require web session + super admin)
	// Obfuscated path to reduce exposure
	adminHandlers := admin.NewHandlers(s.db, s.storage)
	r.Route("/admin-8b5492d3-a268-4a1b-8b3b-f1dfd34f249b", func(r chi.Router) {
		r.Use(csrfMiddleware)
		r.Use(auth.RequireSession(s.db))
		r.Use(admin.Middleware(s.db))

		r.Get("/users", withMaxBody(MaxBodyXS, adminHandlers.HandleListUsers))
		r.Post("/users/{id}/deactivate", withMaxBody(MaxBodyXS, adminHandlers.HandleDeactivateUser))
		r.Post("/users/{id}/activate", withMaxBody(MaxBodyXS, adminHandlers.HandleActivateUser))
		r.Post("/users/{id}/delete", withMaxBody(MaxBodyXS, adminHandlers.HandleDeleteUser))

		// System share admin page
		r.Get("/system-shares", withMaxBody(MaxBodyXS, adminHandlers.HandleSystemSharePage))
		r.Post("/system-shares", withMaxBody(MaxBodyXS, func(w http.ResponseWriter, r *http.Request) {
			adminHandlers.HandleCreateSystemShareForm(w, r, s.frontendURL)
		}))
	})

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Validate Content-Type for POST/PUT/PATCH requests
		r.Use(validateContentType)

		// Protected routes require API key authentication (for CLI)
		// No CSRF protection for API key routes (CLI doesn't use cookies)
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAPIKey(s.db))

			// API key validation endpoint with rate limiting to prevent abuse
			r.Get("/auth/validate", withMaxBody(MaxBodyXS, ratelimit.HandlerFunc(s.validationLimiter, s.handleValidateAPIKey)))

			// Sync endpoints with user-based rate limiting and zstd decompression
			r.Group(func(r chi.Router) {
				// Rate limit by user ID (not IP) to allow backfill of many sessions
				r.Use(ratelimit.MiddlewareWithKey(s.uploadLimiter, ratelimit.UserKeyFunc(auth.GetUserIDContextKey())))
				// Decompress zstd-compressed request bodies
				r.Use(decompressMiddleware())

				// Incremental sync endpoints (for daemon-based uploads)
				r.Post("/sync/init", withMaxBody(MaxBodyM, s.handleSyncInit))
				// TODO(2026-Q2): Replace withMaxBodyGraced with withMaxBody once user 9 is migrated
				r.Post("/sync/chunk", withMaxBodyGraced(MaxBodyXL, 0, 9, s.handleSyncChunk))
				r.Post("/sync/event", withMaxBody(MaxBodyM, s.handleSyncEvent))
			})

			// Session metadata update (by external_id for CLI convenience)
			r.Patch("/sessions/{external_id}/summary", withMaxBody(MaxBodyM, s.handleUpdateSessionSummary))
		})

		// Protected routes for web dashboard (require web session)
		// CSRF protection applied here to prevent forged requests
		r.Group(func(r chi.Router) {
			r.Use(csrfMiddleware)
			r.Use(auth.RequireSession(s.db))

			// CSRF token endpoint - must be inside CSRF middleware to set cookie
			r.Get("/csrf-token", withMaxBody(MaxBodyXS, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-CSRF-Token", csrf.Token(r))
				respondJSON(w, http.StatusOK, map[string]string{
					"csrf_token": csrf.Token(r),
				})
			}))
			r.Get("/me", withMaxBody(MaxBodyXS, s.handleGetMe))
			// Legacy weekly usage endpoint removed

			// API key management
			r.Post("/keys", withMaxBody(MaxBodyM, HandleCreateAPIKey(s.db)))
			r.Get("/keys", withMaxBody(MaxBodyXS, HandleListAPIKeys(s.db)))
			r.Delete("/keys/{id}", withMaxBody(MaxBodyXS, HandleDeleteAPIKey(s.db)))

			// Session listing (requires auth)
			r.Get("/sessions", withMaxBody(MaxBodyXS, HandleListSessions(s.db)))
			// Session title update (requires auth + ownership)
			r.Patch("/sessions/{id}/title", withMaxBody(MaxBodyS, HandleUpdateSessionTitle(s.db)))

			// Session deletion
			r.Delete("/sessions/{id}", withMaxBody(MaxBodyXS, HandleDeleteSession(s.db, s.storage)))

			// Session sharing
			// Note: FRONTEND_URL is validated at startup in main.go
			frontendURL := os.Getenv("FRONTEND_URL")
			r.Post("/sessions/{id}/share", withMaxBody(MaxBodyM, HandleCreateShare(s.db, frontendURL, s.emailService)))
			r.Get("/sessions/{id}/shares", withMaxBody(MaxBodyXS, HandleListShares(s.db)))
			r.Get("/shares", withMaxBody(MaxBodyXS, HandleListAllUserShares(s.db)))
			r.Delete("/shares/{shareID}", withMaxBody(MaxBodyXS, HandleRevokeShare(s.db)))
		})

		// Session lookup by external_id - requires auth (session cookie OR API key)
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireSessionOrAPIKey(s.db))
			r.Get("/sessions/by-external-id/{external_id}", withMaxBody(MaxBodyXS, HandleLookupSessionByExternalID(s.db)))
			// GitHub links - create (CLI or web)
			r.Post("/sessions/{id}/github-links", withMaxBody(MaxBodyM, HandleCreateGitHubLink(s.db)))
		})

		// Canonical session access (CF-132) - supports optional authentication
		// Works for: owner access, public shares, system shares, recipient shares
		r.Group(func(r chi.Router) {
			r.Use(auth.OptionalAuth(s.db))
			r.Get("/sessions/{id}", withMaxBody(MaxBodyXS, HandleGetSession(s.db)))
			// Canonical shared sync file access endpoint (CF-132)
			// Uses same session access logic as /sessions/{id}
			r.Get("/sessions/{id}/sync/file", withMaxBody(MaxBodyXS, s.handleCanonicalSyncFileRead))
			// GitHub links - list (viewable by anyone with session access)
			r.Get("/sessions/{id}/github-links", withMaxBody(MaxBodyXS, HandleListGitHubLinks(s.db)))
		})

		// GitHub links - delete (owner-only, web session required)
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireSession(s.db))
			r.Delete("/sessions/{id}/github-links/{linkID}", withMaxBody(MaxBodyXS, HandleDeleteGitHubLink(s.db)))
			r.Delete("/sessions/{id}/github-links", withMaxBody(MaxBodyXS, HandleDeleteGitHubLinksByType(s.db)))
		})
	})

	// Static file serving (production mode when frontend is bundled with backend)
	if staticDir != "" {
		logger.Info("serving static files", "dir", staticDir)
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

// handleDeleteAccountHelp serves a help page explaining how to request account deletion
func (s *Server) handleDeleteAccountHelp(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Delete Your Account - Confab</title>
    <style>
        * { box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            margin: 0;
            padding: 2rem;
            background: #fafafa;
            color: #1a1a1a;
        }
        .container {
            background: #fff;
            padding: 2.5rem;
            border-radius: 6px;
            border: 1px solid #e5e5e5;
            box-shadow: 0 1px 3px rgba(0,0,0,0.08);
            max-width: 600px;
            width: 100%;
        }
        h1 {
            margin: 0 0 0.5rem 0;
            font-size: 1.5rem;
            font-weight: 600;
            color: #1a1a1a;
        }
        .subtitle {
            color: #666;
            margin: 0 0 2rem 0;
            font-size: 0.875rem;
        }
        h2 {
            font-size: 1rem;
            font-weight: 600;
            margin: 1.5rem 0 0.75rem 0;
            color: #1a1a1a;
        }
        p {
            color: #444;
            line-height: 1.6;
            margin: 0 0 1rem 0;
            font-size: 0.9375rem;
        }
        ul {
            margin: 0 0 1rem 0;
            padding-left: 1.25rem;
            color: #444;
        }
        li {
            margin-bottom: 0.5rem;
            line-height: 1.5;
            font-size: 0.9375rem;
        }
        .email-link {
            color: #007bff;
            text-decoration: none;
            font-weight: 500;
        }
        .email-link:hover {
            text-decoration: underline;
        }
        .warning {
            background: #fff3cd;
            border: 1px solid #ffc107;
            border-radius: 4px;
            padding: 1rem;
            margin: 1.5rem 0;
        }
        .warning p {
            margin: 0;
            color: #856404;
        }
        .back-link {
            display: inline-block;
            margin-top: 1.5rem;
            color: #666;
            text-decoration: none;
            font-size: 0.875rem;
        }
        .back-link:hover {
            color: #333;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Delete Your Account</h1>
        <p class="subtitle">Request permanent deletion of your Confab account</p>

        <h2>How to Request Account Deletion</h2>
        <p>To delete your account and all associated data, please send an email to:</p>
        <p><a href="mailto:support@confabulous.dev?subject=Account%20Deletion%20Request" class="email-link">support@confabulous.dev</a></p>
        <p>Include the email address associated with your Confab account in your request.</p>

        <h2>What Will Be Deleted</h2>
        <p>When your account is deleted, the following data will be permanently removed:</p>
        <ul>
            <li>Your user account and profile information</li>
            <li>All Claude Code session transcripts you've uploaded</li>
            <li>All API keys associated with your account</li>
            <li>All session shares you've created</li>
            <li>Any web sessions (you'll be logged out everywhere)</li>
        </ul>

        <div class="warning">
            <p><strong>Warning:</strong> Account deletion is permanent and cannot be undone. Make sure to download any session transcripts you want to keep before requesting deletion.</p>
        </div>

        <h2>Processing Time</h2>
        <p>We typically process account deletion requests within 7 business days. You'll receive a confirmation email once your account has been deleted.</p>

        <a href="/" class="back-link">&larr; Back to Confab</a>
    </div>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
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
			serveIndexHTML(w, r, cleanStaticDir)
			return
		}

		// Check if file exists
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			// File doesn't exist, serve index.html for SPA routing
			serveIndexHTML(w, r, cleanStaticDir)
			return
		}

		// Set cache headers based on file type
		// Vite adds content hashes to asset filenames, so they can be cached forever
		// index.html must never be cached so browsers get fresh asset references
		if strings.HasPrefix(requestPath, "/assets/") {
			// Hashed assets - cache for 1 year (immutable)
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else if requestPath == "/" || requestPath == "/index.html" {
			// HTML - never cache
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		}

		// File exists, serve it
		fileServer.ServeHTTP(w, r)
	}
}

// serveIndexHTML serves index.html with no-cache headers for SPA routing
func serveIndexHTML(w http.ResponseWriter, r *http.Request, staticDir string) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
}

// respondJSON writes a JSON response
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
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
					"default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; img-src 'self' data: https:; font-src 'self' https://fonts.gstatic.com; connect-src 'self'; frame-ancestors 'none'")
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

// wwwRedirectMiddleware redirects www.confabulous.dev to confabulous.dev
func wwwRedirectMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := r.Host
			// Strip port if present for comparison
			if colonIdx := strings.LastIndex(host, ":"); colonIdx != -1 {
				host = host[:colonIdx]
			}

			if strings.HasPrefix(host, "www.") {
				// Build redirect URL with apex domain
				newHost := strings.TrimPrefix(host, "www.")
				// Preserve port if original request had one
				if colonIdx := strings.LastIndex(r.Host, ":"); colonIdx != -1 {
					newHost += r.Host[colonIdx:]
				}

				scheme := "https"
				if r.TLS == nil && os.Getenv("INSECURE_DEV_MODE") == "true" {
					scheme = "http"
				}

				newURL := scheme + "://" + newHost + r.URL.RequestURI()
				http.Redirect(w, r, newURL, http.StatusMovedPermanently)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
