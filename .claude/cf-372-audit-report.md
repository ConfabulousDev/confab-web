# CF-372 — Codex audit: backend ingest, sync, dedup, validation

**Audit date:** 2026-05-18
**Branch:** `cf-372-codex-audit-backend-ingest`
**Audit scope (per ticket):** `backend/internal/api/sync.go`, `backend/internal/db/session/sync.go`, `backend/internal/db/provider.go` (no longer exists — relevant logic now in `backend/internal/models/provider.go`), `backend/internal/validation/input.go`, related integration tests.
**Out of scope:** Any frontend rendering work, analytics card content, share/ACL changes. Cross-references to the frontend (Codex schemas, unknown-row rendering) are evidence only.

## TL;DR

The ingest path is in good shape on the dimensions the ticket asks about. The provider-aware `(user_id, provider, external_id)` invariant on `FindOrCreateSyncSession` is enforced and tested end-to-end (HTTP and DB layers). Per-chunk storage paths are provider-scoped in S3. `NormalizeProvider` is applied at every read site I checked. Unknown line types pass through both the backend parser and the frontend Zod schemas.

Two narrow gaps worth filing as follow-ups, both low real-world impact but real invariant violations:

1. **External-ID-only lookups bypass provider isolation.** `GetSessionIDByExternalID` (powers `GET /api/v1/sessions/by-external-id/{external_id}`) and `UpdateSessionSummary` (powers `PATCH /api/v1/sessions/{external_id}/summary`) both filter on `(external_id, user_id)` without a provider segment. If a single user ever has two sessions with the same `external_id` under different providers, these two endpoints return / mutate an arbitrary row. The DB schema does not prevent this collision (the unique constraint is `(user_id, external_id, session_type)`).
2. **Chunk-level retry is not idempotent.** Re-submitting the *exact same* chunk after a successful upload is hard-rejected with `first_line must be N (got N-K)`. If a network error truncates the response after the S3 write committed and the `sync_files` row updated, the CLI has no way to recognize it from the chunk endpoint alone and must re-call `/sync/init` to reconcile. Workable, not great.

A third item is cosmetic but worth noting:

3. **No machine-readable provider discovery.** The backend's `models.CanonicalProviders` list is enforced via validation but is not surfaced as a discovery endpoint. The CLI hard-codes `claude-code` / `codex` in lockstep with the backend; today this is fine because the enum is small and closed, but adding a third provider will require simultaneous backend + CLI releases.

Details below, by checklist item.

---

## Checklist

### 1. Provider plumbing on every sync endpoint — PASS

- **POST /api/v1/sync/init** (`sync.go:143`) reads `req.Provider *string` so it can distinguish *omitted* (defaulted to `claude-code` for backward compat with older CLI binaries) from *explicit empty string* (rejected as a `validation.ValidateProvider` error). The resolved provider is plumbed into `db.SyncSessionParams` and echoed back in the response as a `Provider` field so callers can verify acceptance.
- **POST /api/v1/sync/chunk** (`sync.go:269`) does not take a provider on the wire. It looks the provider up via `VerifySessionOwnership` (`db/session/session.go:667`), which returns the canonical form via `models.NormalizeProvider`. The provider then flows into:
  - `storage.UploadChunk(ctx, userID, provider, externalID, ...)` — S3 key prefix is `{userID}/{provider}/{externalID}/chunks/...` (`storage/s3.go:189`).
  - The Claude-Code-vs-Codex parse gate (`sync.go:421`): `parseClaudeCode := provider == models.ProviderClaudeCode && req.FileType == "transcript"` controls PR-link extraction; `extractTimestamps := req.FileType == "transcript"` is provider-agnostic per CF-355.
  - The `codex_rollouts` sidecar upsert (`sync.go:516`), gated to codex sessions with a defense-in-depth check at `sync.go:365` after `VerifySessionOwnership` (so we don't leak existence of another tenant's claude-code session via the validation error).
- **POST /api/v1/sync/event** (`sync.go:598`) routes through `VerifySessionOwnership` for access control, but the event payload itself is provider-agnostic and persisted as `json.RawMessage`. Nothing leaks here.
- **Storage layer defense-in-depth.** `storage/s3.go:198, 239, 288` reject invalid provider values at `UploadChunk` / `ListChunks` / `DeleteAllSessionChunks` so a plumbing bug fails loudly at the boundary instead of as missing objects in S3. `TestUploadChunk_RejectsInvalidProvider` (`storage/s3_test.go:215`) and siblings cover this.

**No silent defaulting to `claude-code` outside the documented `req.Provider == nil` backward-compat default.** I checked every code path that constructs a `SyncSessionParams` or calls a chunk method.

### 2. Codex external-ID isolation — PASS for sync, FAIL for two read paths

