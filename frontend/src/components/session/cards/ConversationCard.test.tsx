import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import { ConversationCard } from './ConversationCard';
import type { ConversationCardData } from '@/schemas/api';

function makeData(overrides: Partial<ConversationCardData> = {}): ConversationCardData {
  return {
    user_turns: 5,
    assistant_turns: 5,
    avg_assistant_turn_ms: 90_500,
    avg_user_thinking_ms: 4_000,
    total_assistant_duration_ms: 450_000,
    total_user_duration_ms: 20_000,
    assistant_utilization_pct: 95.6,
    ...overrides,
  };
}

describe('ConversationCard', () => {
  it('renders all stat rows when all fields are populated', () => {
    const { getByText } = render(<ConversationCard data={makeData()} loading={false} />);
    expect(getByText('Claude utilization')).toBeInTheDocument();
    expect(getByText('Total Claude time')).toBeInTheDocument();
    expect(getByText('Total user time')).toBeInTheDocument();
    expect(getByText('User prompts')).toBeInTheDocument();
    expect(getByText('Avg Claude time')).toBeInTheDocument();
    expect(getByText('Avg user time')).toBeInTheDocument();
  });

  it.each([
    ['assistant_utilization_pct', 'Claude utilization'],
    ['total_assistant_duration_ms', 'Total Claude time'],
    ['total_user_duration_ms', 'Total user time'],
    ['avg_assistant_turn_ms', 'Avg Claude time'],
    ['avg_user_thinking_ms', 'Avg user time'],
  ] as const)('hides %s row when value is null', (field, label) => {
    const { queryByText } = render(
      <ConversationCard data={makeData({ [field]: null })} loading={false} />
    );
    expect(queryByText(label)).toBeNull();
  });

  it.each([
    [3_700_000, '1h 1m'],
    [3_600_000, '1h'],
    [90_500, '1m 30s'],
    [5_000, '5s'],
    [500, '500ms'],
  ])('formats duration ms=%i as "%s"', (ms, expected) => {
    const { getByText } = render(
      <ConversationCard
        data={makeData({ avg_assistant_turn_ms: ms })}
        loading={false}
      />
    );
    expect(getByText(expected)).toBeInTheDocument();
  });

  it('renders utilization rounded with %', () => {
    const { getByText } = render(
      <ConversationCard
        data={makeData({ assistant_utilization_pct: 95.6 })}
        loading={false}
      />
    );
    expect(getByText('96%')).toBeInTheDocument();
  });

  it('renders loading state', () => {
    const { getByText } = render(<ConversationCard data={null} loading={true} />);
    expect(getByText('Conversation')).toBeInTheDocument();
    expect(getByText('Loading...')).toBeInTheDocument();
  });

  it('renders CardError', () => {
    const { getByText } = render(
      <ConversationCard data={null} loading={false} error="bork" />
    );
    expect(getByText(/Failed to compute: bork/)).toBeInTheDocument();
  });
});
