// Service for fetching and parsing OpenCode transcripts.
//
// Mirrors codexTranscriptService: the backend sync/file endpoint streams raw
// JSONL bytes regardless of provider; the difference is entirely in the parse +
// normalize layer. Each OpenCode line is one message `{ info, parts }`.

import {
  RawOpenCodeLineSchema,
  type RawOpenCodeLine,
  type OpenCodePart,
} from '@/schemas/opencodeTranscript';
import type { OpenCodeRenderItem } from '@/components/session/opencodeCategories';
import type { TokenUsage } from '@/utils/tokenStats';
import { syncFilesAPI } from './api';

// ============================================================================
// Raw entry types
// ============================================================================

/** A line that failed JSON.parse or the (permissive) shape schema. CF-574 keeps
 *  it in the stream — instead of dropping it — so it surfaces as an "unknown"
 *  row that can be reported. `raw` is the parsed object (shape failure) or the
 *  original line string (JSON.parse failure). */
export interface OpenCodeInvalidLine {
  __invalid: true;
  raw: unknown;
}

/** Accumulated transcript stream element: a validated line or an invalid one. */
export type OpenCodeRawEntry = RawOpenCodeLine | OpenCodeInvalidLine;

function isInvalidEntry(entry: OpenCodeRawEntry): entry is OpenCodeInvalidLine {
  return typeof entry === 'object' && entry !== null && '__invalid' in entry;
}

// Part `type` discriminators OpenCode emits (12 total; see
// backend/docs/opencode-sqlite-format.md). We render text/reasoning/tool and
// intentionally ignore the rest — a part type OUTSIDE this set is a genuine
// parser gap (CF-574) rather than a deliberate skip.
const KNOWN_OPENCODE_PART_TYPES = new Set([
  'text',
  'reasoning',
  'tool',
  'file',
  'step-start',
  'step-finish',
  'snapshot',
  'patch',
  'agent',
  'subtask',
  'compaction',
  'retry',
]);

// ============================================================================
// JSONL parsing
// ============================================================================

export interface OpenCodeParseResult {
  rawLines: OpenCodeRawEntry[];
  /** Count of non-empty lines (including those that failed to parse), so the
   *  line-offset incremental fetch stays in sync with the file. */
  totalLines: number;
}

/** Parse an OpenCode transcript JSONL string into stream entries. Empty lines
 *  are skipped; malformed lines (bad JSON or bad shape) are kept as
 *  `OpenCodeInvalidLine` entries (CF-574) so they surface as unknown rows. */
export function parseOpenCodeJSONL(jsonl: string): OpenCodeParseResult {
  const lines = jsonl.split('\n').filter((line) => line.trim().length > 0);
  const rawLines: OpenCodeRawEntry[] = [];
  let errorCount = 0;

  for (const line of lines) {
    let parsed: unknown;
    try {
      parsed = JSON.parse(line);
    } catch {
      errorCount++;
      rawLines.push({ __invalid: true, raw: line });
      continue;
    }
    const result = RawOpenCodeLineSchema.safeParse(parsed);
    if (result.success) {
      rawLines.push(result.data);
    } else {
      errorCount++;
      rawLines.push({ __invalid: true, raw: parsed });
    }
  }

  if (errorCount > 0) {
    console.warn(`OpenCode transcript: surfaced ${errorCount} unparseable line(s) as unknown rows`);
  }

  return { rawLines, totalLines: lines.length };
}

// ============================================================================
// Normalization
// ============================================================================

/** Join the text of every part of the given type into one string. */
function joinParts(parts: OpenCodePart[], type: string): string {
  const out: string[] = [];
  for (const p of parts) {
    if (p.type === type && typeof p.text === 'string' && p.text.length > 0) {
      out.push(p.text);
    }
  }
  return out.join('\n');
}

/** Compact one-line summary of a tool call's input for the row header. */
function toolInputSummary(part: OpenCodePart): string {
  const input = part.state?.input;
  if (input) {
    for (const key of ['command', 'file_path', 'pattern', 'path']) {
      const v = input[key];
      if (typeof v === 'string' && v.length > 0) return v;
    }
  }
  return part.state?.title ?? '';
}

function usageFromTokens(line: RawOpenCodeLine): TokenUsage | undefined {
  const t = line.info.tokens;
  if (!t) return undefined;
  const cacheRead = t.cache?.read ?? 0;
  let input = t.input ?? 0;
  let cacheWrite = t.cache?.write ?? 0;
  // Mirror the backend's per-provider normalization (opencode_compute.go): OpenAI
  // bills cached input as a subset of `input` and never charges cache writes, so
  // the pricing-table cost fallback matches the backend's number.
  if (line.info.providerID === 'openai') {
    input = Math.max(0, input - cacheRead);
    cacheWrite = 0;
  }
  // OpenCode reports a single flat cache-write count (no 5m/1h tier split).
  return { input, output: t.output ?? 0, cacheWrite, cacheWrite1h: 0, cacheRead };
}

const TERMINAL_TOOL_STATUSES = new Set(['completed', 'error']);

/** Emit an unknown render item for each part whose `type` is not one OpenCode is
 *  known to emit (CF-574). Known-but-unhandled types (file, step-start, …) and
 *  non-terminal tool statuses are deliberate skips, not parser gaps. */
function pushUnknownParts(
  items: OpenCodeRenderItem[],
  line: RawOpenCodeLine,
  lineIndex: number,
  created: number,
): void {
  line.parts.forEach((part, partIdx) => {
    if (KNOWN_OPENCODE_PART_TYPES.has(part.type)) return;
    items.push({
      kind: 'unknown',
      id: part.id ?? `oc-unknown-part-${lineIndex}-${partIdx}`,
      reason: 'unrecognized part type',
      unrecognizedType: part.type,
      rawLine: part,
      timeCreated: created,
    });
  });
}

