// Service for fetching and parsing Codex rollout transcripts.
// Mirrors the Claude transcriptService surface but consumes a different
// on-disk schema. The backend sync/file endpoint streams raw JSONL bytes
// regardless of provider; the difference is entirely in the parse layer.

import type {
  CodexAssistantUsage,
  CodexRenderItem,
  CodexToolCallItem,
  CodexTurnSeparatorItem,
} from '@/types/codexRenderItem';
import type { CodexTokenUsageDetails } from '@/schemas/codexTranscript';
import {
  type TranscriptValidationError,
  formatValidationErrorsForLog,
} from '@/schemas/transcript';
import {
  RawCodexLineSchema,
  isKnownCodexLine,
  isKnownResponseItemPayload,
  isKnownEventPayload,
  type CodexParseResult,
  type RawCodexLine,
  type CodexResponseItemLine,
  type CodexEventMsgLine,
  type CodexResponseMessage,
} from '@/schemas/codexTranscript';
import { isRecord } from '@/utils/utils';
import { syncFilesAPI } from './api';

// Maximum errors per report (must match backend maxClientErrors).
const MAX_ERRORS_PER_REPORT = 50;

// Dedupe across re-parses: only report each session once per page-load.
const reportedSessions = new Set<string>();

/**
 * Report Codex transcript validation errors to the backend for observability.
 * Uses raw fetch (bypasses APIClient) so 401s don't redirect the user.
 * Fire-and-forget: errors are silently ignored.
 *
 * Separate category from Claude's `transcript_validation` so the two can be
 * triaged independently in observability tooling.
 */
export function reportCodexTranscriptErrors(
  sessionId: string,
  errors: TranscriptValidationError[],
): void {
  const payload = {
    category: 'codex_transcript_validation',
    session_id: sessionId,
    errors: errors.slice(0, MAX_ERRORS_PER_REPORT).map((e) => ({
      line: e.line,
      message_type: e.messageType,
      details: e.errors.map((d) => ({
        path: d.path,
        message: d.message,
        expected: d.expected,
        received: d.received,
      })),
      raw_json_preview: e.rawJson.slice(0, 500),
    })),
    context: {
      url: typeof window !== 'undefined' ? window.location.pathname : undefined,
      user_agent: typeof navigator !== 'undefined' ? navigator.userAgent : undefined,
    },
  };

  fetch('/api/v1/client-errors', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify(payload),
  }).catch(() => {}); // Fire-and-forget
}

/** Reset the dedup set (exposed for testing) */
export function _resetReportedCodexSessions(): void {
  reportedSessions.clear();
}

// ============================================================================
// JSONL parsing
// ============================================================================

/**
 * Parse a Codex rollout JSONL string into validated `RawCodexLine` records.
 *
 * Empty lines are skipped. Lines that fail JSON parsing or schema validation
 * are recorded in `errors` but do not abort the parse — remaining lines
 * continue to process.
 *
 * `totalLines` reflects the count of non-empty lines (used by the line-offset
 * incremental fetch to stay in sync with the file even when some lines fail).
 */
export function parseCodexJSONL(jsonl: string): CodexParseResult {
  const lines = jsonl.split('\n').filter((line) => line.trim().length > 0);
  const rawLines: RawCodexLine[] = [];
  const errors: TranscriptValidationError[] = [];

  lines.forEach((line, index) => {
    let parsed: unknown;
    try {
      parsed = JSON.parse(line);
    } catch (e) {
      errors.push({
        line: index + 1,
        rawJson: line.length > 200 ? line.slice(0, 200) + '...' : line,
        errors: [
          {
            path: '(root)',
            message: `Invalid JSON: ${e instanceof Error ? e.message : 'parse error'}`,
          },
        ],
      });
      return;
    }

    const result = RawCodexLineSchema.safeParse(parsed);
    if (result.success) {
      rawLines.push(result.data);
    } else {
      const messageType = extractMessageType(parsed);
      errors.push({
        line: index + 1,
        rawJson: line,
        messageType,
        errors: result.error.issues.map((issue) => ({
          path: issue.path.length > 0 ? issue.path.join('.') : '(root)',
          message: issue.message,
          expected: 'expected' in issue ? String(issue.expected) : undefined,
          received: 'received' in issue ? String(issue.received) : undefined,
        })),
      });
    }
  });

  if (errors.length > 0) {
    console.warn(formatValidationErrorsForLog(errors));
  }

  return {
    rawLines,
    errors,
    totalLines: lines.length,
    successCount: rawLines.length,
    errorCount: errors.length,
  };
}

