# Confab Development Notes

Main web repo. CLI lives in https://github.com/ConfabulousDev/confab (separate repo).

## What belongs in this file

Cross-cutting rules Claude would get wrong by default. Add to this file only when the rule is (a) cross-package, (b) non-obvious from reading the code, and (c) Claude would get it wrong without the instruction. If it's tied to one package, put it in that package's README. If it's tied to one subtree, put it in `backend/CLAUDE.md` or `frontend/CLAUDE.md`.

## API documentation

Backend API is documented in `backend/API.md`. Keep it up to date when modifying API endpoints, request/response schemas, or authentication.

## Documentation maintenance

Two audiences, two doc trees:

- **Internal docs** (READMEs, `CLAUDE.md` files, `backend/API.md`, `CONFIGURATION.md`, `SELF-HOSTING.md`) — for contributors. When changing code, update the corresponding package/module README. Keep current: file lists, exported API descriptions, invariants, dependency lists, extension checklists. If a change spans multiple packages, also check the index READMEs (`backend/internal/README.md`, `frontend/src/README.md`) and this file.
- **User-facing docs site** (`docs/` — Starlight, published at `docs.confabulous.dev`) — for end users and self-hosters. When changing user-visible behavior (new config var, new feature, changed CLI flow), update the corresponding page under `docs/src/content/docs/`. The sidebar tree is in `docs/astro.config.mjs`.

Documentation that contradicts the code is worse than no documentation.

## Shared-code locations (DRY)

Before adding new logic, check the package README for an existing helper. Grep the package first; duplicating a regex, a SQL fragment, or a utility across packages is the most common source of drift.

- `backend/internal/analytics/` — analytics compute, per-provider dispatch, smart recap, trends, org analytics, search index.
- `backend/internal/auth/` — auth middleware, OAuth providers, demo identity, read-only enforcement.
- `backend/internal/db/` — shared DB types/helpers, session visibility CTE, repo-filter SQL fragments, fork→root resolver, codex sidecar.
- `backend/internal/models/` — domain types and provider identity (`NormalizeProvider`, `AllowedProviders`, legacy alias map).
- `frontend/src/providers/` — per-provider transcript adapters behind a shared `ProviderAdapter` interface; `frontend/src/utils/providers.ts` — cosmetic registry (icon/label/color).
- `frontend/src/utils/tokenStats.ts` — canonical `TokenUsage` shape and provider-keyed pricing table.
- For shared DB structs (`SessionDetail`, `SessionListItem`, …): use the shared column-list/Scan-target helpers and add a wire-level integration test. Full procedure in `backend/CLAUDE.md`.

## Per-area conventions

- Backend commands, sharded tests, dead-code tools → `backend/CLAUDE.md`.
- Frontend build/lint/test, Storybook, theme, knip → `frontend/CLAUDE.md`.
- Docs site (Starlight) — sentence case, IA, source-of-truth pages → `docs/CLAUDE.md`.

## Skills

- `/add-session-card` — full playbook for new analytics cards (DB migration, collector, types, store, compute, frontend Zod schema + component + registry, Storybook).
- `/cut-release` — tag, write release notes (with rigorous migration + API verification), publish via gh.
