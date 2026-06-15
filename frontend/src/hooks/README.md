# hooks/

Custom React hooks for the Confab frontend. Organized by responsibility: data fetching, UI state management, auth, and browser APIs.

## Files

| File | Role |
|------|------|
| `useAuth.ts` | Authentication state via React Query (`/me` endpoint) |
| `useSessionsFetch.ts` | Paginated session list with server-side filtering and debounced search |
| `useURLFilters.ts` | Generic URL-synced filter state hook (string, string[], boolean, dateRange fields) |
| `useURLFilters.test.ts` | Tests for `useURLFilters` |
| `useSessionFilters.ts` | URL-synced session filter state (repos, branches, owners, providers, query) via `useURLFilters` |
| `useProviderTranscriptFilters.ts` | Shared machinery for the per-provider transcript filters (x5w2): owns the `?hide=` URL sync, derives typed `filterState` via the provider's `stateFromPaths`, and provides generic category (tri-state for hierarchical) + subcategory toggles. Toggles operate on the canonical hidden-path set so the generic never indexes the opaque state |
| `useClaudeTranscriptFilters.ts` | URL-synced Claude transcript message category filters (thin `useProviderTranscriptFilters` wrapper); exports `pathsFromState`/`stateFromPaths`/`DEFAULT_HIDDEN` |
| `useCodexTranscriptFilters.ts` | URL-synced Codex transcript category filters (CF-361) via `useProviderTranscriptFilters`. Shares the `?hide=` slot with the Claude/OpenCode hooks; foreign tokens are no-ops on read |
| `useOpenCodeTranscriptFilters.ts` | URL-synced OpenCode transcript category filters (x5w2, flat: `user`/`assistant`/`tool`/`unknown`). All visible by default (empty `?hide=`); shares the `?hide=` slot with Claude/Codex. Consumed by `opencodeAdapter` |
| `useCodexTranscriptFilters.test.ts` | Tests for the Codex transcript filter hook (default state, round-trip, toggles, foreign-token tolerance) |
| `useClaudeTranscriptFilters.test.ts` / `useOpenCodeTranscriptFilters.test.tsx` | Round-trip, default-hidden, foreign-token, and (OpenCode) URL-sync toggle tests |
| `useLoadSession.ts` | Single session loading with typed error categories |
| `useAnalyticsPolling.ts` | Session analytics polling with conditional 304 support |
| `useSmartPolling.ts` | Generic smart polling with visibility/activity awareness |
| `useTrends.ts` | Trends data fetching with filter parameters |
| `useOrgAnalytics.ts` | Organization analytics data fetching |
| `useTranscriptSearch.ts` | Generic transcript search with debounced query and match navigation (parameterized over item type via an injected text extractor). Consumed by the Claude, Codex, and OpenCode transcript panes |
| `useShareDialog.ts` | Share dialog form state and API interactions |
| `useAppConfig.ts` | App configuration context accessor |
| `useTheme.ts` | Theme state (light/dark) with toggle |
| `useDocumentTitle.ts` | Sets document title with "| Confabulous" suffix |
| `useCopyToClipboard.ts` | Clipboard copy with success feedback timer |
| `useSuccessMessage.ts` | Auto-dismissing success messages (optional URL param support) |
| `useDropdown.ts` | Dropdown state with click-outside and Escape key handling |
| `useVisibility.ts` | Tracks document/tab visibility via `visibilitychange` |
| `useUserActivity.ts` | Tracks user idle state via DOM events |
| `useRelativeTime.ts` | Auto-updating relative time string with adaptive intervals |
| `useAutoRetry.ts` | Exponential backoff auto-retry with countdown |
| `useServerRecovery.ts` | Invalidates query caches on server error recovery |
| `index.ts` | Barrel exports for commonly-used hooks |

## Hook Categories

### Data Fetching

