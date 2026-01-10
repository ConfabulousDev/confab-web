import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import type { SessionShare } from '@/types';
import { formatDateString } from '@/utils';
import FormField from './FormField';
import Button from './Button';
import styles from './ShareDialog.module.css';

/**
 * ShareDialog is a modal for creating and managing session shares.
 *
 * Since the real component uses hooks (useAuth, useShareDialog, useCopyToClipboard),
 * we create a presentational version for Storybook that accepts all state as props.
 */

interface ShareDialogPresentationalProps {
  // Form state
  isPublic: boolean;
  onPublicChange: (isPublic: boolean) => void;
  recipients: string[];
  newEmail: string;
  onNewEmailChange: (email: string) => void;
  onAddEmail: () => void;
  onRemoveEmail: (email: string) => void;
  expiresInDays: number | null;
  onExpiresChange: (days: number | null) => void;

  // Share state
  createdShareURL: string;
  shares: SessionShare[];

  // Loading/error state
  loading: boolean;
  loadingShares: boolean;
  error: string;

  // Actions
  onCreateShare: () => void;
  onRevokeShare: (shareId: number) => void;
  onClose: () => void;
  onCopy: () => void;
  copied: boolean;
}

function ShareDialogPresentational({
  isPublic,
  onPublicChange,
  recipients,
  newEmail,
  onNewEmailChange,
  onAddEmail,
  onRemoveEmail,
  expiresInDays,
  onExpiresChange,
  createdShareURL,
  shares,
  loading,
  loadingShares,
  error,
  onCreateShare,
  onRevokeShare,
  onClose,
  onCopy,
  copied,
}: ShareDialogPresentationalProps) {
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
          <p className={styles.disclaimer}>
            Best-effort redaction is applied to sensitive data. A quick review before sharing is recommended.
          </p>

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
                <Button size="sm" onClick={onCopy}>
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
                    onChange={() => onPublicChange(true)}
                  />
                  <strong>Public</strong> - Anyone with link
                </label>
                <label>
                  <input
                    type="radio"
                    checked={!isPublic}
                    onChange={() => onPublicChange(false)}
                  />
                  <strong>Private</strong> - Invite specific people
                </label>
              </div>

              {!isPublic && (
                <FormField
                  label="Invite by email"
                  required
                  error={error}
                >
                  <div className={styles.emailInputGroup}>
                    <input
                      type="email"
                      value={newEmail}
                      onChange={(e) => onNewEmailChange(e.target.value)}
                      placeholder="email@example.com"
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') {
                          e.preventDefault();
                          onAddEmail();
                        }
                      }}
                    />
                    <Button size="sm" onClick={onAddEmail}>
                      Add
                    </Button>
                  </div>
                  {recipients.length > 0 && (
                    <div className={styles.emailList}>
                      {recipients.map((email) => (
                        <span key={email} className={styles.emailTag}>
                          {email}
                          <button className={styles.removeBtn} onClick={() => onRemoveEmail(email)}>
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
                    onExpiresChange(e.target.value === 'null' ? null : Number(e.target.value))
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
                  onClick={onCreateShare}
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
                    onClick={() => onRevokeShare(share.id)}
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

// Interactive wrapper that manages state
function ShareDialogInteractive(props: Partial<ShareDialogPresentationalProps>) {
  const [isPublic, setIsPublic] = useState(props.isPublic ?? true);
  const [recipients, setRecipients] = useState<string[]>(props.recipients ?? []);
  const [newEmail, setNewEmail] = useState(props.newEmail ?? '');
  const [expiresInDays, setExpiresInDays] = useState<number | null>(props.expiresInDays ?? 7);
  const [copied, setCopied] = useState(false);

  const handleAddEmail = () => {
    if (newEmail.trim() && !recipients.includes(newEmail.trim())) {
      setRecipients([...recipients, newEmail.trim()]);
      setNewEmail('');
    }
  };

  const handleRemoveEmail = (email: string) => {
    setRecipients(recipients.filter((e) => e !== email));
  };

  const handleCopy = () => {
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <ShareDialogPresentational
      isPublic={isPublic}
      onPublicChange={setIsPublic}
      recipients={recipients}
      newEmail={newEmail}
      onNewEmailChange={setNewEmail}
      onAddEmail={handleAddEmail}
      onRemoveEmail={handleRemoveEmail}
      expiresInDays={expiresInDays}
      onExpiresChange={setExpiresInDays}
      createdShareURL={props.createdShareURL ?? ''}
      shares={props.shares ?? []}
      loading={props.loading ?? false}
      loadingShares={props.loadingShares ?? false}
      error={props.error ?? ''}
      onCreateShare={props.onCreateShare ?? (() => alert('Create share clicked'))}
      onRevokeShare={props.onRevokeShare ?? ((id) => alert(`Revoke share ${id}`))}
      onClose={props.onClose ?? (() => alert('Close clicked'))}
      onCopy={handleCopy}
      copied={copied}
    />
  );
}

// Sample data
const sampleShares: SessionShare[] = [
  {
    id: 1,
    session_id: 'session-123',
    external_id: 'abc123def456',
    is_public: true,
    recipients: null,
    expires_at: new Date(Date.now() + 7 * 24 * 60 * 60 * 1000).toISOString(), // 7 days from now
    created_at: new Date().toISOString(),
    last_accessed_at: null,
  },
  {
    id: 2,
    session_id: 'session-123',
    external_id: 'abc123def456',
    is_public: false,
    recipients: ['alice@example.com', 'bob@example.com'],
    expires_at: null,
    created_at: new Date(Date.now() - 2 * 24 * 60 * 60 * 1000).toISOString(), // 2 days ago
    last_accessed_at: new Date().toISOString(),
  },
];

const meta: Meta<typeof ShareDialogInteractive> = {
  title: 'Components/ShareDialog',
  component: ShareDialogInteractive,
  parameters: {
    layout: 'centered',
  },
};

export default meta;
type Story = StoryObj<typeof ShareDialogInteractive>;

// Default state - public share form
export const Default: Story = {
  args: {},
};

// Private share form selected
export const PrivateShare: Story = {
  args: {
    isPublic: false,
  },
};

// Private share with recipients added
export const WithRecipients: Story = {
  args: {
    isPublic: false,
    recipients: ['alice@example.com', 'bob@example.com', 'charlie@example.com'],
  },
};

// Success state after share is created
export const ShareCreated: Story = {
  args: {
    createdShareURL: 'https://app.confabulous.dev/sessions/abc123def456',
  },
};

// With existing active shares
export const WithActiveShares: Story = {
  args: {
    shares: sampleShares,
  },
};

// Loading state while creating
export const Loading: Story = {
  args: {
    loading: true,
  },
};

// Loading shares list
export const LoadingShares: Story = {
  args: {
    loadingShares: true,
  },
};

// With validation error
export const WithError: Story = {
  args: {
    isPublic: false,
    error: 'At least one recipient is required for private shares',
  },
};

// Complete flow: success with active shares
export const SuccessWithShares: Story = {
  args: {
    createdShareURL: 'https://app.confabulous.dev/sessions/abc123def456',
    shares: sampleShares,
  },
};
