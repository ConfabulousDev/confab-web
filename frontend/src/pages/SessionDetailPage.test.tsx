import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, act } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { SessionDetail } from '@/types';
import SessionDetailPage from './SessionDetailPage';

// Mock session data
const mockSession: SessionDetail = {
  id: 'test-session-uuid',
  external_id: 'abc123def456',
  custom_title: null,
  summary: 'Test session summary',
  first_user_message: 'Help me test',
  first_seen: '2025-01-15T10:00:00Z',
  last_sync_at: '2025-01-15T12:30:00Z',
  cwd: '/test',
  transcript_path: '/test/transcript.jsonl',
  git_info: null,
  files: [],
  hostname: 'test-host',
  username: 'tester',
  is_owner: true,
  owner_email: 'test@example.com',
};

// Mock hooks
vi.mock('@/hooks', () => ({
  useAppConfig: () => ({ sharesEnabled: false }),
  useAuth: () => ({ isAuthenticated: true }),
  useDocumentTitle: vi.fn(),
  useSuccessMessage: () => ({ message: null, fading: false }),
  useLoadSession: () => ({
    session: mockSession,
    setSession: vi.fn(),
    loading: false,
    error: null,
    errorType: null,
  }),
}));

// Track SessionViewer render calls for prop inspection
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const sessionViewerCalls: any[][] = [];
vi.mock('@/components/session', () => ({
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  SessionViewer: (props: any) => {
    sessionViewerCalls.push([props]);
    return <div data-testid="session-viewer" />;
  },
}));

// Mock ShareDialog
vi.mock('@/components/ShareDialog', () => ({
  default: () => null,
}));

function renderWithRouter(initialEntry: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialEntry]}>
        <Routes>
          <Route path="/sessions/:id" element={<SessionDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );
}

describe('SessionDetailPage deep-link', () => {
  beforeEach(() => {
    sessionViewerCalls.length = 0;
  });

  function lastProps() {
    const call = sessionViewerCalls[sessionViewerCalls.length - 1];
    if (!call) throw new Error('SessionViewer was never rendered');
    return call[0];
  }

  it('passes targetMessageUuid from msg search param to SessionViewer', () => {
    renderWithRouter('/sessions/test-session-uuid?msg=target-uuid-123');
    expect(lastProps().targetMessageUuid).toBe('target-uuid-123');
  });

  it('forces transcript tab when msg param is present', () => {
    renderWithRouter('/sessions/test-session-uuid?msg=target-uuid-123');
    expect(lastProps().activeTab).toBe('transcript');
  });

  it('forces transcript tab even when tab=summary is set alongside msg', () => {
    renderWithRouter('/sessions/test-session-uuid?tab=summary&msg=target-uuid-123');
    expect(lastProps().activeTab).toBe('transcript');
  });

  it('passes undefined targetMessageUuid when msg param is absent', () => {
    renderWithRouter('/sessions/test-session-uuid');
    expect(lastProps().targetMessageUuid).toBeUndefined();
  });

  it('defaults to summary tab when no msg param', () => {
    renderWithRouter('/sessions/test-session-uuid');
    expect(lastProps().activeTab).toBe('summary');
  });

  it('clears msg param when onTabChange is called with summary', () => {
    renderWithRouter('/sessions/test-session-uuid?tab=transcript&msg=target-uuid-123');

    // Simulate switching to summary tab via the callback
    act(() => {
      lastProps().onTabChange('summary');
    });

    // After tab change, SessionViewer re-renders without msg
    expect(lastProps().targetMessageUuid).toBeUndefined();
    expect(lastProps().activeTab).toBe('summary');
  });

  it('preserves msg param when switching to transcript tab', () => {
    renderWithRouter('/sessions/test-session-uuid?tab=transcript&msg=target-uuid-123');

    // Switching to transcript should NOT clear msg
    act(() => {
      lastProps().onTabChange('transcript');
    });

    expect(lastProps().targetMessageUuid).toBe('target-uuid-123');
  });
});
