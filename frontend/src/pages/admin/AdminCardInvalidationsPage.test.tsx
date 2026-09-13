import { describe, it, expect } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import {
  AdminCardInvalidationsPageContent,
  type AdminCardInvalidationsPageContentProps,
} from './AdminCardInvalidationsPage';
import { buildInvalidateCardsRequest } from './invalidateCardsRequest';
import { PROVIDER_VALUES, providerLabel } from '@/utils/providers';

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
    selectedProviders: new Set<string>(),
    reason: '',
    preview: null,
    onStartDateChange: noop,
    onEndDateChange: noop,
    onToggleCard: noop,
    onToggleProvider: noop,
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

function targetsGroup(): HTMLElement {
  return screen.getByRole('group', { name: /targets/i });
}

function providersGroup(): HTMLElement {
  return screen.getByRole('group', { name: /providers/i });
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
      expect(within(targetsGroup()).getByText(name)).toBeInTheDocument();
    }
    expect(within(targetsGroup()).getAllByRole('checkbox')).toHaveLength(cardTypes.length);
  });

  it('does not render card types that are not in the served list', () => {
    render(<AdminCardInvalidationsPageContent {...baseProps(['session_card_tokens'])} />);
    expect(within(targetsGroup()).getByText('session_card_tokens')).toBeInTheDocument();
    expect(screen.queryByText('session_card_session')).not.toBeInTheDocument();
    expect(within(targetsGroup()).getAllByRole('checkbox')).toHaveLength(1);
  });

  it('shows an unavailable message and no target checkboxes when the list is empty (load/error)', () => {
    render(<AdminCardInvalidationsPageContent {...baseProps([])} />);
    expect(screen.getByText('Card types unavailable.')).toBeInTheDocument();
    expect(within(targetsGroup()).queryAllByRole('checkbox')).toHaveLength(0);
  });
});

// nbrd: session_title is a non-card target served alongside the card tables.
describe('AdminCardInvalidationsPageContent session_title target (nbrd)', () => {
  it('renders a friendly label for session_title and raw names for card tables', () => {
    render(
      <AdminCardInvalidationsPageContent {...baseProps(['session_card_tokens', 'session_title'])} />,
    );
    const group = targetsGroup();
    expect(within(group).getByText('session_card_tokens')).toBeInTheDocument();
    expect(within(group).getByLabelText(/session title: re-derive missing codex titles/i)).toBeInTheDocument();
    expect(within(group).getAllByRole('checkbox')).toHaveLength(2);
  });

  it('explains that title recompute runs in the background and does not recompute cards', () => {
    render(<AdminCardInvalidationsPageContent {...baseProps(['session_title'])} />);
    expect(screen.getByText(/runs in the background worker/i)).toBeInTheDocument();
    expect(screen.getByText(/does not recompute cards/i)).toBeInTheDocument();
  });
});

describe('AdminCardInvalidationsPageContent provider filter (nbrd)', () => {
  it('renders one checkbox per canonical provider, labeled for display', () => {
    render(<AdminCardInvalidationsPageContent {...baseProps(['session_card_tokens'])} />);
    const group = providersGroup();
    expect(within(group).getAllByRole('checkbox')).toHaveLength(PROVIDER_VALUES.length);
    for (const p of PROVIDER_VALUES) {
      expect(within(group).getByLabelText(providerLabel(p))).toBeInTheDocument();
    }
  });

  it('reflects the selected providers', () => {
    render(
      <AdminCardInvalidationsPageContent
        {...baseProps(['session_card_tokens'])}
        selectedProviders={new Set(['codex'])}
      />,
    );
    const group = providersGroup();
    expect(within(group).getByLabelText(providerLabel('codex'))).toBeChecked();
    expect(within(group).getByLabelText(providerLabel('cursor'))).not.toBeChecked();
    expect(screen.getByText(/none selected means all providers/i)).toBeInTheDocument();
  });
});

describe('buildInvalidateCardsRequest (nbrd)', () => {
  const base = {
    startDate: '2026-08-20T00:00',
    endDate: '',
    selectedCards: new Set(['session_title']),
    reason: '  codex title repair ',
    confirmInput: '12',
  };

  it('omits providers when none are selected', () => {
    const req = buildInvalidateCardsRequest({ ...base, selectedProviders: new Set(), dryRun: true });
    expect(req).not.toHaveProperty('providers');
    expect(req).toMatchObject({
      start_date: '2026-08-20T00:00:00Z',
      card_types: ['session_title'],
      reason: 'codex title repair',
      dry_run: true,
    });
    expect(req).not.toHaveProperty('confirm');
  });

  it('includes the selected providers', () => {
    const req = buildInvalidateCardsRequest({ ...base, selectedProviders: new Set(['codex']), dryRun: true });
    expect(req.providers).toEqual(['codex']);
  });

  it('includes the typed confirmation only on execute', () => {
    const req = buildInvalidateCardsRequest({ ...base, selectedProviders: new Set(), dryRun: false });
    expect(req.confirm).toBe('12');
    expect(req.dry_run).toBe(false);
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
