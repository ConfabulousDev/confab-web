import type { Meta, StoryObj } from '@storybook/react-vite';
import { TrendsOverviewCard } from './TrendsOverviewCard';

const meta: Meta<typeof TrendsOverviewCard> = {
  title: 'Trends/Cards/TrendsOverviewCard',
  component: TrendsOverviewCard,
  parameters: {
    layout: 'centered',
  },
  decorators: [
    (Story) => (
      <div style={{ width: '320px' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof TrendsOverviewCard>;

export const Default: Story = {
  args: {
    data: {
      session_count: 42,
      total_duration_ms: 86400000, // 24 hours
      avg_duration_ms: 2057142, // ~34 minutes
      days_covered: 7,
      total_assistant_duration_ms: 43200000, // 12 hours
      assistant_utilization_pct: 50.0,
    },
  },
};

export const SingleSession: Story = {
  args: {
    data: {
      session_count: 1,
      total_duration_ms: 3600000, // 1 hour
      avg_duration_ms: 3600000,
      days_covered: 1,
      total_assistant_duration_ms: 2700000, // 45 min
      assistant_utilization_pct: 75.0,
    },
  },
};

export const HighUsage: Story = {
  args: {
    data: {
      session_count: 250,
      total_duration_ms: 604800000, // 1 week
      avg_duration_ms: 2419200,
      days_covered: 30,
      total_assistant_duration_ms: 302400000, // ~3.5 days
      assistant_utilization_pct: 50.0,
    },
  },
};

export const NoAverageDuration: Story = {
  args: {
    data: {
      session_count: 5,
      total_duration_ms: 0,
      avg_duration_ms: null,
      days_covered: 3,
      total_assistant_duration_ms: 0,
      assistant_utilization_pct: null,
    },
  },
};

export const NullData: Story = {
  args: {
    data: null,
  },
};
