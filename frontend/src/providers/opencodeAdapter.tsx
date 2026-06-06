// OpenCode provider adapter (Phase 3b stub).
//
// Minimal implementation to satisfy the ProviderAdapter contract.
// Full implementation will be added in Phase 4.

import type { OpenCodeAdapter } from './types';
import type { OpenCodeRenderItem, OpenCodeFilterState } from '@/components/session/opencodeCategories';
import {
  countOpenCodeCategories,
  opencodeItemMatchesFilter,
  DEFAULT_OPENCODE_FILTER_STATE,
} from '@/components/session/opencodeCategories';
import { useState } from 'react';

interface OpenCodeRawLine {
  info: { role: string; modelID?: string; providerID?: string; time: { created: number } };
  parts: unknown[];
}

export const opencodeAdapter: OpenCodeAdapter = {
  id: 'opencode',
  supportsTILs: false,

  async fetchInitial() {
    const items: OpenCodeRenderItem[] = [];
    const raw: OpenCodeRawLine[] = [];
    return { items, totalLines: 0, raw };
  },

  async fetchIncremental() {
    const newItems: OpenCodeRenderItem[] = [];
    const newRaw: OpenCodeRawLine[] = [];
    return { newItems, newRaw, newTotalLineCount: 0 };
  },

  normalize(raw: OpenCodeRawLine[]) {
    return raw;
  },

  extractModel(): string | undefined {
    return undefined;
  },

  computeMeta(_items: OpenCodeRenderItem[], _raw: OpenCodeRawLine[], fallback: { firstSeen?: string; lastSyncAt?: string }) {
    return { durationMs: undefined, sessionDate: fallback.firstSeen ? new Date(fallback.firstSeen) : undefined };
  },

  useFilters() {
    const [state, setState] = useState<OpenCodeFilterState>({ ...DEFAULT_OPENCODE_FILTER_STATE });
    return {
      state,
      setState: (next: OpenCodeFilterState) => setState(next),
      toggles: {
        toggleCategory: (cat: 'user' | 'assistant' | 'tool') => {
          setState((prev) => ({ ...prev, [cat]: !prev[cat] }));
        },
      },
    };
  },

  countCategories: countOpenCodeCategories,
  itemMatchesFilter: opencodeItemMatchesFilter,

  useDeepLinkFilterReset() {
  },

  calculateMessageCost() {
    return 0;
  },

  tokensCostTooltip: 'Cost computed from per-model pricing across all providers used in this session',

  FilterDropdown: () => null,
  TranscriptPane: () => null,
};
