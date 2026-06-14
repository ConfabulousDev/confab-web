import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { adminAPI, APIError } from '@/services/api';
import { useAppConfig } from '@/hooks/useAppConfig';
import { useAuth } from '@/hooks/useAuth';
import { formatRelativeTime } from '@/utils';
import Button from '@/components/Button';
import Alert from '@/components/Alert';
import Modal from '@/components/Modal';
import FormField from '@/components/FormField';
import LoadingSkeleton from '@/components/LoadingSkeleton';
import ErrorDisplay from '@/components/ErrorDisplay';
import styles from './AdminUsersPage.module.css';

type ConfirmActionType = 'deactivate' | 'activate' | 'delete' | 'revoke-admin-self';

interface ConfirmAction {
  type: ConfirmActionType;
  userId: number;
  userEmail: string;
}

const CONFIRM_LABELS: Record<ConfirmActionType, { title: string; button: string; variant: 'primary' | 'danger' }> = {
  activate: { title: 'Activate User', button: 'Activate', variant: 'primary' },
  deactivate: { title: 'Deactivate User', button: 'Deactivate', variant: 'primary' },
  delete: { title: 'Delete User', button: 'Delete', variant: 'danger' },
  // 5k4v D-S3: removing your OWN admin access is a soft confirm (never blocked).
  'revoke-admin-self': { title: 'Remove your admin access', button: 'Remove admin', variant: 'danger' },
};

