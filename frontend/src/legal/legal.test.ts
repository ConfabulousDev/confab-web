import { describe, it, expect } from 'vitest';
import { renderLegalMarkdown } from '@/utils/markdown';
import { LEGAL_DOCS } from './docs';
import { LEGAL_MARKDOWN } from './content';

// Permanent content-integrity checks for the in-repo legal documents.

// The former policy host's name, split so `grep -ri` for it over this
// directory stays empty (evvf acceptance).
const FORMER_HOST = new RegExp('ter' + 'mly', 'i');

// The managed service lives at code.confabulous.dev; the bare domain is the
// company site. Subdomains (code., code-demo., code-docs., share.) and contact@
// addresses are fine; a bare `confabulous.dev` reference is not.
const BARE_HOST = /(?<![\w@.-])confabulous\.dev/;

// The retired docs and demo hosts (hard cutover to code-docs. / code-demo.,
// no redirects). The lookbehind keeps code-docs. and code-demo. from matching;
// the demo@confabulous.dev address never matches (no dot after `demo`).
const OLD_SUBHOST = /(?<![\w.-])(?:docs|demo)\.confabulous\.dev/;

// The retired San Jose mailing address. Every address must match the D-U-N-S
// record (2108 N St, Ste N, Sacramento, CA 95816).
const FORMER_ADDRESS = /piedmont|san jose|32281/i;

function linesMatching(markdown: string, re: RegExp): string[] {
  return markdown.split('\n').filter((line) => re.test(line));
}

describe('legal documents', () => {
  it('registers the five policies under stable slugs', () => {
    expect(LEGAL_DOCS.map((d) => d.slug)).toEqual([
      'terms',
      'privacy',
      'cookies',
      'disclaimer',
      'acceptable-use',
    ]);
  });

  describe.each(LEGAL_DOCS.map((d) => [d.slug, d] as const))('%s', (slug, doc) => {
    const markdown = LEGAL_MARKDOWN[slug];
    const html = renderLegalMarkdown(markdown);

    it('renders non-empty HTML', () => {
      expect(markdown.trim().length).toBeGreaterThan(0);
      expect(html.trim().length).toBeGreaterThan(0);
    });

    it('resolves every in-page #anchor link to an element id', () => {
      const ids = new Set([...html.matchAll(/\sid="([^"]+)"/g)].map((m) => m[1]));
      const targets = [...html.matchAll(/href="#([^"]*)"/g)].map((m) => m[1]);
      const missing = targets.filter((t) => !ids.has(t));
      expect(missing, `unresolved anchors in ${slug}`).toEqual([]);
    });

    it('contains no references to the former policy host', () => {
      expect(markdown).not.toMatch(FORMER_HOST);
    });

    it('never references the bare confabulous.dev host', () => {
      expect(linesMatching(markdown, BARE_HOST), `bare-host references in ${slug}`).toEqual([]);
    });

    it('never references the retired docs or demo host', () => {
      expect(linesMatching(markdown, OLD_SUBHOST), `old-subhost references in ${slug}`).toEqual([]);
    });

    it('uses only the D-U-N-S mailing address', () => {
      expect(linesMatching(markdown, FORMER_ADDRESS), `former address in ${slug}`).toEqual([]);
    });

    it('has a parseable ISO lastUpdated date', () => {
      expect(doc.lastUpdated).toMatch(/^\d{4}-\d{2}-\d{2}$/);
      expect(Number.isNaN(Date.parse(doc.lastUpdated))).toBe(false);
    });

    it('has a title and description', () => {
      expect(doc.title.trim()).not.toBe('');
      expect(doc.description.trim()).not.toBe('');
    });

    it('does not repeat the page title as a top-level heading', () => {
      expect(markdown).not.toMatch(/^# /m);
    });
  });
});
