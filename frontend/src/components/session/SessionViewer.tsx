import { useState, useMemo, useCallback, useRef } from 'react';
import type { SessionDetail, TranscriptLine } from '@/types';
import type { RawCodexLine } from '@/schemas/codexTranscript';
import { getAdapter } from '@/providers/registry';
import { useTranscriptData } from '@/providers/useTranscriptData';
import { useSessionTILs } from '@/providers/useSessionTILs';
import SessionHeader from './SessionHeader';
import SessionSummaryPanel from './SessionSummaryPanel';
import styles from './SessionViewer.module.css';

export type ViewTab = 'summary' | 'transcript';

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
  /** Deep-link target. For Claude this is a message UUID; for Codex it is a
   *  lineId. The adapter for the active provider resolves it. */
  targetMessageUuid?: string;
  /** For Storybook: pass messages directly instead of fetching from API */
  initialMessages?: TranscriptLine[];
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
  targetMessageUuid,
  initialMessages,
  initialAnalytics,
  initialGithubLinks,
  initialCodexRawLines,
}: SessionViewerProps) {
  // Support both controlled and uncontrolled modes
  const [uncontrolledTab, setUncontrolledTab] = useState<ViewTab>('summary');
  const activeTab = controlledTab ?? uncontrolledTab;
  const setActiveTab = onTabChange ?? setUncontrolledTab;

  const adapter = getAdapter(session.provider);

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

  const { items, raw, loading, error } = useTranscriptData(
    adapter,
    session.id,
    transcriptFileName,
    seed,
  );

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

  adapter.useDeepLinkFilterReset(items, targetMessageUuid, filters);

  const tilsByMessageUuid = useSessionTILs(session.id, adapter.supportsTILs);

  const sessionMeta = useMemo(() => {
    const { durationMs, sessionDate } = adapter.computeMeta(items, raw, {
      firstSeen: session.first_seen,
      lastSyncAt: session.last_sync_at,
    });
    return {
      model: adapter.extractModel(raw, items),
      durationMs,
      sessionDate,
    };
  }, [adapter, items, raw, session.first_seen, session.last_sync_at]);

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

        {/* Tab Content */}
        <div className={styles.tabContent}>
          {activeTab === 'summary' ? (
            // CF-364: Summary tab is provider-agnostic. Codex sessions get
            // analytics from ComputeFromCodexRollout (CF-350) via the same
            // SessionSummaryPanel. Smart-recap deep-links and TIL badges
            // skip on Codex via adapter.supportsTILs, since both anchor to
            // message UUIDs that Codex messages don't carry.
            <SessionSummaryPanel
              sessionId={session.id}
              isOwner={isOwner}
              initialAnalytics={initialAnalytics}
              initialGithubLinks={initialGithubLinks}
              onSuggestedTitleChange={handleSuggestedTitleChange}
            />
          ) : (
            <div className={styles.timelineContainer}>
              <AdapterTranscriptPane
                sessionId={session.id}
                items={items}
                filteredItems={filteredItems}
                visibleIndices={visibleIndices}
                loading={loading}
                error={error}
                targetId={targetMessageUuid}
                isCostMode={isCostMode}
                tilsByMessageUuid={tilsByMessageUuid}
              />
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

export default SessionViewer;
