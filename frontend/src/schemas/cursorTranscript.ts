// Zod schema for Cursor transcript JSONL lines (18n2).
//
// Cursor is NOT the Claude Code format. Two top-level line shapes only
// (verified against backend/internal/analytics/testdata/cursor/main.jsonl and
// the backend parser cursor_types.go / cursor_validation.go):
//
//   {role:"user"|"assistant", message:{content:[ContentBlock,…]}}
//   {type:"turn_ended", status:"success"|"error", error?}
//
// Content blocks are only `text` ({type,text}) and `tool_use` ({type,name,input})
// — `tool_use` has NO `id`, and there are NO `tool_result` blocks anywhere
// (Cursor records tool inputs only, never outputs). No per-line model / token /
// cost / timestamp fields exist. Schema is permissive (`.passthrough()`) so an
// unknown future field doesn't drop a line.

import { z } from 'zod';

const CursorTextBlockSchema = z
  .object({
    type: z.literal('text'),
    text: z.string(),
  })
  .passthrough();

const CursorToolUseBlockSchema = z
  .object({
    type: z.literal('tool_use'),
    name: z.string(),
    // Tool input is a free-form object; file tools key on `path` (not file_path).
    input: z.record(z.string(), z.unknown()).optional(),
  })
  .passthrough();

// Any block we don't model explicitly still parses (forward-compat); the
// normalizer ignores blocks it doesn't recognize.
const CursorContentBlockSchema = z.union([
  CursorTextBlockSchema,
  CursorToolUseBlockSchema,
  z.object({ type: z.string() }).passthrough(),
]);

const CursorMessageLineSchema = z
  .object({
    role: z.enum(['user', 'assistant']),
    message: z
      .object({
        content: z.array(CursorContentBlockSchema).default([]),
      })
      .passthrough(),
  })
  .passthrough();

const CursorTurnEndedLineSchema = z
  .object({
    type: z.literal('turn_ended'),
    status: z.string().optional(),
    error: z.string().optional(),
  })
  .passthrough();

export const RawCursorLineSchema = z.union([
  CursorMessageLineSchema,
  CursorTurnEndedLineSchema,
]);

export type RawCursorLine = z.infer<typeof RawCursorLineSchema>;

/**
 * Narrow an arbitrary content block to a text block, returning its typed shape
 * or null. Lets the normalizer extract `text` without a type assertion — the
 * union member is recovered by re-validating the block against the text schema.
 */
export function asCursorTextBlock(block: unknown) {
  const r = CursorTextBlockSchema.safeParse(block);
  return r.success ? r.data : null;
}

/** Narrow an arbitrary content block to a tool_use block, returning its typed
 *  shape or null. */
export function asCursorToolUseBlock(block: unknown) {
  const r = CursorToolUseBlockSchema.safeParse(block);
  return r.success ? r.data : null;
}
