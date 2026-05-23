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
| `renderHighlight.tsx` | React-JSX companion to `highlightSearch.ts`: `renderTextWithHighlight(text, query, isActiveMatch) => ReactNode` for text-node surfaces (command lines, file paths, chips, divider labels) where `dangerouslySetInnerHTML` is overkill. Stays in its own `.tsx` file so plain-text utilities can keep React out of their import graph |
| `sorting.ts` | Generic array sorting by key with type support |
| `dateRange.ts` | Date range types, presets (This Week, Last 30 Days, etc.), URL parsing |
| `git.ts` | Git URL conversion (SSH to HTTPS, branch URLs) |
| `sessionErrors.ts` | Session error types, messages, icons, and 401 redirect skip list |
| `agentSkillChart.ts` | Shared agent/skill chart constants (`AGENT_SKILL_COLORS`) and the `ChartDataItem` shape |
| `chartLabels.ts` | `truncateName` — cap long bar-chart axis labels to "prefix...suffix" form. `truncatedYAxisWidth(labels, minChars)` — px-width for a Recharts YAxis sized off the *truncated* form so one long MCP/agent name can't shove bars off-screen. Pairs with `TruncatedYAxisTick` (Agents & Skills, Tools) |
| `utils.ts` | Low-level utilities: `stripAnsi`, `isRecord` (runtime guard for plain objects, used wherever an `unknown` needs its fields read without an `as` cast), `formatBytes` |
| `markdown.ts` | `renderMarkdownToHtml` — GFM markdown to sanitized HTML via `marked` + `DOMPurify`. `tryParseAsJson` — if a string is a JSON object/array, return a pretty-printed version (used as the JSON fallback before markdown rendering). Used by `ContentBlock`, the Codex message renderers, `AwaySummary`, and `QueuedCommand` |
| `providers.ts` | CF-416 — `PROVIDER_METADATA` registry (icon, label, brandDisplayName, brandColor, resumeCommand) keyed by `ProviderId` (`'claude-code' \| 'codex'`). `getProviderMetadata(id)` strict canonical lookup; `getProviderMetadataOrFallback(value, 'claude' \| 'neutral')` codifies the unknown-provider policy in one place. `isLegacyClaudeCode(value)` (CF-366) — documented seam mirroring backend `models.LegacyAliases`; returns true for non-canonical spellings whose normalized form is `'claude-code'`. `PROVIDER_VALUES`, `providerLabel(value)` preserved for back-compat with surfaces that render arbitrary backend values. Used by `providerIcon`, `SessionHeader`, `CopyIdDropdown`, `TrendsTopSessionsCard`, `FilterChipsBar` |
| `demoIdentity.ts` | CF-483 demo-mode helpers. `getDemoIdentity(): string \| null` defensively reads the backend-injected `window.__DEMO_IDENTITY__` global. `isDemoViewer(email): boolean` returns true when demo mode is on AND `email` matches the demo identity — used by `Header`, `HomePage`, `LoginPage` to branch UI for the read-only viewer (skip `?owner=` pre-filter, show "Log in as yourself", skip post-login redirect). `notifyReadOnlyDemo()` dispatches the `confab:read-only` CustomEvent that `ReadOnlyToast` listens for; called from `services/api.ts` when a request returns `{"error":"read_only_user"}`. `READ_ONLY_EVENT` is the documented event-name constant. |
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

CF-418 made the cost surface provider-agnostic. Both transcript services
normalize their wire shape into the canonical `TokenUsage` at parse time;
components and arithmetic all read one shape downstream.

