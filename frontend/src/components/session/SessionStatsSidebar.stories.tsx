import type { Meta, StoryObj } from '@storybook/react-vite';
import type { TranscriptLine, AssistantMessage, SystemMessage } from '@/types';
import { KeyboardShortcutProvider } from '@/contexts/KeyboardShortcutContext';
import SessionStatsSidebar from './SessionStatsSidebar';

// Helper to create a minimal assistant message with token usage
function createAssistantMessage(
  uuid: string,
  inputTokens: number,
  outputTokens: number,
  cacheCreated = 0,
  cacheRead = 0
): AssistantMessage {
  return {
    type: 'assistant',
    uuid,
    timestamp: new Date().toISOString(),
    parentUuid: null,
    isSidechain: false,
    userType: 'external',
    cwd: '/test',
    sessionId: 'test-session',
    version: '1.0.0',
    requestId: `req-${uuid}`,
    message: {
      model: 'claude-opus-4-5-20251101',
      id: `msg-${uuid}`,
      type: 'message',
      role: 'assistant',
      content: [{ type: 'text', text: 'Test response' }],
      stop_reason: 'end_turn',
      stop_sequence: null,
      usage: {
        input_tokens: inputTokens,
        output_tokens: outputTokens,
        cache_creation_input_tokens: cacheCreated,
        cache_read_input_tokens: cacheRead,
      },
    },
  };
}

// Sample messages with various token usages
const sampleMessages: TranscriptLine[] = [
  createAssistantMessage('1', 15000, 2500, 5000, 0),
  createAssistantMessage('2', 18000, 3200, 0, 5000),
  createAssistantMessage('3', 22000, 4100, 8000, 5000),
  createAssistantMessage('4', 25000, 5000, 0, 13000),
  createAssistantMessage('5', 30000, 6000, 10000, 13000),
];

// Empty session (no messages)
const emptyMessages: TranscriptLine[] = [];

// Small session with low token counts
const smallSessionMessages: TranscriptLine[] = [
  createAssistantMessage('1', 500, 150, 200, 0),
  createAssistantMessage('2', 600, 200, 0, 200),
];

// Helper to create a compact_boundary system message
function createCompactBoundary(
  uuid: string,
  trigger: 'auto' | 'manual',
  preTokens = 150000
): SystemMessage {
  return {
    type: 'system',
    uuid,
    timestamp: new Date().toISOString(),
    parentUuid: null,
    logicalParentUuid: 'prev-uuid',
    isSidechain: false,
    userType: 'external',
    cwd: '/test',
    sessionId: 'test-session',
    version: '1.0.0',
    subtype: 'compact_boundary',
    content: 'Conversation compacted',
    isMeta: false,
    level: 'info',
    compactMetadata: { trigger, preTokens },
  };
}

// Session with compaction events
const sessionWithCompactions: TranscriptLine[] = [
  createAssistantMessage('1', 50000, 5000, 20000, 0),
  createCompactBoundary('2', 'auto', 150000),
  createAssistantMessage('3', 45000, 4500, 0, 20000),
  createAssistantMessage('4', 48000, 5200, 15000, 20000),
  createCompactBoundary('5', 'auto', 160000),
  createAssistantMessage('6', 42000, 4000, 0, 35000),
  createCompactBoundary('7', 'manual', 145000),
  createAssistantMessage('8', 40000, 3800, 10000, 35000),
];

// Long session with many compactions
const longSessionWithManyCompactions: TranscriptLine[] = [
  createAssistantMessage('1', 50000, 5000, 20000, 0),
  createCompactBoundary('2', 'auto'),
  createAssistantMessage('3', 45000, 4500, 0, 20000),
  createCompactBoundary('4', 'auto'),
  createAssistantMessage('5', 48000, 5200, 15000, 20000),
  createCompactBoundary('6', 'auto'),
  createAssistantMessage('7', 42000, 4000, 0, 35000),
  createCompactBoundary('8', 'manual'),
  createAssistantMessage('9', 40000, 3800, 10000, 35000),
  createCompactBoundary('10', 'auto'),
  createAssistantMessage('11', 38000, 3500, 0, 45000),
  createCompactBoundary('12', 'manual'),
  createAssistantMessage('13', 35000, 3200, 8000, 45000),
];

const meta = {
  title: 'Session/SessionStatsSidebar',
  component: SessionStatsSidebar,
  parameters: {
    layout: 'fullscreen',
  },
  decorators: [
    (Story) => (
      <KeyboardShortcutProvider>
        <div style={{ display: 'flex', height: '100vh', background: '#fafafa' }}>
          <Story />
          <div style={{ flex: 1, padding: '24px', color: '#666' }}>
            Main content area (press Cmd+Shift+E to toggle debug stats)
          </div>
        </div>
      </KeyboardShortcutProvider>
    ),
  ],
} satisfies Meta<typeof SessionStatsSidebar>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    messages: sampleMessages,
  },
};

export const EmptySession: Story = {
  args: {
    messages: emptyMessages,
  },
};

export const SmallSession: Story = {
  args: {
    messages: smallSessionMessages,
  },
};

export const Loading: Story = {
  args: {
    messages: [],
    loading: true,
  },
};

export const WithCompactions: Story = {
  args: {
    messages: sessionWithCompactions,
  },
};

export const ManyCompactions: Story = {
  args: {
    messages: longSessionWithManyCompactions,
  },
};
