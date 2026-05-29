# Adding a New Provider

This guide covers what it takes to add a third AI agent provider (alongside Claude Code and Codex). The provider abstraction was introduced in CF-399 / CF-402 / CF-403; the registry lives in `internal/analytics/provider.go`.

## Contract

A provider implements `SessionProvider` in `internal/analytics/provider.go`:

```go
type SessionProvider interface {
    Parse(ctx context.Context, input ParseInput) (Rollout, error)
    ComputeCards(ctx context.Context, rollout Rollout) *ComputeResult
    SearchText(ctx context.Context, rollout Rollout) string
    PrepareTranscript(ctx context.Context, rollout Rollout) (xml string, idMap map[int]string, err error)
    ClearMessageIDs() bool
    DisplayName() string
}
```

- `Parse` downloads transcript bytes from `ParseInput.Store` (S3), parses them into a provider-specific `Rollout`, and returns `(nil, nil)` for an empty session (no transcript file yet). The `Rollout` is opaque — a marker interface — so each provider can use its own struct.
- `ComputeCards` maps the parsed rollout onto `ComputeResult` (the cross-provider canonical shape).
- `SearchText` returns the Weight-C content for the search index (typically: user messages + assistant final text + tool-call summaries, capped at 500 KB).
- `PrepareTranscript` builds the XML envelope (`<transcript><user>…</transcript>`) the smart-recap LLM consumes, plus an `idMap` from sequential ids to provider-specific message identifiers. The smart-recap system prompt is **provider-agnostic by design** (CF-447) — it describes the XML structure categorically, so a new provider does not need to touch the prompt; whatever element shapes you emit will be summarized correctly.
- `ClearMessageIDs` returns `true` when smart-recap annotated items should drop their MessageID (the provider has no stable frontend anchors). Codex returns `true`; Claude returns `false`.
- `DisplayName` returns the human-facing label ("Claude Code", "Codex") used in email subject lines and other display surfaces.

## Single-goroutine contract on Rollout

Each provider's `Rollout` is **not safe for concurrent use**. The API handler and precompute worker both call methods sequentially on a given rollout instance. Providers can keep mutable cache state on the rollout without a mutex.

## Lazy-materialize for multi-file sessions

For providers whose session spans multiple files on disk (Claude's main + agent files, Codex's main + subagent rollouts), cache parsed files on the rollout during the first traversal so subsequent methods reuse them without a second S3 download. See `claudeRollout.cachedAgents` / `codexRollout.cachedAgents` for the reference implementations. The caching is invisible to callers; it just makes `ComputeCards` + `PrepareTranscript` on the same rollout cheap.

## Registration

Register in an `init()` function in the provider's file:

```go
func init() {
    RegisterProvider(&newProvider{}, "new-canonical", "Optional Legacy Alias 1", "Optional Legacy Alias 2")
}
```

- Canonical names are stored in `sessions.session_type` by `api/sync.go::handleSyncInit`.
- Legacy aliases route to the same provider instance. Used by Confab to keep historical display-form values (e.g. `"Claude Code"`) working without backfill migrations — see CF-400 for the OSS self-hosted rationale.
- Duplicate registrations panic. Empty names panic. Both fail loudly at process start.

## Test guardrails

- `TestRegistryCoversAllowedProviders` (in `provider_dispatch_test.go`) walks every value in `models.AllowedProviders` and asserts `ProviderFor` returns a handler. **Add your provider value to `models.AllowedProviders` AND register the handler — if you miss one, this test fails immediately.**
- `TestPrecomputeGoHasNoProviderSwitchOrLiterals` (this package) source-scans `precompute.go`.
- `TestAnalyticsGoHasNoProviderLiterals` (in `internal/api/`) source-scans `api/analytics.go`.

If you reintroduce a `switch session.Provider` or hardcode a provider literal at either boundary, these tests fail loudly.

## Optional: per-provider HTTP intake metadata

CF-403 intentionally left HTTP-intake metadata as **typed per-provider fields**. The registry is not used for intake — each provider declares its sub-block directly on `SyncChunkMetadata` in `internal/api/sync.go` (e.g. Codex's `CodexRollout *SyncCodexRolloutMetadata`).

To add intake metadata for a new provider:
1. Add a typed sub-block field on `SyncChunkMetadata` (~5 lines).
2. Add a `ValidateXxxMetadata` function in `internal/validation/input.go`.
3. Call the validator in `handleSyncChunk`.
4. After session ownership is verified and S3 + sync_files writes succeed, upsert via a dedicated `internal/db/<provider>/` store.

The Codex implementation (`SyncCodexRolloutMetadata` + `ValidateCodexRolloutMetadata` + `dbcodex.UpsertRollout`) is the reference pattern. Per-thread relationships in a sidecar table (e.g. `codex_rollouts`) keep the main `sessions` schema clean and let the frontend render tree views without coupling analytics to the sidecar.

## DB-side considerations

- `sessions.session_type` has **no CHECK constraint** today. The registry is the only validation — `ProviderFor` returns an error for unknown values, which both the worker and HTTP handler treat as "skip + log".
- If a CHECK constraint is added later, it must be regenerated from `models.AllowedProviders` so legacy aliases survive.
- The codex `ListSubtree` recursive CTE (`internal/db/codex/rollouts.go`) is the reference for per-provider sidecar trees. It's currently uncalled from production code (kept for the future frontend tree view).

## Per-card / per-provider file layout

Each card has a dedicated analyzer file per provider, named `analyzer_<card>_<provider>.go`. `ls analyzer_*.go` shows the full grid at a glance. When adding a third provider, create the parallel set of analyzer files and update the README matrix.

## Checklist for the third-provider author

- [ ] Add canonical name + permanent aliases to `internal/models/provider.go::CanonicalProviders` / `AllowedProviders` / `LegacyAliases`.
- [ ] Implement `SessionProvider` in `internal/analytics/<provider>_provider.go`. Include lazy-materialize if your sessions span multiple files.
- [ ] Register the provider in `init()`.
- [ ] Add per-card analyzers `analyzer_<card>_<provider>.go` (7 cards: tokens, session, tools, code_activity, conversation, agents_and_skills, redactions).
- [ ] Implement transcript XML emission in `<provider>_transcript.go` and search-index extraction in `<provider>_search.go`. Keep the orchestrator (`ComputeFromXxxRollout`) thin — it dispatches to the per-card analyzers.
- [ ] Optional: typed HTTP-intake metadata sub-block (above).
- [ ] Frontend adapter — see `frontend/src/providers/README.md` for the matching frontend checklist.
- [ ] Per-provider test fixtures in `frontend/src/test-fixtures/session.ts::DEFAULTS_BY_PROVIDER`.
- [ ] Pricing: add the provider's families to the single source `backend/internal/pricingsource/pricing.json` (provider-nested) and bump `updated_at`. The frontend reads the table from the backend at runtime — no second table to update.
- [ ] Ensure `TestRegistryCoversAllowedProviders` passes (it will if step 1 + step 3 are in sync).
