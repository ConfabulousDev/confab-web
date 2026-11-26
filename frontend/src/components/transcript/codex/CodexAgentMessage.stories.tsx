import type { Meta, StoryObj } from '@storybook/react-vite';
import CodexAgentMessage from './CodexAgentMessage';
import type { CodexAgentMessageItem } from '@/types/codexRenderItem';

// kpqz: inter-agent (parent <-> subagent) message rows.
const meta: Meta<typeof CodexAgentMessage> = {
  title: 'Transcript/Codex/CodexAgentMessage',
  component: CodexAgentMessage,
};

export default meta;
type Story = StoryObj<typeof CodexAgentMessage>;

function item(author: string, recipient: string, text: string): CodexAgentMessageItem {
  return {
    kind: 'agent_message',
    lineId: '0',
    timestamp: '2026-11-25T10:00:00Z',
    author,
    recipient,
    text,
  };
}

// A subagent reporting back to its parent.
export const ChildReplyToParent: Story = {
  args: {
    item: item(
      '/root/implement_feature',
      '/root',
      'FINAL_ANSWER: the **parser** now handles the new record types. All tests pass.',
    ),
  },
};

// In a child session, the first agent message is the parent's task prompt.
export const ParentTaskToChild: Story = {
  args: {
    item: item(
      '/root',
      '/root/implement_feature',
      'Implement the change described below.\n\n- Add the schema\n- Wire the normalizer\n- Update the docs',
    ),
  },
};

// Subagents often answer with a JSON block; it renders syntax-highlighted.
export const JsonReply: Story = {
  args: {
    item: item('/root/review', '/root', '{"status":"done","filesTouched":["src/a.ts","src/b.ts"]}'),
  },
};

// Long agent paths wrap instead of pushing the row past a phone-width screen.
export const LongAgentPaths: Story = {
  args: {
    item: item(
      '/root/a_very_long_subagent_name_for_wrapping/and_a_nested_grandchild_agent',
      '/root/a_very_long_subagent_name_for_wrapping',
      'done',
    ),
  },
};

export const WithRowActions: Story = {
  args: {
    item: item('/root/worker', '/root', 'hover the row to see copy + skip-nav buttons'),
    sessionId: 'story-session',
    onSkipToNext: () => undefined,
    onSkipToPrevious: () => undefined,
  },
};

export const WithDeepLinkTarget: Story = {
  args: {
    item: item('/root/worker', '/root', 'this row is the deep-link target'),
    sessionId: 'story-session',
    isDeepLinkTarget: true,
  },
};