// ============================================================================
// Normalization helpers
// ============================================================================

/**
 * Read `.type` from a possibly-unknown JSON value without an `as` cast.
 * Returns undefined when the input isn't an object with a string `type`.
 */
function extractMessageType(value: unknown): string | undefined {
  if (!isRecord(value)) return undefined;
  return typeof value.type === 'string' ? value.type : undefined;
}

/**
 * Codex injects `<image name=[Image #N]>` / `</image>` sentinel wrappers
 * around each attached image (see openai/codex
 * `local_image_content_items_with_label_number`). These are machine-generated,
 * not user-typed, so stripping them is safe. The regex tolerates any
 * attribute payload (`<image>`, `<image name=[Image #1]>`, future
 * `<image id=42>`); the closing tag is always literal `</image>`.
 */
const CODEX_IMAGE_SENTINEL_RE = /<image[^>]*>|<\/image>/g;

/**
 * Walk a response message's content array and split it into rendered text
 * vs. image URLs (CF-388). `input_text` / `output_text` blocks are joined
 * after sentinel-wrapper stripping; `input_image` / `output_image` blocks
 * contribute their `image_url` (typically an inlined base64 data URL).
 * Unknown block types are dropped — renderers branch on them upstream if
 * they ever matter.
 */
function joinMessageText(msg: CodexResponseMessage): {
  text: string;
  images: string[];
} {
  const texts: string[] = [];
  const images: string[] = [];
  for (const block of msg.content) {
    if ((block.type === 'input_text' || block.type === 'output_text') && 'text' in block) {
      const raw = block.text;
      if (typeof raw !== 'string') continue;
      const stripped = raw.replace(CODEX_IMAGE_SENTINEL_RE, '').trim();
      if (stripped.length > 0) texts.push(stripped);
      continue;
    }
    if (block.type === 'input_image' || block.type === 'output_image') {
      const url = 'image_url' in block ? block.image_url : undefined;
      if (typeof url === 'string' && url.length > 0) images.push(url);
    }
  }
  return { text: texts.join('\n'), images };
}

/** Drop `<environment_context>…</environment_context>` blocks from user text. */
function stripEnvironmentContext(text: string): string {
  return text
    .replace(/<environment_context>[\s\S]*?<\/environment_context>/g, '')
    .trim();
}

/**
 * Codex `function_call_output.output` is wrapped with a metadata preamble:
 *
 *   Chunk ID: 155fed
 *   Wall time: 0.0000 seconds
 *   Process exited with code 0
 *   Original token count: 7
 *   Output:
 *   <actual command output>
 *
 * Split on the `Output:\n` sentinel, parse the preamble fields, and return
 * the body separately.
 */
const OUTPUT_SENTINEL = 'Output:\n';

function parseExecOutput(raw: string): {
  body: string;
  exitCode: number;
  wallTimeMs: number;
} {
  // The sentinel sits on its own line. Match either at the very start of the
  // string or right after a newline, so we don't false-match a string that
  // happens to contain "Output:" mid-line.
  const sentinelIdx = raw.startsWith(OUTPUT_SENTINEL)
    ? 0
    : raw.indexOf(`\n${OUTPUT_SENTINEL}`);

  if (sentinelIdx === -1) {
    // No preamble: treat the whole string as the body.
    return { body: trimTrailingNewline(raw), exitCode: 0, wallTimeMs: 0 };
  }

  const preambleEnd = sentinelIdx === 0 ? 0 : sentinelIdx + 1; // skip the leading \n
  const preamble = raw.slice(0, preambleEnd);
  const body = raw.slice(preambleEnd + OUTPUT_SENTINEL.length);

  const exitMatch = preamble.match(/Process exited with code\s+(-?\d+)/);
  const exitCode = exitMatch?.[1] ? Number.parseInt(exitMatch[1], 10) : 0;

  const wallMatch = preamble.match(/Wall time:\s+([\d.]+)\s*seconds?/i);
  const wallTimeMs = wallMatch?.[1]
    ? Math.round(Number.parseFloat(wallMatch[1]) * 1000)
    : 0;

  return { body: trimTrailingNewline(body), exitCode, wallTimeMs };
}

