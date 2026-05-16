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

  // ---------------------------------------------------------------------------
  // CF-388 — image rendering
  // ---------------------------------------------------------------------------

  describe('image rendering', () => {
    const SRC_1 = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkAAIAAAoAAv/lxKUAAAAASUVORK5CYII=';
    const SRC_2 = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAcAAc6POE4AAAAASUVORK5CYII=';

    it('renders an <img> with the data URL src when images are present', () => {
      const { container } = render(
        <CodexAssistantMessage item={assistant({ images: [SRC_1] })} />,
      );
      const img = container.querySelector('img');
      expect(img).not.toBeNull();
      expect(img?.getAttribute('src')).toBe(SRC_1);
    });

    it('applies loading="lazy" to the rendered <img>', () => {
      const { container } = render(
        <CodexAssistantMessage item={assistant({ images: [SRC_1] })} />,
      );
      const img = container.querySelector('img');
      expect(img?.getAttribute('loading')).toBe('lazy');
    });

    it('uses an alt text that identifies the image as assistant-generated and 1-indexed', () => {
      const { container } = render(
        <CodexAssistantMessage item={assistant({ images: [SRC_1, SRC_2] })} />,
      );
      const imgs = Array.from(container.querySelectorAll('img'));
      expect(imgs.length).toBe(2);
      expect(imgs[0]?.getAttribute('alt')).toBe('Assistant-generated image #1');
      expect(imgs[1]?.getAttribute('alt')).toBe('Assistant-generated image #2');
    });

    it('does not render any <img> when item.images is undefined', () => {
      const { container } = render(<CodexAssistantMessage item={assistant()} />);
      expect(container.querySelector('img')).toBeNull();
    });
  });

  // ---------------------------------------------------------------------------
  // CF-362 — cost-mode badges
  // ---------------------------------------------------------------------------

  describe('cost mode', () => {
    const usage = {
      input_tokens: 12_345,
      output_tokens: 1_200,
      cached_input_tokens: 200,
      reasoning_output_tokens: 250,
    };

    it('renders the $ cost badge when isCostMode is on and messageCost is provided', () => {
      render(
        <CodexAssistantMessage
          item={assistant({ usage })}
          isCostMode
          messageCost={0.42}
        />,
      );
      expect(screen.getByText('$0.42')).toBeInTheDocument();
    });

    it('renders a token pill showing gross input and (output + reasoning) output', () => {
      // Use a reasoning value that produces a crisp rounded display.
      const pillUsage = {
        input_tokens: 12_345,
        output_tokens: 1_200,
        reasoning_output_tokens: 300, // 1200 + 300 = 1500 -> "1.5k"
      };
      const { container } = render(
        <CodexAssistantMessage
          item={assistant({ usage: pillUsage })}
          isCostMode
          messageCost={0.42}
        />,
      );
      // The pill is a single <span> with `{N} in · {N} out` JSX that yields
      // multiple text nodes; assert on the merged textContent instead of
      // relying on getByText to walk across text-node boundaries.
      const pill = Array.from(container.querySelectorAll('span')).find((el) =>
        /\bin\b/.test(el.textContent ?? '') && /\bout\b/.test(el.textContent ?? ''),
      );
      expect(pill).toBeTruthy();
      expect(pill?.textContent).toMatch(/12\.3k\s*in/);
      expect(pill?.textContent).toMatch(/1\.5k\s*out/);
    });

    it('renders a cache pill when cached_input_tokens > 0', () => {
      render(
        <CodexAssistantMessage
          item={assistant({ usage })}
          isCostMode
          messageCost={0.42}
        />,
      );
      expect(screen.getByText(/200 hit/)).toBeInTheDocument();
    });

    it('omits the cache pill when cached_input_tokens is 0', () => {
      render(
        <CodexAssistantMessage
          item={assistant({ usage: { ...usage, cached_input_tokens: 0 } })}
          isCostMode
          messageCost={0.42}
        />,
      );
      expect(screen.queryByText(/hit/)).not.toBeInTheDocument();
    });

    it('does not render any cost badges when isCostMode is false', () => {
      const { container } = render(
        <CodexAssistantMessage
          item={assistant({ usage })}
          isCostMode={false}
          messageCost={0.42}
        />,
      );
      expect(screen.queryByText('$0.42')).not.toBeInTheDocument();
      // No span carries the "N in · N out" pill text.
      const pill = Array.from(container.querySelectorAll('span')).find((el) =>
        /\bin\b/.test(el.textContent ?? '') && /\bout\b/.test(el.textContent ?? ''),
      );
      expect(pill).toBeUndefined();
    });

    it('does not render badges when usage is undefined (no token_count seen yet)', () => {
      render(
        <CodexAssistantMessage item={assistant()} isCostMode messageCost={0.42} />,
      );
      expect(screen.queryByText('$0.42')).not.toBeInTheDocument();
    });

    it('does not render badges when messageCost is 0 (zero-cost no-op call)', () => {
      render(
        <CodexAssistantMessage
          item={assistant({ usage })}
          isCostMode
          messageCost={0}
        />,
      );
      expect(screen.queryByText('$0.00')).not.toBeInTheDocument();
    });

    it('attaches a verbose multi-line tooltip on the cost badge', () => {
      render(
        <CodexAssistantMessage
          item={assistant({ usage })}
          isCostMode
          messageCost={0.42}
        />,
      );
      const costBadge = screen.getByText('$0.42');
      const tooltip = costBadge.getAttribute('title');
      expect(tooltip).toBeTruthy();
      expect(tooltip).toContain('$0.42');
      expect(tooltip).toContain('Input tokens (in): 12,345');
      expect(tooltip).toContain('Cached (hit): 200');
      expect(tooltip).toContain('Reasoning: 250');
    });
  });
});