/** Transform accumulated raw OpenCode entries into the render-item stream.
 *  Pure + synchronous; safe inside `useMemo`. The entry's index in the full
 *  (append-only) stream provides a stable id for unknown rows with no natural id. */
export function normalizeOpenCodeLines(rawLines: OpenCodeRawEntry[]): OpenCodeRenderItem[] {
  const items: OpenCodeRenderItem[] = [];

  rawLines.forEach((entry, lineIndex) => {
    if (isInvalidEntry(entry)) {
      items.push({
        kind: 'unknown',
        id: `oc-unknown-${lineIndex}`,
        reason: 'malformed line',
        unrecognizedType: '(unparseable)',
        rawLine: entry.raw,
        timeCreated: 0,
      });
      return;
    }

    const line = entry;
    const info = line.info;
    const created = info.time?.created ?? 0;
    const msgId = info.id ?? '';

    if (info.role === 'user') {
      const text = joinParts(line.parts, 'text');
      if (text.length > 0) {
        items.push({ kind: 'user', id: msgId, text, timeCreated: created });
      }
      pushUnknownParts(items, line, lineIndex, created);
      return;
    }

    if (info.role === 'assistant') {
      const reasoning = joinParts(line.parts, 'reasoning');
      const text = joinParts(line.parts, 'text');
      const usage = usageFromTokens(line);
      if (text.length > 0 || reasoning.length > 0) {
        items.push({
          kind: 'assistant',
          id: msgId,
          text,
          ...(reasoning.length > 0 ? { reasoning } : {}),
          ...(info.modelID ? { model: info.modelID } : {}),
          ...(typeof info.cost === 'number' ? { cost: info.cost } : {}),
          ...(usage ? { usage } : {}),
          timeCreated: created,
        });
      }

      pushUnknownParts(items, line, lineIndex, created);

      line.parts.forEach((part, partIdx) => {
        if (part.type !== 'tool') return;
        const status = part.state?.status;
        if (!status || !TERMINAL_TOOL_STATUSES.has(status)) return;
        items.push({
          kind: 'tool',
          // Stable across re-normalize: the part's own id/callID, else its index
          // within this message's parts — NOT the running items length, which
          // shifts when an earlier tool changes state between polls.
          id: part.id ?? part.callID ?? `${msgId}-tool-${partIdx}`,
          toolName: part.tool ?? 'tool',
          status,
          input: toolInputSummary(part),
          output: part.state?.output ?? part.state?.error ?? '',
          timeCreated: created,
        });
      });
      return;
    }

    // Unrecognized message role — surface instead of silently dropping (CF-574).
    items.push({
      kind: 'unknown',
      id: msgId || `oc-unknown-${lineIndex}`,
      reason: 'unrecognized message role',
      unrecognizedType: info.role,
      rawLine: line,
      timeCreated: created,
    });
  });

  return items;
}

/** First non-empty modelID across the (valid) messages, or undefined. */
export function extractOpenCodeModel(rawLines: OpenCodeRawEntry[]): string | undefined {
  for (const entry of rawLines) {
    if (isInvalidEntry(entry)) continue;
    if (entry.info.modelID) return entry.info.modelID;
  }
  return undefined;
}

// ============================================================================
// Fetch + cache
// ============================================================================

interface CacheEntry {
  rawLines: OpenCodeRawEntry[];
  totalLines: number;
}

const opencodeCache = new Map<string, CacheEntry>();

async function fetchWithCache(
  sessionId: string,
  fileName: string,
  skipCache?: boolean,
): Promise<CacheEntry> {
  const cacheKey = `${sessionId}-${fileName}`;
  if (!skipCache) {
    const cached = opencodeCache.get(cacheKey);
    if (cached) return cached;
  }
  const content = await syncFilesAPI.getContent(sessionId, fileName);
  const { rawLines, totalLines } = parseOpenCodeJSONL(content);
  const entry: CacheEntry = { rawLines, totalLines };
  opencodeCache.set(cacheKey, entry);
  return entry;
}

export interface ParsedOpenCodeTranscript {
  sessionId: string;
  items: OpenCodeRenderItem[];
  rawLines: OpenCodeRawEntry[];
  totalLines: number;
}

/** Fetch + parse the full OpenCode transcript for a session. */
export async function fetchParsedOpenCodeTranscript(
  sessionId: string,
  fileName: string,
  skipCache?: boolean,
): Promise<ParsedOpenCodeTranscript> {
  const entry = await fetchWithCache(sessionId, fileName, skipCache);
  return {
    sessionId,
    items: normalizeOpenCodeLines(entry.rawLines),
    rawLines: entry.rawLines,
    totalLines: entry.totalLines,
  };
}

/** Fetch OpenCode lines after `currentLineCount` (incremental poll). The
 *  backend serves only lines past `line_offset`; callers append the returned
 *  raw lines and re-derive items via `useMemo`. */
export async function fetchNewOpenCodeLines(
  sessionId: string,
  fileName: string,
  currentLineCount: number,
): Promise<{ newRawLines: OpenCodeRawEntry[]; newTotalLineCount: number }> {
  const content = await syncFilesAPI.getContent(sessionId, fileName, currentLineCount);
  if (!content.trim()) {
    return { newRawLines: [], newTotalLineCount: currentLineCount };
  }
  const { rawLines, totalLines } = parseOpenCodeJSONL(content);
  return {
    newRawLines: rawLines,
    newTotalLineCount: currentLineCount + totalLines,
  };
}
