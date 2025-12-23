import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useShareDialog } from './useShareDialog';

// Mock the API module
vi.mock('@/services/api', () => ({
  sessionsAPI: {
    getShares: vi.fn(),
    createShare: vi.fn(),
    revokeShare: vi.fn(),
  },
}));

import { sessionsAPI } from '@/services/api';

describe('useShareDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('initializes with default state', () => {
    const { result } = renderHook(() =>
      useShareDialog({ sessionId: 'test-session' })
    );

    expect(result.current.isPublic).toBe(true);
    expect(result.current.recipients).toEqual([]);
    expect(result.current.newEmail).toBe('');
    expect(result.current.expiresInDays).toBe(7);
    expect(result.current.createdShareURL).toBe('');
    expect(result.current.shares).toEqual([]);
    expect(result.current.loading).toBe(false);
    expect(result.current.loadingShares).toBe(false);
    expect(result.current.error).toBe('');
    expect(result.current.validationErrors).toBeUndefined();
  });

  it('allows changing isPublic', () => {
    const { result } = renderHook(() =>
      useShareDialog({ sessionId: 'test-session' })
    );

    act(() => {
      result.current.setIsPublic(false);
    });

    expect(result.current.isPublic).toBe(false);
  });

  it('allows setting new email', () => {
    const { result } = renderHook(() =>
      useShareDialog({ sessionId: 'test-session' })
    );

    act(() => {
      result.current.setNewEmail('test@example.com');
    });

    expect(result.current.newEmail).toBe('test@example.com');
  });

  it('allows changing expiration days', () => {
    const { result } = renderHook(() =>
      useShareDialog({ sessionId: 'test-session' })
    );

    act(() => {
      result.current.setExpiresInDays(30);
    });

    expect(result.current.expiresInDays).toBe(30);

    act(() => {
      result.current.setExpiresInDays(null);
    });

    expect(result.current.expiresInDays).toBeNull();
  });

  it('adds valid email to recipients list', () => {
    const { result } = renderHook(() =>
      useShareDialog({ sessionId: 'test-session' })
    );

    act(() => {
      result.current.setNewEmail('valid@example.com');
    });

    act(() => {
      result.current.addEmail();
    });

    expect(result.current.recipients).toContain('valid@example.com');
    expect(result.current.newEmail).toBe('');
    expect(result.current.error).toBe('');
  });

  it('rejects invalid email', () => {
    const { result } = renderHook(() =>
      useShareDialog({ sessionId: 'test-session' })
    );

    act(() => {
      result.current.setNewEmail('invalid-email');
    });

    act(() => {
      result.current.addEmail();
    });

    expect(result.current.recipients).toEqual([]);
    expect(result.current.error).toBeTruthy();
  });

  it('prevents adding duplicate emails', () => {
    const { result } = renderHook(() =>
      useShareDialog({ sessionId: 'test-session' })
    );

    act(() => {
      result.current.setNewEmail('test@example.com');
    });
    act(() => {
      result.current.addEmail();
    });

    act(() => {
      result.current.setNewEmail('test@example.com');
    });
    act(() => {
      result.current.addEmail();
    });

    expect(result.current.recipients).toHaveLength(1);
    expect(result.current.error).toBe('Email already added');
  });

  it('prevents self-invite', () => {
    const { result } = renderHook(() =>
      useShareDialog({
        sessionId: 'test-session',
        userEmail: 'user@example.com',
      })
    );

    act(() => {
      result.current.setNewEmail('user@example.com');
    });
    act(() => {
      result.current.addEmail();
    });

    expect(result.current.recipients).toEqual([]);
    expect(result.current.error).toBe('You cannot invite yourself');
  });

  it('removes email from recipients list', () => {
    const { result } = renderHook(() =>
      useShareDialog({ sessionId: 'test-session' })
    );

    act(() => {
      result.current.setNewEmail('test@example.com');
    });
    act(() => {
      result.current.addEmail();
    });

    expect(result.current.recipients).toContain('test@example.com');

    act(() => {
      result.current.removeEmail('test@example.com');
    });

    expect(result.current.recipients).not.toContain('test@example.com');
  });

  it('resets form state', () => {
    const { result } = renderHook(() =>
      useShareDialog({ sessionId: 'test-session' })
    );

    // Modify state
    act(() => {
      result.current.setIsPublic(false);
      result.current.setNewEmail('test@example.com');
      result.current.setExpiresInDays(30);
    });

    // Reset
    act(() => {
      result.current.resetForm();
    });

    expect(result.current.isPublic).toBe(true);
    expect(result.current.recipients).toEqual([]);
    expect(result.current.newEmail).toBe('');
    expect(result.current.expiresInDays).toBe(7);
  });

  it('fetches shares successfully', async () => {
    const mockShares = [
      {
        id: 1,
        session_id: 'session-1',
        external_id: 'ext-1',
        is_public: true,
        created_at: '2024-01-01T00:00:00Z',
      },
      {
        id: 2,
        session_id: 'session-2',
        external_id: 'ext-2',
        is_public: false,
        created_at: '2024-01-02T00:00:00Z',
      },
    ];
    vi.mocked(sessionsAPI.getShares).mockResolvedValue(mockShares);

    const { result } = renderHook(() =>
      useShareDialog({ sessionId: 'test-session' })
    );

    await act(async () => {
      await result.current.fetchShares();
    });

    expect(sessionsAPI.getShares).toHaveBeenCalledWith('test-session');
    expect(result.current.shares).toEqual(mockShares);
    expect(result.current.loadingShares).toBe(false);
  });

  it('handles fetch shares error', async () => {
    vi.mocked(sessionsAPI.getShares).mockRejectedValue(new Error('Network error'));

    const { result } = renderHook(() =>
      useShareDialog({ sessionId: 'test-session' })
    );

    await act(async () => {
      await result.current.fetchShares();
    });

    expect(result.current.error).toBe('Failed to load existing shares');
    expect(result.current.loadingShares).toBe(false);
  });

  it('creates share successfully', async () => {
    vi.mocked(sessionsAPI.createShare).mockResolvedValue({
      share_url: 'https://example.com/share/abc123',
    });
    vi.mocked(sessionsAPI.getShares).mockResolvedValue([]);

    const { result } = renderHook(() =>
      useShareDialog({ sessionId: 'test-session' })
    );

    await act(async () => {
      await result.current.createShare();
    });

    expect(sessionsAPI.createShare).toHaveBeenCalledWith('test-session', {
      is_public: true,
      recipients: [],
      expires_in_days: 7,
    });
    expect(result.current.createdShareURL).toBe('https://example.com/share/abc123');
    expect(result.current.loading).toBe(false);
  });

  it('validates non-public share requires recipients', async () => {
    const { result } = renderHook(() =>
      useShareDialog({ sessionId: 'test-session' })
    );

    act(() => {
      result.current.setIsPublic(false);
    });

    await act(async () => {
      await result.current.createShare();
    });

    expect(result.current.validationErrors).toBeDefined();
    expect(sessionsAPI.createShare).not.toHaveBeenCalled();
  });
});
