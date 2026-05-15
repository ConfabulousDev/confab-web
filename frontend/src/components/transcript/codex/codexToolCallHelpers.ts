// Pure helpers for Codex tool-call rendering.
//
// Split out of `CodexToolCallBlock.tsx` so the component file exports only
// the component (required by react-refresh) and the helpers stay
// independently testable.

import type { CodexToolCallItem } from '@/types/codexRenderItem';
import { isRecord } from '@/utils/utils';

// ----------------------------------------------------------------------------
// CF-360: per-tool copy-text composition
// ----------------------------------------------------------------------------

/**
 * Compose the text the Copy Text button copies for this tool call.
 *
 * Returns undefined (button hidden) when there's no useful text to share —
 * e.g. a web_search_call with no queries, or a generic tool with neither
 * input nor output.
 *
 * Per CF-360 interview:
 *   - exec_command: `$ <cmd>\n<output>`
 *   - apply_patch: raw `rawInput` only (no output, no structured summary)
 *   - web_search_call: queries joined by newlines
 *   - generic / unknown: stringified rawInput + rawOutput
 */
export function buildToolCallCopyText(item: CodexToolCallItem): string | undefined {
  switch (item.toolName) {
    case 'exec_command': {
      const cmd = readStringField(item.rawInput, 'cmd') ?? '';
      const output = item.rawOutput ?? '';
      const lines: string[] = [];
      if (cmd) lines.push(`$ ${cmd}`);
      if (output) lines.push(output);
      return lines.length > 0 ? lines.join('\n') : undefined;
    }
    case 'apply_patch':
      return typeof item.rawInput === 'string' && item.rawInput.length > 0
        ? item.rawInput
        : undefined;
    case 'web_search_call': {
      const qs = readWebSearchQueries(item.rawInput);
      return qs.length > 0 ? qs.join('\n') : undefined;
    }
    default: {
      const inputStr = stringifyGenericInput(item.rawInput);
      const outputStr = item.rawOutput ?? '';
      const joined = [inputStr, outputStr].filter((s) => s.length > 0).join('\n\n');
      return joined.length > 0 ? joined : undefined;
    }
  }
}

/** Stringify a generic-tool `rawInput` for the copy-text composition.
 *  Strings pass through; objects / arrays become indented JSON; null /
 *  undefined become the empty string (treated as "absent" by the caller).
 */
function stringifyGenericInput(input: unknown): string {
  if (input === undefined || input === null) return '';
  if (typeof input === 'string') return input;
  return JSON.stringify(input, null, 2);
}

// ----------------------------------------------------------------------------
// Unknown-shape readers
// ----------------------------------------------------------------------------
//
// `rawInput` and `structuredOutput` are typed `unknown` because the underlying
// JSONL can carry whatever fields a given tool emits. These helpers extract
// only the fields each renderer cares about, with runtime guards so unfamiliar
// payloads degrade gracefully instead of crashing.

export interface PatchChange {
  type: string;
  content?: string;
}

export function readStringField(value: unknown, key: string): string | null {
  if (!isRecord(value)) return null;
  const v = value[key];
  return typeof v === 'string' ? v : null;
}

export function readPatchChanges(value: unknown): Record<string, PatchChange> {
  if (!isRecord(value)) return {};
  const changes = value.changes;
  if (!isRecord(changes)) return {};
  const out: Record<string, PatchChange> = {};
  for (const [path, raw] of Object.entries(changes)) {
    if (isRecord(raw)) {
      const type = typeof raw.type === 'string' ? raw.type : 'unknown';
      const content = typeof raw.content === 'string' ? raw.content : undefined;
      out[path] = { type, content };
    }
  }
  return out;
}

export function readWebSearchQueries(value: unknown): string[] {
  if (!isRecord(value)) return [];
  const queries = value.queries;
  if (Array.isArray(queries)) {
    return queries.filter((q): q is string => typeof q === 'string');
  }
  const query = value.query;
  return typeof query === 'string' ? [query] : [];
}
