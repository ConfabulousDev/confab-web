import type { Meta, StoryObj } from '@storybook/react-vite';
import CodexUnknownItem from './CodexUnknownItem';

const meta: Meta<typeof CodexUnknownItem> = {
  title: 'Transcript/Codex/CodexUnknownItem',
  component: CodexUnknownItem,
};

export default meta;
type Story = StoryObj<typeof CodexUnknownItem>;

export const FutureTopLevelType: Story = {
  args: {
    item: {
      kind: 'unknown',
      timestamp: '2026-05-13T18:03:00Z',
      rawLine: {
        timestamp: '2026-05-13T18:03:00Z',
        type: 'future_top_level_type',
        payload: { some: 'shape' },
      },
    },
  },
};

export const UnknownResponseItemPayload: Story = {
  args: {
    item: {
      kind: 'unknown',
      timestamp: '2026-05-13T18:03:10Z',
      rawLine: {
        timestamp: '2026-05-13T18:03:10Z',
        type: 'response_item',
        payload: { type: 'future_payload_type', unknown_field: 'unknown_value' },
      },
    },
  },
};