| Hook | Signature | Description |
|------|-----------|-------------|
| `useAuth` | `() => UseAuthReturn` | Auth state via React Query. Retries 5xx errors twice, never retries 401. |
| `useSessionsFetch` | `(filters: SessionFilters) => UseSessionsFetchReturn` | Cursor-based paginated list. Debounces search query by 300ms. |
| `useLoadSession` | `({ fetchSession, deps }) => UseLoadSessionResult` | Loads a single session with typed error states (`not_found`, `expired`, `forbidden`, `auth_required`, `general`). |
| `useAnalyticsPolling` | `(sessionId, enabled?) => UseAnalyticsPollingReturn` | Polls session analytics. Sends `as_of_line` for 304 Not Modified support. |
| `useSmartPolling` | `(fetchFn, options?) => UseSmartPollingReturn<T>` | Generic polling: suspended (tab hidden), passive (idle, 60s), active (30s). Supports merge functions and interval overrides. |
| `useTrends` | `(initialParams?) => UseTrendsReturn` | Fetches aggregated trends data. Single fetch on mount with manual refetch. `TrendsParams.providers` (CF-424) serializes to singular `?provider=`; `TrendsParams.owners` (CF-495) serializes to singular `?owner=`. |
| `useOrgAnalytics` | `(initialParams) => UseOrgAnalyticsReturn` | Fetches org-level analytics. Single fetch on mount with manual refetch. |

### UI State

| Hook | Signature | Description |
|------|-----------|-------------|
| `useURLFilters` | `<T>(config) => URLFiltersResult<T>` | Generic URL filter persistence. Supports string, string[], boolean, and dateRange fields. Provides `setFilter`, `setAll`, `toggleArrayValue`, `clearAll`, and `commitHistory`. |
| `useSessionFilters` | `() => SessionFilters & Actions` | Reads/writes session filter state (repos, branches, owners, providers, query) to URL search params via `useURLFilters`. |
| `useProviderTranscriptFilters` | `<TState>(config) => { filterState, setFilterState, toggleCategory, toggleSubcategory }` | x5w2 — shared `?hide=` URL sync + tri-state category / subcategory toggles for the per-provider transcript filters. `config` supplies `defaultState`, `pathsFromState`, `stateFromPaths`, and `hierarchicalKeys`. Backs the three hooks below. |
| `useClaudeTranscriptFilters` | `() => ClaudeTranscriptFiltersResult` | Claude transcript message category visibility (URL `hide`), built on `useProviderTranscriptFilters`. Typed toggles for categories and subcategories (user/assistant/attachment). |
| `useCodexTranscriptFilters` | `() => CodexTranscriptFiltersResult` | CF-361 — Codex parallel. Same `?hide=` URL slot with provider-specific token grammar (`user`, `assistant.commentary`, `tool_call.exec_command`, …). Default-hidden: `reasoning_hidden`. Toggles for `category`, `assistantSubcategory`, `toolCallSubcategory`. |
| `useOpenCodeTranscriptFilters` | `() => { filterState, setFilterState, toggleCategory }` | x5w2 — OpenCode parallel (flat categories: `user`/`assistant`/`tool`/`unknown`). All visible by default; shares the `?hide=` slot. Consumed by `opencodeAdapter.useFilters`. |
| `useTranscriptSearch` | `<T>(items, extractText) => TranscriptSearchResult` | Generic over item type. Builds a lowercased search index via `extractText`, debounces query (150ms search, 300ms highlight), provides match navigation. Shared by the Claude (`extractClaudeMessageText` from `services/claudeMessageParser`), Codex (`extractCodexItemText` from `components/transcript/codex`), and OpenCode (`extractOpenCodeItemText` from `components/session`) timelines. |
| `useShareDialog` | `({ sessionId, userEmail?, onShareCreated? }) => UseShareDialogReturn` | Full share dialog state: form fields, email validation (Zod), create/revoke API calls. |
| `useDropdown` | `<T extends HTMLElement>(initialOpen?: boolean) => UseDropdownReturn<T>` | Open/close state with click-outside detection and Escape key. `initialOpen` defaults to `false`; pass `true` in stories/tests to render open. |
| `useSuccessMessage` | `(options?) => UseSuccessMessageReturn` | Auto-fading success message with optional URL param extraction. |
| `useCopyToClipboard` | `(options?) => UseCopyToClipboardReturn` | Clipboard write with configurable success duration. |
| `useAutoRetry` | `(retryFn, options) => UseAutoRetryReturn` | Exponential backoff with countdown display and exhaustion tracking. |

