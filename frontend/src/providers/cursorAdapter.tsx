// Cursor provider adapter (6qwh + 18n2).
//
// Wraps cursorTranscriptService / cursorCategories / CursorFilterDropdown /
// CursorTranscriptPane to satisfy the ProviderAdapter contract, mirroring
// opencodeAdapter. The transcript pane is a virtualized list + Cmd-F search
// with a turn-based timeline minimap (zztp) but no cost rail.
//
// Wire format has no model, token, cost, or per-line timestamp fields (st5f).
// Session timing falls back to firstSeen/lastSyncAt; cost UI shows "Not available".

import { useEffect } from 'react';
import {
  fetchParsedCursorTranscript,
  fetchNewCursorLines,
  normalizeCursorLines,
  extractCursorModel,
} from '@/services/cursorTranscriptService';
import { useCursorTranscriptFilters } from '@/hooks/useCursorTranscriptFilters';
import {
  DEFAULT_CURSOR_FILTER_STATE,
  countCursorCategories,
  cursorItemMatchesFilter,
} from '@/components/session/cursorCategories';
import CursorFilterDropdown from '@/components/session/CursorFilterDropdown';
import CursorTranscriptPane from '@/components/session/CursorTranscriptPane';
import type { CursorAdapter, SessionMetaFallback, SessionMetaResult } from './types';

// Cursor lines carry no timestamp, so session timing always comes from the
// session-level firstSeen/lastSyncAt fallback.
function cursorSessionMeta(fallback: SessionMetaFallback): SessionMetaResult {
  const start = fallback.firstSeen ? Date.parse(fallback.firstSeen) : NaN;
  const end = fallback.lastSyncAt ? Date.parse(fallback.lastSyncAt) : NaN;
  return {
    durationMs:
      !Number.isNaN(start) && !Number.isNaN(end) && end > start ? end - start : undefined,
    sessionDate: !Number.isNaN(start) ? new Date(start) : undefined,
  };
}

export const cursorAdapter: CursorAdapter = {
  id: 'cursor',
  tokensMeasurable: false,

  async fetchInitial(sessionId, fileName, skipCache) {
    const parsed = await fetchParsedCursorTranscript(sessionId, fileName, skipCache);
    return { items: parsed.items, totalLines: parsed.totalLines, raw: parsed.rawLines };
  },

  async fetchIncremental(sessionId, fileName, currentLineCount) {
    const { newRawLines, newTotalLineCount } = await fetchNewCursorLines(
      sessionId,
      fileName,
      currentLineCount,
    );
    return {
      newItems: normalizeCursorLines(newRawLines),
      newRaw: newRawLines,
      newTotalLineCount,
    };
  },

  normalize: normalizeCursorLines,

  extractModel() {
    return extractCursorModel();
  },

  computeMeta(_items, _raw, fallback) {
    return cursorSessionMeta(fallback);
  },

  useFilters() {
    const { filterState, setFilterState, toggleCategory } = useCursorTranscriptFilters();
    return {
      state: filterState,
      setState: setFilterState,
      toggles: { toggleCategory },
    };
  },

  countCategories: countCursorCategories,
  itemMatchesFilter: cursorItemMatchesFilter,

  tokensCostTooltip:
    'Cursor transcripts do not include token or cost data, so per-session cost is not available.',

  tokenSpeedUnavailableTooltip:
    'Cursor transcripts do not include token counts or per-turn timing, so output token speed cannot be computed.',

  calculateMessageCost: () => 0,

  useDeepLinkFilterReset(items, targetId, filters) {
    useEffect(() => {
      if (!targetId || items.length === 0) return;
      const target = items.find((it) => it.id === targetId);
      if (!target) return;
      if (cursorItemMatchesFilter(target, filters.state)) return;
      // Target is filtered out — reveal everything so the deep link lands.
      filters.setState({ ...DEFAULT_CURSOR_FILTER_STATE }, { replace: true });
    }, [targetId, items, filters]);
  },

  FilterDropdown({ counts, filters }) {
    return (
      <CursorFilterDropdown
        counts={counts}
        filterState={filters.state}
        onToggleCategory={filters.toggles.toggleCategory}
      />
    );
  },

  // The pane derives its own greying from items vs filteredItems (zztp) and has
  // no cost rail, so it ignores the contract's visibleIndices / isCostMode. It
  // reads firstSeen/lastSyncAt to estimate per-row timestamps (ce79), which also
  // size the timeline minimap.
  TranscriptPane({ sessionId, items, filteredItems, loading, error, targetId, firstSeen, lastSyncAt }) {
    return (
      <CursorTranscriptPane
        sessionId={sessionId}
        items={items}
        filteredItems={filteredItems}
        loading={loading}
        error={error}
        targetId={targetId}
        firstSeen={firstSeen}
        lastSyncAt={lastSyncAt}
      />
    );
  },
};
