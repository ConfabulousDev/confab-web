// Registry of the in-repo legal documents. Metadata only: the router and
// PoliciesPage import this, so it must stay small. The Markdown itself lives
// in ./content.ts, which only the lazy LegalDocPage chunk imports.

interface LegalDocMeta {
  /** Top-level route (`/<slug>`). `terms` and `privacy` must never change. */
  slug: string;
  title: string;
  description: string;
  /** ISO date (YYYY-MM-DD) shown as "Last updated". Bump on every edit. */
  lastUpdated: string;
}

export const LEGAL_DOCS = [
  {
    slug: 'terms',
    title: 'Terms of Service',
    description: 'Terms and conditions for using Confabulous.dev',
    lastUpdated: '2026-01-09',
  },
  {
    slug: 'privacy',
    title: 'Privacy Notice',
    description: 'How we collect, use, and protect your data',
    lastUpdated: '2026-01-09',
  },
  {
    slug: 'cookies',
    title: 'Cookie Policy',
    description: 'How we use cookies and similar technologies',
    lastUpdated: '2026-10-29',
  },
  {
    slug: 'disclaimer',
    title: 'Disclaimer',
    description: 'Important notices and limitations of liability',
    lastUpdated: '2026-01-09',
  },
  {
    slug: 'acceptable-use',
    title: 'Acceptable Use Policy',
    description: 'Guidelines for appropriate use of our services',
    lastUpdated: '2026-01-09',
  },
] as const satisfies readonly LegalDocMeta[];

export type LegalSlug = (typeof LEGAL_DOCS)[number]['slug'];