function AdminUsersPage() {
  const queryClient = useQueryClient();
  const { passwordAuthEnabled } = useAppConfig();
  const { user: currentUser } = useAuth();
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [newEmail, setNewEmail] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [feedback, setFeedback] = useState<{ type: 'success' | 'error'; message: string } | null>(null);
  const [confirmAction, setConfirmAction] = useState<ConfirmAction | null>(null);

  const usersQueryKey = ['admin', 'users'];

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: usersQueryKey,
    queryFn: adminAPI.listUsers,
  });

  function showFeedback(type: 'success' | 'error', message: string): void {
    setFeedback({ type, message });
  }

  function formatMutationError(err: unknown, fallback: string): string {
    return err instanceof APIError ? err.message : fallback;
  }

  const createMutation = useMutation({
    mutationFn: adminAPI.createUser,
    onSuccess: (result) => {
      showFeedback('success', `User ${result.email} created successfully.`);
      setShowCreateForm(false);
      setNewEmail('');
      setNewPassword('');
      queryClient.invalidateQueries({ queryKey: usersQueryKey });
    },
    onError: (err) => {
      showFeedback('error', formatMutationError(err, 'Failed to create user.'));
    },
  });

  const deactivateMutation = useMutation({
    mutationFn: adminAPI.deactivateUser,
    onSuccess: () => {
      showFeedback('success', 'User deactivated.');
      queryClient.invalidateQueries({ queryKey: usersQueryKey });
    },
    onError: (err) => {
      showFeedback('error', formatMutationError(err, 'Failed to deactivate user.'));
    },
  });

  const activateMutation = useMutation({
    mutationFn: adminAPI.activateUser,
    onSuccess: () => {
      showFeedback('success', 'User activated.');
      queryClient.invalidateQueries({ queryKey: usersQueryKey });
    },
    onError: (err) => {
      showFeedback('error', formatMutationError(err, 'Failed to activate user.'));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: adminAPI.deleteUser,
    onSuccess: () => {
      showFeedback('success', 'User deleted.');
      queryClient.invalidateQueries({ queryKey: usersQueryKey });
    },
    onError: (err) => {
      showFeedback('error', formatMutationError(err, 'Failed to delete user.'));
    },
  });

  const grantAdminMutation = useMutation({
    mutationFn: adminAPI.grantAdmin,
    onSuccess: () => {
      showFeedback('success', 'User granted admin.');
      queryClient.invalidateQueries({ queryKey: usersQueryKey });
    },
    onError: (err) => {
      showFeedback('error', formatMutationError(err, 'Failed to grant admin.'));
    },
  });

  const revokeAdminMutation = useMutation({
    mutationFn: adminAPI.revokeAdmin,
    onSuccess: () => {
      showFeedback('success', 'Admin access revoked.');
      // Self-revoke loses access to this very page on next gate; refresh /me too.
      queryClient.invalidateQueries({ queryKey: usersQueryKey });
      queryClient.invalidateQueries({ queryKey: ['auth', 'me'] });
    },
    onError: (err) => {
      showFeedback('error', formatMutationError(err, 'Failed to revoke admin.'));
    },
  });

  function handleConfirm(): void {
    if (!confirmAction) return;
    switch (confirmAction.type) {
      case 'deactivate':
        deactivateMutation.mutate(confirmAction.userId);
        break;
      case 'activate':
        activateMutation.mutate(confirmAction.userId);
        break;
      case 'delete':
        deleteMutation.mutate(confirmAction.userId);
        break;
      case 'revoke-admin-self':
        revokeAdminMutation.mutate(confirmAction.userId);
        break;
    }
    setConfirmAction(null);
  }

  // Toggle the is_admin column. Revoking your OWN access goes through a soft
  // confirm (D-S3); every other grant/revoke applies immediately.
  function handleToggleAdmin(user: { id: number; email: string; is_admin: boolean }): void {
    if (!user.is_admin) {
      grantAdminMutation.mutate(user.id);
      return;
    }
    // Self-detection by email (the /me User shape doesn't carry id); email is unique.
    if (currentUser && currentUser.email === user.email) {
      setConfirmAction({ type: 'revoke-admin-self', userId: user.id, userEmail: user.email });
      return;
    }
    revokeAdminMutation.mutate(user.id);
  }

  function handleCreateSubmit(): void {
    if (!newEmail.trim() || !newPassword.trim()) return;
    createMutation.mutate({ email: newEmail.trim(), password: newPassword });
  }

  function resetCreateForm(): void {
    setShowCreateForm(false);
    setNewEmail('');
    setNewPassword('');
  }

  const users = data?.users ?? [];
  const totals = data?.totals;

  return (
    <div>
      {feedback && (
        <Alert variant={feedback.type} onClose={() => setFeedback(null)}>
          {feedback.message}
        </Alert>
      )}

      {error && (
        <ErrorDisplay
          message={error instanceof Error ? error.message : 'Failed to load users'}
          retry={refetch}
        />
      )}

      {totals && (
        <div className={styles.summaryCards}>
          <div className={styles.summaryCard}>
            <div className={styles.summaryValue}>{totals.total_sessions}</div>
            <div className={styles.summaryLabel}>Total Sessions</div>
          </div>
          <div className={styles.summaryCard}>
            <div className={styles.summaryValue}>{totals.non_empty_sessions}</div>
            <div className={styles.summaryLabel}>Non-Empty Sessions</div>
          </div>
          <div className={styles.summaryCard}>
            <div className={styles.summaryValue}>{totals.sessions_with_cache}</div>
            <div className={styles.summaryLabel}>Sessions w/ Cache</div>
          </div>
          <div className={styles.summaryCard}>
            <div className={styles.summaryValue}>{totals.computations_this_month}</div>
            <div className={styles.summaryLabel}>Computations (Month)</div>
          </div>
        </div>
      )}

      {passwordAuthEnabled && (
        <div className={styles.createSection}>
          {!showCreateForm ? (
            <Button variant="primary" onClick={() => setShowCreateForm(true)}>
              + Create User
            </Button>
          ) : (
            <div className={styles.createForm}>
              <h3>Create New User</h3>
              <FormField label="Email" required>
                <input
                  type="email"
                  placeholder="user@example.com"
                  value={newEmail}
                  onChange={(e) => setNewEmail(e.target.value)}
                  className={styles.input}
                  disabled={createMutation.isPending}
                />
              </FormField>
              <FormField label="Password" required>
                <input
                  type="password"
                  placeholder="Password"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      e.preventDefault();
                      handleCreateSubmit();
                    }
                  }}
                  className={styles.input}
                  disabled={createMutation.isPending}
                />
              </FormField>
              <div className={styles.formActions}>
                <Button variant="primary" onClick={handleCreateSubmit} disabled={createMutation.isPending}>
                  {createMutation.isPending ? 'Creating...' : 'Create'}
                </Button>
                <Button variant="secondary" onClick={resetCreateForm} disabled={createMutation.isPending}>
                  Cancel
                </Button>
              </div>
            </div>
          )}
        </div>
      )}

      <div className={styles.card}>
        {isLoading ? (
          <LoadingSkeleton variant="list" count={5} />
        ) : users.length === 0 ? (
          <p className={styles.empty}>No users found.</p>
        ) : (
          <div className={styles.tableWrapper}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>Email</th>
                  <th>Name</th>
                  <th>Status</th>
                  <th>Sessions</th>
                  <th>Recap Cache</th>
                  <th>Recaps/Month</th>
                  <th>Last API Key</th>
                  <th>Last Login</th>
                  <th>Created</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {users.map((user) => (
                  <tr key={user.id}>
                    <td className={styles.emailCell}>{user.email}</td>
                    <td>{user.name || '\u2014'}</td>
                    <td>
                      <span className={`${styles.statusChip} ${styles[`status_${user.status}`] || ''}`}>
                        {user.status}
                      </span>
                    </td>
                    <td>{user.session_count}</td>
                    <td>{user.recap_cache_count}</td>
                    <td>{user.recaps_this_month}</td>
                    <td className={styles.timestamp}>
                      {user.last_api_key_used ? formatRelativeTime(user.last_api_key_used) : '\u2014'}
                    </td>
                    <td className={styles.timestamp}>
                      {user.last_logged_in ? formatRelativeTime(user.last_logged_in) : '\u2014'}
                    </td>
                    <td className={styles.timestamp}>{formatRelativeTime(user.created_at)}</td>
                    <td>
                      <div className={styles.actions}>
                        {user.is_super_admin ? (
                          <span className={styles.adminBadge} title="Admin via SUPER_ADMIN_EMAILS (set in the environment)">
                            Admin · env
                          </span>
                        ) : (
                          <Button
                            size="sm"
                            variant={user.is_admin ? 'secondary' : 'primary'}
                            onClick={() => handleToggleAdmin(user)}
                          >
                            {user.is_admin ? 'Revoke admin' : 'Make admin'}
                          </Button>
                        )}
                        {user.status === 'active' ? (
                          <Button
                            size="sm"
                            variant="secondary"
                            onClick={() => setConfirmAction({ type: 'deactivate', userId: user.id, userEmail: user.email })}
                          >
                            Deactivate
                          </Button>
                        ) : (
                          <Button
                            size="sm"
                            variant="primary"
                            onClick={() => setConfirmAction({ type: 'activate', userId: user.id, userEmail: user.email })}
                          >
                            Activate
                          </Button>
                        )}
                        <Button
                          size="sm"
                          variant="danger"
                          onClick={() => setConfirmAction({ type: 'delete', userId: user.id, userEmail: user.email })}
                        >
                          Delete
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <Modal
        isOpen={confirmAction !== null}
        onClose={() => setConfirmAction(null)}
        ariaLabel="Confirm action"
      >
        {confirmAction && (
          <div className={styles.confirmModal}>
            <h3>{CONFIRM_LABELS[confirmAction.type].title}</h3>
            {confirmAction.type === 'revoke-admin-self' ? (
              <p>
                You&rsquo;re about to remove <strong>your own</strong> admin access and may lose
                access to this page. You can be re-granted by another admin, or via
                <code> SUPER_ADMIN_EMAILS</code>.
              </p>
            ) : (
              <p>
                Are you sure you want to {confirmAction.type}{' '}
                <strong>{confirmAction.userEmail}</strong>?
                {confirmAction.type === 'delete' && ' This action cannot be undone.'}
              </p>
            )}
            <div className={styles.modalActions}>
              <Button
                variant={CONFIRM_LABELS[confirmAction.type].variant}
                onClick={handleConfirm}
              >
                {CONFIRM_LABELS[confirmAction.type].button}
              </Button>
              <Button variant="secondary" onClick={() => setConfirmAction(null)}>
                Cancel
              </Button>
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
}

export default AdminUsersPage;
