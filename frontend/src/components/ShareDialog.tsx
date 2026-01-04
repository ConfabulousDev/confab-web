import { useEffect } from 'react';
import { useCopyToClipboard, useAuth, useShareDialog } from '@/hooks';
import { formatDateString } from '@/utils';
import { getFieldError } from '@/schemas/validation';
import Modal from './Modal';
import FormField from './FormField';
import Button from './Button';
import styles from './ShareDialog.module.css';

interface ShareDialogProps {
  sessionId: string;
  isOpen: boolean;
  onClose: () => void;
}

function ShareDialog({ sessionId, isOpen, onClose }: ShareDialogProps) {
  const { user } = useAuth();
  const { copy, copied } = useCopyToClipboard();

  const {
    isPublic,
    setIsPublic,
    recipients,
    newEmail,
    setNewEmail,
    expiresInDays,
    setExpiresInDays,
    createdShareURL,
    shares,
    loading,
    loadingShares,
    error,
    validationErrors,
    addEmail,
    removeEmail,
    createShare,
    revokeShare,
    resetForm,
    fetchShares,
  } = useShareDialog({
    sessionId,
    userEmail: user?.email,
  });

  useEffect(() => {
    if (isOpen) {
      resetForm();
      fetchShares();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen]);

  return (
    <Modal isOpen={isOpen} onClose={onClose} className={styles.modal} ariaLabel="Share Session" showCloseButton={false}>
      <div className={styles.modalHeader}>
        <h2>Share Session</h2>
        <button className={styles.closeBtn} onClick={onClose} aria-label="Close">
          ×
        </button>
      </div>

      <div className={styles.modalBody}>
          {createdShareURL ? (
            <div className={styles.successMessage}>
              <h3>✓ Share Created</h3>
              <p className={styles.shareUrlLabel}>Session link:</p>
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
                    checked={isPublic}
                    onChange={() => setIsPublic(true)}
                  />
                  <strong>Public</strong> - Anyone with link
                </label>
                <label>
                  <input
                    type="radio"
                    checked={!isPublic}
                    onChange={() => setIsPublic(false)}
                  />
                  <strong>Private</strong> - Invite specific people
                </label>
              </div>

              {!isPublic && (
                <FormField
                  label="Invite by email"
                  required
                  error={error || getFieldError(validationErrors, 'recipients')}
                >
                  <div className={styles.emailInputGroup}>
                    <input
                      type="email"
                      value={newEmail}
                      onChange={(e) => setNewEmail(e.target.value)}
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
                  {recipients.length > 0 && (
                    <div className={styles.emailList}>
                      {recipients.map((email) => (
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
                  {loading ? 'Creating...' : 'Create Share'}
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
                <div key={share.id} className={styles.shareItem}>
                  <div className={styles.shareInfo}>
                    <span className={`${styles.visibilityBadge} ${share.is_public ? styles.public : styles.private}`}>
                      {share.is_public ? 'public' : 'private'}
                    </span>
                    {!share.is_public && share.recipients && (
                      <span className={styles.invited}>{share.recipients.join(', ')}</span>
                    )}
                    {share.expires_at ? (
                      <span className={styles.expires}>Expires: {formatDateString(share.expires_at)}</span>
                    ) : (
                      <span className={styles.neverExpires}>Never expires</span>
                    )}
                  </div>
                  <Button
                    variant="danger"
                    size="sm"
                    onClick={() => revokeShare(share.id)}
                  >
                    Revoke
                  </Button>
                </div>
              ))
            )}
          </div>
      </div>
    </Modal>
  );
}

export default ShareDialog;
