import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState, useMemo } from 'react';
import { CostBar } from './CostBar';
import { TimelineBar } from './TimelineBar';
import type { TranscriptLine, UserMessage, AssistantMessage } from '@/types';
import { calculateMessageCost } from '@/utils/tokenStats';

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
    const cost = calculateMessageCost(messages[i]!);
    if (cost > 0) {
      messageCosts.set(i, cost);
      totalCost += cost;
    }
  }
  return { messageCosts, totalCost };
}

/**
 * Side-by-side with TimelineBar, showing both bars as they appear in the real UI.
 */
function IntegratedDemo() {
  const messages = useMemo(() => createConversation(), []);
  const { messageCosts, totalCost } = useMemo(() => buildCostMap(messages), [messages]);
  const [selectedIndex, setSelectedIndex] = useState(0);

  return (
    <div style={{ padding: '24px', background: 'var(--color-bg)', minHeight: '100vh' }}>
      <h3 style={{ marginBottom: '16px', color: 'var(--color-text-primary)' }}>CostBar + TimelineBar Side-by-Side</h3>
      <p style={{ marginBottom: '16px', color: 'var(--color-text-secondary)', fontSize: '14px' }}>
        Left: cost heatmap (red intensity = relative cost). Right: speaker timeline.
      </p>

      <div style={{ display: 'flex', gap: '4px', height: '500px', width: '60px' }}>
        <CostBar
          messages={messages}
          messageCosts={messageCosts}
          totalCost={totalCost}
          selectedIndex={selectedIndex}
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

  return (
    <div style={{ padding: '24px', background: 'var(--color-bg)', minHeight: '100vh' }}>
      <h3 style={{ marginBottom: '16px', color: 'var(--color-text-primary)' }}>CostBar — Isolated</h3>

      <div style={{ display: 'flex', gap: '24px', alignItems: 'flex-start' }}>
        <div style={{ height: '400px', width: '40px', padding: '0 8px' }}>
          <CostBar
            messages={messages}
            messageCosts={messageCosts}
            totalCost={totalCost}
            selectedIndex={selectedIndex}
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
export const ZeroCost: Story = {
  args: {
    messages: [],
    messageCosts: new Map(),
    totalCost: 0,
    selectedIndex: 0,
    onSeek: () => { /* no-op */ },
  },
  decorators: [
    (Story) => (
      <div style={{ height: '400px', padding: '24px' }}>
        <Story />
        <p style={{ marginTop: '16px', color: '#666' }}>
          (Nothing renders when total cost is zero)
        </p>
      </div>
    ),
  ],
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

  return (
    <div style={{ padding: '24px', background: 'var(--color-bg)', minHeight: '100vh' }}>
      <h3 style={{ marginBottom: '16px', color: 'var(--color-text-primary)' }}>Single Expensive Turn</h3>
      <div style={{ display: 'flex', gap: '4px', height: '400px', width: '60px' }}>
        <CostBar
          messages={messages}
          messageCosts={messageCosts}
          totalCost={totalCost}
          selectedIndex={selectedIndex}
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
