import { useState, useMemo, useCallback, useRef } from 'react';
import type { SessionDetail, TranscriptLine } from '@/types';
import type { RawCodexLine } from '@/schemas/codexTranscript';
import { getAdapter } from '@/providers/registry';
import type { TranscriptThreadRef } from '@/providers/types';
import { useTranscriptData } from '@/providers/useTranscriptData';
import SessionHeader from './SessionHeader';
import SessionSummaryPanel from './SessionSummaryPanel';
import TranscriptThreadTabs from './TranscriptThreadTabs';
import ThreadBreadcrumb from './ThreadBreadcrumb';
import { threadPath } from './threadDetails';
import styles from './SessionViewer.module.css';

export type ViewTab = 'summary' | 'transcript';

const NO_THREADS: TranscriptThreadRef[] = [];
const NO_CHILDREN: ReadonlyMap<string, TranscriptThreadRef[]> = new Map();
const NO_LINES: unknown[] = [];
// et0r D9: a revisited thread renders from the service cache, then polls.
const THREAD_DATA_OPTIONS = { preferCache: true };

interface SessionViewerProps {
  session: SessionDetail;
  onShare?: () => void;
  onDelete?: () => void;
  onSessionUpdate?: (session: SessionDetail) => void;
  isOwner?: boolean;
  isShared?: boolean;
  /** Controlled active tab - if provided, component is controlled */
  activeTab?: ViewTab;
  /** Callback when tab changes - required if activeTab is provided */
  onTabChange?: (tab: ViewTab) => void;
  /** et0r: controlled active transcript thread (subagent id); null = Main. */
  activeThreadId?: string | null;
  /**
   * et0r: thread change callback. `targetId` is the row to land on in the new
   * thread (Go to parent). Providing it makes the thread selection controlled.
   */
  onThreadChange?: (threadId: string | null, targetId?: string) => void;
  /** Deep-link target. Forwarded opaquely to the active provider's adapter,
   *  which interprets it per its own identity scheme (Claude: message UUID;
   *  Codex: lineId per CF-360). */
  targetId?: string;
  /** For Storybook: pass messages directly instead of fetching from API */
  initialMessages?: TranscriptLine[];
  /** For Storybook: per-thread messages keyed by thread id (et0r) */
  initialThreadMessages?: Record<string, TranscriptLine[]>;
  /** For Storybook: pass analytics directly instead of fetching from API */
  initialAnalytics?: import('@/services/api').SessionAnalytics;
  /** For Storybook: pass GitHub links directly instead of fetching from API */
  initialGithubLinks?: import('@/services/api').GitHubLink[];
  /** For Storybook: pass raw Codex lines directly instead of fetching from API */
  initialCodexRawLines?: RawCodexLine[];
}

