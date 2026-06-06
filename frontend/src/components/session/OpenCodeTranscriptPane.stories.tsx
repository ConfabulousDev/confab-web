import type { Meta, StoryObj } from '@storybook/react-vite';
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
    usage: { input: 10000, output: 5000, cacheWrite: 2000, cacheRead: 3000 },
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
    usage: { input: 6000, output: 1200, cacheWrite: 0, cacheRead: 2000 },
    timeCreated: 1717689603000,
  },
];

export const Default: Story = {
  args: { sessionId: 'demo', items, filteredItems: items, loading: false, error: null },
};

export const CostMode: Story = {
  args: { sessionId: 'demo', items, filteredItems: items, loading: false, error: null, isCostMode: true },
};

export const Loading: Story = {
  args: { sessionId: 'demo', items: [], filteredItems: [], loading: true, error: null },
};

export const Empty: Story = {
  args: { sessionId: 'demo', items: [], filteredItems: [], loading: false, error: null },
};
