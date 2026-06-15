import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import {
  AdminCardInvalidationsPageContent,
  type AdminCardInvalidationsPageContentProps,
} from './AdminCardInvalidationsPage';

const noop = () => {};

// vd31: the card-type checkboxes are rendered from a backend-served `cardTypes`
// list (passed as a prop), not a hardcoded frontend array. This pins that the
// content renders exactly the provided list — including the entries the old
// hardcoded list had dropped (session_card_tokens_v2, session_card_workflows).
function baseProps(cardTypes: string[]): AdminCardInvalidationsPageContentProps {
  return {
    cardTypes,
    startDate: '',
    endDate: '',
    selectedCards: new Set<string>(),
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
    confirmInput: '',
    onConfirmInputChange: noop,
    onConfirmClose: noop,
    onConfirmExecute: noop,
    isExecuting: false,
    feedback: null,
    onFeedbackClose: noop,
    history: { rows: [] },
    historyLoading: false,
  };
}

describe('AdminCardInvalidationsPageContent card-type checkboxes', () => {
  it('renders a checkbox for each provided card type', () => {
    const cardTypes = [
      'session_card_tokens',
      'session_card_tokens_v2',
      'session_card_workflows',
    ];
    render(<AdminCardInvalidationsPageContent {...baseProps(cardTypes)} />);
    for (const name of cardTypes) {
      expect(screen.getByText(name)).toBeInTheDocument();
    }
    expect(screen.getAllByRole('checkbox')).toHaveLength(cardTypes.length);
  });

  it('does not render card types that are not in the served list', () => {
    render(<AdminCardInvalidationsPageContent {...baseProps(['session_card_tokens'])} />);
    expect(screen.getByText('session_card_tokens')).toBeInTheDocument();
    expect(screen.queryByText('session_card_session')).not.toBeInTheDocument();
    expect(screen.getAllByRole('checkbox')).toHaveLength(1);
  });

  it('shows an unavailable message and no checkboxes when the list is empty (load/error)', () => {
    render(<AdminCardInvalidationsPageContent {...baseProps([])} />);
    expect(screen.getByText('Card types unavailable.')).toBeInTheDocument();
    expect(screen.queryAllByRole('checkbox')).toHaveLength(0);
  });
});

// kyrr: the execute confirm modal requires the admin to type the affected-session
// count from the preview before "Confirm & Execute" enables.
describe('AdminCardInvalidationsPageContent execute confirmation (kyrr)', () => {
  const preview = {
    correlation_id: 'c0ffee00-0000-0000-0000-000000000000',
    affected_sessions: 42,
    affected_cards: { session_card_tokens: 42 },
    executed: false,
  };

  function modalProps(confirmInput: string): AdminCardInvalidationsPageContentProps {
    return {
      ...baseProps(['session_card_tokens']),
      selectedCards: new Set<string>(['session_card_tokens']),
      reason: 'pricing backfill',
      preview,
      showConfirmModal: true,
      confirmInput,
    };
  }

  function executeButton(): HTMLButtonElement {
    return screen.getByRole('button', { name: /confirm & execute/i });
  }

  it('disables Confirm & Execute until the typed count matches', () => {
    render(<AdminCardInvalidationsPageContent {...modalProps('')} />);
    expect(executeButton()).toBeDisabled();
  });

  it('keeps Confirm & Execute disabled when the typed count is wrong', () => {
    render(<AdminCardInvalidationsPageContent {...modalProps('7')} />);
    expect(executeButton()).toBeDisabled();
  });

  it('enables Confirm & Execute when the typed count matches the preview', () => {
    render(<AdminCardInvalidationsPageContent {...modalProps('42')} />);
    expect(executeButton()).toBeEnabled();
  });
});
