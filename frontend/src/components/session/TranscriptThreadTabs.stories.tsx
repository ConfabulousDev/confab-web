import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { userEvent, within } from 'storybook/test';
import type { TranscriptThreadRef, TranscriptThreadStatus } from '@/providers/types';
import TranscriptThreadTabs from './TranscriptThreadTabs';

// we3k: thread switcher. Fictional thread data (never real session ids). Use the
// theme toolbar for light / dark; hover or Tab onto a chip for its tooltip.

function thread(id: string, label: string, extra: Partial<TranscriptThreadRef> = {}): TranscriptThreadRef {
  return { id, fileName: `agent-${id}.jsonl`, label, parentThreadId: null, launchTargetId: `launch-${id}`, ...extra };
}

const STATUS_CYCLE: TranscriptThreadStatus[] = ['completed', 'running', 'completed', 'failed', 'completed', 'stopped'];

const oneSubagent = [
  thread('a1', 'Explore the auth middleware', { status: 'running', subtitle: 'Explore', model: 'claude-opus-5' }),
];

const mixedStatuses = [
  thread('a1', 'Find smart recap callers', { status: 'running', subtitle: 'Explore', model: 'claude-opus-5' }),
  thread('a2', 'Implement pedp', { status: 'completed', subtitle: 'general-purpose', model: 'claude-opus-5', durationMs: 412_000 }),
  thread('a3', 'Simplify diff', { status: 'failed', model: 'claude-sonnet-5', durationMs: 38_000 }),
  thread('a4', 'Run the flaky shard again', { status: 'stopped', subtitle: 'claude' }),
  thread('a5', 'a9f2b8d95dea9ece'),
];

const TWELVE_LABELS = [
  'Review the migration ordering',
  'Find every caller of resolveRepo',
  'Write integration tests for sharing',
  'Explore the auth middleware',
  'Simplify the recap prompt',
  'Audit dead exports with knip',
  'Check pricing table drift',
  'Draft the release notes',
  'Trace the polling backoff',
  'Reproduce the Codex title bug',
  'Port the docs sidebar',
  'Verify dark theme tokens',
];

const twelveThreads = TWELVE_LABELS.map((label, i) =>
  thread(`m${i + 1}`, label, {
    status: STATUS_CYCLE[i % STATUS_CYCLE.length],
    subtitle: 'claude',
    model: 'claude-opus-5',
    durationMs: 30_000 + i * 17_000,
  }),
);

// 48ch labels: the longest real subagent descriptions.
const fortyThreads = Array.from({ length: 40 }, (_, i) =>
  thread(`l${i + 1}`, `Reconcile token usage for provider batch ${String(i + 1).padStart(2, '0')} now`, {
    status: STATUS_CYCLE[i % STATUS_CYCLE.length],
    subtitle: i % 3 === 0 ? 'Explore' : 'claude',
    model: i % 4 === 0 ? 'claude-haiku-4-5' : 'claude-opus-5',
  }),
);

const nestedThreads = [
  thread('a1', 'Explore the auth middleware', { status: 'completed', subtitle: 'Explore', model: 'claude-opus-5' }),
  thread('c1', 'Read session store', { status: 'running', parentThreadId: 'a1', launchTargetId: 'launch-c1' }),
  thread('c2', 'Check cookie flags', { status: 'completed', parentThreadId: 'a1', launchTargetId: 'launch-c2' }),
  thread('a2', 'Write integration tests', { status: 'running', model: 'claude-opus-5' }),
];

function Interactive({
  threads,
  initial,
  width,
}: {
  threads: TranscriptThreadRef[];
  initial: string | null;
  width?: number;
}) {
  const [active, setActive] = useState<string | null>(initial);
  const [landing, setLanding] = useState<string | undefined>(undefined);
  return (
    <div style={{ maxWidth: width }}>
      <TranscriptThreadTabs
        threads={threads}
        activeThreadId={active}
        sessionId="story-session"
        onSelect={(threadId, targetId) => {
          setActive(threadId);
          setLanding(targetId);
        }}
      />
      <p style={{ padding: '12px 16px', margin: 0, color: 'var(--color-text-secondary)', fontSize: 'var(--font-sm)' }}>
        Active thread: {active ?? 'Main'}
        {landing && ` (landing on ${landing})`}
      </p>
    </div>
  );
}

const meta: Meta<typeof Interactive> = {
  title: 'Session/TranscriptThreadTabs',
  component: Interactive,
  parameters: { layout: 'fullscreen' },
};
export default meta;

type Story = StoryObj<typeof Interactive>;

async function openActiveChipMenu(canvasElement: HTMLElement) {
  await userEvent.click(within(canvasElement).getByRole('tab', { selected: true }));
}

async function openAllSubagents(canvasElement: HTMLElement) {
  await userEvent.click(within(canvasElement).getByRole('button', { name: /^All subagents/ }));
}

/** One subagent: no overflow, no fades or step buttons; the menu button still shows. */
export const OneSubagent: Story = {
  args: { threads: oneSubagent, initial: null },
};

/** Running (spinning ring), completed, failed, stopped and unknown (deep-linked) glyphs. */
export const FewMixedStatuses: Story = {
  args: { threads: mixedStatuses, initial: 'a2' },
};

/** 12 subagents with the active chip mid-strip, so both edge fades and step buttons show at desktop width. */
export const ManyOverflow: Story = {
  args: { threads: twelveThreads, initial: 'm7' },
};

/** 40 subagents with 48ch labels; the All subagents list opens with its filter. */
export const FortyLongLabels: Story = {
  args: { threads: fortyThreads, initial: 'l20' },
  play: async ({ canvasElement }) => openAllSubagents(canvasElement),
};

/** Two nested subagents placed after their parent, with an indent mark; open the nested chip's menu for Go to parent. */
export const NestedLive: Story = {
  args: { threads: nestedThreads, initial: 'c1' },
};

/** The active subagent chip's menu: Go to parent (Main) and Copy link. */
export const ActiveChipMenuOpen: Story = {
  args: { threads: mixedStatuses, initial: 'a2' },
  play: async ({ canvasElement }) => openActiveChipMenu(canvasElement),
};

/** All subagents list: Main first, status, full label, type, model, duration. */
export const AllSubagentsOpen: Story = {
  args: { threads: twelveThreads, initial: 'm3' },
  play: async ({ canvasElement }) => openAllSubagents(canvasElement),
};

/** ~400px: step buttons hide (swipe the strip), labels shorten, All subagents collapses to icon + count. */
export const NarrowWidth: Story = {
  args: { threads: twelveThreads, initial: 'm3', width: 400 },
};

/** A deep-linked subagent whose parent is unknown: its menu has only Copy link. */
export const DeepLinkUnknownParent: Story = {
  args: {
    threads: [
      ...mixedStatuses.slice(0, 3),
      thread('zz91c4e07b2f5a11', 'zz91c4e07b2f5a11', { parentThreadId: undefined, launchTargetId: undefined }),
    ],
    initial: 'zz91c4e07b2f5a11',
  },
  play: async ({ canvasElement }) => openActiveChipMenu(canvasElement),
};
