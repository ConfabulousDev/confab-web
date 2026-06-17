// Service for fetching and parsing Cursor transcripts (18n2).
//
// Mirrors opencodeTranscriptService: the backend sync/file endpoint streams raw
// JSONL bytes regardless of provider; the difference is entirely in the parse +
// normalize layer. Cursor lines are NOT the Claude Code format — see
// schemas/cursorTranscript.ts for the two line shapes.
//
// Wire facts (locked by backend PR #350 / fixture testdata/cursor/main.jsonl):
//   - Two line shapes: {role, message:{content[]}} and {type:"turn_ended", …}.
//   - Content blocks: `text` and `tool_use` ({name, input}); tool_use has NO id.
//   - No `tool_result` blocks — tool INPUTS only, never outputs.
//   - No per-line model / token / cost / timestamp — so extractModel is always
//     undefined, computeMeta falls back to firstSeen/lastSyncAt, and cost UI
//     stays hidden (empty pricing).

import {
  RawCursorLineSchema,
  asCursorTextBlock,
  asCursorToolUseBlock,
  type RawCursorLine,
} from '@/schemas/cursorTranscript';
import type { CursorRenderItem, CursorUserSection } from '@/components/session/cursorCategories';
import { syncFilesAPI } from './api';

// ============================================================================
// Raw entry types
// ============================================================================

/** A line that failed JSON.parse or the shape schema. Kept in the stream — not
 *  dropped — so line offsets stay aligned with the file for incremental fetch.
 *  `raw` is the parsed object (shape failure) or the original string (JSON
 *  failure). Invalid lines produce no render item (Cursor MVP has no unknown
 *  row); they only hold the line slot. */
export interface CursorInvalidLine {
  __invalid: true;
  raw: unknown;
}

export type CursorRawEntry = RawCursorLine | CursorInvalidLine;

function isInvalidEntry(entry: CursorRawEntry): entry is CursorInvalidLine {
  return typeof entry === 'object' && entry !== null && '__invalid' in entry;
}

function isTurnEnded(entry: RawCursorLine): entry is Extract<RawCursorLine, { type: 'turn_ended' }> {
  return 'type' in entry && entry.type === 'turn_ended';
}

// ============================================================================
// JSONL parsing
// ============================================================================

export interface CursorParseResult {
  rawLines: CursorRawEntry[];
  /** Count of non-empty lines (including those that failed to parse), so the
   *  line-offset incremental fetch stays in sync with the file. */
  totalLines: number;
}

/** Parse a Cursor transcript JSONL string into stream entries. Empty lines are
 *  skipped; malformed lines (bad JSON or bad shape) are kept as
 *  `CursorInvalidLine` entries so line offsets stay aligned. */
export function parseCursorJSONL(jsonl: string): CursorParseResult {
  const lines = jsonl.split('\n').filter((line) => line.trim().length > 0);
  const rawLines: CursorRawEntry[] = [];
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
    const result = RawCursorLineSchema.safeParse(parsed);
    if (result.success) {
      rawLines.push(result.data);
    } else {
      errorCount++;
      rawLines.push({ __invalid: true, raw: parsed });
    }
  }

  if (errorCount > 0) {
    console.warn(`Cursor transcript: skipped ${errorCount} unparseable line(s)`);
  }

  return { rawLines, totalLines: lines.length };
}

// ============================================================================
// Normalization
// ============================================================================

// One-line summary of a tool call's input for the row header. File tools key on
// `path` (NOT file_path); other tools have their own primary field.
const TOOL_INPUT_KEYS = [
  'path',
  'command',
  'glob_pattern',
  'pattern',
  'query',
  'search_term',
  'title',
  'description',
] as const;

function toolInputSummary(input: Record<string, unknown> | undefined): string {
  if (!input) return '';
  for (const key of TOOL_INPUT_KEYS) {
    const v = input[key];
    if (typeof v === 'string' && v.length > 0) return v;
  }
  return '';
}

