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
 *   - update_plan (CF-368): the rendered summary line (never the raw plan JSON)
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
    case 'update_plan': {
      // The body never renders the raw plan; copying the rendered summary
      // keeps the clipboard contents in sync with what the user can see.
      // Returns undefined when the input isn't a usable plan object so the
      // button stays hidden (consistent with the other tools' empty-input case).
      if (!isRecord(item.rawInput)) return undefined;
      return buildPlanSummaryText(readPlanSummary(item.rawInput));
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

// ----------------------------------------------------------------------------
// CF-368: update_plan summary
// ----------------------------------------------------------------------------
//
// Codex's `update_plan` tool calls fire many times per session with the same
// plan repeated and one or two statuses flipped. The raw JSON payload is
// uninteresting; what a reader actually wants to see is the currently-active
// step and the progress count. `readPlanSummary` classifies a payload into
// one of five buckets, and `buildPlanSummaryText` renders the canonical
// one-line summary that both the body component and the search-index
// projection use (no rendering / projection drift).

/** Classification of an `update_plan` payload for the renderer + search. */
export interface PlanSummary {
  bucket: 'empty' | 'complete' | 'in_progress' | 'paused' | 'pending';
  /** Only set when `bucket === 'in_progress'`. */
  activeStep?: string;
  completedCount: number;
  totalCount: number;
}

/**
 * Classify an `update_plan` call's rawInput (the parsed function-call
 * arguments) into a `PlanSummary`. Forward-compat: unrecognized `status`
 * strings count toward `totalCount` but never bump `completedCount`, and
 * malformed entries (non-string `step` field) are skipped cleanly.
 */
export function readPlanSummary(rawInput: unknown): PlanSummary {
  const empty: PlanSummary = { bucket: 'empty', completedCount: 0, totalCount: 0 };
  if (!isRecord(rawInput)) return empty;

  const plan = rawInput.plan;
  if (!Array.isArray(plan) || plan.length === 0) return empty;

  let completedCount = 0;
  let totalCount = 0;
  let pendingCount = 0;
  let activeStep: string | undefined;

  for (const raw of plan) {
    if (!isRecord(raw)) continue;
    if (typeof raw.step !== 'string') continue;
    totalCount += 1;
    const status = typeof raw.status === 'string' ? raw.status : '';
    if (status === 'completed') {
      completedCount += 1;
      continue;
    }
    if (status === 'in_progress' && activeStep === undefined) {
      activeStep = raw.step;
      continue;
    }
    if (status === 'pending') {
      pendingCount += 1;
    }
  }

  if (totalCount === 0) return empty;
  if (activeStep !== undefined) {
    return { bucket: 'in_progress', activeStep, completedCount, totalCount };
  }
  if (completedCount === totalCount) {
    return { bucket: 'complete', completedCount, totalCount };
  }
  if (completedCount === 0 && pendingCount === totalCount) {
    return { bucket: 'pending', completedCount, totalCount };
  }
  // Mix of completed + pending (or unknown statuses) with no active step.
  return { bucket: 'paused', completedCount, totalCount };
}

/**
 * Render the canonical one-line summary for the body and the search index.
 * Shapes:
 *   empty       → `Empty plan`
 *   complete    → `Plan complete · N/N complete`
 *   in_progress → `Now: <activeStep> · N/M complete`
 *   paused      → `Plan paused · N/M complete`
 *   pending     → `Plan registered · 0/N complete`
 */
export function buildPlanSummaryText(summary: PlanSummary): string {
  const progress = `${summary.completedCount}/${summary.totalCount} complete`;
  switch (summary.bucket) {
    case 'empty':
      return 'Empty plan';
    case 'complete':
      return `Plan complete · ${progress}`;
    case 'in_progress':
      return `Now: ${summary.activeStep ?? ''} · ${progress}`;
    case 'paused':
      return `Plan paused · ${progress}`;
    case 'pending':
      return `Plan registered · ${progress}`;
  }
}
