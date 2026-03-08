# components/

Reusable UI components and domain-specific component subdirectories for the Confab frontend.

## Organization

Root-level files are **shared, reusable components** used across multiple pages. Domain-specific components live in subdirectories:

- `session/` -- Session viewer, summary panel, analytics cards, message timeline
- `transcript/` -- Transcript rendering (code blocks, bash output, timeline bars)
- `trends/` -- Trends analytics dashboard cards
- `org/` -- Organization analytics (OrgTable, OrgFilters)

## Files

| File | Role |
|------|------|
| `Alert.tsx` | Dismissible alert banner (success/error/info) |
| `AnalysisModal.tsx` | Modal for viewing session analysis details |
| `Button.tsx` | Styled button with variants |
| `CardGrid.module.css` | Grid layout for analytics cards |
| `CardGrid.tsx` | CSS grid container for rendering card layouts |
| `Chip.tsx` | Tag/chip component for filter selections |
| `DeployCTA.tsx` | Call-to-action for self-hosted deployment |
| `ErrorBoundary.tsx` | React error boundary with fallback UI |
| `ErrorDisplay.tsx` | Styled error message display |
| `FilterChipsBar.tsx` | Horizontal bar of active filter chips with clear-all |
| `Footer.tsx` | App footer (SaaS mode only) |
| `FormField.tsx` | Form field wrapper with label and validation error display |
| `Header.tsx` | App header with navigation and auth state |
| `HeroCards.tsx` | Landing page hero section cards |
| `HowItWorksModal.tsx` | Onboarding modal explaining the product |
| `icons.tsx` | SVG icon components (ClaudeCodeIcon, GitHubIcon, etc.) |
| `LoadingSkeleton.tsx` | Animated loading placeholder |
| `Modal.tsx` | Base modal component with backdrop and close handling |
| `PageHeader.tsx` | Page-level header with title and optional actions |
| `PageSidebar.tsx` | Page-level sidebar for filters and navigation |
| `Pagination.tsx` | Cursor-based pagination controls (prev/next) |
| `PRLinkingModal.tsx` | Modal for linking GitHub PRs to sessions |
| `ProtectedRoute.tsx` | Route wrapper that requires authentication |
| `Quickstart.tsx` | Quickstart guide component |
| `QuickstartCTA.tsx` | Call-to-action directing users to quickstart |
| `QuickstartModal.tsx` | Modal with quickstart instructions |
| `RelativeTime.tsx` | Auto-updating relative timestamp display |
| `ReviewModal.tsx` | Modal for reviewing session details |
| `ScrollNavButtons.tsx` | Floating scroll-to-top/bottom buttons |
| `SelfHostedModal.tsx` | Modal with self-hosted setup instructions |
| `ServerError.tsx` | Full-page server error state with auto-retry |
| `SessionEmptyState.tsx` | Empty state when no sessions exist |
| `ShareDialog.tsx` | Dialog for sharing sessions (public/private, recipients) |
| `ShareModal.tsx` | Modal wrapper around ShareDialog |
| `SmartRecapModal.tsx` | Modal for displaying AI-generated smart recap |
| `SortableHeader.tsx` | Table header with sort direction indicator |
| `ThemedImage.tsx` | Image component that swaps src based on light/dark theme |
| `ThemeToggle.tsx` | Light/dark theme toggle button |

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
