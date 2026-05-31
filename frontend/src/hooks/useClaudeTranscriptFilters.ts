import { useCallback, useMemo } from 'react';
import { useURLFilters, type URLFiltersConfig } from './useURLFilters';
import {
  DEFAULT_CLAUDE_FILTER_STATE,
  type ClaudeFilterState,
  type ClaudeCategory,
  type ClaudeUserSubcategory,
  type ClaudeAssistantSubcategory,
  type ClaudeAttachmentSubcategory,
} from '@/components/session/claudeCategories';

// Wire format: `hide` is a `string[]` of dot-paths into ClaudeFilterState
// (e.g. 'user.prompt', 'attachment.hook', 'away-summary'). A path appears in
// the array iff the matching ClaudeFilterState boolean is `false` (i.e. hidden).
//
// All filter wiring funnels through `pathsFromState` / `stateFromPaths` so the
// path list is the single source of truth — DEFAULT_HIDDEN is derived from
// DEFAULT_CLAUDE_FILTER_STATE rather than maintained independently.

const SUB_KEYS = {
  user: ['prompt', 'tool-result', 'skill'] as const satisfies readonly ClaudeUserSubcategory[],
  assistant: ['text', 'tool-use', 'thinking'] as const satisfies readonly ClaudeAssistantSubcategory[],
  attachment: [
    'hook',
    'file-edit',
    'queued-command',
    'deferred-tools',
    'mcp-instructions',
  ] as const satisfies readonly ClaudeAttachmentSubcategory[],
};

const FLAT_KEYS = [
  'system',
  'file-history-snapshot',
  'summary',
  'queue-operation',
  'pr-link',
  'away-summary',
  'unknown',
] as const satisfies readonly Exclude<ClaudeCategory, 'user' | 'assistant' | 'attachment'>[];

function pathsFromState(state: ClaudeFilterState): string[] {
  const hidden: string[] = [];
  for (const sub of SUB_KEYS.user) {
    if (!state.user[sub]) hidden.push(`user.${sub}`);
  }
  for (const sub of SUB_KEYS.assistant) {
    if (!state.assistant[sub]) hidden.push(`assistant.${sub}`);
  }
  for (const sub of SUB_KEYS.attachment) {
    if (!state.attachment[sub]) hidden.push(`attachment.${sub}`);
  }
  for (const key of FLAT_KEYS) {
    if (!state[key]) hidden.push(key);
  }
  return hidden;
}

function stateFromPaths(hide: string[]): ClaudeFilterState {
  const hidden = new Set(hide);
  const visible = (path: string) => !hidden.has(path);
  return {
    user: {
      prompt: visible('user.prompt'),
      'tool-result': visible('user.tool-result'),
      skill: visible('user.skill'),
    },
    assistant: {
      text: visible('assistant.text'),
      'tool-use': visible('assistant.tool-use'),
      thinking: visible('assistant.thinking'),
    },
    attachment: {
      hook: visible('attachment.hook'),
      'file-edit': visible('attachment.file-edit'),
      'queued-command': visible('attachment.queued-command'),
      'deferred-tools': visible('attachment.deferred-tools'),
      'mcp-instructions': visible('attachment.mcp-instructions'),
    },
    system: visible('system'),
    'file-history-snapshot': visible('file-history-snapshot'),
    summary: visible('summary'),
    'queue-operation': visible('queue-operation'),
    'pr-link': visible('pr-link'),
    'away-summary': visible('away-summary'),
    unknown: visible('unknown'),
  };
}

// Derive the default-hidden list from DEFAULT_CLAUDE_FILTER_STATE so the two never
// drift. Anything visible-by-default is omitted from `?hide=...`.
const DEFAULT_HIDDEN = pathsFromState(DEFAULT_CLAUDE_FILTER_STATE);

const TRANSCRIPT_FILTERS_CONFIG: URLFiltersConfig = {
  hide: { type: 'string[]', default: DEFAULT_HIDDEN, paramName: 'hide' },
};

interface HideFilters {
  hide: string[];
}

interface ClaudeTranscriptFiltersResult {
  filterState: ClaudeFilterState;
  setFilterState: (state: ClaudeFilterState, opts?: { replace?: boolean }) => void;
  toggleCategory: (category: ClaudeCategory) => void;
  toggleUserSubcategory: (subcategory: ClaudeUserSubcategory) => void;
  toggleAssistantSubcategory: (subcategory: ClaudeAssistantSubcategory) => void;
  toggleAttachmentSubcategory: (subcategory: ClaudeAttachmentSubcategory) => void;
}

export function useClaudeTranscriptFilters(): ClaudeTranscriptFiltersResult {
  const { filters, setFilter } = useURLFilters<HideFilters>(TRANSCRIPT_FILTERS_CONFIG);

  const filterState = useMemo(
    () => stateFromPaths(filters.hide),
    [filters.hide],
  );

  const setFilterState = useCallback(
    (state: ClaudeFilterState, opts?: { replace?: boolean }) => {
      setFilter('hide', pathsFromState(state), opts);
    },
    [setFilter],
  );

  const toggleCategory = useCallback(
    (category: ClaudeCategory) => {
      const next = { ...filterState };
      if (category === 'user') {
        const allVisible = SUB_KEYS.user.every((k) => filterState.user[k]);
        next.user = { prompt: !allVisible, 'tool-result': !allVisible, skill: !allVisible };
      } else if (category === 'assistant') {
        const allVisible = SUB_KEYS.assistant.every((k) => filterState.assistant[k]);
        next.assistant = { text: !allVisible, 'tool-use': !allVisible, thinking: !allVisible };
      } else if (category === 'attachment') {
        const allVisible = SUB_KEYS.attachment.every((k) => filterState.attachment[k]);
        next.attachment = {
          hook: !allVisible,
          'file-edit': !allVisible,
          'queued-command': !allVisible,
          'deferred-tools': !allVisible,
          'mcp-instructions': !allVisible,
        };
      } else {
        next[category] = !filterState[category];
      }
      setFilter('hide', pathsFromState(next));
    },
    [filterState, setFilter],
  );

  const toggleUserSubcategory = useCallback(
    (subcategory: ClaudeUserSubcategory) => {
      const next: ClaudeFilterState = {
        ...filterState,
        user: { ...filterState.user, [subcategory]: !filterState.user[subcategory] },
      };
      setFilter('hide', pathsFromState(next));
    },
    [filterState, setFilter],
  );

  const toggleAssistantSubcategory = useCallback(
    (subcategory: ClaudeAssistantSubcategory) => {
      const next: ClaudeFilterState = {
        ...filterState,
        assistant: { ...filterState.assistant, [subcategory]: !filterState.assistant[subcategory] },
      };
      setFilter('hide', pathsFromState(next));
    },
    [filterState, setFilter],
  );

  const toggleAttachmentSubcategory = useCallback(
    (subcategory: ClaudeAttachmentSubcategory) => {
      const next: ClaudeFilterState = {
        ...filterState,
        attachment: { ...filterState.attachment, [subcategory]: !filterState.attachment[subcategory] },
      };
      setFilter('hide', pathsFromState(next));
    },
    [filterState, setFilter],
  );

  return {
    filterState,
    setFilterState,
    toggleCategory,
    toggleUserSubcategory,
    toggleAssistantSubcategory,
    toggleAttachmentSubcategory,
  };
}
