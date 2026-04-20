import type { Meta, StoryObj } from '@storybook/react-vite';
import {
  AdminCardInvalidationsPageContent,
  type AdminCardInvalidationsPageContentProps,
} from './AdminCardInvalidationsPage';
import type { CardTableName, InvalidateCardsResponse, CardInvalidationsListResponse } from '@/schemas/api';

const noop = () => {};

const baseProps: AdminCardInvalidationsPageContentProps = {
  startDate: '',
  endDate: '',
  selectedCards: new Set<CardTableName>(),
  reason: '',
  preview: null,
  onStartDateChange: noop,
  onEndDateChange: noop,
  onToggleCard: noop,
  onReasonChange: noop,
  onPreview: noop,
  isPreviewing: false,
  onExecuteClick: noop,
  showConfirmModal: false,
  onConfirmClose: noop,
  onConfirmExecute: noop,
  isExecuting: false,
  feedback: null,
  onFeedbackClose: noop,
  history: { rows: [] },
  historyLoading: false,
};

const meta: Meta<typeof AdminCardInvalidationsPageContent> = {
  title: 'Pages/Admin/AdminCardInvalidationsPage',
  component: AdminCardInvalidationsPageContent,
  parameters: { layout: 'padded' },
  decorators: [
    (Story) => (
      <div style={{ maxWidth: '1200px', margin: '0 auto' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof AdminCardInvalidationsPageContent>;

export const EmptyForm: Story = { args: baseProps };

export const PartiallyFilled: Story = {
  args: {
    ...baseProps,
    startDate: '2026-04-01T00:00',
    endDate: '2026-04-20T00:00',
    selectedCards: new Set<CardTableName>(['session_card_tokens']),
    reason: 'Opus 4.7 pricing backfill',
  },
};

const previewResponse: InvalidateCardsResponse = {
  correlation_id: '0191a3e0-1234-7000-8000-aabbccddeeff',
  affected_sessions: 1234,
  affected_cards: {
    session_card_tokens: 1234,
    session_card_session: 1190,
  },
  executed: false,
};

export const PreviewShown: Story = {
  args: {
    ...baseProps,
    startDate: '2026-04-01T00:00',
    endDate: '2026-04-20T00:00',
    selectedCards: new Set<CardTableName>(['session_card_tokens', 'session_card_session']),
    reason: 'Opus 4.7 pricing backfill',
    preview: previewResponse,
  },
};

export const ConfirmModalOpen: Story = {
  args: {
    ...baseProps,
    startDate: '2026-04-01T00:00',
    endDate: '2026-04-20T00:00',
    selectedCards: new Set<CardTableName>(['session_card_tokens']),
    reason: 'Opus 4.7 pricing backfill',
    preview: previewResponse,
    showConfirmModal: true,
  },
};

export const ExecuteSuccessToast: Story = {
  args: {
    ...baseProps,
    feedback: {
      type: 'success',
      message: 'Invalidated 1,234 sessions. correlation_id: 0191a3e0-1234-7000-8000-aabbccddeeff',
    },
  },
};

export const PartialFailureToast: Story = {
  args: {
    ...baseProps,
    feedback: {
      type: 'error',
      message:
        'Invalidation partial: 4000 / 10000 sessions (4 batches). Re-run to finish. correlation_id: 0191a3e0-1234-7000-8000-aabbccddeeff',
    },
  },
};

const historyWithRows: CardInvalidationsListResponse = {
  rows: [
    {
      id: 42,
      session_id: 'aaaaaaaa-1111-2222-3333-444444444444',
      admin_user_id: 7,
      admin_email: 'admin@example.com',
      invalidated_at: new Date(Date.now() - 1000 * 60 * 5).toISOString(),
      card_types: ['session_card_tokens'],
      correlation_id: '0191a3e0-1234-7000-8000-aabbccddeeff',
      reason: 'Opus 4.7 pricing backfill',
    },
    {
      id: 41,
      session_id: 'bbbbbbbb-5555-6666-7777-888888888888',
      admin_user_id: 7,
      admin_email: 'admin@example.com',
      invalidated_at: new Date(Date.now() - 1000 * 60 * 60).toISOString(),
      card_types: ['session_card_tokens', 'session_card_session'],
      correlation_id: '0190a3e0-5678-7000-8000-ccddeeff0011',
      reason: 'Tokens card version bump',
    },
  ],
};

export const WithHistory: Story = {
  args: {
    ...baseProps,
    history: historyWithRows,
  },
};
