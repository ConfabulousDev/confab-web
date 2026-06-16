import type { Meta, StoryObj } from '@storybook/react-vite';
import {
  AdminUnpricedModelsPageContent,
  type AdminUnpricedModelsPageContentProps,
} from './AdminUnpricedModelsPage';

const baseProps: AdminUnpricedModelsPageContentProps = {
  data: { models: [] },
  isLoading: false,
  error: null,
};

const meta: Meta<typeof AdminUnpricedModelsPageContent> = {
  title: 'Pages/Admin/AdminUnpricedModelsPage',
  component: AdminUnpricedModelsPageContent,
  parameters: { layout: 'padded' },
  decorators: [
    (Story) => (
      <div style={{ maxWidth: '1200px', margin: '0 auto' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof AdminUnpricedModelsPageContent>;

export const Empty: Story = { args: baseProps };

export const Loading: Story = {
  args: { ...baseProps, data: null, isLoading: true },
};

export const WithRows: Story = {
  args: {
    ...baseProps,
    data: {
      models: [
        { provider: 'claude-code', family: 'opus-9', session_count: 42, last_seen: '2026-06-16T12:00:00Z' },
        { provider: 'codex', family: 'gpt-9', session_count: 7, last_seen: '2026-06-15T09:30:00Z' },
        { provider: 'opencode', family: 'sonnet-9', session_count: 1, last_seen: '2026-06-14T18:45:00Z' },
      ],
    },
  },
};

export const ErrorState: Story = {
  args: { ...baseProps, data: null, error: 'Failed to load unpriced models.' },
};
