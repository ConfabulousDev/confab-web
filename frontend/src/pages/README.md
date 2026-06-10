# pages/

Page-level components corresponding to routes. All pages are lazy-loaded for code splitting.

## Files

| File | Role |
|------|------|
| `HomePage.tsx` | Landing page with hero, quickstart CTA, and feature overview. Auto-redirects authenticated users to `/sessions?owner=<your email>` (or plain `/sessions` for the demo identity, which owns nothing). |
| `SessionsPage.tsx` | Paginated session list with server-side filtering (repos, branches, owners, search) |
| `SessionDetailPage.tsx` | Session detail view wrapping `SessionViewer` with share/delete modals |
| `TrendsPage.tsx` | Trends analytics dashboard with date range, repo, provider, and owner (CF-495) filters. Document title + heading "Trends" (was "Personal Trends"). Owner + repo dropdown options come from `data.filter_options` — no side-call to `/api/sessions`. Owner-narrowed empty state offers a one-click clear-filter CTA. The Costliest Sessions card's 10/25/50 top-N selector (h7xe) is URL-synced page state (`?topN=`, sent to the backend as `?top_n=`), kept separate from the filter-bar value but committed through the same refetch path. |
| `OrgPage.tsx` | Organization-level analytics with per-user table |
| `APIKeysPage.tsx` | API key management (create, list, delete) |
| `LoginPage.tsx` | OAuth login page with provider selection. Carries understated "Docs" (`DOCS_URL`) and "Report an issue" (`GITHUB_ISSUES_URL`) links below the auth options (CF-571); these only render when the form is shown (not during single-provider auto-redirect). |
| `PoliciesPage.tsx` | Legal policies page (SaaS mode only) |
| `NotFoundPage.tsx` | 404 page |
| `pageLayout.module.css` | Shared page layout styles (layout, page title, refresh button, toolbar actions, filter bar) |

## Routing

Routes are defined in `src/router.tsx` using `createBrowserRouter`. All page components are lazy-loaded with `React.lazy()`:

```typescript
const SessionsPage = lazy(() => import('@/pages/SessionsPage'));
```

### Route Table

| Path | Component | Auth Required | Notes |
|------|-----------|---------------|-------|
| `/` | `HomePage` | No | Landing page |
| `/sessions` | `SessionsPage` | Yes | Protected route |
| `/sessions/:id` | `SessionDetailPage` | No | Handles owner, shared, and public access |
| `/trends` | `TrendsPage` | Yes | Protected route |
| `/org` | `OrgPage` | Yes | Protected + org analytics feature flag |
| `/keys` | `APIKeysPage` | Yes | Protected route |
| `/shares` | `ShareLinksPage` | Yes | Protected route |
| `/login` | `LoginPage` | No | |
| `/policies` | `PoliciesPage` | No | SaaS mode only |
| `/terms` | Redirect | No | External Termly redirect, SaaS only |
| `/privacy` | Redirect | No | External Termly redirect, SaaS only |
| `*` | `NotFoundPage` | No | Catch-all |

### Access Control

- **`ProtectedRoute`** wraps authenticated pages -- redirects to login if not authenticated
- **`SaasRoute`** gates SaaS-only pages -- returns `NotFoundPage` when SaaS footer is disabled
- **`OrgAnalyticsRoute`** gates org analytics -- shows disabled message when feature flag is off
- **`SessionDetailPage`** handles all access types (owner, private share, public share) without requiring authentication upfront. The backend determines access.

### Legacy URL Support

`/sessions/:sessionId/shared/:token` redirects to `/sessions/:sessionId` preserving query params. This supports old share URLs from before share access was unified.

## Key Components

### SessionDetailPage

The most complex page. It:
- Loads session data via `useLoadSession`
- Manages `ShareDialog` and delete confirmation
- Handles deep-linking to specific messages via `?msg=UUID` query param
- Switches to transcript tab automatically when a deep link is present
- Renders typed error states (not found, expired, forbidden, auth required)

### SessionsPage

- Uses `useSessionFilters` (URL-synced) + `useSessionsFetch` (API calls)
- Renders `FilterChipsBar` for active filter display
- Shows `SessionEmptyState` with `QuickstartCTA` for new users

### TrendsPage

- Date range picker with presets (This Week, Last 7 Days, Last 30 Days, etc.)
- Repo filter multi-select (source: `data.filter_options.repos`)
- AI provider filter (CF-424): canonical values `claude-code` / `codex`; URL-persisted via singular `?provider=` key; empty state = aggregate across all providers
- Owner filter (CF-495): multi-select narrowing within the visible-session set; URL-persisted via singular `?owner=` key; viewer's own email pinned to the top in the dropdown; owner-narrowed empty state shows a clear-filter CTA. Source: `data.filter_options.owners` (static across active filters)
- Renders trend cards from `@/components/trends/cards/`. `TrendsTokensCard` self-switches between single-series StatRows and indented per-provider sections based on `data.cards.tokens.per_provider` (CF-472, replaces the CF-435 table)

## How to Extend

### Adding a new page
1. Create `NewPage.tsx` with a default export
2. Add the lazy import and route in `src/router.tsx`
3. Wrap with `ProtectedRoute` if authentication is required
4. Create a `.stories.tsx` file
5. Add a `.module.css` file for page-specific styles

## Invariants / Conventions

- All pages are default exports (required for `React.lazy()`)
- Pages compose components from `@/components/` -- they should not contain complex rendering logic
- All top-level pages use `PageHeader` for the page heading (`<h1>` at 1.25rem/600 weight via shared `.pageTitle`)
- Page-specific styles use CSS Modules; shared layout styles are in `pageLayout.module.css`
- `useDocumentTitle()` is called in each page to set the browser tab title
- Protected pages use `useAuth()` to check authentication state

## Design Decisions

- **Lazy loading**: Every page is code-split via `React.lazy()` to keep the initial bundle small. The `Suspense` fallback is `null` (no loading spinner) for instant perceived navigation.
- **URL-synced filters**: `SessionsPage`, `TrendsPage`, and `OrgPage` store filter state in URL search params via `useURLFilters`/`useSessionFilters` so filters survive page refreshes and can be bookmarked/shared.
- **Unified session access**: `SessionDetailPage` doesn't distinguish between owner/shared/public access upfront. It fetches the session and lets the backend return the appropriate data or error (401/403/404/410).

## Testing

- `SessionDetailPage.test.tsx` -- Session loading, error states, deep linking
- `LoginPage.test.tsx` -- Login form rendering, OAuth provider display

## Dependencies

- `react-router-dom` (routing, `useParams`, `useSearchParams`, `useNavigate`)
- `@/hooks/` (useAuth, useSessionsFetch, useSessionFilters, useLoadSession, useTrends, useOrgAnalytics, useDocumentTitle, etc.)
- `@/services/api` (API client for direct calls in some pages)
- `@/components/` (UI components)
