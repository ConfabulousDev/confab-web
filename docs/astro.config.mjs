// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

const SITE = 'https://docs.confabulous.dev';
// SVG by default; rasterize to /og-image.png for maximum social-preview
// compatibility (Facebook/LinkedIn are picky about SVG).
const OG_IMAGE = `${SITE}/og-image.svg`;

/** @type {import('@astrojs/starlight/types').StarlightUserConfig['head']} */
const head = [
  // Default first-visit theme to dark. Must stay first in `head` so it runs before
  // Starlight's ThemeProvider reads localStorage. Explicit user choices are preserved.
  {
    tag: 'script',
    content: `try { if (localStorage.getItem('starlight-theme') === null) localStorage.setItem('starlight-theme', 'dark'); } catch {}`,
  },

  // Preconnect to Google Fonts for the Lobster wordmark.
  { tag: 'link', attrs: { rel: 'preconnect', href: 'https://fonts.googleapis.com' } },
  { tag: 'link', attrs: { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: true } },

  // Default Open Graph / Twitter card. Per-page frontmatter can override.
  { tag: 'meta', attrs: { property: 'og:image', content: OG_IMAGE } },
  { tag: 'meta', attrs: { property: 'og:image:width', content: '1200' } },
  { tag: 'meta', attrs: { property: 'og:image:height', content: '630' } },
  { tag: 'meta', attrs: { property: 'og:image:alt', content: 'Confabulous documentation' } },
  { tag: 'meta', attrs: { name: 'twitter:card', content: 'summary_large_image' } },
  { tag: 'meta', attrs: { name: 'twitter:image', content: OG_IMAGE } },
];

// Cloudflare Web Analytics — opt-in via CF_ANALYTICS_TOKEN env var at build time.
const cfAnalyticsToken = process.env.CF_ANALYTICS_TOKEN;
if (cfAnalyticsToken) {
  head.push({
    tag: 'script',
    attrs: {
      defer: true,
      src: 'https://static.cloudflareinsights.com/beacon.min.js',
      'data-cf-beacon': JSON.stringify({ token: cfAnalyticsToken }),
    },
  });
}

export default defineConfig({
  site: SITE,
  integrations: [
    starlight({
      title: 'Confabulous',
      description: 'Open-source analytics for your Claude Code and Codex sessions — managed or self-hosted.',
      customCss: ['./src/styles/custom.css'],
      head,
      social: [
        {
          icon: 'github',
          label: 'GitHub',
          href: 'https://github.com/ConfabulousDev/confab-web',
        },
      ],
      editLink: {
        baseUrl: 'https://github.com/ConfabulousDev/confab-web/edit/main/docs/',
      },
      sidebar: [
        {
          label: 'Getting started',
          items: [
            { label: 'Introduction', slug: 'getting-started/introduction' },
            { label: 'Quickstart for end users', slug: 'getting-started/end-user-quickstart' },
            { label: 'Quickstart for admins', slug: 'getting-started/admin-quickstart' },
            { label: 'First sync', slug: 'getting-started/first-sync' },
            { label: 'Concepts', slug: 'getting-started/concepts' },
          ],
        },
        {
          label: 'Self-hosting',
          items: [
            { label: 'Deployment walkthrough', slug: 'self-hosting/deploy' },
            { label: 'Sample deployments', slug: 'self-hosting/examples' },
            { label: 'Configuration reference', slug: 'self-hosting/configuration' },
            { label: 'Demo mode', slug: 'self-hosting/demo-mode' },
          ],
        },
        {
          label: 'CLI',
          items: [
            { label: 'Overview', slug: 'cli/overview' },
            { label: 'Commands', slug: 'cli/commands' },
            { label: 'Skills', slug: 'cli/skills' },
          ],
        },
        {
          label: 'Providers',
          items: [
            { label: 'Claude Code', slug: 'providers/claude-code' },
            { label: 'Codex', slug: 'providers/codex' },
            { label: 'OpenCode', slug: 'providers/opencode' },
          ],
        },
        {
          label: 'Features',
          items: [
            { label: 'Sessions', slug: 'features/sessions' },
            { label: 'Per-session analytics', slug: 'features/analytics' },
            { label: 'PR linking', slug: 'features/pr-linking' },
            { label: 'TILs', slug: 'features/tils' },
            { label: 'Trends', slug: 'features/trends' },
            { label: 'Organization analytics', slug: 'features/organization-analytics' },
            { label: 'Sharing', slug: 'features/sharing' },
            { label: 'Smart recap', slug: 'features/smart-recap' },
          ],
        },
        {
          label: 'API reference',
          items: [{ label: 'Overview', slug: 'api/overview' }],
        },
        {
          label: 'Architecture',
          items: [{ label: 'Overview', slug: 'architecture/overview' }],
        },
        { label: 'FAQ', slug: 'faq' },
      ],
    }),
  ],
});