function SessionViewer({
  session,
  onShare,
  onDelete,
  onSessionUpdate,
  isOwner = true,
  isShared = false,
  activeTab: controlledTab,
  onTabChange,
  activeThreadId: controlledThreadId,
  onThreadChange,
  targetId,
  initialMessages,
  initialThreadMessages,
  initialAnalytics,
  initialGithubLinks,
  initialCodexRawLines,
}: SessionViewerProps) {
  // Support both controlled and uncontrolled modes
  const [uncontrolledTab, setUncontrolledTab] = useState<ViewTab>('summary');
  const activeTab = controlledTab ?? uncontrolledTab;
  const setActiveTab = onTabChange ?? setUncontrolledTab;

  // et0r: thread selection, controlled by the page (URL `agent` / `msg`) or
  // local for Storybook.
  const [uncontrolledNav, setUncontrolledNav] = useState<{ threadId: string | null; targetId?: string }>(
    { threadId: null },
  );
  const isThreadControlled = onThreadChange !== undefined;
  const activeThreadId = isThreadControlled ? (controlledThreadId ?? null) : uncontrolledNav.threadId;
  const effectiveTargetId = isThreadControlled ? targetId : (uncontrolledNav.targetId ?? targetId);
  const changeThread = useCallback(
    (threadId: string | null, target?: string) => {
      if (onThreadChange) onThreadChange(threadId, target);
      else setUncontrolledNav({ threadId, targetId: target });
    },
    [onThreadChange],
  );

  const adapter = getAdapter(session.provider);
  const threadsCapability = adapter.threads;

  // Cost mode toggle (only meaningful on the transcript tab)
  const [isCostMode, setIsCostMode] = useState(false);
  const toggleCostMode = useCallback(() => setIsCostMode((prev) => !prev), []);

  const transcriptFileName = useMemo(
    () => session.files.find((f) => f.file_type === 'transcript')?.file_name,
    [session.files],
  );

  // Storybook bypass: the active provider's initial-* prop becomes the seed.
  const seed = useMemo(() => {
    if (initialMessages !== undefined) return { raw: initialMessages };
    if (initialCodexRawLines !== undefined) return { raw: initialCodexRawLines };
    return undefined;
  }, [initialMessages, initialCodexRawLines]);

  // Main stays loaded and polling on every tab: the thread list and the header
  // model/duration derive from it (et0r D9).
  const main = useTranscriptData(adapter, session.id, transcriptFileName, seed);

  const mainThreads = useMemo(
    () => threadsCapability?.discover(main.items, null) ?? NO_THREADS,
    [threadsCapability, main.items],
  );

  // jgk8 D1: each subagent's children, recorded when it loads and remembered for
  // the visit, so every level stays reachable after switching away. Scoped to
  // the session so a session switch starts clean.
  const [childMemory, setChildMemory] = useState<{
    sessionId: string;
    byParent: ReadonlyMap<string, TranscriptThreadRef[]>;
  }>({ sessionId: session.id, byParent: NO_CHILDREN });
  const childrenByParent = childMemory.sessionId === session.id ? childMemory.byParent : NO_CHILDREN;

  // Every thread discovered this visit; Main's discovery wins, then first seen.
  const knownThreads = useMemo(() => {
    const known = new Map(mainThreads.map((t) => [t.id, t]));
    for (const children of childrenByParent.values()) {
      for (const child of children) if (!known.has(child.id)) known.set(child.id, child);
    }
    return known;
  }, [mainThreads, childrenByParent]);

  // A thread id with no discovered ref (cold deep link to a nested agent, or
  // Main not yet showing its launch) still opens, labeled by id (et0r D10).
  const activeThread = useMemo<TranscriptThreadRef | null>(() => {
    if (!activeThreadId || !threadsCapability) return null;
    return (
      knownThreads.get(activeThreadId) ?? {
        id: activeThreadId,
        fileName: threadsCapability.fileNameFor(activeThreadId),
        label: activeThreadId,
        parentThreadId: undefined,
      }
    );
  }, [activeThreadId, threadsCapability, knownThreads]);

  // Depth-1 → open thread; undefined when the ancestry isn't known (jgk8 D8).
  const activePath = useMemo(
    () => (activeThread ? threadPath(activeThread, knownThreads) : undefined),
    [activeThread, knownThreads],
  );

  // jgk8 D4: the strip is Main's direct subagents, plus a chip for an open
  // thread whose ancestry is unknown (D8). Nested agents live in the path row.
  const stripThreads = useMemo(
    () => (activeThread && !activePath ? [...mainThreads, activeThread] : mainThreads),
    [activeThread, activePath, mainThreads],
  );

  // jgk8 D5: the depth-1 ancestor chip shows "in path" and lands on the launch row.
  const inPath = useMemo(() => {
    const [depth1, next] = activePath ?? [];
    return depth1 && next ? { threadId: depth1.id, targetId: next.launchTargetId } : undefined;
  }, [activePath]);

  // jgk8 D7: the All-subagents total counts every uploaded thread file. That
  // is a page-load snapshot, so on a live session it is floored at every
  // distinct agent known this visit (Main's, remembered children, an unknown
  // open one): the total never drops below a list it summarizes.
  const threadFileCount = useMemo(
    () => threadsCapability?.countThreadFiles?.(session.files),
    [threadsCapability, session.files],
  );
  const knownThreadCount = knownThreads.size + (activeThread && !knownThreads.has(activeThread.id) ? 1 : 0);
  const allThreadsCount = Math.max(threadFileCount ?? 0, knownThreadCount);
  const nestedThreadCount = Math.max(0, allThreadsCount - mainThreads.length);

  const childrenOf = useCallback(
    (threadId: string) => childrenByParent.get(threadId) ?? NO_THREADS,
    [childrenByParent],
  );
  const childCountOf = useCallback((threadId: string) => childrenOf(threadId).length, [childrenOf]);

  const threadSeed = useMemo(() => {
    if (seed === undefined) return undefined;
    return { raw: (activeThread && initialThreadMessages?.[activeThread.id]) || NO_LINES };
  }, [seed, activeThread, initialThreadMessages]);

  // Only the active thread is fetched and polled; no fetch while Main is active.
  const thread = useTranscriptData(adapter, session.id, activeThread?.fileName, threadSeed, THREAD_DATA_OPTIONS);

  // Threads launched from the open subagent thread, rediscovered on each load and poll.
  const openThreadId = activeThread?.id;
  const childThreads = useMemo(
    () =>
      openThreadId !== undefined && threadsCapability
        ? threadsCapability.discover(thread.items, openThreadId)
        : NO_THREADS,
    [openThreadId, threadsCapability, thread.items],
  );

  // Record the open thread's latest discovery (live status). Transcripts only
  // grow, so an empty discovery (still loading, or a failed fetch) never
  // replaces remembered children.
  const [syncedChildThreads, setSyncedChildThreads] = useState(NO_THREADS);
  if (childThreads !== syncedChildThreads) {
    setSyncedChildThreads(childThreads);
    if (openThreadId !== undefined && childThreads.length > 0) {
      setChildMemory((prev) => {
        const byParent = new Map(prev.sessionId === session.id ? prev.byParent : NO_CHILDREN);
        byParent.set(openThreadId, childThreads);
        return { sessionId: session.id, byParent };
      });
    }
  }

  // Everything transcript-facing follows the open tab: counts, filters,
  // deep-link reset, and the pane (et0r D1). Session meta stays on Main.
  const active = activeThread ? thread : main;
  const items = active.items;

  const filters = adapter.useFilters();
  const counts = useMemo(() => adapter.countCategories(items), [adapter, items]);

  const { filteredItems, visibleIndices } = useMemo(() => {
    const filtered: unknown[] = [];
    const visible = new Set<number>();
    items.forEach((item, idx) => {
      if (adapter.itemMatchesFilter(item, filters.state)) {
        filtered.push(item);
        visible.add(idx);
      }
    });
    return { filteredItems: filtered, visibleIndices: visible };
  }, [adapter, items, filters.state]);

  adapter.useDeepLinkFilterReset(items, effectiveTargetId, filters);

  const sessionMeta = useMemo(() => {
    const { durationMs, sessionDate } = adapter.computeMeta(main.items, main.raw, {
      firstSeen: session.first_seen,
      lastSyncAt: session.last_sync_at,
    });
    return {
      model: adapter.extractModel(main.raw, main.items),
      durationMs,
      sessionDate,
    };
  }, [adapter, main.items, main.raw, session.first_seen, session.last_sync_at]);

  const lastAppliedSuggestedTitleRef = useRef<string | null>(null);
  const handleSuggestedTitleChange = useCallback(
    (title: string) => {
      if (title === lastAppliedSuggestedTitleRef.current) return;
      if (onSessionUpdate && session) {
        onSessionUpdate({ ...session, suggested_session_title: title });
        lastAppliedSuggestedTitleRef.current = title;
      }
    },
    [session, onSessionUpdate],
  );

  const showTranscriptControls = activeTab === 'transcript';
  const { FilterDropdown: AdapterFilterDropdown, TranscriptPane: AdapterTranscriptPane } = adapter;

  return (
    <div className={styles.sessionViewer}>
      <div className={styles.mainContent}>
        <SessionHeader
          sessionId={session.id}
          title={
            session.custom_title ??
            session.suggested_session_title ??
            session.summary ??
            session.first_user_message ??
            undefined
          }
          hasCustomTitle={!!session.custom_title}
          autoTitle={
            session.suggested_session_title ?? session.summary ?? session.first_user_message ?? undefined
          }
          externalId={session.external_id}
          provider={session.provider}
          ownerEmail={session.owner_email}
          model={sessionMeta.model}
          durationMs={sessionMeta.durationMs}
          sessionDate={sessionMeta.sessionDate}
          gitInfo={session.git_info}
          onShare={onShare}
          onDelete={onDelete}
          onSessionUpdate={onSessionUpdate}
          isOwner={isOwner}
          isShared={isShared}
          sharedByEmail={session.shared_by_email}
          isCostMode={showTranscriptControls ? isCostMode : undefined}
          onToggleCostMode={showTranscriptControls ? toggleCostMode : undefined}
          filterSlot={
            showTranscriptControls ? (
              <AdapterFilterDropdown counts={counts} filters={filters} />
            ) : null
          }
        />

        {/* Tabs */}
        <div className={styles.tabs}>
          <button
            className={`${styles.tab} ${activeTab === 'summary' ? styles.tabActive : ''}`}
            onClick={() => setActiveTab('summary')}
          >
            Summary
          </button>
          <button
            className={`${styles.tab} ${activeTab === 'transcript' ? styles.tabActive : ''}`}
            onClick={() => setActiveTab('transcript')}
          >
            Transcript
          </button>
        </div>

        {showTranscriptControls && stripThreads.length > 0 && (
          <TranscriptThreadTabs
            threads={stripThreads}
            activeThreadId={activeThread?.id ?? null}
            sessionId={session.id}
            onSelect={changeThread}
            allThreadsCount={allThreadsCount}
            nestedThreadCount={nestedThreadCount}
            childCountOf={childCountOf}
            inPath={inPath}
            launchedHere={activePath?.length === 1 ? childrenOf(activePath[0]!.id) : undefined}
          />
        )}

        {showTranscriptControls && activeThread && (
          <ThreadBreadcrumb
            active={activeThread}
            path={activePath}
            childrenOf={childrenOf}
            sessionId={session.id}
            onSelect={changeThread}
          />
        )}

        {/* Tab Content */}
        <div className={styles.tabContent}>
          {activeTab === 'summary' ? (
            // CF-364: Summary tab is provider-agnostic. Codex sessions get
            // analytics from ComputeFromCodexRollout (CF-350) via the same
            // SessionSummaryPanel.
            <SessionSummaryPanel
              sessionId={session.id}
              isOwner={isOwner}
              provider={session.provider}
              initialAnalytics={initialAnalytics}
              initialGithubLinks={initialGithubLinks}
              onSuggestedTitleChange={handleSuggestedTitleChange}
            />
          ) : (
            <div className={styles.timelineContainer}>
              {/* Keyed per thread so search, scroll, and selection reset on switch (et0r D11). */}
              <AdapterTranscriptPane
                key={activeThread?.id ?? '\u0000main'}
                sessionId={session.id}
                items={items}
                filteredItems={filteredItems}
                visibleIndices={visibleIndices}
                loading={active.loading}
                error={active.error}
                targetId={effectiveTargetId}
                isCostMode={isCostMode}
                firstSeen={session.first_seen}
                lastSyncAt={session.last_sync_at}
                activeThreadId={activeThread?.id ?? null}
                onOpenThread={threadsCapability ? changeThread : undefined}
                notSynced={activeThread !== null && thread.notFound}
              />
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

export default SessionViewer;
