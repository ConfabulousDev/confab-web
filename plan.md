# CF-29: Session List Filtering Expansion

## Overview
Expand the session list filtering by adding hostname and title text filters, integrated into the existing `SessionsFilterDropdown` component.

## Current State
- `SessionsFilterDropdown` supports filtering by **repository** and **branch**
- Filter state is managed in `useSessionFilters` hook with URL persistence
- Sessions have `hostname` and title fields (`summary`, `first_user_message`, `custom_title`)

## Implementation Plan

### 1. Update `useSessionFilters` hook
**File:** `frontend/src/hooks/useSessionFilters.ts`

Add new filter state:
- `selectedHostname: string | null` - hostname filter (URL param: `hostname`)
- `searchQuery: string` - title text search (URL param: `q`)

Add new actions:
- `setSelectedHostname(value: string | null)`
- `setSearchQuery(value: string)`

### 2. Update `SessionsFilterDropdown` component
**File:** `frontend/src/components/SessionsFilterDropdown.tsx`

Add new props:
- `hostnames: string[]` - list of unique hostnames
- `selectedHostname: string | null`
- `hostnameCounts: Record<string, number>`
- `onHostnameClick: (hostname: string | null) => void`
- `searchQuery: string`
- `onSearchChange: (query: string) => void`

UI changes:
1. Add a **search input** at the top of the dropdown (before "All Sessions")
   - Text input with search icon
   - Placeholder: "Search by title..."
   - Debounced to avoid excessive re-renders
2. Add a **Hostnames section** after the Branches section
   - Similar UI pattern to repos/branches (checkboxes with counts)
   - Uses ComputerIcon
   - Only shown when there are hostnames to display

### 3. Update `SessionsFilterDropdown.module.css`
**File:** `frontend/src/components/SessionsFilterDropdown.module.css`

Add styles for:
- `.searchInput` - text input styling
- `.searchWrapper` - container with search icon

### 4. Update `SessionsPage` component
**File:** `frontend/src/pages/SessionsPage.tsx`

Changes:
- Extract unique hostnames from sessions (similar to repos/branches)
- Calculate hostname counts
- Pass new props to `SessionsFilterDropdown`
- Update filter logic in `sortedSessions` to include:
  - Hostname filter: `if (selectedHostname && s.hostname !== selectedHostname) return false`
  - Title search: case-insensitive match on `summary`, `first_user_message`, or `custom_title`

### 5. Update tests and stories
- **Stories:** Update `SessionsFilterDropdown.stories.tsx` with new props and add stories for hostname filter and search
- **Tests:** Add unit tests for the new filtering logic in `useSessionFilters`

## File Changes Summary

| File | Change |
|------|--------|
| `frontend/src/hooks/useSessionFilters.ts` | Add hostname and search state |
| `frontend/src/components/SessionsFilterDropdown.tsx` | Add hostname section and search input |
| `frontend/src/components/SessionsFilterDropdown.module.css` | Add search input styles |
| `frontend/src/pages/SessionsPage.tsx` | Wire up new filters |
| `frontend/src/components/SessionsFilterDropdown.stories.tsx` | Add new stories |
| `frontend/src/hooks/useSessionFilters.test.ts` | Add tests for new filters |

## UI Mockup

```
┌─────────────────────────────────┐
│ Session Filters                 │
├─────────────────────────────────┤
│ ┌─────────────────────────────┐ │
│ │ 🔍 Search by title...       │ │
│ └─────────────────────────────┘ │
│                                 │
│ ☑ All Sessions              80 │
│ ─────────────────────────────── │
│ REPOSITORIES                    │
│ ☐ confab-web                45 │
│ ☐ confab-cli                23 │
│ ─────────────────────────────── │
│ HOSTNAMES                       │
│ ☐ macbook-pro               38 │
│ ☐ desktop-linux             25 │
│ ☐ work-laptop               17 │
└─────────────────────────────────┘
```

## Notes
- Hostname filter only makes sense for owned sessions (not shared), since `hostname` is owner-only
- Search is case-insensitive and matches against any title field
- All filters are cumulative (AND logic)
- Clear search resets text filter but preserves other filters
