import type {
  ProviderAdapter,
  OpenCodeAdapter,
} from './types';
import type { OpenCodeRenderItem, OpenCodeFilterState, OpenCodeHierarchicalCounts } from '@/components/session/opencodeCategories';
import {
  countOpenCodeCategories,
  opencodeItemMatchesFilter,
  DEFAULT_OPENCODE_FILTER_STATE,
} from '@/components/session/opencodeCategories';
import { useState } from 'react';

type OpenCodeRawLine = { info: { role: string; modelID?: string; providerID?: string; time: { created: number } }; parts: unknown[] };

export const opencodeAdapter: OpenCodeAdapter = {
  id: 'opencode',
  supportsTILs: false,

  async fetchInitial(_sessionId: string, _fileName: string) {
    return { items: [] as OpenCodeRenderItem[], totalLines: 0, raw: [] as OpenCodeRawLine[] };
  },

  async fetchIncremental(_sessionId: string, _fileName: string, _currentLineCount: number) {
    return { newItems: [] as OpenCodeRenderItem[], newRaw: [] as OpenCodeRawLine[], newTotalLineCount: 0 };
  },

  normalize(raw: OpenCodeRawLine[]) {
    return raw as unknown as OpenCodeRenderItem[];
  },

  extractModel(_raw: OpenCodeRawLine[], _items: OpenCodeRenderItem[]) {
    return null;
  },

  computeMeta(_items: OpenCodeRenderItem[], _raw: OpenCodeRawLine[], fallback: { firstSeen?: string; lastSyncAt?: string }) {
    return { durationMs: null, sessionDate: fallback.firstSeen ? new Date(fallback.firstSeen) : null };
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

  useDeepLinkFilterReset(_items: OpenCodeRenderItem[], _targetId: string | undefined, _filters: ReturnType<OpenCodeAdapter['useFilters']>) {
  },

  calculateMessageCost(_model: string | null, _usage: { input: number; output: number; cacheWrite: number; cacheRead: number }, _message: unknown) {
    return 0;
  },

  tokensCostTooltip: 'Cost computed from per-model pricing across all providers used in this session',

  FilterDropdown: () => null,
  TranscriptPane: () => null,
};
