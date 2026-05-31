import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState, useMemo } from 'react';
import { CostBar } from './CostBar';
import { TimelineBar } from './claude/TimelineBar';
import { useSegmentLayout } from './timelineSegments';
import type { TranscriptLine, UserMessage, AssistantMessage } from '@/types';
import { isAssistantMessage } from '@/types';
import { normalizeClaudeUsage } from '@/utils/tokenStats';
import { claudeAdapter } from '@/providers/claudeAdapter';
import { codexAdapter } from '@/providers/codexAdapter';
import { useCodexSegmentLayout } from './codex/codexTimelineSegments';
import type { CodexRenderItem } from '@/types/codexRenderItem';

const meta: Meta<typeof CostBar> = {
  title: 'Transcript/CostBar',
  component: CostBar,
  parameters: {
    layout: 'fullscreen',
  },
};

export default meta;
type Story = StoryObj<typeof CostBar>;

const baseMessage = {
  parentUuid: null,
  isSidechain: false,
  userType: 'external',
  cwd: '/project',
  sessionId: 'test-session',
  version: '1.0.0',
};

function createUserMessage(uuid: string, timestamp: string, content: string): UserMessage {
  return {
    ...baseMessage,
    type: 'user',
    uuid,
    timestamp,
    message: { role: 'user', content },
  };
}

function createAssistantMessage(
  uuid: string,
  timestamp: string,
  text: string,
  inputTokens: number,
  outputTokens: number,
  model = 'claude-sonnet-4-20250514',
  extra?: { speed?: string; server_tool_use?: { web_search_requests?: number } },
): AssistantMessage {
  return {
    ...baseMessage,
    type: 'assistant',
    uuid,
    timestamp,
    requestId: `req-${uuid}`,
    message: {
      model,
      id: `msg-${uuid}`,
      type: 'message',
      role: 'assistant',
      content: [{ type: 'text', text }],
      stop_reason: 'end_turn',
      stop_sequence: null,
      usage: {
        input_tokens: inputTokens,
        output_tokens: outputTokens,
        ...extra,
      },
    },
  };
}

function createConversation(): TranscriptLine[] {
  const messages: TranscriptLine[] = [];
  let time = new Date('2025-01-01T10:00:00Z').getTime();
  let idx = 0;

  // Turn 1: cheap
  messages.push(createUserMessage(`u${idx++}`, new Date(time).toISOString(), 'Hello, how are you?'));
  time += 2000;
  messages.push(createAssistantMessage(`a${idx++}`, new Date(time).toISOString(), 'Hello! I am fine.', 500, 200));
  time += 15000;

  // Turn 2: moderate
  messages.push(createUserMessage(`u${idx++}`, new Date(time).toISOString(), 'Help me refactor this function'));
  time += 5000;
  messages.push(createAssistantMessage(`a${idx++}`, new Date(time).toISOString(), 'Reading file...', 10000, 500));
  time += 3000;
  messages.push(createAssistantMessage(`a${idx++}`, new Date(time).toISOString(), 'Editing file...', 15000, 2000));
  time += 2000;
  messages.push(createAssistantMessage(`a${idx++}`, new Date(time).toISOString(), 'Done!', 12000, 800));
  time += 20000;

  // Turn 3: expensive (opus + lots of tokens)
  messages.push(createUserMessage(`u${idx++}`, new Date(time).toISOString(), 'Now implement the full feature'));
  time += 10000;
  messages.push(createAssistantMessage(`a${idx++}`, new Date(time).toISOString(), 'Planning...', 50000, 5000, 'claude-opus-4-5-20251101'));
  time += 15000;
  messages.push(createAssistantMessage(`a${idx++}`, new Date(time).toISOString(), 'Implementing...', 80000, 15000, 'claude-opus-4-5-20251101'));
  time += 20000;
  messages.push(createAssistantMessage(`a${idx++}`, new Date(time).toISOString(), 'Testing...', 60000, 8000, 'claude-opus-4-5-20251101'));
  time += 30000;

  // Turn 4: cheap
  messages.push(createUserMessage(`u${idx++}`, new Date(time).toISOString(), 'Looks good, thanks!'));
  time += 3000;
  messages.push(createAssistantMessage(`a${idx++}`, new Date(time).toISOString(), 'You are welcome!', 1000, 300));
  time += 10000;

  // Turn 5: moderate with web search
  messages.push(createUserMessage(`u${idx++}`, new Date(time).toISOString(), 'Search for best practices'));
  time += 8000;
  messages.push(createAssistantMessage(`a${idx++}`, new Date(time).toISOString(), 'Searching...', 20000, 3000, 'claude-sonnet-4-20250514', { server_tool_use: { web_search_requests: 5 } }));
  time += 5000;
  messages.push(createAssistantMessage(`a${idx++}`, new Date(time).toISOString(), 'Here are the results.', 25000, 4000));

  return messages;
}

