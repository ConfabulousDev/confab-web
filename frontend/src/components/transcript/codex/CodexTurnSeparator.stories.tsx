import type { Meta, StoryObj } from '@storybook/react-vite';
import CodexTurnSeparator from './CodexTurnSeparator';

const meta: Meta<typeof CodexTurnSeparator> = {
  title: 'Transcript/Codex/CodexTurnSeparator',
  component: CodexTurnSeparator,
};

export default meta;
type Story = StoryObj<typeof CodexTurnSeparator>;

export const Short: Story = {
  args: {
    item: {
      kind: 'turn_separator',
      lineId: '0',
      timestamp: '2026-05-13T18:00:11Z',
      turnIndex: 1,
      durationMs: 11000,
      timeToFirstTokenMs: 1704,
    },
  },
};

export const NoTTFT: Story = {
  args: {
    item: {
      kind: 'turn_separator',
      lineId: '0',
      timestamp: '2026-05-13T18:01:06Z',
      turnIndex: 2,
      durationMs: 6000,
    },
  },
};

export const LongTurn: Story = {
  args: {
    item: {
      kind: 'turn_separator',
      lineId: '0',
      timestamp: '2026-05-13T18:05:00Z',
      turnIndex: 7,
      durationMs: 184000,
      timeToFirstTokenMs: 2500,
    },
  },
};

// CF-360: deep-link landing variant on a thin divider — pulse still visible.
export const WithDeepLinkTarget: Story = {
  args: {
    item: {
      kind: 'turn_separator',
      lineId: '0',
      timestamp: '2026-05-13T18:00:11Z',
      turnIndex: 1,
      durationMs: 11000,
      timeToFirstTokenMs: 1704,
    },
    sessionId: 'story-session',
    isDeepLinkTarget: true,
  },
};
