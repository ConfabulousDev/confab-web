import type { Meta, StoryObj } from '@storybook/react-vite';
import Alert from './Alert';

const meta: Meta<typeof Alert> = {
  title: 'Components/Alert',
  component: Alert,
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
  argTypes: {
    variant: {
      control: 'select',
      options: ['info', 'success', 'warning', 'error'],
    },
    onClose: { action: 'closed' },
  },
};

export default meta;
type Story = StoryObj<typeof Alert>;

export const Info: Story = {
  args: {
    variant: 'info',
    children: 'This is an informational message.',
  },
};

export const Success: Story = {
  args: {
    variant: 'success',
    children: 'Operation completed successfully!',
  },
};

export const Warning: Story = {
  args: {
    variant: 'warning',
    children: 'Please review your changes before proceeding.',
  },
};

export const Error: Story = {
  args: {
    variant: 'error',
    children: 'An error occurred while processing your request.',
  },
};

export const WithCloseButton: Story = {
  args: {
    variant: 'info',
    children: 'Click the X to dismiss this alert.',
    onClose: () => {},
  },
};

export const LongContent: Story = {
  args: {
    variant: 'warning',
    children:
      'This is a much longer alert message that demonstrates how the component handles text that wraps to multiple lines. It should remain readable and well-formatted.',
  },
};

export const WithLink: Story = {
  args: {
    variant: 'info',
    children: (
      <>
        Check out our <a href="#">documentation</a> for more details.
      </>
    ),
  },
};
