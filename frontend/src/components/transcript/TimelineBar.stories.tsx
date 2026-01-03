import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState, useRef, useCallback, useMemo, useEffect } from 'react';
import { TimelineBar } from './TimelineBar';
import type { TranscriptLine, UserMessage, AssistantMessage } from '@/types';

const meta: Meta<typeof TimelineBar> = {
  title: 'Transcript/TimelineBar',
  component: TimelineBar,
  parameters: {
    layout: 'fullscreen',
  },
};

export default meta;
type Story = StoryObj<typeof TimelineBar>;

// Base message template
const baseMessage = {
  parentUuid: null,
  isSidechain: false,
  userType: 'external',
  cwd: '/project',
  sessionId: 'test-session',
  version: '1.0.0',
};

// Helper to create user messages
function createUserMessage(uuid: string, timestamp: string, content: string): UserMessage {
  return {
    ...baseMessage,
    type: 'user',
    uuid,
    timestamp,
    message: { role: 'user', content },
  };
}

// Helper to create assistant messages
function createAssistantMessage(uuid: string, timestamp: string, text: string): AssistantMessage {
  return {
    ...baseMessage,
    type: 'assistant',
    uuid,
    timestamp,
    requestId: `req-${uuid}`,
    message: {
      model: 'claude-sonnet-4',
      id: `msg-${uuid}`,
      type: 'message',
      role: 'assistant',
      content: [{ type: 'text', text }],
      stop_reason: 'end_turn',
      stop_sequence: null,
      usage: { input_tokens: 100, output_tokens: 50 },
    },
  };
}

// Create a realistic long conversation with many messages
function createLargeConversation(): TranscriptLine[] {
  const messages: TranscriptLine[] = [];
  let currentTime = new Date('2025-01-01T10:00:00Z').getTime();
  let msgIndex = 0;

  // 15 turns with varying message counts and durations
  for (let turn = 0; turn < 15; turn++) {
    // User prompt
    messages.push(createUserMessage(
      `u${msgIndex++}`,
      new Date(currentTime).toISOString(),
      `User message for turn ${turn + 1}: ${getRandomPrompt(turn)}`
    ));

    // Random Claude work time (5-90 seconds)
    const workTimeMs = 5000 + Math.random() * 85000;

    // Multiple assistant messages per turn (1-5 messages simulating tool calls)
    const assistantMsgCount = 1 + Math.floor(Math.random() * 5);
    const timePerMsg = workTimeMs / assistantMsgCount;

    for (let i = 0; i < assistantMsgCount; i++) {
      currentTime += timePerMsg;
      messages.push(createAssistantMessage(
        `a${msgIndex++}`,
        new Date(currentTime).toISOString(),
        `Response ${i + 1}/${assistantMsgCount} for turn ${turn + 1}: ${getRandomResponse(i)}`
      ));
    }

    // Random user thinking time (3-45 seconds)
    currentTime += 3000 + Math.random() * 42000;
  }

  return messages;
}

const PROMPTS = [
  'Help me refactor this function to be more readable',
  'Can you add unit tests for the UserService class?',
  'Explain how the authentication flow works',
  'Fix the bug in the payment processing module',
  'Update the API to use the new schema',
  'Add error handling to the file upload feature',
  'Optimize the database queries for better performance',
  'Create a new component for displaying charts',
  'Review this PR and suggest improvements',
  'Help me debug this failing test',
  'Add logging to track user actions',
  'Implement caching for the search results',
  'Update the documentation for the API endpoints',
  'Add validation for the form inputs',
  'Refactor the state management logic',
];

const RESPONSES = [
  "I'll help you with that. Let me first examine the codebase...",
  'Reading the file to understand the current implementation...',
  'Found the issue. Making the necessary changes...',
  'Running the tests to verify the fix...',
  'All tests pass. Here is the updated code.',
  'I have completed the implementation.',
];

