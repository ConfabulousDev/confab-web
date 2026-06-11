import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { TrendsCostByModelCard } from './TrendsCostByModelCard';
import type {
  TrendsCostByModelCard as TrendsCostByModelCardData,
  TrendsCostByModelRow,
} from '@/schemas/api';

function row(overrides: Partial<TrendsCostByModelRow> = {}): TrendsCostByModelRow {
  return {
    model: 'opus-4-5',
    provider: 'claude-code',
    cost_usd: '1.00',
    pct_of_total: 25,
    input: 1000,
    output: 500,
    cache_read: 100,
    cache_write: 50,
    session_count: 1,
    ...overrides,
  };
}

function makeData(overrides: Partial<TrendsCostByModelCardData> = {}): TrendsCostByModelCardData {
  return {
    rows: [row()],
    covered_session_count: 1,
    total_session_count: 1,
    timed_out: false,
    ...overrides,
  };
}

describe('TrendsCostByModelCard', () => {
  it('renders one row per (provider, model) with formatted model label and cost', () => {
    render(
      <TrendsCostByModelCard
        data={makeData({
          rows: [
            row({ model: 'opus-4-5', provider: 'claude-code', cost_usd: '1.00' }),
            row({ model: 'gpt-5', provider: 'codex', cost_usd: '2.00' }),
          ],
        })}
      />,
    );
    expect(screen.getByText('Opus 4.5')).toBeInTheDocument();
    expect(screen.getByText('GPT-5')).toBeInTheDocument();
    expect(screen.getByText('$2.00')).toBeInTheDocument();
  });

  it("renders the empty model key as 'Unknown'", () => {
    render(<TrendsCostByModelCard data={makeData({ rows: [row({ model: '', cost_usd: '0.00' })] })} />);
    expect(screen.getByText('Unknown')).toBeInTheDocument();
  });

  it("keeps the '· fast' suffix on the fast variant label", () => {
    render(
      <TrendsCostByModelCard data={makeData({ rows: [row({ model: 'opus-4-5 · fast' })] })} />,
    );
    expect(screen.getByText(/Opus 4\.5.*·.*fast/)).toBeInTheDocument();
  });

  it('shows split cache read / write values (not a combined sum)', () => {
    render(
      <TrendsCostByModelCard
        data={makeData({ rows: [row({ cache_read: 1234, cache_write: 56 })] })}
      />,
    );
    // Distinct read "/" write — a summed value (1290) must NOT appear.
    expect(screen.getByText(/Cache R\/W: 1\.2k \/ 56/)).toBeInTheDocument();
  });

  it('renders a coverage caption (N of M sessions), not a reconciliation line', () => {
    render(
      <TrendsCostByModelCard data={makeData({ covered_session_count: 3, total_session_count: 7 })} />,
    );
    expect(screen.getByText(/Covers 3 of 7/i)).toBeInTheDocument();
  });

  it('renders a timed-out notice instead of an empty state when timed_out is true', () => {
    render(
      <TrendsCostByModelCard
        data={makeData({ rows: [], covered_session_count: 0, total_session_count: 0, timed_out: true })}
      />,
    );
    expect(screen.getByText(/narrow/i)).toBeInTheDocument();
  });
});