/** Trim a single trailing newline; preserves intentional blank lines inside. */
function trimTrailingNewline(s: string): string {
  return s.endsWith('\n') ? s.slice(0, -1) : s;
}

/** Parse a custom_tool_call_output.output JSON envelope, if present. */
function tryParseJSON(raw: string): unknown {
  try {
    return JSON.parse(raw);
  } catch {
    return undefined;
  }
}

/**
 * CF-362: attach a `last_token_usage` delta to the most-recent assistant
 * item that doesn't already have usage. Walking backwards handles multi-call
 * turns (commentary → token_count → final → token_count) where each
 * inference's cost lands on its own assistant message. No-op if the delta is
 * missing or no assistant has been emitted yet.
 */
function attachTokenCountToAssistant(
  items: CodexRenderItem[],
  delta: CodexTokenUsageDetails | undefined,
): void {
  if (!delta) return;
  for (let i = items.length - 1; i >= 0; i--) {
    const item = items[i];
    if (!item || item.kind !== 'assistant' || item.usage !== undefined) continue;
    const usage: CodexAssistantUsage = {
      input_tokens: delta.input_tokens,
      output_tokens: delta.output_tokens,
    };
    if (delta.cached_input_tokens !== undefined) {
      usage.cached_input_tokens = delta.cached_input_tokens;
    }
    if (delta.reasoning_output_tokens !== undefined) {
      usage.reasoning_output_tokens = delta.reasoning_output_tokens;
    }
    items[i] = { ...item, usage };
    return;
  }
}

/** Coerce a raw web_search_call status string into the render-item enum. */
function webSearchStatus(raw: string | undefined): CodexToolCallItem['status'] {
  switch (raw) {
    case 'completed':
      return 'completed';
    case 'failed':
      return 'failed';
    default:
      return 'unknown';
  }
}

// ============================================================================
// Normalization
// ============================================================================

type ToolCallDraft = {
  index: number; // position in the items array (for in-place updates)
  toolName: string;
};

/**
 * Transform validated raw Codex lines into the render-item stream the
 * timeline component consumes. Pure synchronous function; safe inside
 * `useMemo`.
 *
 * Responsibilities:
 *   - Drop noise: session_meta, turn_context, event_msg.token_count,
 *     event_msg.task_started, event_msg.user_message, event_msg.agent_message,
 *     response_item.message[role=developer]
 *   - Strip `<environment_context>…</environment_context>` from user messages
 *   - Strip exec output preamble; surface exit code + wall time as execMetadata
 *   - Pair function_call ↔ function_call_output by call_id
 *   - Pair custom_tool_call ↔ custom_tool_call_output ↔ event_msg.patch_apply_end
 *   - Emit CodexReasoningHiddenItem for each reasoning line
 *   - Emit CodexTurnSeparatorItem per task_complete (durationMs, timeToFirstTokenMs)
 *   - Emit CodexCompactedItem for each compacted line
 *   - Track current model from session_meta and task_started for assistant items
 *   - Fall back to CodexUnknownItem for unrecognized types
 */
export function normalizeCodexLines(rawLines: RawCodexLine[]): CodexRenderItem[] {
  const items: CodexRenderItem[] = [];
  // Index by call_id so out-of-order outputs / patch_apply_end can still
  // find their matching tool call.
  const callIdToDraft = new Map<string, ToolCallDraft>();
  let currentModel = 'unknown';
  let turnIndex = 0;

  rawLines.forEach((line, idx) => {
    // CF-360: `lineId` is the position in the validated `rawLines` array,
    // stringified. Stable for the lifetime of an append-only rawLines stream
    // and unique per emitted item. See `codexRenderItem.ts` for invariants.
    const lineId = String(idx);

    // Filter the catch-all branch first so the subsequent switch narrows
    // cleanly to one of the five known shapes.
    if (!isKnownCodexLine(line)) {
      items.push({ kind: 'unknown', lineId, timestamp: line.timestamp, rawLine: line });
      return;
    }

    switch (line.type) {
      case 'session_meta': {
        // Pluck the model from the header, drop the line itself.
        if (line.payload.model) currentModel = line.payload.model;
        break;
      }
      case 'turn_context': {
        // Always dropped.
        break;
      }
      case 'compacted': {
        items.push({
          kind: 'compacted',
          lineId,
          timestamp: line.timestamp,
          replacementCount: line.payload.replacement_history?.length ?? 0,
        });
        break;
      }
      case 'response_item': {
        handleResponseItem(line, lineId, items, callIdToDraft, currentModel);
        break;
      }
      case 'event_msg': {
        const { separator, modelUpdate } = handleEventMsg(line, lineId, items, callIdToDraft);
        if (separator) {
          turnIndex += 1;
          separator.turnIndex = turnIndex;
        }
        if (modelUpdate) currentModel = modelUpdate;
        break;
      }
    }
  });

  return items;
}

