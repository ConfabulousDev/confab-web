// Codex provider adapter (CF-417).
//
// Wraps the existing codexTranscriptService / useCodexTranscriptFilters /
// CodexFilterDropdown / CodexTranscriptPane modules to satisfy the
// `ProviderAdapter` contract. No data-layer reimplementation.

import { useEffect } from 'react';
import {
  fetchParsedCodexTranscript,
  fetchNewCodexLines,
  normalizeCodexLines,
  extractCodexModel,
} from '@/services/codexTranscriptService';
import { useCodexTranscriptFilters } from '@/hooks/useCodexTranscriptFilters';
import {
  DEFAULT_CODEX_FILTER_STATE,
  countCodexCategories,
  codexItemMatchesFilter,
} from '@/components/session/codexCategories';
import CodexFilterDropdown from '@/components/session/CodexFilterDropdown';
import CodexTranscriptPane from '@/components/session/CodexTranscriptPane';
import type { RawCodexLine } from '@/schemas/codexTranscript';
import type {
  CodexAdapter,
  SessionMetaFallback,
  SessionMetaResult,
} from './types';

// Walk rawLines for min/max timestamp. Falls back to firstSeen/lastSyncAt when
// rawLines is empty (e.g., Storybook seed). RawCodexLine is a discriminated
// union; most kinds carry a `timestamp` string — skip lines missing one.
function codexSessionMeta(
  raw: RawCodexLine[],
  fallback: SessionMetaFallback,
): SessionMetaResult {
  let minTs: number | undefined;
  let maxTs: number | undefined;

  for (const line of raw) {
    if (!('timestamp' in line) || typeof line.timestamp !== 'string') continue;
    const ts = Date.parse(line.timestamp);
    if (Number.isNaN(ts)) continue;
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

export const codexAdapter: CodexAdapter = {
  id: 'codex',
  supportsTILs: false,

  async fetchInitial(sessionId, fileName, skipCache) {
    const parsed = await fetchParsedCodexTranscript(sessionId, fileName, skipCache);
    return { items: parsed.items, totalLines: parsed.totalLines, raw: parsed.rawLines };
  },

  async fetchIncremental(sessionId, fileName, currentLineCount) {
    const { newRawLines, newTotalLineCount } = await fetchNewCodexLines(
      sessionId,
      fileName,
      currentLineCount,
    );
    return {
      newItems: normalizeCodexLines(newRawLines),
      newRaw: newRawLines,
      newTotalLineCount,
    };
  },

  normalize: normalizeCodexLines,

  extractModel(raw) {
    return extractCodexModel(raw);
  },

  computeMeta(_items, raw, fallback) {
    return codexSessionMeta(raw, fallback);
  },

  useFilters() {
    const hook = useCodexTranscriptFilters();
    return {
      state: hook.filterState,
      setState: hook.setFilterState,
      toggles: {
        toggleCategory: hook.toggleCategory,
        toggleAssistantSubcategory: hook.toggleAssistantSubcategory,
        toggleToolCallSubcategory: hook.toggleToolCallSubcategory,
      },
    };
  },

  countCategories: countCodexCategories,
  itemMatchesFilter: codexItemMatchesFilter,

  useDeepLinkFilterReset(items, targetId, filters) {
    useEffect(() => {
      if (!targetId || items.length === 0) return;
      const target = items.find((it) => it.lineId === targetId);
      if (!target) return;
      if (codexItemMatchesFilter(target, filters.state)) return;
      filters.setState(
        {
          ...DEFAULT_CODEX_FILTER_STATE,
          reasoning_hidden: target.kind === 'reasoning_hidden',
        },
        { replace: true },
      );
    }, [targetId, items, filters]);
  },

  FilterDropdown({ counts, filters }) {
    return (
      <CodexFilterDropdown
        counts={counts}
        filterState={filters.state}
        onToggleCategory={filters.toggles.toggleCategory}
        onToggleAssistantSubcategory={filters.toggles.toggleAssistantSubcategory}
        onToggleToolCallSubcategory={filters.toggles.toggleToolCallSubcategory}
      />
    );
  },

  TranscriptPane({
    sessionId,
    items,
    filteredItems,
    visibleIndices,
    loading,
    error,
    targetId,
    isCostMode,
  }) {
    return (
      <CodexTranscriptPane
        sessionId={sessionId}
        items={items}
        filteredItems={filteredItems}
        visibleIndices={visibleIndices}
        loading={loading}
        error={error}
        targetLineId={targetId}
        isCostMode={isCostMode}
      />
    );
  },
};
