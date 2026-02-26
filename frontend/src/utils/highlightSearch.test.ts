import { describe, it, expect } from 'vitest';
import { getHighlightClass, escapeRegExp, highlightTextInHtml, splitTextByQuery } from './highlightSearch';

describe('getHighlightClass', () => {
  it('returns active class when true', () => {
    expect(getHighlightClass(true)).toBe('search-highlight-active');
  });

  it('returns default class when false', () => {
    expect(getHighlightClass(false)).toBe('search-highlight');
  });
});

describe('escapeRegExp', () => {
  it('escapes special regex characters', () => {
    expect(escapeRegExp('foo.bar')).toBe('foo\\.bar');
    expect(escapeRegExp('a+b*c?')).toBe('a\\+b\\*c\\?');
    expect(escapeRegExp('(test)')).toBe('\\(test\\)');
    expect(escapeRegExp('[a]')).toBe('\\[a\\]');
    expect(escapeRegExp('a{1}')).toBe('a\\{1\\}');
    expect(escapeRegExp('$100')).toBe('\\$100');
    expect(escapeRegExp('^start')).toBe('\\^start');
    expect(escapeRegExp('a|b')).toBe('a\\|b');
    expect(escapeRegExp('back\\slash')).toBe('back\\\\slash');
  });

  it('returns plain strings unchanged', () => {
    expect(escapeRegExp('hello')).toBe('hello');
    expect(escapeRegExp('')).toBe('');
  });
});

describe('highlightTextInHtml', () => {
  it('wraps matches in mark tags', () => {
    const html = 'hello world';
    expect(highlightTextInHtml(html, 'world')).toBe(
      'hello <mark class="search-highlight">world</mark>',
    );
  });

  it('is case insensitive', () => {
    expect(highlightTextInHtml('Hello World', 'hello')).toBe(
      '<mark class="search-highlight">Hello</mark> World',
    );
  });

  it('highlights multiple occurrences', () => {
    expect(highlightTextInHtml('foo bar foo', 'foo')).toBe(
      '<mark class="search-highlight">foo</mark> bar <mark class="search-highlight">foo</mark>',
    );
  });

  it('does not match inside HTML tags', () => {
    const html = '<span class="test">test content</span>';
    expect(highlightTextInHtml(html, 'test')).toBe(
      '<span class="test"><mark class="search-highlight">test</mark> content</span>',
    );
  });

  it('does not match inside HTML attributes', () => {
    const html = '<a href="https://example.com">click here</a>';
    expect(highlightTextInHtml(html, 'example')).toBe(
      '<a href="https://example.com">click here</a>',
    );
  });

  it('handles nested HTML elements', () => {
    const html = '<p>hello <strong>world</strong> hello</p>';
    expect(highlightTextInHtml(html, 'hello')).toBe(
      '<p><mark class="search-highlight">hello</mark> <strong>world</strong> <mark class="search-highlight">hello</mark></p>',
    );
  });

  it('uses custom CSS class', () => {
    expect(highlightTextInHtml('test', 'test', 'active')).toBe(
      '<mark class="active">test</mark>',
    );
  });

  it('returns unchanged HTML for empty query', () => {
    const html = '<p>hello</p>';
    expect(highlightTextInHtml(html, '')).toBe(html);
    expect(highlightTextInHtml(html, '  ')).toBe(html);
  });

  it('handles special regex characters in query', () => {
    const html = 'price is $100.00';
    expect(highlightTextInHtml(html, '$100.00')).toBe(
      'price is <mark class="search-highlight">$100.00</mark>',
    );
  });

  it('handles self-closing tags', () => {
    const html = 'before<br/>after';
    expect(highlightTextInHtml(html, 'before')).toBe(
      '<mark class="search-highlight">before</mark><br/>after',
    );
  });
});

describe('splitTextByQuery', () => {
  it('splits text around matches', () => {
    const result = splitTextByQuery('hello world', 'world');
    expect(result).toEqual(['hello ', { match: 'world' }]);
  });

  it('is case insensitive', () => {
    const result = splitTextByQuery('Hello World', 'hello');
    expect(result).toEqual([{ match: 'Hello' }, ' World']);
  });

  it('handles multiple matches', () => {
    const result = splitTextByQuery('foo bar foo baz foo', 'foo');
    expect(result).toEqual([
      { match: 'foo' },
      ' bar ',
      { match: 'foo' },
      ' baz ',
      { match: 'foo' },
    ]);
  });

  it('returns original text for empty query', () => {
    expect(splitTextByQuery('hello', '')).toEqual(['hello']);
    expect(splitTextByQuery('hello', '  ')).toEqual(['hello']);
  });

  it('returns original text when no match', () => {
    expect(splitTextByQuery('hello', 'xyz')).toEqual(['hello']);
  });

  it('handles special regex characters in query', () => {
    const result = splitTextByQuery('cost: $10.00 total', '$10.00');
    expect(result).toEqual(['cost: ', { match: '$10.00' }, ' total']);
  });
});
