import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { WorkflowsCard } from './WorkflowsCard';
import type { WorkflowRun, WorkflowsCardData } from '@/schemas/api';

function makeRun(overrides: Partial<WorkflowRun> = {}): WorkflowRun {
  return {
    run_id: 'wf-2026-06-05_abc',
    agent_count: 3,
    input_tokens: 1000,
    output_tokens: 500,
    cache_creation: 0,
    cache_read: 0,
    estimated_usd: '0.42',
    succeeded_agents: 3,
    has_journal: true,
    duration_ms: 90000,
    ...overrides,
  };
}

function renderCard(data: WorkflowsCardData | null, loading = false) {
  return render(<WorkflowsCard data={data} loading={loading} />);
}

describe('WorkflowsCard', () => {
  it('renders one labelled row per run with its agent count', () => {
    renderCard({
      runs: [makeRun({ run_id: 'run-a', agent_count: 2 }), makeRun({ run_id: 'run-b', agent_count: 5 })],
    });

    expect(screen.getByText('Run 1')).toBeInTheDocument();
    expect(screen.getByText('Run 2')).toBeInTheDocument();
    expect(screen.getByText(/2 agents/)).toBeInTheDocument();
    expect(screen.getByText(/5 agents/)).toBeInTheDocument();
  });

  it('singularizes the agent label for a one-agent run', () => {
    renderCard({ runs: [makeRun({ agent_count: 1 })] });
    expect(screen.getByText(/1 agent$/)).toBeInTheDocument();
  });

  it('exposes the opaque runId as a hover title', () => {
    renderCard({ runs: [makeRun({ run_id: 'wf-xyz' })] });
    expect(screen.getByTitle('wf-xyz')).toBeInTheDocument();
  });

  it('shows succeeded/total completion when the run has a journal', () => {
    renderCard({ runs: [makeRun({ agent_count: 5, succeeded_agents: 4, has_journal: true })] });
    expect(screen.getByText('4/5 completed')).toBeInTheDocument();
  });

  it('omits the completion count when the run has no journal', () => {
    renderCard({ runs: [makeRun({ agent_count: 5, succeeded_agents: 0, has_journal: false })] });
    expect(screen.queryByText(/completed/)).not.toBeInTheDocument();
  });

  it('renders the per-run cost', () => {
    renderCard({ runs: [makeRun({ estimated_usd: '1.50' })] });
    expect(screen.getByText('$1.50')).toBeInTheDocument();
  });

  it('hides duration when the span is zero', () => {
    const { container } = renderCard({ runs: [makeRun({ duration_ms: 0 })] });
    // Duration is the only place an "m"/"s" duration string would appear.
    expect(container.textContent).not.toMatch(/\d+m\b/);
  });

  it('renders nothing when there are no runs', () => {
    const { container } = renderCard({ runs: [] });
    expect(container).toBeEmptyDOMElement();
  });

  it('renders nothing when data is null and not loading', () => {
    const { container } = renderCard(null);
    expect(container).toBeEmptyDOMElement();
  });

  it('shows a loading state while data is pending', () => {
    renderCard(null, true);
    expect(screen.getByText('Loading...')).toBeInTheDocument();
  });
});
