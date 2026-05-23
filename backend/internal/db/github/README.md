# github

GitHub link CRUD for associating pull requests and commits with sessions.

## Files

| File | Role |
|------|------|
| `store.go` | `Store` struct definition and OpenTelemetry tracer |
| `links.go` | `CreateGitHubLink`, `GetGitHubLinksForSession`, `GetGitHubLinkByID`, `DeleteGitHubLink` |
| `links_integration_test.go` | Integration tests for insert, upsert source/URL update, the three `overwriteTitle` branches, list ordering, empty-result, invalid-UUID → `ErrSessionNotFound`, FK violation, and the shared `ErrGitHubLinkNotFound` paths on Get/Delete |

## Key API

- **`CreateGitHubLink(ctx, link, overwriteTitle)`** -- Upserts a GitHub link (PR or commit) for a session. The unique key is `(session_id, link_type, owner, repo, ref)`. On conflict, updates `source` and `url`. Title handling depends on `overwriteTitle`: when true the new title always wins; when false the existing title is preserved if non-null (fill-only semantics for background enrichment).
- **`GetGitHubLinksForSession(ctx, sessionID)`** -- Returns all GitHub links for a session, ordered by `created_at DESC`.
- **`GetGitHubLinkByID(ctx, linkID)`** -- Returns a single link by primary key. Returns `ErrGitHubLinkNotFound` if missing.
- **`DeleteGitHubLink(ctx, linkID)`** -- Deletes a link by ID. Returns `ErrGitHubLinkNotFound` if missing.

## How to Extend

1. **New link type**: Add a new `link_type` value (e.g., `"issue"`). The schema uses a text column, so no migration is needed for the enum. Update the session list CTEs in `db/session/` to aggregate the new type if it should appear in list views.
2. **Bulk upsert**: Add a batch version of `CreateGitHubLink` using multi-row INSERT with a values builder, following the pattern in `access/shares.go`.

## Invariants

- The unique constraint on `(session_id, link_type, owner, repo, ref)` ensures one link per PR number or commit SHA per session. Re-syncing the same link updates metadata without creating duplicates.
- `overwriteTitle = false` uses `COALESCE(session_github_links.title, EXCLUDED.title)` in the ON CONFLICT clause -- this preserves manually-set titles while allowing automatic enrichment to fill in missing titles.
- Invalid UUID session IDs are caught and returned as `ErrSessionNotFound` in `GetGitHubLinksForSession`.

## Design Decisions

- **Upsert over insert-or-ignore**: Links can come from multiple sources (CLI sync, webhook, manual). Upsert ensures the latest source and URL are always recorded while preserving the creation timestamp.
- **Two-mode title handling**: The `overwriteTitle` flag supports two use cases: explicit user actions (always overwrite) and background enrichment (fill-only, don't clobber user edits).
- **Flat owner/repo/ref columns**: Stored denormalized rather than as a URL string, enabling efficient queries for PR aggregation in session list views (see `github_pr_refs` and `github_commit_refs` CTEs in `db/session/`).
- **Fork→root inference happens outside this package (CF-491)**: PR links are the canonical signal for "fork → upstream" because every PR URL points to the merge target. `CreateGitHubLink` is intentionally pure — the resolver lives in `api/sync.go::HandleSyncChunk` after the PR-link loop and calls `db.RecordRepoRoot` on `session_repos` when the session's extracted `git_repo_url` differs from the PR's `owner/repo`. The manual API path (`HandleCreateGitHubLink`) does not invoke the resolver; later sync chunks pick it up.

## Testing

- Integration tests in `links_integration_test.go` cover the four store functions: insert, upsert (source/URL change preserves ID), the three `overwriteTitle` branches (overwrite, preserve, fill-null), list ordering by `created_at DESC`, empty list, invalid-UUID → `ErrSessionNotFound`, FK violation on Create, and the `ErrGitHubLinkNotFound` sentinel on both `GetGitHubLinkByID` and `DeleteGitHubLink`. Uses `testutil.SetupTestEnvironment`. End-to-end PR/commit aggregation continues to be exercised by the API-level session list tests.

## Dependencies

- `github.com/ConfabulousDev/confab-web/internal/db` -- Root DB package for types, errors, helpers (`IsInvalidUUIDError`)
- `github.com/ConfabulousDev/confab-web/internal/models` -- `GitHubLink` type with `LinkType` enum
- `go.opentelemetry.io/otel` -- Distributed tracing
