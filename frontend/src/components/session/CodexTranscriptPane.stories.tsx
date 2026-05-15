import type { Meta, StoryObj } from '@storybook/react-vite';
import CodexTranscriptPane from './CodexTranscriptPane';
import type { RawCodexLine } from '@/schemas/codexTranscript';
import { normalizeCodexLines } from '@/services/codexTranscriptService';
import {
  codexItemMatchesFilter,
  DEFAULT_CODEX_FILTER_STATE,
  type CodexFilterState,
} from './codexCategories';

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

// SessionViewer-shaped derivation helper for the stories (mirrors the live
// memo chain in `SessionViewer.tsx`).
function deriveProps(rawLines: RawCodexLine[], filterState: CodexFilterState) {
  const items = normalizeCodexLines(rawLines);
  const filteredItems = items.filter((it) => codexItemMatchesFilter(it, filterState));
  const visibleIndices = new Set<number>();
  items.forEach((it, idx) => {
    if (codexItemMatchesFilter(it, filterState)) visibleIndices.add(idx);
  });
  return { items, filteredItems, visibleIndices };
}

/**
 * Presentational: SessionViewer feeds normalized items from its own fetch
 * (CF-386) and the filter pipeline (CF-361).
 */
export const FullSession: Story = {
  render: () => {
    const { items, filteredItems, visibleIndices } = deriveProps(
      fixtureLines,
      DEFAULT_CODEX_FILTER_STATE,
    );
    return (
      <div style={{ height: '100vh' }}>
        <CodexTranscriptPane
          sessionId="storybook"
          items={items}
          filteredItems={filteredItems}
          visibleIndices={visibleIndices}
          loading={false}
          error={null}
        />
      </div>
    );
  },
};

/**
 * CF-361: filtered view — only `final` assistant messages pass. The timeline
 * bar greys segments whose visible-item count is zero; the row list shrinks
 * to just the matching items.
 */
export const FilteredFinalOnly: Story = {
  render: () => {
    const filterState: CodexFilterState = {
      ...DEFAULT_CODEX_FILTER_STATE,
      user: false,
      assistant: { commentary: false, final: true },
      tool_call: { exec_command: false, apply_patch: false, web_search: false, generic: false },
      compacted: false,
      turn_separator: false,
    };
    const { items, filteredItems, visibleIndices } = deriveProps(fixtureLines, filterState);
    return (
      <div style={{ height: '100vh' }}>
        <CodexTranscriptPane
          sessionId="storybook"
          items={items}
          filteredItems={filteredItems}
          visibleIndices={visibleIndices}
          loading={false}
          error={null}
        />
      </div>
    );
  },
};

/**
 * CF-361: all items filtered out — distinct empty state with the hint.
 */
export const FilteredAllOut: Story = {
  render: () => {
    const filterState: CodexFilterState = {
      user: false,
      assistant: { commentary: false, final: false },
      tool_call: { exec_command: false, apply_patch: false, web_search: false, generic: false },
      reasoning_hidden: false,
      compacted: false,
      turn_separator: false,
      unknown: false,
    };
    const { items, filteredItems, visibleIndices } = deriveProps(fixtureLines, filterState);
    return (
      <div style={{ height: '100vh' }}>
        <CodexTranscriptPane
          sessionId="storybook"
          items={items}
          filteredItems={filteredItems}
          visibleIndices={visibleIndices}
          loading={false}
          error={null}
        />
      </div>
    );
  },
};

/**
 * Empty raw-lines list — the timeline shows its empty-state placeholder.
 */
export const Empty: Story = {
  render: () => (
    <div style={{ height: '100vh' }}>
      <CodexTranscriptPane
        sessionId="storybook"
        items={[]}
        filteredItems={[]}
        loading={false}
        error={null}
      />
    </div>
  ),
};

/**
 * Loading state — the pane just shows a spinner. Owner (SessionViewer) is
 * still fetching the rollout.
 */
export const Loading: Story = {
  render: () => (
    <div style={{ height: '100vh' }}>
      <CodexTranscriptPane
        sessionId="storybook"
        items={[]}
        filteredItems={[]}
        loading={true}
        error={null}
      />
    </div>
  ),
};

/**
 * Error state — fetch in SessionViewer failed.
 */
export const ErrorState: Story = {
  render: () => (
    <div style={{ height: '100vh' }}>
      <CodexTranscriptPane
        sessionId="storybook"
        items={[]}
        filteredItems={[]}
        loading={false}
        error="No transcript file found"
      />
    </div>
  ),
};
