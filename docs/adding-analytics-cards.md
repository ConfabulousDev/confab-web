# Adding a New Analytics Card

This playbook covers the end-to-end process for adding a new analytics card to the session summary panel.

## Overview

The analytics system uses a **card-per-table** architecture where each card type has:
- Its own database table (`session_card_<name>`)
- Independent version constant for cache invalidation
- Custom staleness logic (defaults to line-count based)

## Steps

### 1. Database Migration

Create `backend/internal/db/migrations/000XXX_session_card_<name>.up.sql`:

```sql
CREATE TABLE session_card_<name> (
    session_id UUID PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    version INT NOT NULL DEFAULT 1,
    computed_at TIMESTAMPTZ NOT NULL,
    up_to_line BIGINT NOT NULL,
    -- card-specific columns
    my_metric BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX idx_session_card_<name>_version ON session_card_<name>(version);
```

Create corresponding `.down.sql`:

```sql
DROP TABLE IF EXISTS session_card_<name>;
```

### 2. Define Card Types

In `backend/internal/analytics/cards.go`:

```go
// Version constant - bump to invalidate all cached cards
const <Name>CardVersion = 1

// Database record type
type <Name>CardRecord struct {
    SessionID  string    `json:"session_id"`
    Version    int       `json:"version"`
    ComputedAt time.Time `json:"computed_at"`
    UpToLine   int64     `json:"up_to_line"`
    MyMetric   int64     `json:"my_metric"`
}

// Validation - customize staleness logic here
func (c *<Name>CardRecord) IsValid(currentLineCount int64) bool {
    return c != nil &&
           c.Version == <Name>CardVersion &&
           c.UpToLine == currentLineCount
}

// API response type (JSON serialization)
type <Name>CardData struct {
    MyMetric int64 `json:"my_metric"`
}
```

Add field to the `Cards` struct:

```go
type Cards struct {
    Tokens     *TokensCardRecord
    Cost       *CostCardRecord
    Compaction *CompactionCardRecord
    <Name>     *<Name>CardRecord  // Add new field
}
```

Update `AllValid()` to include the new card:

```go
func (c *Cards) AllValid(currentLineCount int64) bool {
    if c == nil {
        return false
    }
    return c.Tokens.IsValid(currentLineCount) &&
           c.Cost.IsValid(currentLineCount) &&
           c.Compaction.IsValid(currentLineCount) &&
           c.<Name>.IsValid(currentLineCount)  // Add validation
}
```

### 3. Implement Store Operations

In `backend/internal/analytics/store.go`, add get/upsert methods:

```go
func (s *Store) get<Name>Card(ctx context.Context, sessionID string) (*<Name>CardRecord, error) {
    var card <Name>CardRecord
    err := s.db.QueryRowContext(ctx, `
        SELECT session_id, version, computed_at, up_to_line, my_metric
        FROM session_card_<name>
        WHERE session_id = $1
    `, sessionID).Scan(
        &card.SessionID,
        &card.Version,
        &card.ComputedAt,
        &card.UpToLine,
        &card.MyMetric,
    )
    if err == sql.ErrNoRows {
        return nil, nil
    }
    if err != nil {
        return nil, fmt.Errorf("querying <name> card: %w", err)
    }
    return &card, nil
}

func (s *Store) upsert<Name>Card(ctx context.Context, card *<Name>CardRecord) error {
    if card == nil {
        return nil
    }
    _, err := s.db.ExecContext(ctx, `
        INSERT INTO session_card_<name> (session_id, version, computed_at, up_to_line, my_metric)
        VALUES ($1, $2, $3, $4, $5)
        ON CONFLICT (session_id) DO UPDATE SET
            version = EXCLUDED.version,
            computed_at = EXCLUDED.computed_at,
            up_to_line = EXCLUDED.up_to_line,
            my_metric = EXCLUDED.my_metric
    `, card.SessionID, card.Version, card.ComputedAt, card.UpToLine, card.MyMetric)
    return err
}
```

Update `GetCards()` and `UpsertCards()` to include the new card.

### 4. Add Compute Logic

In `backend/internal/analytics/compute.go`, add field to `ComputeResult`:

```go
type ComputeResult struct {
    // ... existing fields
    MyMetric int64
}
```

Add computation logic in `ComputeFromJSONL()`:

```go
// Process relevant line types
if line.IsRelevantType() {
    result.MyMetric += extractValue(line)
}
```

### 5. Wire Up Card Creation

