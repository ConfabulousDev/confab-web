// OpenCode provider adapter.
//
// Wraps opencodeTranscriptService / opencodeCategories / OpenCodeFilterDropdown
// / OpenCodeTranscriptPane to satisfy the ProviderAdapter contract, mirroring
// codexAdapter. The transcript pane is intentionally leaner than Claude/Codex
// (no minimap bar / cost rail / search yet) but real.

import { useEffect, useState } from 'react';
import {
  fetchParsedOpenCodeTranscript,
  fetchNewOpenCodeLines,
  normalizeOpenCodeLines,
  extractOpenCodeModel,
} from '@/services/opencodeTranscriptService';
import {
  DEFAULT_OPENCODE_FILTER_STATE,
  countOpenCodeCategories,
  opencodeItemMatchesFilter,
  type OpenCodeCategory,
  type OpenCodeFilterState,
  type OpenCodeRenderItem,
} from '@/components/session/opencodeCategories';
import { calculateCost } from '@/utils/tokenStats';
import OpenCodeFilterDropdown from '@/components/session/OpenCodeFilterDropdown';
import OpenCodeTranscriptPane from '@/components/session/OpenCodeTranscriptPane';
import type { OpenCodeAdapter, SessionMetaFallback, SessionMetaResult } from './types';

// Walk render items for min/max creation time. Falls back to firstSeen/lastSyncAt
// when there are no items (e.g., Storybook seed).
function opencodeSessionMeta(
  items: OpenCodeRenderItem[],
  fallback: SessionMetaFallback,
): SessionMetaResult {
  let minTs: number | undefined;
  let maxTs: number | undefined;
  for (const item of items) {
    const ts = item.timeCreated;
    if (!ts) continue;
    if (minTs === undefined || ts < minTs) minTs = ts;
    if (maxTs === undefined || ts > maxTs) maxTs = ts;
  }

  if (minTs !== undefined && maxTs !== undefined) {
    return {
      durationMs: maxTs > minTs ? maxTs - minTs : undefined,
      sessionDate: new Date(minTs),
    };
  }

  const start = fallback.firstSeen ? Date.parse(fallback.firstSeen) : NaN;
  const end = fallback.lastSyncAt ? Date.parse(fallback.lastSyncAt) : NaN;
  return {
    durationMs:
      !Number.isNaN(start) && !Number.isNaN(end) && end > start ? end - start : undefined,
    sessionDate: !Number.isNaN(start) ? new Date(start) : undefined,
  };
}

export const opencodeAdapter: OpenCodeAdapter = {
  id: 'opencode',
  supportsTILs: false,

  async fetchInitial(sessionId, fileName, skipCache) {
    const parsed = await fetchParsedOpenCodeTranscript(sessionId, fileName, skipCache);
    return { items: parsed.items, totalLines: parsed.totalLines, raw: parsed.rawLines };
  },

  async fetchIncremental(sessionId, fileName, currentLineCount) {
    const { newRawLines, newTotalLineCount } = await fetchNewOpenCodeLines(
      sessionId,
      fileName,
      currentLineCount,
    );
    return {
      newItems: normalizeOpenCodeLines(newRawLines),
      newRaw: newRawLines,
      newTotalLineCount,
    };
  },

  normalize: normalizeOpenCodeLines,

  extractModel(raw) {
    return extractOpenCodeModel(raw);
  },

  computeMeta(items, _raw, fallback) {
    return opencodeSessionMeta(items, fallback);
  },

  useFilters() {
    const [state, setState] = useState<OpenCodeFilterState>({ ...DEFAULT_OPENCODE_FILTER_STATE });
    return {
      state,
      setState: (next: OpenCodeFilterState) => setState(next),
      toggles: {
        toggleCategory: (cat: OpenCodeCategory) =>
          setState((prev) => ({ ...prev, [cat]: !prev[cat] })),
      },
    };
  },

  countCategories: countOpenCodeCategories,
  itemMatchesFilter: opencodeItemMatchesFilter,

  tokensCostTooltip:
    'Cost reported by OpenCode per message across all providers used in this session.',

  calculateMessageCost(model, usage, message) {
    // OpenCode reports an authoritative per-message cost; prefer it. Fall back to
    // the pricing table only when a message has no reported cost (mirrors the
    // backend hybrid in computeOpenCodeTokens).
    if (message.kind === 'assistant' && typeof message.cost === 'number') {
      return message.cost;
    }
    if (message.kind !== 'assistant') return 0;
    return calculateCost('opencode', model, usage);
  },

  useDeepLinkFilterReset(items, targetId, filters) {
    useEffect(() => {
      if (!targetId || items.length === 0) return;
      const target = items.find((it) => it.id === targetId);
      if (!target) return;
      if (opencodeItemMatchesFilter(target, filters.state)) return;
      // Target is filtered out — reveal everything so the deep link lands.
      filters.setState({ ...DEFAULT_OPENCODE_FILTER_STATE }, { replace: true });
    }, [targetId, items, filters]);
  },

  FilterDropdown({ counts, filters }) {
    return (
      <OpenCodeFilterDropdown
        counts={counts}
        filterState={filters.state}
        onToggleCategory={filters.toggles.toggleCategory}
      />
    );
  },

  TranscriptPane({ sessionId, items, filteredItems, loading, error, targetId, isCostMode }) {
    return (
      <OpenCodeTranscriptPane
        sessionId={sessionId}
        items={items}
        filteredItems={filteredItems}
        loading={loading}
        error={error}
        targetId={targetId}
        isCostMode={isCostMode}
      />
    );
  },
};
