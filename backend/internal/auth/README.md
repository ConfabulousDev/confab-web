# auth

Authentication and authorization for Confab. Supports multiple OAuth providers, password auth, session cookies, API key auth, and device code flow for headless CLI login.

## Files

| File | Role |
|------|------|
| `auth.go` | Core auth primitives: `GenerateAPIKey`, `HashAPIKey` (both delegate to `db.HashToken` — the shared sha256 primitive also used for web-session IDs and device codes, 40hj), API key context key, `RequireAPIKey` middleware, `TryAPIKeyAuth` (non-rejecting), `GetUserID` context extractor, `SetUserIDForTest` helper, `setLogUserID` for FlyLogger integration, OpenTelemetry span enrichment |
| `oauth.go` | Shared OAuth/session core (3vsq): session cookie management, all auth middleware (`RequireSession`, `RequireSessionOrAPIKey`, `OptionalAuth`), `TrySessionAuth`, logout, CLI authorize flow (`HandleCLIAuthorize`, `isLocalhostURL`), user cap enforcement (`CanUserLogin`, `DefaultMaxUsers`), `OAuthConfig` struct + lazy OIDC endpoint discovery method (`getOIDCEndpoints`), and the cross-provider helpers (`generatePKCE`, `setOAuthLoginCookies`, `oauthHTTPClient`, `generateRandomString`, cookie/redirect/email-mismatch helpers, plus the shared callback helpers `validateOAuthCallback` (state+PKCE+code) and `checkUserEligibility` (email-domain + user-cap, returning `errEmailDomainNotPermitted`/`errUserCapReached`) with `redirectUserIneligible` mapping those to the login-page redirect — e7py), plus `redirectInactiveUser` (w8tz) which the three OAuth callbacks use to reject a deactivated account before `CreateWebSession` (login-loop fix). The four login protocols live in their own files. |
| `oauth_github.go` | GitHub OAuth (3vsq): `HandleGitHubLogin`/`HandleGitHubCallback`, `exchangeGitHubCode`, `getGitHubUser`, `getGitHubPrimaryEmail` (separate `/user/emails` call for verified email), `githubUser`/`githubEmail` types. |
| `oauth_google.go` | Google OAuth (3vsq): `HandleGoogleLogin`/`HandleGoogleCallback`, `exchangeGoogleCode`, `getGoogleUser`, `googleUser` type. |
| `oauth_oidc.go` | Generic OIDC (3vsq): `HandleOIDCLogin`/`HandleOIDCCallback`, `DiscoverOIDC`, `exchangeOIDCCode`, `getOIDCUser`, `OIDCEndpoints`/`oidcUser` types + `IsEmailVerified` (handles bool and string). (`getOIDCEndpoints` stays in `oauth.go` as a method on the shared `OAuthConfig`.) |
| `oauth_device.go` | Device code flow (3vsq, RFC 8628 subset): `HandleDeviceCode`/`HandleDeviceToken`/`HandleDevicePage`/`HandleDeviceVerify`, device HTML generators, `generateUserCode` (rejection sampling for an unbiased alphabet), `generateDeviceCode`, device request/response types + expiry consts. `HandleDeviceVerify` applies a per-verifier brute-force lockout (see `device_verify_throttle.go`) (8epk). |
| `device_verify_throttle.go` | `attemptLimiter` — in-memory, per-key failed-attempt lockout (count failures → lock for a window → reset on success/expiry; bounded map). Used by `HandleDeviceVerify`, keyed by the verifier's user ID, mirroring the password-auth lockout without a DB column (8epk). |
| `password.go` | Password authentication: `HandlePasswordLogin`, `HashPassword`/`CheckPassword` (bcrypt), `BootstrapAdmin` for initial admin user creation, `redirectWithError` helper |
| `demo.go` | CF-483 demo identity support. Single env var `DEMO_IDENTITY_EMAIL` activates: `BootstrapDemoIdentity` provisions the demo user and shared session row, `AutoImpersonateIfDemo` is the fallback called by the three session-aware middlewares when real auth fails, `EnforceReadOnly` is the structured-403 middleware chained inside every auth middleware, `DemoSessionCookieID` derives the shared HMAC cookie, `RenderDemoBannerScriptTag` injects the `window.__DEMO_IDENTITY__` global into index.html, `IsDemoLoginEmail` short-circuits password + OAuth callbacks for the demo email, `redirectDemoLoginRejected` is the shared OAuth-callback redirect helper, `WithReadOnly`/`ReadOnlyFromContext` plumb the read-only flag through request context. **Inert when env var is unset.** |

