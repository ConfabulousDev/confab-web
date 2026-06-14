// Claude Code provider adapter (CF-417).
//
// Wraps the existing claudeTranscriptService / useClaudeTranscriptFilters / ClaudeFilterDropdown /
// ClaudeTranscriptPane modules to satisfy the `ProviderAdapter` contract.
// No data-layer reimplementation; everything delegates.

import { useEffect } from 'react';
import {
  fetchParsedClaudeTranscript,
  fetchNewClaudeTranscriptMessages,
} from '@/services/claudeTranscriptService';
import { useClaudeTranscriptFilters } from '@/hooks/useClaudeTranscriptFilters';
import {
  DEFAULT_CLAUDE_FILTER_STATE,
  countClaudeCategories,
  claudeItemMatchesFilter,
} from '@/components/session/claudeCategories';
import { isAssistantMessage } from '@/types';
import { computeSessionMeta } from '@/utils/sessionMeta';
import {
  calculateCost,
  cacheWriteTotal,
  FAST_MODE_MULTIPLIER,
  WEB_SEARCH_COST_PER_REQUEST,
} from '@/utils/tokenStats';
import ClaudeFilterDropdown from '@/components/session/ClaudeFilterDropdown';
import ClaudeTranscriptPane from '@/components/session/ClaudeTranscriptPane';
import type { ClaudeAdapter } from './types';

export const claudeAdapter: ClaudeAdapter = {
  id: 'claude-code',
  async fetchInitial(sessionId, fileName, skipCache) {
    const parsed = await fetchParsedClaudeTranscript(sessionId, fileName, skipCache);
    // Claude has no separate "raw" stream — TranscriptLine[] doubles as raw + items.
    return { items: parsed.messages, totalLines: parsed.totalLines, raw: parsed.messages };
  },

  async fetchIncremental(sessionId, fileName, currentLineCount) {
    const { newMessages, newTotalLineCount } = await fetchNewClaudeTranscriptMessages(
      sessionId,
      fileName,
      currentLineCount,
    );
    return { newItems: newMessages, newRaw: newMessages, newTotalLineCount };
  },

  normalize(raw) {
    return raw;
  },

  extractModel(_raw, items) {
    return items.find(isAssistantMessage)?.message.model;
  },

  computeMeta(items, _raw, fallback) {
    return computeSessionMeta(items, {
      firstSeen: fallback.firstSeen ?? undefined,
      lastSyncAt: fallback.lastSyncAt ?? undefined,
    });
  },

  useFilters() {
    const hook = useClaudeTranscriptFilters();
    return {
      state: hook.filterState,
      setState: hook.setFilterState,
      toggles: {
        toggleCategory: hook.toggleCategory,
        toggleUserSubcategory: hook.toggleUserSubcategory,
        toggleAssistantSubcategory: hook.toggleAssistantSubcategory,
        toggleAttachmentSubcategory: hook.toggleAttachmentSubcategory,
      },
    };
  },

  countCategories: countClaudeCategories,
  itemMatchesFilter: claudeItemMatchesFilter,

  tokensCostTooltip:
    'Estimated API cost based on token usage and model pricing (assumes 5-minute prompt caching)',
  tokensFastTooltip: 'Cost from turns using Anthropic priority tier (~6x base rate)',

  calculateMessageCost(model, usage, message) {
    let cost = calculateCost('claude-code', model, usage);
    // Fast mode (Claude-only) and server-tool dollars live on the wire
    // payload, not the canonical TokenUsage.
    const wire = isAssistantMessage(message) ? message.message.usage : undefined;
    if (wire?.speed === 'fast') cost *= FAST_MODE_MULTIPLIER;
    const stu = wire?.server_tool_use;
    if (stu?.web_search_requests) {
      cost += stu.web_search_requests * WEB_SEARCH_COST_PER_REQUEST;
    }
    return cost;
  },

  extendCostTooltip(base, usage, message) {
    const lines = [...base];
    // Show the full cache-creation count (5m + 1h); the tier split is a billing
    // detail, not surfaced as a separate line (rd9v).
    const cacheWrite = cacheWriteTotal(usage);
    if (cacheWrite) {
      lines.push(`Cache write tokens (write): ${cacheWrite.toLocaleString()}`);
    }
    if (usage.cacheRead) {
      lines.push(`Cache read tokens (hit): ${usage.cacheRead.toLocaleString()}`);
    }
    const wire = isAssistantMessage(message) ? message.message.usage : undefined;
    if (wire?.speed) {
      lines.push('');
      lines.push(`Speed: ${wire.speed}${wire.speed === 'fast' ? ' (6x pricing)' : ''}`);
    }
    if (wire?.service_tier) {
      lines.push(`Tier: ${wire.service_tier}`);
    }
    const stu = wire?.server_tool_use;
    if (stu && (stu.web_search_requests || stu.web_fetch_requests || stu.code_execution_requests)) {
      lines.push('');
      if (stu.web_search_requests) {
        const dollars = (stu.web_search_requests * WEB_SEARCH_COST_PER_REQUEST).toFixed(2);
        lines.push(`Web searches: ${stu.web_search_requests} ($${dollars})`);
      }
      if (stu.web_fetch_requests) lines.push(`Web fetches: ${stu.web_fetch_requests}`);
      if (stu.code_execution_requests) lines.push(`Code executions: ${stu.code_execution_requests}`);
    }
    return lines;
  },

  useDeepLinkFilterReset(items, targetId, filters) {
    useEffect(() => {
      if (!targetId || items.length === 0) return;
      const target = items.find((m) => 'uuid' in m && m.uuid === targetId);
      if (!target) return;
      if (claudeItemMatchesFilter(target, filters.state)) return;
      filters.setState(
        { ...DEFAULT_CLAUDE_FILTER_STATE, system: target.type === 'system' },
        { replace: true },
      );
    }, [targetId, items, filters]);
  },

  FilterDropdown({ counts, filters }) {
    return (
      <ClaudeFilterDropdown
        counts={counts}
        filterState={filters.state}
        onToggleCategory={filters.toggles.toggleCategory}
        onToggleUserSubcategory={filters.toggles.toggleUserSubcategory}
        onToggleAssistantSubcategory={filters.toggles.toggleAssistantSubcategory}
        onToggleAttachmentSubcategory={filters.toggles.toggleAttachmentSubcategory}
      />
    );
  },

  TranscriptPane({
    sessionId,
    items,
    filteredItems,
    loading,
    error,
    targetId,
    isCostMode,
  }) {
    return (
      <ClaudeTranscriptPane
        loading={loading}
        error={error}
        filteredMessages={filteredItems}
        allMessages={items}
        sessionId={sessionId}
        targetMessageUuid={targetId}
        isCostMode={isCostMode}
      />
    );
  },
};
