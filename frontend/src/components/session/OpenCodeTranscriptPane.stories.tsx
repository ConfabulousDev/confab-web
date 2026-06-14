import type { Meta, StoryObj } from '@storybook/react-vite';
import { userEvent, within, waitFor } from 'storybook/test';
import OpenCodeTranscriptPane from './OpenCodeTranscriptPane';
import type { OpenCodeRenderItem } from './opencodeCategories';

const meta: Meta<typeof OpenCodeTranscriptPane> = {
  title: 'Session/OpenCodeTranscriptPane',
  component: OpenCodeTranscriptPane,
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
type Story = StoryObj<typeof OpenCodeTranscriptPane>;

const items: OpenCodeRenderItem[] = [
  { kind: 'user', id: 'msg_1', text: 'Find all Go files and count the lines.', timeCreated: 1717689500000 },
  {
    kind: 'assistant',
    id: 'msg_2',
    text: "I'll search for Go files and tally their line counts.",
    reasoning: 'The user wants a count across *.go files. Use Glob then read each.',
    model: 'claude-sonnet-4-20250514',
    cost: 0.0152,
    usage: { input: 10000, output: 5000, cacheWrite: 2000, cacheWrite1h: 0, cacheRead: 3000 },
    timeCreated: 1717689600000,
  },
  {
    kind: 'tool',
    id: 'prt_3',
    toolName: 'Glob',
    status: 'completed',
    input: '**/*.go',
    output: 'main.go\ninternal/server.go\ninternal/db.go',
    timeCreated: 1717689601000,
  },
  {
    kind: 'tool',
    id: 'prt_4',
    toolName: 'Bash',
    status: 'error',
    input: 'wc -l *.go',
    output: 'wc: *.go: No such file or directory',
    timeCreated: 1717689602000,
  },
  {
    kind: 'assistant',
    id: 'msg_5',
    text: 'Found 3 Go files. Let me count lines with the correct paths.',
    model: 'gpt-4o',
    cost: 0.004,
    usage: { input: 6000, output: 1200, cacheWrite: 0, cacheWrite1h: 0, cacheRead: 2000 },
    timeCreated: 1717689603000,
  },
];

export const Default: Story = {
  args: { sessionId: 'demo', items, filteredItems: items, loading: false, error: null },
};

// Tools filtered out: the bar greys segments whose whole range is hidden.
export const Filtered: Story = {
  args: {
    sessionId: 'demo',
    items,
    filteredItems: items.filter((it) => it.kind !== 'tool'),
    loading: false,
    error: null,
  },
};

export const CostMode: Story = {
  args: { sessionId: 'demo', items, filteredItems: items, loading: false, error: null, isCostMode: true },
};

// 5p9j: opens the Cmd-F search bar and types a query that matches across row
// kinds (user prompt "Go" + assistant body). Visual regression cover for the
// highlight + match-count wiring.
export const SearchActive: Story = {
  args: { sessionId: 'demo', items, filteredItems: items, loading: false, error: null },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    // Cmd-F opens the shared search bar (document-level keydown intercept).
    await userEvent.keyboard('{Meta>}f{/Meta}');
    const input = await canvas.findByLabelText('Search transcript');
    await userEvent.type(input, 'Go');
    // Wait for the debounced highlight to land.
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
          '5p9j — the OpenCode transcript search bar opens via Cmd-F (shared useTranscriptSearch toolkit). Typing a query highlights matches inline across user / assistant / tool rows; the active match has an amber ring.',
      },
    },
  },
};

// 5p9j: a query whose only match lives inside a collapsed <details> (the
// assistant reasoning) — decision 5 force-opens that <details> so the counted
// match is visible.
export const SearchInsideDetails: Story = {
  args: { sessionId: 'demo', items, filteredItems: items, loading: false, error: null },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.keyboard('{Meta>}f{/Meta}');
    const input = await canvas.findByLabelText('Search transcript');
    // "Glob" only appears inside the assistant's reasoning text.
    await userEvent.type(input, 'Glob');
    await waitFor(() => {
      const open = canvasElement.querySelector('details[open]');
      if (!open) throw new Error('details not yet force-open');
    });
  },
  parameters: {
    docs: {
      description: {
        story:
          '5p9j (decision 5) — a match inside a collapsed reasoning / tool-output <details> force-opens that section so the search bar never counts a match the user cannot see.',
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
