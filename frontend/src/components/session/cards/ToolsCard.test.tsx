import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import { ToolsCard } from './ToolsCard';
import type { ToolsCardData } from '@/schemas/api';

function makeData(overrides: Partial<ToolsCardData> = {}): ToolsCardData {
  return {
    total_calls: 5,
    tool_stats: { Bash: { success: 5, errors: 0 } },
    error_count: 0,
    ...overrides,
  };
}

describe('ToolsCard', () => {
  it('returns null when total_calls is 0', () => {
    const { container } = render(
      <ToolsCard data={makeData({ total_calls: 0, tool_stats: {} })} loading={false} />
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('renders subtitle "N calls" when error_count is 0', () => {
    const { getByText } = render(<ToolsCard data={makeData()} loading={false} />);
    expect(getByText('5 calls')).toBeInTheDocument();
  });

  it('renders subtitle with plural "errors" when error_count > 1', () => {
    const { getByText } = render(
      <ToolsCard
        data={makeData({ total_calls: 10, error_count: 3 })}
        loading={false}
      />
    );
    expect(getByText('10 calls (3 errors)')).toBeInTheDocument();
  });

  it('renders subtitle with singular "error" when error_count === 1', () => {
    const { getByText } = render(
      <ToolsCard data={makeData({ total_calls: 4, error_count: 1 })} loading={false} />
    );
    expect(getByText('4 calls (1 error)')).toBeInTheDocument();
  });

  it('renders loading state with title only', () => {
    const { getByText } = render(<ToolsCard data={null} loading={true} />);
    expect(getByText('Tools')).toBeInTheDocument();
    expect(getByText('Loading...')).toBeInTheDocument();
  });

  it('renders CardError when error and no data', () => {
    const { getByText } = render(<ToolsCard data={null} loading={false} error="nope" />);
    expect(getByText(/Failed to compute: nope/)).toBeInTheDocument();
  });

  it('renders recharts stub when tools are present', () => {
    const { getAllByTestId } = render(<ToolsCard data={makeData()} loading={false} />);
    expect(getAllByTestId('recharts-stub').length).toBeGreaterThan(0);
  });
});
