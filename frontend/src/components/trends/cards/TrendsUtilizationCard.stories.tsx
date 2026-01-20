import type { Meta, StoryObj } from '@storybook/react-vite';
import { TrendsUtilizationCard } from './TrendsUtilizationCard';

const meta: Meta<typeof TrendsUtilizationCard> = {
  title: 'Trends/Cards/TrendsUtilizationCard',
  component: TrendsUtilizationCard,
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
type Story = StoryObj<typeof TrendsUtilizationCard>;

export const Default: Story = {
  args: {
    data: {
      daily_utilization: [
        { date: '2024-01-08', utilization_pct: 45.2 },
        { date: '2024-01-09', utilization_pct: 52.8 },
        { date: '2024-01-10', utilization_pct: 38.5 },
        { date: '2024-01-11', utilization_pct: 61.3 },
        { date: '2024-01-12', utilization_pct: 55.0 },
        { date: '2024-01-13', utilization_pct: 42.1 },
        { date: '2024-01-14', utilization_pct: 48.7 },
      ],
    },
  },
};

export const HighUtilization: Story = {
  args: {
    data: {
      daily_utilization: [
        { date: '2024-01-08', utilization_pct: 75.2 },
        { date: '2024-01-09', utilization_pct: 82.8 },
        { date: '2024-01-10', utilization_pct: 78.5 },
        { date: '2024-01-11', utilization_pct: 91.3 },
        { date: '2024-01-12', utilization_pct: 85.0 },
        { date: '2024-01-13', utilization_pct: 72.1 },
        { date: '2024-01-14', utilization_pct: 88.7 },
      ],
    },
  },
};

export const WithMissingDays: Story = {
  args: {
    data: {
      daily_utilization: [
        { date: '2024-01-08', utilization_pct: 45.2 },
        { date: '2024-01-09', utilization_pct: null },
        { date: '2024-01-10', utilization_pct: 38.5 },
        { date: '2024-01-11', utilization_pct: null },
        { date: '2024-01-12', utilization_pct: null },
        { date: '2024-01-13', utilization_pct: 42.1 },
        { date: '2024-01-14', utilization_pct: 48.7 },
      ],
    },
  },
};

export const SingleDay: Story = {
  args: {
    data: {
      daily_utilization: [
        { date: '2024-01-14', utilization_pct: 65.0 },
      ],
    },
  },
};

export const NullData: Story = {
  args: {
    data: null,
  },
};
