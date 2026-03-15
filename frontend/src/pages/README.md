# pages/

Page-level components corresponding to routes. All pages are lazy-loaded for code splitting.

## Files

| File | Role |
|------|------|
| `HomePage.tsx` | Landing page with hero, quickstart CTA, and feature overview |
| `SessionsPage.tsx` | Paginated session list with server-side filtering (repos, branches, owners, search) |
| `SessionDetailPage.tsx` | Session detail view wrapping `SessionViewer` with share/delete modals |
| `TrendsPage.tsx` | Trends analytics dashboard with date range and repo filters |
| `OrgPage.tsx` | Organization-level analytics with per-user table |
| `APIKeysPage.tsx` | API key management (create, list, delete) |
| `ShareLinksPage.tsx` | List and manage active share links |
| `TILsPage.tsx` | TIL list with create/edit/delete for user's "Today I Learned" entries |
| `LoginPage.tsx` | OAuth login page with provider selection |
| `PoliciesPage.tsx` | Legal policies page (SaaS mode only) |
| `NotFoundPage.tsx` | 404 page |
| `pageLayout.module.css` | Shared page layout styles |

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
| `/tils` | `TILsPage` | Yes | Protected route |
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
- Manages `ShareModal` and delete confirmation
- Handles deep-linking to specific messages via `?msg=UUID` query param
- Switches to transcript tab automatically when a deep link is present
- Renders typed error states (not found, expired, forbidden, auth required)

### SessionsPage

- Uses `useSessionFilters` (URL-synced) + `useSessionsFetch` (API calls)
- Renders `FilterChipsBar` for active filter display
- Shows `SessionEmptyState` with `QuickstartCTA` for new users

### TrendsPage

- Date range picker with presets (This Week, Last 7 Days, Last 30 Days, etc.)
- Repo filter multi-select
- Renders trend cards from `@/components/trends/cards/`

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
- Page-specific styles use CSS Modules; shared layout styles are in `pageLayout.module.css`
- `useDocumentTitle()` is called in each page to set the browser tab title
- Protected pages use `useAuth()` to check authentication state

## Design Decisions

- **Lazy loading**: Every page is code-split via `React.lazy()` to keep the initial bundle small. The `Suspense` fallback is `null` (no loading spinner) for instant perceived navigation.
- **URL-synced filters**: `SessionsPage` and `TrendsPage` store filter state in URL search params so filters survive page refreshes and can be bookmarked/shared.
- **Unified session access**: `SessionDetailPage` doesn't distinguish between owner/shared/public access upfront. It fetches the session and lets the backend return the appropriate data or error (401/403/404/410).

## Testing

- `SessionDetailPage.test.tsx` -- Session loading, error states, deep linking
- `LoginPage.test.tsx` -- Login form rendering, OAuth provider display

## Dependencies

- `react-router-dom` (routing, `useParams`, `useSearchParams`, `useNavigate`)
- `@/hooks/` (useAuth, useSessionsFetch, useSessionFilters, useLoadSession, useTrends, useOrgAnalytics, useDocumentTitle, etc.)
- `@/services/api` (API client for direct calls in some pages)
- `@/components/` (UI components)
