// Locks the contract that phase: 'commentary' vs 'final' renders with
// different styling, that the model badge appears, and that the assistant
// text is rendered through the shared markdown / JSON pretty-print pipeline.

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import CodexAssistantMessage from './CodexAssistantMessage';
import type { CodexAssistantItem } from '@/types/codexRenderItem';

function assistant(overrides: Partial<CodexAssistantItem> = {}): CodexAssistantItem {
  return {
    kind: 'assistant',
    lineId: '0',
    timestamp: '2026-05-13T01:00:00Z',
    text: 'Hello from the assistant.',
    phase: 'final',
    model: 'gpt-5',
    ...overrides,
  };
}

describe('CodexAssistantMessage', () => {
  it('renders the assistant text', () => {
    render(<CodexAssistantMessage item={assistant()} />);
    expect(screen.getByText(/Hello from the assistant\./)).toBeInTheDocument();
  });

  it('displays the model name as a visible badge', () => {
    render(<CodexAssistantMessage item={assistant({ model: 'gpt-5' })} />);
    expect(screen.getByText(/gpt-5/)).toBeInTheDocument();
  });

  it('applies a distinct DOM marker for phase: commentary vs final', () => {
    const { container: commentaryEl } = render(
      <CodexAssistantMessage item={assistant({ phase: 'commentary' })} />,
    );
    const { container: finalEl } = render(
      <CodexAssistantMessage item={assistant({ phase: 'final' })} />,
    );
    // Some attribute, class, or data-* on the root must differ between the two.
    // Use innerHTML as a coarse signal; if equal, the component has not
    // distinguished phases.
    expect(commentaryEl.innerHTML).not.toBe(finalEl.innerHTML);
  });

  // ---------------------------------------------------------------------------
  // Selection / newSpeaker contract (CF-357)
  // ---------------------------------------------------------------------------

  it('applies the selected class when isSelected is true', () => {
    const { container } = render(
      <CodexAssistantMessage item={assistant()} isSelected />,
    );
    expect(container.firstChild).toHaveClass(/selected/);
  });

  it('does not apply the selected class by default', () => {
    const { container } = render(<CodexAssistantMessage item={assistant()} />);
    expect(container.firstChild).not.toHaveClass(/selected/);
  });

  it('applies the newSpeaker class when isNewSpeaker is true', () => {
    const { container } = render(
      <CodexAssistantMessage item={assistant()} isNewSpeaker />,
    );
    expect(container.firstChild).toHaveClass(/newSpeaker/);
  });

  // ---------------------------------------------------------------------------
  // Markdown rendering parity (CF-358)
  // ---------------------------------------------------------------------------

  it('renders markdown headings as heading elements', () => {
    const { container } = render(
      <CodexAssistantMessage item={assistant({ text: '# Heading One\n\nbody copy' })} />,
    );
    // marked produces <h1>; we just need a heading element to exist.
    expect(container.querySelector('h1, h2, h3')).not.toBeNull();
    expect(container.querySelector('h1, h2, h3')?.textContent).toContain('Heading One');
  });

  it('renders inline code in a <code> element', () => {
    const { container } = render(
      <CodexAssistantMessage item={assistant({ text: 'run `pwd` first' })} />,
    );
    const code = container.querySelector('code');
    expect(code).not.toBeNull();
    expect(code?.textContent).toBe('pwd');
  });

  it('renders fenced code with a prism language class', () => {
    const text = ['Here is some TS:', '', '```ts', 'type X = number', '```'].join('\n');
    const { container } = render(<CodexAssistantMessage item={assistant({ text })} />);
    const codeEl = container.querySelector('code[class*="language-"]');
    expect(codeEl).not.toBeNull();
    // Either `language-typescript` (via the alias map) or `language-ts` is
    // acceptable; the contract is "prism language class present".
    expect(codeEl?.className).toMatch(/language-(typescript|ts)/);
  });

  it('pretty-prints JSON content as a syntax-highlighted block', () => {
    const { container } = render(
      <CodexAssistantMessage item={assistant({ text: '{"action":"run","cmd":"pwd"}' })} />,
    );
    const codeEl = container.querySelector('code[class*="language-json"]');
    expect(codeEl).not.toBeNull();
    expect(codeEl?.textContent).toContain('"action"');
    expect(codeEl?.textContent).toContain('"run"');
  });
});
