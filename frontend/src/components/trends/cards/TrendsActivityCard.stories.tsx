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
    providersPresent: [],
    data: {
      total_files_read: 500,
      total_files_modified: 150,
      total_lines_added: 5000,
      total_lines_removed: 2000,
      daily_session_counts: [
        { date: '2024-01-08', session_count: 5, per_provider: {} },
        { date: '2024-01-09', session_count: 8, per_provider: {} },
        { date: '2024-01-10', session_count: 3, per_provider: {} },
        { date: '2024-01-11', session_count: 12, per_provider: {} },
        { date: '2024-01-12', session_count: 6, per_provider: {} },
        { date: '2024-01-13', session_count: 2, per_provider: {} },
        { date: '2024-01-14', session_count: 10, per_provider: {} },
      ],
    },
  },
};

export const LargeNumbers: Story = {
  args: {
    providersPresent: [],
    data: {
      total_files_read: 15000,
      total_files_modified: 3500,
      total_lines_added: 125000,
      total_lines_removed: 45000,
      daily_session_counts: [
        { date: '2024-01-01', session_count: 15, per_provider: {} },
        { date: '2024-01-02', session_count: 20, per_provider: {} },
        { date: '2024-01-03', session_count: 18, per_provider: {} },
        { date: '2024-01-04', session_count: 25, per_provider: {} },
        { date: '2024-01-05', session_count: 12, per_provider: {} },
        { date: '2024-01-06', session_count: 8, per_provider: {} },
        { date: '2024-01-07', session_count: 22, per_provider: {} },
      ],
    },
  },
};

export const SingleDay: Story = {
  args: {
    providersPresent: [],
    data: {
      total_files_read: 50,
      total_files_modified: 10,
      total_lines_added: 200,
      total_lines_removed: 50,
      daily_session_counts: [{ date: '2024-01-15', session_count: 3, per_provider: {} }],
    },
  },
};

export const NoChartData: Story = {
  args: {
    providersPresent: [],
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
    providersPresent: [],
    data: null,
  },
};

// Claude-only window: Files Read row renders without a caveat; chart stacks
// render with a single Claude-colored series.
export const ClaudeOnly: Story = {
  args: {
    providersPresent: ['claude-code'],
    data: {
      total_files_read: 432,
      total_files_modified: 128,
      total_lines_added: 4500,
      total_lines_removed: 1800,
      daily_session_counts: [
        { date: '2024-01-08', session_count: 4, per_provider: { 'claude-code': 4 } },
        { date: '2024-01-09', session_count: 6, per_provider: { 'claude-code': 6 } },
        { date: '2024-01-10', session_count: 3, per_provider: { 'claude-code': 3 } },
        { date: '2024-01-11', session_count: 9, per_provider: { 'claude-code': 9 } },
        { date: '2024-01-12', session_count: 5, per_provider: { 'claude-code': 5 } },
        { date: '2024-01-13', session_count: 2, per_provider: { 'claude-code': 2 } },
        { date: '2024-01-14', session_count: 7, per_provider: { 'claude-code': 7 } },
      ],
    },
  },
};

// Mixed Claude + Codex window: Files Read row renders with the ⓘ caveat
// (excludes Codex sessions); chart stacks Claude and Codex series.
export const Mixed: Story = {
  args: {
    providersPresent: ['claude-code', 'codex'],
    data: {
      total_files_read: 1234,
      total_files_modified: 410,
      total_lines_added: 12000,
      total_lines_removed: 5400,
      daily_session_counts: [
        { date: '2024-01-08', session_count: 5, per_provider: { 'claude-code': 4, codex: 1 } },
        { date: '2024-01-09', session_count: 8, per_provider: { 'claude-code': 5, codex: 3 } },
        { date: '2024-01-10', session_count: 3, per_provider: { 'claude-code': 2, codex: 1 } },
        { date: '2024-01-11', session_count: 12, per_provider: { 'claude-code': 9, codex: 3 } },
        { date: '2024-01-12', session_count: 7, per_provider: { 'claude-code': 4, codex: 3 } },
        { date: '2024-01-13', session_count: 4, per_provider: { 'claude-code': 2, codex: 2 } },
        { date: '2024-01-14', session_count: 10, per_provider: { 'claude-code': 6, codex: 4 } },
      ],
    },
  },
};

// Codex-only window: Files Read row is omitted entirely (Codex has no Read
// tool, so the total would always be 0). Chart shows a Codex-colored
// single-stack series.
export const CodexOnly: Story = {
  args: {
    providersPresent: ['codex'],
    data: {
      total_files_read: 0,
      total_files_modified: 156,
      total_lines_added: 3200,
      total_lines_removed: 980,
      daily_session_counts: [
        { date: '2024-01-08', session_count: 2, per_provider: { codex: 2 } },
        { date: '2024-01-09', session_count: 4, per_provider: { codex: 4 } },
        { date: '2024-01-10', session_count: 1, per_provider: { codex: 1 } },
        { date: '2024-01-11', session_count: 5, per_provider: { codex: 5 } },
        { date: '2024-01-12', session_count: 3, per_provider: { codex: 3 } },
        { date: '2024-01-13', session_count: 2, per_provider: { codex: 2 } },
        { date: '2024-01-14', session_count: 4, per_provider: { codex: 4 } },
      ],
    },
  },
};
