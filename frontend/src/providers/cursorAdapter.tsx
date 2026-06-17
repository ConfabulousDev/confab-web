// Cursor provider adapter (6qwh + 18n2).
//
// Wraps cursorTranscriptService / cursorCategories / CursorFilterDropdown /
// CursorTranscriptPane to satisfy the ProviderAdapter contract, mirroring
// opencodeAdapter. The transcript pane is the MVP (virtualized list + Cmd-F
// search, no minimap/cost rail).
//
// Cursor-specific degradations (all from the wire format having no model / token
// / cost / timestamp fields):
//   - extractModel always returns undefined.
//   - computeMeta has no per-line timestamp, so it always falls back to the
//     session's firstSeen/lastSyncAt.
//   - calculateMessageCost returns 0 (no usage on messages; backend serves
//     empty tokens_v2 + no cursor pricing — cost UI stays hidden). Real Cursor
//     cost tracking is a follow-up (kata 59m1).

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

  // Cursor JSONL has no per-message usage or cost, and the backend serves no
  // cursor pricing — every message resolves to $0, so the cost UI stays hidden.
  calculateMessageCost() {
    return 0;
  },

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

  // The pane is MVP and ignores visibleIndices / isCostMode (no timeline bar,
  // no cost rail), but the adapter contract supplies them.
  TranscriptPane({ sessionId, items, filteredItems, loading, error, targetId }) {
    return (
      <CursorTranscriptPane
        sessionId={sessionId}
        items={items}
        filteredItems={filteredItems}
        loading={loading}
        error={error}
        targetId={targetId}
      />
    );
  },
};
