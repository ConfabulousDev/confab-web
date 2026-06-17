import { describe, expect, it } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { TokensV2Card, type TokensV2CardData } from './TokensV2Card';

// d3rp: single-provider, single MODEL section → auto-expands (1 section total).
function makeSingleModel(): TokensV2CardData {
  return {
    total_cost_usd: '0.95',
    total_input: 100000,
    total_output: 30000,
    by_provider: {
      anthropic: {
        cost_usd: '0.95',
        models: {
          'claude-sonnet-4-20250514': {
            input: 100000, output: 30000, cache_read: 20000,
            cache_write: 5000, reasoning: 10000, cost_usd: '0.95',
          },
        },
      },
    },
  };
}

// d3rp: single provider, TWO model sections → both collapsed by default. Each
// model carries a detail row the other lacks so independence is observable:
// the base model has Cache write (>0, fast has 0); the fast model has Reasoning
// (>0, base has 0).
function makeTwoModel(): TokensV2CardData {
  return {
    total_cost_usd: '4.90',
    total_input: 1_200_000,
    total_output: 340_000,
    by_provider: {
      'claude-code': {
        cost_usd: '4.90',
        models: {
          'opus-4-8': {
            input: 1_000_000, output: 300_000, cache_read: 800_000,
            cache_write: 50_000, reasoning: 0, cost_usd: '3.50',
          },
          'opus-4-8 · fast': {
            input: 200_000, output: 40_000, cache_read: 0,
            cache_write: 0, reasoning: 12_000, cost_usd: '1.40',
          },
        },
      },
    },
  };
}

function makeData(overrides: Partial<TokensV2CardData> = {}): TokensV2CardData {
  return {
    total_cost_usd: '1.23',
    total_input: 150000,
    total_output: 50000,
    by_provider: {
      anthropic: {
        cost_usd: '0.95',
        models: {
          'claude-sonnet-4-20250514': {
            input: 100000,
            output: 30000,
            cache_read: 20000,
            cache_write: 5000,
            reasoning: 10000,
            cost_usd: '0.95',
          },
        },
      },
      openai: {
        cost_usd: '0.28',
        models: {
          'gpt-4o': {
            input: 50000,
            output: 20000,
            cache_read: 10000,
            cache_write: 0,
            reasoning: 0,
            cost_usd: '0.28',
          },
        },
      },
    },
    ...overrides,
  };
}

