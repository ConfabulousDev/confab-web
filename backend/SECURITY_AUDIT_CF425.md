# Security Audit — CF-425

**Date:** 2026-05-16
**Branch / PR:** `cf-425-security-audit-backend`
**Ticket:** [CF-425](https://linear.app/confabulous/issue/CF-425)
**Auditor:** Claude Code (multi-agent sweep), reviewed by Jackie Tung

## Threat model

The Confab backend is OSS, self-hosted by operators on their own infrastructure
(Fly.io, bare VPS, k8s, etc.). There is no Confab-operated SaaS. The audit
assumes:

- **Outsiders:** untrusted attackers on the open internet who can reach the
  public HTTP surface, scan share IDs, abuse rate limits, and inject crafted
  inputs.
- **Operators:** high-trust insiders who control deployment, but who can
  misconfigure environment variables, leave default credentials, or
  accidentally enable dev modes.
- **Logged-in users:** authenticated insiders who may attempt IDOR, privilege
  escalation, or abuse shareable surfaces.

Out of scope: physical host compromise, malicious operator deliberately
exfiltrating their own DB, frontend XSS rendering, CLI binary supply chain.

## Methodology

Eight parallel sub-audits, each scoped to one attack surface, plus a ninth pass
on supply-chain (govulncheck, go.mod, Dockerfile):

1. Authentication & authorization (OAuth, API keys, sessions, password auth)
2. API endpoints & input validation
3. Database & SQL injection
4. Storage (S3/MinIO) & file handling
5. Admin / operator surface
6. Middleware, CORS, CSRF, rate limits
7. Secrets, config, logging, errors
8. Sharing & public access
9. Dependencies & supply chain

Each sub-audit returned a markdown report classified Critical / High / Medium /
Low / Info with `file:line` citations. This document is the consolidated
deliverable.

## Overall posture

**Strong.** The codebase shows deliberate security engineering: parameterized
SQL throughout, layered access checks (`db/access` canonical model + handler
guards), bounded request bodies, CSRF via Fetch metadata, rate limits keyed by
user *and* IP, comprehensive security headers, bcrypt-12 for passwords,
SHA-256-hashed API keys, HttpOnly+Secure+SameSite=Lax cookies.

The findings below are *predominantly* hardening rather than active
vulnerabilities. The biggest near-term risks are **operator footguns**
(misconfiguration leading to silent insecure states) and **defense-in-depth
gaps** (e.g., zstd decompression bounded only at the route level, OAuth
without PKCE).

## Remediation summary

**Landed in this PR** (mechanical, defense-in-depth):

- Bump Go toolchain to 1.25.10 + `golang.org/x/net` to v0.54.0 + OTel SDK to
  v1.43.0 — clears 16 of 18 production-reachable `govulncheck` findings
  (including OTel SDK arbitrary-code-execution via PATH and HTTP/2 infinite
  loop). Remaining 2 (Moby AuthZ/plugin-priv via testcontainers) are test-only,
  no upstream fix.
- Reject `ALLOWED_ORIGINS=*` at startup AND in `parseAllowedOrigins` — paired
  with `AllowCredentials=true` this would have been an exploitable misconfig.
- Log a `WARN` at startup when `INSECURE_DEV_MODE=true` so operators can't
  silently leave dev mode on.
- Bound zstd-decompressed request bodies at `MaxBodyXL` (16 MB) inside
  `decompressMiddleware` — previously only the per-route `withMaxBody` wrapper
  enforced this.
- Set `Content-Disposition: attachment` (with sanitized filename) on the
  authenticated file-download endpoint so a user-uploaded transcript whose
  bytes look like HTML/JS can't render inline.
- Fix the `SECURITY.md` ↔ code mismatch: `SESSION_SECRET` is documented as
  required but never read by code; remove it and document that session IDs are
  cryptographically random per-session.
- Document the MinIO demo-credentials risk, `INSECURE_DEV_MODE` runtime
  behavior, `ENABLE_PPROF` env var, and wildcard-origin rule in `SECURITY.md`.

**Filed as follow-up tickets** (need design / substantial implementation):

| Ticket | Title | Severity |
|--------|-------|----------|
| [CF-426](https://linear.app/confabulous/issue/CF-426) | OAuth PKCE for GitHub / Google / OIDC | High |
| [CF-427](https://linear.app/confabulous/issue/CF-427) | Bootstrap admin race condition on concurrent first-user creation | High |
| [CF-428](https://linear.app/confabulous/issue/CF-428) | OAuth → password account auto-linking allows takeover | High |
| [CF-429](https://linear.app/confabulous/issue/CF-429) | Share creation quota + upfront email rate-limit check | High |
| [CF-430](https://linear.app/confabulous/issue/CF-430) | Admin surface hardening (confirmation steps, audit-log tamper evidence, SUPER_ADMIN_EMAILS fallback, last-admin protection) | Medium |
| [CF-431](https://linear.app/confabulous/issue/CF-431) | Web session idle timeout (sliding window) | Medium |
| [CF-432](https://linear.app/confabulous/issue/CF-432) | `TRUSTED_PROXY` allowlist + rate-limiter bucket cap | Medium |
| [CF-433](https://linear.app/confabulous/issue/CF-433) | Share listing hygiene + invitation URL cleanup | Low |
| [CF-434](https://linear.app/confabulous/issue/CF-434) | Build hardening — pin Docker base images, `-trimpath`, govulncheck in CI | Low |

## Detailed findings

### Authentication & authorization

| # | Severity | Location | Finding |
|---|----------|----------|---------|
| A1 | High | `internal/auth/oauth.go:1129` | OAuth code-grant lacks PKCE for all three IdPs. MitM with code interception is mitigated only by HTTPS. Self-hosted operators behind a reverse proxy with weak TLS termination are exposed. **→ [CF-426](https://linear.app/confabulous/issue/CF-426)** |
| A2 | High | `internal/db/dbauth/oauth.go:68-95` | OAuth identity is auto-linked to an existing password-auth user when emails match. An attacker who can register the victim's email on GitHub/Google could hijack the existing local account. Requires email verification on the IdP side (GitHub does enforce this) so risk is moderate. **→ [CF-428](https://linear.app/confabulous/issue/CF-428)** |
| A3 | High | `internal/auth/password.go:169-231` (`BootstrapAdmin`) | `CountUsers() == 0` check + `CreatePasswordUser` is not atomic. Concurrent first-user creation (two pods on startup) is racy. The unique constraint on email saves us from corruption, but the losing pod doesn't surface the conflict. **→ [CF-427](https://linear.app/confabulous/issue/CF-427)** |
| A4 | High | `internal/auth/oauth.go:1506-1507` | The CLI-authorize handler redirects to `callback?key=<raw_api_key>`. Raw key appears in browser history, server access logs, and any forwarded `Referer`. **→ [CF-434](https://linear.app/confabulous/issue/CF-434)** (or separate ticket; deferred) |
| A5 | Medium | `internal/auth/password.go:29` | `SessionDuration = 7d` is absolute with no idle timeout. Stolen cookies are valid for the full week. **→ [CF-431](https://linear.app/confabulous/issue/CF-431)** |
| A6 | Medium | `internal/auth/oauth.go:285-286` | OAuth state comparison uses `!=` rather than `subtle.ConstantTimeCompare`. Timing channel is tiny on local strings; defense-in-depth fix. (Deferred — not separately ticketed; low risk.) |
| A7 | Medium | `internal/auth/oauth.go:1647` (`HandleDeviceToken`) | Device-code polling endpoint enforces the suggested 5s interval client-side but not server-side. A misbehaving client could brute-force codes. (Deferred.) |
| A8 | Info / passed | `internal/auth/auth.go:33-54` | API key generation: 256 bits of crypto/rand, SHA-256 hashed at rest. Constant-time `bcrypt.CompareHashAndPassword`. Sessions are cryptographically random 32-byte IDs. Cookies are `HttpOnly`, `Secure`, `SameSite=Lax`. |
| A9 | Info / passed | `internal/auth/oauth.go` localhost validator | Comprehensive — rejects IPv6, credentials in URL, trailing dots, non-localhost hosts, non-HTTP schemes. |

### API endpoints & input validation

| # | Severity | Location | Finding |
|---|----------|----------|---------|
| B1 | Medium | Many handlers, e.g. `internal/api/keys.go:47`, `shares.go:75` | `json.NewDecoder(r.Body).Decode(&req)` is missing `DisallowUnknownFields()` across all handlers. Unknown fields are silently accepted. Not actively exploitable but masks API drift. (Deferred — not separately ticketed.) |
| B2 | Info / passed | `internal/api/server.go:43-56` | Every route is wrapped in a `withMaxBody(MaxBody{XS,S,M,L,XL})`. No streaming endpoints without a limit. |
| B3 | Info / passed | `internal/api/access.go:42-75` | Canonical access model (CF-132) is enforced uniformly. DELETE/PATCH endpoints verify ownership via `db/access` before mutating. No IDOR found in audited handlers. |
| B4 | Info / passed | `internal/api/content_type.go:10-43` | Content-Type validation is strict (charset suffix correctly stripped). |
| B5 | Info / passed | `internal/api/server.go:666-680` | Static-file SPA serving correctly uses `filepath.Clean` + prefix check. |

### Database & SQL injection

| # | Severity | Location | Finding |
|---|----------|----------|---------|
| C1 | Info / passed | All 126 audited queries | All use `$N` placeholders. Two safe `fmt.Sprintf` uses (placeholder index building, table names from a `validateCardTypes` whitelist). No string-concatenated user input. |
| C2 | Info / passed | `internal/db/session/session.go:393` | Full-text search routes user input through `BuildPrefixTsquery` which strips tsquery operators before `to_tsquery('english', $N)`. Avoids the `to_tsquery` panic-on-bad-syntax class. |
| C3 | High (operator) | All migrations | Migrations run as the connection-string user, typically a superuser. Schema permissions are not split between migration-time and runtime roles. Self-hosted operators should create a least-privileged runtime role. **→ documented in SECURITY.md (existing) — no code change**. |
| C4 | Info | `internal/db/db.go` connection | `sslmode` is not enforced by code; relies on operator setting it in `DATABASE_URL`. Already covered by the deployment checklist. |

### Storage & file handling

| # | Severity | Location | Finding |
|---|----------|----------|---------|
| D1 | Medium | `internal/api/external.go:344` | `handleDownloadSessionFile` lacked `Content-Disposition`. **Fixed in this PR.** |
| D2 | Medium | `internal/api/compression.go:27` | Zstd decoder output bounded only at the route level. **Fixed in this PR** — `MaxBytesReader(decoder, MaxBodyXL)` now caps decompressed bytes at the middleware boundary. |
| D3 | Low | `internal/codex/parser.go` | `bufio.Scanner` line limit is 4 MB; total-file and JSON-depth bounds are not enforced. Analytics is a background worker, so DoS impact is contained, but a malicious rollout could OOM a worker. (Deferred — not separately ticketed; recommend monitoring worker memory.) |
| D4 | Low | `internal/storage/chunks.go:216-284` | Chunk merge silently skips gaps (nil entries in the line index). Sync protocol normally rejects this, but defense-in-depth would error explicitly. (Deferred.) |
| D5 | Info / passed | `internal/storage/s3.go:179-191` | S3 keys are built from `userID` (int64), validated `provider` (canonical-only), and length+UTF-8-validated `externalID`. No path traversal risk in keys. |

### Admin / operator surface

| # | Severity | Location | Finding |
|---|----------|----------|---------|
| E1 | High | bootstrap | See A3. **→ [CF-427](https://linear.app/confabulous/issue/CF-427)** |
| E2 | Medium | `internal/admin/middleware.go:32` | Admin status is determined by `SUPER_ADMIN_EMAILS` env-var allowlist. Typo locks out all admins with no recovery short of a restart. Recommend a secondary DB-backed `is_admin` flag as fallback. (Deferred — design question.) |
| E3 | Medium | `internal/admin/api_handlers.go:232,279,354` | Destructive admin actions (deactivate, delete, system share, card invalidation) execute immediately without a confirm-token step. Operator misclick causes data loss. **→ [CF-430](https://linear.app/confabulous/issue/CF-430)** |
| E4 | Medium | `internal/admin/audit.go` | Audit log lives in the same DB; an admin with DB access can rewrite history. **→ [CF-430](https://linear.app/confabulous/issue/CF-430)** |
| E5 | Medium | (multi) | No "last admin" protection. If demote/promote endpoints are added, this needs guardrails. (Deferred — not currently exploitable.) |

### Middleware, CORS, CSRF, rate limits

| # | Severity | Location | Finding |
|---|----------|----------|---------|
| F1 | High | `internal/api/server.go:122-137` + `cmd/server/main.go` | `ALLOWED_ORIGINS=*` paired with `AllowCredentials=true` is a CORS-spec violation that some lenient clients honor. **Fixed in this PR** at both startup (fatal) and parser (drops `*`). |
| F2 | Medium | `internal/clientip/middleware.go:28-147` | Trusts `Fly-Client-IP`, `CF-Connecting-IP`, `True-Client-IP`, `X-Real-IP`, `X-Forwarded-For` without a `TRUSTED_PROXY` allowlist. If deployed behind a proxy that doesn't strip these headers, IP-based rate limits are spoofable. **→ [CF-432](https://linear.app/confabulous/issue/CF-432)** |
| F3 | Medium | `internal/ratelimit/ratelimit.go:73-126` | Per-key bucket map is unbounded. Cleanup runs every 5 min for keys idle 10+ min, but a fast-rotating-IP attacker can grow the map within that window. **→ [CF-432](https://linear.app/confabulous/issue/CF-432)** |
| F4 | Info / passed | `internal/api/server.go:140-217` | Middleware order: Recoverer → ClientIP → RateLimit → RequestID → Logger → SecurityHeaders → Compression → CORS → CSRF → Auth. Rate limit is correctly before auth. |
| F5 | Info / passed | `internal/api/server.go:749-804` | Security headers (CSP, HSTS, X-Frame-Options DENY, nosniff, Referrer-Policy, Cross-Domain-Policies) all applied globally. HSTS gated on `INSECURE_DEV_MODE != "true"`. |
| F6 | Info / passed | `cmd/server/main.go:354-361` | pprof binds `127.0.0.1:6060` and is opt-in via `ENABLE_PPROF=true`. Now documented in `SECURITY.md`. |

### Secrets, config, logging

| # | Severity | Location | Finding |
|---|----------|----------|---------|
| G1 | High (docs/code mismatch) | `SECURITY.md` line 813 | `SESSION_SECRET` was documented as required but never read by the code. **Fixed in this PR** — removed from docs and replaced with a clarifying note. |
| G2 | Medium | `cmd/server/main.go` | `INSECURE_DEV_MODE=true` had no startup warning. **Fixed in this PR** — logs WARN at startup. |
| G3 | Medium | `cmd/server/worker.go:422-457` | Smart Recap silently disables if `ANTHROPIC_API_KEY` is missing while `SMART_RECAP_ENABLED=true`. Operator gets no warning. (Deferred — recommend a startup warning; not separately ticketed.) |
| G4 | Low | `cmd/server/main.go:204-243` | Partial OAuth configs (ID set, secret missing) are silently disabled. (Deferred.) |
| G5 | Info / passed | `internal/api/flylogger.go:142-153` | Log sanitization strips control characters, prevents log injection. |
| G6 | Info / passed | `internal/anthropic/client.go:84` | Anthropic API key is set as `X-API-Key` and never included in OTel span attributes (which record only token counts and model name). |

### Sharing & public access

| # | Severity | Location | Finding |
|---|----------|----------|---------|
| H1 | High | `internal/api/shares.go:159-206` | `HandleCreateShare` does not call `emailService.CheckRateLimit(userID, len(recipients))` upfront — only per-email checks happen inside the send loop. A 50-recipient share can drain quota for legitimate users. **→ [CF-429](https://linear.app/confabulous/issue/CF-429)** |
| H2 | Medium | (no rate limit on share creation) | A user can mint unlimited shares per session. DB / disk growth attack. **→ [CF-429](https://linear.app/confabulous/issue/CF-429)** |
| H3 | Medium | `internal/db/access/access.go:64-88` | Share expiry is enforced in `GetSessionAccessType` (good) but NOT in `ListShares` / `ListAllUserShares`. UX confusion, no security impact directly. **→ [CF-433](https://linear.app/confabulous/issue/CF-433)** |
| H4 | Low | `internal/api/shares.go:182` | Invitation URLs include `?email=<recipient>` for UX. Leaked to proxy/access logs. **→ [CF-433](https://linear.app/confabulous/issue/CF-433)** |
| H5 | Medium | `internal/db/access/shares.go:141-209` | `CreateSystemShare` does NOT enforce admin auth itself; it trusts the caller. Verify the HTTP handler is behind admin middleware. (Manual verification confirmed it is — handler is in `admin/api_handlers.go` under admin route group. No code change needed; added inline comment recommendation deferred.) |
| H6 | Low / info | `internal/db/access/access.go:177-181` | Public shares return `shared_by_email` (owner identity). Intentional but worth documenting for self-hosted operators who may want to hide owner identity. (Deferred — design question.) |

### Dependencies & supply chain

| # | Severity | Location | Finding |
|---|----------|----------|---------|
| I1 | High | `go.mod` | govulncheck reported 18 production-reachable findings against Go 1.25.5 stdlib + `golang.org/x/net@v0.48.0` + `go.opentelemetry.io/otel/sdk@v1.39.0`. **Fixed in this PR** — Go 1.25.10 + x/net@v0.54.0 + otel/sdk@v1.43.0 → 16 cleared. Remaining 2 (Moby GO-2026-4887 / GO-2026-4883) are test-only via testcontainers, no upstream fix, do not ship in the runtime binary. |
| I2 | Low | `Dockerfile` | Base images (`node:24-alpine`, `golang:1.25-alpine`, `alpine:latest`, `migrate/migrate:v4.19.1`) are floating tags, not pinned by digest. Self-hosted users get supply-chain drift on rebuild. **→ [CF-434](https://linear.app/confabulous/issue/CF-434)** |
| I3 | Low | `Dockerfile` build flags | `go build` lacks `-trimpath` (leaks absolute paths into binary). **→ [CF-434](https://linear.app/confabulous/issue/CF-434)** |
| I4 | Info | `go.mod` | No `replace` directives, no vendored modules, no `v0.0.0-` pseudo-version pins. Clean. |
| I5 | Recommend | (no CI gate) | `govulncheck ./...` is not run in CI. Recommend wiring it in. (Deferred — not separately ticketed.) |

## Test plan

- `go build ./...` — clean after dep bumps.
- `go test -short ./...` — all 30+ packages pass.
- New tests added:
  - `TestParseAllowedOrigins` — wildcard drop, multi-origin parsing, whitespace
    handling.
  - `TestZstdBombBounded` — confirms `MaxBytesReader` fires when decompressed
    body exceeds `maxDecompressedBody`.
  - `TestSanitizeContentDispositionFilename` — filename sanitization, header
    injection resistance, empty/Unicode handling.
- `govulncheck ./...` — production-reachable vulns dropped 18 → 2 (both
  test-only, both unfixed upstream).
- Full integration test suite (`DOCKER_HOST=... go test ./...`) — run before
  PR open.

## Acknowledgements

This audit was conducted with eight parallel sub-agents over a single
afternoon. Each agent's report is preserved in the conversation transcript for
traceability. Findings that I could not independently verify in the time
available are flagged in this report with a more conservative severity than the
sub-agent's original rating.
