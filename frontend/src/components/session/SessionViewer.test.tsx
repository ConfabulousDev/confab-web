// CF-364 — Summary tab on Codex sessions must render the same
// SessionSummaryPanel as Claude sessions, not the CodexSummaryEmpty placeholder.
//
// CF-386 — SessionViewer owns parsed Codex transcript state (mirroring Claude)
// and derives the model via `extractCodexModel(rawLines)`, which walks the
// rollout for session_meta.model → turn_context.model. Replaces CF-383's
// line-1-only `fetchCodexSessionMeta` approach.

import { isValidElement } from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { act, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import SessionViewer from './SessionViewer';
import type { TranscriptLine } from '@/types';
import { countClaudeCategories } from './claudeCategories';
import {
  agentToolUse,
  asyncAgentResult,
  subagentAssistantText,
  taskNotificationMessage,
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

  // jgk8: a1 (depth 1) launches three background judges (depth 2); j1 launches
  // g1 (depth 3). Every launch is an `async_launched` background result.
  const a1Messages: TranscriptLine[] = [
    subagentAssistantText('s1', 'a1', 'Looking around'),
    agentToolUse({ uuid: 's2', toolUseId: 'tj1', description: 'Judge arrays r1', agentId: 'a1' }),
    asyncAgentResult({ uuid: 's3', toolUseId: 'tj1', agentId: 'j1', description: 'Judge arrays r1' }),
    agentToolUse({ uuid: 's4', toolUseId: 'tj2', description: 'Judge arrays r2', agentId: 'a1' }),
    asyncAgentResult({ uuid: 's5', toolUseId: 'tj2', agentId: 'j2', description: 'Judge arrays r2' }),
    agentToolUse({ uuid: 's6', toolUseId: 'tj3', description: 'Simplify Go changes', agentId: 'a1' }),
    asyncAgentResult({ uuid: 's7', toolUseId: 'tj3', agentId: 'j3', description: 'Simplify Go changes' }),
  ];

  const j1Messages: TranscriptLine[] = [
    subagentAssistantText('j1-1', 'j1', 'Judging'),
    agentToolUse({ uuid: 'j1-2', toolUseId: 'tg1', description: 'Re-run the flaky judge', agentId: 'j1' }),
    asyncAgentResult({ uuid: 'j1-3', toolUseId: 'tg1', agentId: 'g1', description: 'Re-run the flaky judge' }),
  ];

  const threadMessages: Record<string, TranscriptLine[]> = {
    a1: a1Messages,
    j1: j1Messages,
    j2: [subagentAssistantText('j2-1', 'j2', 'Judging r2')],
    g1: [subagentAssistantText('g1-1', 'g1', 'Re-running')],
  };

  function viewer(props: Partial<React.ComponentProps<typeof SessionViewer>> = {}) {
    return (
      <MemoryRouter>
        <SessionViewer
          session={claudeSession}
          activeTab="transcript"
          onTabChange={() => {}}
          initialMessages={mainMessages}
          initialThreadMessages={threadMessages}
          {...props}
        />
      </MemoryRouter>
    );
  }

  function crumbs() {
    return within(screen.getByRole('navigation', { name: 'Subagent path' }));
  }

  /** jgk8 D9 review: the open depth-1 agent's children live in the strip bar. */
  function stripLaunchedHere(count: number) {
    return screen.getByRole('button', { name: `Launched here (${count})` });
  }

  function stripLabels() {
    return screen.getAllByRole('tab').map((t) => t.textContent);
  }

  /** Controlled walk Main → a1 → … through the pane's "Open transcript", re-rendering as the page would. */
  function walk(path: string[], props: Partial<React.ComponentProps<typeof SessionViewer>> = {}) {
    const onThreadChange = vi.fn();
    const utils = render(viewer({ activeThreadId: path[0], onThreadChange, ...props }));
    for (const id of path.slice(1)) {
      act(() => {
        openThread(id);
      });
      utils.rerender(viewer({ activeThreadId: id, onThreadChange, ...props }));
    }
    onThreadChange.mockClear();
    return { onThreadChange, ...utils };
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
    await user.click(screen.getByRole('tab', { name: /^Write tests/ }));
    expect(onThreadChange).toHaveBeenCalledWith('a2', undefined);
  });

  it('names each strip chip with its subagent status', () => {
    render(viewer());
    expect(screen.getByRole('tab', { name: 'Explore the codebase, running' })).toBeInTheDocument();
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
    expect(screen.getByRole('tab', { name: /^Explore the codebase/ })).toHaveAttribute('aria-selected', 'true');
  });

  it('opens an id-labeled tab for a deep-linked agent not launched from Main, with no Go to parent', async () => {
    const user = userEvent.setup();
    render(viewer({ activeThreadId: 'zz-unknown', onThreadChange: () => {} }));
    const deepLinked = screen.getByRole('tab', { name: 'zz-unknown, status unknown' });
    expect(deepLinked).toHaveAttribute('aria-selected', 'true');
    await user.click(deepLinked);
    expect(screen.getByRole('menu')).toBeInTheDocument();
    expect(screen.queryByRole('menuitem', { name: /Go to parent/ })).not.toBeInTheDocument();
  });

  it('Go to parent returns to Main at the launching row', async () => {
    const user = userEvent.setup();
    const onThreadChange = vi.fn();
    render(viewer({ activeThreadId: 'a1', onThreadChange }));
    await user.click(screen.getByRole('tab', { name: /^Explore the codebase/ }));
    await user.click(screen.getByRole('menuitem', { name: 'Go to parent (Main)' }));
    expect(onThreadChange).toHaveBeenCalledWith(null, 'r1');
  });

  // jgk8 D4: the strip holds Main + Main's direct agents only.
  it('keeps nested agents out of the strip when opened from a subagent tab', () => {
    const { onThreadChange, rerender } = walk(['a1']);
    act(() => {
      openThread('j1');
    });
    expect(onThreadChange).toHaveBeenCalledWith('j1', undefined);
    rerender(viewer({ activeThreadId: 'j1', onThreadChange }));
    expect(stripLabels()).toEqual(['Main', 'Explore the codebase', 'Write tests']);
    expect(claudePaneProps.current?.activeThreadId).toBe('j1');
    expect(claudePaneProps.current?.allMessages).toEqual(j1Messages);
  });

  // jgk8 D7: the total comes from session.files; the footer counts the rest.
  it('counts every uploaded subagent file in All subagents, with a footer for the nested ones', async () => {
    const user = userEvent.setup();
    const file = (file_name: string, file_type = 'agent') => ({
      file_name,
      file_type,
      last_synced_line: 3,
      updated_at: '2026-09-13T10:00:00Z',
    });
    const session = {
      ...claudeSession,
      files: [
        file('transcript.jsonl', 'transcript'),
        ...['a1', 'a2', 'j1', 'j2', 'j3', 'g1'].map((id) => file(`agent-${id}.jsonl`)),
        file('subagents/workflows/run1/agent-w1.jsonl'),
      ],
    };
    render(viewer({ session }));
    await user.click(screen.getByRole('button', { name: 'All subagents (6)' }));
    expect(screen.getAllByRole('option').map((o) => o.textContent)).toEqual([
      expect.stringContaining('Main'),
      expect.stringContaining('Explore the codebase'),
      expect.stringContaining('Write tests'),
    ]);
    expect(screen.getByText('4 more launched by subagents. Open one to see the ones it launched.')).toBeInTheDocument();
  });

  it("floors the All subagents count at Main's direct agents and shows no footer when nothing is nested", async () => {
    const user = userEvent.setup();
    render(viewer());
    await user.click(screen.getByRole('button', { name: 'All subagents (2)' }));
    expect(screen.queryByText(/more launched by/)).not.toBeInTheDocument();
  });

  // jgk8 D9 review (trim + inline): depth 1 uses the strip; the row is for depth ≥ 2.
  it('shows no path row and no Launched here for a depth-1 agent that launched nothing', () => {
    render(viewer({ activeThreadId: 'a2', onThreadChange: () => {} }));
    expect(screen.queryByRole('navigation', { name: 'Subagent path' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^Launched here/ })).not.toBeInTheDocument();
  });

  it('puts "Launched here (n)" in the strip, not a second row, for a depth-1 agent that launched subagents', () => {
    render(viewer({ activeThreadId: 'a1', onThreadChange: () => {} }));
    expect(stripLaunchedHere(3)).toBeInTheDocument();
    expect(screen.queryByRole('navigation', { name: 'Subagent path' })).not.toBeInTheDocument();
  });

  it('opens a child picked from Launched here; the row starts at depth 2 and the depth-1 chip shows in path', async () => {
    const user = userEvent.setup();
    const onThreadChange = vi.fn();
    const { rerender } = render(viewer({ activeThreadId: 'a1', onThreadChange }));
    await user.click(stripLaunchedHere(3));
    await user.click(screen.getByRole('option', { name: /^Judge arrays r2/ }));
    expect(onThreadChange).toHaveBeenCalledWith('j2', undefined);

    rerender(viewer({ activeThreadId: 'j2', onThreadChange }));
    expect(crumbs().getAllByRole('button').map((b) => b.getAttribute('aria-label'))).toEqual([
      'Judge arrays r2',
      'Copy link to this subagent',
    ]);
    expect(screen.queryByRole('button', { name: /^Launched here/ })).not.toBeInTheDocument();
    const ancestor = screen.getByRole('tab', { name: /^Explore the codebase/ });
    expect(ancestor).toHaveAttribute('data-in-path', 'true');
    expect(ancestor).toHaveAttribute('aria-selected', 'true');
  });

  it('lists the depth-2 siblings with the open one checked, and switches to a picked sibling', async () => {
    const user = userEvent.setup();
    const { onThreadChange } = walk(['a1', 'j1']);
    await user.click(crumbs().getByRole('button', { name: 'Judge arrays r1' }));
    const options = screen.getAllByRole('option');
    expect(options.map((o) => o.textContent)).toEqual([
      expect.stringContaining('Judge arrays r1'),
      expect.stringContaining('Judge arrays r2'),
      expect.stringContaining('Simplify Go changes'),
    ]);
    expect(options[0]).toHaveAttribute('aria-selected', 'true');
    expect(options[0]).toHaveTextContent('launched 1');
    await user.click(options[2]!);
    expect(onThreadChange).toHaveBeenCalledWith('j3', undefined);
  });

  it('picking an ancestor lands on the launch row of the agent you came from', async () => {
    const user = userEvent.setup();
    const { onThreadChange } = walk(['a1', 'j1', 'g1']);
    await user.click(crumbs().getByRole('button', { name: 'Judge arrays r1' }));
    await user.click(screen.getByRole('option', { name: /^Judge arrays r1/ }));
    expect(onThreadChange).toHaveBeenLastCalledWith('j1', 'j1-3');
    await user.click(screen.getByRole('tab', { name: /^Explore the codebase/ }));
    expect(onThreadChange).toHaveBeenLastCalledWith('a1', 's3');
  });

  it('shows two dropdown segments at depth 3', () => {
    walk(['a1', 'j1', 'g1']);
    const dropdowns = crumbs()
      .getAllByRole('button')
      .filter((b) => b.getAttribute('aria-haspopup') === 'listbox')
      .map((b) => b.getAttribute('aria-label'));
    expect(dropdowns).toEqual(['Judge arrays r1', 'Re-run the flaky judge']);
    expect(stripLabels()).toEqual(['Main', 'Explore the codebase', 'Write tests']);
  });

  it('remembers a thread’s children after leaving it, so the sibling dropdown still lists them', async () => {
    const user = userEvent.setup();
    const { rerender, onThreadChange } = walk(['a1', 'j1']);
    // a1 is no longer loaded (only the open thread is fetched): drop its seed to prove it.
    rerender(viewer({ activeThreadId: 'j1', onThreadChange, initialThreadMessages: { j1: j1Messages } }));
    await user.click(crumbs().getByRole('button', { name: 'Judge arrays r1' }));
    expect(screen.getAllByRole('option')).toHaveLength(3);
  });

  it("refreshes a nested agent's status from its parent's latest transcript, and keeps it after leaving", async () => {
    const user = userEvent.setup();
    const { rerender, onThreadChange } = walk(['a1']);
    await user.click(stripLaunchedHere(3));
    expect(screen.getByRole('option', { name: /^Judge arrays r1/ })).toHaveTextContent('running');
    await user.keyboard('{Escape}');

    const a1Finished = [
      ...a1Messages,
      taskNotificationMessage('s8', { taskId: 'j1', toolUseId: 'tj1', status: 'completed' }),
    ];
    const finished = { ...threadMessages, a1: a1Finished };
    rerender(viewer({ activeThreadId: 'a1', onThreadChange, initialThreadMessages: finished }));
    await user.click(stripLaunchedHere(3));
    expect(screen.getByRole('option', { name: /^Judge arrays r1/ })).toHaveTextContent('completed');
    await user.keyboard('{Escape}');

    act(() => {
      openThread('j1');
    });
    rerender(viewer({ activeThreadId: 'j1', onThreadChange, initialThreadMessages: finished }));
    await user.click(crumbs().getByRole('button', { name: 'Judge arrays r1' }));
    expect(screen.getByRole('option', { name: /^Judge arrays r1/ })).toHaveTextContent('completed');
  });

  // jgk8 D8: a cold deep link to a nested agent keeps the id chip, with no ancestor crumbs.
  it('a cold deep link to a nested agent shows an id chip and a row with only Launched here and copy link', () => {
    render(viewer({ activeThreadId: 'j1', onThreadChange: () => {} }));
    expect(screen.getByRole('tab', { name: 'j1, status unknown' })).toHaveAttribute('aria-selected', 'true');
    expect(crumbs().getAllByRole('button').map((b) => b.getAttribute('aria-label') ?? b.textContent)).toEqual([
      'Launched here (1)',
      'Copy link to this subagent',
    ]);
    expect(screen.getAllByRole('button', { name: /^Launched here/ })).toHaveLength(1);
  });

  it("a child of a cold deep-linked agent gets its own labeled chip, since its ancestry is unknown", () => {
    const { rerender, onThreadChange } = walk(['j1']);
    act(() => {
      openThread('g1');
    });
    expect(onThreadChange).toHaveBeenCalledWith('g1', undefined);
    rerender(viewer({ activeThreadId: 'g1', onThreadChange }));
    expect(stripLabels()).toEqual(['Main', 'Explore the codebase', 'Write tests', 'Re-run the flaky judge']);
    expect(screen.getByRole('tab', { name: 'Re-run the flaky judge, running' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.queryByRole('navigation', { name: 'Subagent path' })).not.toBeInTheDocument();
  });

  it('switching sessions forgets the remembered children', () => {
    const { rerender, onThreadChange } = walk(['a1', 'j1']);
    const other = { ...claudeSession, id: 'other-session' };
    rerender(viewer({ session: other, activeThreadId: 'j1', onThreadChange }));
    expect(screen.getByRole('tab', { name: 'j1, status unknown' })).toBeInTheDocument();
    expect(screen.queryByRole('tab', { name: /^Explore the codebase/ })).not.toHaveAttribute('data-in-path');
    expect(screen.queryByRole('button', { name: 'Judge arrays r1' })).not.toBeInTheDocument();
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
