---
name: add-session-card
description: Add a new analytics card to the session summary panel. Covers backend collector, database migration, API response, and frontend component with Storybook stories.
---

# Add Session Analytics Card

Add a new analytics card to the session summary panel following the card-per-table architecture.

## Overview

The analytics system uses a **card-per-table** architecture where each card type has:
- Its own database table (`session_card_<name>`)
- Independent version constant for cache invalidation
- A **collector** that extracts metrics during a single JSONL pass
- Frontend component registered in the card registry

## Instructions for Claude

Use **TodoWrite** to track all phases. This is a multi-step task requiring both backend and frontend changes.

### Phase 1: Plan the Card

Before writing any code:

- [ ] Understand what metrics the card will display
- [ ] Identify which transcript line types contain the data
- [ ] Plan the database schema (what columns are needed)
- [ ] Plan the API response format

### Phase 2: Backend - Database Migration

Create migration files in `backend/internal/db/migrations/`:

**Up migration** (`000XXX_session_card_<name>.up.sql`):
```sql
CREATE TABLE session_card_<name> (
    session_id UUID PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    version INT NOT NULL DEFAULT 1,
    computed_at TIMESTAMPTZ NOT NULL,
    up_to_line BIGINT NOT NULL,
    -- card-specific columns (use snake_case)
    my_metric BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX idx_session_card_<name>_version ON session_card_<name>(version);
```

**Down migration** (`000XXX_session_card_<name>.down.sql`):
```sql
DROP TABLE IF EXISTS session_card_<name>;
```

Get the next migration number:
```bash
ls backend/internal/db/migrations/*.up.sql | sort | tail -1
```

### Phase 3: Backend - Card Types

In `backend/internal/analytics/cards.go`, add:

1. **Version constant** (bump to invalidate all cached cards):
```go
const <Name>CardVersion = 1
```

2. **Record type** (matches database schema):
```go
type <Name>CardRecord struct {
    SessionID  string    `json:"session_id"`
    Version    int       `json:"version"`
    ComputedAt time.Time `json:"computed_at"`
    UpToLine   int64     `json:"up_to_line"`
    MyMetric   int64     `json:"my_metric"`
}

func (c *<Name>CardRecord) IsValid(currentLineCount int64) bool {
    return c != nil &&
           c.Version == <Name>CardVersion &&
           c.UpToLine == currentLineCount
}
```

3. **API data type** (JSON response):
```go
type <Name>CardData struct {
    MyMetric int64 `json:"my_metric"`
}
```

4. **Add to Cards struct and AllValid()**

### Phase 4: Backend - Collector

Create `backend/internal/analytics/collector_<name>.go`:

```go
package analytics

// <Name>Collector extracts <name> metrics from transcript lines.
type <Name>Collector struct {
    MyMetric int64
}

func New<Name>Collector() *<Name>Collector {
    return &<Name>Collector{}
}

func (c *<Name>Collector) Collect(line *TranscriptLine, ctx *CollectContext) {
    // Process relevant line types
    // Use line helpers: IsUserMessage(), IsAssistantMessage(), GetToolUses(), etc.
}

func (c *<Name>Collector) Finalize(ctx *CollectContext) {
    // Post-processing (compute averages, etc.)
}
```

**TranscriptLine helpers:**
- `IsUserMessage()` - true for user messages
- `IsAssistantMessage()` - true for assistant messages with usage
- `IsCompactBoundary()` - true for compaction markers
- `GetTimestamp()` - parse timestamp
- `GetToolUses()` - extract tool_use blocks
- `GetModel()` - get model ID
- `GetStopReason()` - get stop reason

### Phase 5: Backend - Wire Up

1. **Register collector** in `compute.go`:
   - Add to `ComputeResult` struct
   - Create collector in `ComputeFromJSONL()`
   - Pass to `RunCollectors()`
   - Extract result after run

