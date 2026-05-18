import { describe, it, expect } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { TrendsTokensCard } from './TrendsTokensCard';
import type {
  TrendsTokensCard as TrendsTokensCardData,
  TrendsTokensPerProvider,
} from '@/schemas/api';
import { TrendsResponseSchema } from '@/schemas/api';

// CF-435 spec tests. The Tokens card on Personal Trends switches between two
// layouts based on the `per_provider` map's key count:
//   - 0 or 1 keys → existing single-series StatRow stack
//   - 2+ keys     → per-provider <table> replacing every headline StatRow
//
// The CF-424 muted caveat ("Totals include sessions across multiple AI
// providers.") is removed entirely — the table now communicates this directly.

function entry(overrides: Partial<TrendsTokensPerProvider> = {}): TrendsTokensPerProvider {
  return {
    total_input_tokens: 0,
    total_output_tokens: 0,
    total_cache_creation_tokens: 0,
    total_cache_read_tokens: 0,
    total_cost_usd: '0',
    ...overrides,
  };
}

function makeData(
  perProvider: Record<string, TrendsTokensPerProvider>,
  overrides: Partial<TrendsTokensCardData> = {},
): TrendsTokensCardData {
  return {
    total_input_tokens: 1000,
    total_output_tokens: 500,
    total_cache_creation_tokens: 100,
    total_cache_read_tokens: 200,
    total_cost_usd: '5.00',
    daily_costs: [{ date: '2025-01-01', cost_usd: '5.00' }],
    per_provider: perProvider,
    ...overrides,
  };
}

describe('TrendsTokensCard caveat removal (CF-435)', () => {
  it('never renders the CF-424 muted caveat regardless of per_provider shape', () => {
    const data = makeData({
      'claude-code': entry({ total_input_tokens: 1000, total_cost_usd: '5.00' }),
      codex: entry({ total_input_tokens: 800, total_cost_usd: '2.00' }),
    });
    render(<TrendsTokensCard data={data} />);
    expect(screen.queryByText(/multiple ai providers/i)).not.toBeInTheDocument();
  });
});

describe('TrendsTokensCard single-provider mode (CF-435)', () => {
  it('renders the existing StatRow layout when per_provider has one key', () => {
    const data = makeData({
      'claude-code': entry({
        total_input_tokens: 1000,
        total_output_tokens: 500,
        total_cache_creation_tokens: 100,
        total_cache_read_tokens: 200,
        total_cost_usd: '5.00',
      }),
    });
    render(<TrendsTokensCard data={data} />);

    // StatRow stack present.
    expect(screen.getByText('Total Cost')).toBeInTheDocument();
    expect(screen.getByText('Total Tokens')).toBeInTheDocument();
    expect(screen.getByText('Input / Output')).toBeInTheDocument();

    // No table.
    expect(screen.queryByRole('table')).not.toBeInTheDocument();
  });

  it('renders the existing StatRow layout when per_provider is empty {}', () => {
    const data = makeData({});
    render(<TrendsTokensCard data={data} />);

    expect(screen.getByText('Total Cost')).toBeInTheDocument();
    expect(screen.queryByRole('table')).not.toBeInTheDocument();
  });

  it('treats missing per_provider as empty map (Zod default) — single-series layout', () => {
    // Backwards-compat: older backends that don't ship per_provider still parse.
    // Build a TrendsResponse without per_provider on the tokens card and verify
    // the schema defaults it to {}, falling back to the single-series layout.
    const wireResponse = {
      computed_at: '2025-01-01T00:00:00Z',
      date_range: { start_date: '2025-01-01', end_date: '2025-01-01' },
      session_count: 1,
      repos_included: [],
      include_no_repo: true,
      providers_present: ['claude-code'],
      cards: {
        overview: null,
        tokens: {
          total_input_tokens: 1000,
          total_output_tokens: 500,
          total_cache_creation_tokens: 100,
          total_cache_read_tokens: 200,
          total_cost_usd: '5.00',
          daily_costs: [{ date: '2025-01-01', cost_usd: '5.00' }],
          // per_provider intentionally omitted — Zod must default to {}.
        },
        activity: null,
        tools: null,
        utilization: null,
        agents_and_skills: null,
        top_sessions: null,
      },
    };
    const parsed = TrendsResponseSchema.parse(wireResponse);
    expect(parsed.cards.tokens?.per_provider).toEqual({});

    render(<TrendsTokensCard data={parsed.cards.tokens} />);
    expect(screen.getByText('Total Cost')).toBeInTheDocument();
    expect(screen.queryByRole('table')).not.toBeInTheDocument();
  });
});