function getRandomPrompt(turn: number): string {
  const idx = turn % PROMPTS.length;
  const prompt = PROMPTS[idx];
  return prompt !== undefined ? prompt : 'Help me with this task';
}

function getRandomResponse(msgIndex: number): string {
  const idx = msgIndex % RESPONSES.length;
  const response = RESPONSES[idx];
  return response !== undefined ? response : 'Working on it...';
}

// Integrated demo with actual scrollable list
function IntegratedDemo() {
  // Memoize to prevent regeneration on re-render (random data would change)
  const messages = useMemo(() => createLargeConversation(), []);
  const scrollRef = useRef<HTMLDivElement>(null);
  // Separate state: firstVisibleIndex for scroll tracking, selectedIndex for explicit selection
  const [firstVisibleIndex, setFirstVisibleIndex] = useState(0);
  const [selectedIndex, setSelectedIndex] = useState<number | null>(null);
  const [highlightedIndex, setHighlightedIndex] = useState<number | null>(null);

  // Track first visible message on scroll (fallback when nothing explicitly selected)
  const handleScroll = useCallback(() => {
    if (!scrollRef.current) return;
    const container = scrollRef.current;
    const containerRect = container.getBoundingClientRect();

    // Find first message element in view
    const messageElements = container.querySelectorAll('[data-index]');
    for (const el of messageElements) {
      const rect = el.getBoundingClientRect();
      if (rect.top >= containerRect.top - 50) {
        const index = parseInt(el.getAttribute('data-index') || '0', 10);
        setFirstVisibleIndex(index);
        return;
      }
    }
  }, []);

  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    el.addEventListener('scroll', handleScroll, { passive: true });
    return () => el.removeEventListener('scroll', handleScroll);
  }, [handleScroll]);

  // Handle seeking from timeline bar - sets selected message explicitly
  const handleSeek = useCallback((startIndex: number, endIndex: number) => {
    const el = scrollRef.current;
    if (!el) return;

    // Try each index in the range until we find one
    for (let i = startIndex; i <= endIndex; i++) {
      const messageEl = el.querySelector(`[data-index="${i}"]`);
      if (messageEl) {
        messageEl.scrollIntoView({ behavior: 'smooth', block: 'start' });
        setSelectedIndex(i); // Explicit selection drives position indicator
        setHighlightedIndex(i);
        // Clear highlight after animation
        setTimeout(() => setHighlightedIndex(null), 1500);
        return;
      }
    }
  }, []);

  // Selected message drives position, falls back to first visible
  const effectiveSelectedIndex = selectedIndex ?? firstVisibleIndex;

  // All message indices are visible (no filtering in this demo)
  const visibleIndices = useMemo(() => {
    const set = new Set<number>();
    messages.forEach((_, idx) => set.add(idx));
    return set;
  }, [messages]);

  return (
    <div style={{
      display: 'flex',
      height: '100vh',
      background: '#fafafa',
      fontFamily: 'system-ui, -apple-system, sans-serif',
    }}>
      {/* Message list */}
      <div
        ref={scrollRef}
        style={{
          flex: 1,
          overflow: 'auto',
          padding: '16px 24px',
        }}
      >
        <div style={{ marginBottom: '16px', color: '#666', fontSize: '14px' }}>
          <strong>{messages.length} messages</strong> — Click on the timeline bar to jump to a segment
        </div>
        {messages.map((msg, index) => {
          // Only render user and assistant messages
          if (msg.type !== 'user' && msg.type !== 'assistant') return null;

          const content = msg.type === 'user'
            ? (typeof msg.message.content === 'string' ? msg.message.content : '[complex content]')
            : msg.message.content[0]?.type === 'text'
              ? msg.message.content[0].text
              : '[tool use]';

          return (
            <div
              key={msg.uuid}
              data-index={index}
              onMouseEnter={() => setSelectedIndex(index)}
              style={{
                padding: '12px 16px',
                marginBottom: '8px',
                background: highlightedIndex === index ? '#fff3cd' : '#fff',
                border: '1px solid #e5e5e5',
                borderRadius: '6px',
                borderLeft: msg.type === 'user' ? '3px solid #0066cc' : '3px solid #E57B3A',
                boxShadow: selectedIndex === index ? 'inset 0 0 0 2px #bbb' : 'none',
                transition: 'background 0.2s ease, box-shadow 0.2s ease',
                cursor: 'pointer',
              }}
            >
              <div style={{
                fontSize: '11px',
                color: msg.type === 'user' ? '#0066cc' : '#E57B3A',
                fontWeight: 600,
                textTransform: 'uppercase',
                marginBottom: '4px',
              }}>
                {msg.type === 'user' ? 'User' : 'Claude'}
                <span style={{
                  marginLeft: '8px',
                  color: '#999',
                  fontWeight: 400,
                  textTransform: 'none',
                }}>
                  {new Date(msg.timestamp).toLocaleTimeString()}
                </span>
              </div>
              <div style={{ fontSize: '14px', color: '#333' }}>
                {content}
              </div>
            </div>
          );
        })}
      </div>

      {/* Timeline bar - positioned with room for tooltip on left */}
      <div style={{
        width: '40px',
        padding: '16px 8px 16px 8px',
        background: '#f5f5f5',
        borderLeft: '1px solid #e5e5e5',
        position: 'relative',
      }}>
        <TimelineBar
          messages={messages}
          selectedIndex={effectiveSelectedIndex}
          visibleIndices={visibleIndices}
          onSeek={handleSeek}
        />
      </div>
    </div>
  );
}

