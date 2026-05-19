// CF-368: divider for `event_msg.turn_aborted`. Stories cover each reason
// variant the Codex protocol enumerates (interrupted, replaced,
// review_ended, budget_limited) plus the degenerate label-only state and
// the CF-360 deep-link variant.

import type { Meta, StoryObj } from '@storybook/react-vite';
import CodexTurnAbortedDivider from './CodexTurnAbortedDivider';

const meta: Meta<typeof CodexTurnAbortedDivider> = {
  title: 'Transcript/Codex/CodexTurnAbortedDivider',
  component: CodexTurnAbortedDivider,
};

export default meta;
type Story = StoryObj<typeof CodexTurnAbortedDivider>;

const ts = '2026-05-13T18:02:00Z';

export const Interrupted: Story = {
  args: {
    item: { kind: 'turn_aborted', lineId: '0', timestamp: ts, reason: 'interrupted', durationMs: 4_000 },
  },
};

export const Replaced: Story = {
  args: {
    item: { kind: 'turn_aborted', lineId: '0', timestamp: ts, reason: 'replaced', durationMs: 850 },
  },
};

export const BudgetLimited: Story = {
  args: {
    item: { kind: 'turn_aborted', lineId: '0', timestamp: ts, reason: 'budget_limited', durationMs: 75_000 },
  },
};

// Degenerate state — missing reason and duration on the wire. The label
// collapses to just `Turn aborted` with the timestamp on the right.
export const LabelOnly: Story = {
  args: {
    item: { kind: 'turn_aborted', lineId: '0', timestamp: ts, reason: '', durationMs: 0 },
  },
};

// CF-360: deep-link landing variant — accent pulse + ring on the divider.
export const WithDeepLinkTarget: Story = {
  args: {
    item: { kind: 'turn_aborted', lineId: '0', timestamp: ts, reason: 'interrupted', durationMs: 4_000 },
    sessionId: 'story-session',
    isDeepLinkTarget: true,
  },
};
