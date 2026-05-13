import type { Meta, StoryObj } from '@storybook/react-vite';
import AwaySummary from './AwaySummary';
import type { SystemMessage } from '@/types';

const meta: Meta<typeof AwaySummary> = {
  title: 'Transcript/Attachments/AwaySummary',
  component: AwaySummary,
  parameters: { layout: 'padded' },
};
export default meta;

type Story = StoryObj<typeof AwaySummary>;

const baseMessage: SystemMessage = {
  type: 'system',
  uuid: 'sys-1',
  timestamp: '2026-04-20T22:35:57.594Z',
  parentUuid: null,
  isSidechain: false,
  userType: 'external',
  cwd: '/home/user/project',
  sessionId: 'session-1',
  version: '2.1.116',
  subtype: 'away_summary',
};

export const Default: Story = {
  args: {
    message: {
      ...baseMessage,
      content:
        'Cutting a new release was the goal; `v0.3.22` is tagged, pushed, and published with full release notes. Nothing left to do unless you want changes to the notes. (disable recaps in `/config`)',
    },
  },
};

export const LongRecap: Story = {
  args: {
    message: {
      ...baseMessage,
      content:
        'CF-345 is fully delivered and Linear is marked Done; PR #128 is open with auto-merge enabled.\n\nNext action: wait for CI. If CI fails, investigate `frontend/src/services/transcriptService.test.ts` first since that\'s the file that was most recently touched.',
    },
  },
};
