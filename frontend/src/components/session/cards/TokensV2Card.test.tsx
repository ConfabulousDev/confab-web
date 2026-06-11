import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { TokensV2Card, type TokensV2CardData } from './TokensV2Card';

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
    render(<TokensV2Card data={makeData()} loading={false} />);
    expect(screen.getByText('$1.23')).toBeInTheDocument();
  });

  it('renders total input tokens formatted', () => {
    render(<TokensV2Card data={makeData()} loading={false} />);
    expect(screen.getByText('150.0k')).toBeInTheDocument();
  });

  it('renders total output tokens formatted', () => {
    render(<TokensV2Card data={makeData()} loading={false} />);
    expect(screen.getAllByText('50.0k').length).toBeGreaterThanOrEqual(1);
  });

  it('renders provider names', () => {
    render(<TokensV2Card data={makeData()} loading={false} />);
    expect(screen.getByText('anthropic')).toBeInTheDocument();
    expect(screen.getByText('openai')).toBeInTheDocument();
  });

  it('renders per-provider cost', () => {
    render(<TokensV2Card data={makeData()} loading={false} />);
    expect(screen.getAllByText('$0.95').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('$0.28').length).toBeGreaterThanOrEqual(1);
  });

  it('renders formatted model names (not raw ids)', () => {
    render(<TokensV2Card data={makeData()} loading={false} />);
    expect(screen.getByText('Sonnet 4')).toBeInTheDocument();
    expect(screen.getByText('GPT-4o')).toBeInTheDocument();
    expect(screen.queryByText('claude-sonnet-4-20250514')).not.toBeInTheDocument();
  });

  it('renders loading state', () => {
    render(<TokensV2Card data={null} loading={true} />);
    expect(screen.getByText('Loading...')).toBeInTheDocument();
  });

  it('renders error state', () => {
    render(<TokensV2Card data={null} loading={false} error="compute failed" />);
    expect(screen.getByText(/compute failed/)).toBeInTheDocument();
  });

  it('returns null when no data and not loading', () => {
    const { container } = render(<TokensV2Card data={null} loading={false} />);
    expect(container.firstChild).toBeNull();
  });

  it('multi-provider DOES render provider section headers', () => {
    render(<TokensV2Card data={makeData()} loading={false} />);
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
    render(<TokensV2Card data={singleProvider} loading={false} />);
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
    render(<TokensV2Card data={withFast} loading={false} />);
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
    render(<TokensV2Card data={withUnknown} loading={false} />);
    expect(screen.getByText('Unknown')).toBeInTheDocument();
  });

  it('shows the unpriced ($0) warning tooltip on the cost row', () => {
    render(<TokensV2Card data={makeData({ total_cost_usd: '0.00' })} loading={false} />);
    const costRow = screen.getByText('Estimated cost').closest('div');
    expect(costRow).toHaveAttribute('title', 'Cost unavailable — session may use models not yet in the pricing table');
  });
});
