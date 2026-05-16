import { useState, type ReactNode } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import CodexTimelineBar from './CodexTimelineBar';
import { useCodexSegmentLayout } from './codexTimelineSegments';
import type { CodexRenderItem } from '@/types/codexRenderItem';

// CF-362: CodexTimelineBar now takes a precomputed layout. Stories drive
// it via this thin wrapper to keep the story bodies focused on item shapes.
function Bar({
  items,
  selectedIndex = 0,
  onSeek,
}: {
  items: CodexRenderItem[];
  selectedIndex?: number;
  onSeek: (idx: number) => void;
}) {
  const layout = useCodexSegmentLayout(items, selectedIndex);
  return <CodexTimelineBar layout={layout} onSeek={onSeek} />;
}

const meta: Meta<typeof CodexTimelineBar> = {
  title: 'Transcript/Codex/CodexTimelineBar',
  component: CodexTimelineBar,
  parameters: { layout: 'fullscreen' },
};

export default meta;
type Story = StoryObj<typeof CodexTimelineBar>;

function user(timestamp: string, text = 'hi'): CodexRenderItem {
  return { kind: 'user', lineId: '0', timestamp, text };
}

function assistant(timestamp: string): CodexRenderItem {
  return { kind: 'assistant', lineId: '0', timestamp, text: 'response', phase: 'final', model: 'gpt-5' };
}

function toolCall(timestamp: string, callId: string): CodexRenderItem {
  return {
    kind: 'tool_call',
    lineId: '0',
    timestamp,
    toolName: 'exec_command',
    callId,
    rawInput: { cmd: 'pwd' },
    rawOutput: '/tmp',
    status: 'completed',
    execMetadata: { exitCode: 0, wallTimeMs: 100 },
  };
}

function turnSep(
  timestamp: string,
  turnIndex: number,
  durationMs: number,
  timeToFirstTokenMs?: number,
): CodexRenderItem {
  return { kind: 'turn_separator', lineId: '0', timestamp, turnIndex, durationMs, timeToFirstTokenMs };
}

// Three turns of varying duration / item count.
const multipleTurns: CodexRenderItem[] = [
  user('2026-05-13T18:00:00Z', 'short turn'),
  assistant('2026-05-13T18:00:01Z'),
  turnSep('2026-05-13T18:00:01Z', 1, 1500, 200),
  user('2026-05-13T18:01:00Z', 'medium turn'),
  toolCall('2026-05-13T18:01:01Z', 'c1'),
  toolCall('2026-05-13T18:01:10Z', 'c2'),
  assistant('2026-05-13T18:01:30Z'),
  turnSep('2026-05-13T18:01:30Z', 2, 30000, 600),
  user('2026-05-13T18:02:00Z', 'long turn'),
  toolCall('2026-05-13T18:02:05Z', 'c3'),
  toolCall('2026-05-13T18:02:30Z', 'c4'),
  toolCall('2026-05-13T18:03:00Z', 'c5'),
  assistant('2026-05-13T18:05:00Z'),
  turnSep('2026-05-13T18:05:00Z', 3, 180000, 1500),
];

const singleTurn: CodexRenderItem[] = [
  user('2026-05-13T18:00:00Z'),
  assistant('2026-05-13T18:00:05Z'),
  turnSep('2026-05-13T18:00:05Z', 1, 5000, 800),
];

const inFlight: CodexRenderItem[] = [
  user('2026-05-13T18:00:00Z'),
  toolCall('2026-05-13T18:00:01Z', 'c1'),
  // No closing turn_separator → trailing in-flight segment.
];

function BarFrame({ children, label }: { children: ReactNode; label: string }) {
  return (
    <div
      style={{
        display: 'flex',
        gap: '24px',
        padding: '24px',
        height: '500px',
        background: 'var(--color-bg)',
        color: 'var(--color-text-primary)',
      }}
    >
      <div style={{ flex: 1, fontSize: '14px' }}>{label}</div>
      <div style={{ width: '40px', position: 'relative' }}>{children}</div>
    </div>
  );
}

function MultipleTurnsDemo() {
  const [selectedIndex, setSelectedIndex] = useState(0);
  return (
    <BarFrame label="Multiple turns — hover segments to see tooltip; click to update selectedIndex.">
      <Bar
        items={multipleTurns}
        selectedIndex={selectedIndex}
        onSeek={(idx) => setSelectedIndex(idx)}
      />
    </BarFrame>
  );
}

export const MultipleTurns: Story = {
  render: () => <MultipleTurnsDemo />,
};

export const SingleTurn: Story = {
  render: () => (
    <BarFrame label="Single completed turn.">
      <Bar items={singleTurn} selectedIndex={0} onSeek={() => undefined} />
    </BarFrame>
  ),
};

export const InFlight: Story = {
  render: () => (
    <BarFrame label="In-flight turn (no separator yet).">
      <Bar items={inFlight} selectedIndex={0} onSeek={() => undefined} />
    </BarFrame>
  ),
};

export const Empty: Story = {
  render: () => (
    <BarFrame label="Empty items — bar renders nothing.">
      <Bar items={[]} selectedIndex={0} onSeek={() => undefined} />
    </BarFrame>
  ),
};

// Two turns separated by a ~10 minute thinking gap. The blue user stripe
// for turn 2 visibly dominates because the time blend favors duration.
const longUserGap: CodexRenderItem[] = [
  user('2026-05-13T18:00:00Z', 'first prompt'),
  assistant('2026-05-13T18:00:05Z'),
  turnSep('2026-05-13T18:00:06Z', 1, 6000, 800),
  user('2026-05-13T18:10:00Z', 'after a walk'), // ~10 min user thinking gap
  assistant('2026-05-13T18:10:08Z'),
  turnSep('2026-05-13T18:10:09Z', 2, 8000, 900),
];

export const LongUserGap: Story = {
  render: () => (
    <BarFrame label="Two turns with a long user thinking gap between them — note the dominant blue stripe.">
      <Bar items={longUserGap} selectedIndex={0} onSeek={() => undefined} />
    </BarFrame>
  ),
};
