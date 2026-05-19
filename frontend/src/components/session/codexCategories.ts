// CF-361: Codex transcript category model — parallel to messageCategories.ts
// but tuned to the Codex render-item shape.
//
// Categories:
//   - user: flat (no subs). CodexUserItem carries optional images[] (CF-388)
//     but image-only filtering is not exposed; the chip toggles all user rows.
//   - assistant: subs `commentary` | `final` (mirrors CodexAssistantItem.phase).
//   - tool_call: subs `exec_command` | `apply_patch` | `web_search` | `generic`.
//     Bucket selected by `categorizeCodexToolCall` from the raw toolName.
//     Future tool names fall into `generic`; the mapping is the single edit.
//   - flat: `reasoning_hidden`, `compacted`, `turn_separator`, `turn_aborted`
//     (CF-368), `unknown`.
//
// `reasoning_hidden` is the only default-hidden category — parallel to
// Claude's attachment chips defaulting to hidden. Everything else is visible
// by default so first-time users see the full transcript.

import type { CodexRenderItem, CodexToolCallItem } from '@/types/codexRenderItem';

export type CodexCategory =
  | 'user'
  | 'assistant'
  | 'tool_call'
  | 'reasoning_hidden'
  | 'compacted'
  | 'turn_separator'
  | 'turn_aborted'
  | 'unknown';

export type CodexAssistantSubcategory = 'commentary' | 'final';

export type CodexToolCallSubcategory =
  | 'exec_command'
  | 'apply_patch'
  | 'web_search'
  | 'generic';

export interface CodexAssistantSubcategoryCounts {
  commentary: number;
  final: number;
}

export interface CodexToolCallSubcategoryCounts {
  exec_command: number;
  apply_patch: number;
  web_search: number;
  generic: number;
}

export interface CodexHierarchicalCounts {
  user: number;
  assistant: { total: number } & CodexAssistantSubcategoryCounts;
  tool_call: { total: number } & CodexToolCallSubcategoryCounts;
  reasoning_hidden: number;
  compacted: number;
  turn_separator: number;
  turn_aborted: number;
  unknown: number;
}

export interface CodexFilterState {
  user: boolean;
  assistant: { commentary: boolean; final: boolean };
  tool_call: {
    exec_command: boolean;
    apply_patch: boolean;
    web_search: boolean;
    generic: boolean;
  };
  reasoning_hidden: boolean;
  compacted: boolean;
  turn_separator: boolean;
  turn_aborted: boolean;
  unknown: boolean;
}

export const DEFAULT_CODEX_FILTER_STATE: CodexFilterState = {
  user: true,
  assistant: { commentary: true, final: true },
  tool_call: { exec_command: true, apply_patch: true, web_search: true, generic: true },
  reasoning_hidden: false,
  compacted: true,
  turn_separator: true,
  turn_aborted: true,
  unknown: true,
};

export function categorizeCodexToolCall(item: CodexToolCallItem): CodexToolCallSubcategory {
  switch (item.toolName) {
    case 'exec_command':
      return 'exec_command';
    case 'apply_patch':
      return 'apply_patch';
    case 'web_search_call':
      return 'web_search';
    default:
      return 'generic';
  }
}

export function countCodexCategories(items: CodexRenderItem[]): CodexHierarchicalCounts {
  const counts: CodexHierarchicalCounts = {
    user: 0,
    assistant: { total: 0, commentary: 0, final: 0 },
    tool_call: { total: 0, exec_command: 0, apply_patch: 0, web_search: 0, generic: 0 },
    reasoning_hidden: 0,
    compacted: 0,
    turn_separator: 0,
    turn_aborted: 0,
    unknown: 0,
  };

  for (const item of items) {
    switch (item.kind) {
      case 'user':
        counts.user++;
        break;
      case 'assistant':
        counts.assistant.total++;
        counts.assistant[item.phase]++;
        break;
      case 'tool_call': {
        const sub = categorizeCodexToolCall(item);
        counts.tool_call.total++;
        counts.tool_call[sub]++;
        break;
      }
      case 'reasoning_hidden':
        counts.reasoning_hidden++;
        break;
      case 'compacted':
        counts.compacted++;
        break;
      case 'turn_separator':
        counts.turn_separator++;
        break;
      case 'turn_aborted':
        counts.turn_aborted++;
        break;
      case 'unknown':
        counts.unknown++;
        break;
    }
  }

  return counts;
}

export function codexItemMatchesFilter(
  item: CodexRenderItem,
  state: CodexFilterState,
): boolean {
  switch (item.kind) {
    case 'user':
      return state.user;
    case 'assistant':
      return state.assistant[item.phase];
    case 'tool_call':
      return state.tool_call[categorizeCodexToolCall(item)];
    case 'reasoning_hidden':
      return state.reasoning_hidden;
    case 'compacted':
      return state.compacted;
    case 'turn_separator':
      return state.turn_separator;
    case 'turn_aborted':
      return state.turn_aborted;
    case 'unknown':
      return state.unknown;
  }
}
