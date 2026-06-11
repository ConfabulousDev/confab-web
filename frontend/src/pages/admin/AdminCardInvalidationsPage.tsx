import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { adminAPI, APIError } from '@/services/api';
import { formatRelativeTime } from '@/utils';
import Button from '@/components/Button';
import Alert from '@/components/Alert';
import Modal from '@/components/Modal';
import {
  InvalidateCardsResponseSchema,
  type InvalidateCardsResponse,
  type CardInvalidationsListResponse,
} from '@/schemas/api';
import styles from './AdminCardInvalidationsPage.module.css';

type Feedback = { type: 'success' | 'error' | 'info'; message: string };

// toIsoUtc converts a `datetime-local` input value (e.g. "2026-04-20T12:34")
// to an ISO-8601 timestamp with explicit UTC timezone. The input is treated as
// wall-clock UTC per the "UTC" label next to the field.
function toIsoUtc(datetimeLocal: string): string {
  if (!datetimeLocal) return '';
  const withSeconds = datetimeLocal.length === 16 ? `${datetimeLocal}:00` : datetimeLocal;
  return `${withSeconds}Z`;
}

// partialFailureFrom extracts the structured partial-failure body from an APIError
// (500 response shaped like InvalidateCardsResponse).
function partialFailureFrom(err: unknown): InvalidateCardsResponse | null {
  if (!(err instanceof APIError)) return null;
  const parsed = InvalidateCardsResponseSchema.safeParse(err.data);
  return parsed.success ? parsed.data : null;
}

export interface AdminCardInvalidationsPageContentProps {
  /** Card table names served by the backend (GET /admin/cards/types). */
  cardTypes: string[];
  startDate: string;
  endDate: string;
  selectedCards: Set<string>;
  reason: string;
  preview: InvalidateCardsResponse | null;

  onStartDateChange: (v: string) => void;
  onEndDateChange: (v: string) => void;
  onToggleCard: (card: string) => void;
  onReasonChange: (v: string) => void;

  onPreview: () => void;
  isPreviewing: boolean;

  onExecuteClick: () => void;
  showConfirmModal: boolean;
  onConfirmClose: () => void;
  onConfirmExecute: () => void;
  isExecuting: boolean;

  feedback: Feedback | null;
  onFeedbackClose: () => void;

  history: CardInvalidationsListResponse | null;
  historyLoading: boolean;
}

