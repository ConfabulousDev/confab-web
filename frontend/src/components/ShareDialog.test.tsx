import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import type { SessionShare } from '@/types';
import ShareDialog from './ShareDialog';

vi.mock('@/hooks', () => ({
  useAuth: () => ({ user: { email: 'owner@example.com' } }),
  useCopyToClipboard: () => ({ copy: vi.fn(), copied: false }),
  useShareDialog: vi.fn(),
}));

import { useShareDialog } from '@/hooks';

function makeShare(overrides: Partial<SessionShare> = {}): SessionShare {
  return {
    id: 1,
    session_id: 'session-1',
    external_id: 'ext-1',
    is_public: false,
    recipients: null,
    expires_at: null,
    created_at: '2024-01-01T00:00:00Z',
    last_accessed_at: null,
    ...overrides,
  };
}

function mockShareDialog(overrides: Record<string, unknown> = {}): void {
  vi.mocked(useShareDialog).mockReturnValue({
    isPublic: true,
    setIsPublic: vi.fn(),
    recipients: [],
    newEmail: '',
    setNewEmail: vi.fn(),
    expiresInDays: 7,
    setExpiresInDays: vi.fn(),
    createdShareURL: '',
    shares: [],
    loading: false,
    loadingShares: false,
    error: '',
    validationErrors: undefined,
    addEmail: vi.fn(),
    removeEmail: vi.fn(),
    createShare: vi.fn(),
    revokeShare: vi.fn(),
    resetForm: vi.fn(),
    fetchShares: vi.fn(),
    ...overrides,
  });
}

function renderDialog(): void {
  render(<ShareDialog sessionId="session-1" isOpen={true} onClose={vi.fn()} />);
}

describe('ShareDialog', () => {
  it('shows recipient emails for a private share with recipients', () => {
    mockShareDialog({
      shares: [makeShare({ recipients: ['alice@example.com', 'bob@example.com'] })],
    });

    renderDialog();
    expect(screen.getByText('alice@example.com, bob@example.com')).toBeInTheDocument();
  });

  it('shows "No recipients" for a private share with empty recipients', () => {
    mockShareDialog({
      shares: [makeShare({ recipients: [] })],
    });

    renderDialog();
    expect(screen.getByText('No recipients')).toBeInTheDocument();
  });

  it('shows "No recipients" for a private share with null recipients', () => {
    mockShareDialog({
      shares: [makeShare({ recipients: null })],
    });

    renderDialog();
    expect(screen.getByText('No recipients')).toBeInTheDocument();
  });

  it('does not show "No recipients" for a public share', () => {
    mockShareDialog({
      shares: [makeShare({ is_public: true })],
    });

    renderDialog();
    expect(screen.queryByText('No recipients')).not.toBeInTheDocument();
  });
});
