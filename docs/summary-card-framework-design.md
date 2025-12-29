# Summary Card Framework Design

A unified architecture for analytics cards in the Summary tab, enabling efficient addition of new metrics with minimal code changes.

**Date:** December 2024
**Status:** Design proposal

---

## Goals

1. **Single poll, N cards** - One API request fetches data for all cards
2. **Easy card addition** - Adding a new card requires:
   - Backend: Add computation + key to response
   - Frontend: Register card + build UI component
3. **Efficient updates** - Conditional requests (304 Not Modified) continue to work
4. **Future-proof** - Support for freemium gating, conditional cards, visualizations

---

## Current State

### Backend
- `GET /api/v1/sessions/{id}/analytics` returns flat response
- Computes: tokens, cost, compaction in single JSONL pass
- Has unused `Details map[string]interface{}` for expansion
- Supports conditional requests via `as_of_line` parameter

### Frontend
- `useAnalyticsPolling` hook fetches analytics
- `SessionSummaryPanel` renders 3 hardcoded cards inline
- Cards are not componentized or registered

---

## Proposed Architecture

### API Response Structure

```typescript
// New response shape - keyed by card
interface SummaryResponse {
  computed_at: string;
  computed_lines: number;

  cards: {
    tokens: TokensCardData;
    cost: CostCardData;
    compaction: CompactionCardData;
    session: SessionCardData;        // NEW
    efficiency: EfficiencyCardData;  // NEW
    tools: ToolsCardData;            // NEW
    // ... future cards
  };
}

// Example card data shapes
interface TokensCardData {
  input: number;
  output: number;
  cache_creation: number;
  cache_read: number;
}

interface SessionCardData {
  duration_ms: number | null;      // first → last timestamp
  turn_count: number;              // user/assistant pairs
  user_messages: number;
  assistant_messages: number;
  first_timestamp: string | null;
  last_timestamp: string | null;
}

interface EfficiencyCardData {
  cache_hit_rate: number;          // cache_read / (input + cache_read)
  output_input_ratio: number;      // output / input
  tokens_per_turn: number;         // total tokens / turns
  cost_per_turn_usd: string;       // cost / turns
}

interface ToolsCardData {
  total_invocations: number;
  unique_tools: number;
  by_category: Record<string, number>;  // { "file_ops": 45, "search": 12, ... }
  top_tools: Array<{ name: string; count: number }>;
  error_count: number;
  success_rate: number;
}
```

### Backend Changes

```go
// analytics/models.go - new response structure
type SummaryResponse struct {
    ComputedAt    time.Time         `json:"computed_at"`
    ComputedLines int64             `json:"computed_lines"`
    Cards         map[string]any    `json:"cards"`
}

// Each card's data is computed and added to Cards map
// Example:
// response.Cards["tokens"] = TokensCardData{...}
// response.Cards["session"] = SessionCardData{...}
```

The existing `ComputeFromJSONL` function would be extended to compute all metrics in a single pass through the JSONL data.

### Frontend Card Registry

```typescript
// types/summaryCards.ts
interface CardDefinition<T> {
  key: string;                           // Matches backend key
  title: string;
  icon?: React.ReactNode;
  component: React.ComponentType<CardProps<T>>;
  defaultEnabled: boolean;
  order: number;                         // Display order

  // Optional: conditions for showing/hiding
  shouldShow?: (data: T | null, session: SessionDetail) => boolean;
}

interface CardProps<T> {
  data: T | null;
  loading: boolean;
  error: Error | null;
}

// Registry
const cardRegistry: CardDefinition<any>[] = [
  {
    key: 'session',
    title: 'Session',
    component: SessionCard,
    defaultEnabled: true,
    order: 0,
  },
  {
    key: 'tokens',
    title: 'Tokens',
    component: TokensCard,
    defaultEnabled: true,
    order: 1,
  },
  {
    key: 'cost',
    title: 'Cost',
    component: CostCard,
    defaultEnabled: true,
    order: 2,
  },
  {
    key: 'efficiency',
    title: 'Efficiency',
    component: EfficiencyCard,
    defaultEnabled: true,
    order: 3,
  },
  {
    key: 'tools',
    title: 'Tools',
    component: ToolsCard,
    defaultEnabled: true,
    order: 4,
    shouldShow: (data) => data !== null && data.total_invocations > 0,
  },
  {
    key: 'compaction',
    title: 'Compaction',
    component: CompactionCard,
    defaultEnabled: true,
    order: 5,
  },
];
```

