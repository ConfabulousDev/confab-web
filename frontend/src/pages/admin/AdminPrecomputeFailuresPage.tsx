import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { adminAPI, APIError } from '@/services/api';
import { formatRelativeTime } from '@/utils';
import { providerLabel } from '@/utils/providers';
import Alert from '@/components/Alert';
import Button from '@/components/Button';
import type {
  PrecomputeFailureRow,
  PrecomputeFailuresResponse,
  ReleasePrecomputeFailuresResponse,
} from '@/schemas/api';
import styles from './AdminPrecomputeFailuresPage.module.css';

const QUERY_KEY = ['admin', 'precompute-failures'];

const BUCKET_LABELS: Record<string, string> = {
  regular_cards: 'Regular cards',
  smart_recap: 'Smart recap',
  search_index: 'Search index',
  session_title: 'Session title',
};

function bucketLabel(bucket: string): string {
  return BUCKET_LABELS[bucket] ?? bucket;
}

function rowKey(row: PrecomputeFailureRow): string {
  return `${row.session_id} ${row.bucket}`;
}

interface Feedback {
  type: 'success' | 'error';
  message: string;
}

export interface AdminPrecomputeFailuresPageContentProps {
  data: PrecomputeFailuresResponse | null;
  isLoading: boolean;
  error: string | null;
  feedback: Feedback | null;
  onFeedbackClose: () => void;
  onRelease: (row: PrecomputeFailureRow) => void;
  onReleaseAll: () => void;
  /** `${session_id} ${bucket}` of the row whose release is in flight, if any. */
  releasingKey: string | null;
  isReleasingAll: boolean;
}

export function AdminPrecomputeFailuresPageContent({
  data,
  isLoading,
  error,
  feedback,
  onFeedbackClose,
  onRelease,
  onReleaseAll,
  releasingKey,
  isReleasingAll,
}: AdminPrecomputeFailuresPageContentProps) {
  const rows = data?.failures ?? [];
  const busy = isReleasingAll || releasingKey !== null;

  return (
    <div>
      {error && <Alert variant="error">{error}</Alert>}
      {feedback && (
        <Alert variant={feedback.type} onClose={onFeedbackClose}>
          {feedback.message}
        </Alert>
      )}

      <div className={styles.card}>
        <div className={styles.header}>
          <h3>Precompute Failures</h3>
          {rows.length > 0 && (
            <Button size="sm" variant="secondary" onClick={onReleaseAll} disabled={busy}>
              {isReleasingAll ? 'Releasing all...' : 'Release all'}
            </Button>
          )}
        </div>
        <p className={styles.description}>
          Sessions the background worker stopped retrying after repeated failures in one precompute
          bucket. A quarantined session is skipped by that bucket until it is released here, or
          until someone views it and the on-view compute succeeds (regular cards and smart recap
          only). Release once the cause is fixed; the worker retries on its next cycle. After an
          outage (storage, database or LLM down), <strong>Release all</strong> recovers every
          affected session.
        </p>

        {isLoading && <div className={styles.status}>Loading...</div>}
        {!isLoading && !error && rows.length === 0 && (
          <div className={styles.status}>No quarantined sessions. The worker is retrying everything.</div>
        )}
        {!isLoading && rows.length > 0 && (
          <div className={styles.tableWrapper}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>Session</th>
                  <th>Provider</th>
                  <th>Bucket</th>
                  <th>Attempts</th>
                  <th>Quarantined</th>
                  <th>Last error</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {rows.map((row) => {
                  const key = rowKey(row);
                  const releasing = releasingKey === key;
                  return (
                    <tr key={key}>
                      <td>
                        <Link to={`/sessions/${row.session_id}`} title={row.session_id} className={styles.session}>
                          {row.external_id}
                        </Link>
                      </td>
                      <td>{providerLabel(row.provider)}</td>
                      <td>{bucketLabel(row.bucket)}</td>
                      <td>{row.attempts}</td>
                      <td className={styles.timestamp}>{formatRelativeTime(row.quarantined_at)}</td>
                      <td>
                        <span className={styles.error} title={row.last_error}>
                          {row.last_error}
                        </span>
                      </td>
                      <td>
                        <Button size="sm" variant="primary" onClick={() => onRelease(row)} disabled={busy}>
                          {releasing ? 'Releasing...' : 'Release'}
                        </Button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof APIError ? error.message : fallback;
}

function releasedMessage(n: number): string {
  return `Released ${n} ${n === 1 ? 'quarantine' : 'quarantines'}. The worker retries on its next cycle.`;
}

function AdminPrecomputeFailuresPage() {
  const queryClient = useQueryClient();
  const [feedback, setFeedback] = useState<Feedback | null>(null);

  const { data, isLoading, error } = useQuery({
    queryKey: QUERY_KEY,
    queryFn: () => adminAPI.listPrecomputeFailures(),
  });

  const onSuccess = (result: ReleasePrecomputeFailuresResponse) =>
    setFeedback({ type: 'success', message: releasedMessage(result.released) });
  const onError = (err: unknown) =>
    setFeedback({ type: 'error', message: errorMessage(err, 'Failed to release.') });
  const onSettled = () => queryClient.invalidateQueries({ queryKey: QUERY_KEY });

  const releaseMutation = useMutation({
    mutationFn: (row: PrecomputeFailureRow) =>
      adminAPI.releasePrecomputeFailure({ session_id: row.session_id, bucket: row.bucket }),
    onSuccess,
    onError,
    onSettled,
  });

  const releaseAllMutation = useMutation({
    mutationFn: () => adminAPI.releaseAllPrecomputeFailures(),
    onSuccess,
    onError,
    onSettled,
  });

  return (
    <AdminPrecomputeFailuresPageContent
      data={data ?? null}
      isLoading={isLoading}
      error={error ? errorMessage(error, 'Failed to load precompute failures.') : null}
      feedback={feedback}
      onFeedbackClose={() => setFeedback(null)}
      onRelease={(row) => releaseMutation.mutate(row)}
      onReleaseAll={() => releaseAllMutation.mutate()}
      releasingKey={releaseMutation.isPending && releaseMutation.variables ? rowKey(releaseMutation.variables) : null}
      isReleasingAll={releaseAllMutation.isPending}
    />
  );
}

export default AdminPrecomputeFailuresPage;
