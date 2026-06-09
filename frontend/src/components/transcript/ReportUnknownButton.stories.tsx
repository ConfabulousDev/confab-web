import type { Meta, StoryObj } from '@storybook/react-vite';
import ReportUnknownButton from './ReportUnknownButton';

const meta: Meta<typeof ReportUnknownButton> = {
  title: 'Transcript/ReportUnknownButton',
  component: ReportUnknownButton,
};

export default meta;
type Story = StoryObj<typeof ReportUnknownButton>;

export const ClaudeMessage: Story = {
  args: {
    descriptor: {
      provider: 'claude',
      surface: 'message',
      type: 'queue-operation-v2',
      keyFingerprint: ['type', 'timestamp', 'uuid', 'operation'],
    },
  },
};

export const ClaudeContentBlock: Story = {
  args: {
    descriptor: {
      provider: 'claude',
      surface: 'content block',
      type: 'tool_progress',
      keyFingerprint: ['type', 'id', 'progress'],
    },
  },
};

export const CodexLine: Story = {
  args: {
    descriptor: {
      provider: 'codex',
      surface: 'line',
      type: 'future_payload_type',
      reason: 'unrecognized response_item payload type',
      keyFingerprint: ['type', 'payload', 'payload.type', 'payload.call_id'],
    },
  },
};

export const OpenCodeLine: Story = {
  args: {
    descriptor: {
      provider: 'opencode',
      surface: 'line',
      type: 'orchestrator',
      reason: 'unrecognized message role',
      keyFingerprint: ['info', 'info.role', 'parts'],
    },
  },
};
