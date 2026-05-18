import { describe, it, expect } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { TrendsTokensCard } from './TrendsTokensCard';
import type {
  TrendsTokensCard as TrendsTokensCardData,
  TrendsTokensPerProvider,
} from '@/schemas/api';
import { TrendsResponseSchema } from '@/schemas/api';

// The Tokens card switches between two layouts based on the `per_provider`
// map's key count:
//   - 0 or 1 keys → single StatRow stack (Total Cost / Total Tokens / Input/Output / Cache)
//   - 2+ keys     → top-level "Total Cost" StatRow + indented per-provider
//                   sections, each headed by the provider label with its own
//                   nested Cost / Total Tokens / Input/Output / Cache rows.

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
    daily_costs: [{ date: '2025-01-01', cost_usd: '5.00', per_provider: {} }],
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

describe('TrendsTokensCard single-provider mode', () => {
  it('renders the StatRow layout when per_provider has one key', () => {
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

    expect(screen.getByText('Total Cost')).toBeInTheDocument();
    expect(screen.getByText('Total Tokens')).toBeInTheDocument();
    expect(screen.getByText('Input / Output')).toBeInTheDocument();
    expect(screen.getByText('Cache (Create / Read)')).toBeInTheDocument();

    // No per-provider sub-headers in single-provider mode (only one provider,
    // so the heading would be redundant with the card title).
    expect(screen.queryByText('Claude Code')).not.toBeInTheDocument();
  });

  it('renders the StatRow layout when per_provider is empty {}', () => {
    const data = makeData({});
    render(<TrendsTokensCard data={data} />);

    expect(screen.getByText('Total Cost')).toBeInTheDocument();
    expect(screen.queryByText('Claude Code')).not.toBeInTheDocument();
  });

  it('treats missing per_provider as empty map (Zod default) — single-series layout', () => {
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
          daily_costs: [{ date: '2025-01-01', cost_usd: '5.00', per_provider: {} }],
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
  });

  // CF-436 tri-state cache row, still in force for single-provider mode.
  it('hides the cache row entirely when both create and read are 0', () => {
    const data = makeData({
      'claude-code': entry({
        total_input_tokens: 1000,
        total_cache_creation_tokens: 0,
        total_cache_read_tokens: 0,
        total_cost_usd: '5.00',
      }),
    });
    // Top-level totals must also be zero for the tri-state logic to see zero.
    data.total_cache_creation_tokens = 0;
    data.total_cache_read_tokens = 0;
    render(<TrendsTokensCard data={data} />);
    expect(screen.queryByText(/^Cache/)).not.toBeInTheDocument();
  });

  it('collapses to "Cache Read" when create is 0 and read > 0 (Codex-only window)', () => {
    const data = makeData({
      codex: entry({
        total_input_tokens: 800,
        total_cache_creation_tokens: 0,
        total_cache_read_tokens: 120,
        total_cost_usd: '4.25',
      }),
    });
    data.total_cache_creation_tokens = 0;
    data.total_cache_read_tokens = 120;
    render(<TrendsTokensCard data={data} />);
    expect(screen.getByText('Cache Read')).toBeInTheDocument();
    expect(screen.queryByText('Cache (Create / Read)')).not.toBeInTheDocument();
  });
});

