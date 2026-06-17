import type { Meta, StoryObj } from '@storybook/react-vite';
import { userEvent, within, waitFor } from 'storybook/test';
import CursorTranscriptPane from './CursorTranscriptPane';
import type { CursorRenderItem } from './cursorCategories';
import {
  parseCursorJSONL,
  normalizeCursorLines,
} from '@/services/cursorTranscriptService';

const meta: Meta<typeof CursorTranscriptPane> = {
  title: 'Session/CursorTranscriptPane',
  component: CursorTranscriptPane,
  parameters: { layout: 'fullscreen' },
  decorators: [
    (Story) => (
      <div style={{ height: '600px', width: '720px', border: '1px solid var(--color-border)' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof CursorTranscriptPane>;

// Items mirror the real Cursor wire fixture: a user prompt, an assistant turn
// with text + tool calls (Read / Grep), then a follow-up. Tool rows show the
// call only (Cursor records inputs, never outputs).
const items: CursorRenderItem[] = [
  { kind: 'user', id: '0', text: 'Add input validation to the session handler and write a test for it.' },
  {
    kind: 'assistant',
    id: '1',
    text: 'Reading the handler and searching for the existing validation helpers.',
  },
  { kind: 'tool', id: '1-0', toolName: 'Read', input: 'internal/api/session_handler.go' },
  { kind: 'tool', id: '1-1', toolName: 'Grep', input: 'func validate' },
  {
    kind: 'assistant',
    id: '2',
    text: 'Found the validation seam. Editing the handler to reject empty session ids.',
  },
  { kind: 'tool', id: '2-0', toolName: 'StrReplace', input: 'internal/api/session_handler.go' },
];

export const Default: Story = {
  args: { sessionId: 'demo', items, filteredItems: items, loading: false, error: null },
};

// Tool rows filtered out.
export const Filtered: Story = {
  args: {
    sessionId: 'demo',
    items,
    filteredItems: items.filter((it) => it.kind !== 'tool'),
    loading: false,
    error: null,
  },
};

export const SearchActive: Story = {
  args: { sessionId: 'demo', items, filteredItems: items, loading: false, error: null },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.keyboard('{Meta>}f{/Meta}');
    const input = await canvas.findByLabelText('Search transcript');
    await userEvent.type(input, 'validation');
    await waitFor(() => {
      if (canvasElement.querySelectorAll('mark').length === 0) {
        throw new Error('no highlight yet');
      }
    });
  },
  parameters: {
    docs: {
      description: {
        story:
          'Cmd-F opens the shared transcript search bar (useTranscriptSearch). Typing a query highlights matches inline across user / assistant / tool rows.',
      },
    },
  },
};

// fa3h: Cursor's on-disk JSONL appends a bare `[REDACTED]` to nearly every
// assistant turn — as a trailing suffix after narrative, or as the entire text
// block on tool-only turns. Normalizing these raw lines strips the placeholder:
// the first assistant row shows narrative only, and the second (text was only
// `[REDACTED]`) drops its assistant bubble while its Shell tool row still
// renders. A Confab CLI `[REDACTED:TYPE]` marker is a different contract and
// stays visible.
const redactedRawJSONL = [
  JSON.stringify({
    role: 'user',
    message: { content: [{ type: 'text', text: 'Check for open Dependabot alerts.' }] },
  }),
  JSON.stringify({
    role: 'assistant',
    message: {
      content: [
        {
          type: 'text',
          text: 'Checking the repo for open Dependabot alerts via the GitHub CLI.\n\n[REDACTED]',
        },
        { type: 'tool_use', name: 'Shell', input: { command: 'gh api dependabot/alerts' } },
      ],
    },
  }),
  JSON.stringify({
    role: 'assistant',
    message: {
      content: [
        { type: 'text', text: '[REDACTED]' },
        { type: 'tool_use', name: 'Shell', input: { command: 'gh pr list --state open' } },
      ],
    },
  }),
  JSON.stringify({
    role: 'assistant',
    message: {
      content: [{ type: 'text', text: 'The token [REDACTED:GITHUB_TOKEN] is set in the env.' }],
    },
  }),
].join('\n');

const redactedItems = normalizeCursorLines(parseCursorJSONL(redactedRawJSONL).rawLines);

export const RedactedStripped: Story = {
  args: {
    sessionId: 'demo',
    items: redactedItems,
    filteredItems: redactedItems,
    loading: false,
    error: null,
  },
  parameters: {
    docs: {
      description: {
        story:
          "Cursor's native bare `[REDACTED]` is stripped during normalize (fa3h): trailing suffixes vanish, a `[REDACTED]`-only turn drops its assistant bubble but keeps its tool row, and a Confab CLI `[REDACTED:TYPE]` marker stays visible.",
      },
    },
  },
};

export const Loading: Story = {
  args: { sessionId: 'demo', items: [], filteredItems: [], loading: true, error: null },
};

export const Empty: Story = {
  args: { sessionId: 'demo', items: [], filteredItems: [], loading: false, error: null },
};
