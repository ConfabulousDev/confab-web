# hooks/

Custom React hooks for the Confab frontend. Organized by responsibility: data fetching, UI state management, auth, and browser APIs.

## Files

| File | Role |
|------|------|
| `useAuth.ts` | Authentication state via React Query (`/me` endpoint) |
| `useSessionsFetch.ts` | Paginated session list with server-side filtering and debounced search |
| `useSessionFilters.ts` | URL-synced filter state (repos, branches, owners, query) |
| `useLoadSession.ts` | Single session loading with typed error categories |
| `useAnalyticsPolling.ts` | Session analytics polling with conditional 304 support |
| `useSmartPolling.ts` | Generic smart polling with visibility/activity awareness |
| `useTrends.ts` | Trends data fetching with filter parameters |
| `useOrgAnalytics.ts` | Organization analytics data fetching |
| `useTranscriptSearch.ts` | Transcript search with debounced query and match navigation |
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
| `useTrends` | `(initialParams?) => UseTrendsReturn` | Fetches aggregated trends data. Single fetch on mount with manual refetch. |
| `useOrgAnalytics` | `(initialParams) => UseOrgAnalyticsReturn` | Fetches org-level analytics. Single fetch on mount with manual refetch. |

### UI State

| Hook | Signature | Description |
|------|-----------|-------------|
| `useSessionFilters` | `() => SessionFilters & Actions` | Reads/writes filter state (repos, branches, owners, query) to URL search params. |
| `useTranscriptSearch` | `(messages) => TranscriptSearchResult` | Builds a lowercased search index, debounces query (150ms search, 300ms highlight), provides match navigation. |
| `useShareDialog` | `({ sessionId, userEmail?, onShareCreated? }) => UseShareDialogReturn` | Full share dialog state: form fields, email validation (Zod), create/revoke API calls. |
| `useDropdown` | `<T extends HTMLElement>() => UseDropdownReturn<T>` | Open/close state with click-outside detection and Escape key. |
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
- **URL-synced filters**: `useSessionFilters` writes to `URLSearchParams` so filter state survives page refreshes and can be shared via URL.
- **Adaptive relative time intervals**: `useRelativeTime` adjusts its update frequency based on timestamp age to avoid unnecessary renders for old timestamps while keeping recent ones fresh.

## Testing

Most hooks have co-located test files:
- `useAuth.test.tsx`, `useAutoRetry.test.ts`, `useDropdown.test.ts`
- `useLoadSession.test.ts`, `useSessionFilters.test.ts`, `useSessionsFetch.test.ts`
- `useShareDialog.test.ts`, `useSmartPolling.test.ts`
- `useTranscriptSearch.test.ts`, `useRelativeTime.test.ts`
- `useVisibility.test.ts`, `useUserActivity.test.ts`
- `useServerRecovery.test.tsx`

## Dependencies

- `react` (hooks API)
- `react-router-dom` (`useSearchParams` in `useSessionFilters`, `useSuccessMessage`)
- `@tanstack/react-query` (`useAuth`, `useServerRecovery`)
- `@/services/api` (API client methods)
- `@/schemas/api` (response types)
- `@/schemas/validation` (Zod schemas for form validation in `useShareDialog`)
- `@/config/polling` (polling interval constants)
