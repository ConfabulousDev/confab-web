import type { Meta, StoryObj } from '@storybook/react-vite';
import CursorMessageBody from './CursorMessageBody';

const meta: Meta<typeof CursorMessageBody> = {
  title: 'Session/CursorMessageBody',
  component: CursorMessageBody,
  decorators: [
    (Story) => (
      <div style={{ maxWidth: '640px', padding: '1rem' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof CursorMessageBody>;

// Assistant Dependabot final-response shape (pt81 acceptance): bold repo name,
// a GFM pipe table, `###` section headers, and a clickable link — all rendered
// as HTML instead of literal monospace source.
export const AssistantDependabot: Story = {
  args: {
    text: [
      'Yes — **ConfabulousDev/confab-web** has **14 open Dependabot alerts**:',
      '',
      '| Severity | Count |',
      '|----------|-------|',
      '| High     | 8     |',
      '| Moderate | 4     |',
      '| Low      | 2     |',
      '',
      '### Next steps',
      '',
      'Review the high-severity alerts first.',
      '',
      'Full list: [github.com/ConfabulousDev/confab-web/security/dependabot](https://github.com/ConfabulousDev/confab-web/security/dependabot)',
    ].join('\n'),
  },
};

// User prompt shape: a markdown list with inline code.
export const UserListWithInlineCode: Story = {
  args: {
    text: [
      'Please do the following:',
      '',
      '- Add validation to `parseSessionId`',
      '- Write a test in `session_handler_test.go`',
      '- Update the `README.md`',
    ].join('\n'),
  },
};

// JSON-shaped narrative text pretty-prints as a syntax-highlighted code block
// (matches CodexMessageBody's JSON-or-markdown fallback).
export const JsonPayload: Story = {
  args: {
    text: '{"action":"run","cmd":"pwd","workdir":"/tmp/proj"}',
  },
};

// Search highlight: matches inside the rendered markdown HTML are wrapped in
// `<mark>` (the active-match class), so Cmd-F + scroll-to-mark keeps working.
export const SearchHighlight: Story = {
  args: {
    text: 'Yes — **ConfabulousDev/confab-web** has 14 open Dependabot alerts.',
    searchQuery: 'Dependabot',
    isCurrentSearchMatch: true,
  },
};
