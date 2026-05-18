# updatecheck

Reports whether the running backend build is behind the latest GitHub release of `ConfabulousDev/confab-web`. Powers the in-product "Update available" badge surfaced via `GET /api/v1/auth/config`.

## Files

| File | Purpose |
|------|---------|
| `checker.go` | `Checker` with lazy fetch + in-memory TTL cache, `NewChecker`, `Status`, semver comparison via `golang.org/x/mod/semver` |
| `checker_test.go` | `httptest.Server`-driven unit tests covering disabled, success, cache hit/expiry, failure cooldown, prerelease filter, dev bias, headers, context cancellation |

## Exported API

- `type Status struct` — snapshot serialized as the `version` object on `/api/v1/auth/config`. Fields: `Current`, `Latest`, `LatestURL`, `UpdateAvailable`, `UpdateCheckDisabled`, `UpdateCheckFailed`.
- `type Checker struct` — owns the cache and HTTP client. Safe for concurrent use.
- `NewChecker(version string, disabled bool) *Checker` — binds to the running version. When `disabled` is true the checker never contacts GitHub.
- `(*Checker).Status(ctx) Status` — returns the cached status, refreshing if stale. Blocks up to `requestTimeout` (3 s) on the first call after a cache expiry.

## Behavior

- **Lazy**: zero work at construction; first `Status()` call triggers the fetch.
- **TTLs**: 6 h after a successful fetch, 15 min after a failure. Tunables are package-level `var`s so tests can shrink them.
- **GitHub headers**: `User-Agent: confab-backend/<version>`, `Accept: application/vnd.github+json`.
- **Prerelease filter**: ignored even though `/releases/latest` returns only stable releases by design (defensive).
- **Dev bias**: when `Current == ""` (local `go run` without `-ldflags`) the checker still fetches and forces `UpdateAvailable: true` so the badge is visible during development.
- **No concurrent-fetch dedupe**: two callers seeing a stale cache may both fetch GitHub; accepted because confab-web is single-tenant self-hosted and the surge is bounded.
- **Logging**: `logger.Warn("github release check failed", ...)` on every failed attempt; the 15 min cooldown bounds the volume.

## Invariants

- `Status.Current` is always the value passed to `NewChecker`, regardless of fetch outcome.
- `Status.UpdateAvailable` implies `Status.LatestURL` is non-empty; the frontend uses both as the badge's render gate.
- `UpdateCheckDisabled` and `UpdateCheckFailed` are mutually informative but technically independent; the frontend hides the badge if either is true.

## Repo coordinates

`githubRepo` is a `const`: `"ConfabulousDev/confab-web"`. Forks that want their own update channel must patch the constant.

## Adding a new release-source field

If the response shape needs a new field (e.g. `release_channel`), update:

1. `Status` struct + JSON tag here.
2. `Status`-construction call sites inside `Status()`.
3. `frontend/src/contexts/AppConfigContext.tsx` `VersionInfo` interface and `fetchAppConfig.ts` parser.
4. `backend/API.md` — the `version` object documentation under `GET /api/v1/auth/config`.
