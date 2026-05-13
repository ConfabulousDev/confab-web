import type { Meta, StoryObj } from '@storybook/react-vite';
import CodexTranscriptPane from './CodexTranscriptPane';
import type { RawCodexLine } from '@/schemas/codexTranscript';

const meta: Meta<typeof CodexTranscriptPane> = {
  title: 'Session/CodexTranscriptPane',
  component: CodexTranscriptPane,
  parameters: { layout: 'fullscreen' },
};

export default meta;
type Story = StoryObj<typeof CodexTranscriptPane>;

// Minimal fixture covering: session_meta (sets model), one full turn with
// reasoning, exec_command tool call, assistant final, and a task_complete
// turn separator, plus a compacted divider.
const fixtureLines: RawCodexLine[] = [
  {
    timestamp: '2026-05-13T18:00:00Z',
    type: 'session_meta',
    payload: { id: 'fixture', model_provider: 'openai', model: 'gpt-5' },
  },
  {
    timestamp: '2026-05-13T18:00:00.5Z',
    type: 'response_item',
    payload: {
      type: 'message',
      role: 'user',
      content: [{ type: 'input_text', text: 'add the linear mcp to my codex config' }],
    },
  },
  {
    timestamp: '2026-05-13T18:00:01Z',
    type: 'response_item',
    payload: {
      type: 'reasoning',
      summary: [],
      content: null,
      encrypted_content: 'opaque-blob',
    },
  },
  {
    timestamp: '2026-05-13T18:00:02Z',
    type: 'response_item',
    payload: {
      type: 'message',
      role: 'assistant',
      content: [{ type: 'output_text', text: "I'll check how this repo manages MCP entries." }],
      phase: 'commentary',
    },
  },
  {
    timestamp: '2026-05-13T18:00:03Z',
    type: 'response_item',
    payload: {
      type: 'function_call',
      name: 'exec_command',
      arguments: JSON.stringify({ cmd: 'pwd', workdir: '/Users/dev/example-project' }),
      call_id: 'call_pwd',
    },
  },
  {
    timestamp: '2026-05-13T18:00:03.5Z',
    type: 'response_item',
    payload: {
      type: 'function_call_output',
      call_id: 'call_pwd',
      output:
        'Chunk ID: 155fed\nWall time: 0.7 seconds\nProcess exited with code 0\nOriginal token count: 7\nOutput:\n/Users/dev/example-project\n',
    },
  },
  {
    timestamp: '2026-05-13T18:00:11Z',
    type: 'response_item',
    payload: {
      type: 'message',
      role: 'assistant',
      content: [
        {
          type: 'output_text',
          text: 'Added the Linear MCP entry to your Codex config.\n\nReload the session for the change to take effect.',
        },
      ],
      phase: 'final',
    },
  },
  {
    timestamp: '2026-05-13T18:00:11.5Z',
    type: 'event_msg',
    payload: {
      type: 'task_complete',
      turn_id: 't1',
      last_agent_message: 'Added the Linear MCP entry.',
      completed_at: 0,
      duration_ms: 11000,
      time_to_first_token_ms: 1704,
    },
  },
  {
    timestamp: '2026-05-13T18:02:00Z',
    type: 'compacted',
    payload: { message: '', replacement_history: [{ a: 1 }, { b: 2 }] },
  },
];

/**
 * Storybook bypass: `initialRawLines` skips fetch + poll and renders directly.
 */
export const FullSession: Story = {
  render: () => (
    <div style={{ height: '100vh' }}>
      <CodexTranscriptPane
        sessionId="storybook"
        transcriptFileName="rollout.jsonl"
        initialRawLines={fixtureLines}
      />
    </div>
  ),
};

/**
 * Empty raw-lines list — the timeline shows its empty-state placeholder.
 */
export const Empty: Story = {
  render: () => (
    <div style={{ height: '100vh' }}>
      <CodexTranscriptPane
        sessionId="storybook"
        transcriptFileName="rollout.jsonl"
        initialRawLines={[]}
      />
    </div>
  ),
};
