// CF-361: URL-synced filter hook for Codex transcripts.
//
// Parallel to `useTranscriptFilters` (Claude). Shares the same `?hide=`
// URL slot with provider-specific token grammars; tokens from the other
// provider are safely ignored on read so cross-provider URLs degrade
// gracefully.
//
// Codex tokens emitted by this hook:
//   - flat:    `user`, `reasoning_hidden`, `compacted`, `turn_separator`, `unknown`
//   - assistant subs: `assistant.commentary`, `assistant.final`
//   - tool_call subs: `tool_call.exec_command`, `tool_call.apply_patch`,
//                     `tool_call.web_search`, `tool_call.generic`
//
// A path appears in the URL iff its FilterState boolean is `false` (hidden).
// `DEFAULT_HIDDEN` is derived from `DEFAULT_CODEX_FILTER_STATE` so the two
// can never drift.

import { useCallback, useMemo } from 'react';
import { useURLFilters, type URLFiltersConfig } from './useURLFilters';
import {
  DEFAULT_CODEX_FILTER_STATE,
  type CodexCategory,
  type CodexAssistantSubcategory,
  type CodexFilterState,
  type CodexToolCallSubcategory,
} from '@/components/session/codexCategories';

const ASSISTANT_SUBS = ['commentary', 'final'] as const satisfies readonly CodexAssistantSubcategory[];

const TOOL_CALL_SUBS = [
  'exec_command',
  'apply_patch',
  'web_search',
  'generic',
] as const satisfies readonly CodexToolCallSubcategory[];

const FLAT_KEYS = [
  'user',
  'reasoning_hidden',
  'compacted',
  'turn_separator',
  'unknown',
] as const satisfies readonly Exclude<CodexCategory, 'assistant' | 'tool_call'>[];

export function pathsFromState(state: CodexFilterState): string[] {
  const hidden: string[] = [];
  for (const sub of ASSISTANT_SUBS) {
    if (!state.assistant[sub]) hidden.push(`assistant.${sub}`);
  }
  for (const sub of TOOL_CALL_SUBS) {
    if (!state.tool_call[sub]) hidden.push(`tool_call.${sub}`);
  }
  for (const key of FLAT_KEYS) {
    if (!state[key]) hidden.push(key);
  }
  return hidden;
}

export function stateFromPaths(paths: string[]): CodexFilterState {
  const hidden = new Set(paths);
  const visible = (p: string): boolean => !hidden.has(p);
  return {
    user: visible('user'),
    assistant: {
      commentary: visible('assistant.commentary'),
      final: visible('assistant.final'),
    },
    tool_call: {
      exec_command: visible('tool_call.exec_command'),
      apply_patch: visible('tool_call.apply_patch'),
      web_search: visible('tool_call.web_search'),
      generic: visible('tool_call.generic'),
    },
    reasoning_hidden: visible('reasoning_hidden'),
    compacted: visible('compacted'),
    turn_separator: visible('turn_separator'),
    unknown: visible('unknown'),
  };
}

export const DEFAULT_HIDDEN: string[] = pathsFromState(DEFAULT_CODEX_FILTER_STATE);

const CODEX_FILTERS_CONFIG: URLFiltersConfig = {
  hide: { type: 'string[]', default: DEFAULT_HIDDEN, paramName: 'hide' },
};

interface HideFilters {
  hide: string[];
}

export interface CodexTranscriptFiltersResult {
  filterState: CodexFilterState;
  setFilterState: (state: CodexFilterState, opts?: { replace?: boolean }) => void;
  toggleCategory: (category: CodexCategory) => void;
  toggleAssistantSubcategory: (sub: CodexAssistantSubcategory) => void;
  toggleToolCallSubcategory: (sub: CodexToolCallSubcategory) => void;
}

export function useCodexTranscriptFilters(): CodexTranscriptFiltersResult {
  const { filters, setFilter } = useURLFilters<HideFilters>(CODEX_FILTERS_CONFIG);

  const filterState = useMemo(() => stateFromPaths(filters.hide), [filters.hide]);

  const setFilterState = useCallback(
    (state: CodexFilterState, opts?: { replace?: boolean }) => {
      setFilter('hide', pathsFromState(state), opts);
    },
    [setFilter],
  );

  const toggleCategory = useCallback(
    (category: CodexCategory) => {
      const next: CodexFilterState = { ...filterState };
      if (category === 'assistant') {
        const allVisible = ASSISTANT_SUBS.every((k) => filterState.assistant[k]);
        next.assistant = { commentary: !allVisible, final: !allVisible };
      } else if (category === 'tool_call') {
        const allVisible = TOOL_CALL_SUBS.every((k) => filterState.tool_call[k]);
        next.tool_call = {
          exec_command: !allVisible,
          apply_patch: !allVisible,
          web_search: !allVisible,
          generic: !allVisible,
        };
      } else {
        next[category] = !filterState[category];
      }
      setFilter('hide', pathsFromState(next));
    },
    [filterState, setFilter],
  );

  const toggleAssistantSubcategory = useCallback(
    (sub: CodexAssistantSubcategory) => {
      const next: CodexFilterState = {
        ...filterState,
        assistant: { ...filterState.assistant, [sub]: !filterState.assistant[sub] },
      };
      setFilter('hide', pathsFromState(next));
    },
    [filterState, setFilter],
  );

  const toggleToolCallSubcategory = useCallback(
    (sub: CodexToolCallSubcategory) => {
      const next: CodexFilterState = {
        ...filterState,
        tool_call: { ...filterState.tool_call, [sub]: !filterState.tool_call[sub] },
      };
      setFilter('hide', pathsFromState(next));
    },
    [filterState, setFilter],
  );

  return {
    filterState,
    setFilterState,
    toggleCategory,
    toggleAssistantSubcategory,
    toggleToolCallSubcategory,
  };
}
