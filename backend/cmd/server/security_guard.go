package main

import "strings"

// knownDefaultCSRFSecrets are the placeholder CSRF keys shipped in the repo's
// templates (docker-compose.yml and the .env.example files). They are public, so
// an instance running with one outside local dev has effectively no CSRF/session
// protection — the demo-session cookie is also HMAC-derived from this key.
var knownDefaultCSRFSecrets = map[string]bool{
	"local-dev-csrf-secret-change-me-32chars": true, // docker-compose.yml eval default
	"your-csrf-secret-key-at-least-32-chars":  true, // backend/.env.example placeholder
}

// knownDefaultAdminPasswords are the placeholder bootstrap admin passwords from
// the repo templates. An exposed instance still using one has a publicly-known
// admin login.
var knownDefaultAdminPasswords = map[string]bool{
	"localdevpassword":      true, // docker-compose.yml / .env.example eval default
	"change-me-immediately": true, // historical backend/.env.example default
}

// insecureDefaultReason reports why the server should refuse to start, or "" when
// the configuration is safe.
//
// It blocks the one genuinely dangerous combination: production intent together
// with a public default secret from the repo templates. Production intent is
// signalled by INSECURE_DEV_MODE != "true" OR an https:// FRONTEND_URL (so an
// operator who exposes the instance via the Caddy profile but forgets to flip
// INSECURE_DEV_MODE is still caught). In pure local eval (insecure mode + a
// localhost http URL) the shipped defaults are allowed so `docker compose up`
// works with zero configuration — a separate insecure-cookie warning is logged.
func insecureDefaultReason(insecureDevMode bool, frontendURL, csrfSecretKey, adminBootstrapPassword string) string {
	productionIntent := !insecureDevMode || strings.HasPrefix(strings.ToLower(frontendURL), "https://")
	if !productionIntent {
		return ""
	}
	if knownDefaultCSRFSecrets[csrfSecretKey] {
		return "CSRF_SECRET_KEY is a public default from the repo templates"
	}
	if knownDefaultAdminPasswords[adminBootstrapPassword] {
		return "ADMIN_BOOTSTRAP_PASSWORD is a public default from the repo templates"
	}
	return ""
}
