import { useCallback } from 'react';
import {
  useProviderTranscriptFilters,
  type ProviderTranscriptFiltersConfig,
} from './useProviderTranscriptFilters';
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
// DEFAULT_CLAUDE_FILTER_STATE rather than maintained independently. The URL
// sync + toggle machinery is shared via useProviderTranscriptFilters (x5w2).

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

export function pathsFromState(state: ClaudeFilterState): string[] {
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

export function stateFromPaths(hide: string[]): ClaudeFilterState {
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
export const DEFAULT_HIDDEN = pathsFromState(DEFAULT_CLAUDE_FILTER_STATE);

const CONFIG = {
  defaultState: DEFAULT_CLAUDE_FILTER_STATE,
  pathsFromState,
  stateFromPaths,
  hierarchicalKeys: SUB_KEYS,
} satisfies ProviderTranscriptFiltersConfig<ClaudeFilterState>;

interface ClaudeTranscriptFiltersResult {
  filterState: ClaudeFilterState;
  setFilterState: (state: ClaudeFilterState, opts?: { replace?: boolean }) => void;
  toggleCategory: (category: ClaudeCategory) => void;
  toggleUserSubcategory: (subcategory: ClaudeUserSubcategory) => void;
  toggleAssistantSubcategory: (subcategory: ClaudeAssistantSubcategory) => void;
  toggleAttachmentSubcategory: (subcategory: ClaudeAttachmentSubcategory) => void;
}

export function useClaudeTranscriptFilters(): ClaudeTranscriptFiltersResult {
  const { filterState, setFilterState, toggleCategory, toggleSubcategory } =
    useProviderTranscriptFilters<ClaudeFilterState>(CONFIG);

  const toggleUserSubcategory = useCallback(
    (subcategory: ClaudeUserSubcategory) => toggleSubcategory('user', subcategory),
    [toggleSubcategory],
  );
  const toggleAssistantSubcategory = useCallback(
    (subcategory: ClaudeAssistantSubcategory) => toggleSubcategory('assistant', subcategory),
    [toggleSubcategory],
  );
  const toggleAttachmentSubcategory = useCallback(
    (subcategory: ClaudeAttachmentSubcategory) => toggleSubcategory('attachment', subcategory),
    [toggleSubcategory],
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