function buildCostMap(messages: TranscriptLine[]): { messageCosts: Map<number, number>; totalCost: number } {
  const messageCosts = new Map<number, number>();
  let totalCost = 0;
  for (let i = 0; i < messages.length; i++) {
    const msg = messages[i]!;
    if (!isAssistantMessage(msg)) continue;
    const usage = msg.tokenUsage ?? normalizeClaudeUsage(msg.message.usage);
    const cost = claudeAdapter.calculateMessageCost(msg.message.model, usage, msg);
    if (cost > 0) {
      messageCosts.set(i, cost);
      totalCost += cost;
    }
  }
  return { messageCosts, totalCost };
}

// Mirrors `CostBarSlot` in MessageTimeline.tsx: dedup by message.id so the
// density math reflects unique API calls, not raw row counts.
function useClaudeSegmentUniqueCounts(
  messages: TranscriptLine[],
  segments: ReturnType<typeof useSegmentLayout>['segments'],
): number[] {
  return useMemo(() => {
    return segments.map((seg) => {
      const seen = new Set<string>();
      for (let i = seg.startIndex; i <= seg.endIndex; i++) {
        const msg = messages[i];
        if (msg && isAssistantMessage(msg)) seen.add(msg.message.id);
      }
      return seen.size;
    });
  }, [messages, segments]);
}

/**
 * Side-by-side with TimelineBar, showing both bars as they appear in the real UI.
 */
function IntegratedDemo() {
  const messages = useMemo(() => createConversation(), []);
  const { messageCosts, totalCost } = useMemo(() => buildCostMap(messages), [messages]);
  const [selectedIndex, setSelectedIndex] = useState(0);
  const layout = useSegmentLayout(messages, selectedIndex);
  const uniqueCounts = useClaudeSegmentUniqueCounts(messages, layout.segments);

  return (
    <div style={{ padding: '24px', background: 'var(--color-bg)', minHeight: '100vh' }}>
      <h3 style={{ marginBottom: '16px', color: 'var(--color-text-primary)' }}>CostBar + TimelineBar Side-by-Side</h3>
      <p style={{ marginBottom: '16px', color: 'var(--color-text-secondary)', fontSize: '14px' }}>
        Left: cost heatmap (red intensity = relative cost). Right: speaker timeline.
      </p>

      <div style={{ display: 'flex', gap: '4px', height: '500px', width: '60px' }}>
        <CostBar
          layout={layout}
          costByIndex={messageCosts}
          segmentUniqueCounts={uniqueCounts}
          totalCost={totalCost}
          onSeek={(startIndex) => setSelectedIndex(startIndex)}
        />
        <TimelineBar
          messages={messages}
          selectedIndex={selectedIndex}
          onSeek={(startIndex) => setSelectedIndex(startIndex)}
        />
      </div>

      <div style={{ marginTop: '16px' }}>
        <label style={{ display: 'block', marginBottom: '8px', fontSize: '14px', color: 'var(--color-text-primary)' }}>
          Selected message: {selectedIndex} / {messages.length - 1}
        </label>
        <input
          type="range"
          min="0"
          max={messages.length - 1}
          value={selectedIndex}
          onChange={(e) => setSelectedIndex(Number(e.target.value))}
          style={{ width: '200px' }}
        />
      </div>
    </div>
  );
}

export const Integrated: Story = {
  render: () => <IntegratedDemo />,
};

/**
 * Isolated CostBar with slider control.
 */
