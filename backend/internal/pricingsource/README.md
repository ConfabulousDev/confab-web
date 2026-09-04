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
| `pricing.json` | The single source of truth: `{ schema_version, updated_at, pricing }`, provider-nested (`claude-code` / `codex` / `opencode` → family → rates, USD per million tokens). Each rate has `input`, `output`, `cacheWrite` (5-minute cache writes), `cacheWrite1h` (1-hour cache writes, 2x input; `0` ⇒ consumers fall back to `cacheWrite`), and `cacheRead`. **Edit this and bump `updated_at` to change a price.** `cacheWrite1h` is additive/optional — do **not** bump `schema_version` for it. |
| `source.go` | `Rate`, `Document`, `Source`; `Embedded()`, `NewSource`, `NewFromEnv`, `Effective`, `RefreshInterval`; validation + fetch. |
| `source_test.go` | Freshest-wins, fallback, validation, TTL, and env-wiring tests. |

## Key exports

- `Embedded() Document` — the compiled-in floor table (validated at `init`; a broken artifact panics at startup).
- `NewSource(embedded, url, refresh)` — testable core. **An empty `url` disables fetching** (never egresses).
- `NewFromEnv(forceDisabled bool)` — reads `PRICING_SOURCE_URL` / `PRICING_REFRESH_INTERVAL`; `forceDisabled` blanks the URL.
- `(*Source).Effective(ctx) Document` — the freshest valid table: a remote document when reachable, valid, and strictly newer than the embedded floor; otherwise the embedded floor (or the last-good remote). Lazy refresh (2h success / 15m failure), keeps last-good, never blocks beyond the request timeout.
- `(*Source).RefreshInterval()` — success TTL, used for the endpoint's `Cache-Control: max-age`.

## Rate conventions

`Rate` is a single flat set of numbers per family (`input`, `output`,
`cacheWrite`, `cacheWrite1h`, `cacheRead`). Real vendor pricing carries
dimensions that shape cannot express, so the table follows fixed conventions.
Read these before adding or repricing a row.

- **Never encode a future price change.** A rate that is scheduled to change,
  and a promotional rate with a known expiry, are both stored as the flat rate
  in effect *today*. Put a calendar reminder to edit the table on the date; do
  not write code that switches price on a timer. We shipped exactly that once —
  a Sonnet 5 increase encoded ahead of time, then cancelled by the vendor, which
  overcharged every Sonnet 5 session by 50% from the day the cutover fired.
  Rates currently promotional: `gpt-5.6-sol` ($4/$20, ~Nov 21 2026) and
  `gemini-3.6/3.7/3.8-flash` ($0.75/$3.75 through Dec 31 2026, reverting to
  $1.50/$7.50 with a $0.15 cache read).
- **Context-tiered models take the low/short-context tier.** Every Gemini Pro,
  every Grok model, and the OpenAI long-context tiers bill more above a token
  threshold (200k for Gemini/Grok, 272k for OpenAI). The family key derives from
  the model name, so nothing in the compute path could pick a tier per request.
  The ≤200k / short-context rate is stored, which understates requests above the
  threshold — sharpest on `gpt-6-astra`, whose 1.05M context makes >272k requests
  routine and where the long-context tier is 2x input / 1.5x output (and is the
  only OpenAI tier that bills cache writes, at $25/MTok). Fixing the class needs
  per-turn token accounting; tracked separately.
- **Cache reads are 0.1x base input — except Fable 5.1 and Mythos 5.1.** Those
  two bill cache hits at 0.025x ($0.25/MTok against $10 input). The 5.0
  generation (`fable-5`, `mythos-5`) stays at 0.1x ($1.00). Cache reads dominate
  token volume in agentic sessions, so "normalizing" the 5.1 rows to $1.00 is a
  silent 4x overcharge. Pinned by a test in `internal/analytics/pricing_test.go`.
- **DeepSeek V4 is stored at off-peak rates.** DeepSeek bills peak during
  01:00–04:00 and 06:00–10:00 UTC Mon–Fri and half that otherwise — 35 of 168
  hours, so off-peak covers ~79% of wall-clock time and is the better
  single-value estimate. Peak equivalents are exactly 2x: `deepseek-v4-flash`
  $0.44/$1.32 with a $0.014 cache read, `deepseek-v4-pro` $1.32/$3.96 with
  $0.044.
- **Mistral rows carry `cacheRead: 0`.** Mistral advertises cached input at up to
  90% off but publishes no per-model cached rate, so cache hits bill at the full
  input rate. That errs high, never low.
- **Gemini Flash rows take the text/image/video tier.** Audio input is priced
  higher and the transcript gives us no way to tell the modalities apart.
- **Retired models keep their rows.** Historical sessions still reference them,
  and deleting a key silently reprices those sessions to $0 rather than failing
  loudly.
- **Keys must match the wire string byte for byte.** For non-Claude, non-OpenAI
  names `getModelFamily` passes the model id straight through, so a key that
  misspells the vendor's API model id is invisible: no error, no distinct
  warning, the model just stays unpriced at $0. Use the exact API model id from
  vendor docs (`mistral-medium-3504`, not `mistral-medium-3.5`). After adding
  rows, check `GET /api/v1/admin/unpriced-models` — anything still listed there
  is a spelling mismatch, not a missing price.

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