### Frontend Hook

```typescript
// hooks/useSummaryPolling.ts
interface UseSummaryPollingReturn {
  cards: Record<string, any>;  // Card data keyed by card key
  loading: boolean;
  error: Error | null;
  computedAt: string | null;
  refetch: () => Promise<void>;
}

function useSummaryPolling(sessionId: string): UseSummaryPollingReturn {
  // Single fetch for all card data
  // Manages conditional requests via computed_lines
  // Distributes data to cards via the cards map
}
```

### Frontend Panel Component

```typescript
// components/session/SessionSummaryPanel.tsx
function SessionSummaryPanel({ sessionId, isOwner }: Props) {
  const { cards, loading, error, computedAt } = useSummaryPolling(sessionId);

  // Filter to enabled cards, sorted by order
  const enabledCards = cardRegistry
    .filter(def => def.defaultEnabled)
    .filter(def => !def.shouldShow || def.shouldShow(cards[def.key], session))
    .sort((a, b) => a.order - b.order);

  return (
    <div className={styles.panel}>
      <Header computedAt={computedAt} />
      <div className={styles.grid}>
        <GitHubLinksCard ... />

        {enabledCards.map(def => {
          const CardComponent = def.component;
          return (
            <CardComponent
              key={def.key}
              data={cards[def.key] ?? null}
              loading={loading}
              error={error}
            />
          );
        })}
      </div>
    </div>
  );
}
```

---

## Design Decisions

### Decision 1: Backend vs Frontend Computation

**Options:**
1. **All backend** - Backend computes all metrics from JSONL
2. **Hybrid** - Backend computes from JSONL, frontend computes derived metrics
3. **All frontend** - Backend sends raw data, frontend computes everything

**Recommendation: Option 1 (All backend)**

Rationale:
- Single JSONL parse is efficient
- Results are cacheable (already cached in DB)
- Frontend doesn't need transcript data loaded for analytics
- Consistent computation across clients
- Easier to add premium-only metrics (backend can gate)

### Decision 2: Card Key Selection

**Options:**
1. **Frontend specifies keys** - `GET /analytics?cards=tokens,cost,session`
2. **Backend returns all** - Frontend filters on display

**Recommendation: Option 2 (Backend returns all)**

Rationale:
- Simpler implementation
- Single cache key per session
- Freemium gating can happen at frontend level initially
- If needed later, backend can gate premium keys

Note: For future premium gating, we could add a `tier` field to card definitions and filter based on user subscription level.

### Decision 3: Conditional Cards

Some cards only make sense for certain sessions:
- **Tools card**: Only if tool calls exist
- **Tasks card**: Only if TodoWrite was used
- **Thinking card**: Only if thinking blocks exist

**Recommendation: `shouldShow` callback**

Each card definition includes an optional `shouldShow(data, session)` function. The panel only renders cards where this returns true (or is undefined).

### Decision 4: Card Ordering

**Options:**
1. **Fixed order** - Defined in registry
2. **User configurable** - Stored in preferences

**Recommendation: Fixed order initially**

Start with fixed order defined in registry. User preferences can be added later if there's demand.

---

## Migration Path

### Phase 1: Backend Restructure
1. Refactor `AnalyticsResponse` to use `cards` map structure
2. Keep existing flat fields for backward compatibility during transition
3. Add new card data computations in `ComputeFromJSONL`

```go
// Temporary: support both old and new formats
type AnalyticsResponse struct {
    // Old format (deprecated, for backward compat)
    Tokens     TokenStats     `json:"tokens"`
    Cost       CostStats      `json:"cost"`
    Compaction CompactionInfo `json:"compaction"`

    // New format
    Cards map[string]any `json:"cards"`
}
```

### Phase 2: Frontend Framework
1. Create card registry and types
2. Create `useSummaryPolling` hook
3. Migrate existing cards to component pattern
4. Update `SessionSummaryPanel` to use registry