describe('TrendsTokensCard multi-provider table (CF-435)', () => {
  it('renders a table with header columns Provider/Input/Output/Cache Read/Cache Create/Cost when per_provider has 2+ keys', () => {
    const data = makeData({
      'claude-code': entry({
        total_input_tokens: 5_000_000,
        total_output_tokens: 2_000_000,
        total_cache_creation_tokens: 100_000,
        total_cache_read_tokens: 500_000,
        total_cost_usd: '125.00',
      }),
      codex: entry({
        total_input_tokens: 800_000,
        total_output_tokens: 150_000,
        total_cache_creation_tokens: 0,
        total_cache_read_tokens: 120_000,
        total_cost_usd: '4.25',
      }),
    });
    render(<TrendsTokensCard data={data} />);

    const table = screen.getByRole('table');
    expect(table).toBeInTheDocument();

    // All six expected column headers present.
    expect(within(table).getByRole('columnheader', { name: 'Provider' })).toBeInTheDocument();
    expect(within(table).getByRole('columnheader', { name: 'Input' })).toBeInTheDocument();
    expect(within(table).getByRole('columnheader', { name: 'Output' })).toBeInTheDocument();
    expect(within(table).getByRole('columnheader', { name: 'Cache Read' })).toBeInTheDocument();
    expect(within(table).getByRole('columnheader', { name: 'Cache Create' })).toBeInTheDocument();
    expect(within(table).getByRole('columnheader', { name: 'Cost' })).toBeInTheDocument();

    // Provider labels via providerLabel() — canonical brand strings.
    expect(within(table).getByText('Claude Code')).toBeInTheDocument();
    expect(within(table).getByText('Codex')).toBeInTheDocument();
  });

  it('replaces the headline StatRows entirely (Total Cost / Total Tokens / Input/Output / Cache rows hidden)', () => {
    const data = makeData({
      'claude-code': entry({ total_input_tokens: 5_000_000, total_cost_usd: '125.00' }),
      codex: entry({ total_input_tokens: 800_000, total_cost_usd: '4.25' }),
    });
    render(<TrendsTokensCard data={data} />);

    // None of the single-series StatRow labels should appear above the table.
    // ("Cache Read" is intentionally NOT asserted here because the new table
    // uses it as a column header.)
    expect(screen.queryByText('Total Cost')).not.toBeInTheDocument();
    expect(screen.queryByText('Total Tokens')).not.toBeInTheDocument();
    expect(screen.queryByText('Input / Output')).not.toBeInTheDocument();
    expect(screen.queryByText('Cache (Create / Read)')).not.toBeInTheDocument();
  });

  it('sorts provider rows alphabetically by canonical id', () => {
    // Insert codex BEFORE claude-code in the map iteration order to make sure
    // the component sorts rather than preserving JSON order.
    const data = makeData({
      codex: entry({ total_input_tokens: 800_000, total_cost_usd: '4.25' }),
      'claude-code': entry({ total_input_tokens: 5_000_000, total_cost_usd: '125.00' }),
    });
    render(<TrendsTokensCard data={data} />);

    const table = screen.getByRole('table');
    const bodyRows = within(table).getAllByRole('row').slice(1); // drop header row
    // First data row should be claude-code (alphabetically first), Codex second,
    // Total last.
    expect(within(bodyRows[0]!).getByText('Claude Code')).toBeInTheDocument();
    expect(within(bodyRows[1]!).getByText('Codex')).toBeInTheDocument();
    expect(within(bodyRows[2]!).getByText('Total')).toBeInTheDocument();
  });

  it("renders em-dashes ('—') for all four token columns in the Total row, real dollars for Cost", () => {
    const data = makeData(
      {
        'claude-code': entry({
          total_input_tokens: 5_000_000,
          total_output_tokens: 2_000_000,
          total_cache_creation_tokens: 100_000,
          total_cache_read_tokens: 500_000,
          total_cost_usd: '125.00',
        }),
        codex: entry({
          total_input_tokens: 800_000,
          total_output_tokens: 150_000,
          total_cache_creation_tokens: 0,
          total_cache_read_tokens: 120_000,
          total_cost_usd: '4.25',
        }),
      },
      // Backend ships the cross-provider rollup; the table's Total row reads it.
      { total_cost_usd: '129.25' },
    );
    render(<TrendsTokensCard data={data} />);

    const table = screen.getByRole('table');
    const bodyRows = within(table).getAllByRole('row').slice(1);
    const totalRow = bodyRows[bodyRows.length - 1]!;
    const totalCells = within(totalRow).getAllByRole('cell');
    // Layout: [Provider, Input, Output, Cache Read, Cache Create, Cost]
    // Provider cell shows "Total"; next 4 cells are em-dashes; Cost shows a $-value.
    expect(totalCells[0]?.textContent).toBe('Total');
    expect(totalCells[1]?.textContent).toBe('—');
    expect(totalCells[2]?.textContent).toBe('—');
    expect(totalCells[3]?.textContent).toBe('—');
    expect(totalCells[4]?.textContent).toBe('—');
    expect(totalCells[5]?.textContent).toMatch(/\$129\.25/);
  });

  it("dashes Codex's per-row Cache Create cell when total_cache_creation_tokens === 0 (structural N/A)", () => {
    const data = makeData({
      'claude-code': entry({
        total_input_tokens: 5_000_000,
        total_cache_creation_tokens: 100_000,
        total_cost_usd: '125.00',
      }),
      codex: entry({
        total_input_tokens: 800_000,
        total_cache_creation_tokens: 0,
        total_cost_usd: '4.25',
      }),
    });
    render(<TrendsTokensCard data={data} />);

    const table = screen.getByRole('table');
    const bodyRows = within(table).getAllByRole('row').slice(1);
    const codexRow = bodyRows.find((r) => within(r).queryByText('Codex'))!;
    expect(codexRow).toBeTruthy();
    const codexCells = within(codexRow).getAllByRole('cell');
    // Cache Create is column index 4.
    expect(codexCells[4]?.textContent).toBe('—');

    // Claude's Cache Create cell stays a literal formatted number (not dashed).
    const claudeRow = bodyRows.find((r) => within(r).queryByText('Claude Code'))!;
    const claudeCells = within(claudeRow).getAllByRole('cell');
    expect(claudeCells[4]?.textContent).not.toBe('—');
  });

  it('renders an unknown provider id verbatim as the row label (no icon, no error)', () => {
    const data = makeData({
      'claude-code': entry({ total_input_tokens: 1000, total_cost_usd: '5.00' }),
      gemini: entry({ total_input_tokens: 200, total_cost_usd: '0.80' }),
    });
    render(<TrendsTokensCard data={data} />);

    const table = screen.getByRole('table');
    expect(within(table).getByText('gemini')).toBeInTheDocument();
  });

  it('renders provider rows even when a present provider has all-zero data (LEFT JOIN unmeasured case)', () => {
    // A Codex session existed in the filtered range but had no session_card_tokens row,
    // so the backend returns a per_provider entry with zeros across the board.
    const data = makeData({
      'claude-code': entry({ total_input_tokens: 5_000_000, total_cost_usd: '125.00' }),
      codex: entry(), // all zeros, cost '0'
    });
    render(<TrendsTokensCard data={data} />);

    const table = screen.getByRole('table');
    const bodyRows = within(table).getAllByRole('row').slice(1);
    const codexRow = bodyRows.find((r) => within(r).queryByText('Codex'));
    expect(codexRow).toBeTruthy();
  });

  it('keeps the daily-cost chart below the table in multi-provider mode', () => {
    const data = makeData(
      {
        'claude-code': entry({ total_input_tokens: 5_000_000, total_cost_usd: '125.00' }),
        codex: entry({ total_input_tokens: 800_000, total_cost_usd: '4.25' }),
      },
      {
        daily_costs: [
          { date: '2025-01-01', cost_usd: '5.00' },
          { date: '2025-01-02', cost_usd: '6.50' },
        ],
      },
    );
    render(<TrendsTokensCard data={data} />);
    expect(screen.getByText('Daily Cost')).toBeInTheDocument();
  });
});
