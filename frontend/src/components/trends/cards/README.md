# trends/cards/

Trend analytics cards for the Trends dashboard. Each card visualizes aggregated data across multiple sessions over a date range.

## Files

| File | Role |
|------|------|
| `TrendsCard.tsx` | Base card wrapper (`TrendsCard`) and stat row (`StatRow`) shared by all trend cards |
| `TrendsOverviewCard.tsx` | Session count, total/avg duration, assistant utilization |
| `TrendsTokensCard.tsx` | Aggregated token usage and daily cost chart |
| `TrendsActivityCard.tsx` | Code activity totals and daily session count chart |
| `TrendsToolsCard.tsx` | Aggregated tool usage with per-tool success/error breakdown |
| `TrendsUtilizationCard.tsx` | Daily assistant utilization percentage chart |
| `TrendsAgentsAndSkillsCard.tsx` | Aggregated agent and skill invocation counts |
| `TrendsTopSessionsCard.tsx` | Top sessions by cost with links to session detail |
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

- `TrendsOverviewCard` -- session count, duration, utilization
- `TrendsTokensCard` -- token totals, cost, `daily_costs[]`
- `TrendsActivityCard` -- file/line totals, `daily_session_counts[]`
- `TrendsToolsCard` -- tool call totals, `tool_stats` map
- `TrendsUtilizationCard` -- `daily_utilization[]`
- `TrendsAgentsAndSkillsCard` -- agent/skill invocation totals and breakdowns
- `TrendsTopSessionsCard` -- top sessions by cost

## How to Extend

To add a new trends card:

1. Add the card data Zod schema to `TrendsCardsSchema` in `@/schemas/api.ts`
2. Create `TrendsNewCard.tsx` using `TrendsCard` and `StatRow` as building blocks
3. Add a `.stories.tsx` file
4. Export from `index.ts`
5. Render it in `TrendsPage.tsx` with the appropriate data slice

## Invariants / Conventions

- All cards accept `data: T | null` and return `null` when data is absent
- Daily data arrays (costs, session counts, utilization) are rendered as simple bar/line charts using CSS (no charting library)
- Chart styles are shared via `trendsChart.module.css`
- Cards use `@/utils/formatting` for duration/cost formatting, keeping display logic consistent with session cards

## Design Decisions

- **No registry pattern**: Unlike session cards, trends cards are a fixed set rendered directly in `TrendsPage`. The overhead of a registry isn't warranted since new trend cards are rare and the layout is different (full-width sections, not a responsive grid).
- **CSS-only charts**: Daily data visualizations use pure CSS (flexbox bars with percentage heights) rather than a charting library, keeping the bundle lean.
- **Epoch-based date parameters**: The `trendsAPI` converts local YYYY-MM-DD dates to epoch seconds with timezone offset, ensuring correct daily grouping regardless of the user's timezone.

## Testing

Trend cards are covered by Storybook stories (`*.stories.tsx`) for visual regression testing.

## Dependencies

- `@/schemas/api` for card data types
- `@/utils/formatting` for `formatDuration`, `formatModelName`
- `@/utils/tokenStats` for `formatCost`
- `@/components/icons` for stat row icons
- `react-router-dom` (TrendsTopSessionsCard links to session detail pages)
