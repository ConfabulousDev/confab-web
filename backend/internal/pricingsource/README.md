# pricingsource

Owns the model price table. There is exactly one source of price data in the
repo: **`pricing.json`** (embedded here via `go:embed`). This package serves
that table — and an optional fresher copy pulled from confabulous.dev — to the
frontend (`GET /api/v1/pricing`) and to the analytics cost compute.

Modeled on [`internal/updatecheck`](../updatecheck/): a lazy, TTL-cached,
best-effort fetch that never blocks and always returns a valid document.

## Why

Model prices change often. Baking them into the binary meant a self-hosted
backend had to redeploy to pick up a new price (or a new model in an existing
provider). Now a self-hosted backend pulls the freshest table from
confabulous.dev at runtime; only the canonical instance ships the authoritative
`pricing.json`. New providers and new billing *mechanics* still need code.

## Files

| File | Contents |
|------|----------|
| `pricing.json` | The single source of truth: `{ schema_version, updated_at, pricing }`, provider-nested (`claude-code` / `codex` → family → rates, USD per million tokens). **Edit this and bump `updated_at` to change a price.** |
| `source.go` | `Rate`, `Document`, `Source`; `Embedded()`, `NewSource`, `NewFromEnv`, `Effective`, `RefreshInterval`; validation + fetch. |
| `source_test.go` | Freshest-wins, fallback, validation, TTL, and env-wiring tests. |

## Key exports

- `Embedded() Document` — the compiled-in floor table (validated at `init`; a broken artifact panics at startup).
- `NewSource(embedded, url, refresh)` — testable core. **An empty `url` disables fetching** (never egresses).
- `NewFromEnv(forceDisabled bool)` — reads `PRICING_SOURCE_URL` / `PRICING_REFRESH_INTERVAL`; `forceDisabled` blanks the URL.
- `(*Source).Effective(ctx) Document` — the freshest valid table: a remote document when reachable, valid, and strictly newer than the embedded floor; otherwise the embedded floor (or the last-good remote). Lazy refresh (2h success / 15m failure), keeps last-good, never blocks beyond the request timeout.
- `(*Source).RefreshInterval()` — success TTL, used for the endpoint's `Cache-Control: max-age`.

## Invariants

- **Leaf package.** Must not import `internal/analytics` or `internal/api`. It is app-agnostic: it reads only the `PRICING_*` env vars and takes a `forceDisabled` bool — it does **not** know about `ENABLE_SAAS_FOOTER` (the composition roots pass that in).
- **Freshest-wins, whole-document swap, no merge.** A remote table is adopted only when strictly newer (`updated_at`) than embedded; ties and older remotes keep embedded. Remote can only ever move a backend forward.
- **Tolerant reader.** Unknown JSON fields are dropped; a `schema_version` higher than `maxSchemaVersion` (0) is rejected → embedded. Invalid (malformed, empty, negative/non-finite rate) → embedded/last-good.
- **Never blocks the data path.** Fetch failures fall back; the embedded floor is always valid.

## Wiring

- **API server** (`internal/api`): constructs `NewFromEnv(saasFooterEnabled)`, serves `Effective()` on `/api/v1/pricing`.
- **Worker** (`cmd/server`): constructs `NewFromEnv(ENABLE_SAAS_FOOTER=="true")`, calls `analytics.SetActivePricing(Effective(ctx))` at the top of each precompute cycle so new cards cost out at the freshest prices.
- **confabulous.dev** runs as SaaS → fetch disabled → serves its own embedded table (the root); it never fetches from itself.

## Config

| Env var | Default | Description |
|---------|---------|-------------|
| `PRICING_SOURCE_URL` | `https://confabulous.dev/api/v1/pricing` | Where to pull the freshest table. Set to empty (`""`) to disable fetching and serve the embedded table only (air-gapped). |
| `PRICING_REFRESH_INTERVAL` | `2h` | Success-cache TTL (Go duration). Failures are retried after 15m. |
