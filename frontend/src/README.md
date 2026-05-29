# Frontend Module Index

React + TypeScript single-page application for the Confab web dashboard.
Built with Vite, styled with CSS Modules, tested with Vitest, documented
with Storybook.

## Module Index

| Directory | Purpose | Change this when... |
|-----------|---------|---------------------|
| `components/` | Shared UI components (Button, Alert, Modal, Header, etc.) | Adding reusable UI elements, changing design system |
| `components/filters/` | Page-level filter dropdowns shared by OrgFilters/TrendsFilters: `RepoFilter` (repo multi-select + include-no-repo + Clear), `ProviderFilter` (provider multi-select). All styled from `styles/filterDropdown.module.css` (CF-508) | Changing repo/provider filter behavior; adding a new shared filter dropdown |
| `components/session/` | Session detail view: provider-agnostic shell (`SessionViewer`, `SessionHeader`) that delegates per-provider transcript fetch / filter / rendering to `providers/` adapters (CF-417). The Summary tab is shared (`SessionSummaryPanel`, CF-364) | Changing session detail layout, adding session UI features |
| `components/session/cards/` | Analytics card components + registry (TokensCard, ToolsCard, SmartRecapCard, etc.) | Adding new analytics cards, changing card layout |
| `components/transcript/` | Claude transcript rendering: content blocks, code blocks, timeline/cost bars, and `attachments/` renderers for `attachment.*` side-channel rows + `away_summary` | Changing how Claude transcript messages are displayed |
| `components/transcript/codex/` | Codex transcript rendering: virtualized timeline + turn-based navigation rail (`CodexTimelineBar`) + per-item renderers (user, assistant, tool call, turn separator, reasoning placeholder, compaction divider, unknown fallback) with `isSelected` / `isNewSpeaker` hover/selection state | Changing how Codex transcript items are displayed |
| `config/` | App configuration constants (polling intervals) | Changing polling behavior, adding feature flags |
| `contexts/` | React contexts: ThemeContext, AppConfigContext, KeyboardShortcutContext | Adding app-wide state, changing context providers |
| `hooks/` | Custom React hooks: data fetching, polling, auth, UI state | Adding data-fetching logic, changing state management |
| `pages/` | Route-level page components (SessionsPage, TrendsPage, LoginPage, etc.) | Adding new pages/routes, changing page layout |
| `providers/` | Per-provider transcript adapters: `ProviderAdapter` interface, `getAdapter()` registry, `useTranscriptData` / `useSessionTILs` shared hooks, per-provider `calculateMessageCost` / `extendCostTooltip` (CF-418). SessionViewer dispatches through this layer instead of branching on `session.provider` (CF-417) | Adding a new provider's transcript pipeline; changing how SessionViewer fetches / filters / renders per provider |
| `schemas/` | Zod schemas for API response validation and transcript parsing | Changing API contracts, adding new response types |
| `services/` | API client (fetch wrapper + Zod validation), transcript/message parsing. Both transcript services stamp canonical `TokenUsage` (`input`/`output`/`cacheWrite`/`cacheRead`) on assistant messages at parse time (CF-418) | Changing API calls, adding new endpoints |
| `styles/` | CSS variables for theme support (light/dark), shared CSS module base styles | Changing theme colors, adding design tokens, extracting shared component styles |
| `test/` | Test setup (Vitest configuration) | Changing test infrastructure |
| `test-fixtures/` | Shared per-provider fixtures: `codex-rollout.jsonl`, plus `makeSessionFixture(provider, overrides)` / `makeSessionDetailFixture(provider, overrides)` (CF-420) | Adding per-provider default test data; reducing fixture churn when a new provider lands |
| `types/` | Shared TypeScript type definitions | Adding cross-module types |
| `utils/` | Pure utility functions: formatting, date ranges, canonical `TokenUsage` + provider-keyed pricing (CF-418), sorting | Adding helper functions, changing display formatting |

## Data Flow

How data flows from the backend API to the rendered UI:

