import type { Meta, StoryObj } from '@storybook/react-vite';
import { WorkflowsCard } from './WorkflowsCard';

const meta: Meta<typeof WorkflowsCard> = {
  title: 'Session/Cards/WorkflowsCard',
  component: WorkflowsCard,
  parameters: { layout: 'centered' },
  decorators: [
    (Story) => (
      <div style={{ width: '280px' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof WorkflowsCard>;

export const Default: Story = {
  args: {
    loading: false,
    data: {
      runs: [
        {
          run_id: 'wf-2026-06-05_a1b2c3',
          agent_count: 6,
          input_tokens: 42000,
          output_tokens: 18000,
          cache_creation: 12000,
          cache_read: 96000,
          estimated_usd: '1.84',
          succeeded_agents: 6,
          has_journal: true,
          duration_ms: 132000,
        },
        {
          run_id: 'wf-2026-06-05_d4e5f6',
          agent_count: 3,
          input_tokens: 15000,
          output_tokens: 7000,
          cache_creation: 4000,
          cache_read: 32000,
          estimated_usd: '0.61',
          succeeded_agents: 2,
          has_journal: true,
          duration_ms: 48000,
        },
      ],
    },
  },
};

export const SingleRun: Story = {
  args: {
    loading: false,
    data: {
      runs: [
        {
          run_id: 'wf-2026-06-05_solo',
          agent_count: 1,
          input_tokens: 8000,
          output_tokens: 3000,
          cache_creation: 0,
          cache_read: 0,
          estimated_usd: '0.09',
          succeeded_agents: 1,
          has_journal: true,
          duration_ms: 21000,
        },
      ],
    },
  },
};

// A run where some agents have no journal result line (errored or still running).
export const IncompleteAgents: Story = {
  args: {
    loading: false,
    data: {
      runs: [
        {
          run_id: 'wf-2026-06-05_partial',
          agent_count: 8,
          input_tokens: 60000,
          output_tokens: 22000,
          cache_creation: 8000,
          cache_read: 110000,
          estimated_usd: '2.40',
          succeeded_agents: 5,
          has_journal: true,
          duration_ms: 210000,
        },
      ],
    },
  },
};

// No journal uploaded for the run → completion count is omitted.
export const NoJournal: Story = {
  args: {
    loading: false,
    data: {
      runs: [
        {
          run_id: 'wf-2026-06-05_nojournal',
          agent_count: 4,
          input_tokens: 20000,
          output_tokens: 9000,
          cache_creation: 3000,
          cache_read: 40000,
          estimated_usd: '0.73',
          succeeded_agents: 0,
          has_journal: false,
          duration_ms: 0,
        },
      ],
    },
  },
};

export const Loading: Story = {
  args: {
    loading: true,
    data: null,
  },
};
