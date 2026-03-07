import type { Meta, StoryObj } from '@storybook/react-vite';
import ServerError from './ServerError';

const meta: Meta<typeof ServerError> = {
  title: 'Components/ServerError',
  component: ServerError,
  parameters: {
    layout: 'fullscreen',
  },
};

export default meta;
type Story = StoryObj<typeof ServerError>;

export const Default: Story = {
  args: {
    message: null,
    onRetry: () => new Promise((_, reject) => setTimeout(() => reject(new Error('still down')), 1000)),
  },
};

export const WithMessage: Story = {
  args: {
    message: 'Request failed: Service Unavailable',
    onRetry: () => new Promise((_, reject) => setTimeout(() => reject(new Error('still down')), 1000)),
  },
};

export const RetrySucceeds: Story = {
  args: {
    message: null,
    onRetry: () => new Promise((resolve) => setTimeout(resolve, 500)),
  },
};
