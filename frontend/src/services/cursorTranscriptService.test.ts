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
  cleanCursorAssistantText,
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

// fa3h: Cursor's on-disk JSONL appends a bare `[REDACTED]` to nearly every
// assistant turn — either as a trailing suffix after narrative or as the
// entire text block on tool-only turns. Strip it during normalize. Never touch
// Confab CLI `[REDACTED:TYPE]` markers (different contract — stay visible).
describe('cleanCursorAssistantText (fa3h)', () => {
  it('strips a trailing "\\n\\n[REDACTED]" suffix and preserves the narrative', () => {
    expect(cleanCursorAssistantText('Checking the repo for open alerts.\n\n[REDACTED]')).toBe(
      'Checking the repo for open alerts.',
    );
  });

  it('strips a trailing single-newline "\\n[REDACTED]" suffix', () => {
    expect(cleanCursorAssistantText('Fetching details.\n[REDACTED]')).toBe('Fetching details.');
  });

  it('returns "" for a block whose entire content is "[REDACTED]"', () => {
    expect(cleanCursorAssistantText('[REDACTED]')).toBe('');
  });

  it('returns "" for whitespace + "[REDACTED]" only', () => {
    expect(cleanCursorAssistantText('  \n[REDACTED]\n  ')).toBe('');
  });

  it('handles carriage returns before the placeholder', () => {
    expect(cleanCursorAssistantText('Doing work.\r\n\r\n[REDACTED]')).toBe('Doing work.');
  });

  it('leaves text with no placeholder untouched', () => {
    expect(cleanCursorAssistantText('Just narrative, no redaction.')).toBe(
      'Just narrative, no redaction.',
    );
  });

  it('NEVER strips a Confab CLI [REDACTED:TYPE] marker (different contract)', () => {
    expect(cleanCursorAssistantText('See [REDACTED:GITHUB_TOKEN] in the env.')).toBe(
      'See [REDACTED:GITHUB_TOKEN] in the env.',
    );
    // even when trailing
    expect(cleanCursorAssistantText('Token is [REDACTED:GITHUB_TOKEN]')).toBe(
      'Token is [REDACTED:GITHUB_TOKEN]',
    );
  });
});

describe('normalizeCursorLines [REDACTED] handling (fa3h)', () => {
  const narrativePlusRedactedPlusTool = {
    role: 'assistant',
    message: {
      content: [
        { type: 'text', text: 'Checking the repo for open alerts.\n\n[REDACTED]' },
        { type: 'tool_use', name: 'Shell', input: { command: 'gh api alerts' } },
      ],
    },
  };

  const redactedOnlyPlusTool = {
    role: 'assistant',
    message: {
      content: [
        { type: 'text', text: '[REDACTED]' },
        { type: 'tool_use', name: 'Shell', input: { command: 'gh api alerts' } },
      ],
    },
  };

  it('strips the trailing [REDACTED] but keeps the assistant narrative and the tool row', () => {
    const items = normalizeCursorLines(rawOf(narrativePlusRedactedPlusTool));
    expect(items.map((i) => i.kind)).toEqual(['assistant', 'tool']);
    const assistant = items[0];
    if (assistant?.kind === 'assistant') {
      expect(assistant.text).toBe('Checking the repo for open alerts.');
      expect(assistant.text).not.toContain('[REDACTED]');
    }
  });

  it('omits the assistant item entirely on a [REDACTED]-only line but keeps the tool row', () => {
    const items = normalizeCursorLines(rawOf(redactedOnlyPlusTool));
    expect(items.map((i) => i.kind)).toEqual(['tool']);
  });

  it('preserves a Confab CLI [REDACTED:TYPE] marker in normalized assistant text', () => {
    const withTypedMarker = {
      role: 'assistant',
      message: {
        content: [{ type: 'text', text: 'Using [REDACTED:GITHUB_TOKEN] for auth.' }],
      },
    };
    const items = normalizeCursorLines(rawOf(withTypedMarker));
    const assistant = items[0];
    expect(assistant?.kind).toBe('assistant');
    if (assistant?.kind === 'assistant') {
      expect(assistant.text).toContain('[REDACTED:GITHUB_TOKEN]');
    }
  });
});

describe('extractCursorModel', () => {
  it('returns undefined — Cursor lines carry no model field', () => {
    expect(extractCursorModel()).toBeUndefined();
  });
});
