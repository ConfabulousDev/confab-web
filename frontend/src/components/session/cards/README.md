# cards/

Analytics card components for the session summary panel. Each card visualizes one category of session analytics data. Cards are managed through a central registry pattern.

## Files

| File | Role |
|------|------|
| `registry.ts` | Central card registry -- ordered list of all cards with render conditions |
| `types.ts` | `CardProps<T>`, `CardDefinition` interfaces |
| `Card.tsx` | Shared building blocks: `CardWrapper`, `StatRow`, `CardLoading`, `CardError`, `SectionHeader` |
| `TokensCard.tsx` | Token usage breakdown (input, output, cache) with estimated cost |
| `SessionCard.tsx` | Session metadata: message counts, duration, models, compaction stats |
| `ConversationCard.tsx` | Turn-based metrics: user/assistant turns, avg response time, utilization |
| `CodeActivityCard.tsx` | Code activity: files read/modified, lines added/removed, language breakdown |
| `ToolsCard.tsx` | Tool usage stats: per-tool success/error counts |
| `AgentsAndSkillsCard.tsx` | Agent and skill invocation counts with per-type breakdown |
| `RedactionsCard.tsx` | Redaction counts by type (shown only when redactions exist) |
| `SmartRecapCard.tsx` | AI-generated session recap with actionable suggestions and deep links |
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
8. Redactions (compact, hidden when `total_redactions === 0`)

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

## Dependencies

- `@/schemas/api` for card data types (`TokensCardData`, `SessionCardData`, etc.)
- `@/utils/tokenStats` for cost formatting (`formatCost`)
- `@/utils/formatting` for duration/model formatting
- `@/components/icons` for stat row icons
