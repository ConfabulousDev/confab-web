import { useState, useEffect, useMemo, useCallback, useRef } from 'react';
import type { SessionDetail, TranscriptLine } from '@/types';
import { isAssistantMessage } from '@/types';
import { fetchParsedTranscript, fetchNewTranscriptMessages } from '@/services/transcriptService';
import {
  fetchParsedCodexTranscript,
  fetchNewCodexLines,
  extractCodexModel,
  normalizeCodexLines,
} from '@/services/codexTranscriptService';
import type { RawCodexLine } from '@/schemas/codexTranscript';
import type { CodexRenderItem } from '@/types/codexRenderItem';
import { tilsAPI, type TIL } from '@/services/api';
import { useVisibility } from '@/hooks/useVisibility';
import { useTranscriptFilters } from '@/hooks/useTranscriptFilters';
import { useCodexTranscriptFilters } from '@/hooks/useCodexTranscriptFilters';
import { computeSessionMeta } from '@/utils/sessionMeta';
import {
  countHierarchicalCategories,
  messageMatchesFilter,
  DEFAULT_FILTER_STATE,
} from './messageCategories';
import {
  countCodexCategories,
  codexItemMatchesFilter,
  DEFAULT_CODEX_FILTER_STATE,
} from './codexCategories';
import SessionHeader from './SessionHeader';
import SessionSummaryPanel from './SessionSummaryPanel';
import ClaudeTranscriptPane from './ClaudeTranscriptPane';
import CodexTranscriptPane from './CodexTranscriptPane';
import styles from './SessionViewer.module.css';

// Provider canonical names per CF-347. Legacy 'Claude Code' rows still exist
// in the database but the API normalizes them to 'claude-code' on read.
function isCodexProvider(provider: string): boolean {
  return provider === 'codex';
}

export type ViewTab = 'summary' | 'transcript';

// Polling interval for new transcript messages (15 seconds)
const TRANSCRIPT_POLL_INTERVAL_MS = 15000;

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
  /** UUID of a message to scroll to and highlight (deep-link target) */
  targetMessageUuid?: string;
  /** For Storybook: pass messages directly instead of fetching from API */
  initialMessages?: TranscriptLine[];
  /** For Storybook: pass analytics directly instead of fetching from API */
  initialAnalytics?: import('@/services/api').SessionAnalytics;
  /** For Storybook: pass GitHub links directly instead of fetching from API */
  initialGithubLinks?: import('@/services/api').GitHubLink[];
  /** For Storybook: pass raw Codex lines directly instead of fetching from API */
  initialCodexRawLines?: import('@/schemas/codexTranscript').RawCodexLine[];
}

