// Markdown source of each legal document, bundled at build time. Import this
// only from lazily loaded code (LegalDocPage) so the policy text stays out of
// the main bundle.
import type { LegalSlug } from './docs';
import terms from './terms.md?raw';
import privacy from './privacy.md?raw';
import cookies from './cookies.md?raw';
import disclaimer from './disclaimer.md?raw';
import acceptableUse from './acceptable-use.md?raw';

export const LEGAL_MARKDOWN: Record<LegalSlug, string> = {
  terms,
  privacy,
  cookies,
  disclaimer,
  'acceptable-use': acceptableUse,
};