## Key Types

- **`OAuthConfig`** -- central configuration struct holding credentials and feature flags for all auth providers (GitHub, Google, OIDC, password), email domain restrictions, and lazily-discovered OIDC endpoints.
- **`contextKey` / `userIDContextKey`** -- typed context key for storing authenticated user ID. All middleware sets this; handlers read it via `GetUserID(ctx)`.
- **`apiKeyAuthResult` / `sessionAuthResult`** -- internal result types returned by `TryAPIKeyAuth` and `TrySessionAuth`, carrying user ID and email for the authenticated user.

## Key API

### Middleware (mount in route groups via `r.Use(...)`)

| Middleware | Auth Mode | Behavior on Failure |
|------------|-----------|-------------------|
| `RequireAPIKey(db, allowedDomains)` | API key (Bearer token) | 401 Unauthorized |
| `RequireSession(db, config)` | Session cookie | 401 Unauthorized (or demo auto-impersonate when `config.DemoIdentityEmail` is set) |
| `RequireSessionOrAPIKey(db, config)` | Session cookie first, then API key | 401 Unauthorized (or demo auto-impersonate when configured) |
| `OptionalAuth(db, config)` | API key first, then session cookie | Continues without user ID (unless `allowedDomains` is set, then 401; demo auto-impersonate runs first when configured) |

All middleware functions:
1. Validate the credential (API key hash lookup or session cookie lookup)
2. Check user status (reject inactive users)
3. Enforce email domain restrictions if `allowedDomains` is non-empty
4. Set user ID + read-only flag (CF-483) in request context via `context.WithValue`
5. Enrich the request-scoped logger with `user_id`
6. Enrich the OpenTelemetry span with user attributes
7. Set user ID on the FlyLogger response writer for access logging
8. Chain `EnforceReadOnly` (CF-483) internally so mutating requests from a read-only user return the structured 403 — runs AFTER user resolution so the context has the read-only flag

### Handler factories (return `http.HandlerFunc`, registered in `api/server.go`)

| Function | Route | Description |
|----------|-------|-------------|
| `HandleGitHubLogin(config)` | `GET /auth/github/login` | Initiates GitHub OAuth, sets state + PKCE verifier cookies, redirects to GitHub |
| `HandleGitHubCallback(config, db)` | `GET /auth/github/callback` | Exchanges code for token, fetches user, creates/finds user, sets session cookie |
| `HandleGoogleLogin(config)` | `GET /auth/google/login` | Initiates Google OAuth with OpenID Connect scopes |
| `HandleGoogleCallback(config, db)` | `GET /auth/google/callback` | Same flow as GitHub but for Google, requires verified email |
| `HandleOIDCLogin(config)` | `GET /auth/oidc/login` | Initiates generic OIDC flow with lazy endpoint discovery |
| `HandleOIDCCallback(config, db)` | `GET /auth/oidc/callback` | Same flow for generic OIDC, strict email_verified check |
| `HandlePasswordLogin(db, allowedDomains)` | `POST /auth/password/login` | Form-based password login with bcrypt verification and account lockout |
| `HandleLogout(db)` | `GET /auth/logout` | Clears session cookie, deletes DB session, redirects |
| `HandleCLIAuthorize(db)` | `GET /auth/cli/authorize` | Browser-based CLI auth: requires web session, generates API key, redirects to localhost callback |
| `HandleDeviceCode(db, backendURL)` | `POST /auth/device/code` | Initiates device code flow: generates user code (XXXX-XXXX) and device code |
| `HandleDeviceToken(db, allowedDomains)` | `POST /auth/device/token` | Polls device code status, returns API key when authorized |
| `HandleDevicePage(db)` | `GET /auth/device` | Serves HTML device verification form (redirects to login if not authenticated) |
| `HandleDeviceVerify(db, allowedDomains)` | `POST /auth/device/verify` | Processes device code verification form submission |