describe('TrendsTokensCard multi-provider sections', () => {
  it('renders a top-level "Total Cost" row plus one section per provider, no table', () => {
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
      { total_cost_usd: '129.25' },
    );
    render(<TrendsTokensCard data={data} />);

    // No table in the new layout.
    expect(screen.queryByRole('table')).not.toBeInTheDocument();

    // Top-level Total Cost is present and shows the rolled-up dollar amount.
    const totalCostRow = screen.getByText('Total Cost').closest('div');
    expect(totalCostRow?.textContent).toMatch(/\$129\.25/);

    // One <section> per provider, headed by the canonical provider label.
    const claudeHeader = screen.getByText('Claude Code');
    const codexHeader = screen.getByText('Codex');
    expect(claudeHeader.tagName.toLowerCase()).toBe('header');
    expect(codexHeader.tagName.toLowerCase()).toBe('header');
  });

  it("each provider section contains its own Cost / Total Tokens / Input / Output rows with that provider's numbers", () => {
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
      { total_cost_usd: '129.25' },
    );
    render(<TrendsTokensCard data={data} />);

    const claudeSection = screen.getByText('Claude Code').closest('section')!;
    expect(claudeSection).toBeTruthy();
    expect(claudeSection.textContent).toMatch(/Cost/);
    expect(claudeSection.textContent).toMatch(/\$125\.00/);
    expect(claudeSection.textContent).toMatch(/Total Tokens/);
    // 5M input + 2M output → 7.0M total
    expect(within(claudeSection).getByText('7.0M')).toBeInTheDocument();
    expect(claudeSection.textContent).toMatch(/Input \/ Output/);
    expect(claudeSection.textContent).toMatch(/Cache \(Create \/ Read\)/);

    const codexSection = screen.getByText('Codex').closest('section')!;
    expect(codexSection.textContent).toMatch(/\$4\.25/);
    // 800K input + 150K output → 950.0k total (formatTokenCount uses lowercase k)
    expect(within(codexSection).getByText('950.0k')).toBeInTheDocument();
  });

  it("collapses Codex's cache row to 'Cache Read' (no Create) when total_cache_creation_tokens === 0", () => {
    const data = makeData({
      'claude-code': entry({
        total_input_tokens: 5_000_000,
        total_cache_creation_tokens: 100_000,
        total_cache_read_tokens: 500_000,
        total_cost_usd: '125.00',
      }),
      codex: entry({
        total_input_tokens: 800_000,
        total_cache_creation_tokens: 0,
        total_cache_read_tokens: 120_000,
        total_cost_usd: '4.25',
      }),
    });
    render(<TrendsTokensCard data={data} />);

    const codexSection = screen.getByText('Codex').closest('section')!;
    expect(within(codexSection).getByText('Cache Read')).toBeInTheDocument();
    expect(within(codexSection).queryByText('Cache (Create / Read)')).not.toBeInTheDocument();

    const claudeSection = screen.getByText('Claude Code').closest('section')!;
    expect(within(claudeSection).getByText('Cache (Create / Read)')).toBeInTheDocument();
  });

  it("omits the cache row entirely for a provider with all-zero cache numbers", () => {
    const data = makeData({
      'claude-code': entry({ total_input_tokens: 5_000_000, total_cost_usd: '125.00' }),
      codex: entry({
        total_input_tokens: 800_000,
        total_cache_creation_tokens: 0,
        total_cache_read_tokens: 0,
        total_cost_usd: '4.25',
      }),
    });
    render(<TrendsTokensCard data={data} />);
    const codexSection = screen.getByText('Codex').closest('section')!;
    expect(within(codexSection).queryByText(/^Cache/)).not.toBeInTheDocument();
  });

  it('sorts provider sections alphabetically by canonical id', () => {
    // Insert codex BEFORE claude-code to verify sort, not insertion order.
    const data = makeData({
      codex: entry({ total_input_tokens: 800_000, total_cost_usd: '4.25' }),
      'claude-code': entry({ total_input_tokens: 5_000_000, total_cost_usd: '125.00' }),
    });
    const { container } = render(<TrendsTokensCard data={data} />);
    const sections = Array.from(container.querySelectorAll('section'));
    expect(sections.length).toBe(2);
    expect(sections[0]?.textContent).toMatch(/^Claude Code/);
    expect(sections[1]?.textContent).toMatch(/^Codex/);
  });

  it('renders an unknown provider id verbatim as the section header (no icon, no error)', () => {
    const data = makeData({
      'claude-code': entry({ total_input_tokens: 1000, total_cost_usd: '5.00' }),
      gemini: entry({ total_input_tokens: 200, total_cost_usd: '0.80' }),
    });
    render(<TrendsTokensCard data={data} />);
    expect(screen.getByText('gemini')).toBeInTheDocument();
  });

  it('renders provider sections even when a present provider has all-zero data (LEFT JOIN unmeasured case)', () => {
    const data = makeData({
      'claude-code': entry({ total_input_tokens: 5_000_000, total_cost_usd: '125.00' }),
      codex: entry(),
    });
    render(<TrendsTokensCard data={data} />);
    expect(screen.getByText('Codex')).toBeInTheDocument();
  });

  it('keeps the daily-cost chart below the per-provider sections in multi-provider mode', () => {
    const data = makeData(
      {
        'claude-code': entry({ total_input_tokens: 5_000_000, total_cost_usd: '125.00' }),
        codex: entry({ total_input_tokens: 800_000, total_cost_usd: '4.25' }),
      },
      {
        daily_costs: [
          { date: '2025-01-01', cost_usd: '5.00', per_provider: { 'claude-code': '4.50', codex: '0.50' } },
          { date: '2025-01-02', cost_usd: '6.50', per_provider: { 'claude-code': '6.00', codex: '0.50' } },
        ],
      },
    );
    render(<TrendsTokensCard data={data} />);
    expect(screen.getByText('Daily Cost')).toBeInTheDocument();
  });
});
