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
      <div style={{ width: '600px', background: '#fff', borderRadius: '8px' }}>
        <Story />
      </div>
    ),
  ],
  argTypes: {
    variant: {
      control: 'select',
      options: ['no-shared', 'no-matches'],
    },
  },
};

export default meta;
type Story = StoryObj<typeof SessionEmptyState>;

export const NoShared: Story = {
  args: {
    variant: 'no-shared',
  },
};

export const NoMatches: Story = {
  args: {
    variant: 'no-matches',
  },
};