The core sync invariant — `(user_id, provider, external_id)` is the dedup key — is enforced correctly in `FindOrCreateSyncSession` (`db/session/sync.go:34`). The SELECT uses `session_type = ANY($3)` with `models.ExpandWithAliases` so legacy `'Claude Code'` rows still match a canonical `claude-code` lookup. The INSERT writes the canonical form. The race-condition recovery path re-runs the same lookup. Coverage:

- `TestFindOrCreateSyncSession_ProviderIsolation` (`db/session/sync_test.go:882`) — two sessions for the same user/external_id under `claude-code` and `codex` get distinct UUIDs and distinct `session_type` columns.
- `TestSyncInit_Provider_HTTP_Integration / dedupes per provider` (`api/sync_http_integration_test.go:3637`) — same assertion at the HTTP layer.
- `TestFindOrCreateSyncSession_CodexLookupSkipsLegacyClaudeCode` (`db/session/sync_test.go:1018`) — a codex `FindOrCreate` does not accidentally pick up a legacy `'Claude Code'` row for the same external_id.

**Two paths bypass this invariant by reading on `(external_id, user_id)` without provider:**

- `db/session/session.go:842` — `GetSessionIDByExternalID`:
  ```go
  query := `SELECT id FROM sessions WHERE external_id = $1 AND user_id = $2`
  ```
  Used by `HandleLookupSessionByExternalID` at `api/sessions_view.go:163`, exposed as `GET /api/v1/sessions/by-external-id/{external_id}`. If the user has two sessions with the same external_id under different providers, `QueryRowContext` returns the first row Postgres scans, which is non-deterministic.

- `db/session/session.go:718` — `UpdateSessionSummary`:
  ```go
  query := `UPDATE sessions SET summary = $1 WHERE external_id = $2 AND user_id = $3`
  ```
  Used by `handleUpdateSessionSummary` at `api/sync.go:979`, exposed as `PATCH /api/v1/sessions/{external_id}/summary`. Updates **every** matching row — both the Claude and Codex sessions get the same new summary.

  > Note: the affected-rows check at `session.go:730` only differentiates `RowsAffected == 0` (404/403); it does not warn when N > 1.

**Real-world likelihood:** very low. Both providers mint UUIDv4 external IDs for session keys, so collisions across providers within one user are vanishingly improbable. But the schema permits the collision (the unique constraint is `(user_id, external_id, session_type)`), and the audit invariant is that the key tuple includes provider everywhere it's used. Recommended fix: either add a `provider` parameter to both calls and update the route to accept it, or change the schema to make `(user_id, external_id)` globally unique — which would also force a cleaner CLI handshake.

**Storage layer is unaffected.** S3 keys are `{userID}/{provider}/{externalID}/chunks/...`, so even if two such collisions existed, the chunk subtrees would not collide. Verified `chunkPrefix` at `storage/s3.go:189`.

### 3. Schema validation: every Codex line type from real sessions passes `KnownCodexLineSchema` — PASS (frontend); N/A (backend)

The ticket references `KnownCodexLineSchema`. That name only exists on the **frontend** (`frontend/src/schemas/codexTranscript.ts:425`). The backend has no equivalent Zod-style strict-schema gate — its Codex parser (`backend/internal/codex/parser.go`) is permissive by design: lines that fail `json.Unmarshal` into a `rawLine` envelope get logged to `ValidationErrors`; unknown top-level types are silently skipped (`parser.go:160`); unknown nested payload types fall through their respective switch defaults. This is the right design for an ingest pipeline that cannot afford to reject a forward-compatible Codex CLI upgrade.

Frontend coverage:

- `RawCodexLineSchema` (`schemas/codexTranscript.ts:433`) is the union of `KnownCodexLineSchema` (`session_meta`, `turn_context`, `response_item`, `event_msg`, `compacted`) and `CodexUnknownLineSchema` (catch-all on `{ timestamp, type, payload? }`). Everything uses `.passthrough()`.
- Nested discriminators have the same shape: `KnownResponseItemPayloadSchema` (7 known + 1 unknown) and `KnownEventPayloadSchema` (7 known + 1 unknown).
- `isKnownCodexLine`, `isKnownResponseItemPayload`, `isKnownEventPayload` predicates narrow the union for downstream switches.
- `schemas/codexTranscript.test.ts` exercises every known type and explicitly asserts the unknown catch-all branch (`it('accepts unknown top-level type via catch-all')`).

**Asymmetry note (informational, not a defect):** the backend parser and the frontend schema are independently maintained. If a new known line type is added to the Codex CLI, the backend needs an `analyzer_<card>_codex.go` update (per CLAUDE.md), and the frontend needs both a Zod entry in `KnownResponseItemPayloadSchema`/`KnownEventPayloadSchema` and a renderer in the Codex transcript components. There's no test that asserts the backend's known type list matches the frontend's. Acceptable today (small surface, small team).

