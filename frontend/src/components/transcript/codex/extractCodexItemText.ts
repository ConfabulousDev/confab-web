// Plain-text search projection for one `CodexRenderItem`.
//
// Bridges `CodexRenderItem` to the generic `useTranscriptSearch` hook
// (CF-359). The returned string is everything the user can see on the
// row — the hook lowercases and `includes`-matches against it. Divider
// kinds whose visible text is metadata-only (`turn_separator`,
// `reasoning_hidden`) return the empty string so they never match.
//
// Re-uses shape readers from `codexToolCallHelpers.ts` for tool_call
// kinds so JSON-shape parsing is not duplicated, and the
// `compactedLabel` helper from `CodexCompactedDivider.tsx` so the
// divider's visible label is the search index entry (no drift).

import type { CodexRenderItem, CodexToolCallItem } from '@/types/codexRenderItem';
import {
  buildPlanSummaryText,
  readPatchChanges,
  readPlanSummary,
  readStringField,
  readWebSearchQueries,
} from './codexToolCallHelpers';
import { stringifyForDisplay } from './codexFormat';
import { compactedLabel } from './CodexCompactedDivider';
import { turnAbortedLabel } from './CodexTurnAbortedDivider';

export function extractCodexItemText(item: CodexRenderItem): string {
  switch (item.kind) {
    case 'user':
    case 'assistant':
      return item.text;
    case 'tool_call':
      return extractToolCallText(item);
    case 'compacted':
      return compactedLabel(item.replacementCount);
    case 'turn_aborted':
      return turnAbortedLabel(item.reason, item.durationMs);
    case 'unknown':
      return stringifyForDisplay(item.rawLine);
    case 'turn_separator':
    case 'reasoning_hidden':
      return '';
  }
}

function extractToolCallText(item: CodexToolCallItem): string {
  const parts: string[] = [];
  switch (item.toolName) {
    case 'exec_command': {
      const cmd = readStringField(item.rawInput, 'cmd');
      if (cmd) parts.push(cmd);
      if (item.rawOutput) parts.push(item.rawOutput);
      break;
    }
    case 'apply_patch': {
      if (typeof item.rawInput === 'string') parts.push(item.rawInput);
      const filePaths = Object.keys(readPatchChanges(item.structuredOutput));
      if (filePaths.length > 0) parts.push(filePaths.join('\n'));
      if (item.rawOutput) parts.push(item.rawOutput);
      break;
    }
    case 'web_search_call': {
      const queries = readWebSearchQueries(item.rawInput);
      if (queries.length > 0) parts.push(queries.join('\n'));
      break;
    }
    // CF-368: update_plan renders the summary line, not the raw plan JSON.
    // The search index must mirror what the user sees on the row.
    case 'update_plan': {
      parts.push(buildPlanSummaryText(readPlanSummary(item.rawInput)));
      break;
    }
    default: {
      if (item.rawInput !== undefined && item.rawInput !== null) {
        parts.push(stringifyForDisplay(item.rawInput));
      }
      if (item.rawOutput) parts.push(item.rawOutput);
    }
  }
  return parts.join('\n');
}
