import { useState, useEffect } from 'react';
import type { SessionShare } from '@/types';
import { sessionsAPI } from '@/services/api';
import { useCopyToClipboard } from '@/hooks';
import { formatDate } from '@/utils';
import { shareFormSchema, emailSchema, validateForm, getFieldError } from '@/schemas/validation';
import type { ShareFormData } from '@/schemas/validation';
import ErrorDisplay from './ErrorDisplay';
import FormField from './FormField';
import Button from './Button';
import styles from './ShareDialog.module.css';

interface ShareDialogProps {
  sessionId: string;
  isOpen: boolean;
  onClose: () => void;
}

function ShareDialog({ sessionId, isOpen, onClose }: ShareDialogProps) {
  const [shareVisibility, setShareVisibility] = useState<'public' | 'private'>('public');
  const [invitedEmails, setInvitedEmails] = useState<string[]>([]);
  const [newEmail, setNewEmail] = useState('');
  const [expiresInDays, setExpiresInDays] = useState<number | null>(7);
  const [createdShareURL, setCreatedShareURL] = useState('');
  const [shares, setShares] = useState<SessionShare[]>([]);
  const [loading, setLoading] = useState(false);
  const [loadingShares, setLoadingShares] = useState(false);
  const [error, setError] = useState('');
  const [validationErrors, setValidationErrors] = useState<Record<string, string[]>>();
  const { copy, copied } = useCopyToClipboard();

  useEffect(() => {
    if (isOpen) {
      resetForm();
      fetchShares();
    }
  }, [isOpen]);

  function resetForm() {
    setShareVisibility('public');
    setInvitedEmails([]);
    setNewEmail('');
    setExpiresInDays(7);
    setCreatedShareURL('');
    setError('');
    setValidationErrors(undefined);
  }

  async function fetchShares() {
    setLoadingShares(true);
    setError('');
    try {
      const data = await sessionsAPI.getShares(sessionId);
      setShares(data);
    } catch (err) {
      console.error('Failed to load shares:', err);
      setError('Failed to load existing shares');
    } finally {
      setLoadingShares(false);
    }
  }

  function addEmail() {
    const email = newEmail.trim();

    if (!email) return;

    // Validate email with Zod
    const result = emailSchema.safeParse(email);
    if (!result.success) {
      setError(result.error.issues[0].message);
      return;
    }

    if (invitedEmails.includes(email)) {
      setError('Email already added');
      return;
    }

    setInvitedEmails([...invitedEmails, email]);
    setNewEmail('');
    setError('');
    setValidationErrors(undefined);
  }

  function removeEmail(email: string) {
    setInvitedEmails(invitedEmails.filter((e) => e !== email));
  }

  async function createShare() {
    setLoading(true);
    setError('');
    setValidationErrors(undefined);

    // Validate form data with Zod
    const formData: ShareFormData = {
      visibility: shareVisibility,
      invited_emails: shareVisibility === 'private' ? invitedEmails : [],
      expires_in_days: expiresInDays,
    };

    const validation = validateForm(shareFormSchema, formData);

    if (!validation.success) {
      setValidationErrors(validation.errors);
      setLoading(false);
      return;
    }

    try {
      const result = await sessionsAPI.createShare(sessionId, validation.data);
      setCreatedShareURL(result.share_url);
      await fetchShares();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create share');
    } finally {
      setLoading(false);
    }
  }

  async function revokeShare(shareToken: string) {
    if (!confirm('Are you sure you want to revoke this share?')) {
      return;
    }

    setError('');
    try {
      await sessionsAPI.revokeShare(shareToken);
      await fetchShares();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to revoke share');
    }
  }

  if (!isOpen) return null;

  return (
    <div className={styles.modalOverlay} onClick={onClose}>
      <div className={styles.modal} onClick={(e) => e.stopPropagation()}>
        <div className={styles.modalHeader}>
          <h2>Share Session</h2>
          <button className={styles.closeBtn} onClick={onClose}>
            ×
          </button>
        </div>

        <div className={styles.modalBody}>
          {error && <ErrorDisplay message={error} />}
          {validationErrors && getFieldError(validationErrors, 'invited_emails') && (
            <ErrorDisplay message={getFieldError(validationErrors, 'invited_emails')!} />
          )}

          {createdShareURL ? (
            <div className={styles.successMessage}>
              <h3>✓ Share Link Created</h3>
              <div className={styles.shareUrlBox}>
                <input
                  type="text"
                  readOnly
                  value={createdShareURL}
                  className={styles.shareUrlInput}
                />
                <Button
                  size="sm"
                  onClick={() => copy(createdShareURL)}
                >
                  {copied ? 'Copied!' : 'Copy'}
                </Button>
              </div>
            </div>
          ) : (
            <>
              <div className={styles.formGroup}>
                <label>
                  <input
                    type="radio"
                    checked={shareVisibility === 'public'}
                    onChange={() => setShareVisibility('public')}
                  />
                  <strong>Public</strong> - Anyone with link
                </label>
                <label>
                  <input
                    type="radio"
                    checked={shareVisibility === 'private'}
                    onChange={() => setShareVisibility('private')}
                  />
                  <strong>Private</strong> - Invite specific people
                </label>
              </div>

              {shareVisibility === 'private' && (
                <FormField
                  label="Invite by email"
                  required
                  error={getFieldError(validationErrors, 'invited_emails')}
                >
                  <div className={styles.emailInputGroup}>
                    <input
                      type="email"
                      value={newEmail}
                      onChange={(e) => {
                        setNewEmail(e.target.value);
                        setError('');
                      }}
                      placeholder="email@example.com"
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') {
                          e.preventDefault();
                          addEmail();
                        }
                      }}
                    />
                    <Button size="sm" onClick={addEmail}>
                      Add
                    </Button>
                  </div>
                  {invitedEmails.length > 0 && (
                    <div className={styles.emailList}>
                      {invitedEmails.map((email) => (
                        <span key={email} className={styles.emailTag}>
                          {email}
                          <button className={styles.removeBtn} onClick={() => removeEmail(email)}>
                            ×
                          </button>
                        </span>
                      ))}
                    </div>
                  )}
                </FormField>
              )}

              <div className={styles.formGroup}>
                <label>Expires:</label>
                <select
                  value={expiresInDays ?? 'null'}
                  onChange={(e) =>
                    setExpiresInDays(e.target.value === 'null' ? null : Number(e.target.value))
                  }
                >
                  <option value={1}>1 day</option>
                  <option value={7}>7 days</option>
                  <option value={30}>30 days</option>
                  <option value="null">Never</option>
                </select>
              </div>

              <div className={styles.modalFooter}>
                <Button
                  variant="primary"
                  onClick={createShare}
                  disabled={loading}
                >
                  {loading ? 'Creating...' : 'Create Share Link'}
                </Button>
                <Button variant="secondary" onClick={onClose}>
                  Cancel
                </Button>
              </div>
            </>
          )}

          <div className={styles.sharesList}>
            <h3>Active Shares</h3>
            {loadingShares ? (
              <p>Loading...</p>
            ) : shares.length === 0 ? (
              <p className={styles.empty}>No active shares</p>
            ) : (
              shares.map((share) => (
                <div key={share.share_token} className={styles.shareItem}>
                  <div className={styles.shareInfo}>
                    <span className={`${styles.visibilityBadge} ${styles[share.visibility]}`}>
                      {share.visibility}
                    </span>
                    {share.visibility === 'private' && share.invited_emails && (
                      <span className={styles.invited}>{share.invited_emails.join(', ')}</span>
                    )}
                    {share.expires_at ? (
                      <span className={styles.expires}>Expires: {formatDate(share.expires_at)}</span>
                    ) : (
                      <span className={styles.neverExpires}>Never expires</span>
                    )}
                  </div>
                  <Button
                    variant="danger"
                    size="sm"
                    onClick={() => revokeShare(share.share_token)}
                  >
                    Revoke
                  </Button>
                </div>
              ))
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

export default ShareDialog;
