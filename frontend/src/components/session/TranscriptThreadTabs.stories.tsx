import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import type { TranscriptThreadRef } from '@/providers/types';
import TranscriptThreadTabs from './TranscriptThreadTabs';

function thread(id: string, label: string, parentThreadId: string | null | undefined = null): TranscriptThreadRef {
  return { id, fileName: `agent-${id}.jsonl`, label, parentThreadId, launchTargetId: `launch-${id}` };
}

const fewThreads = [
  thread('a1', 'Explore the auth middleware'),
  thread('a2', 'Write integration tests'),
  thread('a3', 'Review the migration'),
];

const manyThreads = Array.from({ length: 20 }, (_, i) =>
  thread(`m${i + 1}`, `Implement issue batch item ${i + 1} with a long description`),
);

function Interactive({ threads, initial }: { threads: TranscriptThreadRef[]; initial: string | null }) {
  const [active, setActive] = useState<string | null>(initial);
  const activeThread = threads.find((t) => t.id === active);
  const parentLabel =
    activeThread?.parentThreadId === null
      ? 'Main'
      : threads.find((t) => t.id === activeThread?.parentThreadId)?.label;
  return (
    <TranscriptThreadTabs
      threads={threads}
      activeThreadId={active}
      onSelect={setActive}
      backLink={
        activeThread && parentLabel
          ? { label: parentLabel, onClick: () => setActive(activeThread.parentThreadId ?? null) }
          : undefined
      }
    />
  );
}

const meta: Meta<typeof Interactive> = {
  title: 'Session/TranscriptThreadTabs',
  component: Interactive,
  parameters: { layout: 'fullscreen' },
};
export default meta;

type Story = StoryObj<typeof Interactive>;

export const FewThreads: Story = {
  args: { threads: fewThreads, initial: null },
};

export const SubagentActiveWithBackLink: Story = {
  args: { threads: fewThreads, initial: 'a2' },
};

export const ManyThreadsOverflow: Story = {
  args: { threads: manyThreads, initial: 'm12' },
};

export const NestedLabel: Story = {
  args: {
    threads: [...fewThreads, thread('n1', 'Explore the auth middleware › Read session store', 'a1')],
    initial: 'n1',
  },
};