Update `ToCards()` in `store.go`:

```go
func (r *ComputeResult) ToCards(sessionID string, lineCount int64) *Cards {
    now := time.Now().UTC()
    return &Cards{
        // ... existing cards
        <Name>: &<Name>CardRecord{
            SessionID:  sessionID,
            Version:    <Name>CardVersion,
            ComputedAt: now,
            UpToLine:   lineCount,
            MyMetric:   r.MyMetric,
        },
    }
}
```

### 6. Add to API Response

Update `ToResponse()` in `store.go`:

```go
func (c *Cards) ToResponse() *AnalyticsResponse {
    resp := &AnalyticsResponse{
        Cards: make(map[string]interface{}),
        // ... existing setup
    }

    // ... existing cards

    if c.<Name> != nil {
        resp.Cards["<name>"] = <Name>CardData{
            MyMetric: c.<Name>.MyMetric,
        }
    }

    return resp
}
```

### 7. Frontend: Zod Schema

In `frontend/src/schemas/api.ts`:

```typescript
// Add card data schema
export const <Name>CardDataSchema = z.object({
  my_metric: z.number(),
});

// Add to AnalyticsCardsSchema
export const AnalyticsCardsSchema = z.object({
  tokens: TokensCardDataSchema.optional(),
  cost: CostCardDataSchema.optional(),
  compaction: CompactionCardDataSchema.optional(),
  <name>: <Name>CardDataSchema.optional(),  // Add new card
});

// Add inferred type
export type <Name>CardData = z.infer<typeof <Name>CardDataSchema>;
```

### 8. Frontend: Card Component

Create `frontend/src/components/session/cards/<Name>Card.tsx`:

```typescript
import { Card } from './Card';
import type { CardProps } from './types';

export function <Name>Card({ data }: CardProps<'<name>'>) {
  return (
    <Card title="<Display Name>">
      <div className="text-2xl font-semibold text-gray-900 dark:text-white">
        {data.my_metric.toLocaleString()}
      </div>
      <div className="text-xs text-gray-500 dark:text-gray-400">
        some label
      </div>
    </Card>
  );
}
```

### 9. Register the Card

In `frontend/src/components/session/cards/registry.ts`:

```typescript
import { <Name>Card } from './<Name>Card';

// Add registration (order determines display position)
registerCard('<name>', <Name>Card, { order: 40 });
```

Export from `frontend/src/components/session/cards/index.ts`:

```typescript
export { <Name>Card } from './<Name>Card';
```

### 10. Add Tests

**Backend tests** (`backend/internal/analytics/`):

- `cache_test.go`: Test `IsValid()` logic
- `compute_test.go`: Test metric extraction from JSONL
- `store_test.go`: Integration tests for get/upsert

**Frontend tests** (`frontend/src/components/session/cards/`):

Create `<Name>Card.test.tsx`:

```typescript
import { render, screen } from '@testing-library/react';
import { <Name>Card } from './<Name>Card';

describe('<Name>Card', () => {
  it('renders metric value', () => {
    render(<<Name>Card data={{ my_metric: 42 }} />);
    expect(screen.getByText('42')).toBeInTheDocument();
  });
});
```

### 11. Update Documentation

Update `backend/API.md` with the new card schema in the analytics response.

## Key Principles

| Principle | Description |
|-----------|-------------|
| **Independent versioning** | Each card has its own version constant. Bump to force recompute. |
| **Custom staleness** | Override `IsValid()` for non-line-count based invalidation. |
| **Backward compatible** | New cards appear in the `cards` map without breaking existing clients. |
| **Lazy computation** | Cards are computed on first request, then cached. |

## Common Patterns

### Time-based invalidation

For cards that should refresh periodically:

```go
func (c *<Name>CardRecord) IsValid(currentLineCount int64) bool {
    if c == nil || c.Version != <Name>CardVersion {
        return false
    }
    // Refresh if older than 1 hour
    return time.Since(c.ComputedAt) < time.Hour
}
```

### External data dependency

For cards that depend on external APIs (e.g., GitHub):

```go
func (c *<Name>CardRecord) IsValid(currentLineCount int64) bool {
    if c == nil || c.Version != <Name>CardVersion {
        return false
    }
    // Refresh every 5 minutes for external data freshness
    return time.Since(c.ComputedAt) < 5*time.Minute
}
```

### Derived metrics

For cards that derive values from other cards, compute in `ToCards()` using `ComputeResult` fields that aggregate underlying data.
