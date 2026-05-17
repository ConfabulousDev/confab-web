// Claude Code provider adapter (CF-417).
//
// Wraps the existing transcriptService / useTranscriptFilters / FilterDropdown /
// ClaudeTranscriptPane modules to satisfy the `ProviderAdapter` contract.
// No data-layer reimplementation; everything delegates.

import { useEffect } from 'react';
import {
  fetchParsedTranscript,
  fetchNewTranscriptMessages,
} from '@/services/transcriptService';
import { useTranscriptFilters } from '@/hooks/useTranscriptFilters';
import {
  DEFAULT_FILTER_STATE,
  countHierarchicalCategories,
  messageMatchesFilter,
} from '@/components/session/messageCategories';
import { isAssistantMessage } from '@/types';
import { computeSessionMeta } from '@/utils/sessionMeta';
import FilterDropdown from '@/components/session/FilterDropdown';
import ClaudeTranscriptPane from '@/components/session/ClaudeTranscriptPane';
import type { ClaudeAdapter } from './types';

export const claudeAdapter: ClaudeAdapter = {
  id: 'claude-code',
  supportsTILs: true,

  async fetchInitial(sessionId, fileName, skipCache) {
    const parsed = await fetchParsedTranscript(sessionId, fileName, skipCache);
    // Claude has no separate "raw" stream — TranscriptLine[] doubles as raw + items.
    return { items: parsed.messages, totalLines: parsed.totalLines, raw: parsed.messages };
  },

  async fetchIncremental(sessionId, fileName, currentLineCount) {
    const { newMessages, newTotalLineCount } = await fetchNewTranscriptMessages(
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
    const hook = useTranscriptFilters();
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

  countCategories: countHierarchicalCategories,
  itemMatchesFilter: messageMatchesFilter,

  useDeepLinkFilterReset(items, targetId, filters) {
    useEffect(() => {
      if (!targetId || items.length === 0) return;
      const target = items.find((m) => 'uuid' in m && m.uuid === targetId);
      if (!target) return;
      if (messageMatchesFilter(target, filters.state)) return;
      filters.setState(
        { ...DEFAULT_FILTER_STATE, system: target.type === 'system' },
        { replace: true },
      );
    }, [targetId, items, filters]);
  },

  FilterDropdown({ counts, filters }) {
    return (
      <FilterDropdown
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
    tilsByMessageUuid,
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
        tilsByMessageUuid={tilsByMessageUuid}
      />
    );
  },
};
