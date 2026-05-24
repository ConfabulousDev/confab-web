# Confabulous docs site

The documentation site published at [docs.confabulous.dev](https://docs.confabulous.dev), built with [Starlight](https://starlight.astro.build/) (Astro).

## Local development

```bash
cd docs
npm install
npm run dev
```

Then open the URL Astro prints (default: `http://localhost:4321`).

## Building

```bash
npm run build      # Static output to dist/
npm run preview    # Serve the built output for sanity-check
```

## Content layout

All content is markdown/MDX under `src/content/docs/`. The sidebar tree is configured in `astro.config.mjs`.

```
src/content/docs/
├── index.mdx                  # Landing page (splash template)
├── getting-started/
├── self-hosting/
├── cli/
├── providers/
├── features/
├── api/
└── architecture/
```

For authoring conventions (sentence case, source-of-truth pages, IA), see [`docs/CLAUDE.md`](./CLAUDE.md).

## Editing a page

Every page has an "Edit on GitHub" link in the footer that points at its source file in this repo. Editing on GitHub directly is the fastest way to fix typos or small additions.

For larger changes, edit locally and run `npm run build` to catch broken links and frontmatter errors before pushing.

## Shared assets

The repo's main `README.md` references screenshots and the architecture diagram from `docs/public/`. That directory is also Starlight's static-assets root (served at `/` on the docs site), so both consumers share one source of truth.

## Deployment (Cloudflare Pages via GitHub Actions)

The build runs in GitHub Actions (`.github/workflows/docs-deploy.yml`) and uploads the static output to Cloudflare Pages via the Wrangler CLI authenticated with a scoped Cloudflare API token. **Cloudflare has no access to GitHub** — only GitHub holds a token that can deploy to Cloudflare Pages.

The workflow triggers on:
- Pushes to `main` that touch `docs/**` or the workflow file itself.
- Manual `workflow_dispatch` runs (the **Run workflow** button on the Actions page).

### One-time setup

**1. Create the Pages project on Cloudflare (no GitHub link).**

- **Workers & Pages → Create → Pages → Upload assets**.
- Project name: `confab-docs` (matches `--project-name` in the workflow).
- On the upload page, you can ignore the file uploader — the GitHub Action will populate it on the first push. Click **Deploy site** with an empty zip, or just close the wizard once the project exists.

**2. Get your Cloudflare Account ID.**

- **Workers & Pages → Overview** → right sidebar shows **Account ID**. Copy it.

**3. Create a scoped Cloudflare API token.**

- **My Profile → API Tokens → Create Token → Create Custom Token**.
- Name: `docs-deploy` (or similar).
- **Permissions:** `Account` → `Cloudflare Pages` → `Edit`.
- **Account Resources:** `Include` → your specific account (not "All accounts").
- (Optional) **Client IP Address Filtering:** none, since GitHub Actions runners have rotating IPs.
- (Optional) **TTL:** set an expiration if you want to force rotation.
- Click **Continue to summary → Create Token**. **Copy the token now** — Cloudflare won't show it again.

**4. Add three secrets to the GitHub repo.**

- GitHub: **Settings → Secrets and variables → Actions → New repository secret**, add:
  - `CLOUDFLARE_API_TOKEN` — the token from step 3.
  - `CLOUDFLARE_ACCOUNT_ID` — the account ID from step 2.
  - `CF_ANALYTICS_TOKEN` — leave blank for now; fill in after step 6.

**5. Trigger the first deploy.**

- Either push any change to `docs/`, or go to **Actions → Deploy docs → Run workflow → Run** (uses `workflow_dispatch`).
- Watch the run complete. The final step (`pages deploy …`) prints the production URL (`*.pages.dev`).

**6. Add the custom domain.**

- **Cloudflare → Workers & Pages → confab-docs → Custom domains → Set up a custom domain** → `docs.confabulous.dev`.
- Since `confabulous.dev` already lives on Cloudflare DNS, the CNAME and TLS cert are wired automatically.

**7. Add Cloudflare Web Analytics.**

- **Cloudflare → Analytics & Logs → Web Analytics → Add a site** → `docs.confabulous.dev`.
- Copy the token (the value inside `data-cf-beacon='{"token":"..."}'`).
- Update the `CF_ANALYTICS_TOKEN` GitHub secret with this value.
- Trigger a redeploy (push or **Run workflow**) so the beacon ships with the build.

### Subsequent deploys

Push changes under `docs/` to `main` → the workflow runs → the new build replaces the live site within ~2 minutes. View runs at **GitHub → Actions → Deploy docs**.

To re-deploy without a code change: **Actions → Deploy docs → Run workflow**.

### Rolling back

- **Cloudflare dashboard** → Workers & Pages → confab-docs → **Deployments** → open any past deployment → **Rollback to this deployment**. No git changes needed.
- Or **revert the commit on `main`** and let the workflow re-deploy the previous content.

### Why this setup

The workflow uses an API token scoped to **only Cloudflare Pages on your account** — no zone access, no user-level permissions, no GitHub access on Cloudflare's side. The token can be rotated or revoked from the Cloudflare dashboard at any time without touching GitHub.

## Search

Powered by [Pagefind](https://pagefind.app/) (built into Starlight). The index is generated at build time, runs entirely client-side, and adds no runtime dependency.

## SEO and discoverability

- **`sitemap-index.xml`** — generated automatically by Starlight on every build.
- **`robots.txt`** — at `public/robots.txt`, permissive (allows all crawlers, references the sitemap).
- **OpenGraph / Twitter cards** — site-wide defaults in `astro.config.mjs`. The default OG image is `public/og-image.svg` (Lobster wordmark + tagline on a dark gradient). For best compatibility with Facebook/LinkedIn previews, rasterize it to `og-image.png` (1200×630) and update the `OG_IMAGE` const in `astro.config.mjs`. Per-page frontmatter can override the default.

## Analytics

Cloudflare Web Analytics, gated on the `CF_ANALYTICS_TOKEN` build-time env var. Privacy-friendly, no cookies, no per-user tracking. Set the env var in the Cloudflare Pages project settings; unset it (or use a different value per branch) to disable.
