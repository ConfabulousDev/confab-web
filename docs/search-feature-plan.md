# Session Search Feature Plan

## Overview

Add ability to search across:
- User's own sessions
- Sessions shared privately with the user
- Public sessions

## Architecture Options

### Option 1: PostgreSQL Full-Text Search (Recommended Start)

**Approach**: Use PostgreSQL's built-in `tsvector`/`tsquery` with GIN indexes

**Pros**:
- No additional infrastructure
- ACID compliance with existing data
- Access control in same query
- Good for metadata search

**Cons**:
- Limited for large transcript content
- No fuzzy matching without `pg_trgm`
- Performance degrades at scale

**Expected Performance**:
| Dataset Size | Query Time (metadata) | Query Time (transcripts) |
|-------------|----------------------|--------------------------|
| 1K sessions | < 20ms | 50-200ms |
| 10K sessions | 20-50ms | 200ms-2s |
| 100K sessions | 50-100ms | 2-10s |

### Option 2: Dedicated Search Engine

**Candidates**: Meilisearch, Typesense, Elasticsearch

**Pros**:
- Excellent full-text search
- Typo tolerance, fuzzy matching
- Relevance ranking
- Scales well

**Cons**:
- Additional infrastructure
- Sync complexity
- Access control in search layer
- Cost

### Option 3: Hybrid

PostgreSQL for metadata, dedicated engine for transcript content.

Most complex but best performance for transcript search at scale.

## Recommended Approach

Start with **Option 1** (PostgreSQL) for MVP:

1. Sufficient for hundreds to low thousands of sessions
2. No additional infrastructure
3. Can migrate to Option 2 later if needed

## Implementation Plan

### Phase 1: Database Schema

```sql
-- Add search vectors to sessions table
ALTER TABLE sessions ADD COLUMN search_vector tsvector;

-- Create GIN index
CREATE INDEX sessions_search_idx ON sessions USING GIN(search_vector);

-- Trigger to update search vector
CREATE FUNCTION sessions_search_update() RETURNS trigger AS $$
BEGIN
  NEW.search_vector :=
    setweight(to_tsvector('english', COALESCE(NEW.session_id, '')), 'A') ||
    setweight(to_tsvector('english', COALESCE(NEW.cwd, '')), 'B');
  RETURN NEW;
END
$$ LANGUAGE plpgsql;

CREATE TRIGGER sessions_search_trigger
  BEFORE INSERT OR UPDATE ON sessions
  FOR EACH ROW EXECUTE FUNCTION sessions_search_update();
```

### Phase 2: Access Control Query

```sql
-- Search with access control
SELECT s.* FROM sessions s
LEFT JOIN shares sh ON sh.session_id = s.id
WHERE (
  s.user_id = $1  -- own sessions
  OR sh.visibility = 'public'
  OR (sh.visibility = 'private' AND $2 = ANY(sh.invited_emails))
)
AND s.search_vector @@ plainto_tsquery('english', $3)
ORDER BY ts_rank(s.search_vector, plainto_tsquery('english', $3)) DESC
LIMIT 50;
```

### Phase 3: API Endpoint

```go
// GET /api/v1/search?q=query&scope=all|own|shared
type SearchRequest struct {
    Query string `json:"q"`
    Scope string `json:"scope"` // all, own, shared
    Limit int    `json:"limit"`
}

type SearchResult struct {
    Sessions []SessionSummary `json:"sessions"`
    Total    int              `json:"total"`
}
```

### Phase 4: Frontend UI

- Search input in header or sessions page
- Scope selector (My Sessions / Shared with Me / All)
- Results list with highlighting
- Filters: date range, visibility

## Optimizations (If Needed)

1. **Materialized view** for pre-computed searchable sessions per user
2. **pg_trgm extension** for fuzzy/LIKE queries
3. **Partial indexes** on frequently searched columns
4. **Limit transcript indexing** to summary or first N characters

## Migration Triggers

Move to dedicated search engine (Option 2) when:
- Query times consistently exceed 500ms
- Need fuzzy matching on full transcripts
- Dataset exceeds 50K sessions
- Users expect "instant" Google-like results

## Open Questions

1. What fields to search?
   - Session ID, cwd, reason (metadata)
   - Transcript content?
   - Git info (repo, branch, commit message)?

2. Search result display
   - Show snippet/context of match?
   - Highlight matched terms?

3. Scope defaults
   - Default to "all" or "own sessions"?
   - Remember user preference?

4. Real-time vs batch indexing
   - Update search vector on every save?
   - Background job for transcript indexing?
