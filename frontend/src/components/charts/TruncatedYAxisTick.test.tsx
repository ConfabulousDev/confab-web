import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import { TruncatedYAxisTick } from './TruncatedYAxisTick';

function renderTick(props: Parameters<typeof TruncatedYAxisTick>[0]) {
  return render(
    <svg>
      <TruncatedYAxisTick {...props} />
    </svg>
  );
}

describe('TruncatedYAxisTick', () => {
  it('returns null when payload is undefined', () => {
    const { container } = renderTick({ x: 10, y: 20 });
    expect(container.querySelector('g')).toBeNull();
  });

  it('renders truncated text and full name as <title> tooltip', () => {
    const longName = 'an-extremely-long-agent-name-that-will-be-truncated';
    const { container } = renderTick({ x: 5, y: 12, payload: { value: longName } });

    const text = container.querySelector('text');
    const title = container.querySelector('title');
    expect(title?.textContent).toBe(longName);
    expect(text?.textContent).not.toBe(longName);
    expect(text?.textContent?.length).toBeLessThan(longName.length);
  });

  it('renders the full name verbatim in <text> when it is short', () => {
    const { container } = renderTick({ x: 0, y: 0, payload: { value: 'short' } });
    expect(container.querySelector('text')?.textContent).toBe('short');
    expect(container.querySelector('title')?.textContent).toBe('short');
  });

  it('applies x,y translate transform', () => {
    const { container } = renderTick({ x: 11, y: 22, payload: { value: 'a' } });
    expect(container.querySelector('g')?.getAttribute('transform')).toBe('translate(11,22)');
  });
});