```
Backend API (/api/v1/...)
      │
      ▼
┌──────────────────────────────────────────────┐
│  services/api.ts                             │
│  Centralized fetch wrapper                   │
│  - Sends requests with credentials           │
│  - Validates responses with Zod schemas      │
│  - Handles 401 → redirect to login           │
│  - Re-exports typed response data            │
└──────────────────┬───────────────────────────┘
                   │
                   ▼
┌──────────────────────────────────────────────┐
│  schemas/api.ts                              │
│  Zod schemas = single source of truth        │
│  - Define shape of every API response        │
│  - Runtime validation (parse, not assert)    │
│  - Infer TypeScript types with z.infer<>     │
└──────────────────┬───────────────────────────┘
                   │
                   ▼
┌──────────────────────────────────────────────┐
│  hooks/                                      │
│  Custom hooks own all data-fetching logic    │
│  - useSessionsFetch: session list + filters  │
│  - useLoadSession: session detail loading     │
│  - useAnalyticsPolling: analytics cards      │
│  - useTrends: trends aggregation             │
│  - useAuth: login state                      │
│  - useSmartPolling: adaptive poll intervals  │
└──────────────────┬───────────────────────────┘
                   │
                   ▼
┌──────────────────────────────────────────────┐
│  pages/                                      │
│  Route-level components                      │
│  - Compose hooks + components                │
│  - Handle URL params and navigation          │
│  - Lazy-loaded for code splitting            │
└──────────────────┬───────────────────────────┘
                   │
                   ▼
┌──────────────────────────────────────────────┐
│  components/                                 │
│  Presentational UI                           │
│  - Receive data as props                     │
│  - Render with CSS Modules                   │
│  - Emit callbacks for user actions           │
└──────────────────────────────────────────────┘
```

## Key Architectural Patterns

### Zod Schemas as Single Source of Truth

All API response types are defined as Zod schemas in `schemas/api.ts`.
TypeScript types are inferred from these schemas with `z.infer<>`. The API
client (`services/api.ts`) validates every response at runtime, catching
backend contract changes immediately rather than letting bad data propagate
through the UI.

```
schemas/api.ts  →  defines SessionDetailSchema
                   exports type SessionDetail = z.infer<typeof SessionDetailSchema>
services/api.ts →  uses validateResponse(SessionDetailSchema, data, endpoint)
hooks/          →  consumes typed SessionDetail
components/     →  renders typed SessionDetail
```

### Hooks for Logic, Components for Rendering

- **Hooks** (`hooks/`) own all side effects: API calls, polling, URL state,
  timers. Pages compose hooks to get data.
- **Components** (`components/`) are primarily presentational. They receive
  data via props and emit user actions via callbacks.
- **Pages** (`pages/`) wire hooks to components and handle routing concerns
  (URL params, navigation, code splitting via `lazy()`).

### CSS Modules + Theme Variables

- Every component uses a co-located `.module.css` file for scoped styles.
- Colors and spacing use CSS custom properties from `styles/variables.css`.
- The `[data-theme="dark"]` selector drives dark mode. Never hardcode colors.

### Storybook

- Stories live alongside components (`Component.stories.tsx` next to
  `Component.tsx`).
- All new or modified components must have corresponding stories.
- Run `npm run build-storybook` to verify, `npm run storybook` to preview.

### Lazy Loading / Code Splitting

- All page components are `lazy()`-imported in `router.tsx`.
- Each page becomes a separate JS chunk, loaded on navigation.
- Shared components are bundled into the main chunk.

### Analytics Card Registry

Session analytics cards use a registry pattern
(`components/session/cards/registry.ts`). To add a new card:

1. Create the card component in `components/session/cards/`.
2. Register it in `registry.ts`.
3. Add the corresponding Zod schema fields in `schemas/api.ts`.

See the `/add-session-card` skill in CLAUDE.md for the full playbook.

### Smart Polling

The `useSmartPolling` hook provides adaptive polling that slows down when the
user is idle or the tab is hidden, and speeds up when the user returns. Used
by `useAnalyticsPolling` to avoid unnecessary API calls.
