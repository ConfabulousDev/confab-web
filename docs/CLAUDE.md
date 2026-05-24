# Docs site notes

User-facing documentation site at [docs.confabulous.dev](https://docs.confabulous.dev), built with [Starlight](https://starlight.astro.build/) (Astro). Source lives under `docs/src/content/docs/`; sidebar tree is in `docs/astro.config.mjs`.

## What belongs in this file

Docs-site conventions Claude would get wrong by default. Add a rule here only when it's (a) site-wide, (b) non-obvious from reading existing pages, and (c) Claude would get it wrong without the instruction.

## Sentence case for titles and headings

All page titles, sidebar labels, and `##` / `###` headings use **sentence case**: capitalize only the first word. Proper nouns (Confabulous, Claude Code, Codex, Docker, GitHub, Fly.io, Linode, Caddy, MinIO, PostgreSQL, Neon, Resend, Honeycomb, Anthropic, OpenAI) and acronyms (CLI, API, HTTP, HTTPS, OAuth, OIDC, SSO, TIL, TILs, TLS, DNS, S3, AWS) keep their canonical capitalization.

Examples:
- ✅ `title: Quickstart for end users` (not `Quickstart for End Users`)
- ✅ `title: API reference` (not `API Reference`)
- ✅ `## How sync works` (not `## How Sync Works`)
- ✅ `### Generate secrets` (not `### Generate Secrets`)
- ✅ `### GitHub OAuth` — both `GitHub` and `OAuth` stay capitalized

Apply this rule everywhere a title is rendered: frontmatter `title:`, `description:` (sentence case prose), `astro.config.mjs` `label:`, MDX `<Card title="...">`, and every Markdown heading.

## Where content lives

```
docs/src/content/docs/
├── index.mdx                  # Splash landing page
├── getting-started/           # Audience-specific onboarding (end users vs admins)
├── self-hosting/              # Deployment walkthrough, samples, config reference, demo mode
├── cli/                       # confab CLI overview, commands, bundled skills
├── providers/                 # Per-provider details (claude-code, codex)
├── features/                  # Per-feature docs (sessions, TILs, trends, org analytics, sharing, smart recap)
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
- Site default colors are Starlight's stock palette — intentionally bland. Don't introduce custom brand colors without explicit direction.

## Deployment

Hosted on Cloudflare Pages, built from `docs/` on push to `main`. Custom domain: `docs.confabulous.dev`. Search is Pagefind (built into Starlight, static, zero runtime cost).
