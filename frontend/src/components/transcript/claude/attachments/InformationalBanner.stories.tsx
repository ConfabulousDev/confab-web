import type { Meta, StoryObj } from '@storybook/react-vite';
import InformationalBanner from './InformationalBanner';
import type { SystemMessage } from '@/types';

const meta: Meta<typeof InformationalBanner> = {
  title: 'Transcript/Attachments/InformationalBanner',
  component: InformationalBanner,
  parameters: { layout: 'padded' },
};
export default meta;

type Story = StoryObj<typeof InformationalBanner>;

const baseMessage: SystemMessage = {
  type: 'system',
  uuid: 'sys-info-1',
  timestamp: '2026-06-15T00:00:00.000Z',
  parentUuid: null,
  isSidechain: false,
  userType: 'external',
  cwd: '/home/user/project',
  sessionId: 'session-1',
  version: '2.1.143',
  subtype: 'informational',
};

// The real CC 2.1.143 auto-mode onboarding banner.
export const AutoModeWarning: Story = {
  args: {
    message: {
      ...baseMessage,
      level: 'warning',
      content:
        'Auto mode lets Claude handle permission prompts automatically — Claude evaluates each tool call for risk and prompt injection before running it, auto-approving safe actions and blocking risky ones.',
    },
  },
};

export const InfoLevel: Story = {
  args: {
    message: {
      ...baseMessage,
      level: 'info',
      content: 'A neutral informational notice with a `code` span and **bold** text.',
    },
  },
};

export const ErrorLevel: Story = {
  args: {
    message: {
      ...baseMessage,
      level: 'error',
      content: 'Something went wrong with this informational row.',
    },
  },
};

// `level` is absent on some informational rows — must degrade to neutral chrome.
export const NoLevel: Story = {
  args: {
    message: {
      ...baseMessage,
      content: 'An informational banner with no `level` field — falls back to neutral styling.',
    },
  },
};
