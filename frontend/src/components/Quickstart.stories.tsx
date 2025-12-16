import type { Meta, StoryObj } from '@storybook/react-vite';
import Quickstart from './Quickstart';

const meta: Meta<typeof Quickstart> = {
  title: 'Components/Quickstart',
  component: Quickstart,
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
};

export default meta;
type Story = StoryObj<typeof Quickstart>;

export const Default: Story = {};