function SessionViewer({ session, onShare, onDelete, onSessionUpdate, isOwner = true, isShared = false, activeTab: controlledTab, onTabChange, targetMessageUuid, initialMessages, initialAnalytics, initialGithubLinks, initialCodexRawLines }: SessionViewerProps) {
  // Support both controlled and uncontrolled modes
  const [uncontrolledTab, setUncontrolledTab] = useState<ViewTab>('summary');
  const activeTab = controlledTab ?? uncontrolledTab;
  const setActiveTab = onTabChange ?? setUncontrolledTab;
  const isCodex = isCodexProvider(session.provider);
  // CF-386: SessionViewer now owns transcript state for both providers.
  // Initial loading is true whenever we expect to fetch (no Storybook bypass).
  const willFetch = isCodex
    ? initialCodexRawLines === undefined
    : initialMessages === undefined;
  const [loading, setLoading] = useState(willFetch);
  const [error, setError] = useState<string | null>(null);
  const [messages, setMessages] = useState<TranscriptLine[]>(initialMessages ?? []);
  const [codexRawLines, setCodexRawLines] = useState<RawCodexLine[]>(
    initialCodexRawLines ?? []
  );

  // Track the current line count for incremental fetching
  const lineCountRef = useRef(0);

  // Track visibility for smart polling
  const isVisible = useVisibility();

  // Cost mode toggle
  const [isCostMode, setIsCostMode] = useState(false);

  // TILs for this session (fetched once on mount).
  // Skip for Codex — TILs are message-anchored via UUIDs, which Codex
  // sessions don't have. The request would return an empty list anyway.
  const [sessionTILs, setSessionTILs] = useState<Map<string, TIL[]>>(new Map());
  useEffect(() => {
    setSessionTILs(new Map()); // Clear stale badges when navigating between sessions
    if (isCodex) return;
    tilsAPI.listForSession(session.id).then((response) => {
      const map = new Map<string, TIL[]>();
      for (const til of response.tils) {
        if (!til.message_uuid) continue;
        const existing = map.get(til.message_uuid) ?? [];
        existing.push(til);
        map.set(til.message_uuid, existing);
      }
      setSessionTILs(map);
    }).catch(() => {
      // Non-critical — TIL markers simply won't appear
    });
  }, [session.id, isCodex]);

  // Claude filter state — synced to URL via ?hide= param.
  const {
    filterState, setFilterState,
    toggleCategory, toggleUserSubcategory, toggleAssistantSubcategory, toggleAttachmentSubcategory,
  } = useTranscriptFilters();

  // Codex filter state (CF-361) — same ?hide= URL slot, provider-specific
  // token grammar. Both hooks render unconditionally; only the active
  // provider's outputs reach SessionHeader / CodexTranscriptPane downstream.
  const {
    filterState: codexFilterState,
    setFilterState: setCodexFilterState,
    toggleCategory: toggleCodexCategory,
    toggleAssistantSubcategory: toggleCodexAssistantSubcategory,
    toggleToolCallSubcategory: toggleCodexToolCallSubcategory,
  } = useCodexTranscriptFilters();

  // Compute hierarchical category counts
  const categoryCounts = useMemo(() => countHierarchicalCategories(messages), [messages]);

  // Filter messages based on filter state
  const filteredMessages = useMemo(() => {
    return messages.filter((message) => messageMatchesFilter(message, filterState));
  }, [messages, filterState]);

  // CF-361: lift Codex render items here so SessionHeader can show counts on
  // the Summary tab too, and we can compute the visible-index set in one
  // place for the timeline bar.
  const codexItems = useMemo(() => normalizeCodexLines(codexRawLines), [codexRawLines]);
  const codexCategoryCounts = useMemo(() => countCodexCategories(codexItems), [codexItems]);
  const { filteredCodexItems, visibleCodexIndices } = useMemo(() => {
    const filtered: CodexRenderItem[] = [];
    const visible = new Set<number>();
    codexItems.forEach((item, idx) => {
      if (codexItemMatchesFilter(item, codexFilterState)) {
        filtered.push(item);
        visible.add(idx);
      }
    });
    return { filteredCodexItems: filtered, visibleCodexIndices: visible };
  }, [codexItems, codexFilterState]);

  // When a deep-link target exists but is hidden by the active filter, reset
  // filters so the target becomes visible. Runs once per target change since
  // the reset itself makes the target pass on the next check.
  useEffect(() => {
    if (isCodex || !targetMessageUuid || messages.length === 0) return;

    const targetMessage = messages.find(
      (m) => 'uuid' in m && m.uuid === targetMessageUuid
    );
    if (!targetMessage) return;
    if (messageMatchesFilter(targetMessage, filterState)) return;

    setFilterState({
      ...DEFAULT_FILTER_STATE,
      system: targetMessage.type === 'system',
    }, { replace: true });
  }, [isCodex, targetMessageUuid, messages, filterState, setFilterState]);

  // CF-361: Codex parallel of the deep-link filter-reset fallback. If the
  // target lineId resolves to a row hidden by the active Codex filter, reset
  // to defaults and force the target's category visible (only matters when
  // the target itself is reasoning_hidden, since that's the only category
  // hidden by default).
  useEffect(() => {
    if (!isCodex || !targetMessageUuid || codexItems.length === 0) return;
    const target = codexItems.find((it) => it.lineId === targetMessageUuid);
    if (!target) return;
    if (codexItemMatchesFilter(target, codexFilterState)) return;

    setCodexFilterState({
      ...DEFAULT_CODEX_FILTER_STATE,
      reasoning_hidden: target.kind === 'reasoning_hidden',
    }, { replace: true });
  }, [isCodex, targetMessageUuid, codexItems, codexFilterState, setCodexFilterState]);

  // Get transcript file info
  const transcriptFile = useMemo(() => {
    return session.files.find((f) => f.file_type === 'transcript');
  }, [session.files]);

  const transcriptFileName = transcriptFile?.file_name;

  // CF-386: provider-aware initial load. SessionViewer owns both Claude and
  // Codex transcript state so SessionHeader can derive the model name without
  // waiting for the Transcript tab to mount its pane.
  useEffect(() => {
    if (!willFetch) return;
    loadTranscript();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [session.id, willFetch, isCodex]);

  async function loadTranscript() {
    setLoading(true);
    setError(null);
    lineCountRef.current = 0;

    try {
      if (!transcriptFileName) {
        throw new Error('No transcript file found');
      }

      // Skip cache on initial load to ensure fresh data when navigating to a session.
      if (isCodex) {
        const parsed = await fetchParsedCodexTranscript(
          session.id,
          transcriptFileName,
          true
        );
        setCodexRawLines(parsed.rawLines);
        lineCountRef.current = parsed.totalLines;
      } else {
        const parsed = await fetchParsedTranscript(session.id, transcriptFileName, true);
        setMessages(parsed.messages);
        // Use totalLines (not messages.length) to track line_offset accurately.
        // This accounts for parse errors and ensures we don't re-fetch lines.
        lineCountRef.current = parsed.totalLines;
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load transcript');
      console.error('Failed to load transcript:', e);
    } finally {
      setLoading(false);
    }
  }

  // CF-386: provider-aware polling. One useEffect for both providers; branches
  // on `isCodex` to call the right fetcher and append to the right state.
  useEffect(() => {
    if (!willFetch || !isVisible || loading || !transcriptFileName) {
      return;
    }

    const poll = async () => {
      try {
        if (isCodex) {
          const { newRawLines, newTotalLineCount } = await fetchNewCodexLines(
            session.id,
            transcriptFileName,
            lineCountRef.current
          );
          if (newRawLines.length > 0) {
            setCodexRawLines((prev) => [...prev, ...newRawLines]);
            lineCountRef.current = newTotalLineCount;
          }
        } else {
          const { newMessages, newTotalLineCount } = await fetchNewTranscriptMessages(
            session.id,
            transcriptFileName,
            lineCountRef.current
          );
          if (newMessages.length > 0) {
            setMessages((prev) => [...prev, ...newMessages]);
            lineCountRef.current = newTotalLineCount;
          }
        }
      } catch (e) {
        // Don't show error for polling failures - just log.
        console.warn('Failed to poll for new transcript lines:', e);
      }
    };

    const intervalId = setInterval(poll, TRANSCRIPT_POLL_INTERVAL_MS);
    return () => {
      clearInterval(intervalId);
    };
  }, [willFetch, isVisible, loading, isCodex, session.id, transcriptFileName]);

  const toggleCostMode = useCallback(() => setIsCostMode((prev) => !prev), []);

  // Track the last successfully applied suggested title to avoid duplicate updates
  const lastAppliedSuggestedTitleRef = useRef<string | null>(null);

  // Handle suggested title change from analytics
  const handleSuggestedTitleChange = useCallback((title: string) => {
    // Skip if we already applied this title
    if (title === lastAppliedSuggestedTitleRef.current) {
      return;
    }

    if (onSessionUpdate && session) {
      onSessionUpdate({
        ...session,
        suggested_session_title: title,
      });
      lastAppliedSuggestedTitleRef.current = title;
    }
  }, [session, onSessionUpdate]);

  // Compute session metadata for header
  const sessionMeta = useMemo(() => {
    // Claude path: first assistant message carries the model. Codex path
    // (CF-386): walk the parsed rawLines for the first non-empty model via
    // the same session_meta → turn_context fallback the backend parser uses.
    const firstAssistant = messages.find(isAssistantMessage);
    const model = isCodex
      ? extractCodexModel(codexRawLines)
      : firstAssistant?.message.model;

    // Compute duration and date from message timestamps (matches analytics calculation)
    const { durationMs, sessionDate } = computeSessionMeta(messages, {
      firstSeen: session.first_seen,
      lastSyncAt: session.last_sync_at,
    });

    return { model, durationMs, sessionDate };
  }, [messages, isCodex, codexRawLines, session.first_seen, session.last_sync_at]);

  // The transcript header controls (filter chips) apply to both providers on
  // the Transcript tab (CF-361 added Codex filter chips). CF-362 enables the
  // cost-mode toggle for Codex too once per-message usage is wired.
  const showTranscriptControls = activeTab === 'transcript';
  const showClaudeFilters = showTranscriptControls && !isCodex;
  const showCodexFilters = showTranscriptControls && isCodex;
  const showCostToggle = showTranscriptControls;

  return (
    <div className={styles.sessionViewer}>
      <div className={styles.mainContent}>
        <SessionHeader
          sessionId={session.id}
          title={session.custom_title ?? session.suggested_session_title ?? session.summary ?? session.first_user_message ?? undefined}
          hasCustomTitle={!!session.custom_title}
          autoTitle={session.suggested_session_title ?? session.summary ?? session.first_user_message ?? undefined}
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
          isCostMode={showCostToggle ? isCostMode : undefined}
          onToggleCostMode={showCostToggle ? toggleCostMode : undefined}
          categoryCounts={showClaudeFilters ? categoryCounts : undefined}
          filterState={showClaudeFilters ? filterState : undefined}
          onToggleCategory={showClaudeFilters ? toggleCategory : undefined}
          onToggleUserSubcategory={showClaudeFilters ? toggleUserSubcategory : undefined}
          onToggleAssistantSubcategory={showClaudeFilters ? toggleAssistantSubcategory : undefined}
          onToggleAttachmentSubcategory={showClaudeFilters ? toggleAttachmentSubcategory : undefined}
          codexCategoryCounts={showCodexFilters ? codexCategoryCounts : undefined}
          codexFilterState={showCodexFilters ? codexFilterState : undefined}
          onToggleCodexCategory={showCodexFilters ? toggleCodexCategory : undefined}
          onToggleCodexAssistantSubcategory={showCodexFilters ? toggleCodexAssistantSubcategory : undefined}
          onToggleCodexToolCallSubcategory={showCodexFilters ? toggleCodexToolCallSubcategory : undefined}
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
            // still skip on Codex — see SmartRecapCard and the TIL effect
            // above — because both anchor to message UUIDs that Codex
            // messages don't carry.
            <SessionSummaryPanel
              sessionId={session.id}
              isOwner={isOwner}
              initialAnalytics={initialAnalytics}
              initialGithubLinks={initialGithubLinks}
              onSuggestedTitleChange={handleSuggestedTitleChange}
            />
          ) : (
            <div className={styles.timelineContainer}>
              {isCodex ? (
                <CodexTranscriptPane
                  sessionId={session.id}
                  items={codexItems}
                  filteredItems={filteredCodexItems}
                  visibleIndices={visibleCodexIndices}
                  loading={loading}
                  error={error}
                  targetLineId={targetMessageUuid}
                  isCostMode={isCostMode}
                />
              ) : (
                <ClaudeTranscriptPane
                  loading={loading}
                  error={error}
                  filteredMessages={filteredMessages}
                  allMessages={messages}
                  sessionId={session.id}
                  targetMessageUuid={targetMessageUuid}
                  isCostMode={isCostMode}
                  tilsByMessageUuid={sessionTILs}
                />
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

export default SessionViewer;
