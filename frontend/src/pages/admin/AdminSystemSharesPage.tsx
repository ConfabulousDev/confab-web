import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { adminAPI, APIError } from '@/services/api';
import { useAppConfig } from '@/hooks/useAppConfig';
import { useCopyToClipboard } from '@/hooks';
import { formatRelativeTime } from '@/utils';
import Button from '@/components/Button';
import Alert from '@/components/Alert';
import FormField from '@/components/FormField';
import LoadingSkeleton from '@/components/LoadingSkeleton';
import ErrorDisplay from '@/components/ErrorDisplay';
import styles from './AdminSystemSharesPage.module.css';

function AdminSystemSharesPage() {
  const queryClient = useQueryClient();
  const { sharesEnabled } = useAppConfig();
  const { copy, message: copyMessage } = useCopyToClipboard({ successMessage: 'Share URL copied!' });
  const [sessionId, setSessionId] = useState('');
  const [feedback, setFeedback] = useState<{ type: 'success' | 'error'; message: string } | null>(null);

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['admin', 'system-shares'],
    queryFn: adminAPI.listSystemShares,
    enabled: sharesEnabled,
  });

  const createMutation = useMutation({
    mutationFn: adminAPI.createSystemShare,
    onSuccess: (result) => {
      setFeedback({ type: 'success', message: `System share created. URL: ${result.share_url}` });
      setSessionId('');
      queryClient.invalidateQueries({ queryKey: ['admin', 'system-shares'] });
    },
    onError: (err) => {
      setFeedback({ type: 'error', message: err instanceof APIError ? err.message : 'Failed to create system share.' });
    },
  });

  const handleCreate = () => {
    if (!sessionId.trim()) return;
    createMutation.mutate({ session_id: sessionId.trim() });
  };

  if (!sharesEnabled) {
    return (
      <div className={styles.disabled}>
        System shares are not enabled. Enable shares in the server configuration to use this feature.
      </div>
    );
  }

  const shares = data?.shares ?? [];

  return (
    <div>
      {copyMessage && <Alert variant="success">{copyMessage}</Alert>}
      {feedback && (
        <Alert variant={feedback.type} onClose={() => setFeedback(null)}>
          {feedback.message}
        </Alert>
      )}

      {error && (
        <ErrorDisplay
          message={error instanceof Error ? error.message : 'Failed to load system shares'}
          retry={refetch}
        />
      )}

      <div className={styles.createSection}>
        <div className={styles.createForm}>
          <h3>Create System Share</h3>
          <FormField label="Session ID" required>
            <input
              type="text"
              placeholder="Session UUID"
              value={sessionId}
              onChange={(e) => setSessionId(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  e.preventDefault();
                  handleCreate();
                }
              }}
              className={styles.input}
              disabled={createMutation.isPending}
            />
          </FormField>
          <div className={styles.formActions}>
            <Button variant="primary" onClick={handleCreate} disabled={createMutation.isPending || !sessionId.trim()}>
              {createMutation.isPending ? 'Creating...' : 'Create Share'}
            </Button>
          </div>
        </div>
      </div>

      <div className={styles.card}>
        {isLoading ? (
          <LoadingSkeleton variant="list" count={3} />
        ) : shares.length === 0 ? (
          <p className={styles.empty}>No system shares yet.</p>
        ) : (
          <div className={styles.tableWrapper}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>Session ID</th>
                  <th>External ID</th>
                  <th>Share URL</th>
                  <th>Created</th>
                  <th>Last Accessed</th>
                  <th>Expires</th>
                </tr>
              </thead>
              <tbody>
                {shares.map((share) => (
                  <tr key={share.id}>
                    <td>
                      <code className={styles.code}>{share.session_id.substring(0, 8)}</code>
                    </td>
                    <td>
                      <code className={styles.code}>{share.external_id.substring(0, 8)}</code>
                    </td>
                    <td>
                      <div className={styles.urlCell}>
                        <span className={styles.urlText}>{share.share_url}</span>
                        <Button size="sm" onClick={() => copy(share.share_url)}>
                          Copy
                        </Button>
                      </div>
                    </td>
                    <td className={styles.timestamp}>{formatRelativeTime(share.created_at)}</td>
                    <td className={styles.timestamp}>
                      {share.last_accessed_at ? formatRelativeTime(share.last_accessed_at) : '\u2014'}
                    </td>
                    <td className={styles.timestamp}>
                      {share.expires_at ? formatRelativeTime(share.expires_at) : 'Never'}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}

export default AdminSystemSharesPage;
