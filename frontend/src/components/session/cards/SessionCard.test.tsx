import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import { SessionCard } from './SessionCard';
import { prepareBreakdownData } from './sessionCardBreakdown';
import type { SessionCardData } from '@/schemas/api';

function makeData(overrides: Partial<SessionCardData> = {}): SessionCardData {
  return {
    total_messages: 50,
    user_messages: 20,
    assistant_messages: 30,
    human_prompts: 10,
    tool_results: 10,
    text_responses: 15,
    tool_calls: 10,
    thinking_blocks: 5,
    duration_ms: 90_000,
    models_used: ['claude-sonnet-4-20241022'],
    compaction_auto: 0,
    compaction_manual: 0,
    compaction_avg_time_ms: null,
    ...overrides,
  };
}

describe('SessionCard', () => {
  it('renders Duration / Models / Messages stat rows', () => {
    const { getByText } = render(
      <SessionCard data={makeData()} loading={false} provider="claude-code" />
    );
    expect(getByText('Duration')).toBeInTheDocument();
    expect(getByText('Model')).toBeInTheDocument();
    expect(getByText('Messages')).toBeInTheDocument();
    expect(getByText('50 (20/30)')).toBeInTheDocument();
  });

  it.each([
    [5_000, '5s'],
    [90_000, '1m'],
    [3_600_000, '1h'],
    [3_700_000, '1h 1m'],
  ])('formats duration_ms=%i as "%s"', (ms, expected) => {
    const { getByText } = render(
      <SessionCard data={makeData({ duration_ms: ms })} loading={false} provider="claude-code" />
    );
    expect(getByText(expected)).toBeInTheDocument();
  });

  it('renders "Models" (plural) and comma-joined list when multiple models', () => {
    const { getByText } = render(
      <SessionCard
        data={makeData({
          models_used: ['claude-sonnet-4', 'claude-opus-4-5-20251101'],
        })}
        loading={false}
        provider="claude-code"
      />
    );
    expect(getByText('Models')).toBeInTheDocument();
    expect(getByText('Sonnet 4, Opus 4.5')).toBeInTheDocument();
  });

  it('omits Duration row when duration_ms is null', () => {
    const { queryByText } = render(
      <SessionCard data={makeData({ duration_ms: null })} loading={false} provider="claude-code" />
    );
    expect(queryByText('Duration')).toBeNull();
  });

  it('renders Compactions row when auto+manual > 0', () => {
    const { getByText } = render(
      <SessionCard
        data={makeData({
          compaction_auto: 2,
          compaction_manual: 1,
          compaction_avg_time_ms: 5_000,
        })}
        loading={false}
        provider="claude-code"
      />
    );
    expect(getByText('Compactions')).toBeInTheDocument();
    expect(getByText('3 (1/2)')).toBeInTheDocument();
    expect(getByText('Avg time (auto)')).toBeInTheDocument();
  });

  it('omits Avg time row when compaction_avg_time_ms is null', () => {
    const { queryByText } = render(
      <SessionCard
        data={makeData({
          compaction_auto: 2,
          compaction_manual: 1,
          compaction_avg_time_ms: null,
        })}
        loading={false}
        provider="claude-code"
      />
    );
    expect(queryByText('Avg time (auto)')).toBeNull();
  });

  it('renders loading state', () => {
    const { getByText } = render(
      <SessionCard data={null} loading={true} provider="claude-code" />
    );
    expect(getByText('Session')).toBeInTheDocument();
    expect(getByText('Loading...')).toBeInTheDocument();
  });

  it('renders CardError', () => {
    const { getByText } = render(
      <SessionCard data={null} loading={false} error="nope" provider="claude-code" />
    );
    expect(getByText(/Failed to compute: nope/)).toBeInTheDocument();
  });

  // ─────────────────────────────────────────────────────────────────────
  // CF-437: provider-aware bar labels, hidden Tool results bar, OpenAI
  // model formatter, provider-aware Messages tooltip.
  // ─────────────────────────────────────────────────────────────────────

  describe('formatModelName (CF-437)', () => {
    it.each([
      // Claude — existing behavior
      ['claude-sonnet-4', 'Sonnet 4'],
      ['claude-opus-4-5-20251101', 'Opus 4.5'],
      // OpenAI GPT family — Title Case
      ['gpt-5', 'GPT-5'],
      ['gpt-5-codex', 'GPT-5 Codex'],
      ['gpt-5-mini', 'GPT-5 Mini'],
      ['gpt-5-codex-2025-01-01', 'GPT-5 Codex'],
      ['gpt-4o', 'GPT-4o'],
      ['gpt-4o-mini', 'GPT-4o Mini'],
      // OpenAI o-series — lowercase preserved
      ['o3-mini', 'o3-mini'],
      ['o4-mini-high', 'o4-mini-high'],
      // Date stripping is strict trailing `-YYYY-MM-DD` only
      ['gpt-5-2025-01-01', 'GPT-5'],
      // Unknown family — passthrough
      ['mistral-7b', 'mistral-7b'],
    ])('formats %s as "%s"', (model, expected) => {
      const { getByText } = render(
        <SessionCard data={makeData({ models_used: [model] })} loading={false} provider="claude-code" />
      );
      expect(getByText(expected)).toBeInTheDocument();
    });
  });

  // The bar chart is rendered by Recharts inside a ResponsiveContainer
  // whose dimensions jsdom doesn't measure, so the rendered SVG axis labels
  // aren't queryable. We test the bar-label contract via prepareBreakdownData
  // directly — that function is the source of truth Recharts consumes.
  describe('prepareBreakdownData reasoning bar label is provider-aware (CF-437)', () => {
    it('uses "Thinking" / "Thinking blocks" for Claude', () => {
      const entries = prepareBreakdownData(makeData({ thinking_blocks: 3 }), 'claude-code');
      const reasoning = entries.find((e) => e.value === 3);
      expect(reasoning?.name).toBe('Thinking');
      expect(reasoning?.fullName).toBe('Thinking blocks');
    });

    it('uses "Reasoning" / "Reasoning steps" for Codex', () => {
      const entries = prepareBreakdownData(makeData({ thinking_blocks: 3 }), 'codex');
      const reasoning = entries.find((e) => e.value === 3);
      expect(reasoning?.name).toBe('Reasoning');
      expect(reasoning?.fullName).toBe('Reasoning steps');
    });
  });

  describe('prepareBreakdownData tool-results hide rule for Codex (CF-437)', () => {
    it('hides the Tool results bar for Codex when tool_calls == tool_results', () => {
      const entries = prepareBreakdownData(
        makeData({ tool_calls: 6, tool_results: 6 }),
        'codex',
      );
      expect(entries.find((e) => e.name === 'Tool calls')).toBeDefined();
      expect(entries.find((e) => e.fullName === 'Tool results')).toBeUndefined();
    });

    it('shows the Tool results bar for Codex when tool_calls != tool_results', () => {
      const entries = prepareBreakdownData(
        makeData({ tool_calls: 6, tool_results: 3 }),
        'codex',
      );
      expect(entries.find((e) => e.name === 'Tool calls')).toBeDefined();
      expect(entries.find((e) => e.fullName === 'Tool results')).toBeDefined();
    });

    it('shows both bars for Claude even when tool_calls == tool_results', () => {
      const entries = prepareBreakdownData(
        makeData({ tool_calls: 6, tool_results: 6 }),
        'claude-code',
      );
      expect(entries.find((e) => e.name === 'Tool calls')).toBeDefined();
      expect(entries.find((e) => e.fullName === 'Tool results')).toBeDefined();
    });
  });

  describe('Messages tooltip is provider-aware (CF-437)', () => {
    function messagesTooltip(getByText: (t: string) => HTMLElement): string | null {
      const row = getByText('Messages').closest('[title]');
      return row?.getAttribute('title') ?? null;
    }

    it('mentions "tool results" in tooltip for Claude', () => {
      const { getByText } = render(
        <SessionCard data={makeData()} loading={false} provider="claude-code" />
      );
      const tip = messagesTooltip(getByText);
      expect(tip).toMatch(/tool results/);
    });

    it('says tool outputs are counted separately for Codex', () => {
      const { getByText } = render(
        <SessionCard data={makeData()} loading={false} provider="codex" />
      );
      const tip = messagesTooltip(getByText);
      expect(tip).toMatch(/tool outputs counted separately/);
      expect(tip).not.toMatch(/\(human prompts \+ tool results\)/);
    });
  });
});