### Phase 3: New Cards
1. Add new computations to backend (session, efficiency, tools)
2. Create frontend card components
3. Register in card registry

### Phase 4: Cleanup
1. Remove deprecated flat fields from backend response
2. Update frontend to only use `cards` map

---

## Adding a New Card: Checklist

### Backend
1. [ ] Define card data struct in `analytics/models.go`
2. [ ] Add computation logic in `analytics/compute.go` (extend `ComputeFromJSONL`)
3. [ ] Add card to `Cards` map in response builder
4. [ ] Update `API.md` documentation

### Frontend
1. [ ] Add TypeScript type for card data in `schemas/api.ts`
2. [ ] Create card component in `components/session/cards/`
3. [ ] Add Storybook story for card
4. [ ] Register card in `cardRegistry`
5. [ ] Write tests for card component

---

## Example: Adding "Session Overview" Card

### Backend (compute.go)

```go
type SessionCardData struct {
    DurationMs       *int64 `json:"duration_ms"`
    TurnCount        int    `json:"turn_count"`
    UserMessages     int    `json:"user_messages"`
    AssistantMessages int   `json:"assistant_messages"`
    FirstTimestamp   string `json:"first_timestamp,omitempty"`
    LastTimestamp    string `json:"last_timestamp,omitempty"`
}

// In ComputeFromJSONL:
var firstTimestamp, lastTimestamp time.Time
var userCount, assistantCount int

for scanner.Scan() {
    line, _ := ParseLine(scanner.Bytes())

    // Track timestamps
    if ts, err := line.GetTimestamp(); err == nil {
        if firstTimestamp.IsZero() || ts.Before(firstTimestamp) {
            firstTimestamp = ts
        }
        if ts.After(lastTimestamp) {
            lastTimestamp = ts
        }
    }

    // Count message types
    if line.Type == "user" {
        userCount++
    } else if line.IsAssistantMessage() {
        assistantCount++
    }
}

// Add to result
result.Cards["session"] = SessionCardData{
    TurnCount:        min(userCount, assistantCount),
    UserMessages:     userCount,
    AssistantMessages: assistantCount,
    // ... etc
}
```

### Frontend (SessionCard.tsx)

```typescript
interface SessionCardData {
  duration_ms: number | null;
  turn_count: number;
  user_messages: number;
  assistant_messages: number;
  first_timestamp: string | null;
  last_timestamp: string | null;
}

function SessionCard({ data, loading }: CardProps<SessionCardData>) {
  if (loading && !data) return <CardSkeleton />;
  if (!data) return null;

  return (
    <Card title="Session">
      <StatRow label="Duration" value={formatDuration(data.duration_ms)} />
      <StatRow label="Turns" value={data.turn_count} />
      <StatRow label="Messages" value={data.user_messages + data.assistant_messages} />
    </Card>
  );
}

// Register
cardRegistry.push({
  key: 'session',
  title: 'Session',
  component: SessionCard,
  defaultEnabled: true,
  order: 0,
});
```

---

## Deferred Decisions

The following decisions are intentionally deferred. Keep the implementation simple now; extend later when requirements are clearer.

| Topic | Current Approach | Revisit When |
|-------|------------------|--------------|
| Visualization cards | Decide per-card when we build them | Adding first chart card |
| Premium gating UX | Not implemented | Freemium launch |
| Per-card refresh rates | Unified polling | Performance issues arise |
| Card expansion/collapse | Not implemented | User feedback requests it |

---

## Related Files

### Backend
- `backend/internal/analytics/models.go` - Data structures
- `backend/internal/analytics/compute.go` - Computation logic
- `backend/internal/analytics/parser.go` - JSONL parsing
- `backend/internal/api/analytics.go` - HTTP handler

### Frontend
- `frontend/src/components/session/SessionSummaryPanel.tsx` - Current panel
- `frontend/src/hooks/useAnalyticsPolling.ts` - Current polling hook
- `frontend/src/schemas/api.ts` - API type definitions

---

## Next Steps

1. Review and approve this design
2. Implement Phase 1 (backend restructure)
3. Implement Phase 2 (frontend framework)
4. Implement Phase 3 (new cards) - prioritized from survey doc
