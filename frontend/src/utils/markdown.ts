// Shared markdown -> sanitized-HTML renderer.
// Used by transcript content blocks and attachment components (CF-346).
import { marked } from 'marked';
import DOMPurify from 'dompurify';
import { stripAnsi } from './utils';

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