### Standalone functions

| Function | Description |
|----------|-------------|
| `GenerateAPIKey()` | Returns `(rawKey, hash, error)`. Key format: `cfb_` + 40 chars of base64-encoded random bytes. Hash: SHA-256 hex. |
| `HashAPIKey(rawKey)` | SHA-256 hex hash for validation lookups |
| `GetUserID(ctx)` | Extracts user ID from context. Returns `(int64, bool)`. |
| `TryAPIKeyAuth(r, db)` | Attempts API key auth without rejecting. Returns `*apiKeyAuthResult` or nil. |
| `TrySessionAuth(r, db)` | Attempts session cookie auth without rejecting. Returns `*sessionAuthResult` or nil. |
| `CanUserLogin(ctx, db, email)` | Checks user cap (MAX_USERS env var). Existing users always pass. |
| `HashPassword(password)` | bcrypt hash at cost 12 |
| `CheckPassword(hash, password)` | bcrypt comparison (constant-time) |
| `BootstrapAdmin(ctx, db, allowedDomains)` | Creates initial admin user from ADMIN_BOOTSTRAP_EMAIL/ADMIN_BOOTSTRAP_PASSWORD if no users exist |
| `DiscoverOIDC(issuerURL)` | Fetches `.well-known/openid-configuration`, validates issuer match, returns endpoints |

## How to Extend

### Adding a new OAuth provider

1. **Add config fields to `OAuthConfig`** in `oauth.go` (e.g., `SlackEnabled`, `SlackClientID`, `SlackClientSecret`, `SlackRedirectURL`). The shared struct stays in `oauth.go`; put the new provider's handlers in their own `oauth_slack.go` (mirroring `oauth_github.go`/`oauth_google.go`/`oauth_oidc.go`).

2. **Write `HandleSlackLogin`** (in `oauth_slack.go`) -- generate random state, store in `oauth_state` cookie (HttpOnly, Secure, SameSite=Lax, 5min TTL), store optional `post_login_redirect` and `expected_email` cookies, redirect to provider's authorization URL.

3. **Write `HandleSlackCallback`** -- validate state cookie, exchange code for access token, fetch user info from provider API, enforce email verification, normalize email to lowercase, check `AllowedEmailDomains`, check user cap via `CanUserLogin`, call `authStore.FindOrCreateUserByOAuth`, create web session, set session cookie, call `handlePostLoginRedirect`.

4. **Register routes** in `api/server.go` under the auth section with `ratelimit.HandlerFunc(s.authLimiter, ...)` and `withMaxBody(MaxBodyXS, ...)`.

5. **Add provider to `handleAuthConfig`** in `api/auth_config.go` so the frontend login page shows the new button.

6. **Add tests** in a `*_test.go` file. Use the existing patterns: mock the HTTP client for token exchange and user info endpoints, test state validation, email domain enforcement, and inactive user rejection.

### Adding a new auth mode

If the new mode is neither API key nor session cookie, add a new `Try*Auth` function following the pattern of `TryAPIKeyAuth`/`TrySessionAuth`, then create a new middleware that calls it and sets the context. Update `OptionalAuth` to try the new mode as well.

## Invariants

