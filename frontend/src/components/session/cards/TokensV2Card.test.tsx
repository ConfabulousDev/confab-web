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

  it('renders model names', () => {
    render(<TokensV2Card data={makeData()} loading={false} />);
    expect(screen.getByText('claude-sonnet-4-20250514')).toBeInTheDocument();
    expect(screen.getByText('gpt-4o')).toBeInTheDocument();
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

  it('renders single provider without provider section header', () => {
    const singleProvider = makeData({
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
      },
    });
    render(<TokensV2Card data={singleProvider} loading={false} />);
    expect(screen.getAllByText('$0.95').length).toBeGreaterThanOrEqual(1);
  });

  it('shows zero cost with warning style', () => {
    render(<TokensV2Card data={makeData({ total_cost_usd: '0.00' })} loading={false} />);
    const costEl = screen.getByText('$0.00');
    expect(costEl).toBeInTheDocument();
  });
});