// Cursor's on-disk JSONL appends a bare `[REDACTED]` to nearly every assistant
// turn (parent wkkd) — either as the entire text block on tool-only turns or as
// a trailing suffix after the narrative. It is provider scaffolding noise, not a
// counted secret, so it is stripped before display, search, and recap.
//
// Ground truth (real ~/.cursor transcripts, fa3h): the bare token is ALWAYS
// either the whole trimmed block or a trailing suffix — never embedded
// mid-sentence. Confab CLI `[REDACTED:TYPE]` markers are a DIFFERENT contract
// (a real redacted secret) and must stay visible — we match only the bare token
// with no colon/type.
const TRAILING_BARE_REDACTED = /\s*\[REDACTED\]\s*$/;

/** Strip Cursor's native bare `[REDACTED]` placeholder from assistant text.
 *  Removes a trailing bare `[REDACTED]` (with any surrounding whitespace) and
 *  returns `""` when the block's entire content was the placeholder, so the
 *  caller can omit an empty assistant row. Never touches Confab CLI
 *  `[REDACTED:TYPE]` markers. */
export function cleanCursorAssistantText(raw: string): string {
  const cleaned = raw.replace(TRAILING_BARE_REDACTED, '');
  return cleaned.trim().length === 0 ? '' : cleaned;
}

// nfbe: Cursor wraps every user `text` block in an envelope. The human prompt
// lives in `<user_query>…</user_query>`; injected context (rules, attached
// files, manually attached skills, …) arrives as sibling top-level tagged
// blocks. The user row must show ONLY the prompt — the tags must never render
// literally. Mirrors the backend parseCursorUserPrompt (the backend discards
// the sections; the frontend keeps them for the collapsible-context UI, 0rcv).
//
// `[s]` (dotAll) lets the body span newlines (queries are multi-line). The
// matcher is anchored on each tag name so only well-formed (closed) blocks are
// recognized; an unclosed tag is left in the fallback raw text, never dropped.
const USER_QUERY_BLOCK = /<user_query>([\s\S]*?)<\/user_query>/g;
const TAGGED_BLOCK = /<([a-z_][a-z0-9_]*)>([\s\S]*?)<\/\1>/g;

/** Humanize an envelope tag name into a section heading
 *  (`manually_attached_skills` → `Manually attached skills`). */
function humanizeTag(tag: string): string {
  const spaced = tag.replace(/_/g, ' ').trim();
  return spaced.charAt(0).toUpperCase() + spaced.slice(1);
}

export interface ParsedCursorUserText {
  /** The human prompt: concatenated, trimmed `<user_query>` content. Falls back
   *  to the raw text (trimmed) when no well-formed `<user_query>` tag exists. */
  prompt: string;
  /** Every other recognized top-level tagged block (injected context), in order.
   *  Empty when the input was a bare prompt or only `<user_query>` blocks. */
  sections: CursorUserSection[];
}

/** Split a Cursor user `text` block into its human prompt and the injected
 *  context sections. Never drops content: with no well-formed `<user_query>`
 *  tag the whole raw text becomes the prompt. */
export function parseCursorUserText(raw: string): ParsedCursorUserText {
  const queries: string[] = [];
  for (const m of raw.matchAll(USER_QUERY_BLOCK)) {
    queries.push((m[1] ?? '').trim());
  }

  if (queries.length === 0) {
    // No envelope (plain text or an unclosed tag) — keep everything as the prompt.
    return { prompt: raw.trim(), sections: [] };
  }

  const sections: CursorUserSection[] = [];
  for (const m of raw.matchAll(TAGGED_BLOCK)) {
    const tag = m[1] ?? '';
    if (tag === '' || tag === 'user_query') continue; // user_query is the prompt, captured above
    sections.push({ tag, label: humanizeTag(tag), content: (m[2] ?? '').trim() });
  }

  return { prompt: queries.join('\n').trim(), sections };
}

/** Transform accumulated raw Cursor entries into the render-item stream. Pure +
 *  synchronous; safe inside `useMemo`. Cursor wire lines carry no stable id, so
 *  render-item ids are synthetic and line-derived: `${lineIndex}` for
 *  user/assistant rows and `${lineIndex}-${blockIndex}` for tool rows. These are
 *  stable across re-normalize because the append-only raw stream never reorders.
 *  turn_ended rows are dropped (markers, no message content). */
