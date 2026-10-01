import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { userEvent, within } from 'storybook/test';
import type { TranscriptThreadRef, TranscriptThreadStatus } from '@/providers/types';
import ThreadBreadcrumb from './ThreadBreadcrumb';
import TranscriptThreadTabs from './TranscriptThreadTabs';
import { threadPath } from './threadDetails';

// jgk8 (D9 review: trim + inline): the strip and the nested-subagent path row
// together, wired like SessionViewer. Fictional data shaped like an eval run: a
// depth-1 implementer launches 43 background judges; one judge launches its own
// helper. Use the theme toolbar for light / dark.

function thread(
  id: string,
  label: string,
  parentThreadId: string | null | undefined,
  extra: Partial<TranscriptThreadRef> = {},
): TranscriptThreadRef {
  return { id, fileName: `agent-${id}.jsonl`, label, parentThreadId, launchTargetId: `launch-${id}`, ...extra };
}

const STATUS_CYCLE: TranscriptThreadStatus[] = ['completed', 'completed', 'running', 'completed', 'failed', 'stopped'];
const TOPICS = [
  'arrays', 'async-event-loops', 'b-trees', 'bloom-filters', 'consistent-hashing', 'dijkstra', 'graph-coloring',
  'hash-maps', 'heaps', 'interval-trees', 'lru-cache', 'merge-sort', 'rate-limiters', 'red-black-trees',
  'regex-engines', 'ring-buffers', 'skip-lists', 'tries', 'union-find', 'vector-clocks', 'websocket-framing',
];

const impl = thread('a05d3d18', 'Implement 0e6y verification step', null, {
  status: 'completed',
  subtitle: 'general-purpose',
  model: 'claude-opus-5',
  durationMs: 5_412_000,
});
const explore = thread('b17c9e20', 'Explore the eval harness', null, { status: 'completed', subtitle: 'Explore' });

const judges = Array.from({ length: 43 }, (_, i) => {
  const topic = TOPICS[Math.floor(i / 2) % TOPICS.length];
  return thread(`j${String(i + 1).padStart(2, '0')}c3e9`, i === 42 ? 'Simplify 0e6y Go changes' : `Judge ${topic} r${(i % 2) + 1}`, impl.id, {
    status: STATUS_CYCLE[i % STATUS_CYCLE.length],
    subtitle: 'general-purpose',
    model: i % 3 === 0 ? 'claude-sonnet-5' : 'claude-opus-5',
    durationMs: 40_000 + i * 3_100,
  });
});
const firstJudge = judges[0]!;
const rerun = thread('g1f0aa7e', 'Re-run the flaky arrays judge with a fresh worktree', firstJudge.id, {
  status: 'running',
  model: 'claude-opus-5',
});

const fewJudges = judges.slice(0, 4);
const COLD_ID = 'a9f2b8d95dea9ece';
const coldChildren = [
  thread('c0ld0001', 'Judge tries r1', COLD_ID, { status: 'completed' }),
  thread('c0ld0002', 'Judge tries r2', COLD_ID, { status: 'running' }),
];

interface HarnessProps {
  mainThreads: TranscriptThreadRef[];
  childrenById: Record<string, TranscriptThreadRef[]>;
  initial: string;
  width?: number;
}

