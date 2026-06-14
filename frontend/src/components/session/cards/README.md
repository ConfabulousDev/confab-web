# cards/

Analytics card components for the session summary panel. Each card visualizes one category of session analytics data. Cards are managed through a central registry pattern.

## Files

| File | Role |
|------|------|
| `registry.ts` | Central card registry -- ordered list of all cards with render conditions |
| `types.ts` | `CardProps<T>`, `CardDefinition` interfaces |
| `Card.tsx` | Shared building blocks: `CardWrapper`, `StatRow`, `CardLoading`, `CardError`, `SectionHeader` |
| `TokensCard.tsx` | Flat token usage breakdown (input, output, cache) with estimated cost. Provider-aware via `getAdapter(provider)`: cost / fast-mode tooltips come from `tokensCostTooltip` / `tokensFastTooltip` on the adapter. "Cache created" row hidden when value is 0 (CF-436). Direct callers pass required `provider`; the registry uses `TokensCardForRegistry`. **Superseded by `TokensV2Card` whenever `tokens_v2` is present** (`SessionSummaryPanel` suppresses this card); it remains the fallback for sessions whose v2 tree hasn't been computed yet. |
| `TokensV2Card.tsx` | Per-model token + cost breakdown from the `tokens_v2` tree (all providers as of 7eje). Layout mirrors the Trends "Tokens & Cost" card so the two token cards read consistently (hp73): an elevated bordered grand-total headline ("Estimated cost" + large `<CostAmount>`, carrying the `$0`/unpriced amber warning), then a summary stack ("Total Tokens" + a combined "Input / Output" row), then the provider/model tree rendered as indentation depth. Multi-provider: grouped `<section>`s with an uppercase `.providerHeader` (distinct from the model headline) + a left-indented `.providerRows` stack (per-provider cost row + model sections). Single-provider sessions (Claude/Codex; OpenCode single-vendor) drop the provider wrapper and render model sections directly under the totals. Each model section is a collapsible disclosure (d3rp): the headline is a full-width button (`aria-expanded`/`aria-controls`, keyboard-operable, rotating `ChevronIcon`) showing the formatted model label + its `<CostAmount>`, deliberately subordinate to the provider header (lighter weight, not uppercase); expanding reveals the deeper-indented Input / Output / Cache read / Cache write / Reasoning detail rows (cache/reasoning gated on `>0`). Cost stays in the always-visible headline. A card with exactly one model section total auto-expands it; with several, all start collapsed (local `useState`, no persistence). Model keys are formatted via the shared `formatModelDisplayName` (mp4e); `"<family> · fast"` keeps its suffix, `""` renders as "Unknown". CSS classes (`.totalCostRow`/`.providerHeader`/`.providerRows`/`.modelDetail`) live in `SessionSummaryPanel.module.css`, copied from `TrendsTokensCard.module.css`. |
| `SessionCard.tsx` | Session metadata: message counts, duration, models (via shared `formatModelDisplayName`), compaction stats |
| `ConversationCard.tsx` | Turn-based metrics: user/assistant turns, avg response time, token speed (output tokens/sec, CF-525), utilization. `tokenSpeed` is precomputed by `SessionSummaryPanel` and injected via `extraProps` (it needs the Tokens card's `output` plus this card's `total_assistant_duration_ms`), so the card stays presentational. |
| `CodeActivityCard.tsx` | Code activity: files read/modified, lines added/removed, language breakdown |
| `ToolsCard.tsx` | Tool usage stats: per-tool success/error counts. Exports `prepareChartData` for testing. Defensively filters orphan `<unknown>` entries so a literal `<unknown>` bar never paints, even for stale ComputeResults predating the CF-438 backend skip. |
| `AgentsAndSkillsCard.tsx` | Agent and skill invocation counts with per-type breakdown. Provider-agnostic copy: Claude buckets by `subagent_type`, Codex (CF-443) buckets by `agent_role` (`"default"`, `"explorer"`). Renders for both providers via the registry's `agent_invocations + skill_invocations > 0` gate. |
| `RedactionsCard.tsx` | Redaction counts by type (shown only when redactions exist) |
| `WorkflowsCard.tsx` | Per-run workflow subagent aggregates (CF-534): one row per run labelled `Run 1…N` (opaque `run_id` in hover title), showing agent count, a token subtotal, cost, an activity-span duration, and a `succeeded/total completed` count when the run journal was uploaded (`has_journal`). Backend-sourced (`cards.workflows`); hidden when there are no runs. |
| `SmartRecapCard.tsx` | AI-generated session recap with actionable suggestions and deep links. `MessageLink` short-circuits when `item.message_id` is empty — this is the intentional state for Codex sessions (Codex rollout JSONL has no stable per-message id; the backend `PrepareCodexTranscript` synthesizes ids only for the LLM's internal use, and `codexProvider.ClearMessageIDs()` zeroes them before the card is saved). Claude sessions render the icon link; Codex sessions render plain text. |
| `index.ts` | Barrel export: `getOrderedCards()` |

## Key Types

```typescript
interface CardProps<T> {
  data: T | null;      // Card data from API, null if not loaded
  loading: boolean;    // Whether data is being fetched
  error?: string;      // Error message if computation failed
}

interface CardDefinition {
  key: keyof AnalyticsCards;  // Must match backend cards map key
  title: string;
  component: React.ComponentType<CardProps<any>>;
  order: number;              // Lower = rendered earlier
  span?: 1 | 2 | 3 | 'full'; // Grid columns to span
  size?: 'compact' | 'standard' | 'tall';
  shouldRender?: (data: any) => boolean;  // Gate rendering
}
```

## Card Registry Pattern

All cards are registered in `registry.ts` as an ordered array of `CardDefinition` objects. `SessionSummaryPanel` calls `getOrderedCards()` to get the sorted list, then iterates over it to render each card.

**Current card order:**
1. Smart Recap (`span: 'full'`, always rendered -- handles no-data states internally)
2. Tokens (standard)
3. Session (standard)
4. Conversation (compact)
5. Code Activity (standard)
6. Tools (`span: 2`, tall, hidden when `total_calls === 0`)
7. Agents and Skills (`span: 2`, tall, hidden when no invocations)
8. Workflows (`span: 2`, hidden when `runs.length === 0`)
9. Redactions (compact, hidden when `total_redactions === 0`)

Cards with `shouldRender` returning false are not rendered at all (no empty grid cell).

## How to Extend

To add a new analytics card:

1. Define the card data Zod schema in `@/schemas/api.ts` and add it to `AnalyticsCardsSchema`
2. Create `NewCard.tsx` in this directory, implementing `CardProps<NewCardData>`
3. Use `CardWrapper` and `StatRow` from `Card.tsx` for consistent styling
4. Add the card to `cardRegistry` in `registry.ts` with appropriate `order`, `span`, `size`, and optional `shouldRender`
5. Create a `.stories.tsx` file with representative data
6. The `SessionSummaryPanel` will render it automatically -- no changes needed there

See the `/add-session-card` skill for a full step-by-step playbook including backend changes.

## Invariants / Conventions

- Card `key` values must match the keys in the backend `AnalyticsCards` response map exactly
- Cards receive `null` data during loading and must handle that state (typically via `CardLoading`)
- Cards may receive an `error` string if computation failed on the backend; use `CardError` for display
- The `SmartRecapCard` is special: it handles quota-exceeded and unavailable states internally and may return `null` to hide itself, so it has no `shouldRender` gate in the registry
- All cards use `CardWrapper` and `StatRow` from `Card.tsx` for visual consistency

## Design Decisions

- **Registry pattern over manual rendering**: Adding a card requires only a component file and a registry entry. No changes to `SessionSummaryPanel`.
- **`shouldRender` gates**: Cards like Tools and Redactions are hidden entirely when data is empty, preventing distracting empty cards in the grid.
- **Graceful degradation**: The `card_errors` field in the analytics response allows individual cards to fail without breaking the entire summary panel. Each card shows its own error state.

## Testing

- `Card.test.tsx` -- `CardWrapper`, `StatRow`, `CardLoading` rendering
- `TokensCard.test.tsx` -- Token formatting, cost display
- `SmartRecapCard.test.tsx` -- Recap display, quota exceeded state, deep link handling
- `registry.test.ts` -- Registry ordering, `shouldRender` logic
- `SessionCard.test.tsx` -- Duration/model/messages formatting, compaction rows
- `ConversationCard.test.tsx` -- Per-field nullability and duration formatting
- `ToolsCard.test.tsx` -- Subtitle pluralization, empty-state hiding, tooltip payload
- `AgentsAndSkillsCard.test.tsx` -- Loading/error/empty paths, agent+skill legend
- `RedactionsCard.test.tsx` -- Sort-by-count ordering, singular/plural tooltip
- `WorkflowsCard.test.tsx` -- Per-run rows, agent-count pluralization, journal status / no-journal, empty + loading states
- `CodeActivityCard.test.tsx` -- Stat rows and conditional File extensions section

Chart-based card tests run under a global `recharts` mock in `src/test/setup.ts`
that invokes inline `tickFormatter` and `Tooltip.content` callbacks with a
synthetic payload so per-card `CustomTooltip` logic is exercised.

## Dependencies

- `@/schemas/api` for card data types (`TokensCardData`, `SessionCardData`, etc.)
- `@/utils/tokenStats` for cost formatting (`formatCost`) and token-speed formatting (`formatTokenSpeed`, CF-525)
- `@/utils/formatting` for duration/model formatting
- `@/components/icons` for stat row icons
