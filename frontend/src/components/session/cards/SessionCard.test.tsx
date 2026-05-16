import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import { SessionCard } from './SessionCard';
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
    const { getByText } = render(<SessionCard data={makeData()} loading={false} />);
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
      <SessionCard data={makeData({ duration_ms: ms })} loading={false} />
    );
    expect(getByText(expected)).toBeInTheDocument();
  });

  it.each([
    ['claude-sonnet-4', 'Sonnet 4'],
    ['claude-opus-4-5-20251101', 'Opus 4.5'],
    ['gpt-5-codex', 'gpt-5-codex'],
  ])('formats model name %s as "%s"', (model, expected) => {
    const { getByText } = render(
      <SessionCard data={makeData({ models_used: [model] })} loading={false} />
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
      />
    );
    expect(getByText('Models')).toBeInTheDocument();
    expect(getByText('Sonnet 4, Opus 4.5')).toBeInTheDocument();
  });

  it('omits Duration row when duration_ms is null', () => {
    const { queryByText } = render(
      <SessionCard data={makeData({ duration_ms: null })} loading={false} />
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
      />
    );
    expect(queryByText('Avg time (auto)')).toBeNull();
  });

  it('renders loading state', () => {
    const { getByText } = render(<SessionCard data={null} loading={true} />);
    expect(getByText('Session')).toBeInTheDocument();
    expect(getByText('Loading...')).toBeInTheDocument();
  });

  it('renders CardError', () => {
    const { getByText } = render(
      <SessionCard data={null} loading={false} error="nope" />
    );
    expect(getByText(/Failed to compute: nope/)).toBeInTheDocument();
  });
});
