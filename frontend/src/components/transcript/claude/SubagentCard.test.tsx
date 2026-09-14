// et0r: subagent launch / finished card with an "Open transcript" action.

import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ClaudeAgentInfo } from './claudeAgentIndex';
import SubagentCard from './SubagentCard';

function agent(overrides: Partial<ClaudeAgentInfo> = {}): ClaudeAgentInfo {
  return {
    agentId: 'a1',
    toolUseId: 't1',
    resultMessageUuid: 'r1',
    description: 'Explore the codebase',
    subagentType: 'Explore',
    model: 'claude-opus-5',
    status: 'running',
    fileName: 'agent-a1.jsonl',
    ...overrides,
  };
}

describe('SubagentCard', () => {
  it('launch variant shows description, subagent type, model and status', () => {
    render(<SubagentCard variant="launch" agent={agent()} rawLabel="Raw result">raw</SubagentCard>);
    expect(screen.getByText('Explore the codebase')).toBeInTheDocument();
    expect(screen.getByText('Explore')).toBeInTheDocument();
    expect(screen.getByText('claude-opus-5')).toBeInTheDocument();
    expect(screen.getByText('running')).toBeInTheDocument();
  });

  it('launch variant shows foreground stats when present', () => {
    render(
      <SubagentCard
        variant="launch"
        agent={agent({ status: 'completed', totalDurationMs: 90_000, totalTokens: 12_345, totalToolUseCount: 7 })}
        rawLabel="Raw result"
      >
        raw
      </SubagentCard>,
    );
    expect(screen.getByText('1m 30s')).toBeInTheDocument();
    expect(screen.getByText('12.3k tokens')).toBeInTheDocument();
    expect(screen.getByText('7 tool uses')).toBeInTheDocument();
  });

  it('falls back to the agent id when there is no description', () => {
    render(
      <SubagentCard variant="launch" agent={agent({ description: undefined, subagentType: undefined })} rawLabel="Raw result">
        raw
      </SubagentCard>,
    );
    expect(screen.getByText('a1')).toBeInTheDocument();
  });

  it('finished variant shows the summary and its own status', () => {
    render(
      <SubagentCard
        variant="finished"
        agent={agent({ status: 'killed' })}
        status="completed"
        summary='Agent "Explore the codebase" finished'
        rawLabel="Raw notification"
      >
        raw
      </SubagentCard>,
    );
    expect(screen.getByText('Subagent finished')).toBeInTheDocument();
    expect(screen.getByText('Agent "Explore the codebase" finished')).toBeInTheDocument();
    expect(screen.getByText('completed')).toBeInTheDocument();
  });

  // we3k D3: statuses are normalized, so failed/killed no longer render unstyled.
  it('shows an errored launch as failed with error styling', () => {
    render(<SubagentCard variant="launch" agent={agent({ status: 'error' })} rawLabel="Raw result">raw</SubagentCard>);
    expect(screen.getByText('failed').className).toMatch(/statusError/);
  });

  it('shows a failed notification as failed with error styling', () => {
    render(
      <SubagentCard variant="finished" agent={agent({ status: 'completed' })} status="failed" rawLabel="Raw notification">
        raw
      </SubagentCard>,
    );
    expect(screen.getByText('failed').className).toMatch(/statusError/);
  });

  it('shows a killed agent as stopped with muted styling', () => {
    render(
      <SubagentCard variant="finished" agent={agent({ status: 'completed' })} status="killed" rawLabel="Raw notification">
        raw
      </SubagentCard>,
    );
    const pill = screen.getByText('stopped');
    expect(pill.className).toMatch(/statusStopped/);
    expect(pill.className).not.toMatch(/statusError/);
  });

  it('styles running and completed statuses', () => {
    const { rerender } = render(
      <SubagentCard variant="launch" agent={agent({ status: 'running' })} rawLabel="Raw result">raw</SubagentCard>,
    );
    expect(screen.getByText('running').className).toMatch(/statusRunning/);
    rerender(<SubagentCard variant="launch" agent={agent({ status: 'completed' })} rawLabel="Raw result">raw</SubagentCard>);
    expect(screen.getByText('completed').className).toMatch(/statusCompleted/);
  });

  it('calls onOpenThread with the agent id', async () => {
    const user = userEvent.setup();
    const onOpenThread = vi.fn();
    render(
      <SubagentCard variant="launch" agent={agent()} rawLabel="Raw result" onOpenThread={onOpenThread}>
        raw
      </SubagentCard>,
    );
    await user.click(screen.getByRole('button', { name: 'Open transcript →' }));
    expect(onOpenThread).toHaveBeenCalledWith('a1');
  });

  it('hides the open button when onOpenThread is absent', () => {
    render(<SubagentCard variant="launch" agent={agent()} rawLabel="Raw result">raw</SubagentCard>);
    expect(screen.queryByRole('button', { name: 'Open transcript →' })).not.toBeInTheDocument();
  });

  it('keeps the raw content in a collapsed details element', () => {
    const { container } = render(
      <SubagentCard variant="launch" agent={agent()} rawLabel="Raw result">
        <span>RAW_BODY</span>
      </SubagentCard>,
    );
    const details = container.querySelector('details');
    expect(details).not.toBeNull();
    expect(details).not.toHaveAttribute('open');
    expect(screen.getByText('Raw result')).toBeInTheDocument();
  });

  it('auto-opens the raw details when it becomes the active search match', () => {
    const { container, rerender } = render(
      <SubagentCard variant="launch" agent={agent()} rawLabel="Raw result">raw</SubagentCard>,
    );
    rerender(
      <SubagentCard variant="launch" agent={agent()} rawLabel="Raw result" isCurrentSearchMatch>
        raw
      </SubagentCard>,
    );
    expect(container.querySelector('details')).toHaveAttribute('open');
  });
});