function handleResponseItem(
  line: CodexResponseItemLine,
  lineId: string,
  items: CodexRenderItem[],
  callIdToDraft: Map<string, ToolCallDraft>,
  currentModel: string,
): void {
  const payload = line.payload;
  if (!isKnownResponseItemPayload(payload)) {
    items.push({ kind: 'unknown', lineId, timestamp: line.timestamp, rawLine: line });
    return;
  }

  switch (payload.type) {
    case 'message': {
      handleResponseMessage(line, lineId, payload, items, currentModel);
      break;
    }

    case 'function_call': {
      const rawInput = tryParseJSON(payload.arguments) ?? payload.arguments;
      const toolName = payload.name;
      const item: CodexToolCallItem = {
        kind: 'tool_call',
        lineId,
        timestamp: line.timestamp,
        toolName,
        callId: payload.call_id,
        rawInput,
        status: 'pending',
      };
      items.push(item);
      callIdToDraft.set(payload.call_id, { index: items.length - 1, toolName });
      break;
    }

    case 'function_call_output': {
      const draft = callIdToDraft.get(payload.call_id);
      if (!draft) {
        // No matching call — surface as unknown to avoid silent drops.
        items.push({ kind: 'unknown', lineId, timestamp: line.timestamp, rawLine: line });
        return;
      }
      const existing = items[draft.index];
      if (!existing || existing.kind !== 'tool_call') return;
      if (draft.toolName === 'exec_command') {
        const { body, exitCode, wallTimeMs } = parseExecOutput(payload.output);
        const status: CodexToolCallItem['status'] = exitCode === 0 ? 'completed' : 'failed';
        items[draft.index] = {
          ...existing,
          rawOutput: body,
          execMetadata: { exitCode, wallTimeMs },
          status,
        };
      } else {
        items[draft.index] = {
          ...existing,
          rawOutput: payload.output,
          status: 'completed',
        };
      }
      break;
    }

    case 'custom_tool_call': {
      const item: CodexToolCallItem = {
        kind: 'tool_call',
        lineId,
        timestamp: line.timestamp,
        toolName: payload.name,
        callId: payload.call_id,
        rawInput: payload.input,
        status: payload.status === 'completed' ? 'completed' : 'pending',
      };
      items.push(item);
      callIdToDraft.set(payload.call_id, {
        index: items.length - 1,
        toolName: payload.name,
      });
      break;
    }

    case 'custom_tool_call_output': {
      const draft = callIdToDraft.get(payload.call_id);
      if (!draft) {
        items.push({ kind: 'unknown', lineId, timestamp: line.timestamp, rawLine: line });
        return;
      }
      const existing = items[draft.index];
      if (!existing || existing.kind !== 'tool_call') return;
      items[draft.index] = {
        ...existing,
        rawOutput: payload.output,
        status: existing.status === 'pending' ? 'completed' : existing.status,
      };
      break;
    }

    case 'reasoning': {
      items.push({ kind: 'reasoning_hidden', lineId, timestamp: line.timestamp });
      break;
    }

    case 'web_search_call': {
      // Treat web search like any other tool call — render as inline summary.
      items.push({
        kind: 'tool_call',
        lineId,
        timestamp: line.timestamp,
        toolName: 'web_search_call',
        callId: `web-search-${items.length}`,
        rawInput: payload.action ?? {},
        status: webSearchStatus(payload.status),
      });
      break;
    }
  }
}

