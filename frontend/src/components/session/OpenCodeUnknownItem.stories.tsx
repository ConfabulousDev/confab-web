import type { Meta, StoryObj } from '@storybook/react-vite';
import OpenCodeUnknownItem from './OpenCodeUnknownItem';

const meta: Meta<typeof OpenCodeUnknownItem> = {
  title: 'Session/OpenCodeUnknownItem',
  component: OpenCodeUnknownItem,
};

export default meta;
type Story = StoryObj<typeof OpenCodeUnknownItem>;

export const UnrecognizedRole: Story = {
  args: {
    item: {
      kind: 'unknown',
      id: 'oc-unknown-0',
      reason: 'unrecognized message role',
      unrecognizedType: 'orchestrator',
      rawLine: { info: { id: 'msg_x', role: 'orchestrator' }, parts: [{ type: 'text' }] },
      timeCreated: 1717689600000,
    },
  },
};

export const UnrecognizedPartType: Story = {
  args: {
    item: {
      kind: 'unknown',
      id: 'oc-unknown-part-1-2',
      reason: 'unrecognized part type',
      unrecognizedType: 'future_part_type',
      rawLine: { type: 'future_part_type', some: 'shape' },
      timeCreated: 1717689600000,
    },
  },
};

export const MalformedLine: Story = {
  args: {
    item: {
      kind: 'unknown',
      id: 'oc-unknown-3',
      reason: 'malformed line',
      unrecognizedType: '(unparseable)',
      rawLine: '{not valid json',
      timeCreated: 0,
    },
  },
};
