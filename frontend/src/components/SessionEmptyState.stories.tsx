import type { Meta, StoryObj } from '@storybook/react-vite';
import SessionEmptyState from './SessionEmptyState';

const meta: Meta<typeof SessionEmptyState> = {
  title: 'Components/SessionEmptyState',
  component: SessionEmptyState,
  parameters: {
    layout: 'centered',
  },
  decorators: [
    (Story) => (
      <div style={{ width: '600px', background: 'var(--color-bg-primary)', borderRadius: '8px' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof SessionEmptyState>;

export const Default: Story = {};