function handleResponseMessage(
  line: CodexResponseItemLine,
  lineId: string,
  msg: CodexResponseMessage,
  items: CodexRenderItem[],
  currentModel: string,
): void {
  switch (msg.role) {
    case 'developer':
      // Drop developer-role messages entirely (sandbox/permissions noise).
      return;
    case 'user': {
      const { text, images } = joinMessageText(msg);
      const cleaned = stripEnvironmentContext(text);
      // CF-388: keep image-only messages — sentinel-stripped text may be
      // empty but the user clearly attached an image and meant to send it.
      // env_context-only messages (no text, no images) still skip.
      if (cleaned.length === 0 && images.length === 0) return;
      items.push({
        kind: 'user',
        lineId,
        timestamp: line.timestamp,
        text: cleaned,
        ...(images.length > 0 ? { images } : {}),
      });
      return;
    }
    case 'assistant': {
      const { text, images } = joinMessageText(msg);
      const phase = msg.phase === 'commentary' ? 'commentary' : 'final';
      items.push({
        kind: 'assistant',
        lineId,
        timestamp: line.timestamp,
        text,
        phase,
        model: currentModel,
        ...(images.length > 0 ? { images } : {}),
      });
      return;
    }
    default:
      // Unknown role — keep as unknown item so it surfaces in the UI.
      items.push({ kind: 'unknown', lineId, timestamp: line.timestamp, rawLine: line });
  }
}

/**
 * Result of handling one `event_msg` line. Both fields are optional:
 *   - `separator`: a newly-created turn boundary; the caller assigns its
 *     1-based `turnIndex` so the count is monotonic across all turns.
 *   - `modelUpdate`: the next assistant message should be attributed to this
 *     model (sourced from `task_started.model`).
 */
interface EventMsgResult {
  separator?: CodexTurnSeparatorItem;
  modelUpdate?: string;
}

function handleEventMsg(
  line: CodexEventMsgLine,
  lineId: string,
  items: CodexRenderItem[],
  callIdToDraft: Map<string, ToolCallDraft>,
): EventMsgResult {
  const payload = line.payload;
  if (!isKnownEventPayload(payload)) {
    items.push({ kind: 'unknown', lineId, timestamp: line.timestamp, rawLine: line });
    return {};
  }

  switch (payload.type) {
    case 'user_message':
    case 'agent_message':
      // Dropped: redundant with response_item.
      return {};
    case 'token_count':
      // CF-362: per-API-call usage. Attach `last_token_usage` to the most-
      // recent unannotated assistant render-item (any phase). One model
      // inference produces a group of response_items; the assistant message
      // among them is the natural anchor for cost. Multi-call turns produce
      // multiple token_count events, each binding to its own assistant item.
      attachTokenCountToAssistant(items, payload.info?.last_token_usage);
      return {};
    case 'task_started':
      return payload.model ? { modelUpdate: payload.model } : {};
    case 'task_complete': {
      const separator: CodexTurnSeparatorItem = {
        kind: 'turn_separator',
        lineId,
        timestamp: line.timestamp,
        turnIndex: 0, // overwritten by caller
        durationMs: payload.duration_ms ?? 0,
        timeToFirstTokenMs: payload.time_to_first_token_ms,
      };
      items.push(separator);
      return { separator };
    }
    case 'patch_apply_end': {
      if (!payload.call_id) return {};
      const draft = callIdToDraft.get(payload.call_id);
      if (!draft) return {};
      const existing = items[draft.index];
      if (!existing || existing.kind !== 'tool_call') return {};
      items[draft.index] = {
        ...existing,
        structuredOutput: {
          success: payload.success ?? false,
          stdout: payload.stdout,
          stderr: payload.stderr,
          changes: payload.changes ?? {},
        },
        status: payload.success === false ? 'failed' : existing.status,
      };
      return {};
    }
  }
}

// ============================================================================
// Fetch + cache
// ============================================================================

interface CacheEntry {
  rawLines: RawCodexLine[];
  errors: TranscriptValidationError[];
  totalLines: number;
}

const codexCache = new Map<string, CacheEntry>();

