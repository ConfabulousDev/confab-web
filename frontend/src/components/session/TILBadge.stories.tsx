import type { Meta, StoryObj } from '@storybook/react-vite';
import { MemoryRouter } from 'react-router-dom';
import TILBadge from './TILBadge';
import type { TIL } from '@/schemas/api';

const meta: Meta<typeof TILBadge> = {
  title: 'Session/TILBadge',
  component: TILBadge,
  parameters: { layout: 'centered' },
  decorators: [(Story) => <MemoryRouter><Story /></MemoryRouter>],
};

export default meta;
type Story = StoryObj<typeof TILBadge>;

const baseTIL: TIL = {
  id: 1,
  title: 'Go channels for concurrency',
  summary: 'Learned that Go channels are a first-class concurrency primitive that enable safe communication between goroutines. Use buffered channels for decoupling producer/consumer speeds.',
  session_id: 'session-123',
  message_uuid: 'msg-uuid-1',
  created_at: '2026-03-14T10:00:00Z',
};

export const SingleTIL: Story = {
  args: {
    tils: [baseTIL],
  },
};

export const MultipleTILs: Story = {
  args: {
    tils: [
      baseTIL,
      {
        ...baseTIL,
        id: 2,
        title: 'Context propagation patterns',
        summary: 'context.WithTimeout and context.WithCancel are essential for managing request lifecycles. Always pass context as the first parameter.',
      },
      {
        ...baseTIL,
        id: 3,
        title: 'Error wrapping with fmt.Errorf',
        summary: 'Use %w verb in fmt.Errorf to wrap errors for errors.Is/As compatibility. This enables callers to check for specific error types.',
      },
    ],
  },
};

export const Empty: Story = {
  args: {
    tils: [],
  },
};
