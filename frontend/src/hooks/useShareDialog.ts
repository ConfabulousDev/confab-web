import { useState, useCallback } from 'react';
import type { SessionShare } from '@/types';
import { sessionsAPI } from '@/services/api';
import { shareFormSchema, emailSchema, validateForm } from '@/schemas/validation';
import type { ShareFormData } from '@/schemas/validation';

interface UseShareDialogOptions {
  sessionId: string;
  userEmail?: string;
  onShareCreated?: (shareUrl: string) => void;
}

interface UseShareDialogReturn {
  // Form state
  visibility: 'public' | 'private';
  setVisibility: (v: 'public' | 'private') => void;
  invitedEmails: string[];
  newEmail: string;
  setNewEmail: (email: string) => void;
  expiresInDays: number | null;
  setExpiresInDays: (days: number | null) => void;

  // Share state
  createdShareURL: string;
  shares: SessionShare[];

  // Loading/error state
  loading: boolean;
  loadingShares: boolean;
  error: string;
  validationErrors: Record<string, string[]> | undefined;

  // Actions
  addEmail: () => void;
  removeEmail: (email: string) => void;
  createShare: () => Promise<void>;
  revokeShare: (shareToken: string) => Promise<void>;
  resetForm: () => void;
  fetchShares: () => Promise<void>;
}

/**
 * Hook to manage share dialog form state and API interactions
 */
export function useShareDialog({
  sessionId,
  userEmail,
  onShareCreated,
}: UseShareDialogOptions): UseShareDialogReturn {
  // Form state
  const [visibility, setVisibility] = useState<'public' | 'private'>('public');
  const [invitedEmails, setInvitedEmails] = useState<string[]>([]);
  const [newEmail, setNewEmail] = useState('');
  const [expiresInDays, setExpiresInDays] = useState<number | null>(7);

  // Share state
  const [createdShareURL, setCreatedShareURL] = useState('');
  const [shares, setShares] = useState<SessionShare[]>([]);

  // Loading/error state
  const [loading, setLoading] = useState(false);
  const [loadingShares, setLoadingShares] = useState(false);
  const [error, setError] = useState('');
  const [validationErrors, setValidationErrors] = useState<Record<string, string[]>>();

  const resetForm = useCallback(() => {
    setVisibility('public');
    setInvitedEmails([]);
    setNewEmail('');
    setExpiresInDays(7);
    setCreatedShareURL('');
    setError('');
    setValidationErrors(undefined);
  }, []);

  const fetchShares = useCallback(async () => {
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
  }, [sessionId]);

  const addEmail = useCallback(() => {
    const email = newEmail.trim().toLowerCase();

    if (!email) return;

    // Validate email with Zod
    const result = emailSchema.safeParse(email);
    if (!result.success) {
      setError(result.error.issues[0]?.message ?? 'Invalid email');
      return;
    }

    // Prevent self-invite
    if (userEmail && email === userEmail.toLowerCase()) {
      setError('You cannot invite yourself');
      return;
    }

    if (invitedEmails.some((e) => e.toLowerCase() === email)) {
      setError('Email already added');
      return;
    }

    setInvitedEmails((prev) => [...prev, email]);
    setNewEmail('');
    setError('');
    setValidationErrors(undefined);
  }, [newEmail, userEmail, invitedEmails]);

  const removeEmail = useCallback((email: string) => {
    setInvitedEmails((prev) => prev.filter((e) => e !== email));
  }, []);

  const createShare = useCallback(async () => {
    setLoading(true);
    setError('');
    setValidationErrors(undefined);

    // Validate form data with Zod
    const formData: ShareFormData = {
      visibility,
      invited_emails: visibility === 'private' ? invitedEmails : [],
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
      onShareCreated?.(result.share_url);
      await fetchShares();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create share');
    } finally {
      setLoading(false);
    }
  }, [sessionId, visibility, invitedEmails, expiresInDays, fetchShares, onShareCreated]);

  const revokeShare = useCallback(
    async (shareToken: string) => {
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
    },
    [fetchShares]
  );

  return {
    // Form state
    visibility,
    setVisibility,
    invitedEmails,
    newEmail,
    setNewEmail,
    expiresInDays,
    setExpiresInDays,

    // Share state
    createdShareURL,
    shares,

    // Loading/error state
    loading,
    loadingShares,
    error,
    validationErrors,

    // Actions
    addEmail,
    removeEmail,
    createShare,
    revokeShare,
    resetForm,
    fetchShares,
  };
}