### 4. Unknown line forward-compat — PASS

- **Backend**: `parser.go:147` dispatches on `typ`; the `default` branch silently skips the line. Re-reads of the raw transcript (`/api/v1/sessions/{id}/sync/file`) return the original bytes — the unknown line is preserved on disk and replayable.
- **Backend chunk endpoint**: the chunk handler does no per-line schema validation. It treats the body as opaque text, writes it to S3, and extracts only timestamps (provider-agnostic, requires only top-level `"timestamp"`) and PR-links (claude-code only). An unknown line type in either provider's transcript would not cause a 4xx.
- **Frontend**: `isKnownCodexLine` narrows to known types; the unknown branch renders via `CodexUnknownItem` (component file `frontend/src/components/transcript/codex/CodexUnknownItem.stories.tsx` confirms the renderer exists and has stories). `services/codexTranscriptService.ts` routes through `RawCodexLineSchema.safeParse`, so a totally novel `type` value still produces a renderable item.

### 5. Re-import idempotence (session level) — PASS

`FindOrCreateSyncSession` is idempotent by `(user_id, provider, external_id)`. Re-init does not duplicate the session row; it returns the existing UUID and refreshes mutable metadata via `updateSessionMetadata` (`db/session/sync.go:119`). Race recovery on unique-violation re-runs the lookup. Tests at `db/session/sync_test.go:882-1013` cover both the canonical-form and legacy-`'Claude Code'`-row paths.

**S3 chunk leaks on re-init:** none. Re-init returns the same `session_id`, and the chunk endpoint deduplicates by `(session_id, file_name, last_synced_line)` continuity. The CLI's resume-from-`last_synced_line` flow has no path to write a duplicate chunk for an already-stored line range.

### 5b. Re-import idempotence (chunk level) — PARTIAL

The chunk endpoint enforces strict line continuity: a chunk's `first_line` must equal `last_synced_line + 1`. Submitting the same chunk twice in a row produces `400 first_line must be N (got M)`. This is correct for the happy path (every retry of a successfully-uploaded chunk would otherwise duplicate data) but it leaves one failure mode:

- The chunk uploads successfully to S3 (`storage/s3.go:222`), the DB row updates (`db/session/sync.go:185`), and then the HTTP response is lost (timeout, connection reset, client crash before reading the body).
- On retry, the CLI sees `400 first_line must be N` and has no other signal that the previous attempt actually committed.
- Recovery requires calling `/sync/init` again to read the updated `LastSyncedLine` and resume.

Workable in practice (the CLI does this) but a `/sync/state?session_id=...&file_name=...` GET endpoint would let the CLI confirm without re-paying the dedup work in `FindOrCreateSyncSession`. Filing as a follow-up.

There is one explicit partial-failure path inside the handler (`sync.go:484`): if `UpdateSyncFileState` fails after `UploadChunk` succeeded, the chunk is in S3 but the DB row is stale. The next legitimate retry (with the *same* `first_line`) will be rejected by the continuity check. The CLI must re-init to discover the drift. The self-healing `chunk_count` reconciliation on read (`sync.go:786-810`) does not help here — it corrects the count but not `last_synced_line`.

### 6. Large-file handling — PASS

- **Hard limits**:
  - `storage.MaxChunksPerFile = 30000` (`storage/s3.go:41`) → enforced both pre-upload (`sync.go:398`, "soft" check via cached count) and during `ListChunks` (`storage/s3.go:269`, hard fail with `ErrTooManyChunks`).
  - `storage.MaxLineNumber = 99999999` (`storage/s3.go:172`) — line numbers fit in 8-digit zero-padded S3 keys so lexicographic order matches numeric order.
  - `api.MaxBodyXL = 16 * 1024 * 1024` (`server.go:48`) — applied to `/sync/chunk`.
  - `storage.MaxMergeLines = 10_000_000` (`storage/chunks.go:28`) — safety net during read-side merge.
- **Boundary handling**:
  - Off-by-one: `first_line < 1` rejected (`sync.go:302`); empty `lines` array rejected (`sync.go:306`); `lastLine = firstLine + len(lines) - 1` (`sync.go:452`) — consistent inclusive bounds.
  - Last chunk: no special-case needed; the merge is line-indexed, not chunk-indexed (`storage/chunks.go:222`).
  - Retry mid-upload: see §5b for the gap.
- **Self-healing on read**: `sync.go:786-810` reconciles `sync_files.chunk_count` against the actual S3 count if it drifts (owner-only, on full reads). This corrects past partial-failure damage without requiring a one-shot migration.
- **Codex-specific boundaries**: nothing in the chunk code is provider-shaped; the off-by-one math and limits apply identically to Codex transcripts. Test `TestSyncChunk_Provider_HTTP_Integration` (`api/sync_http_integration_test.go:3723`) exercises the codex transcript chunk path including the chunk-count, timestamp, and codex-rollout-sidecar branches.

