import type { Meta, StoryObj } from '@storybook/react-vite';
import { MemoryRouter } from 'react-router-dom';
import {
  AdminPrecomputeFailuresPageContent,
  type AdminPrecomputeFailuresPageContentProps,
} from './AdminPrecomputeFailuresPage';

const noop = () => {};

const baseProps: AdminPrecomputeFailuresPageContentProps = {
  data: { failures: [] },
  isLoading: false,
  error: null,
  feedback: null,
  onFeedbackClose: noop,
  onRelease: noop,
  onReleaseAll: noop,
  releasingKey: null,
  isReleasingAll: false,
};

const rows: AdminPrecomputeFailuresPageContentProps['data'] = {
  failures: [
    {
      session_id: '2a6f42e3-0000-4000-8000-000000000001',
      user_id: 1,
      external_id: '019e3bb1-e75b-7423-b373-f8b8d573b237',
      provider: 'codex',
      bucket: 'regular_cards',
      attempts: 5,
      last_error: 'parse rollout: bufio.Scanner: token too long while reading chunk 3 of rollout-2026-10-19T08-26-15.jsonl',
      quarantined_at: '2026-10-19T12:00:00Z',
    },
    {
      session_id: '3c071784-0000-4000-8000-000000000002',
      user_id: 2,
      external_id: 'c0ffee00-1234-4abc-8def-000000000002',
      provider: 'claude-code',
      bucket: 'smart_recap',
      attempts: 6,
      last_error: 'worker exited during attempt',
      quarantined_at: '2026-10-18T08:00:00Z',
    },
    {
      session_id: '4d182895-0000-4000-8000-000000000003',
      user_id: 2,
      external_id: 'cursor-session-3',
      provider: 'cursor',
      bucket: 'search_index',
      attempts: 5,
      last_error: 'get chunk: connection reset by peer',
      quarantined_at: '2026-10-17T21:30:00Z',
    },
  ],
};

const meta: Meta<typeof AdminPrecomputeFailuresPageContent> = {
  title: 'Pages/Admin/AdminPrecomputeFailuresPage',
  component: AdminPrecomputeFailuresPageContent,
  parameters: { layout: 'padded' },
  decorators: [
    (Story) => (
      <MemoryRouter>
        <div style={{ maxWidth: '1200px', margin: '0 auto' }}>
          <Story />
        </div>
      </MemoryRouter>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof AdminPrecomputeFailuresPageContent>;

export const Empty: Story = { args: baseProps };

export const Loading: Story = {
  args: { ...baseProps, data: null, isLoading: true },
};

export const WithRows: Story = {
  args: { ...baseProps, data: rows },
};

export const ReleasingRow: Story = {
  args: {
    ...baseProps,
    data: rows,
    releasingKey: '2a6f42e3-0000-4000-8000-000000000001 regular_cards',
  },
};

export const AfterRelease: Story = {
  args: {
    ...baseProps,
    data: rows,
    feedback: { type: 'success', message: 'Released 1 quarantine. The worker retries on its next cycle.' },
  },
};

export const ErrorState: Story = {
  args: { ...baseProps, data: null, error: 'Failed to load precompute failures.' },
};
