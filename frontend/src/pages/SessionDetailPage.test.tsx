import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, act } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { SessionDetail } from '@/types';
import SessionDetailPage from './SessionDetailPage';
import { makeSessionDetailFixture } from '@/test-fixtures/session';

// CF-367: parameterized over both providers via `describe.each` to prove the
// shell forwards `?msg=` opaquely. `vi.hoisted` shares a mutable ref with the
// `useLoadSession` mock factory so each iteration can swap the session in
// before render.
const sessionRef: { current: SessionDetail | null } = vi.hoisted(() => ({
  current: null,
}));

// Mock hooks
vi.mock('@/hooks', () => ({
  useAppConfig: () => ({ sharesEnabled: false }),
  useAuth: () => ({ isAuthenticated: true }),
  useDocumentTitle: vi.fn(),
  useSuccessMessage: () => ({ message: null, fading: false }),
  useLoadSession: () => ({
    session: sessionRef.current,
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

// CF-367: per-provider target shape. Claude uses a UUID-shaped string;
// Codex uses a non-UUID numeric string mirroring the CF-360 `lineId` format
// (raw JSONL line index as a string). The shell must forward both opaquely.
const PROVIDER_CASES = [
  { provider: 'claude-code' as const, target: 'target-uuid-123' },
  { provider: 'codex' as const, target: '12' },
];

describe.each(PROVIDER_CASES)(
  'SessionDetailPage deep-link / $provider',
  ({ provider, target }) => {
    beforeEach(() => {
      sessionViewerCalls.length = 0;
      sessionRef.current = makeSessionDetailFixture(provider, {
        id: 'test-session-uuid',
      });
    });

    function lastProps() {
      const call = sessionViewerCalls[sessionViewerCalls.length - 1];
      if (!call) throw new Error('SessionViewer was never rendered');
      return call[0];
    }

    it('passes targetId from msg search param to SessionViewer', () => {
      renderWithRouter(`/sessions/test-session-uuid?msg=${target}`);
      expect(lastProps().targetId).toBe(target);
    });

    it('forces transcript tab when msg param is present', () => {
      renderWithRouter(`/sessions/test-session-uuid?msg=${target}`);
      expect(lastProps().activeTab).toBe('transcript');
    });

    it('forces transcript tab even when tab=summary is set alongside msg', () => {
      renderWithRouter(`/sessions/test-session-uuid?tab=summary&msg=${target}`);
      expect(lastProps().activeTab).toBe('transcript');
    });

    it('passes undefined targetId when msg param is absent', () => {
      renderWithRouter('/sessions/test-session-uuid');
      expect(lastProps().targetId).toBeUndefined();
    });

    it('defaults to summary tab when no msg param', () => {
      renderWithRouter('/sessions/test-session-uuid');
      expect(lastProps().activeTab).toBe('summary');
    });

    it('clears msg param when onTabChange is called with summary', () => {
      renderWithRouter(`/sessions/test-session-uuid?tab=transcript&msg=${target}`);

      // Simulate switching to summary tab via the callback
      act(() => {
        lastProps().onTabChange('summary');
      });

      // After tab change, SessionViewer re-renders without msg
      expect(lastProps().targetId).toBeUndefined();
      expect(lastProps().activeTab).toBe('summary');
    });

    it('preserves msg param when switching to transcript tab', () => {
      renderWithRouter(`/sessions/test-session-uuid?tab=transcript&msg=${target}`);

      // Switching to transcript should NOT clear msg
      act(() => {
        lastProps().onTabChange('transcript');
      });

      expect(lastProps().targetId).toBe(target);
    });
  }
);

// et0r: `?agent=<agentId>` selects a subagent subtab under Transcript.
describe('SessionDetailPage subagent thread param', () => {
  beforeEach(() => {
    sessionViewerCalls.length = 0;
    sessionRef.current = makeSessionDetailFixture('claude-code', { id: 'test-session-uuid' });
  });

  function lastProps() {
    const call = sessionViewerCalls[sessionViewerCalls.length - 1];
    if (!call) throw new Error('SessionViewer was never rendered');
    return call[0];
  }

  it('passes the agent param as activeThreadId', () => {
    renderWithRouter('/sessions/test-session-uuid?tab=transcript&agent=a1');
    expect(lastProps().activeThreadId).toBe('a1');
  });

  it('passes null activeThreadId when the agent param is absent', () => {
    renderWithRouter('/sessions/test-session-uuid?tab=transcript');
    expect(lastProps().activeThreadId).toBeNull();
  });

  it('forces the transcript tab when the agent param is present', () => {
    renderWithRouter('/sessions/test-session-uuid?agent=a1');
    expect(lastProps().activeTab).toBe('transcript');
  });

  it('clears agent and msg when switching to the summary tab', () => {
    renderWithRouter('/sessions/test-session-uuid?tab=transcript&agent=a1&msg=m1');
    act(() => {
      lastProps().onTabChange('summary');
    });
    expect(lastProps().activeTab).toBe('summary');
    expect(lastProps().activeThreadId).toBeNull();
    expect(lastProps().targetId).toBeUndefined();
  });

  it('sets agent and clears msg on a thread change without a target', () => {
    renderWithRouter('/sessions/test-session-uuid?tab=transcript&msg=m1');
    act(() => {
      lastProps().onThreadChange('a2');
    });
    expect(lastProps().activeThreadId).toBe('a2');
    expect(lastProps().targetId).toBeUndefined();
    expect(lastProps().activeTab).toBe('transcript');
  });

  it('clears agent and sets msg when returning to Main at the launching row', () => {
    renderWithRouter('/sessions/test-session-uuid?tab=transcript&agent=a1');
    act(() => {
      lastProps().onThreadChange(null, 'launch-row');
    });
    expect(lastProps().activeThreadId).toBeNull();
    expect(lastProps().targetId).toBe('launch-row');
  });
});
