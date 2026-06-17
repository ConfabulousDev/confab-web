// Spec for CursorMessageBody (pt81). The body is the shared rendering path for
// Cursor user-prompt and assistant narrative rows: cleaned text (fa3h strips
// `[REDACTED]`; nfbe extracts `<user_query>`) flows through the GFM markdown
// pipeline so bold / tables / links render as HTML, while the Cmd-F highlight
// contract matches the Codex body (mark inside rendered HTML).

import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import CursorMessageBody from './CursorMessageBody';
import { getHighlightClass } from '@/utils/highlightSearch';

describe('CursorMessageBody — markdown rendering', () => {
  it('renders bold markdown as <strong>', () => {
    const { container } = render(
      <CursorMessageBody text="**ConfabulousDev/confab-web** has 14 alerts" />,
    );
    const strong = container.querySelector('strong');
    expect(strong).not.toBeNull();
    expect(strong?.textContent).toBe('ConfabulousDev/confab-web');
  });

  it('renders a GFM pipe table as a real <table>', () => {
    const md = '| Severity | Count |\n|----------|-------|\n| High | 8 |';
    const { container } = render(<CursorMessageBody text={md} />);
    const table = container.querySelector('table');
    expect(table).not.toBeNull();
    expect(table?.querySelectorAll('td').length).toBeGreaterThan(0);
  });

  it('renders a markdown link as an <a> with its href', () => {
    const { container } = render(
      <CursorMessageBody text="Full list: [github.com](https://github.com/x/y)" />,
    );
    const anchor = container.querySelector('a');
    expect(anchor).not.toBeNull();
    expect(anchor?.getAttribute('href')).toBe('https://github.com/x/y');
  });

  it('renders plain (non-markdown) text', () => {
    const { container } = render(<CursorMessageBody text="just plain prose" />);
    expect(container.textContent).toContain('just plain prose');
  });
});

describe('CursorMessageBody — search highlight', () => {
  it('wraps matches in <mark> when searchQuery is set (markdown path)', () => {
    const { container } = render(
      <CursorMessageBody text="hello world" searchQuery="hello" />,
    );
    const mark = container.querySelector('mark');
    expect(mark).not.toBeNull();
    expect(mark?.textContent).toBe('hello');
  });

  it('uses the active-match class when isCurrentSearchMatch is true', () => {
    const { container } = render(
      <CursorMessageBody text="hello world" searchQuery="hello" isCurrentSearchMatch />,
    );
    expect(container.querySelector('mark')?.className).toBe(getHighlightClass(true));
  });

  it('uses the non-active class when isCurrentSearchMatch is false', () => {
    const { container } = render(
      <CursorMessageBody text="hello world" searchQuery="hello" isCurrentSearchMatch={false} />,
    );
    expect(container.querySelector('mark')?.className).toBe(getHighlightClass(false));
  });

  it('does not wrap anything in <mark> when searchQuery is undefined', () => {
    const { container } = render(<CursorMessageBody text="hello world" />);
    expect(container.querySelector('mark')).toBeNull();
  });

  it('does not wrap anything in <mark> when searchQuery is empty/whitespace', () => {
    const { container } = render(
      <CursorMessageBody text="hello world" searchQuery="   " />,
    );
    expect(container.querySelector('mark')).toBeNull();
  });

  it('case-insensitive match', () => {
    const { container } = render(
      <CursorMessageBody text="Hello World" searchQuery="hello" />,
    );
    expect(container.querySelector('mark')?.textContent).toBe('Hello');
  });
});
