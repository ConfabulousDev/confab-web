---
name: add-session-card
description: Add a new analytics card to the session summary panel. Covers backend analyzer, database migration, API response, and frontend component with Storybook stories.
---

# Add Session Analytics Card

Add a new analytics card to the session summary panel following the card-per-table architecture.

## Overview

The analytics system uses a **card-per-table** architecture where each card type has:
- Its own database table (`session_card_<name>`)
- Independent version constant for cache invalidation
- An **analyzer** that extracts metrics from a `FileCollection`
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

### Phase 4: Backend - Analyzer

Create `backend/internal/analytics/analyzer_<name>.go`:

```go
package analytics

// <Name>Result contains <name> metrics.
type <Name>Result struct {
    MyMetric int64
}

// <Name>Analyzer extracts <name> metrics from transcripts.
type <Name>Analyzer struct{}

// Analyze processes the file collection and returns <name> metrics.
func (a *<Name>Analyzer) Analyze(fc *FileCollection) (*<Name>Result, error) {
    result := &<Name>Result{}

    // Process files - typically main transcript, sometimes agents too
    for _, line := range fc.Main.Lines {
        // Use line helpers to extract data
        if line.IsAssistantMessage() {
            // Process assistant messages
        }
        if line.IsUserMessage() {
            // Process user messages
        }
    }

    return result, nil
}
```

**FileCollection methods:**
- `fc.Main` - main transcript file
- `fc.Agents` - slice of agent transcript files
- `fc.AllFiles()` - iterate over main + all agents

**TranscriptLine helpers:**
- `IsUserMessage()` - true for user messages
- `IsAssistantMessage()` - true for assistant messages with usage
- `IsHumanMessage()` - true for human prompts (not tool results)
- `IsToolResultMessage()` - true for tool result messages
- `IsCompactBoundary()` - true for compaction markers
- `GetTimestamp()` - parse timestamp
- `GetToolUses()` - extract tool_use blocks
- `GetContentBlocks()` - get all content blocks
- `GetAgentResults()` - get subagent/Task results
- `GetModel()` - get model ID
- `GetStopReason()` - get stop reason
- `HasTextContent()` - true if message has text
- `HasThinking()` - true if message has thinking block

### Phase 5: Backend - Wire Up

1. **Add to ComputeResult** in `compute.go`:
```go
// In ComputeResult struct
MyMetric int64
```

2. **Run analyzer** in `ComputeFromFileCollection()`:
```go
<name>, err := (&<Name>Analyzer{}).Analyze(fc)
if err != nil {
    return nil, err
}
```

3. **Populate result** in `ComputeFromFileCollection()`:
```go
return &ComputeResult{
    // ... existing fields ...
    MyMetric: <name>.MyMetric,
}, nil
```

4. **Store operations** in `store.go`:
   - Add `get<Name>Card()` method
   - Add `upsert<Name>Card()` method
   - Update `GetCards()` to include new card
   - Update `UpsertCards()` to include new card

5. **Card creation** in `store.go` `ToCards()`:
```go
<Name>: &<Name>CardRecord{
    SessionID:  sessionID,
    Version:    <Name>CardVersion,
    ComputedAt: now,
    UpToLine:   lineCount,
    MyMetric:   r.MyMetric,
},
```

6. **API response** in `store.go` `ToResponse()`:
```go
if c.<Name> != nil {
    response.Cards["<name>"] = <Name>CardData{
        MyMetric: c.<Name>.MyMetric,
    }
}
```

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

### Processing agent files

For metrics that include subagent data:
```go
// Process all files - main and agents
for _, file := range fc.AllFiles() {
    for _, line := range file.Lines {
        // Process each line
    }
}
```

### Building tool ID maps

For correlating tool_use with tool_result:
```go
toolIDToName := file.BuildToolUseIDToNameMap()
```

## Checklist Before Commit

- [ ] Migration creates and drops table correctly
- [ ] Version constant is defined
- [ ] Analyzer extracts correct metrics
- [ ] Store operations handle get/upsert
- [ ] API response includes new card
- [ ] Zod schema validates card data
- [ ] Frontend component renders correctly
- [ ] Card is registered in registry
- [ ] Storybook stories cover key states
- [ ] All backend tests pass
- [ ] All frontend tests pass
- [ ] API.md is updated