function IsolatedDemo() {
  const messages = useMemo(() => createConversation(), []);
  const { messageCosts, totalCost } = useMemo(() => buildCostMap(messages), [messages]);
  const [selectedIndex, setSelectedIndex] = useState(0);
  const layout = useSegmentLayout(messages, selectedIndex);
  const uniqueCounts = useClaudeSegmentUniqueCounts(messages, layout.segments);

  return (
    <div style={{ padding: '24px', background: 'var(--color-bg)', minHeight: '100vh' }}>
      <h3 style={{ marginBottom: '16px', color: 'var(--color-text-primary)' }}>CostBar — Isolated</h3>

      <div style={{ display: 'flex', gap: '24px', alignItems: 'flex-start' }}>
        <div style={{ height: '400px', width: '40px', padding: '0 8px' }}>
          <CostBar
            layout={layout}
            costByIndex={messageCosts}
            segmentUniqueCounts={uniqueCounts}
            totalCost={totalCost}
            onSeek={(startIndex) => setSelectedIndex(startIndex)}
          />
        </div>

        <div>
          <label style={{ display: 'block', marginBottom: '8px', fontSize: '14px', color: 'var(--color-text-primary)' }}>
            Selected: {selectedIndex} / {messages.length - 1}
          </label>
          <input
            type="range"
            min="0"
            max={messages.length - 1}
            value={selectedIndex}
            onChange={(e) => setSelectedIndex(Number(e.target.value))}
            style={{ width: '200px' }}
          />
          <div style={{ marginTop: '8px', fontSize: '12px', color: 'var(--color-text-muted)' }}>
            Total cost: ${totalCost.toFixed(4)}
          </div>
        </div>
      </div>
    </div>
  );
}

export const Isolated: Story = {
  render: () => <IsolatedDemo />,
};

/**
 * Zero cost — CostBar renders null.
 */
function ZeroCostDemo() {
  const layout = useSegmentLayout([], 0);
  return (
    <div style={{ height: '400px', padding: '24px' }}>
      <CostBar
        layout={layout}
        costByIndex={new Map()}
        segmentUniqueCounts={[]}
        totalCost={0}
        onSeek={() => { /* no-op */ }}
      />
      <p style={{ marginTop: '16px', color: '#666' }}>
        (Nothing renders when total cost is zero)
      </p>
    </div>
  );
}

export const ZeroCost: Story = {
  render: () => <ZeroCostDemo />,
};

/**
 * Single expensive turn — one bright red segment surrounded by cheap ones.
 */
function SingleExpensiveDemo() {
  const messages = useMemo<TranscriptLine[]>(() => {
    let time = new Date('2025-01-01T10:00:00Z').getTime();
    return [
      createUserMessage('u0', new Date(time).toISOString(), 'Quick question'),
      createAssistantMessage('a0', new Date(time += 2000).toISOString(), 'Sure!', 500, 200),
      createUserMessage('u1', new Date(time += 10000).toISOString(), 'Now do the big thing'),
      createAssistantMessage('a1', new Date(time += 30000).toISOString(), 'Working...', 200000, 50000, 'claude-opus-4-5-20251101'),
      createUserMessage('u2', new Date(time += 20000).toISOString(), 'Thanks'),
      createAssistantMessage('a2', new Date(time += 2000).toISOString(), 'Welcome!', 500, 200),
    ];
  }, []);
  const { messageCosts, totalCost } = useMemo(() => buildCostMap(messages), [messages]);
  const [selectedIndex, setSelectedIndex] = useState(0);
  const layout = useSegmentLayout(messages, selectedIndex);
  const uniqueCounts = useClaudeSegmentUniqueCounts(messages, layout.segments);

  return (
    <div style={{ padding: '24px', background: 'var(--color-bg)', minHeight: '100vh' }}>
      <h3 style={{ marginBottom: '16px', color: 'var(--color-text-primary)' }}>Single Expensive Turn</h3>
      <div style={{ display: 'flex', gap: '4px', height: '400px', width: '60px' }}>
        <CostBar
          layout={layout}
          costByIndex={messageCosts}
          segmentUniqueCounts={uniqueCounts}
          totalCost={totalCost}
          onSeek={(startIndex) => setSelectedIndex(startIndex)}
        />
        <TimelineBar
          messages={messages}
          selectedIndex={selectedIndex}
          onSeek={(startIndex) => setSelectedIndex(startIndex)}
        />
      </div>
    </div>
  );
}

export const SingleExpensiveTurn: Story = {
  render: () => <SingleExpensiveDemo />,
};

// ---------------------------------------------------------------------------
// CF-362 — Codex CostBar (same component, different segment shape).
// ---------------------------------------------------------------------------

