# Frontend Development Notes

## What belongs in this file

Frontend conventions Claude would get wrong by default — commands, theme rules, build/lint/test gates. For architecture see `README.md`; for the module index see `src/README.md`. Add to this file only when the rule is (a) frontend-wide, (b) non-obvious from reading the code, and (c) Claude would get it wrong without the instruction.

## Build, lint, test

Always run from the `frontend/` directory:

```bash
npm run build && npm run lint && npm test
```

- **build**: TypeScript compile + Vite build. Must succeed.
- **lint**: ESLint, must have 0 errors (warnings are fine).
- **test**: Vitest. All tests must pass.

Use `npm run` — never invoke `tsc`, `eslint`, or `vitest` directly. They live in `node_modules/.bin` which `npm run` adds to PATH automatically. If commands fail with "command not found", run `npm install` first.

## Storybook

All new or modified components get a story (`Component.stories.tsx` next to `Component.tsx`). Visual regression coverage rides on Storybook in addition to unit tests.

```bash
npm run build-storybook   # verify stories build
npm run storybook         # local preview
```

## Theming

Use CSS custom properties from `src/styles/variables.css`: `--color-bg-primary`, `--color-text-primary`, `--color-accent`, `--color-border`, etc. Never hardcode colors. Test changes under both `[data-theme="light"]` and `[data-theme="dark"]`.

## Updating model pricing

The frontend bundles **no** price data. The table is fetched from this app's own backend at bootstrap (`GET /api/v1/pricing`) and installed via `setPricingTable` in `src/utils/tokenStats.ts`. The single source of truth is `backend/internal/pricingsource/pricing.json` — edit prices there. Tests and Storybook install a frozen fixture (`src/test/pricingFixture.ts`) so cost arithmetic is deterministic offline.

## Finding dead code

```bash
npm run knip
```

Categories:

- **Unused files**: dead code — delete.
- **Unused exports**: often intentional (barrel files, public API) — use judgment.
- **Unused dependencies**: verify before removing (`@types/*` may be implicit).
