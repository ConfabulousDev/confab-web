// CF-364 — Summary tab on Codex sessions must render the same
// SessionSummaryPanel as Claude sessions, not the CodexSummaryEmpty placeholder.
//
// CF-386 — SessionViewer owns parsed Codex transcript state (mirroring Claude)
// and derives the model via `extractCodexModel(rawLines)`, which walks the
// rollout for session_meta.model → turn_context.model. Replaces CF-383's
// line-1-only `fetchCodexSessionMeta` approach.

import { isValidElement } from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import SessionViewer from './SessionViewer';
import type { TranscriptLine } from '@/types';
import { countClaudeCategories } from './claudeCategories';
import {
  agentToolUse,
  asyncAgentResult,
  subagentAssistantText,
} from '@/test-fixtures/claudeSubagent';
import type { SessionDetail } from '@/schemas/api';
import type { SessionAnalytics } from '@/schemas/api';
import { makeSessionDetailFixture } from '@/test-fixtures/session';
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

// Stub heavy transcript panes — we're only asserting routing. The Claude stub
// captures its props so et0r thread tests can inspect items / activeThreadId
// and invoke onOpenThread.
const claudePaneProps: { current: Record<string, unknown> | undefined } = { current: undefined };
vi.mock('./ClaudeTranscriptPane', () => ({
  default: (props: Record<string, unknown>) => {
    claudePaneProps.current = props;
    return <div data-testid="claude-transcript-pane" />;
  },
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

// Use the shared `makeSessionDetailFixture` helper but override id/external_id
// so the long-standing `codex-session-uuid` literal that tests assert on is
// preserved. Default provider stays codex since most assertions exercise the
// Codex render path.
function makeSession(overrides: Partial<SessionDetail> = {}): SessionDetail {
  return makeSessionDetailFixture('codex', {
    id: 'codex-session-uuid',
    external_id: 'codex-ext-id',
    owner_email: 'codex@example.com',
    ...overrides,
  });
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

// et0r: subagent subtabs under the Transcript tab.
describe('SessionViewer / subagent thread tabs', () => {
  const claudeSession = makeSession({
    provider: 'claude-code',
    files: [
      {
        file_name: 'transcript.jsonl',
        file_type: 'transcript',
        last_synced_line: 3,
        updated_at: '2026-09-13T10:00:00Z',
      },
    ],
  });

  const mainMessages: TranscriptLine[] = [
    agentToolUse({ uuid: 'u1', toolUseId: 't1', description: 'Explore the codebase', subagentType: 'Explore' }),
    asyncAgentResult({ uuid: 'r1', toolUseId: 't1', agentId: 'a1', description: 'Explore the codebase' }),
    agentToolUse({ uuid: 'u2', toolUseId: 't2', description: 'Write tests' }),
    asyncAgentResult({ uuid: 'r2', toolUseId: 't2', agentId: 'a2', description: 'Write tests' }),
  ];

  const a1Messages: TranscriptLine[] = [
    subagentAssistantText('s1', 'a1', 'Looking around'),
    agentToolUse({ uuid: 's2', toolUseId: 'tn', description: 'Nested helper', agentId: 'a1' }),
    asyncAgentResult({ uuid: 's3', toolUseId: 'tn', agentId: 'child', description: 'Nested helper' }),
  ];

  const childMessages: TranscriptLine[] = [subagentAssistantText('c1', 'child', 'Helping')];

  function viewer(props: Partial<React.ComponentProps<typeof SessionViewer>> = {}) {
    return (
      <MemoryRouter>
        <SessionViewer
          session={claudeSession}
          activeTab="transcript"
          onTabChange={() => {}}
          initialMessages={mainMessages}
          initialThreadMessages={{ a1: a1Messages, child: childMessages }}
          {...props}
        />
      </MemoryRouter>
    );
  }

  function filterCounts() {
    const slot = headerProps.current?.filterSlot;
    if (!isValidElement<{ counts: unknown }>(slot)) throw new Error('filterSlot not rendered');
    return slot.props.counts;
  }

  function openThread(threadId: string) {
    const onOpenThread = claudePaneProps.current?.onOpenThread;
    if (typeof onOpenThread !== 'function') throw new Error('onOpenThread not provided to the pane');
    onOpenThread(threadId);
  }

  it('renders no strip when the session has no subagents', () => {
    render(viewer({ initialMessages: [subagentAssistantText('x', 'none', 'plain')] }));
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
  });

  it('renders Main plus one tab per main-launched subagent, in launch order', () => {
    render(viewer());
    const tabs = screen.getAllByRole('tab');
    expect(tabs.map((t) => t.textContent)).toEqual(['Main', 'Explore the codebase', 'Write tests']);
  });

  it('hides the strip on the Summary tab', () => {
    render(viewer({ activeTab: 'summary', initialAnalytics: codexAnalytics }));
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
  });

  it('calls onThreadChange when a subagent tab is clicked (controlled)', async () => {
    const user = userEvent.setup();
    const onThreadChange = vi.fn();
    render(viewer({ activeThreadId: null, onThreadChange }));
    await user.click(screen.getByRole('tab', { name: 'Write tests' }));
    expect(onThreadChange).toHaveBeenCalledWith('a2', undefined);
  });

  it('switches pane items and header counts to the opened thread (uncontrolled)', () => {
    render(viewer());
    expect(claudePaneProps.current?.allMessages).toEqual(mainMessages);
    expect(filterCounts()).toEqual(countClaudeCategories(mainMessages));

    act(() => {
      openThread('a1');
    });

    expect(claudePaneProps.current?.activeThreadId).toBe('a1');
    expect(claudePaneProps.current?.allMessages).toEqual(a1Messages);
    expect(filterCounts()).toEqual(countClaudeCategories(a1Messages));
    expect(screen.getByRole('tab', { name: 'Explore the codebase' })).toHaveAttribute('aria-selected', 'true');
  });

  it('opens an id-labeled tab for a deep-linked agent not launched from Main, without a back link', () => {
    render(viewer({ activeThreadId: 'zz-unknown', onThreadChange: () => {} }));
    expect(screen.getByRole('tab', { name: 'zz-unknown' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.queryByRole('button', { name: /Launched from/ })).not.toBeInTheDocument();
  });

  it('adds a nested agent opened from inside a subagent tab, labeled with its parent', () => {
    const onThreadChange = vi.fn();
    const { rerender } = render(viewer({ activeThreadId: 'a1', onThreadChange }));

    act(() => {
      openThread('child');
    });
    expect(onThreadChange).toHaveBeenCalledWith('child', undefined);

    rerender(viewer({ activeThreadId: 'child', onThreadChange }));
    expect(screen.getByRole('tab', { name: 'Explore the codebase › Nested helper' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    expect(screen.getByRole('button', { name: '← Launched from Explore the codebase' })).toBeInTheDocument();
  });

  it('back link returns to Main at the launching row', async () => {
    const user = userEvent.setup();
    const onThreadChange = vi.fn();
    render(viewer({ activeThreadId: 'a1', onThreadChange }));
    await user.click(screen.getByRole('button', { name: '← Launched from Main' }));
    expect(onThreadChange).toHaveBeenCalledWith(null, 'r1');
  });

  it('renders no strip for Codex sessions even with a thread id', async () => {
    render(
      <MemoryRouter>
        <SessionViewer
          session={makeSession()}
          activeTab="transcript"
          onTabChange={() => {}}
          activeThreadId="a1"
          onThreadChange={() => {}}
        />
      </MemoryRouter>,
    );
    await waitFor(() => expect(fetchParsedCodexTranscript).toHaveBeenCalled());
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
  });
});
