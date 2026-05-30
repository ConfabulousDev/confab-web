# Confab Frontend

React + TypeScript single-page application for the Confab dashboard. For an architectural overview, see [`src/README.md`](src/README.md).

## Tech Stack

- **React 19** with hooks
- **TypeScript** with strict mode
- **Vite** — dev server and build
- **React Router** — client-side routing with `lazy()` code splitting
- **TanStack Virtual** — virtual scrolling for large transcripts
- **TanStack Query** — server state management
- **Zod** — runtime validation of API responses (single source of truth for response types via `z.infer<>`)
- **CSS Modules** — scoped component styling, theme-aware variables in `src/styles/variables.css`
- **Storybook** — component stories alongside components
- **Vitest** — unit tests

## Project Layout

See [`src/README.md`](src/README.md) for the module-by-module index and data-flow diagram. Highlights:

- `src/pages/` — route components, lazy-imported in `router.tsx`.
- `src/hooks/` — data fetching, polling, auth, UI state.
- `src/services/` — API client wrapper (Zod-validated).
- `src/schemas/` — Zod schemas for API responses.
- `src/components/` — shared UI; session/cards live under `components/session/cards/`.
- `src/providers/` — per-provider transcript adapters (Claude, Codex) behind a shared `ProviderAdapter` interface.
- `src/utils/` — pure helpers (formatting, date ranges, pricing, providers).

## Development

For the full stack, run `make dev` from the repo root — see [Local Development](../README.md#local-development). Frontend-only commands:

```bash
npm install
npm run dev          # Vite dev server at http://localhost:5173 (proxies /api → :8080)
npm run build        # type-check + Vite production build
npm run lint         # ESLint (must have 0 errors)
npm test             # Vitest unit tests
npm run storybook    # local Storybook
npm run build-storybook
npm run knip         # dead-code detection
```

Always run `npm run build && npm run lint && npm test` before declaring work done; see `./CLAUDE.md` for the frontend-specific commands, Storybook expectations, and theming rules, and `../CLAUDE.md` for project-wide conventions.

## Theming

The app supports light and dark themes via the `[data-theme]` attribute on `<html>`. Always use CSS custom properties from `src/styles/variables.css` (`--color-bg-primary`, `--color-text-secondary`, `--color-accent`, etc.) — never hardcode colors.

## API Integration

The dev server proxies `/api/*` to the Go backend at `http://localhost:8080` (configured in `vite.config.ts`). All API calls go through `src/services/api.ts`, which validates every response with Zod schemas from `src/schemas/`. Type definitions are inferred from those schemas via `z.infer<>`.

CSRF protection is server-side via Fetch metadata headers — there is no client-side token to manage.

## License

MIT — see the repository root.
