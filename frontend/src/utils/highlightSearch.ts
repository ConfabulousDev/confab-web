/**
 * Utilities for highlighting search matches in rendered HTML and plain text.
 */

/** CSS class names for search highlight marks */
const HIGHLIGHT_CLASS = 'search-highlight';
const HIGHLIGHT_CLASS_ACTIVE = 'search-highlight-active';

/** Return the CSS class for a search highlight mark element */
export function getHighlightClass(isActiveMatch: boolean): string {
  return isActiveMatch ? HIGHLIGHT_CLASS_ACTIVE : HIGHLIGHT_CLASS;
}

/** Escape a plain-text string so it is safe to embed in HTML */
export function escapeHtml(text: string): string {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}

/** Escape special regex characters in a string */
export function escapeRegExp(str: string): string {
  return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

/**
 * Highlight search query matches within an HTML string.
 * Only replaces within text nodes — never inside HTML tags or attributes.
 *
 * @param html - The HTML string to process
 * @param query - The search query (case-insensitive)
 * @param activeClass - CSS class for the mark element (controls highlight color)
 * @returns Modified HTML with matches wrapped in <mark> elements
 */
export function highlightTextInHtml(
  html: string,
  query: string,
  activeClass: string = 'search-highlight',
): string {
  if (!query || !query.trim()) return html;

  const escaped = escapeRegExp(query);
  const regex = new RegExp(`(${escaped})`, 'gi');

  // Split HTML into tag segments and text segments.
  // The capture group keeps the tags in the result array.
  const parts = html.split(/(<[^>]*>)/);

  return parts
    .map((part) => {
      // HTML tags — leave untouched
      if (part.startsWith('<') && part.endsWith('>')) {
        return part;
      }
      // Text node — wrap matches in <mark>
      return part.replace(regex, `<mark class="${activeClass}">$1</mark>`);
    })
    .join('');
}

/**
 * Split plain text on a search query and return React-compatible segments.
 * Each segment is either a plain string or a { match: string } object.
 *
 * @param text - The plain text to search
 * @param query - The search query (case-insensitive)
 * @returns Array of segments: strings for non-matching parts, { match } for matches
 */
export function splitTextByQuery(
  text: string,
  query: string,
): Array<string | { match: string }> {
  if (!query || !query.trim()) return [text];

  const escaped = escapeRegExp(query);
  const regex = new RegExp(`(${escaped})`, 'gi');
  const parts = text.split(regex);
  const needle = query.toLowerCase();

  return parts
    .filter((part) => part !== '')
    .map((part) => (part.toLowerCase() === needle ? { match: part } : part));
}