// Small demo for isolated component testing
function SmallDemo() {
  // Memoize to prevent regeneration on re-render (random data would change)
  const messages = useMemo(() => createLargeConversation().slice(0, 20), []);
  const [selectedIndex, setSelectedIndex] = useState(0);

  return (
    <div style={{ padding: '24px', background: '#fafafa', minHeight: '100vh' }}>
      <h3 style={{ marginBottom: '16px' }}>Timeline Bar - Isolated Test</h3>
      <p style={{ marginBottom: '16px', color: '#666', fontSize: '14px' }}>
        Hover over segments to see duration and message count.
        Use the slider to simulate scroll position.
      </p>

      <div style={{ display: 'flex', gap: '24px', alignItems: 'flex-start' }}>
        <div style={{
          height: '400px',
          width: '200px',
          background: '#f5f5f5',
          borderRadius: '4px',
          padding: '8px 8px 8px 160px',
          position: 'relative',
        }}>
          <TimelineBar
            messages={messages}
            selectedIndex={selectedIndex}
            onSeek={(startIndex) => {
              console.log('Seek to message:', startIndex);
              setSelectedIndex(startIndex);
            }}
          />
        </div>

        <div>
          <label style={{ display: 'block', marginBottom: '8px', fontSize: '14px' }}>
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
    </div>
  );
}

/**
 * Full integrated demo with scrollable message list.
 * - Scroll the message list to see the position indicator move
 * - Click on timeline segments to jump to that part of the conversation
 * - Hover over segments to see duration and message count
 */
export const Integrated: Story = {
  render: () => <IntegratedDemo />,
};

/**
 * Isolated component with manual scroll control.
 * Good for testing hover tooltips and click behavior.
 */
export const Isolated: Story = {
  render: () => <SmallDemo />,
};

/**
 * Empty state - no messages
 */
export const Empty: Story = {
  args: {
    messages: [],
    selectedIndex: 0,
    onSeek: () => { /* no-op */ },
  },
  decorators: [
    (Story) => (
      <div style={{ height: '400px', padding: '24px' }}>
        <Story />
        <p style={{ marginTop: '16px', color: '#666' }}>
          (Nothing renders when there are no segments)
        </p>
      </div>
    ),
  ],
};