2. **Store operations** in `store.go`:
   - Add `get<Name>Card()` method
   - Add `upsert<Name>Card()` method
   - Update `GetCards()` to include new card
   - Update `UpsertCards()` to include new card

3. **Card creation** in `store.go`:
   - Update `ToCards()` to create record from `ComputeResult`

4. **API response** in `store.go`:
   - Update `ToResponse()` to include card data in `Cards` map

### Phase 6: Backend - Tests

Run backend tests:
```bash
cd backend && DOCKER_HOST=unix:///Users/jackie/.orbstack/run/docker.sock go test ./...
```

### Phase 7: Frontend - Zod Schema

In `frontend/src/schemas/api.ts`:

1. Add card data schema:
```typescript
export const <Name>CardDataSchema = z.object({
  my_metric: z.number(),
});
```

2. Add to `AnalyticsCardsSchema`:
```typescript
<name>: <Name>CardDataSchema.optional(),
```

3. Export type:
```typescript
export type <Name>CardData = z.infer<typeof <Name>CardDataSchema>;
```

### Phase 8: Frontend - Card Component

Create `frontend/src/components/session/cards/<Name>Card.tsx`:

```typescript
import { CardWrapper, StatRow, CardLoading } from './Card';
import type { <Name>CardData } from '@/schemas/api';
import type { CardProps } from './types';

const TOOLTIPS = {
  myMetric: 'Description of what this metric means',
};

export function <Name>Card({ data, loading }: CardProps<<Name>CardData>) {
  if (loading && !data) {
    return (
      <CardWrapper title="<Display Name>">
        <CardLoading />
      </CardWrapper>
    );
  }

  if (!data) return null;

  return (
    <CardWrapper title="<Display Name>">
      <StatRow
        label="My Metric"
        value={data.my_metric}
        tooltip={TOOLTIPS.myMetric}
      />
    </CardWrapper>
  );
}
```

### Phase 9: Frontend - Register Card

1. In `registry.ts`:
   - Import the card component
   - Add to `cardRegistry` array with appropriate order

2. In `index.ts`:
   - Export the card component

3. Update `registry.test.ts`:
   - Add new card to expected cards list

### Phase 10: Frontend - Storybook

Create `frontend/src/components/session/cards/<Name>Card.stories.tsx`:

```typescript
import type { Meta, StoryObj } from '@storybook/react-vite';
import { <Name>Card } from './<Name>Card';

const meta: Meta<typeof <Name>Card> = {
  title: 'Session/Cards/<Name>Card',
  component: <Name>Card,
  parameters: { layout: 'centered' },
  decorators: [
    (Story) => (
      <div style={{ width: '280px' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof <Name>Card>;

export const Default: Story = {
  args: {
    data: { my_metric: 42 },
    loading: false,
  },
};

export const Loading: Story = {
  args: {
    data: undefined,
    loading: true,
  },
};
```

### Phase 11: Frontend - Tests & Build

```bash
cd frontend && npm run build && npm run lint && npm test
cd frontend && npm run build-storybook
```

### Phase 12: Documentation

Update `backend/API.md` with the new card schema in the analytics response.

## Common Patterns

### Time-based invalidation

For cards that should refresh periodically:
```go
func (c *<Name>CardRecord) IsValid(currentLineCount int64) bool {
    if c == nil || c.Version != <Name>CardVersion {
        return false
    }
    return time.Since(c.ComputedAt) < time.Hour
}
```

### Conditional rendering

For cards that shouldn't show when empty:
```typescript
if (data.total_count === 0) return null;
```

## Checklist Before Commit

- [ ] Migration creates and drops table correctly
- [ ] Version constant is defined
- [ ] Collector extracts correct metrics
- [ ] Store operations handle get/upsert
- [ ] API response includes new card
- [ ] Zod schema validates card data
- [ ] Frontend component renders correctly
- [ ] Card is registered in registry
- [ ] Storybook stories cover key states
- [ ] All backend tests pass
- [ ] All frontend tests pass
- [ ] API.md is updated
