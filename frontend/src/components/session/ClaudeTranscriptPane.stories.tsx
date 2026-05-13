import type { Meta, StoryObj } from '@storybook/react-vite';
import { MemoryRouter } from 'react-router-dom';
import ClaudeTranscriptPane from './ClaudeTranscriptPane';
import type { TranscriptLine } from '@/types';

const meta: Meta<typeof ClaudeTranscriptPane> = {
  title: 'Session/ClaudeTranscriptPane',
  component: ClaudeTranscriptPane,
  parameters: { layout: 'fullscreen' },
  decorators: [
    // ClaudeTranscriptPane renders MessageTimeline which uses router hooks
    // (TimelineMessage's copy-link button calls useLocation).
    (Story) => (
      <MemoryRouter>
        <div style={{ height: '100vh' }}>
          <Story />
        </div>
      </MemoryRouter>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof ClaudeTranscriptPane>;

const sampleMessages: TranscriptLine[] = [
  {
    type: 'user',
    uuid: 'u1',
    parentUuid: null,
    timestamp: '2026-05-13T18:00:00Z',
    isSidechain: false,
    userType: 'external',
    cwd: '/proj',
    sessionId: 'storybook',
    version: '1.0.0',
    message: { role: 'user', content: 'Hello!' },
  },
  {
    type: 'assistant',
    uuid: 'a1',
    parentUuid: 'u1',
    timestamp: '2026-05-13T18:00:02Z',
    isSidechain: false,
    userType: 'external',
    cwd: '/proj',
    sessionId: 'storybook',
    version: '1.0.0',
    requestId: 'req-a1',
    message: {
      model: 'claude-sonnet-4-20250514',
      id: 'msg-a1',
      type: 'message',
      role: 'assistant',
      content: [{ type: 'text', text: 'Hello! How can I help?' }],
      stop_reason: 'end_turn',
      stop_sequence: null,
      usage: { input_tokens: 100, output_tokens: 20 },
    },
  },
];

export const Loaded: Story = {
  args: {
    loading: false,
    error: null,
    filteredMessages: sampleMessages,
    allMessages: sampleMessages,
    sessionId: 'storybook',
    isCostMode: false,
    tilsByMessageUuid: new Map(),
  },
};

export const Loading: Story = {
  args: {
    loading: true,
    error: null,
    filteredMessages: [],
    allMessages: [],
    sessionId: 'storybook',
    isCostMode: false,
    tilsByMessageUuid: new Map(),
  },
};

export const ErrorState: Story = {
  args: {
    loading: false,
    error: 'Failed to load transcript: 404 Not Found',
    filteredMessages: [],
    allMessages: [],
    sessionId: 'storybook',
    isCostMode: false,
    tilsByMessageUuid: new Map(),
  },
};
