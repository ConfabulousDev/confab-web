import type { Meta, StoryObj } from '@storybook/react-vite';
import { userEvent, within, waitFor } from 'storybook/test';
import CursorTranscriptPane from './CursorTranscriptPane';
import type { CursorRenderItem } from './cursorCategories';

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

export const Loading: Story = {
  args: { sessionId: 'demo', items: [], filteredItems: [], loading: true, error: null },
};

export const Empty: Story = {
  args: { sessionId: 'demo', items: [], filteredItems: [], loading: false, error: null },
};
