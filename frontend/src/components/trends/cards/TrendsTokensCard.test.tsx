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
