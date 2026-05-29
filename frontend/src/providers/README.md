# providers/

Per-provider transcript adapters (CF-417). `SessionViewer` and
`SessionHeader` dispatch through this layer instead of branching on
`session.provider`.

## Files

| File | Purpose |
| --- | --- |
| `types.ts` | `ProviderAdapter<TRaw, TItem, TFilterState, TToggles, TCounts>` interface, `FilterAPI`, `TranscriptPaneProps`, `SessionMetaFallback` / `SessionMetaResult`. Two views of the same adapter: `ClaudeAdapter` / `CodexAdapter` (concrete-typed for implementers) and `OpaqueAdapter` (`unknown`-typed for consumers). |
| `claudeAdapter.tsx` | Wraps `transcriptService`, `useTranscriptFilters`, `FilterDropdown`, `ClaudeTranscriptPane`. `supportsTILs: true`. Claude has no separate "raw" stream — `TranscriptLine[]` doubles as both `TRaw` and `TItem`, with `normalize` as the identity function. |
| `codexAdapter.tsx` | Wraps `codexTranscriptService`, `useCodexTranscriptFilters`, `CodexFilterDropdown`, `CodexTranscriptPane`. `supportsTILs: false`. `computeMeta` walks rawLines for min/max `timestamp`. |
| `registry.ts` | `getAdapter(provider: string): OpaqueAdapter`. Normalizes `provider` (lowercase, whitespace → `-`), then looks up in a record keyed by `PROVIDER_VALUES`. **Throws on unknown providers** — backend already normalizes on read, so this only fires on a backend-first rollout. |
| `useTranscriptData.ts` | Shared hook: initial fetch + visibility-gated polling. Single hook, both providers. Skipped when a Storybook `seed` is supplied. |
| `useSessionTILs.ts` | Shared hook: fetches TILs when `adapter.supportsTILs === true`; returns an empty Map otherwise. |
| `registry.test.ts` | Drift guard: every `PROVIDER_VALUES` entry must resolve to a distinct adapter; unknown providers must throw. |
| `claudeAdapter.test.ts` / `codexAdapter.test.ts` | Per-adapter delegation + pure-method tests. Services are mocked with `vi.mock`. |

## `ProviderAdapter` interface

```ts
interface ProviderAdapter<TRaw, TItem, TFilterState, TToggles, TCounts> {
  readonly id: ProviderId;
  readonly supportsTILs: boolean;
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
   frontend pricing edit needed.
3. Write `frontend/src/providers/<id>Adapter.tsx`:
   - Type it as `ProviderAdapter<TRaw, TItem, TFilterState, TToggles, TCounts>`.
   - Wrap an existing transcript service, filter hook, dropdown component, and pane component.
   - Decide `supportsTILs`; pick `useDeepLinkFilterReset` semantics.
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
