import type { Meta, StoryObj } from '@storybook/react-vite';
import CodexRowActions from './CodexRowActions';

const meta: Meta<typeof CodexRowActions> = {
  title: 'Transcript/Codex/CodexRowActions',
  component: CodexRowActions,
};

export default meta;
type Story = StoryObj<typeof CodexRowActions>;

// Full chrome: skip prev/next + copy text + copy link. The default args
// match what `CodexUserMessage` passes in production.
export const Default: Story = {
  args: {
    sessionId: 'demo-session',
    lineId: '42',
    copyText: 'the message body that would land in the clipboard',
    onSkipToNext: () => undefined,
    onSkipToPrevious: () => undefined,
    kindLabel: 'user prompt',
  },
};

// Divider variant: only copy-link is rendered (no copy-text, no skip nav).
export const CopyLinkOnly: Story = {
  args: {
    sessionId: 'demo-session',
    lineId: '7',
    kindLabel: 'turn separator',
  },
};

// First-of-kind row: prev-skip hidden.
export const NoPrevSkip: Story = {
  args: {
    sessionId: 'demo-session',
    lineId: '0',
    copyText: 'the first user prompt of the session',
    onSkipToNext: () => undefined,
    kindLabel: 'user prompt',
  },
};

// Last-of-kind row: next-skip hidden.
export const NoNextSkip: Story = {
  args: {
    sessionId: 'demo-session',
    lineId: '99',
    copyText: 'the last user prompt of the session',
    onSkipToPrevious: () => undefined,
    kindLabel: 'user prompt',
  },
};

// Web-search row variant — no copyText (no queries) so copy-text is hidden.
export const NoCopyText: Story = {
  args: {
    sessionId: 'demo-session',
    lineId: '13',
    onSkipToNext: () => undefined,
    onSkipToPrevious: () => undefined,
    kindLabel: 'web search',
  },
};
