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
      lineId: '0',
      timestamp: '2026-05-13T18:03:00Z',
      rawLine: {
        timestamp: '2026-05-13T18:03:00Z',
        type: 'future_top_level_type',
        payload: { some: 'shape' },
      },
      reason: 'unknown-line-type',
      unrecognizedType: 'future_top_level_type',
    },
  },
};

export const UnknownResponseItemPayload: Story = {
  args: {
    item: {
      kind: 'unknown',
      lineId: '0',
      timestamp: '2026-05-13T18:03:10Z',
      rawLine: {
        timestamp: '2026-05-13T18:03:10Z',
        type: 'response_item',
        payload: { type: 'future_payload_type', unknown_field: 'unknown_value' },
      },
      reason: 'unknown-response-payload',
      unrecognizedType: 'future_payload_type',
    },
  },
};

// CF-574: with the "Report this message" affordance (needs a sessionId for the
// per-row actions to render alongside it).
export const WithReportButton: Story = {
  args: {
    item: {
      kind: 'unknown',
      lineId: '0',
      timestamp: '2026-05-13T18:03:20Z',
      rawLine: {
        timestamp: '2026-05-13T18:03:20Z',
        type: 'response_item',
        payload: { type: 'future_payload_type', unknown_field: 'unknown_value' },
      },
      reason: 'unknown-response-payload',
      unrecognizedType: 'future_payload_type',
    },
    sessionId: 'story-session',
  },
};

// CF-360: deep-link landing variant — pulse ring runs even on the
// <details>-based unknown row.
export const WithDeepLinkTarget: Story = {
  args: {
    item: {
      kind: 'unknown',
      lineId: '0',
      timestamp: '2026-05-13T18:03:00Z',
      rawLine: {
        timestamp: '2026-05-13T18:03:00Z',
        type: 'future_top_level_type',
        payload: { some: 'shape' },
      },
      reason: 'unknown-line-type',
      unrecognizedType: 'future_top_level_type',
    },
    sessionId: 'story-session',
    isDeepLinkTarget: true,
  },
};
