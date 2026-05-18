import type { Meta, StoryObj } from '@storybook/react-vite';
import CodexMessageBody from './CodexMessageBody';

const meta: Meta<typeof CodexMessageBody> = {
  title: 'Transcript/Codex/CodexMessageBody',
  component: CodexMessageBody,
};

export default meta;
type Story = StoryObj<typeof CodexMessageBody>;

// Markdown branch (CF-358): non-JSON text flows through the GFM markdown
// pipeline, so bold / inline code / links render as HTML.
export const Markdown: Story = {
  args: {
    text: 'Updated **two** files. The change touches `parseRollout` plus a [test fixture](#).',
  },
};

// JSON branch (CF-358): when the text parses as JSON, render it as a
// syntax-highlighted `CodeBlock` with `language="json"` instead of running it
// through the markdown pipeline.
export const JsonPayload: Story = {
  args: {
    text: '{"action":"run","cmd":"pwd","workdir":"/tmp/proj"}',
  },
};
