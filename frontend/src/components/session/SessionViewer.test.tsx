// CF-364 — Summary tab on Codex sessions must render the same
// SessionSummaryPanel as Claude sessions, not the CodexSummaryEmpty placeholder.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import SessionViewer from './SessionViewer';
import type { SessionDetail } from '@/schemas/api';
import type { SessionAnalytics } from '@/schemas/api';

// Mock useAnalyticsPolling so SessionSummaryPanel doesn't try to fetch.
// Passing initialAnalytics disables polling, but the hook is still invoked
// for its other return values.
vi.mock('@/hooks/useAnalyticsPolling', () => ({
  useAnalyticsPolling: vi.fn(() => ({
    analytics: null,
    loading: false,
    error: null,
    forceRefetch: vi.fn(),
    pollingState: 'idle',
    refetch: vi.fn(),
  })),
}));

// Stub the TIL list call — SessionViewer skips it for Codex, but for the
// non-Codex tab-switching baseline we don't want a real network call either.
vi.mock('@/services/api', async () => {
  const actual = await vi.importActual<typeof import('@/services/api')>(
    '@/services/api'
  );
  return {
    ...actual,
    tilsAPI: {
      listForSession: vi.fn(() => Promise.resolve({ tils: [] })),
    },
  };
});

// Stub heavy transcript panes — we're only asserting Summary-tab routing.
vi.mock('./ClaudeTranscriptPane', () => ({
  default: () => <div data-testid="claude-transcript-pane" />,
}));
vi.mock('./CodexTranscriptPane', () => ({
  default: () => <div data-testid="codex-transcript-pane" />,
}));
vi.mock('./GitHubLinksCard', () => ({
  default: () => null,
}));

// SessionHeader pulls in keyboard-shortcut context; render-only stub.
vi.mock('./SessionHeader', () => ({
  default: () => <div data-testid="session-header" />,
}));

function makeSession(overrides: Partial<SessionDetail> = {}): SessionDetail {
  return {
    id: 'codex-session-uuid',
    external_id: 'codex-ext-id',
    provider: 'codex',
    first_seen: '2026-05-13T01:00:00Z',
    files: [
      {
        file_name: 'rollout.jsonl',
        file_type: 'transcript',
        last_synced_line: 10,
        updated_at: '2026-05-13T01:00:00Z',
      },
    ],
    owner_email: 'codex@example.com',
    ...overrides,
  };
}

const codexAnalytics: SessionAnalytics = {
  computed_at: '2026-05-13T01:01:00Z',
  computed_lines: 10,
  tokens: { input: 800, output: 200, cache_creation: 0, cache_read: 200 },
  cost: { estimated_usd: '0.0123' },
  compaction: { auto: 0, manual: 0 },
  cards: {
    tokens: {
      input: 800,
      output: 200,
      cache_creation: 0,
      cache_read: 200,
      estimated_usd: '0.0123',
    },
  },
};

beforeEach(() => {
  vi.clearAllMocks();
});

describe('SessionViewer / Summary tab on Codex sessions', () => {
  it('renders SessionSummaryPanel (not CodexSummaryEmpty) when activeTab is summary', () => {
    render(
      <MemoryRouter>
        <SessionViewer
          session={makeSession()}
          activeTab="summary"
          onTabChange={() => {}}
          initialAnalytics={codexAnalytics}
        />
      </MemoryRouter>
    );

    // SessionSummaryPanel's heading must be present.
    expect(screen.getByText('Session Summary')).toBeInTheDocument();

    // The old CodexSummaryEmpty placeholder text must NOT be in the DOM.
    expect(
      screen.queryByText(/Summary not yet available for Codex/i)
    ).not.toBeInTheDocument();
  });
});
