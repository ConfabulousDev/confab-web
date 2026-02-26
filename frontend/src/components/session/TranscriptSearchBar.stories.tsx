import { createRef } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import TranscriptSearchBar from './TranscriptSearchBar';

const meta = {
  title: 'Session/TranscriptSearchBar',
  component: TranscriptSearchBar,
  parameters: {
    layout: 'padded',
  },
  decorators: [
    (Story) => (
      <div style={{ position: 'relative', height: 120 }}>
        <Story />
      </div>
    ),
  ],
  args: {
    inputRef: createRef<HTMLInputElement>(),
    onQueryChange: () => {},
    onNext: () => {},
    onPrev: () => {},
    onClose: () => {},
  },
} satisfies Meta<typeof TranscriptSearchBar>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default empty state — no query entered yet.
 */
export const Default: Story = {
  args: {
    query: '',
    currentMatch: 0,
    totalMatches: 0,
  },
};

/**
 * Search with results — navigating through matches.
 */
export const WithResults: Story = {
  args: {
    query: 'analytics',
    currentMatch: 3,
    totalMatches: 12,
  },
};

/**
 * Search query with no matching results.
 */
export const NoResults: Story = {
  args: {
    query: 'xyznonexistent',
    currentMatch: 0,
    totalMatches: 0,
  },
};
