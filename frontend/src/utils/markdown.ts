// Shared markdown -> sanitized-HTML renderers.
// renderMarkdownToHtml: transcript content blocks and attachment components (CF-346).
// renderLegalMarkdown: the in-repo legal documents (frontend/src/legal/).
import { marked, Marked } from 'marked';
import DOMPurify from 'dompurify';
import { stripAnsi } from './utils';

/**
 * If `text` is a JSON object or array (trimmed start of `{` or `[`), return a
 * pretty-printed JSON string. Otherwise return null and let the caller fall
 * back to plain markdown rendering. Primitives (`"foo"`, `42`) return null
 * because the pretty-print only adds value for nested structures.
 *
 * Shared between `ContentBlock` (Claude) and the Codex message renderers so
 * the JSON-fallback contract is identical across providers.
 */
export function tryParseAsJson(text: string): string | null {
  const trimmed = text.trim();
  if (!trimmed.startsWith('{') && !trimmed.startsWith('[')) {
    return null;
  }
  try {
    const parsed = JSON.parse(trimmed);
    if (typeof parsed === 'object' && parsed !== null) {
      return JSON.stringify(parsed, null, 2);
    }
    return null;
  } catch {
    return null;
  }
}

// Configure marked once for the app. Subsequent .use() calls would merge so we
// keep this idempotent by gating on a module-scoped flag.
let configured = false;
function configure() {
  if (configured) return;
  marked.use({
    async: false,
    gfm: true,
    breaks: true,
  });
  configured = true;
}

/**
 * Render markdown text to sanitized HTML. Strips ANSI escapes first, then runs
 * GFM markdown parsing, then DOMPurify with target="_blank" attributes allowed.
 *
 * Pure HTML string output — caller is responsible for setting it via
 * dangerouslySetInnerHTML or feeding it into highlightTextInHtml first.
 */
export function renderMarkdownToHtml(text: string): string {
  configure();
  const cleaned = stripAnsi(text);
  const html = marked.parse(cleaned);
  // marked.parse returns a string synchronously when async: false is configured.
  if (typeof html !== 'string') return '';
  return DOMPurify.sanitize(html, {
    ADD_ATTR: ['target'],
  });
}

const HTML_ENTITIES: Record<string, string> = {
  '&amp;': '&',
  '&lt;': '<',
  '&gt;': '>',
  '&quot;': '"',
  '&#39;': "'",
};

/** Heading text -> URL fragment: lowercase, punctuation dropped, spaces to hyphens. */
function slugify(html: string): string {
  const text = html
    .replace(/<[^>]*>/g, '')
    .replace(/&(?:amp|lt|gt|quot|#39);/g, (e) => HTML_ENTITIES[e] ?? e);
  const slug = text
    .toLowerCase()
    .replace(/[^\p{L}\p{N}\s-]/gu, '')
    .trim()
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-');
  return slug || 'section';
}

// Separate instance so the global `marked` config above (breaks: true for
// transcripts) never leaks into legal rendering, and vice versa.
const usedHeadingIds = new Set<string>();
const legalMarked = new Marked({
  async: false,
  gfm: true,
  renderer: {
    heading({ tokens, depth }) {
      const inner = this.parser.parseInline(tokens);
      const base = slugify(inner);
      let id = base;
      for (let n = 2; usedHeadingIds.has(id); n++) id = `${base}-${n}`;
      usedHeadingIds.add(id);
      return `<h${depth} id="${id}">${inner}</h${depth}>\n`;
    },
  },
});

/**
 * Render a legal document (trusted, bundled Markdown) to sanitized HTML.
 * GFM without `breaks`, and every heading gets a slug `id` (duplicates get
 * `-2`, `-3`, …) so the table of contents and deep links work.
 */
export function renderLegalMarkdown(md: string): string {
  usedHeadingIds.clear();
  const html = legalMarked.parse(md);
  if (typeof html !== 'string') return '';
  return DOMPurify.sanitize(html);
}
