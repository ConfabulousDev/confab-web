import type { Meta, StoryObj } from '@storybook/react-vite';
import QuickstartCTA from './QuickstartCTA';

const meta: Meta<typeof QuickstartCTA> = {
  title: 'Components/QuickstartCTA',
  component: QuickstartCTA,
  parameters: {
    layout: 'padded',
  },
  decorators: [
    (Story) => {
      // Clear localStorage before each story so dismiss state doesn't persist
      localStorage.removeItem('quickstart-cta-dismissed');
      return (
        <div style={{ maxWidth: '800px' }}>
          <Story />
        </div>
      );
    },
  ],
};

export default meta;
type Story = StoryObj<typeof QuickstartCTA>;

export const Visible: Story = {
  args: {
    show: true,
  },
};

export const Hidden: Story = {
  args: {
    show: false,
  },
};