### Browser / Context

| Hook | Signature | Description |
|------|-----------|-------------|
| `useVisibility` | `() => boolean` | Returns `true` when the tab is in the foreground. |
| `useUserActivity` | `() => { isIdle, markActive }` | Tracks mouse/keyboard/scroll/touch events. Idle after configurable threshold. |
| `useRelativeTime` | `(dateStr) => string` | Returns auto-updating relative time string. Updates every 2s (<5min), 5s (<1h), or 60s (>1h). Pauses when tab hidden. |
| `useTheme` | `() => UseThemeReturn` | Light/dark theme state from `ThemeContext`. |
| `useAppConfig` | `() => AppConfig` | App configuration from `AppConfigContext`. |
| `useDocumentTitle` | `(title) => void` | Sets `document.title` with suffix, restores on unmount. |
| `useServerRecovery` | `(serverError) => void` | Watches for server error -> recovery transition and invalidates all non-auth query caches. |

## How to Extend

### Adding a new data fetching hook
1. Create `useNewThing.ts` following the patterns in `useTrends.ts` (simple) or `useSmartPolling.ts` (with polling)
2. Return `{ data, loading, error, refetch }` at minimum
3. Add to `index.ts` barrel export if it will be imported from multiple files
4. Add a `.test.ts` file

### Using smart polling for a new resource
Wrap `useSmartPolling` like `useAnalyticsPolling` does:
```typescript
const { data, state, refetch, loading, error } = useSmartPolling(fetchFn, {
  enabled: true,
  resetKey: resourceId,  // Triggers refetch on change
});
```

## Invariants / Conventions

- Hooks that interact with APIs use the service layer (`@/services/api`) rather than calling `fetch` directly
- All hooks clean up intervals/timeouts in effect cleanup functions
- Filter state is URL-synced via `useSearchParams` -- refreshing the page preserves filters
- Polling hooks respect tab visibility: no network requests when the tab is hidden
- `useSmartPolling` uses refs extensively to avoid stale closures in timeouts

## Design Decisions

- **React Query for auth only**: `useAuth` uses `@tanstack/react-query` for its caching and retry semantics. Other hooks use manual state management because they need custom polling behavior (visibility-aware, conditional 304) that doesn't fit React Query's model well.
- **URL-synced filters**: `useURLFilters` is the generic engine for URL filter persistence. `useSessionFilters` and `useClaudeTranscriptFilters` are thin wrappers that define field configs. Pages like `TrendsPage` and `OrgPage` use `useURLFilters` directly.
- **Adaptive relative time intervals**: `useRelativeTime` adjusts its update frequency based on timestamp age to avoid unnecessary renders for old timestamps while keeping recent ones fresh.

## Testing

Most hooks have co-located test files:
- `useAuth.test.tsx`, `useAutoRetry.test.ts`, `useDropdown.test.ts`
- `useColumnCount.test.tsx`
- `useLoadSession.test.ts`, `useSessionFilters.test.ts`, `useSessionsFetch.test.ts`
- `useOrgAnalytics.test.tsx`
- `useShareDialog.test.ts`, `useSmartPolling.test.ts`
- `useSuccessMessage.test.tsx`
- `useTrends.test.tsx`
- `useTranscriptSearch.test.ts`, `useRelativeTime.test.ts`
- `useVisibility.test.ts`, `useUserActivity.test.ts`
- `useServerRecovery.test.tsx`
- `useURLFilters.test.ts`

## Dependencies

- `react` (hooks API)
- `react-router-dom` (`useSearchParams` in `useURLFilters`, `useSuccessMessage`)
- `@tanstack/react-query` (`useAuth`, `useServerRecovery`)
- `@/services/api` (API client methods)
- `@/schemas/api` (response types)
- `@/schemas/validation` (Zod schemas for form validation in `useShareDialog`)
- `@/config/polling` (polling interval constants)