- **Session cookies are always HttpOnly, SameSite=Lax.** The `Secure` flag is on by default and only disabled when `INSECURE_DEV_MODE=true`.
- **OAuth state is validated via cookie, not database.** The `oauth_state` cookie is set on login initiation and checked on callback. This prevents CSRF attacks on the OAuth flow without database roundtrips.
- **Only verified emails are accepted.** GitHub requires primary+verified email from the `/user/emails` API. Google requires `verified_email=true`. OIDC requires `email_verified=true` (handles both bool and string representations). Missing `email_verified` is treated as unverified.
- **Emails are always normalized to lowercase** before storage or comparison (RFC 5321 convention).
- **API keys are stored as SHA-256 hashes.** The raw key (`cfb_` prefix + 40 chars) is returned to the user exactly once at creation time. Validation hashes the provided key and looks up the hash.
- **Inactive users are rejected by all auth paths.** Both API key and session middleware check `user_status` and reject inactive users. **At login**, deactivated accounts are also rejected before a session is ever minted: the password path returns `ErrInvalidCredentials` (generic "invalid email or password"), and the three OAuth callbacks check `dbUser.Status` after `FindOrCreateUserByOAuth` and redirect to `/login?error=account_inactive` via `redirectInactiveUser` instead of calling `CreateWebSession`. This breaks the app→401→login→app loop a deactivated user would otherwise hit, since re-login no longer silently succeeds (w8tz). The redirect copy is generic ("not active / contact support") and does not confirm deactivation.
- **Email domain restrictions apply to all auth paths.** When `AllowedEmailDomains` is configured, every middleware and OAuth callback enforces it. `OptionalAuth` with domain restrictions requires authentication (no anonymous access).
- **CLI redirect cookies are restricted to `/auth/cli/` paths** to prevent open redirect attacks.
- **Post-login redirects only allow relative paths** (must start with `/`, must not start with `//`) to prevent open redirect attacks.
- **Device codes expire after 5 minutes** and are single-use (deleted after successful token exchange).
- **User codes exclude ambiguous characters** (0, O, I, L, 1) and use `XXXX-XXXX` format for readability.
- **User cap (`MAX_USERS` env var)** is enforced at login time, not registration. Existing users always pass. Checked in all OAuth callbacks and device flow.
- **`ReplaceAPIKey`** is used instead of `CreateAPIKey` for CLI/device flows to prevent unbounded key growth when re-authenticating from the same machine.
- **bcrypt cost is 12** (~250ms on modern hardware), balancing security and performance.
- **OIDC endpoints are lazily discovered** on first request and cached on success only. Failures are not cached so temporary IdP outages don't permanently break OIDC.
- **CF-483 demo identity** is the per-user `users.read_only=true` user named by `DEMO_IDENTITY_EMAIL`. Anonymous web visitors on session-aware routes are auto-impersonated as them via a single shared HMAC-derived cookie (one `web_sessions` row total, 100-year expiry). The demo email is rejected by `HandlePasswordLogin` AND all three OAuth callbacks. `HandleCLIAuthorize` and `HandleDeviceVerify` refuse to mint API keys when the resolved session has `read_only=true` even if the demo cookie is presented (B1). `HandleLogout` clears the demo cookie client-side but skips the DB delete so the shared row survives (B2). `FindOrCreateUserByOAuth` refuses to link new OAuth identities onto a read-only user as a store-layer backstop (D2). When `DEMO_IDENTITY_EMAIL` is unset, every demo-mode predicate short-circuits to today's behavior.

## Design Decisions

