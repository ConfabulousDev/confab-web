// Locks the contract that phase: 'commentary' vs 'final' renders with
// different styling, and that the model badge appears.

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import CodexAssistantMessage from './CodexAssistantMessage';
import type { CodexAssistantItem } from '@/types/codexRenderItem';

function assistant(overrides: Partial<CodexAssistantItem> = {}): CodexAssistantItem {
  return {
    kind: 'assistant',
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
});
