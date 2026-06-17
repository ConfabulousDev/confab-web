// 18n2: cursorTranscriptService parse + normalize, grounded in the real Cursor
// wire fixture shape (backend/internal/analytics/testdata/cursor/main.jsonl):
//   {role:"user"|"assistant", message:{content:[{type:"text",text}|{type:"tool_use",name,input}]}}
//   {type:"turn_ended", status:"success"|"error", error?}
// Cursor records tool INPUTS only (no id, no tool_result), file tools key on `path`.

import { describe, expect, it } from 'vitest';
import {
  parseCursorJSONL,
  normalizeCursorLines,
  extractCursorModel,
} from './cursorTranscriptService';

function line(obj: unknown): string {
  return JSON.stringify(obj);
}

const userLine = {
  role: 'user',
  message: { content: [{ type: 'text', text: 'Add input validation to the session handler.' }] },
};

const assistantLine = {
  role: 'assistant',
  message: {
    content: [
      { type: 'text', text: 'Reading the handler and searching for validation helpers.' },
      { type: 'tool_use', name: 'Read', input: { path: 'internal/api/session_handler.go' } },
      { type: 'tool_use', name: 'Grep', input: { pattern: 'func validate', path: 'internal/api' } },
    ],
  },
};

const turnEndedOk = { type: 'turn_ended', status: 'success' };
const turnEndedErr = { type: 'turn_ended', status: 'error', error: 'usage limit' };

function rawOf(...objs: unknown[]) {
  return parseCursorJSONL(objs.map(line).join('\n')).rawLines;
}

describe('parseCursorJSONL', () => {
  it('parses both line shapes and counts non-empty lines', () => {
    const { rawLines, totalLines } = parseCursorJSONL(
      [userLine, assistantLine, turnEndedOk].map(line).join('\n'),
    );
    expect(totalLines).toBe(3);
    expect(rawLines).toHaveLength(3);
  });

  it('skips blank lines but counts only the non-empty ones', () => {
    const jsonl = `${line(userLine)}\n\n${line(turnEndedOk)}\n`;
    const { totalLines } = parseCursorJSONL(jsonl);
    expect(totalLines).toBe(2);
  });

  it('keeps a malformed line as an invalid entry instead of dropping it', () => {
    const { rawLines, totalLines } = parseCursorJSONL(`${line(userLine)}\n{not json`);
    expect(totalLines).toBe(2);
    expect(rawLines).toHaveLength(2);
  });
});

describe('normalizeCursorLines', () => {
  it('normalizes a user line into a single user render item', () => {
    const items = normalizeCursorLines(rawOf(userLine));
    expect(items).toHaveLength(1);
    const first = items[0];
    expect(first?.kind).toBe('user');
    if (first?.kind === 'user') {
      expect(first.text).toContain('input validation');
    }
  });

  it('normalizes an assistant line into one assistant item plus one tool item per tool_use', () => {
    const items = normalizeCursorLines(rawOf(assistantLine));
    const kinds = items.map((i) => i.kind);
    expect(kinds).toEqual(['assistant', 'tool', 'tool']);
    const tool = items[1];
    expect(tool?.kind).toBe('tool');
    if (tool?.kind === 'tool') {
      expect(tool.toolName).toBe('Read');
      // File tools surface their target via `path` (NOT file_path).
      expect(tool.input).toContain('internal/api/session_handler.go');
    }
  });

  it('excludes turn_ended rows from the render stream (decision 3)', () => {
    const items = normalizeCursorLines(rawOf(turnEndedOk, turnEndedErr));
    expect(items).toHaveLength(0);
  });

  it('assigns synthetic line-based ids (no stable id in the wire format)', () => {
    const items = normalizeCursorLines(rawOf(userLine, assistantLine));
    const ids = items.map((i) => i.id);
    expect(new Set(ids).size).toBe(ids.length); // all distinct
    expect(ids.every((id) => typeof id === 'string' && id.length > 0)).toBe(true);
  });

  it('tool items carry no output/result (Cursor records inputs only)', () => {
    const items = normalizeCursorLines(rawOf(assistantLine));
    const tools = items.filter((i) => i.kind === 'tool');
    expect(tools.length).toBeGreaterThan(0);
    for (const t of tools) {
      expect(t).not.toHaveProperty('output');
    }
  });
});

describe('extractCursorModel', () => {
  it('returns undefined — Cursor lines carry no model field', () => {
    expect(extractCursorModel()).toBeUndefined();
  });
});