function makeCodexItems(): CodexRenderItem[] {
  // Three turns of varying expense. Each turn ends with task_complete; each
  // assistant message carries a `usage` populated as if `event_msg.token_count`
  // had been processed by the normalizer.
  const base = new Date('2026-05-15T10:00:00Z').getTime();
  const ts = (offsetMs: number) => new Date(base + offsetMs).toISOString();

  return [
    { kind: 'user', lineId: '0', timestamp: ts(0), text: 'cheap query' },
    {
      kind: 'assistant', lineId: '1', timestamp: ts(2000),
      phase: 'final', model: 'gpt-5', text: 'ok',
      usage: { input: 1000, output: 100, cacheWrite: 0, cacheRead: 0 },
    },
    { kind: 'turn_separator', lineId: '2', timestamp: ts(3000), turnIndex: 1, durationMs: 3000 },

    { kind: 'user', lineId: '3', timestamp: ts(8000), text: 'mid' },
    {
      kind: 'assistant', lineId: '4', timestamp: ts(10000),
      phase: 'commentary', model: 'gpt-5', text: 'thinking',
      // Pre-normalization: input=20000 cached=5000 → input=15000, cacheRead=5000.
      usage: { input: 15000, output: 1500, cacheWrite: 0, cacheRead: 5000 },
    },
    {
      kind: 'assistant', lineId: '5', timestamp: ts(11000),
      phase: 'final', model: 'gpt-5', text: 'answer',
      // Pre-normalization: input=25000 cached=10000 → input=15000, cacheRead=10000.
      usage: { input: 15000, output: 2000, cacheWrite: 0, cacheRead: 10000 },
    },
    { kind: 'turn_separator', lineId: '6', timestamp: ts(12000), turnIndex: 2, durationMs: 4000 },

    { kind: 'user', lineId: '7', timestamp: ts(20000), text: 'expensive' },
    {
      kind: 'assistant', lineId: '8', timestamp: ts(40000),
      phase: 'final', model: 'gpt-5.5', text: 'big response',
      // Pre-normalization: input=200000 cached=50000 → input=150000, cacheRead=50000.
      // Reasoning=5000 folded into output: 25000+5000=30000.
      usage: { input: 150000, output: 30000, cacheWrite: 0, cacheRead: 50000 },
      reasoningTokens: 5000,
    },
    { kind: 'turn_separator', lineId: '9', timestamp: ts(45000), turnIndex: 3, durationMs: 25000 },
  ];
}

function buildCodexCostMap(items: CodexRenderItem[]): {
  costByIndex: Map<number, number>;
  totalCost: number;
} {
  const costByIndex = new Map<number, number>();
  let totalCost = 0;
  items.forEach((item, idx) => {
    if (item.kind !== 'assistant' || !item.usage) return;
    const cost = codexAdapter.calculateMessageCost(item.model, item.usage, item);
    if (cost > 0) {
      costByIndex.set(idx, cost);
      totalCost += cost;
    }
  });
  return { costByIndex, totalCost };
}

function CodexIntegratedDemo() {
  const items = useMemo(() => makeCodexItems(), []);
  const { costByIndex, totalCost } = useMemo(() => buildCodexCostMap(items), [items]);
  const [selectedIndex, setSelectedIndex] = useState(0);
  const layout = useCodexSegmentLayout(items, selectedIndex);
  const segmentUniqueCounts = useMemo(() => {
    return layout.segments.map((seg) => {
      let n = 0;
      for (let i = seg.startIndex; i <= seg.endIndex; i++) {
        if (items[i]?.kind === 'assistant') n++;
      }
      return n;
    });
  }, [items, layout.segments]);

  return (
    <div style={{ padding: '24px', background: 'var(--color-bg)', minHeight: '100vh' }}>
      <h3 style={{ marginBottom: '16px', color: 'var(--color-text-primary)' }}>CostBar (Codex) — three turns</h3>
      <p style={{ marginBottom: '16px', color: 'var(--color-text-secondary)', fontSize: '14px' }}>
        Same `CostBar` component driven by Codex render-items. Each assistant
        item carries a `usage` block written by `normalizeCodexLines` from
        `event_msg.token_count.info.last_token_usage`.
      </p>
      <div style={{ display: 'flex', gap: '4px', height: '500px', width: '60px' }}>
        <CostBar
          layout={layout}
          costByIndex={costByIndex}
          segmentUniqueCounts={segmentUniqueCounts}
          totalCost={totalCost}
          onSeek={(start) => setSelectedIndex(start)}
        />
      </div>
      <div style={{ marginTop: '12px', fontSize: '12px', color: 'var(--color-text-muted)' }}>
        Total cost: ${totalCost.toFixed(4)} — selected unfiltered index {selectedIndex}
      </div>
    </div>
  );
}

export const CodexIntegrated: Story = {
  render: () => <CodexIntegratedDemo />,
};
