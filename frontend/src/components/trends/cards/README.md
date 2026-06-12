# trends/cards/

Trend analytics cards for the Trends dashboard. Each card visualizes aggregated data across multiple sessions over a date range.

## Files

| File | Role |
|------|------|
| `TrendsCard.tsx` | Base card wrapper (`TrendsCard`) and stat row (`StatRow`) shared by all trend cards. `TrendsCard` accepts an optional `headerAction` slot rendered on the right of the header (used by the Costliest Sessions top-N selector), and an optional `caveat` string that renders a subtle `ⓘ` next to the title with the text as its native-`title=` tooltip (2hh1: flags that an active model filter is session-level, so a card's numbers still reflect full-session cost). |
| `TrendsOverviewCard.tsx` | Session count, total/avg duration, total assistant time, token speed (output tokens/sec, CF-525), assistant utilization |
| `TrendsTokensCard.tsx` | Aggregated token usage and daily cost chart. Switches layout on `Object.keys(per_provider).length >= 2` (CF-472, replaces the CF-435 table): multi-provider sets render the elevated grand-total headline (shared `TotalCostRow`, `data-testid="trends-total-cost"`) followed by indented per-provider sections (alphabetical), each headed by the provider label and containing its own Cost / Total Tokens / Input / Output / Cache StatRows — reuses the same StatRow component for visual continuity with single-provider mode and saves horizontal space vs. the prior table. Single-provider sets render the same elevated grand-total headline followed by the StatRow stack (h7xe unified the total treatment across both modes). The tri-state cache row from CF-436 applies in both modes per provider: `Cache (Create / Read)` when create > 0; `Cache Read` when create is 0 and read > 0; omitted when both are 0. `providerHasCacheWrite` suppresses the Create half for providers that structurally have no cache writes (Codex/OpenAI). The daily-cost chart at the bottom is a stacked bar chart: one bar per day, stacked by provider in the same alphabetical order as the sections above, colored by `PROVIDER_METADATA[id].brandColor`. Single-provider mode renders a 1-stack bar (equivalent to a single series). Tooltip shows the day's total plus a per-provider breakdown when more than one provider contributed that day. Driven by `daily_costs[i].per_provider`, with fallback to the cross-provider total when the per-provider breakdown is absent. |
| `TrendsActivityCard.tsx` | Code activity totals and daily session-count chart. Files Read row has three-state behavior driven by `providersPresent` (CF-444): hidden when only `codex` is present (Codex has no Read tool — total would always be 0, mirrors the CF-439 per-session fix), rendered with a small `ⓘ` native-`title=` caveat when mixed Claude+Codex (copy: "Excludes Codex sessions (no Read tool)"), unchanged otherwise. Other rows (Files Modified, Lines Added/Removed) are always present — they aggregate accurately across providers. Sessions per Day chart renders as stacked bars per canonical provider (alphabetical order, `PROVIDER_METADATA[id].brandColor`, falls back to gray for unknown ids), mirroring the Tokens card's stacked-bar pattern. Single-provider windows still render a 1-stack bar. Tooltip shows total + per-provider breakdown when `>= 2` real providers are present. Driven by `daily_session_counts[i].per_provider` with `FALLBACK_STACK_KEY = '__total__'` synthetic series when the wire payload omits per-provider data (older backends). |
| `TrendsToolsCard.tsx` | Aggregated tool usage with per-tool success/error breakdown |
| `TrendsUtilizationCard.tsx` | Daily assistant utilization percentage chart |
| `TrendsAgentsAndSkillsCard.tsx` | Aggregated agent and skill invocation counts |
| `TrendsTopSessionsCard.tsx` | Top sessions by cost with per-row provider icons (Claude / Codex / neutral) and links to session detail. Renders a 10/25/50 segmented top-N selector in the header when an `onTopNChange` handler is supplied (h7xe); `loading` dims the list and disables the control during a refetch. N is URL-synced via `?topN=` on the page and sent to the backend as `?top_n=`. Accepts `modelFilterActive` (2hh1) to show the session-level caveat tooltip. |
| `TrendsCostByModelCard.tsx` | Per-(provider, model-family) cost breakdown (2hh1). One row per `(provider, model)` with a provider icon, `formatModelKey` label (`""` → "Unknown", `"<family> · fast"` suffix preserved), cost in success-green via theme tokens (`$0`/unpriced in warning color), `pct_of_total`, split cache read/write, and session count; sorted cost-desc. A coverage caption ("Covers N of M sessions with per-model data") — **not** a reconciliation line, since the rows (v2, partial) and the Tokens headline (flat, full) are deliberately different scopes. When `data.timed_out` it renders a "narrow your range" notice instead of the empty state; returns `null` when absent or no rows. |
| `TrendsCostDistributionCard.tsx` | Per-session cost histogram + `avg`/p50/p90/p99 summary tiles (y1w5; `avg` added nd9p, rendered first from `data.stats`). Spans 2 grid columns (`.wrapper` → `grid-column: span 2`). Renders a Recharts `BarChart` (8ffa, matching `TrendsTokensCard`): **dynamic log10 bands** — one bar per power of 10 from `$0.01` up to the band containing the priciest session (the backend supplies the `label`, rendered verbatim on a slanted x-axis, `interval={0}` so every band shows). Sub-cent sessions are excluded by the backend (3tr4), so there is no `"< $0.01"` floor bar. An in-card **Sessions / Total $** metric toggle (`MetricToggle`, nd9p — mirrors `TrendsTopSessionsCard`'s selector; local state, defaults to Sessions) flips what bar height encodes: the data-point count (`session_count`) or the band total `$` (a `parseFloat(total_usd)` accessor on the `<Bar>`); the y-axis rescales to each mode's `dataMax`, and the bars fill with the canonical money green (`--color-cost`) in Total $ mode vs. the neutral `--color-accent` in Sessions mode (jysa). The band's other value is surfaced on hover via the exported `CostDistributionTooltip` (band label + count + unit + `formatCostCompact` total, `$2.1M`-style), which leads with whichever metric is active. A `chartLabel` ("Sessions per cost band" / "Session-model pairs per cost band" under a model filter / "Total cost per cost band" in cost mode) names what bar height encodes. A single coverage/backfill caption ("Covers N of M sessions priced ≥ $0.01; percentiles reflect this subset"). Accepts `modelFilterActive` to show the per-(session, model) ⓘ caveat and switch the unit wording (the toggle's button text stays "Sessions"). When `data.timed_out` it renders a "narrow your range" notice; returns `null` when absent or when `covered_session_count === 0`. |
| `trendsChart.module.css` | Shared chart styling for daily data visualizations |
| `index.ts` | Barrel export for all trend card components |

## Key Components

### TrendsCard (base)

Provides the consistent card frame used by all trend cards:
```tsx
<TrendsCard title="Overview" subtitle="7 days" icon={<CalendarIcon />}>
  <StatRow label="Sessions" value={42} />
</TrendsCard>
```

### Data flow

Trend cards receive their data as props from `TrendsPage`. The page fetches data via `useTrends()` hook, which calls `trendsAPI.get()` and returns a `TrendsResponse` containing a `cards` object. Each card receives its slice:

```
TrendsPage -> useTrends() -> TrendsResponse.cards.overview -> TrendsOverviewCard
                                            .cards.tokens  -> TrendsTokensCard
                                            .cards.activity -> TrendsActivityCard
                                            ...
```

Unlike session cards, trends cards do **not** use a registry pattern. They are rendered directly by `TrendsPage` since the set of trend cards is fixed and doesn't need the extensibility of per-session analytics.

## Key Types

All card data types are defined in `@/schemas/api.ts`:

- `TrendsOverviewCard` -- session count, duration, token speed, utilization
- `TrendsTokensCard` -- token totals, cost, `daily_costs[]`
- `TrendsActivityCard` -- file/line totals, `daily_session_counts[]`
- `TrendsToolsCard` -- tool call totals, `tool_stats` map
- `TrendsUtilizationCard` -- `daily_utilization[]`
- `TrendsAgentsAndSkillsCard` -- agent/skill invocation totals and breakdowns
- `TrendsTopSessionsCard` -- top sessions by cost
- `TrendsCostByModelCard` -- per-(provider, model) cost/token breakdown with coverage + timed-out states (2hh1)
- `TrendsCostDistributionCard` -- dynamic log10 per-session cost histogram with a Sessions/Total $ bar-metric toggle + avg/p50/p90/p99 stat tiles, coverage + timed-out states (y1w5, nd9p)

## How to Extend

To add a new trends card:

1. Add the card data Zod schema to `TrendsCardsSchema` in `@/schemas/api.ts`
2. Create `TrendsNewCard.tsx` using `TrendsCard` and `StatRow` as building blocks
3. Add a `.stories.tsx` file
4. Export from `index.ts`
5. Render it in `TrendsPage.tsx` with the appropriate data slice

## Invariants / Conventions

- All cards accept `data: T | null` and return `null` when data is absent
- Data arrays (daily costs, session counts, utilization, cost-distribution bands) are rendered as Recharts `BarChart`/line charts; tooltip and container chrome are shared via `trendsChart.module.css`
- Cards use `@/utils/formatting` for duration/cost formatting, keeping display logic consistent with session cards

## Design Decisions

- **No registry pattern**: Unlike session cards, trends cards are a fixed set rendered directly in `TrendsPage`. The overhead of a registry isn't warranted since new trend cards are rare and the layout is different (full-width sections, not a responsive grid).
- **Recharts for visualizations**: Data charts (Tokens daily cost, Activity, Cost Distribution) use Recharts `BarChart`/`ResponsiveContainer`, with tooltip/container styling shared via `trendsChart.module.css`. Small inline indicators (e.g. percentile tiles, utilization meters) stay plain CSS.
- **Epoch-based date parameters**: The `trendsAPI` converts local YYYY-MM-DD dates to epoch seconds with timezone offset, ensuring correct daily grouping regardless of the user's timezone.

## Testing

Trend cards are covered by Storybook stories (`*.stories.tsx`) for visual regression testing.

## Dependencies

- `@/schemas/api` for card data types
- `@/utils/formatting` for `formatDuration`, `formatModelKey` (tokens_v2 model-key label, shared with the session Tokens v2 card)
- `@/utils/tokenStats` for `formatCost`, `formatTokenCount`, and `formatTokenSpeed` (CF-525)
- `@/utils/providers` for per-row provider icons (`getProviderMetadataOrFallback`)
- `@/components/icons` for stat row icons
- `react-router-dom` (TrendsTopSessionsCard links to session detail pages)
