# providers/

Per-provider transcript adapters (CF-417). `SessionViewer` and
`SessionHeader` dispatch through this layer instead of branching on
`session.provider`.

## Files

| File | Purpose |
| --- | --- |
| `types.ts` | `ProviderAdapter<TRaw, TItem, TFilterState, TToggles, TCounts>` interface, `FilterAPI`, `TranscriptPaneProps` (incl. optional `firstSeen`/`lastSyncAt` session bounds, read only by the Cursor pane for estimated row timestamps — ce79), `SessionMetaFallback` / `SessionMetaResult`. Two views of the same adapter: `ClaudeAdapter` / `CodexAdapter` (concrete-typed for implementers) and `OpaqueAdapter` (`unknown`-typed for consumers). |
| `claudeAdapter.tsx` | Wraps `claudeTranscriptService`, `useClaudeTranscriptFilters`, `ClaudeFilterDropdown`, `ClaudeTranscriptPane`. Claude has no separate "raw" stream — `TranscriptLine[]` doubles as both `TRaw` and `TItem`, with `normalize` as the identity function. |
| `codexAdapter.tsx` | Wraps `codexTranscriptService`, `useCodexTranscriptFilters`, `CodexFilterDropdown`, `CodexTranscriptPane`. `computeMeta` walks rawLines for min/max `timestamp`. |
| `opencodeAdapter.tsx` | Wraps `opencodeTranscriptService`, `useOpenCodeTranscriptFilters`, `OpenCodeFilterDropdown`, `OpenCodeTranscriptPane`. `computeMeta` walks render-items' epoch-ms `timeCreated`; `calculateMessageCost` prefers the reported `info.cost`, else the pricing fallback. |
| `cursorAdapter.tsx` | Wraps `cursorTranscriptService`, `useCursorTranscriptFilters`, `CursorFilterDropdown`, `CursorTranscriptPane`. Cursor JSONL carries no model/token/cost/timestamp, so `extractModel` is always `undefined`, `computeMeta` always falls back to `firstSeen`/`lastSyncAt`, `tokensMeasurable: false` (st5f — Summary/Trends show "Not available" instead of $0), and `calculateMessageCost` returns 0. With no per-message timestamps it also sets `conversationTimingUnavailableNote` (zsk4), which the Conversation card renders as a muted footer explaining the absent timing/utilization rows. The pane has a turn-based `TimelineBar` minimap (zztp, sized from the estimated row times — self-hides when bounds are unknown) but no cost rail; it derives its own segment greying from `items` vs `filteredItems`, so the adapter's `TranscriptPane` wrapper accepts but ignores the contract's `visibleIndices` / `isCostMode`, and forwards `firstSeen`/`lastSyncAt` so the pane can estimate per-row timestamps (ce79) that also size the minimap. |
| `registry.ts` | `getAdapter(provider: string): OpaqueAdapter` and `isTokensMeasurable(providerId)` (st5f). Normalizes `provider` (lowercase, whitespace → `-`), then looks up in a record keyed by `PROVIDER_VALUES`. **Throws on unknown providers** — backend already normalizes on read, so this only fires on a backend-first rollout. `isTokensMeasurable` returns `false` only for adapters with `tokensMeasurable: false` (Cursor today); unknown ids default to `true`. |
| `useTranscriptData.ts` | Shared hook: initial fetch + visibility-gated polling. Single hook, both providers. Skipped when a Storybook `seed` is supplied. |
| `registry.test.ts` | Drift guard: every `PROVIDER_VALUES` entry must resolve to a distinct adapter; unknown providers must throw. |
| `claudeAdapter.test.ts` / `codexAdapter.test.ts` / `opencodeAdapter.test.ts` / `cursorAdapter.test.ts` | Per-adapter delegation + pure-method tests. Services are mocked with `vi.mock`. |

## `ProviderAdapter` interface

```ts
interface ProviderAdapter<TRaw, TItem, TFilterState, TToggles, TCounts> {
  readonly id: ProviderId;
  fetchInitial(sessionId, fileName, skipCache?): Promise<{ items, totalLines, raw }>;
  fetchIncremental(sessionId, fileName, currentLineCount): Promise<{ newItems, newRaw, newTotalLineCount }>;
  normalize(raw): TItem[];
  extractModel(raw, items): string | undefined;
  computeMeta(items, raw, fallback): SessionMetaResult;
  useFilters(): FilterAPI<TFilterState, TToggles>;
  countCategories(items): TCounts;
  itemMatchesFilter(item, state): boolean;
  useDeepLinkFilterReset(items, targetId, filters): void;  // hook-on-adapter
  // CF-418: provider-specific cost adjustments. Base `calculateCost` is in
  // utils/tokenStats; the adapter applies fast multiplier / server-tool
  // dollars (Claude) or just returns base arithmetic (Codex).
  calculateMessageCost(model, usage: TokenUsage, message: TItem): number;
  extendCostTooltip?(base: string[], usage: TokenUsage, message: TItem): string[];
  // CF-436: static tooltip strings for the per-session Tokens summary card.
  // Claude defines both; Codex defines only `tokensCostTooltip` (no fast tier).
  readonly tokensCostTooltip: string;
  readonly tokensFastTooltip?: string;
  readonly tokenSpeedUnavailableTooltip?: string;          // st5f — Conversation "Token speed" tooltip when unmeasured
  readonly conversationTimingUnavailableNote?: string;     // zsk4 — Conversation card footer when no per-turn timing
  FilterDropdown: FC<{ counts; filters }>;
  TranscriptPane: FC<TranscriptPaneProps>;
}
```

