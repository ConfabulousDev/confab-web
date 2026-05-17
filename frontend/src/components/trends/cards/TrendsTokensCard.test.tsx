import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { TrendsTokensCard } from './TrendsTokensCard';
import type { TrendsTokensCard as TrendsTokensCardData } from '@/schemas/api';

// CF-424: When the filtered trends data spans 2+ providers, the Tokens card
// renders a small muted caveat ("Totals include sessions across multiple AI
// providers.") under the headline numbers. The full per-provider breakdown
// is the subject of a follow-up ticket — this caveat is the interim signal.

const sampleData: TrendsTokensCardData = {
  total_input_tokens: 1000,
  total_output_tokens: 500,
  total_cache_creation_tokens: 100,
  total_cache_read_tokens: 200,
  total_cost_usd: '5.00',
  daily_costs: [{ date: '2025-01-01', cost_usd: '5.00' }],
};

describe('TrendsTokensCard multi-provider caveat (CF-424)', () => {
  it('does not render the caveat when providersPresent is empty', () => {
    render(<TrendsTokensCard data={sampleData} providersPresent={[]} />);
    expect(screen.queryByText(/multiple ai providers/i)).not.toBeInTheDocument();
  });

  it('does not render the caveat when providersPresent has a single provider', () => {
    render(<TrendsTokensCard data={sampleData} providersPresent={['claude-code']} />);
    expect(screen.queryByText(/multiple ai providers/i)).not.toBeInTheDocument();
  });

  it('renders the caveat when providersPresent has two or more providers', () => {
    render(
      <TrendsTokensCard data={sampleData} providersPresent={['claude-code', 'codex']} />
    );
    expect(screen.getByText(/totals include sessions across multiple ai providers/i)).toBeInTheDocument();
  });

  it('does not crash when providersPresent prop is omitted (defaults to no caveat)', () => {
    render(<TrendsTokensCard data={sampleData} />);
    expect(screen.queryByText(/multiple ai providers/i)).not.toBeInTheDocument();
  });
});

// CF-436: TrendsTokensCard mirrors the per-session TokensCard hide rule for
// cache writes. OpenAI doesn't bill cache writes, so a Codex-only filtered
// window has total_cache_creation_tokens === 0. Tri-state:
//   - create > 0           → 'Cache (Create / Read)' with 'X / Y'
//   - create = 0, read > 0 → 'Cache Read' with read total only
//   - both 0               → row hidden entirely
describe('TrendsTokensCard cache row (CF-436)', () => {
  it("renders 'Cache (Create / Read)' with 'X / Y' when creation > 0", () => {
    render(<TrendsTokensCard data={sampleData} />);
    expect(screen.getByText('Cache (Create / Read)')).toBeInTheDocument();
    expect(screen.queryByText('Cache Read')).not.toBeInTheDocument();
  });

  it("renders 'Cache Read' label only when creation is 0 and read > 0", () => {
    const data: TrendsTokensCardData = {
      ...sampleData,
      total_cache_creation_tokens: 0,
      total_cache_read_tokens: 500,
    };
    render(<TrendsTokensCard data={data} />);
    expect(screen.getByText('Cache Read')).toBeInTheDocument();
    expect(screen.queryByText('Cache (Create / Read)')).not.toBeInTheDocument();
    // Value shows only the read total, no '/' separator.
    expect(screen.getByText('500')).toBeInTheDocument();
  });

  it('hides the cache row entirely when both creation and read are 0', () => {
    const data: TrendsTokensCardData = {
      ...sampleData,
      total_cache_creation_tokens: 0,
      total_cache_read_tokens: 0,
    };
    render(<TrendsTokensCard data={data} />);
    expect(screen.queryByText('Cache (Create / Read)')).not.toBeInTheDocument();
    expect(screen.queryByText('Cache Read')).not.toBeInTheDocument();
  });
});
