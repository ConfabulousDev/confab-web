import { useState, useMemo, useCallback, useRef } from 'react';
import type { SessionDetail, TranscriptLine } from '@/types';
import type { RawCodexLine } from '@/schemas/codexTranscript';
import { getAdapter } from '@/providers/registry';
import type { TranscriptThreadRef } from '@/providers/types';
import { useTranscriptData } from '@/providers/useTranscriptData';
import SessionHeader from './SessionHeader';
import SessionSummaryPanel from './SessionSummaryPanel';
import TranscriptThreadTabs from './TranscriptThreadTabs';
import styles from './SessionViewer.module.css';

export type ViewTab = 'summary' | 'transcript';

const NO_THREADS: TranscriptThreadRef[] = [];
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
   * thread (back link). Providing it makes the thread selection controlled.
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

  // Nested threads opened from inside a subagent tab stay in the strip for the
  // rest of the visit. Scoped to the session so a session switch starts clean.
  const [openedThreads, setOpenedThreads] = useState<{ sessionId: string; refs: TranscriptThreadRef[] }>(
    { sessionId: session.id, refs: [] },
  );
  const nestedThreads = openedThreads.sessionId === session.id ? openedThreads.refs : NO_THREADS;

  const knownThreads = useMemo(() => {
    if (nestedThreads.length === 0) return mainThreads;
    const mainIds = new Set(mainThreads.map((t) => t.id));
    return [...mainThreads, ...nestedThreads.filter((t) => !mainIds.has(t.id))];
  }, [mainThreads, nestedThreads]);

  // A thread id with no discovered ref (deep link to a nested agent, or Main
  // not yet showing its launch) still opens, labeled by id (et0r D10).
  const activeThread = useMemo<TranscriptThreadRef | null>(() => {
    if (!activeThreadId || !threadsCapability) return null;
    return (
      knownThreads.find((t) => t.id === activeThreadId) ?? {
        id: activeThreadId,
        fileName: threadsCapability.fileNameFor(activeThreadId),
        label: activeThreadId,
        parentThreadId: undefined,
      }
    );
  }, [activeThreadId, threadsCapability, knownThreads]);

  const stripThreads = useMemo(
    () => (activeThread && !knownThreads.includes(activeThread) ? [...knownThreads, activeThread] : knownThreads),
    [activeThread, knownThreads],
  );

  const threadSeed = useMemo(() => {
    if (seed === undefined) return undefined;
    return { raw: (activeThread && initialThreadMessages?.[activeThread.id]) || NO_LINES };
  }, [seed, activeThread, initialThreadMessages]);

  // Only the active thread is fetched and polled; no fetch while Main is active.
  const thread = useTranscriptData(adapter, session.id, activeThread?.fileName, threadSeed, THREAD_DATA_OPTIONS);

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

  const handleOpenThread = useCallback(
    (threadId: string) => {
      // A nested agent opened from inside a subagent tab joins the strip,
      // labeled with its parent for context.
      if (threadsCapability && activeThread && !knownThreads.some((t) => t.id === threadId)) {
        const child = threadsCapability.discover(thread.items, activeThread.id).find((t) => t.id === threadId);
        if (child) {
          const ref = { ...child, label: `${activeThread.label} › ${child.label}` };
          setOpenedThreads((prev) => ({
            sessionId: session.id,
            refs: [...(prev.sessionId === session.id ? prev.refs : []), ref],
          }));
        }
      }
      changeThread(threadId);
    },
    [threadsCapability, activeThread, knownThreads, thread.items, session.id, changeThread],
  );

  const backLink = useMemo(() => {
    if (!activeThread || activeThread.parentThreadId === undefined || !activeThread.launchTargetId) {
      return undefined;
    }
    const { parentThreadId, launchTargetId } = activeThread;
    const label =
      parentThreadId === null
        ? 'Main'
        : (knownThreads.find((t) => t.id === parentThreadId)?.label ?? parentThreadId);
    return { label, onClick: () => changeThread(parentThreadId, launchTargetId) };
  }, [activeThread, knownThreads, changeThread]);

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
            onSelect={changeThread}
            backLink={backLink}
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
                onOpenThread={threadsCapability ? handleOpenThread : undefined}
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