export function normalizeCursorLines(rawLines: CursorRawEntry[]): CursorRenderItem[] {
  const items: CursorRenderItem[] = [];

  rawLines.forEach((entry, lineIndex) => {
    if (isInvalidEntry(entry)) return; // hold the slot only; no render item
    if (isTurnEnded(entry)) return; // marker; excluded from the render stream

    const { role, message } = entry;
    const blocks = message.content;

    // Join the line's narrative text across all text blocks.
    const text = blocks
      .map(asCursorTextBlock)
      .filter((b): b is NonNullable<typeof b> => b !== null && b.text.length > 0)
      .map((b) => b.text)
      .join('\n');

    if (role === 'user') {
      // Unwrap the <user_query> envelope: the row shows only the human prompt;
      // injected-context sections ride along for the collapsible-context UI
      // (0rcv). An empty prompt (e.g. an empty/whitespace query) emits no row.
      const { prompt, sections } = parseCursorUserText(text);
      if (prompt.length > 0) {
        items.push({
          kind: 'user',
          id: `${lineIndex}`,
          text: prompt,
          ...(sections.length > 0 ? { sections } : {}),
        });
      }
      return;
    }

    // assistant: emit one assistant item (narrative) plus one tool item per
    // tool_use block, in wire order. Strip Cursor's native bare `[REDACTED]`
    // first; when nothing narrative remains, omit the assistant row entirely so
    // a tool-only turn shows only its tool rows (fa3h).
    const assistantText = cleanCursorAssistantText(text);
    if (assistantText.length > 0) {
      items.push({ kind: 'assistant', id: `${lineIndex}`, text: assistantText });
    }

    blocks.forEach((block, blockIndex) => {
      const tool = asCursorToolUseBlock(block);
      if (!tool) return;
      items.push({
        kind: 'tool',
        id: `${lineIndex}-${blockIndex}`,
        toolName: tool.name,
        input: toolInputSummary(tool.input),
      });
    });
  });

  return items;
}

/** Cursor lines carry no model field — always undefined. Present to satisfy the
 *  ProviderAdapter contract; per-session model (when it lands) arrives via sync
 *  metadata → analytics, not the transcript (see backend zsr6). */
export function extractCursorModel(): string | undefined {
  return undefined;
}

// ============================================================================
// Fetch + cache
// ============================================================================

interface CacheEntry {
  rawLines: CursorRawEntry[];
  totalLines: number;
}

const cursorCache = new Map<string, CacheEntry>();

async function fetchWithCache(
  sessionId: string,
  fileName: string,
  skipCache?: boolean,
): Promise<CacheEntry> {
  const cacheKey = `${sessionId}-${fileName}`;
  if (!skipCache) {
    const cached = cursorCache.get(cacheKey);
    if (cached) return cached;
  }
  const content = await syncFilesAPI.getContent(sessionId, fileName);
  const { rawLines, totalLines } = parseCursorJSONL(content);
  const entry: CacheEntry = { rawLines, totalLines };
  cursorCache.set(cacheKey, entry);
  return entry;
}

export interface ParsedCursorTranscript {
  sessionId: string;
  items: CursorRenderItem[];
  rawLines: CursorRawEntry[];
  totalLines: number;
}

/** Fetch + parse the full Cursor transcript for a session. */
export async function fetchParsedCursorTranscript(
  sessionId: string,
  fileName: string,
  skipCache?: boolean,
): Promise<ParsedCursorTranscript> {
  const entry = await fetchWithCache(sessionId, fileName, skipCache);
  return {
    sessionId,
    items: normalizeCursorLines(entry.rawLines),
    rawLines: entry.rawLines,
    totalLines: entry.totalLines,
  };
}

/** Fetch Cursor lines after `currentLineCount` (incremental poll). The backend
 *  serves only lines past `line_offset`; callers append the returned raw lines
 *  and re-derive items via `useMemo`. */
export async function fetchNewCursorLines(
  sessionId: string,
  fileName: string,
  currentLineCount: number,
): Promise<{ newRawLines: CursorRawEntry[]; newTotalLineCount: number }> {
  const content = await syncFilesAPI.getContent(sessionId, fileName, currentLineCount);
  if (!content.trim()) {
    return { newRawLines: [], newTotalLineCount: currentLineCount };
  }
  const { rawLines, totalLines } = parseCursorJSONL(content);
  return {
    newRawLines: rawLines,
    newTotalLineCount: currentLineCount + totalLines,
  };
}
