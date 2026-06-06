// Zod schema for OpenCode transcript JSONL lines.
//
// Each line is one complete message mirroring OpenCode's HTTP API response
// shape: `{ info: Message, parts: Part[] }` (see backend
// internal/analytics/opencode_types.go and backend/docs/opencode-sqlite-format.md).
// The backend already validates on ingest; this schema is the defensive
// frontend parse layer (mirrors schemas/codexTranscript.ts). It is permissive
// (`.passthrough()`) so unknown future fields don't drop a line.

import { z } from 'zod';

const OpenCodeCacheSchema = z
  .object({
    read: z.number().optional(),
    write: z.number().optional(),
  })
  .passthrough();

const OpenCodeTokensSchema = z
  .object({
    input: z.number().optional(),
    output: z.number().optional(),
    reasoning: z.number().optional(),
    cache: OpenCodeCacheSchema.optional(),
  })
  .passthrough();

const OpenCodeTimeSchema = z
  .object({
    created: z.number().optional(),
    completed: z.number().optional(),
  })
  .passthrough();

const OpenCodeToolStateSchema = z
  .object({
    status: z.string().optional(),
    input: z.record(z.string(), z.unknown()).optional(),
    output: z.string().optional(),
    error: z.string().optional(),
    title: z.string().optional(),
  })
  .passthrough();

export const OpenCodePartSchema = z
  .object({
    id: z.string().optional(),
    type: z.string(),
    text: z.string().optional(),
    tool: z.string().optional(),
    callID: z.string().optional(),
    state: OpenCodeToolStateSchema.optional(),
    auto: z.boolean().optional(),
    name: z.string().optional(),
  })
  .passthrough();

export const OpenCodeInfoSchema = z
  .object({
    id: z.string().optional(),
    sessionID: z.string().optional(),
    role: z.string(),
    modelID: z.string().optional(),
    providerID: z.string().optional(),
    cost: z.number().optional(),
    tokens: OpenCodeTokensSchema.optional(),
    time: OpenCodeTimeSchema.optional(),
  })
  .passthrough();

export const RawOpenCodeLineSchema = z.object({
  info: OpenCodeInfoSchema,
  parts: z.array(OpenCodePartSchema).default([]),
});

export type RawOpenCodeLine = z.infer<typeof RawOpenCodeLineSchema>;
export type OpenCodePart = z.infer<typeof OpenCodePartSchema>;
export type OpenCodeToolState = z.infer<typeof OpenCodeToolStateSchema>;
