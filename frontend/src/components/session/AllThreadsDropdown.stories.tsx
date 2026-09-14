import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { userEvent, within } from 'storybook/test';
import type { TranscriptThreadRef, TranscriptThreadStatus } from '@/providers/types';
import AllThreadsDropdown from './AllThreadsDropdown';

// Fictional thread data.
function thread(id: string, label: string, extra: Partial<TranscriptThreadRef> = {}): TranscriptThreadRef {
  return { id, fileName: `agent-${id}.jsonl`, label, parentThreadId: null, launchTargetId: `launch-${id}`, ...extra };
}

const STATUS_CYCLE: TranscriptThreadStatus[] = ['completed', 'running', 'failed', 'stopped'];

const fewThreads = [
  thread('a1', 'Explore the auth middleware', { status: 'running', subtitle: 'Explore', model: 'claude-opus-5' }),
  thread('c1', 'Read session store', { status: 'completed', parentThreadId: 'a1', durationMs: 21_000 }),
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

function Interactive({ threads, initial }: { threads: TranscriptThreadRef[]; initial: string | null }) {
  const [active, setActive] = useState<string | null>(initial);
  return (
    <div style={{ display: 'flex', justifyContent: 'flex-end', minHeight: 520 }}>
      <AllThreadsDropdown threads={threads} activeThreadId={active} onSelect={setActive} />
    </div>
  );
}

const meta: Meta<typeof Interactive> = {
  title: 'Session/AllThreadsDropdown',
  component: Interactive,
  parameters: { layout: 'padded' },
};
export default meta;

type Story = StoryObj<typeof Interactive>;

async function open(canvasElement: HTMLElement) {
  await userEvent.click(within(canvasElement).getByRole('button', { name: /^All subagents/ }));
}

/** Four threads (one nested), no filter. */
export const Open: Story = {
  args: { threads: fewThreads, initial: 'a2' },
  play: async ({ canvasElement }) => open(canvasElement),
};

/** More than 8 subagents: a filter input (label, type or model) is focused on open. */
export const LongListWithFilter: Story = {
  args: { threads: manyThreads, initial: null },
  play: async ({ canvasElement }) => open(canvasElement),
};
