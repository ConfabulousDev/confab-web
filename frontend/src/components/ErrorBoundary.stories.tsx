import type { Meta, StoryObj } from '@storybook/react-vite';
import ErrorBoundary from './ErrorBoundary';

// A child that throws on render so the story shows the fallback card
// (including the CF-571 "Report an issue" link).
function Boom(): never {
  throw new Error('Something went wrong while rendering this view.');
}

const meta: Meta<typeof ErrorBoundary> = {
  title: 'Components/ErrorBoundary',
  component: ErrorBoundary,
  parameters: {
    layout: 'fullscreen',
  },
};

export default meta;
type Story = StoryObj<typeof ErrorBoundary>;

// The thrown-error fallback, with retry / go-home / report-issue affordances.
export const ErrorFallback: Story = {
  render: () => (
    <ErrorBoundary>
      <Boom />
    </ErrorBoundary>
  ),
};

export const Healthy: Story = {
  render: () => (
    <ErrorBoundary>
      <div style={{ padding: 24 }}>Children render normally when nothing throws.</div>
    </ErrorBoundary>
  ),
};