### Why two views (`ClaudeAdapter` / `CodexAdapter` vs `OpaqueAdapter`)

Each adapter file types its literal against the concrete-typed alias so its
closures stay self-checked at compile time. The registry widens once to
`OpaqueAdapter` (all `unknown`s) so `SessionViewer` never narrows. Items flow
opaquely from `fetchInitial` through `itemMatchesFilter` and out to
`TranscriptPane`; the registry guarantees adapter and items came from the
same provider, so the runtime cast is safe. The widening cast in
`registry.ts` is the one approved boundary — see the file-level
`eslint-disable` block.

### Why `useDeepLinkFilterReset` is a hook-on-adapter

The two providers identify deep-link targets differently (Claude: message
UUID; Codex: ISO 8601 timestamp resolved by `resolveCodexDeepLinkTarget` —
CF-475) and reset different filter categories when the target is hidden
(Claude: `system`; Codex: `reasoning_hidden`). Putting the provider-specific
find + reset logic on the adapter keeps SessionViewer agnostic. The hook is
called as `adapter.useDeepLinkFilterReset(...)` — React's rules-of-hooks
plugin accepts property-access calls whose last segment starts with `use`.

## Adding a third provider

1. Register the canonical id in `PROVIDER_VALUES` (Phase 1 / `utils/providers.ts`).
2. Add a `'<id>'` provider block with its model families to the single price
   table, `backend/internal/pricingsource/pricing.json` (CF-515), and bump
   `updated_at`. The frontend reads the table from the backend at runtime — no
   frontend pricing edit needed. **Even when the provider has no pricing**
   (e.g. Cursor, whose transcripts carry no token/cost data), every frontend
   `Record<ProviderId, …>` literal still needs a `'<id>'` key to satisfy `tsc`:
   `activePricing` (`utils/tokenStats.ts`), `PRICING_FIXTURE`
   (`test/pricingFixture.ts`), `PROVIDER_METADATA` (`utils/providers.ts`),
   `REGISTRY` (`providers/registry.ts`), and the `DEFAULTS_BY_PROVIDER` maps in
   `test-fixtures/session.ts` and `test-fixtures/org.ts` — supply an empty `{}`
   for the pricing maps.
3. Write `frontend/src/providers/<id>Adapter.tsx`:
   - Type it as `ProviderAdapter<TRaw, TItem, TFilterState, TToggles, TCounts>`.
   - Wrap an existing transcript service, filter hook, dropdown component, and pane component.
    - Decide `useDeepLinkFilterReset` semantics.
   - Implement `calculateMessageCost(model, usage, message)` (typically just
     `calculateCost('<id>', model, usage)` plus any provider-specific
     adjustments) and an `extendCostTooltip` if the tooltip needs extra lines.
   - Supply `tokensCostTooltip` (and `tokensFastTooltip` if the provider has a
     fast/priority tier). These render as the title attribute on the
     per-session Tokens summary card's "Estimated cost" / "Fast mode" rows.
4. Register the adapter in `registry.ts`'s `REGISTRY` map (one entry, one widening cast).
5. Run `registry.test.ts` to confirm the drift guard accepts the new id.
6. Add a `DEFAULTS_BY_PROVIDER` entry in `frontend/src/test-fixtures/session.ts`
   so `makeSessionFixture('<id>')` / `makeSessionDetailFixture('<id>')` produce
   sensible default test data; also extend `utils/providers.ts` cosmetic metadata.

`SessionViewer.tsx` and `SessionHeader.tsx` should require **zero edits**.

## Invariants

- `session.provider` is constant across the lifetime of a `SessionViewer`
  mount. SessionViewer calls `adapter.useFilters()` and other adapter hooks
  unconditionally; switching providers mid-render would break the rules of
  hooks. The session-detail route already keys SessionViewer per session, so
  this holds in practice.
- `getAdapter()` is a synchronous, pure lookup. Calling it inside `useMemo`
  is unnecessary — the adapter reference is referentially stable per
  module-load.
- `OpaqueAdapter` and `ClaudeAdapter` / `CodexAdapter` describe the same
  runtime object; the widening cast in `registry.ts` is the only place
  TypeScript needs to bridge the two views.

## Out of scope (handled elsewhere)

- Cosmetic per-provider strings (label, icon, brand color, copy-id menu) —
  see `frontend/src/utils/providers.ts` (CF-416).
- Canonical `TokenUsage` shape, pricing table, base `calculateCost` — see
  `frontend/src/utils/tokenStats.ts` (CF-418). The adapter's
  `calculateMessageCost` / `extendCostTooltip` build on those primitives.
- Backend provider identity — see `backend/internal/models/provider.go`
  (CF-401).
