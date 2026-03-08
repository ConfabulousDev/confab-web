# utils/

Utility functions for formatting, computation, and data transformation. Pure functions with no React dependencies (except `index.ts` barrel exports).

## Files

| File | Role |
|------|------|
| `formatting.ts` | Date/time formatting, duration formatting, model name extraction, repo name formatting |
| `tokenStats.ts` | Token cost calculation, model pricing table, per-message cost computation |
| `sessionMeta.ts` | Session duration and date computation from transcript timestamps |
| `compactionStats.ts` | Compaction event counting, average compaction time, and response time formatting |
| `highlightSearch.ts` | Search match highlighting in HTML and plain text |
| `sorting.ts` | Generic array sorting by key with type support |
| `dateRange.ts` | Date range types, presets (This Week, Last 30 Days, etc.), URL parsing |
| `git.ts` | Git URL conversion (SSH to HTTPS, branch URLs) |
| `sessionErrors.ts` | Session error types, messages, icons, and 401 redirect skip list |
| `utils.ts` | Low-level utilities: `stripAnsi`, `formatBytes` |
| `index.ts` | Barrel re-exports of commonly used functions |

## Key Functions

### formatting.ts

| Function | Signature | Description |
|----------|-----------|-------------|
| `formatDateString` | `(dateStr: string) => string` | Locale-formatted date string |
| `formatRelativeTime` | `(dateStr: string) => string` | Relative time ("5m ago", "2d ago", "just now") |
| `formatDuration` | `(ms: number, opts?) => string` | Human-readable duration ("1d 2h", "15m", "4.2s") |
| `formatLocalDate` | `(date: Date) => string` | YYYY-MM-DD using local date components |
| `formatDateTime` | `(date: Date) => string` | "Mar 7, 2026, 02:30 PM" format |
| `formatModelName` | `(model: string) => string` | Technical format: "claude-sonnet-4.5" |
| `formatRepoName` | `(repoUrl: string) => string` | Extract "user/repo" from full URL |

### tokenStats.ts

| Function | Signature | Description |
|----------|-----------|-------------|
| `calculateMessageCost` | `(message: TranscriptLine) => number` | Per-message USD cost from token usage (returns 0 for non-assistant messages) |
| `formatCost` | `(usd: number) => string` | Format as "$0.42" or "<$0.01" |
| `formatTokenCount` | `(count: number) => string` | Format as "500", "1.5k", "1.5M" |

**Model pricing table** (`MODEL_PRICING`): Maps model families to per-million-token prices for input, output, cache write (1.25x input), and cache read (0.1x input). Unknown models use zero pricing (cost underreported rather than wrong).

**Server tool pricing**: `WEB_SEARCH_COST_PER_REQUEST = $0.01`

**Fast mode**: 6x multiplier on all token costs when `speed === 'fast'`.

### sessionMeta.ts

| Function | Signature | Description |
|----------|-----------|-------------|
| `computeSessionMeta` | `(messages, session) => SessionMeta` | Duration and date from message timestamps, falling back to session metadata |

### highlightSearch.ts

| Function | Signature | Description |
|----------|-----------|-------------|
| `getHighlightClass` | `(isActiveMatch: boolean) => string` | CSS class name for search highlight mark elements |
| `highlightTextInHtml` | `(html, query, className) => string` | Wrap search matches in `<mark>` tags, only in text nodes |
| `splitTextByQuery` | `(text, query) => Segment[]` | Split plain text into match/non-match segments for React rendering |
| `escapeHtml` | `(text: string) => string` | HTML entity escaping |
| `escapeRegExp` | `(str: string) => string` | Regex special character escaping |

### dateRange.ts

| Function | Signature | Description |
|----------|-----------|-------------|
| `getDefaultDateRange` | `() => DateRange` | Returns "Last 7 Days" ending today |
| `getDatePresets` | `() => DateRange[]` | Standard presets: This Week, Last Week, This Month, Last Month, Last 30/90 Days |
| `parseDateRangeFromURL` | `(searchParams) => DateRange \| null` | Parse `start` and `end` params from URL |

### sessionErrors.ts

| Function | Signature | Description |
|----------|-----------|-------------|
| `statusToErrorType` | `(status: number) => SessionErrorType` | Map HTTP status to typed error |
| `getErrorMessage` | `(type) => string` | User-facing error message |
| `getErrorIcon` | `(type) => string` | Emoji icon for error type |
| `getErrorDescription` | `(type) => string \| undefined` | Optional extended description for error type |
| `shouldSkip401Redirect` | `(endpoint: string) => boolean` | Check if endpoint handles 401 gracefully |

## How to Extend

### Adding a new utility function
1. Add to the appropriate file by category, or create a new file if it doesn't fit
2. Export from the file; add to `index.ts` if it's widely used
3. Add a `.test.ts` file with test cases

### Updating model pricing
When adding a new Anthropic model, update `MODEL_PRICING` in **both**:
- `frontend/src/utils/tokenStats.ts` (this file)
- `backend/internal/analytics/pricing.go` (backend)

These tables must stay in sync. Look up current prices on the Anthropic pricing page.

## Invariants / Conventions

- **Pure functions only**: No React hooks, no side effects, no DOM access (except `escapeHtml` which uses `document.createElement`)
- **Duration formatting has context-specific variants**: `formatDuration()` in `formatting.ts` is the general-purpose version. `SessionCard`, `ConversationCard`, and `TimelineBar` each have specialized variants noted in their JSDoc comments.
- **Zero pricing for unknown models**: `tokenStats.ts` returns zero cost for unrecognized model names rather than guessing, so cost is underreported but never silently wrong.
- **Date handling normalizes to UTC**: `formatRelativeTime` and `formatDateString` append 'Z' to date strings that lack timezone info to ensure consistent UTC interpretation.

## Design Decisions

- **Frontend cost calculation**: Token costs are computed client-side from the pricing table + transcript token usage data. This avoids adding cost computation to the backend transcript parser and allows instant cost display as messages stream in.
- **HTML-aware search highlighting**: `highlightTextInHtml` splits HTML into tag/text segments and only applies highlighting to text nodes. This prevents breaking HTML structure when highlighting inside rendered markdown.
- **Model family extraction over full name matching**: The internal `getModelFamily()` normalizes model names like "claude-opus-4-5-20251101" to "opus-4-5" so pricing works regardless of date suffix or "claude-" prefix variations.

## Testing

- `formatting.test.ts` -- Date/time formatting, duration formatting, model name extraction
- `tokenStats.test.ts` -- Cost calculation, model family extraction, formatting
- `sessionMeta.test.ts` -- Duration computation from messages, fallback behavior
- `compactionStats.test.ts` -- Compaction event counting, average time calculation
- `highlightSearch.test.ts` -- HTML highlighting, text splitting, edge cases
- `dateRange.test.ts` -- Date range presets, URL parsing
- `utils.test.ts` -- ANSI stripping, byte formatting

## Dependencies

- `@/types` (type imports for `TranscriptLine`, `AssistantMessage`, `GitInfo`)
- No external dependencies (pure TypeScript utilities)
