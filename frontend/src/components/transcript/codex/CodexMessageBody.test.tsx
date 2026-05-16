// Spec for CodexMessageBody's CF-359 highlight wiring. The body is the
// shared rendering path for Codex user + assistant messages, so locking
// the highlight contract here covers both renderer kinds.

import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import CodexMessageBody from './CodexMessageBody';
import { getHighlightClass } from '@/utils/highlightSearch';

describe('CodexMessageBody — search highlight', () => {
  it('wraps matches in <mark> when searchQuery is set (markdown path)', () => {
    const { container } = render(
      <CodexMessageBody text="hello world" searchQuery="hello" />,
    );
    const mark = container.querySelector('mark');
    expect(mark).not.toBeNull();
    expect(mark?.textContent).toBe('hello');
  });

  it('uses the active-match class when isCurrentSearchMatch is true', () => {
    const { container } = render(
      <CodexMessageBody
        text="hello world"
        searchQuery="hello"
        isCurrentSearchMatch
      />,
    );
    const mark = container.querySelector('mark');
    expect(mark?.className).toBe(getHighlightClass(true));
  });

  it('uses the non-active class when isCurrentSearchMatch is false', () => {
    const { container } = render(
      <CodexMessageBody
        text="hello world"
        searchQuery="hello"
        isCurrentSearchMatch={false}
      />,
    );
    const mark = container.querySelector('mark');
    expect(mark?.className).toBe(getHighlightClass(false));
  });

  it('does not wrap anything in <mark> when searchQuery is undefined', () => {
    const { container } = render(<CodexMessageBody text="hello world" />);
    expect(container.querySelector('mark')).toBeNull();
  });

  it('does not wrap anything in <mark> when searchQuery is empty/whitespace', () => {
    const { container } = render(
      <CodexMessageBody text="hello world" searchQuery="   " />,
    );
    expect(container.querySelector('mark')).toBeNull();
  });

  it('case-insensitive match', () => {
    const { container } = render(
      <CodexMessageBody text="Hello World" searchQuery="hello" />,
    );
    expect(container.querySelector('mark')?.textContent).toBe('Hello');
  });

  it('JSON-shaped text routes through CodeBlock with searchQuery forwarded', () => {
    const { container } = render(
      <CodexMessageBody text='{"name":"alpha","count":1}' searchQuery="alpha" />,
    );
    // JSON content lands in a Prism-styled code block; the search highlight
    // is wrapped in a <mark> just like the markdown branch.
    const mark = container.querySelector('mark');
    expect(mark).not.toBeNull();
    expect(mark?.textContent).toBe('alpha');
  });
});
