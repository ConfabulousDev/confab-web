import type { Meta, StoryObj } from '@storybook/react-vite';
import CodexCompactedDivider from './CodexCompactedDivider';

const meta: Meta<typeof CodexCompactedDivider> = {
  title: 'Transcript/Codex/CodexCompactedDivider',
  component: CodexCompactedDivider,
};

export default meta;
type Story = StoryObj<typeof CodexCompactedDivider>;

export const SeveralMessages: Story = {
  args: {
    item: { kind: 'compacted', lineId: '0', timestamp: '2026-05-13T18:02:00Z', replacementCount: 12 },
  },
};

export const OneMessage: Story = {
  args: {
    item: { kind: 'compacted', lineId: '0', timestamp: '2026-05-13T18:02:00Z', replacementCount: 1 },
  },
};

export const Empty: Story = {
  args: {
    item: { kind: 'compacted', lineId: '0', timestamp: '2026-05-13T18:02:00Z', replacementCount: 0 },
  },
};

// CF-360: deep-link landing variant.
export const WithDeepLinkTarget: Story = {
  args: {
    item: { kind: 'compacted', lineId: '0', timestamp: '2026-05-13T18:02:00Z', replacementCount: 7 },
    sessionId: 'story-session',
    isDeepLinkTarget: true,
  },
};
