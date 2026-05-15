import type { Meta, StoryObj } from '@storybook/react-vite';
import TranscriptPaneStatus from './TranscriptPaneStatus';

const meta: Meta<typeof TranscriptPaneStatus> = {
  title: 'Session/TranscriptPaneStatus',
  component: TranscriptPaneStatus,
  parameters: { layout: 'fullscreen' },
};

export default meta;
type Story = StoryObj<typeof TranscriptPaneStatus>;

export const Loading: Story = {
  args: { loading: true, error: null },
};

export const ErrorState: Story = {
  args: { loading: false, error: 'Failed to load transcript: 404 Not Found' },
};

// Renders nothing — included so the story file documents the fall-through case.
export const Idle: Story = {
  args: { loading: false, error: null },
};
