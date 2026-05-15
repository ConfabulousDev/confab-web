// Locks the contract that Codex user prompts render through the same
// markdown / JSON pretty-print pipeline as Claude's ContentBlock.

import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import CodexUserMessage from './CodexUserMessage';
import type { CodexUserItem } from '@/types/codexRenderItem';

function user(text: string): CodexUserItem {
  return { kind: 'user', lineId: '0', timestamp: '2026-05-13T01:00:00Z', text };
}

describe('CodexUserMessage', () => {
  it('renders plain text inside a paragraph', () => {
    const { container } = render(<CodexUserMessage item={user('hello world')} />);
    const p = container.querySelector('p');
    expect(p).not.toBeNull();
    expect(p?.textContent).toContain('hello world');
  });

  it('renders markdown bold as a <strong> element', () => {
    const { container } = render(<CodexUserMessage item={user('this is **bold**')} />);
    const strong = container.querySelector('strong');
    expect(strong).not.toBeNull();
    expect(strong?.textContent).toBe('bold');
  });

  it('renders JSON-shaped text as syntax-highlighted JSON', () => {
    const { container } = render(<CodexUserMessage item={user('{"a":1,"b":"two"}')} />);
    const codeEl = container.querySelector('code[class*="language-json"]');
    expect(codeEl).not.toBeNull();
    expect(codeEl?.textContent).toContain('"a"');
    expect(codeEl?.textContent).toContain('"two"');
  });

  it('does not pretty-print invalid JSON (falls through to markdown)', () => {
    // Starts with `{` but is not valid JSON — should render as markdown text,
    // not as a code block.
    const { container } = render(<CodexUserMessage item={user('{ unbalanced')} />);
    expect(container.querySelector('code[class*="language-json"]')).toBeNull();
    expect(container.textContent).toContain('{ unbalanced');
  });

  // ---------------------------------------------------------------------------
  // CF-360 — deep-link target + row-actions
  // ---------------------------------------------------------------------------

  it('applies the deepLinkTarget class when isDeepLinkTarget is true', () => {
    const { container } = render(
      <CodexUserMessage item={user('hi')} isDeepLinkTarget />,
    );
    expect(container.firstChild).toHaveClass(/deepLinkTarget/);
  });

  it('does not apply deepLinkTarget by default', () => {
    const { container } = render(<CodexUserMessage item={user('hi')} />);
    expect(container.firstChild).not.toHaveClass(/deepLinkTarget/);
  });

  it('renders row-actions (copy-link) when sessionId is provided', () => {
    const { getByLabelText } = render(
      <CodexUserMessage item={user('hi')} sessionId="abc" />,
    );
    expect(getByLabelText(/copy link/i)).toBeInTheDocument();
  });

  it('omits row-actions when sessionId is absent', () => {
    const { queryByLabelText } = render(<CodexUserMessage item={user('hi')} />);
    expect(queryByLabelText(/copy link/i)).toBeNull();
  });
});