function Harness({ mainThreads, childrenById, initial, width }: HarnessProps) {
  const [active, setActive] = useState<string | null>(initial);
  const [landing, setLanding] = useState<string | undefined>(undefined);
  const known = new Map([...mainThreads, ...Object.values(childrenById).flat()].map((t) => [t.id, t]));
  const activeRef = active === null ? undefined : (known.get(active) ?? thread(active, active, undefined));
  const path = activeRef ? threadPath(activeRef, known) : undefined;
  const childrenOf = (id: string) => childrenById[id] ?? [];
  const onSelect = (threadId: string | null, targetId?: string) => {
    setActive(threadId);
    setLanding(targetId);
  };
  const [depth1, next] = path ?? [];
  // A realistic All-subagents total: every agent in the fixture tree (as
  // session.files would list them), plus an unknown open one.
  const total = known.size + (activeRef && !known.has(activeRef.id) ? 1 : 0);
  return (
    <div style={{ maxWidth: width, minHeight: 600 }}>
      <TranscriptThreadTabs
        threads={activeRef && !path ? [...mainThreads, activeRef] : mainThreads}
        activeThreadId={active}
        sessionId="story-session"
        onSelect={onSelect}
        allThreadsCount={total}
        nestedThreadCount={total - mainThreads.length}
        childCountOf={(id) => childrenOf(id).length}
        inPath={depth1 && next ? { threadId: depth1.id, targetId: next.launchTargetId } : undefined}
        launchedHere={path?.length === 1 ? childrenOf(path[0]!.id) : undefined}
      />
      {activeRef && (
        <ThreadBreadcrumb
          active={activeRef}
          path={path}
          childrenOf={childrenOf}
          sessionId="story-session"
          onSelect={onSelect}
        />
      )}
      <p style={{ padding: '12px 16px', margin: 0, color: 'var(--color-text-secondary)', fontSize: 'var(--font-sm)' }}>
        Open thread: {activeRef?.label ?? 'Main'}
        {landing && ` (landing on ${landing})`}
      </p>
    </div>
  );
}

const meta: Meta<typeof Harness> = {
  title: 'Session/ThreadBreadcrumb',
  component: Harness,
  parameters: { layout: 'fullscreen' },
};
export default meta;

type Story = StoryObj<typeof Harness>;

const evalChildren = { [impl.id]: judges, [firstJudge.id]: [rerun] };

async function openDropdown(canvasElement: HTMLElement, name: string | RegExp) {
  await userEvent.click(within(canvasElement).getByRole('button', { name }));
}

/** Depth 1 with children: one row. "Launched here (4)" sits in the strip, left of All subagents; no path row. */
export const Depth1WithChildren: Story = {
  args: { mainThreads: [impl, explore], childrenById: { [impl.id]: fewJudges }, initial: impl.id },
};

/** Depth 1, the strip's Launched here open: 43 judges with a filter, "launched N" on the judge that launched one. */
export const Depth1LaunchedHereOpen: Story = {
  args: { mainThreads: [impl, explore], childrenById: evalChildren, initial: impl.id },
  play: async ({ canvasElement }) => openDropdown(canvasElement, /^Launched here/),
};

/** Depth 2 with no children: the row starts at depth 2 (↳ glyph, label, sibling count ▾), then copy link. The implementer chip is "in path". */
export const Depth2: Story = {
  args: { mainThreads: [impl, explore], childrenById: { [impl.id]: fewJudges }, initial: fewJudges[2]!.id },
};

/** Depth 2 among 43 siblings: the segment dropdown is open with the current judge checked and the filter focused. */
export const Depth2FortyThreeSiblings: Story = {
  args: { mainThreads: [impl, explore], childrenById: evalChildren, initial: judges[5]!.id },
  play: async ({ canvasElement }) => openDropdown(canvasElement, judges[5]!.label),
};

/** Depth 2 with a child: the row ends with "Launched here (1)" before copy link. */
export const Depth2WithChildren: Story = {
  args: { mainThreads: [impl, explore], childrenById: evalChildren, initial: firstJudge.id },
};

/** Depth 3: two dropdown segments (the judge, then its helper with a long, truncated label) and copy link. */
export const Depth3: Story = {
  args: { mainThreads: [impl, explore], childrenById: evalChildren, initial: rerun.id },
};

/** Cold deep link to a nested agent: an id chip in the strip, and a row with only "↳ Launched here (2) ▾" and copy link. */
export const DeepLinkUnknownParent: Story = {
  args: { mainThreads: [impl, explore], childrenById: { [COLD_ID]: coldChildren }, initial: COLD_ID },
};

/** ~400px, depth 1: the strip's Launched here collapses to ↳ + count, like All subagents. */
export const NarrowDepth1: Story = {
  args: { mainThreads: [impl, explore], childrenById: evalChildren, initial: impl.id, width: 400 },
};

/** ~400px, depth 3: the row wraps instead of scrolling; segments keep their 24ch truncation. */
export const NarrowDepth3: Story = {
  args: { mainThreads: [impl, explore], childrenById: evalChildren, initial: rerun.id, width: 400 },
};