describe('TokensV2Card', () => {
  it('renders total cost', () => {
    render(<TokensV2Card data={makeData()} loading={false} provider="claude-code" />);
    expect(screen.getByText('$1.23')).toBeInTheDocument();
  });

  it('renders total tokens (input + output) in the summary stack', () => {
    render(<TokensV2Card data={makeData()} loading={false} provider="claude-code" />);
    // total_input 150k + total_output 50k = 200k
    expect(screen.getByText('200.0k')).toBeInTheDocument();
  });

  it('renders a combined Input / Output summary row', () => {
    render(<TokensV2Card data={makeData()} loading={false} provider="claude-code" />);
    expect(screen.getByText('Total Tokens')).toBeInTheDocument();
    expect(screen.getByText('Input / Output')).toBeInTheDocument();
    expect(screen.getByText('150.0k / 50.0k')).toBeInTheDocument();
  });

  it('renders provider names', () => {
    render(<TokensV2Card data={makeData()} loading={false} provider="claude-code" />);
    expect(screen.getByText('anthropic')).toBeInTheDocument();
    expect(screen.getByText('openai')).toBeInTheDocument();
  });

  it('renders per-provider cost', () => {
    render(<TokensV2Card data={makeData()} loading={false} provider="claude-code" />);
    expect(screen.getAllByText('$0.95').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('$0.28').length).toBeGreaterThanOrEqual(1);
  });

  it('renders formatted model names (not raw ids)', () => {
    render(<TokensV2Card data={makeData()} loading={false} provider="claude-code" />);
    expect(screen.getByText('Sonnet 4')).toBeInTheDocument();
    expect(screen.getByText('GPT-4o')).toBeInTheDocument();
    expect(screen.queryByText('claude-sonnet-4-20250514')).not.toBeInTheDocument();
  });

  it('renders loading state', () => {
    render(<TokensV2Card data={null} loading={true} provider="claude-code" />);
    expect(screen.getByText('Loading...')).toBeInTheDocument();
  });

  it('renders error state', () => {
    render(<TokensV2Card data={null} loading={false} provider="claude-code" error="compute failed" />);
    expect(screen.getByText(/compute failed/)).toBeInTheDocument();
  });

  it('returns null when no data and not loading', () => {
    const { container } = render(<TokensV2Card data={null} loading={false} provider="claude-code" />);
    expect(container.firstChild).toBeNull();
  });

  it('multi-provider DOES render provider section headers', () => {
    render(<TokensV2Card data={makeData()} loading={false} provider="claude-code" />);
    // by_provider keys (vendor ids) pass through providerLabel unchanged.
    expect(screen.getByText('anthropic')).toBeInTheDocument();
    expect(screen.getByText('openai')).toBeInTheDocument();
  });

  it('single provider collapses the provider wrapper (no provider header)', () => {
    const singleProvider = makeData({
      by_provider: {
        'claude-code': {
          cost_usd: '4.20',
          models: {
            'opus-4-8': {
              input: 1000000, output: 300000, cache_read: 800000,
              cache_write: 0, reasoning: 0, cost_usd: '3.50',
            },
          },
        },
      },
    });
    render(<TokensV2Card data={singleProvider} loading={false} provider="claude-code" />);
    // The provider label ("Claude Code") must NOT be rendered as a section header.
    expect(screen.queryByText('Claude Code')).not.toBeInTheDocument();
    // The model section still renders, with a formatted label.
    expect(screen.getByText('Opus 4.8')).toBeInTheDocument();
  });

  it('renders a fast model variant with the · fast suffix preserved', () => {
    const withFast = makeData({
      by_provider: {
        'claude-code': {
          cost_usd: '4.20',
          models: {
            'opus-4-8': {
              input: 1000000, output: 300000, cache_read: 0,
              cache_write: 0, reasoning: 0, cost_usd: '3.50',
            },
            'opus-4-8 · fast': {
              input: 200000, output: 40000, cache_read: 0,
              cache_write: 0, reasoning: 0, cost_usd: '0.70',
            },
          },
        },
      },
    });
    render(<TokensV2Card data={withFast} loading={false} provider="claude-code" />);
    expect(screen.getByText('Opus 4.8')).toBeInTheDocument();
    expect(screen.getByText('Opus 4.8 · fast')).toBeInTheDocument();
  });

  it('maps the empty-string model key to "Unknown"', () => {
    const withUnknown = makeData({
      by_provider: {
        'claude-code': {
          cost_usd: '0.00',
          models: {
            '': {
              input: 5000, output: 1000, cache_read: 0,
              cache_write: 0, reasoning: 0, cost_usd: '0',
            },
          },
        },
      },
    });
    render(<TokensV2Card data={withUnknown} loading={false} provider="claude-code" />);
    expect(screen.getByText('Unknown')).toBeInTheDocument();
  });

  it('shows the unpriced ($0) warning tooltip on the cost row', () => {
    render(<TokensV2Card data={makeData({ total_cost_usd: '0.00' })} loading={false} provider="claude-code" />);
    const costRow = screen.getByText('Estimated cost').closest('div');
    expect(costRow).toHaveAttribute('title', 'Cost unavailable — session may use models not yet in the pricing table');
  });

  // ---- d3rp: collapsible model sections ----

  describe('collapsible model sections (d3rp)', () => {
    it('collapses each model section by default when there is more than one', () => {
      render(<TokensV2Card data={makeTwoModel()} loading={false} provider="claude-code" />);
      // Per-model detail rows (labels that only ever appear inside a model
      // section) are hidden until expanded.
      expect(screen.queryByText('Cache read')).not.toBeInTheDocument();
      expect(screen.queryByText('Cache write')).not.toBeInTheDocument();
      expect(screen.queryByText('Reasoning')).not.toBeInTheDocument();
    });

    it('still shows each model headline (label + cost) when collapsed', () => {
      render(<TokensV2Card data={makeTwoModel()} loading={false} provider="claude-code" />);
      expect(screen.getByText('Opus 4.8')).toBeInTheDocument();
      expect(screen.getByText('Opus 4.8 · fast')).toBeInTheDocument();
      // Cost stays visible in the headline even while collapsed.
      expect(screen.getByText('$3.50')).toBeInTheDocument();
      expect(screen.getByText('$1.40')).toBeInTheDocument();
    });

    it('renders the model headline as a button with aria-expanded=false collapsed', () => {
      render(<TokensV2Card data={makeTwoModel()} loading={false} provider="claude-code" />);
      const headline = screen.getByRole('button', { name: /· fast/ });
      expect(headline).toHaveAttribute('aria-expanded', 'false');
    });

    it('aria-controls points at the (rendered-on-expand) detail region', () => {
      render(<TokensV2Card data={makeTwoModel()} loading={false} provider="claude-code" />);
      const headline = screen.getByRole('button', { name: /· fast/ });
      const controls = headline.getAttribute('aria-controls');
      if (!controls) throw new Error('headline is missing aria-controls');
      // Collapsed: region absent. Expanded: region present and carries that id.
      expect(document.getElementById(controls)).toBeNull();
      fireEvent.click(headline);
      expect(headline).toHaveAttribute('aria-expanded', 'true');
      const region = document.getElementById(controls);
      expect(region).not.toBeNull();
      expect(region).toHaveTextContent('Reasoning');
    });

    it('expands a collapsed section on click to reveal its detail', () => {
      render(<TokensV2Card data={makeTwoModel()} loading={false} provider="claude-code" />);
      expect(screen.queryByText('Reasoning')).not.toBeInTheDocument();
      fireEvent.click(screen.getByRole('button', { name: /· fast/ }));
      expect(screen.getByText('Reasoning')).toBeInTheDocument();
    });

    it('collapses again on a second click (toggle)', () => {
      render(<TokensV2Card data={makeTwoModel()} loading={false} provider="claude-code" />);
      const headline = screen.getByRole('button', { name: /· fast/ });
      fireEvent.click(headline);
      expect(screen.getByText('Reasoning')).toBeInTheDocument();
      fireEvent.click(headline);
      expect(screen.queryByText('Reasoning')).not.toBeInTheDocument();
    });

    it('toggles each section independently', () => {
      render(<TokensV2Card data={makeTwoModel()} loading={false} provider="claude-code" />);
      // Expand only the fast section; the base section stays collapsed, so the
      // base-only "Cache write" detail must NOT appear.
      fireEvent.click(screen.getByRole('button', { name: /· fast/ }));
      expect(screen.getByText('Reasoning')).toBeInTheDocument();
      expect(screen.queryByText('Cache write')).not.toBeInTheDocument();
    });

    it('auto-expands when there is exactly one model section total', () => {
      render(<TokensV2Card data={makeSingleModel()} loading={false} provider="claude-code" />);
      // No click: detail is already visible.
      expect(screen.getByText('Cache read')).toBeInTheDocument();
      expect(screen.getByText('Reasoning')).toBeInTheDocument();
      const headline = screen.getByRole('button', { name: /Sonnet 4/ });
      expect(headline).toHaveAttribute('aria-expanded', 'true');
    });
  });
});
