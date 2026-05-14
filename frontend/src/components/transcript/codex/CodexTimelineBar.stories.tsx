import { useState, type ReactNode } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import CodexTimelineBar from './CodexTimelineBar';
import type { CodexRenderItem } from '@/types/codexRenderItem';

const meta: Meta<typeof CodexTimelineBar> = {
  title: 'Transcript/Codex/CodexTimelineBar',
  component: CodexTimelineBar,
  parameters: { layout: 'fullscreen' },
};

export default meta;
type Story = StoryObj<typeof CodexTimelineBar>;

function user(timestamp: string, text = 'hi'): CodexRenderItem {
  return { kind: 'user', timestamp, text };
}

function assistant(timestamp: string): CodexRenderItem {
  return { kind: 'assistant', timestamp, text: 'response', phase: 'final', model: 'gpt-5' };
}

function toolCall(timestamp: string, callId: string): CodexRenderItem {
  return {
    kind: 'tool_call',
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
  return { kind: 'turn_separator', timestamp, turnIndex, durationMs, timeToFirstTokenMs };
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
      <CodexTimelineBar
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
      <CodexTimelineBar items={singleTurn} selectedIndex={0} onSeek={() => undefined} />
    </BarFrame>
  ),
};

export const InFlight: Story = {
  render: () => (
    <BarFrame label="In-flight turn (no separator yet).">
      <CodexTimelineBar items={inFlight} selectedIndex={0} onSeek={() => undefined} />
    </BarFrame>
  ),
};

export const Empty: Story = {
  render: () => (
    <BarFrame label="Empty items — bar renders nothing.">
      <CodexTimelineBar items={[]} selectedIndex={0} onSeek={() => undefined} />
    </BarFrame>
  ),
};
