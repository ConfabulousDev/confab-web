import { useCallback, useMemo } from 'react';
import { useURLFilters, type URLFiltersConfig } from './useURLFilters';
import {
  type FilterState,
  type MessageCategory,
  type UserSubcategory,
  type AssistantSubcategory,
} from '@/components/session/messageCategories';

// Default hidden categories (derived from DEFAULT_FILTER_STATE)
const DEFAULT_HIDDEN = ['system', 'file-history-snapshot', 'summary', 'queue-operation', 'pr-link'];

const TRANSCRIPT_FILTERS_CONFIG: URLFiltersConfig = {
  hide: { type: 'string[]', default: DEFAULT_HIDDEN, paramName: 'hide' },
};

interface HideFilters {
  hide: string[];
}

function filterStateToHideArray(state: FilterState): string[] {
  const hidden: string[] = [];
  if (!state.user.prompt) hidden.push('user.prompt');
  if (!state.user['tool-result']) hidden.push('user.tool-result');
  if (!state.user.skill) hidden.push('user.skill');
  if (!state.assistant.text) hidden.push('assistant.text');
  if (!state.assistant['tool-use']) hidden.push('assistant.tool-use');
  if (!state.assistant.thinking) hidden.push('assistant.thinking');
  if (!state.system) hidden.push('system');
  if (!state['file-history-snapshot']) hidden.push('file-history-snapshot');
  if (!state.summary) hidden.push('summary');
  if (!state['queue-operation']) hidden.push('queue-operation');
  if (!state['pr-link']) hidden.push('pr-link');
  if (!state.unknown) hidden.push('unknown');
  return hidden;
}

function hideArrayToFilterState(hide: string[]): FilterState {
  const hideSet = new Set(hide);
  return {
    user: {
      prompt: !hideSet.has('user.prompt'),
      'tool-result': !hideSet.has('user.tool-result'),
      skill: !hideSet.has('user.skill'),
    },
    assistant: {
      text: !hideSet.has('assistant.text'),
      'tool-use': !hideSet.has('assistant.tool-use'),
      thinking: !hideSet.has('assistant.thinking'),
    },
    system: !hideSet.has('system'),
    'file-history-snapshot': !hideSet.has('file-history-snapshot'),
    summary: !hideSet.has('summary'),
    'queue-operation': !hideSet.has('queue-operation'),
    'pr-link': !hideSet.has('pr-link'),
    unknown: !hideSet.has('unknown'),
  };
}

export interface TranscriptFiltersResult {
  filterState: FilterState;
  setFilterState: (state: FilterState, opts?: { replace?: boolean }) => void;
  toggleCategory: (category: MessageCategory) => void;
  toggleUserSubcategory: (subcategory: UserSubcategory) => void;
  toggleAssistantSubcategory: (subcategory: AssistantSubcategory) => void;
}

export function useTranscriptFilters(): TranscriptFiltersResult {
  const { filters, setFilter } = useURLFilters<HideFilters>(TRANSCRIPT_FILTERS_CONFIG);

  const filterState = useMemo(
    () => hideArrayToFilterState(filters.hide),
    [filters.hide],
  );

  const setFilterState = useCallback(
    (state: FilterState, opts?: { replace?: boolean }) => {
      setFilter('hide', filterStateToHideArray(state), opts);
    },
    [setFilter],
  );

  const toggleCategory = useCallback(
    (category: MessageCategory) => {
      const next = { ...filterState };
      if (category === 'user') {
        const allVisible = filterState.user.prompt && filterState.user['tool-result'] && filterState.user.skill;
        next.user = { prompt: !allVisible, 'tool-result': !allVisible, skill: !allVisible };
      } else if (category === 'assistant') {
        const allVisible = filterState.assistant.text && filterState.assistant['tool-use'] && filterState.assistant.thinking;
        next.assistant = { text: !allVisible, 'tool-use': !allVisible, thinking: !allVisible };
      } else {
        next[category] = !filterState[category];
      }
      setFilter('hide', filterStateToHideArray(next));
    },
    [filterState, setFilter],
  );

  const toggleUserSubcategory = useCallback(
    (subcategory: UserSubcategory) => {
      const next: FilterState = {
        ...filterState,
        user: { ...filterState.user, [subcategory]: !filterState.user[subcategory] },
      };
      setFilter('hide', filterStateToHideArray(next));
    },
    [filterState, setFilter],
  );

  const toggleAssistantSubcategory = useCallback(
    (subcategory: AssistantSubcategory) => {
      const next: FilterState = {
        ...filterState,
        assistant: { ...filterState.assistant, [subcategory]: !filterState.assistant[subcategory] },
      };
      setFilter('hide', filterStateToHideArray(next));
    },
    [filterState, setFilter],
  );

  return {
    filterState,
    setFilterState,
    toggleCategory,
    toggleUserSubcategory,
    toggleAssistantSubcategory,
  };
}