async function fetchCodexWithCache(
  sessionId: string,
  fileName: string,
  options: { skipCache?: boolean } = {},
): Promise<CacheEntry> {
  const cacheKey = `${sessionId}-${fileName}`;
  if (!options.skipCache) {
    const cached = codexCache.get(cacheKey);
    if (cached) return cached;
  }

  const content = await syncFilesAPI.getContent(sessionId, fileName);
  const parseResult = parseCodexJSONL(content);

  const entry: CacheEntry = {
    rawLines: parseResult.rawLines,
    errors: parseResult.errors,
    totalLines: parseResult.totalLines,
  };
  codexCache.set(cacheKey, entry);
  return entry;
}

/**
 * Parsed Codex transcript with metadata, returned by `fetchParsedCodexTranscript`.
 */
export interface ParsedCodexTranscript {
  sessionId: string;
  items: CodexRenderItem[];
  rawLines: RawCodexLine[];
  validationErrors: TranscriptValidationError[];
  totalLines: number;
  metadata: {
    itemCount: number;
    rawLineCount: number;
    parseErrorCount: number;
  };
}

/**
 * Fetch and parse the Codex transcript for a session.
 */
export async function fetchParsedCodexTranscript(
  sessionId: string,
  fileName: string,
  skipCache?: boolean,
): Promise<ParsedCodexTranscript> {
  const entry = await fetchCodexWithCache(sessionId, fileName, { skipCache });

  if (entry.errors.length > 0 && !reportedSessions.has(sessionId)) {
    reportedSessions.add(sessionId);
    reportCodexTranscriptErrors(sessionId, entry.errors);
  }

  const items = normalizeCodexLines(entry.rawLines);

  return {
    sessionId,
    items,
    rawLines: entry.rawLines,
    validationErrors: entry.errors,
    totalLines: entry.totalLines,
    metadata: {
      itemCount: items.length,
      rawLineCount: entry.rawLines.length,
      parseErrorCount: entry.errors.length,
    },
  };
}

/**
 * Fetch new Codex lines since `currentLineCount`. Mirrors
 * `fetchNewTranscriptMessages` for Claude: the backend serves only lines
 * after `line_offset`, so callers append the returned raw lines to the
 * accumulated `rawLines` state and re-derive items via `useMemo`.
 */
export async function fetchNewCodexLines(
  sessionId: string,
  fileName: string,
  currentLineCount: number,
): Promise<{ newRawLines: RawCodexLine[]; newTotalLineCount: number }> {
  const content = await syncFilesAPI.getContent(sessionId, fileName, currentLineCount);

  if (!content.trim()) {
    return { newRawLines: [], newTotalLineCount: currentLineCount };
  }

  const parseResult = parseCodexJSONL(content);

  // Total lines tracks the raw file position, not just successful parses, so
  // the next incremental fetch stays in sync even when some lines fail.
  const newTotalLineCount = currentLineCount + parseResult.totalLines;

  if (parseResult.errors.length > 0 && !reportedSessions.has(sessionId)) {
    reportedSessions.add(sessionId);
    reportCodexTranscriptErrors(sessionId, parseResult.errors);
  }

  return {
    newRawLines: parseResult.rawLines,
    newTotalLineCount,
  };
}

/**
 * Scan a parsed Codex rollout's raw lines for the canonical session model.
 *
 * Returns the first non-empty `payload.model` found on either a `session_meta`
 * or `turn_context` line, mirroring the backend parser fallback chain at
 * `backend/internal/codex/parser.go:170-177`: older CLIs write `model` to
 * `session_meta`; newer CLIs (per CF-379) write it per-turn to `turn_context`.
 *
 * Replaces CF-383's `fetchCodexSessionMeta` (which read only the first line
 * and missed the canonical turn_context source).
 *
 * Returns undefined when neither envelope carries a non-empty string model —
 * caller falls back to the provider display label.
 */
export function extractCodexModel(rawLines: RawCodexLine[]): string | undefined {
  for (const line of rawLines) {
    // `isKnownCodexLine` filters out the Unknown catch-all branch so TS
    // narrows `line.payload` to the typed schema variants below.
    if (!isKnownCodexLine(line)) continue;
    if (line.type !== 'session_meta' && line.type !== 'turn_context') continue;
    const model = line.payload.model;
    if (typeof model === 'string' && model) return model;
  }
  return undefined;
}
