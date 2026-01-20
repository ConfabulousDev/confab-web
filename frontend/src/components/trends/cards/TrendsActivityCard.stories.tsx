import type { Meta, StoryObj } from '@storybook/react-vite';
import { TrendsActivityCard } from './TrendsActivityCard';

const meta: Meta<typeof TrendsActivityCard> = {
  title: 'Trends/Cards/TrendsActivityCard',
  component: TrendsActivityCard,
  parameters: {
    layout: 'centered',
  },
  decorators: [
    (Story) => (
      <div style={{ width: '400px' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof TrendsActivityCard>;

export const Default: Story = {
  args: {
    data: {
      total_files_read: 500,
      total_files_modified: 150,
      total_lines_added: 5000,
      total_lines_removed: 2000,
      daily_session_counts: [
        { date: '2024-01-08', session_count: 5 },
        { date: '2024-01-09', session_count: 8 },
        { date: '2024-01-10', session_count: 3 },
        { date: '2024-01-11', session_count: 12 },
        { date: '2024-01-12', session_count: 6 },
        { date: '2024-01-13', session_count: 2 },
        { date: '2024-01-14', session_count: 10 },
      ],
    },
  },
};

export const LargeNumbers: Story = {
  args: {
    data: {
      total_files_read: 15000,
      total_files_modified: 3500,
      total_lines_added: 125000,
      total_lines_removed: 45000,
      daily_session_counts: [
        { date: '2024-01-01', session_count: 15 },
        { date: '2024-01-02', session_count: 20 },
        { date: '2024-01-03', session_count: 18 },
        { date: '2024-01-04', session_count: 25 },
        { date: '2024-01-05', session_count: 12 },
        { date: '2024-01-06', session_count: 8 },
        { date: '2024-01-07', session_count: 22 },
      ],
    },
  },
};

export const SingleDay: Story = {
  args: {
    data: {
      total_files_read: 50,
      total_files_modified: 10,
      total_lines_added: 200,
      total_lines_removed: 50,
      daily_session_counts: [{ date: '2024-01-15', session_count: 3 }],
    },
  },
};

export const NoChartData: Story = {
  args: {
    data: {
      total_files_read: 10,
      total_files_modified: 2,
      total_lines_added: 50,
      total_lines_removed: 10,
      daily_session_counts: [],
    },
  },
};

export const NullData: Story = {
  args: {
    data: null,
  },
};