| Export | Signature | Description |
|--------|-----------|-------------|
| `TokenUsage` (type) | `{ input, output, cacheWrite, cacheRead }` | Canonical per-message token usage. `input` is uncached; `output` includes reasoning where applicable; `cacheRead`/`cacheWrite` are split out (cacheWrite is `0` for Codex models, which don't charge for writes). |
| `calculateCost` | `(provider: ProviderId, model: string, usage: TokenUsage) => number` | Base arithmetic. Pure: no fast multiplier, no server-tool add-on (those live on the provider adapter). Throws on unknown provider; warns and returns 0 on unknown model within a known provider. |
| `getModelFamily` | `(provider: ProviderId, modelName: string) => string` | Strip date suffixes / `claude-` prefix to produce a pricing-table key. Claude branch matches `opus|sonnet|haiku-N(-N)?`; Codex branch strips OpenAI's pinned `-YYYY-MM-DD` suffix. |
| `buildCostTooltip` | `(adapter, usage, cost, message) => string` | Cost-badge tooltip. Base lines (`$cost`, blank, input, output) plus the adapter's optional `extendCostTooltip` (Claude: Cache/Speed/Tier/Web-search; Codex: rebuilds the body to show gross input + interleaved cached / reasoning sub-lines for byte-identical pre-refactor formatting). |
| `normalizeClaudeUsage` | `(wire) => TokenUsage` | Bridge from Claude's wire shape (`input_tokens`, `cache_creation_input_tokens`, …) to canonical `TokenUsage`. Used by `transcriptService` at parse time and as the fallback for cached entries written before the parse-time stamping. |
| `formatCost` | `(usd: number) => string` | Format as "$0.42" or "<$0.01". |
| `formatTokenCount` | `(count: number) => string` | Format as "500", "1.5k", "1.5M". |

**Model pricing table** (`MODEL_PRICING`): provider-keyed nested
`Record<ProviderId, Record<family, ModelPricing>>`. Adding a third provider
is one outer key plus N inner rows — no code branches. OpenAI entries set
`cacheWrite: 0` (writes are free) and put the documented cached-input rate
in `cacheRead`. Unknown families fall back to zero pricing with a warning.

**Server tool pricing**: `WEB_SEARCH_COST_PER_REQUEST = $0.01`, applied by
the Claude adapter's `calculateMessageCost`. Not multiplied by fast mode.

**Fast mode**: `FAST_MODE_MULTIPLIER = 6` applied by the Claude adapter when
`message.message.usage.speed === 'fast'`. Codex has no equivalent toggle.

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

### renderHighlight.tsx

| Function | Signature | Description |
|----------|-----------|-------------|
| `renderTextWithHighlight` | `(text, query, isActiveMatch) => ReactNode` | Wraps `splitTextByQuery` with React `<mark>` elements; no-op when `query` is falsy so callers can pipe through unconditionally |

### dateRange.ts

| Function | Signature | Description |
|----------|-----------|-------------|
| `getDefaultDateRange` | `() => DateRange` | Returns "Last 7 Days" ending today |
| `getDateRangeLabel` | `(startDate, endDate) => string` | Infer a human-readable label for a date range, falling back to "start - end" |
| `getDatePresets` | `() => DateRange[]` | Standard presets: This Week, Last Week, Last 7 Days, This Month, Last Month, Last 30/90 Days |
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
When adding a new Anthropic OR OpenAI model, update `MODEL_PRICING` in **both**:
- `frontend/src/utils/tokenStats.ts` (this file)
- `backend/internal/analytics/pricing.go` (backend)

These tables must stay in sync; `TestPricingTableSync` enforces this. Look up current prices on the Anthropic pricing page or OpenAI's developer pricing page. For OpenAI entries set `cacheWrite: 0` (writes are free) and put the documented cached-input rate in `cacheRead`.

## Invariants / Conventions

- **Pure functions only**: No React hooks, no side effects, no DOM access (except `escapeHtml` which uses `document.createElement`)
- **Duration formatting has context-specific variants**: `formatDuration()` in `formatting.ts` is the general-purpose version. `SessionCard`, `ConversationCard`, and `TimelineBar` each have specialized variants noted in their JSDoc comments.
- **Zero pricing for unknown models**: `tokenStats.ts` returns zero cost for unrecognized model names rather than guessing, so cost is underreported but never silently wrong.
- **Date handling normalizes to UTC**: `formatRelativeTime` and `formatDateString` append 'Z' to date strings that lack timezone info to ensure consistent UTC interpretation.

## Design Decisions

- **Frontend cost calculation**: Token costs are computed client-side from the pricing table + transcript token usage data. This avoids adding cost computation to the backend transcript parser and allows instant cost display as messages stream in.
- **HTML-aware search highlighting**: `highlightTextInHtml` splits HTML into tag/text segments and only applies highlighting to text nodes. This prevents breaking HTML structure when highlighting inside rendered markdown.
- **Model family extraction over full name matching**: `getModelFamily(provider, modelName)` normalizes names like "claude-opus-4-5-20251101" to "opus-4-5" or "gpt-5-2026-05-01" to "gpt-5" so pricing works regardless of date suffix or "claude-" prefix variations.
- **Parse-layer normalization to a canonical token shape** (CF-418): both transcript services stamp `TokenUsage` (`input`/`output`/`cacheWrite`/`cacheRead`) on assistant messages at parse time. Downstream components and cost arithmetic read one shape. Provider-specific cost adjustments (Claude fast multiplier, web-search dollars; Codex reasoning-token display) live on the `ProviderAdapter` (`providers/types.ts`).

## Testing

- `formatting.test.ts` -- Date/time formatting, duration formatting, model name extraction
- `tokenStats.test.ts` -- Cost calculation, model family extraction, formatting
- `sessionMeta.test.ts` -- Duration computation from messages, fallback behavior
- `compactionStats.test.ts` -- Compaction event counting, average time calculation
- `highlightSearch.test.ts` -- HTML highlighting, text splitting, edge cases
- `dateRange.test.ts` -- Date range presets, URL parsing
- `git.test.ts` -- Repo/branch URL conversion across SSH, HTTPS, GitHub, GitLab
- `sorting.test.ts` -- Generic sort by string/number/date with null handling and filter
- `utils.test.ts` -- ANSI stripping, byte formatting
- `providers.test.ts` -- Registry drift guard, canonical/fallback lookups, legacy `'Claude Code'` normalization, `providerLabel` passthrough

## Dependencies

- `@/types` (type imports for `TranscriptLine`, `AssistantMessage`, `GitInfo`)
- `marked` (GFM markdown parser, used by `markdown.ts`)
- `dompurify` (HTML sanitization, used by `markdown.ts`)
