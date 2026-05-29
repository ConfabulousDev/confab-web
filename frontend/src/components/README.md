# components/

Reusable UI components and domain-specific component subdirectories for the Confab frontend.

## Organization

Root-level files are **shared, reusable components** used across multiple pages. Domain-specific components live in subdirectories:

- `charts/` -- Shared chart primitives (e.g., `TruncatedYAxisTick` custom Recharts tick that caps long Y-axis labels and exposes the full value via `<title>` hover; used by Agents & Skills + Tools cards in both session and trends scopes)
- `filters/` -- Page-level filter dropdowns shared by `org/OrgFilters` and `trends/TrendsFilters`: `RepoFilter` (repo multi-select + include-no-repo toggle + Clear) and `ProviderFilter` (provider multi-select). Both style directly from `styles/filterDropdown.module.css` rather than a co-located module (CF-508)
- `session/` -- Session viewer, summary panel, analytics cards, message timeline
- `transcript/` -- Transcript rendering (code blocks, bash output, timeline bars)
- `trends/` -- Trends analytics dashboard cards
- `org/` -- Organization analytics (OrgTable, OrgFilters)

## Files

| File | Role |
|------|------|
| `Alert.tsx` | Dismissible alert banner (success/error/info) |
| `Button.tsx` | Styled button with variants |
| `CardGrid.module.css` | Grid layout for analytics cards |
| `CardGrid.tsx` | CSS grid container for rendering card layouts |
| `Chip.tsx` | Tag/chip component for filter selections |
| `CopyIdDropdown.tsx` | Dropdown for copying Confab ID or the agent-native session ID (Claude Code / Codex) with confirmation feedback; label switches per `provider` |
| `CTALinks.tsx` | Trio of italic, color-coded text links (Demo → demo.confabulous.dev, Docs → docs.confabulous.dev Introduction page, GitHub → confab-web repo). Each link gets its own semantic color (`--color-accent` / `--color-selected` / `--color-success`). Rendered above and below the `HeroCards` grid on the landing page. |
| `ErrorBoundary.tsx` | React error boundary with fallback UI |
| `ErrorDisplay.tsx` | Styled error message display |
| `FilterChipsBar.tsx` | Horizontal bar of dimension dropdowns + active filter chips with clear-all and optional history commit on blur. Dimensions: Provider (static enum, opt out via `showProviderFilter={false}`), Repo, Branch, Owner. `DimensionDropdown` (named export) accepts optional `iconFor` / `labelFor` for per-option icons and display labels (used by Provider), and `initialOpen` for stories/tests. Renders a subtle divider between selected and unselected items when both groups are present. |
| `Footer.tsx` | App footer (SaaS mode only) |
| `FormField.tsx` | Form field wrapper with label and validation error display |
| `Header.tsx` | App header with navigation and auth state. Renders a "demo" badge next to the logo when `window.__DEMO_IDENTITY__` is set (CF-483); no badge in normal deployments. Sessions, Trends, and TILs nav links pre-fill `?owner=<your email>` for normal users (CF-495 added Trends to the pattern; nav label renamed "Personal Trends" → "Trends"); the demo identity skips the pre-filter so the page isn't collapsed to zero rows. |
| `HeroCards.tsx` | Landing page hero grid (CF-503). Each card has a fixed icon + title + description and a right-aligned link footer: `Demo →` (when a deep-link exists on `demo.confabulous.dev`) followed by one or more docs links into `docs.confabulous.dev`. Multi-provider is the only 3-link card (`Demo →   Claude Code →   Codex →`). Every link opens in a new tab with `rel="noopener noreferrer"` and an `aria-label` of `'{card title}: {link label}[ docs]'` to disambiguate the 20+ identically-named links for screen readers. Cards are not click targets. |
| `icons.tsx` | SVG icon components (ClaudeCodeIcon, CodexIcon, GitHubIcon, etc.) |
| `providerIcon.ts` | `getProviderIcon(provider)` -- delegates to `getProviderMetadataOrFallback(provider, 'neutral')` and falls back to `RobotIcon` when no metadata matches (CF-366). Canonical and legacy values (`'claude-code'`, `'codex'`, `'Claude Code'`, `'CLAUDE-CODE'`) still resolve to their brand icon; truly unknown values render the neutral glyph rather than impersonating Claude. Lives in its own file to keep `icons.tsx` JSX-only for HMR fast-refresh |
| `LoadingSkeleton.tsx` | Animated loading placeholder |
| `Modal.tsx` | Base modal component with backdrop and close handling |
| `PageHeader.tsx` | Page-level header with title and optional actions |
| `PageSidebar.tsx` | Page-level sidebar for filters and navigation |
| `Pagination.tsx` | Cursor-based pagination controls (prev/next) |
| `ProtectedRoute.tsx` | Route wrapper that requires authentication |
| `Quickstart.tsx` | Quickstart guide component |
| `QuickstartCTA.tsx` | Call-to-action directing users to quickstart |
| `QuickstartModal.tsx` | Modal wrapping `Quickstart` (rendered by `QuickstartCTA` on the empty Sessions page) |
| `ReadOnlyToast.tsx` | CF-483 toast that listens for the `confab:read-only` CustomEvent (dispatched by `services/api.ts` when an API call returns the `read_only_user` structured 403) and shows a transient "This is a read-only demo." message. Single toast at a time; re-firing while visible resets the dismiss timer (debounced replace). |
| `RelativeTime.tsx` | Auto-updating relative timestamp display |
| `ScrollNavButtons.tsx` | Floating scroll-to-top/bottom buttons |
| `ServerError.tsx` | Full-page server error state with auto-retry |
| `SessionEmptyState.tsx` | Empty state when no sessions exist |
| `ShareDialog.tsx` | Dialog for sharing sessions (public/private, recipients) |
| `SortableHeader.tsx` | Table header with sort direction indicator |
| `ThemeToggle.tsx` | Light/dark theme toggle button |
| `UpdateBadge.tsx` | Container: reads `version` from `useAppConfig()` and decides whether to show. Mounted in `Header.tsx` for authenticated users only |
| `UpdateBadgeView.tsx` | Pure presentational pill: small "Update available" link to the latest GitHub release; tooltip shows `current → latest` (or `(dev) → latest` when running unversioned). Stories drive this directly so visuals are stable without mocking hooks |

## Key Patterns

### CSS Modules

Every component uses co-located CSS Modules (`Component.module.css`). Import as:
```tsx
import styles from './Component.module.css';
```

Use theme-aware CSS variables from `styles/variables.css` (e.g., `--color-bg-primary`, `--color-text-secondary`). Never hardcode colors.

### Storybook Requirement

All new or modified components must have a corresponding `.stories.tsx` file. Stories live alongside their component. Verify with:
```bash
cd frontend && npm run build-storybook
```

### Component Conventions

- Function components (no class components)
- Default exports for page-level components, named exports for utilities
- Props interfaces defined in the same file
- Barrel exports (`index.ts`) used sparingly -- only in `session/`, `session/cards/`, and `trends/cards/`

## Dependencies

- React 19 with hooks
- `react-router-dom` for navigation (ProtectedRoute, Header)
- `@tanstack/react-query` for data fetching (used in hooks, consumed by components)
- CSS Modules for styling (no CSS-in-JS)