### 7. CLI ↔ backend handshake for provider negotiation — INFORMATIONAL

There is no explicit handshake endpoint. The backend's accepted provider enum lives in `models.CanonicalProviders = []string{"claude-code", "codex"}` (`models/provider.go:33`) and is enforced by `validation.ValidateProvider` (`validation/input.go:139`), which returns the canonical list inside its error message but does not expose it as a structured API.

The CLI (separate repo) currently hard-codes the same enum. The init endpoint's response includes a `Provider` field (`sync.go:65`) so the CLI can verify *acceptance* of its chosen value, but cannot *discover* the allowed set without parsing error strings.

For two providers and a single-team OSS project, this is adequate. Adding a third provider would require coordinated backend + CLI releases. A trivial `GET /api/v1/sync/providers` returning `{ "providers": ["claude-code", "codex"] }` would let the CLI fail fast against an older backend; filing as a low-priority follow-up.

### 8. `NormalizeProvider` at every `sessions.session_type` read site — PASS

Every production read of `sessions.session_type` that surfaces the value (vs. just filtering on it) runs through `models.NormalizeProvider`:

- `db/session/session.go:108, 624, 701, 825` — session detail / list / `VerifySessionOwnership` / `GetSessionOwnerExternalIDAndProvider`.
- `db/access/access.go:165` — canonical access lookups.
- `db/access/shares.go:61, 179, 246, 284, 368` — every share read variant.
- `analytics/precompute.go:301, 645, 748` — every precompute worker scan.
- `analytics/trends.go:441, 647, 811` — trends per-provider grouping.
- `analytics/org_analytics.go:222` — org provider list.
- `api/shares.go:178` — wire-layer defensive re-normalization.

Filter parameters (where `session_type` is fed *into* a `= ANY(...)` clause) correctly use `models.ExpandWithAliases` / `models.AllowedProviders` instead of `NormalizeProvider`:

- `db/session/sync.go:115` (`buildSessionLookupQuery`) — `ExpandWithAliases`.
- `analytics/precompute.go:222, 570, 703` — `AllowedProviders` via `pq.Array`.
- `analytics/trends.go:193, 357` — `AllowedProviders`.
- `analytics/org_analytics.go:107, 203` — `AllowedProviders`.

`testutil.CreateTestSessionLegacyClaudeCode` (`testutil/helpers.go:133`) plus the read-side normalization tests in `db/session/sync_test.go:840` and `db/access/shares_test.go:353+` lock the contract end-to-end.

I did not find any production read of `s.session_type` (`sessions.session_type`) that bypasses both helpers.

---

## What changed in this PR

Nothing in code. This ticket is an audit; per the project's audit-report convention, the deliverable is this report. The two follow-up gaps and one informational item should be triaged into separate `Bug`-labeled tickets if accepted, so each lands as its own PR with its own test surface.

## Suggested follow-up tickets

1. **Bug** — Add provider isolation to `GetSessionIDByExternalID` and `UpdateSessionSummary`. Either (a) widen both call sites to take a provider hint from the caller and append `AND session_type = ANY($N)` to the SQL, or (b) tighten the schema constraint to `UNIQUE(user_id, external_id)` and back-fill resolution rules. Add wire-level tests that seed two same-`external_id` rows under different providers and assert each endpoint disambiguates.
2. **Bug** — Add a `GET /sync/state?session_id=&file_name=` endpoint (or equivalent) so the CLI can confirm `last_synced_line` after a network-truncated chunk upload without re-paying `FindOrCreateSyncSession`'s lookup + metadata refresh. Update CLI to use it on retry-after-network-error.
3. **Feature (low priority)** — `GET /api/v1/sync/providers` returning `{ "providers": ["claude-code", "codex"] }` so CLI binaries built against a newer backend can either fail fast or auto-degrade. Useful when a third provider is added.

## Files I read end-to-end during this audit

- `backend/internal/api/sync.go`
- `backend/internal/db/session/sync.go`
- `backend/internal/db/session/session.go` (relevant sections)
- `backend/internal/db/session/sync_test.go`
- `backend/internal/api/sync_http_integration_test.go` (provider + chunk subtests)
- `backend/internal/api/sessions_view.go` (HandleLookupSessionByExternalID)
- `backend/internal/api/sessions_http_integration_test.go` (TestLookupSessionByExternalID_HTTP_Integration)
- `backend/internal/validation/input.go`
- `backend/internal/models/provider.go`
- `backend/internal/storage/s3.go` + `chunks.go`
- `backend/internal/codex/parser.go` (relevant sections)
- `frontend/src/schemas/codexTranscript.ts` + tests