- **Per-provider OAuth callbacks** -- GitHub, Google, and OIDC each have their own `Handle*Callback` function. The code notes this duplication is intentional: each provider has different quirks (GitHub needs a separate `/user/emails` call for verified email, Google has `verified_email` as a direct field, OIDC uses `email_verified` which can be string or bool). A generic abstraction would be more complex than the duplication.
- **OAuth PKCE (S256) — verifier in a second cookie (r9zn, audit A1)** -- `setOAuthLoginCookies` generates a PKCE pair via `generatePKCE()` (32 random bytes → base64url verifier; `code_challenge = base64url(SHA256(verifier))`) and stores the verifier in a dedicated HttpOnly+Secure `oauth_verifier` cookie alongside `oauth_state` (same `Path`/`MaxAge=300`/`SameSite=Lax`). Chosen over HMAC-packing the verifier into `oauth_state` or a server-side store: a second short-lived single-use cookie is the simplest browser-bound carrier and needs no key or DB. Each login handler appends `&code_challenge=…&code_challenge_method=S256` to its own auth URL; each callback reads + verifies presence of + clears the verifier cookie (rejecting absent/empty with the same `400 Invalid state parameter` shape) and threads it into the per-provider exchange. Always-on (no config flag): S256 is universally supported by conformant IdPs; a broken IdP surfaces as a token-exchange error. **GitHub caveat:** GitHub accepts the params but its PKCE *enforcement* is not guaranteed — treated as best-effort until a one-time live wrong-verifier test against a real GitHub OAuth App confirms rejection (see `SECURITY_AUDIT_CF425.md` A1).
- **`TryAuth` + `Require` pattern** -- Authentication is split into non-rejecting `Try*Auth` functions and rejecting `Require*` middleware. This allows `OptionalAuth` and `RequireSessionOrAPIKey` to compose auth attempts without code duplication.
- **Session cookies over JWTs** -- Sessions are stored in the database with a random session ID in the cookie. This allows server-side session revocation (logout, admin deactivation) without waiting for token expiry.
- **Device code flow** -- Implements a simplified version of RFC 8628 for CLI authentication on headless/remote machines where the browser runs on a different machine. Uses human-readable codes (XXXX-XXXX) instead of long URLs.
- **Email mismatch detection** -- Share invitation links include `?email=recipient@example.com`. If the user logs in with a different email, the mismatch is propagated through cookies and query parameters so the frontend can show a warning.

## Testing

- **Unit tests** -- `auth_test.go` (API key generation/hashing, context helpers), `oauth_test.go` (OAuth config, CSRF state validation), `oauth_helpers_test.go` (post-login redirect logic, email mismatch), `oauth_helpers_extra_test.go` (`cookieSecure`, `clearCookie`, `handleCLIRedirect` (prefix-only guard), `oauthHTTPClient`, `setOAuthLoginCookies`, `writeDeviceTokenError`), `oauth_callback_test.go` (callback handler patterns), `oauth_extract_test.go` (the extracted `validateOAuthCallback` + `checkUserEligibility` shared helpers — e7py), `password_test.go` (bcrypt, bootstrap admin), `middleware_test.go` (RequireSession, RequireAPIKey, OptionalAuth, RequireSessionOrAPIKey), `localhost_test.go` (localhost URL validation), `oidc_test.go` (OIDC discovery, email_verified parsing), `oidc_http_test.go` (`exchangeOIDCCode` and `getOIDCUser` happy/error paths plus `getOIDCEndpoints` lazy-discovery caching against an `httptest`-backed fake IdP), `device_html_test.go` (device-page and device-result HTML generators, `GetUserIDContextKey` accessor).
- **Integration tests** -- `auth_integration_test.go` uses `testutil.SetupTestEnvironment(t)` for tests requiring a real database (web session creation, API key validation, device code flow).
- Run with `cd backend && DOCKER_HOST=unix:///Users/santaclaude/.orbstack/run/docker.sock go test ./internal/auth/...`
- Use `-short` to skip integration tests during development.

## Dependencies

- **`golang.org/x/crypto/bcrypt`** -- password hashing
- **`go.opentelemetry.io/otel`** -- span enrichment with user attributes
- **`internal/db`** (and `internal/db/dbauth`, `internal/db/user`) -- database access for sessions, API keys, OAuth identities, device codes
- **`internal/logger`** -- structured logging
- **`internal/clientip`** -- client IP for audit logging on failed auth
- **`internal/models`** -- `User`, `OAuthUserInfo`, `UserStatus` types
- **`internal/validation`** -- email validation, domain restriction, API key name validation
