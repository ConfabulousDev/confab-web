import type { Meta, StoryObj } from '@storybook/react-vite';
import RowActions from './RowActions';

const meta: Meta<typeof RowActions> = {
  title: 'Transcript/RowActions',
  component: RowActions,
};

export default meta;
type Story = StoryObj<typeof RowActions>;

// Full chrome: skip prev/next + copy text + copy link. The default args
// match what a Codex user-message row passes in production (deepLinkMsg is
// the row's ISO 8601 timestamp).
export const Default: Story = {
  args: {
    sessionId: 'demo-session',
    deepLinkMsg: '2026-05-13T18:00:00Z',
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
    deepLinkMsg: '2026-05-13T18:00:07Z',
    kindLabel: 'turn separator',
  },
};

// First-of-kind row: prev-skip hidden.
export const NoPrevSkip: Story = {
  args: {
    sessionId: 'demo-session',
    deepLinkMsg: '2026-05-13T18:00:00Z',
    copyText: 'the first user prompt of the session',
    onSkipToNext: () => undefined,
    kindLabel: 'user prompt',
  },
};

// Last-of-kind row: next-skip hidden.
export const NoNextSkip: Story = {
  args: {
    sessionId: 'demo-session',
    deepLinkMsg: '2026-05-13T18:01:39Z',
    copyText: 'the last user prompt of the session',
    onSkipToPrevious: () => undefined,
    kindLabel: 'user prompt',
  },
};

// Web-search row variant — no copyText (no queries) so copy-text is hidden.
export const NoCopyText: Story = {
  args: {
    sessionId: 'demo-session',
    deepLinkMsg: '2026-05-13T18:00:13Z',
    onSkipToNext: () => undefined,
    onSkipToPrevious: () => undefined,
    kindLabel: 'web search',
  },
};

// Cursor variant: deepLinkMsg is the synthetic stable item id (here a tool
// row's "lineIndex-blockIndex" id), not a timestamp. Copy-link encodes it
// into the same ?msg= URL shape.
export const CursorToolRow: Story = {
  args: {
    sessionId: 'demo-session',
    deepLinkMsg: '12-3',
    copyText: 'gh api dependabot/alerts',
    onSkipToNext: () => undefined,
    onSkipToPrevious: () => undefined,
    kindLabel: 'tool call',
  },
};