export function AdminCardInvalidationsPageContent({
  cardTypes,
  startDate,
  endDate,
  selectedCards,
  reason,
  preview,
  onStartDateChange,
  onEndDateChange,
  onToggleCard,
  onReasonChange,
  onPreview,
  isPreviewing,
  onExecuteClick,
  showConfirmModal,
  onConfirmClose,
  onConfirmExecute,
  isExecuting,
  feedback,
  onFeedbackClose,
  history,
  historyLoading,
}: AdminCardInvalidationsPageContentProps) {
  const canPreview = startDate.trim() !== '' && selectedCards.size > 0 && reason.trim() !== '';
  const canExecute = canPreview && preview !== null;

  return (
    <div>
      {feedback && (
        <Alert variant={feedback.type === 'info' ? 'info' : feedback.type} onClose={onFeedbackClose}>
          {feedback.message}
        </Alert>
      )}

      <div className={styles.card}>
        <h3>Invalidate Cards by Date Range</h3>
        <p className={styles.description}>
          Deletes <code>session_card_*</code> rows for sessions in the selected window.
          The worker will recompute them with current logic/pricing on the next tick.
          Typical use: true-up cost after a pricing update.
        </p>

        <div className={styles.formRow}>
          <label className={styles.label}>
            Start (UTC) <span className={styles.required}>*</span>
            <input
              type="datetime-local"
              className={styles.input}
              value={startDate}
              onChange={(e) => onStartDateChange(e.target.value)}
            />
          </label>
          <label className={styles.label}>
            End (UTC, optional)
            <input
              type="datetime-local"
              className={styles.input}
              value={endDate}
              onChange={(e) => onEndDateChange(e.target.value)}
            />
          </label>
        </div>

        <fieldset className={styles.cardTypesGroup}>
          <legend className={styles.legend}>
            Card Types <span className={styles.required}>*</span>
          </legend>
          {cardTypes.length === 0 ? (
            // Empty while the GET /admin/cards/types fetch is loading or failed;
            // with no checkboxes nothing is selectable, so the form stays disabled.
            <p className={styles.cardTypesEmpty}>Card types unavailable.</p>
          ) : (
            cardTypes.map((name) => (
              <label key={name} className={styles.checkboxLabel}>
                <input
                  type="checkbox"
                  checked={selectedCards.has(name)}
                  onChange={() => onToggleCard(name)}
                />
                <code>{name}</code>
              </label>
            ))
          )}
        </fieldset>

        <label className={styles.label}>
          Reason <span className={styles.required}>*</span>
          <textarea
            className={styles.textarea}
            value={reason}
            onChange={(e) => onReasonChange(e.target.value)}
            maxLength={500}
            placeholder="e.g. Opus 4.7 pricing backfill"
          />
          <span className={styles.charCount}>{reason.length} / 500</span>
        </label>

        <div className={styles.actions}>
          <Button variant="secondary" onClick={onPreview} disabled={!canPreview || isPreviewing}>
            {isPreviewing ? 'Previewing...' : 'Preview'}
          </Button>
          <Button variant="danger" onClick={onExecuteClick} disabled={!canExecute || isExecuting}>
            {isExecuting ? 'Executing...' : 'Execute'}
          </Button>
        </div>

        {preview && (
          <div className={styles.previewBox}>
            <h4>Preview</h4>
            <div><strong>{preview.affected_sessions.toLocaleString()}</strong> sessions would be invalidated.</div>
            <ul className={styles.cardCounts}>
              {Object.entries(preview.affected_cards)
                .filter(([, n]) => n > 0)
                .map(([name, n]) => (
                  <li key={name}>
                    <code>{name}</code>: {n.toLocaleString()} rows
                  </li>
                ))}
            </ul>
            <div className={styles.correlationHint}>
              correlation_id: <code>{preview.correlation_id}</code>
            </div>
          </div>
        )}
      </div>

      <div className={styles.card}>
        <h3>Recent Invalidations</h3>
        {historyLoading && <div className={styles.historyStatus}>Loading...</div>}
        {history && history.rows.length === 0 && !historyLoading && (
          <div className={styles.historyStatus}>No invalidations yet.</div>
        )}
        {history && history.rows.length > 0 && (
          <div className={styles.tableWrapper}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>When</th>
                  <th>Admin</th>
                  <th>Session</th>
                  <th>Card Types</th>
                  <th>Reason</th>
                  <th>Correlation</th>
                </tr>
              </thead>
              <tbody>
                {history.rows.map((row) => (
                  <tr key={row.id}>
                    <td className={styles.timestamp}>{formatRelativeTime(row.invalidated_at)}</td>
                    <td>{row.admin_email || `#${row.admin_user_id}`}</td>
                    <td><code>{row.session_id.slice(0, 8)}</code></td>
                    <td>
                      {row.card_types.map((ct) => (
                        <code key={ct} className={styles.cardTypePill}>{ct}</code>
                      ))}
                    </td>
                    <td>{row.reason}</td>
                    <td><code>{row.correlation_id.slice(0, 8)}</code></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <Modal
        isOpen={showConfirmModal}
        onClose={onConfirmClose}
        ariaLabel="Confirm card invalidation"
      >
        <div className={styles.confirmModal}>
          <h3>Execute invalidation?</h3>
          <p>
            This will DELETE{' '}
            <strong>{preview?.affected_sessions.toLocaleString() ?? '?'}</strong> sessions&rsquo; cards
            across <strong>{selectedCards.size}</strong> card type(s).
            The worker will recompute them over the next few minutes. This action cannot be undone.
          </p>
          <div className={styles.modalActions}>
            <Button variant="danger" onClick={onConfirmExecute} disabled={isExecuting}>
              {isExecuting ? 'Executing...' : 'Confirm & Execute'}
            </Button>
            <Button variant="secondary" onClick={onConfirmClose} disabled={isExecuting}>
              Cancel
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}

const HISTORY_QUERY_KEY = ['admin', 'card-invalidations', 'history'];

function AdminCardInvalidationsPage() {
  const queryClient = useQueryClient();
  const [startDate, setStartDate] = useState('');
  const [endDate, setEndDate] = useState('');
  const [selectedCards, setSelectedCards] = useState<Set<string>>(new Set());
  const [reason, setReason] = useState('');
  const [preview, setPreview] = useState<InvalidateCardsResponse | null>(null);
  const [showConfirmModal, setShowConfirmModal] = useState(false);
  const [feedback, setFeedback] = useState<Feedback | null>(null);

  const { data: history, isLoading: historyLoading } = useQuery({
    queryKey: HISTORY_QUERY_KEY,
    queryFn: () => adminAPI.listCardInvalidations(),
  });

  // vd31: the card-type checkboxes are sourced from the backend
  // (analytics.AllCardTableNames) so they can't drift from a hardcoded list.
  const { data: cardTypesData } = useQuery({
    queryKey: ['admin', 'card-types'],
    queryFn: () => adminAPI.getCardTypes(),
  });
  // On loading/error this is empty → no checkboxes render → submit stays disabled.
  const cardTypes = cardTypesData?.card_types ?? [];

  // Any edit to the form drops the stale preview so the user can't execute
  // against counts that no longer reflect the current inputs.
  function setAndInvalidatePreview<T>(setter: (v: T) => void) {
    return (v: T) => {
      setter(v);
      setPreview(null);
    };
  }

  function toggleCard(name: string) {
    setSelectedCards((prev) => {
      const next = new Set(prev);
      if (next.has(name)) {
        next.delete(name);
      } else {
        next.add(name);
      }
      return next;
    });
    setPreview(null);
  }

  function buildRequest(dryRun: boolean) {
    return {
      start_date: toIsoUtc(startDate),
      end_date: endDate ? toIsoUtc(endDate) : undefined,
      card_types: Array.from(selectedCards),
      reason: reason.trim(),
      dry_run: dryRun,
    };
  }

  const previewMutation = useMutation({
    mutationFn: () => adminAPI.invalidateCards(buildRequest(true)),
    onSuccess: (resp) => {
      setPreview(resp);
      setFeedback(null);
    },
    onError: (err) => {
      setPreview(null);
      setFeedback({
        type: 'error',
        message: err instanceof APIError ? err.message : 'Failed to preview invalidation.',
      });
    },
  });

  const executeMutation = useMutation({
    mutationFn: () => adminAPI.invalidateCards(buildRequest(false)),
    onSuccess: (resp) => {
      setShowConfirmModal(false);
      setFeedback({
        type: 'success',
        message: `Invalidated ${resp.affected_sessions.toLocaleString()} sessions. correlation_id: ${resp.correlation_id}`,
      });
      setPreview(null);
      queryClient.invalidateQueries({ queryKey: HISTORY_QUERY_KEY });
    },
    onError: (err) => {
      setShowConfirmModal(false);
      const partial = partialFailureFrom(err);
      if (partial) {
        setFeedback({
          type: 'error',
          message:
            `Invalidation partial: ${partial.affected_sessions_executed ?? 0} / ${preview?.affected_sessions ?? partial.affected_sessions} sessions ` +
            `(${partial.completed_batches ?? 0} batches). Re-run to finish. correlation_id: ${partial.correlation_id}`,
        });
        queryClient.invalidateQueries({ queryKey: HISTORY_QUERY_KEY });
        return;
      }
      setFeedback({
        type: 'error',
        message: err instanceof APIError ? err.message : 'Failed to execute invalidation.',
      });
    },
  });

  return (
    <AdminCardInvalidationsPageContent
      cardTypes={cardTypes}
      startDate={startDate}
      endDate={endDate}
      selectedCards={selectedCards}
      reason={reason}
      preview={preview}
      onStartDateChange={setAndInvalidatePreview(setStartDate)}
      onEndDateChange={setAndInvalidatePreview(setEndDate)}
      onToggleCard={toggleCard}
      onReasonChange={setAndInvalidatePreview(setReason)}
      onPreview={() => previewMutation.mutate()}
      isPreviewing={previewMutation.isPending}
      onExecuteClick={() => setShowConfirmModal(true)}
      showConfirmModal={showConfirmModal}
      onConfirmClose={() => setShowConfirmModal(false)}
      onConfirmExecute={() => executeMutation.mutate()}
      isExecuting={executeMutation.isPending}
      feedback={feedback}
      onFeedbackClose={() => setFeedback(null)}
      history={history ?? null}
      historyLoading={historyLoading}
    />
  );
}

export default AdminCardInvalidationsPage;
