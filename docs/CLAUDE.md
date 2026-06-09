# Docs site notes

User-facing documentation site at [docs.confabulous.dev](https://docs.confabulous.dev), built with [Starlight](https://starlight.astro.build/) (Astro). Source lives under `docs/src/content/docs/`; sidebar tree is in `docs/astro.config.mjs`.

## What belongs in this file

Docs-site conventions Claude would get wrong by default. Add a rule here only when it's (a) site-wide, (b) non-obvious from reading existing pages, and (c) Claude would get it wrong without the instruction.

## Sentence case for titles and headings

All page titles, sidebar labels, and `##` / `###` headings use **sentence case**: capitalize only the first word. Proper nouns (Confabulous, Claude Code, Codex, Docker, GitHub, Fly.io, Linode, Caddy, MinIO, PostgreSQL, Neon, Resend, Honeycomb, Anthropic, OpenAI) and acronyms (CLI, API, HTTP, HTTPS, OAuth, OIDC, SSO, TLS, DNS, S3, AWS) keep their canonical capitalization.

Examples:
- ✅ `title: Quickstart for end users` (not `Quickstart for End Users`)
- ✅ `title: API reference` (not `API Reference`)
- ✅ `## How sync works` (not `## How Sync Works`)
- ✅ `### Generate secrets` (not `### Generate Secrets`)
- ✅ `### GitHub OAuth` — both `GitHub` and `OAuth` stay capitalized

Apply this rule everywhere a title is rendered: frontmatter `title:`, `description:` (sentence case prose), `astro.config.mjs` `label:`, MDX `<Card title="...">`, and every Markdown heading.

## Voice

Concise, to the point, assume competence.

## Where content lives

```
docs/src/content/docs/
├── index.mdx                  # Splash landing page
├── faq.md                     # FAQ (general, managed instance, self-hosting)
├── getting-started/           # Audience-specific onboarding (end users vs admins)
├── self-hosting/              # Deployment walkthrough, samples, config reference, demo mode
├── cli/                       # confab CLI overview, commands, bundled skills
├── providers/                 # Per-provider details (claude-code, codex, opencode)
├── features/                  # Per-feature docs (sessions, trends, org analytics, sharing, smart recap)
├── api/                       # API reference (index linking into backend/API.md)
└── architecture/              # System architecture
```

When adding a page, also wire it into the sidebar tree in `docs/astro.config.mjs`.

## Source-of-truth handling

Several pages are derived from canonical docs that live at the repo root or under `backend/`. Keep them in sync by hand when the source changes:

| Docs site page | Canonical source |
|---|---|
| `self-hosting/configuration.md` | `CONFIGURATION.md` (ported in full) |
| `self-hosting/deploy.md` | `SELF-HOSTING.md` (ported in full) |
| `api/overview.md` | `backend/API.md` (links into, not duplicated) |

When changing `CONFIGURATION.md` or `SELF-HOSTING.md`, update the corresponding docs site page in the same PR. `backend/API.md` is the canonical reference for HTTP details; the docs site overview only adds structure on top.

## Shared assets

Screenshots and the architecture diagram live in `docs/public/` so both the repo root `README.md` and the docs site reference one copy. Root README uses `docs/public/<file>.png`; the docs site uses `/<file>.png` (Starlight serves `public/` at root).

## Local dev and build

```bash
cd docs
npm install      # First time only
npm run dev      # Serves at http://localhost:4321
npm run build    # Static output to dist/
npm run preview  # Sanity-check the built output
```

Config changes (`astro.config.mjs`, `src/content.config.ts`, `src/styles/custom.css`) require a dev-server restart; markdown/MDX content hot-reloads.

## Brand and styling

- Wordmark uses the **Lobster** Google Font (matches the main app's `Header.module.css`). Applied in `src/styles/custom.css` to `.site-title` and `.hero h1`.
- **The site is themed to match the main app** (`frontend/src/styles/variables.css`, the "dive watch" aesthetic) in both light and dark — accent, neutrals, surfaces, and text colors. `src/styles/custom.css` overrides Starlight's CSS custom properties to do this; **keep those values in sync with `frontend/src/styles/variables.css`**, which is the source of truth.
  - The mechanism: Starlight derives almost every surface from a grayscale ramp (`--sl-color-white` = foreground … `--sl-color-black` = page bg, `--sl-color-gray-1..7` between) plus an accent triple (`--sl-color-accent-low` / `--sl-color-accent` / `--sl-color-accent-high`). We override the ramp + accent and only the few derived surfaces whose default doesn't match the app (`--sl-color-bg-nav`, `--sl-color-bg-sidebar`, `--sl-color-bg-inline-code`, dark-mode `--sl-color-text-accent`). `--sl-color-text`, hairlines, etc. resolve from the ramp automatically — don't pin them.
  - Stay variable-only: prefer overriding `--sl-color-*` over per-component CSS so Starlight upgrades don't break the theme. Fenced code blocks (Expressive Code) inherit the gray ramp.
  - Mirror Starlight's own structure: dark values under `:root`, light under `:root[data-theme='light']`.
  - `custom.css` also makes a few chrome elements app-consistent: the wordmark and header social icon use neutral text colors (not Starlight's accent default), and the splash hero CTA is overridden from Starlight's full pill to the app's compact filled-accent button (radius + padding + a shrunk arrow icon). `custom.css` is a plain global stylesheet — do **not** use `:global()` there (it's a CSS-modules/scoped-style construct and is silently dropped as an invalid selector); use plain descendant selectors.
- **Provider logo strip:** `src/components/ProviderLogos.astro` renders the "Works with" row on the splash page (`index.mdx`). It inlines the same official marks the main app uses (`frontend/src/components/icons.tsx` — Claude clay `#d97757`, Codex teal `#10a37f`, OpenCode monochrome via `currentColor`); keep them in sync. SVGs are inlined (not `<img>`) so OpenCode's `currentColor` adapts per theme.
- First-visit theme defaults to **dark** to match the main app, via a pre-paint script at the top of the `head` array in `astro.config.mjs` that seeds `localStorage['starlight-theme']` with `'dark'` when unset. Explicit choices from Starlight's theme switcher are preserved.

## Deployment

Hosted on Cloudflare Pages, built from `docs/` on push to `main`. Custom domain: `docs.confabulous.dev`. Search is Pagefind (built into Starlight, static, zero runtime cost).
