import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import { AgentsAndSkillsCard } from './AgentsAndSkillsCard';
import type { AgentsAndSkillsCardData } from '@/schemas/api';

function makeData(overrides: Partial<AgentsAndSkillsCardData> = {}): AgentsAndSkillsCardData {
  return {
    agent_invocations: 3,
    skill_invocations: 2,
    agent_stats: { reviewer: { success: 3, errors: 0 } },
    skill_stats: { 'add-card': { success: 2, errors: 0 } },
    ...overrides,
  };
}

describe('AgentsAndSkillsCard', () => {
  it('returns null when total invocations is 0', () => {
    const { container } = render(
      <AgentsAndSkillsCard
        data={makeData({ agent_invocations: 0, skill_invocations: 0 })}
        loading={false}
      />
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('renders loading state with title only when loading and no data', () => {
    const { getByText } = render(<AgentsAndSkillsCard data={null} loading={true} />);
    expect(getByText('Agents and Skills')).toBeInTheDocument();
    expect(getByText('Loading...')).toBeInTheDocument();
  });

  it('renders CardError when error and no data', () => {
    const { getByText } = render(
      <AgentsAndSkillsCard data={null} loading={false} error="boom" />
    );
    expect(getByText('Agents and Skills')).toBeInTheDocument();
    expect(getByText(/Failed to compute: boom/)).toBeInTheDocument();
  });

  it('continues to render data when error is present but data is also present', () => {
    const { getByText, queryByText } = render(
      <AgentsAndSkillsCard data={makeData()} loading={false} error="stale" />
    );
    expect(getByText('Agent invocations')).toBeInTheDocument();
    expect(queryByText(/Failed to compute/)).toBeNull();
  });

  it('renders both invocation stat rows and Agents/Skills legend when chart data present', () => {
    const { getByText } = render(<AgentsAndSkillsCard data={makeData()} loading={false} />);
    expect(getByText('Agent invocations')).toBeInTheDocument();
    expect(getByText('Skill invocations')).toBeInTheDocument();
    expect(getByText('Agents')).toBeInTheDocument();
    expect(getByText('Skills')).toBeInTheDocument();
  });

  it('renders recharts stub container when chart data present', () => {
    const { getAllByTestId } = render(
      <AgentsAndSkillsCard data={makeData()} loading={false} />
    );
    expect(getAllByTestId('recharts-stub').length).toBeGreaterThan(0);
  });
});
