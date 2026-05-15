// CF-364 — Summary tab on Codex sessions must render the same
// SessionSummaryPanel as Claude sessions, not the CodexSummaryEmpty placeholder.
//
// CF-386 — SessionViewer owns parsed Codex transcript state (mirroring Claude)
// and derives the model via `extractCodexModel(rawLines)`, which walks the
// rollout for session_meta.model → turn_context.model. Replaces CF-383's
// line-1-only `fetchCodexSessionMeta` approach.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import SessionViewer from './SessionViewer';
import type { SessionDetail } from '@/schemas/api';
import type { SessionAnalytics } from '@/schemas/api';
import {
  fetchParsedCodexTranscript,
  parseCodexJSONL,
  type ParsedCodexTranscript,
} from '@/services/codexTranscriptService';
import type { RawCodexLine } from '@/schemas/codexTranscript';

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
// Capture props in `headerProps` so tests can assert what model SessionViewer
// plumbed through.
const headerProps: { current: Record<string, unknown> | undefined } = { current: undefined };
vi.mock('./SessionHeader', () => ({
  default: (props: Record<string, unknown>) => {
    headerProps.current = props;
    return <div data-testid="session-header" />;
  },
}));

// CF-386: SessionViewer owns the Codex rollout fetch (lifted from
// CodexTranscriptPane). Mock `fetchParsedCodexTranscript` so tests can return
// rawLines with different model configurations and assert what reaches
// SessionHeader via `extractCodexModel`.
vi.mock('@/services/codexTranscriptService', async () => {
  const actual =
    await vi.importActual<typeof import('@/services/codexTranscriptService')>(
      '@/services/codexTranscriptService'
    );
  return {
    ...actual,
    fetchParsedCodexTranscript: vi.fn(() =>
      Promise.resolve({
        sessionId: 'codex-session-uuid',
        items: [],
        rawLines: [],
        validationErrors: [],
        totalLines: 0,
        metadata: { itemCount: 0, rawLineCount: 0, parseErrorCount: 0 },
      })
    ),
  };
});

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

// CF-386: SessionViewer owns the parsed Codex rollout (lifted from
// CodexTranscriptPane). The Codex model meta-item is derived from the
// rawLines via `extractCodexModel`, with the same session_meta → turn_context
// fallback the backend parser uses.
describe('SessionViewer / Codex transcript lift', () => {
  // Build a schema-validated `RawCodexLine` from a single JSONL snippet — keeps
  // tests in terms of the wire shape (matches `extractCodexModel`'s test style).
  // Throws on parse failure so a malformed test fixture surfaces immediately.
  function rawLine(jsonl: string): RawCodexLine {
    const line = parseCodexJSONL(jsonl).rawLines[0];
    if (!line) throw new Error(`rawLine helper: failed to parse ${jsonl}`);
    return line;
  }

  function parsedResult(rawLines: RawCodexLine[]): ParsedCodexTranscript {
    return {
      sessionId: 'codex-session-uuid',
      items: [],
      rawLines,
      validationErrors: [],
      totalLines: rawLines.length,
      metadata: {
        itemCount: 0,
        rawLineCount: rawLines.length,
        parseErrorCount: 0,
      },
    };
  }

  function renderViewer(session: SessionDetail = makeSession()) {
    render(
      <MemoryRouter>
        <SessionViewer
          session={session}
          activeTab="summary"
          onTabChange={() => {}}
          initialAnalytics={codexAnalytics}
        />
      </MemoryRouter>
    );
  }

  it('derives Codex model from session_meta and passes it to SessionHeader', async () => {
    vi.mocked(fetchParsedCodexTranscript).mockResolvedValueOnce(
      parsedResult([
        rawLine(
          '{"timestamp":"2026-05-13T01:00:00Z","type":"session_meta","payload":{"id":"x","model":"gpt-5-codex"}}',
        ),
      ]),
    );

    renderViewer();

    await waitFor(() => {
      expect(headerProps.current?.model).toBe('gpt-5-codex');
    });
    expect(fetchParsedCodexTranscript).toHaveBeenCalledWith(
      'codex-session-uuid',
      'rollout.jsonl',
      true
    );
  });

  it('falls back to turn_context.model when session_meta has no model', async () => {
    vi.mocked(fetchParsedCodexTranscript).mockResolvedValueOnce(
      parsedResult([
        rawLine(
          '{"timestamp":"2026-05-13T01:00:00Z","type":"session_meta","payload":{"id":"x"}}',
        ),
        rawLine(
          '{"timestamp":"2026-05-13T01:00:01Z","type":"turn_context","payload":{"turn_id":"t1","model":"gpt-5"}}',
        ),
      ]),
    );

    renderViewer();

    await waitFor(() => {
      expect(headerProps.current?.model).toBe('gpt-5');
    });
  });

  it('passes undefined model to SessionHeader when no envelope carries model', async () => {
    vi.mocked(fetchParsedCodexTranscript).mockResolvedValueOnce(
      parsedResult([
        rawLine(
          '{"timestamp":"2026-05-13T01:00:00Z","type":"session_meta","payload":{"id":"x"}}',
        ),
      ]),
    );

    renderViewer();

    await waitFor(() => {
      expect(headerProps.current).toBeDefined();
    });
    expect(headerProps.current?.model).toBeUndefined();
  });

  it('does not call fetchParsedCodexTranscript for Claude sessions', async () => {
    renderViewer(makeSession({ provider: 'claude-code' }));

    await waitFor(() => {
      expect(headerProps.current).toBeDefined();
    });
    expect(fetchParsedCodexTranscript).not.toHaveBeenCalled();
  });
});
