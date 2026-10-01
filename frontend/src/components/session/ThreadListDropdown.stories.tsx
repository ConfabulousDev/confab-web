import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { userEvent, within } from 'storybook/test';
import type { TranscriptThreadRef, TranscriptThreadStatus } from '@/providers/types';
import ThreadListDropdown from './ThreadListDropdown';
import { nestedThreadsNote } from './threadDetails';

// we3k D10 / jgk8: the thread list dropdown behind All subagents, the subagent
// path's sibling segments and Launched here. Fictional thread data.

function thread(id: string, label: string, extra: Partial<TranscriptThreadRef> = {}): TranscriptThreadRef {
  return { id, fileName: `agent-${id}.jsonl`, label, parentThreadId: null, launchTargetId: `launch-${id}`, ...extra };
}

const STATUS_CYCLE: TranscriptThreadStatus[] = ['completed', 'running', 'failed', 'stopped'];

const fewThreads = [
  thread('a1', 'Explore the auth middleware', { status: 'running', subtitle: 'Explore', model: 'claude-opus-5' }),
  thread('a2', 'Implement pedp', { status: 'completed', subtitle: 'general-purpose', model: 'claude-opus-5', durationMs: 412_000 }),
  thread('a3', 'Simplify diff', { status: 'failed', model: 'claude-sonnet-5' }),
];

const manyThreads = Array.from({ length: 12 }, (_, i) =>
  thread(`m${i + 1}`, `Reconcile token usage for provider batch ${i + 1}`, {
    status: STATUS_CYCLE[i % STATUS_CYCLE.length],
    subtitle: i % 3 === 0 ? 'Explore' : 'claude',
    model: i % 2 === 0 ? 'claude-haiku-4-5' : 'claude-opus-5',
  }),
);

const childCounts: Record<string, number> = { a1: 2, a2: 43 };

type InteractiveProps = Omit<React.ComponentProps<typeof ThreadListDropdown>, 'activeThreadId' | 'onSelect'> & {
  initial: string | null;
  alignRight?: boolean;
};

function Interactive({ initial, alignRight = true, ...props }: InteractiveProps) {
  const [active, setActive] = useState<string | null>(initial);
  return (
    <div style={{ display: 'flex', justifyContent: alignRight ? 'flex-end' : 'flex-start', minHeight: 560 }}>
      <ThreadListDropdown {...props} activeThreadId={active} onSelect={setActive} />
    </div>
  );
}

const meta: Meta<typeof Interactive> = {
  title: 'Session/ThreadListDropdown',
  component: Interactive,
  parameters: { layout: 'padded' },
};
export default meta;

type Story = StoryObj<typeof Interactive>;

async function open(canvasElement: HTMLElement) {
  await userEvent.click(within(canvasElement).getByRole('button'));
}

/** All subagents: Main first, "launched N" on agents whose children are known, and the footer for nested agents. */
export const AllSubagentsWithFooter: Story = {
  args: {
    threads: fewThreads,
    initial: 'a2',
    includeMain: true,
    label: 'All subagents (375)',
    listLabel: 'All subagents',
    footer: nestedThreadsNote(372),
    childCountOf: (id: string) => childCounts[id] ?? 0,
  },
  play: async ({ canvasElement }) => open(canvasElement),
};

/** More than 8 threads: a filter input (label, type or model) is focused on open. */
export const LongListWithFilter: Story = {
  args: { threads: manyThreads, initial: null, includeMain: true, label: 'All subagents (12)', listLabel: 'All subagents' },
  play: async ({ canvasElement }) => open(canvasElement),
};

/** Launched here: the open agent's children, nothing checked, popover aligned to the trigger's left edge. */
export const LaunchedHere: Story = {
  args: {
    threads: fewThreads,
    initial: null,
    label: 'Launched here (3)',
    listLabel: 'Launched by Implement 0e6y verification step',
    align: 'start',
    alignRight: false,
    childCountOf: (id: string) => childCounts[id] ?? 0,
  },
  play: async ({ canvasElement }) => open(canvasElement),
};
