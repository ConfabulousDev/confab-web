# Session Analytics Feature - Design Plan

## Overview

Add server-side analytics computation for Claude Code sessions. Initial scope matches the existing frontend stats sidebar, moving computation to the backend with caching for performance and future aggregation.

---

## Core Design Decisions

| Decision | Choice |
|----------|--------|
| Compute strategy | On-demand with DB caching |
| Session lifecycle | Never ends - always resumable |
| Staleness | User-controlled refresh button |
| Storage | Separate `session_analytics` table |
| Schema approach | Extracted columns (aggregatable) + JSONB (flexible) |
| Initial scope | Match current frontend sidebar stats |

---

## Initial Scope: Match Frontend Sidebar

The frontend currently computes these stats client-side. We'll move this to backend:

### Token Stats
| Metric | JSONL Source |
|--------|--------------|
| Input tokens | `message.usage.input_tokens` |
| Output tokens | `message.usage.output_tokens` |
| Cache created | `message.usage.cache_creation_input_tokens` |
| Cache read | `message.usage.cache_read_input_tokens` |

### Cost
| Metric | Computation |
|--------|-------------|
| Estimated cost | Tokens × model-specific pricing (Opus/Sonnet/Haiku) |

### Compaction Stats
| Metric | JSONL Source |
|--------|--------------|
| Auto compactions | Count of `system` + `subtype: "compact_boundary"` + `trigger: "auto"` |
| Manual compactions | Same with `trigger: "manual"` |
| Avg compaction time | Time from `logicalParentUuid` to compact_boundary (auto only) |

---

## Schema

```sql
CREATE TABLE session_analytics (
  session_id UUID PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
  version INT NOT NULL,                    -- last_synced_line when computed
  computed_at TIMESTAMPTZ NOT NULL,

  -- Token stats (extracted for future aggregation)
  input_tokens BIGINT,
  output_tokens BIGINT,
  cache_creation_tokens BIGINT,
  cache_read_tokens BIGINT,

  -- Cost (extracted for future aggregation)
  estimated_cost_usd DECIMAL(10,4),

  -- Compaction stats (extracted)
  compaction_auto INT,
  compaction_manual INT,
  compaction_avg_time_ms INT,

  -- Flexible JSONB for future expansion
  details JSONB NOT NULL DEFAULT '{}'
);
```

**Why extracted columns?**
- Enables future aggregation: `SELECT SUM(input_tokens) FROM session_analytics WHERE user_id = ...`
- No indexes needed for aggregation - just need the columns
- JSONB `details` allows adding new metrics without migrations

---

## API

### GET /api/v1/sessions/{id}/analytics

Returns cached analytics if fresh, otherwise computes and caches.

**Response:**
```json
{
  "computed_at": "2025-12-26T17:00:00Z",
  "computed_version": 1547,
  "current_version": 1634,
  "is_stale": true,

  "tokens": {
    "input": 1500000,
    "output": 250000,
    "cache_creation": 890000,
    "cache_read": 1200000
  },

  "cost": {
    "estimated_usd": 4.82
  },

  "compaction": {
    "auto": 3,
    "manual": 1,
    "avg_time_ms": 4200
  }
}
```

### POST /api/v1/sessions/{id}/analytics/refresh

Force recomputation. Returns same format as GET.

---

## Cache Flow

```
GET /sessions/{id}/analytics
         │
         ▼
   Load session + sync_files.last_synced_line
         │
         ▼
   session_analytics.version == last_synced_line?
         │
    ┌────┴────┐
   Yes        No
    │          │
    ▼          ▼
  Return     Fetch S3 chunks
  cached     Parse JSONL
             Compute metrics
             INSERT/UPDATE session_analytics
             Return fresh
```

**Stale behavior:** When `version < last_synced_line`, return cached data WITH `is_stale: true`. User decides when to refresh.

---

## UX

```
┌─────────────────────────────────────────────────────────┐
│ ○ Transcript   ● Analytics                              │
├─────────────────────────────────────────────────────────┤
│ Computed from 1,547 lines (87 new)    [↻ Refresh]      │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  Tokens                                                 │
│  → Input           1.5M                                 │
│  ← Output          250k                                 │
│  ◇ Cache created   890k                                 │
│  ◆ Cache read      1.2M                                 │
│                                                         │
│  Cost                                                   │
│  ⓘ Estimated       $4.82                                │
│                                                         │
│  Compaction                                             │
│  Auto              3                                    │
│  Manual            1                                    │
│  Avg time (auto)   4.2s                                 │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

| State | Behavior |
|-------|----------|
| Fresh | Refresh button disabled |
| Stale | Shows "(N new)", refresh button enabled |
| Loading | Spinner during computation |

---

## Implementation Phases

### Phase 1: Backend Foundation
- [ ] Database migration (`session_analytics` table)
- [ ] JSONL line parser (extract type, usage, compaction metadata)
- [ ] Metrics computation (tokens, cost, compaction)
- [ ] Model pricing table (Opus/Sonnet/Haiku variants)
- [ ] Cache storage/retrieval

### Phase 2: API Endpoints
- [ ] `GET /sessions/{id}/analytics` with cache logic
- [ ] `POST /sessions/{id}/analytics/refresh`
- [ ] Staleness detection (version comparison)

### Phase 3: Frontend
- [ ] Analytics tab component
- [ ] Stale indicator + refresh button
- [ ] Loading states
- [ ] Match existing sidebar styling

### Phase 4: Migration
- [ ] Remove client-side computation from sidebar
- [ ] Sidebar fetches from analytics API
- [ ] Clean up frontend utils (or keep for offline/fallback)

---

## Future Expansion (Out of Scope)

These can be added to `details` JSONB later:

- **Duration**: First timestamp to last timestamp
- **Tool breakdown**: Map of tool name → count
- **User messages**: Count of human messages
- **Error rate**: Errors / tool calls
- **Autonomy score**: 1 - (user messages / tool calls)
- **Files touched**: Map of file path → edit count
- **Patterns**: High autonomy, long session no commit, etc.
- **Time series**: Activity over time

---

## Future Aggregation Path

When user/team aggregates are needed:

```sql
-- Simple backfill from extracted columns
INSERT INTO user_analytics (user_id, total_input_tokens, total_cost_usd, ...)
SELECT
  s.user_id,
  SUM(sa.input_tokens),
  SUM(sa.estimated_cost_usd),
  ...
FROM session_analytics sa
JOIN sessions s ON sa.session_id = s.id
GROUP BY s.user_id;
```

No schema changes to `session_analytics` required.
