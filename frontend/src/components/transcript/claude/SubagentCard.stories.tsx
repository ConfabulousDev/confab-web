import type { Meta, StoryObj } from '@storybook/react-vite';
import type { ClaudeAgentInfo } from './claudeAgentIndex';
import SubagentCard from './SubagentCard';

// Fictional agent data (never real session ids).
const baseAgent: ClaudeAgentInfo = {
  agentId: 'a1b2c3d4e5f60718',
  toolUseId: 'toolu_story_1',
  resultMessageUuid: 'result-uuid-1',
  description: 'Explore the auth middleware',
  subagentType: 'Explore',
  model: 'claude-opus-5',
  status: 'running',
  fileName: 'agent-a1b2c3d4e5f60718.jsonl',
};

const rawAsync = (
  <pre style={{ margin: 0 }}>
    {'Async agent launched successfully.\nagentId: a1b2c3d4e5f60718\nThe agent is working in the background.'}
  </pre>
);

const meta: Meta<typeof SubagentCard> = {
  title: 'Transcript/Claude/SubagentCard',
  component: SubagentCard,
  parameters: { layout: 'padded' },
  args: {
    rawLabel: 'Raw result',
    children: rawAsync,
    onOpenThread: () => {},
  },
};
export default meta;

type Story = StoryObj<typeof SubagentCard>;

export const AsyncRunning: Story = {
  args: { variant: 'launch', agent: baseAgent },
};

export const ForegroundCompleted: Story = {
  args: {
    variant: 'launch',
    agent: {
      ...baseAgent,
      description: 'Find every caller of resolveRepo',
      subagentType: 'general-purpose',
      model: undefined,
      status: 'completed',
      totalDurationMs: 94_000,
      totalTokens: 48_210,
      totalToolUseCount: 17,
    },
    children: <pre style={{ margin: 0 }}>Found 4 call sites in backend/internal/db.</pre>,
  },
};

export const Error: Story = {
  args: {
    variant: 'launch',
    agent: { ...baseAgent, status: 'error', model: undefined },
    children: <pre style={{ margin: 0 }}>Agent failed: tool budget exhausted.</pre>,
  },
};

export const Finished: Story = {
  args: {
    variant: 'finished',
    agent: { ...baseAgent, status: 'completed' },
    status: 'completed',
    summary: 'Agent "Explore the auth middleware" finished',
    rawLabel: 'Raw notification',
    children: (
      <pre style={{ margin: 0 }}>
        {'<task-notification>\n<task-id>a1b2c3d4e5f60718</task-id>\n<status>completed</status>\n</task-notification>'}
      </pre>
    ),
  },
};

/** we3k: a `failed` notification renders with error styling. */
export const FinishedFailed: Story = {
  args: {
    variant: 'finished',
    agent: { ...baseAgent, status: 'failed' },
    status: 'failed',
    summary: 'Agent "Explore the auth middleware" failed',
    rawLabel: 'Raw notification',
    children: (
      <pre style={{ margin: 0 }}>
        {'<task-notification>\n<task-id>a1b2c3d4e5f60718</task-id>\n<status>failed</status>\n</task-notification>'}
      </pre>
    ),
  },
};

/** we3k: a `killed` notification reads as stopped, in neutral styling. */
export const FinishedStopped: Story = {
  args: {
    variant: 'finished',
    agent: { ...baseAgent, status: 'killed' },
    status: 'killed',
    summary: 'Agent "Explore the auth middleware" was stopped',
    rawLabel: 'Raw notification',
    children: (
      <pre style={{ margin: 0 }}>
        {'<task-notification>\n<task-id>a1b2c3d4e5f60718</task-id>\n<status>killed</status>\n</task-notification>'}
      </pre>
    ),
  },
};

export const UntitledWithoutOpenAction: Story = {
  args: {
    variant: 'launch',
    agent: { ...baseAgent, description: undefined, subagentType: undefined, model: undefined },
    onOpenThread: undefined,
  },
};
