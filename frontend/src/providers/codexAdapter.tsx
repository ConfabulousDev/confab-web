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
import { calculateCost } from '@/utils/tokenStats';
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

  tokensCostTooltip: 'Estimated API cost based on token usage and OpenAI model pricing.',

  calculateMessageCost(model, usage) {
    // Codex has no fast multiplier and no server-tool dollars. Parse layer
    // already split input into uncached + cacheRead and folded reasoning
    // into output.
    return calculateCost('codex', model, usage);
  },

  extendCostTooltip(base, usage, message) {
    // CF-418: Codex display preserves the gross-input / raw-output presentation
    // users saw pre-refactor by reversing the canonical normalization at display
    // time. `usage.input` is uncached, `usage.cacheRead` is the cache hit subset,
    // and `usage.output` already includes reasoning — reconstruct the wire shape
    // for display so the tooltip matches the pre-refactor formatting byte-for-byte.
    // Sub-lines (`Cached`, `Reasoning`) are interleaved under their parent lines
    // rather than appended at the end, matching the original layout.
    const reasoning = message.kind === 'assistant' ? message.reasoningTokens ?? 0 : 0;
    const grossInput = usage.input + usage.cacheRead;
    const rawOutput = Math.max(0, usage.output - reasoning);
    // base[0] is `formatCost(cost)`; base[1] is the blank separator line.
    const lines: string[] = [base[0] ?? '', ''];
    lines.push(`Input tokens (in): ${grossInput.toLocaleString()}`);
    if (usage.cacheRead) {
      lines.push(`  Cached (hit): ${usage.cacheRead.toLocaleString()}`);
    }
    lines.push(`Output tokens (out): ${rawOutput.toLocaleString()}`);
    if (reasoning) {
      lines.push(`  Reasoning: ${reasoning.toLocaleString()}`);
    }
    return lines;
  },

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
