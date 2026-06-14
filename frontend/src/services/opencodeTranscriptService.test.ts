import { describe, it, expect } from 'vitest';
import {
  parseOpenCodeJSONL,
  normalizeOpenCodeLines,
  extractOpenCodeModel,
} from './opencodeTranscriptService';
import type { RawOpenCodeLine } from '@/schemas/opencodeTranscript';

function line(obj: unknown): string {
  return JSON.stringify(obj);
}

const userLine = {
  info: { id: 'msg_user', role: 'user', time: { created: 1717689500000 } },
  parts: [{ id: 'prt_1', type: 'text', text: 'Find all Go files' }],
};

const assistantLine = {
  info: {
    id: 'msg_asst',
    role: 'assistant',
    modelID: 'claude-sonnet-4-20250514',
    providerID: 'anthropic',
    cost: 0.015,
    tokens: { input: 10000, output: 5000, cache: { read: 3000, write: 2000 } },
    time: { created: 1717689600000 },
  },
  parts: [
    { id: 'prt_2', type: 'reasoning', text: 'Let me check the files...' },
    {
      id: 'prt_3',
      type: 'tool',
      tool: 'Bash',
      state: { status: 'completed', input: { command: 'ls' }, output: 'file1\nfile2' },
    },
    { id: 'prt_4', type: 'text', text: 'I found 2 files.' },
    {
      id: 'prt_5',
      type: 'tool',
      tool: 'Read',
      state: { status: 'pending', input: { file_path: 'main.go' } },
    },
  ],
};

describe('parseOpenCodeJSONL', () => {
  it('parses valid lines, skips blank, surfaces malformed as invalid entries (CF-574)', () => {
    const jsonl = [line(userLine), '', '   ', '{not json', line(assistantLine)].join('\n');
    const { rawLines, totalLines } = parseOpenCodeJSONL(jsonl);
    // 3 non-empty lines (user, malformed, assistant); malformed kept, not dropped.
    expect(totalLines).toBe(3);
    expect(rawLines).toHaveLength(3);
    expect(rawLines[1]).toMatchObject({ __invalid: true, raw: '{not json' });
  });

  it('surfaces shape-invalid (but JSON-valid) lines as invalid entries', () => {
    const { rawLines } = parseOpenCodeJSONL(line({ not: 'a message' }));
    expect(rawLines).toHaveLength(1);
    expect(rawLines[0]).toMatchObject({ __invalid: true });
  });

  it('returns empty for empty input', () => {
    expect(parseOpenCodeJSONL('').rawLines).toHaveLength(0);
  });
});

describe('normalizeOpenCodeLines', () => {
  const { rawLines } = parseOpenCodeJSONL([line(userLine), line(assistantLine)].join('\n'));
  const items = normalizeOpenCodeLines(rawLines);

  it('emits a user item', () => {
    const user = items.find((i) => i.kind === 'user');
    expect(user).toMatchObject({ id: 'msg_user', text: 'Find all Go files' });
  });

  it('emits an assistant item with reasoning, text, model, cost, usage', () => {
    const asst = items.find((i) => i.kind === 'assistant');
    expect(asst).toMatchObject({
      id: 'msg_asst',
      text: 'I found 2 files.',
      reasoning: 'Let me check the files...',
      model: 'claude-sonnet-4-20250514',
      cost: 0.015,
    });
    if (asst?.kind === 'assistant') {
      expect(asst.usage).toEqual({ input: 10000, output: 5000, cacheWrite: 2000, cacheWrite1h: 0, cacheRead: 3000 });
    }
  });

  it('emits terminal tool items only (skips pending)', () => {
    const tools = items.filter((i) => i.kind === 'tool');
    expect(tools).toHaveLength(1);
    expect(tools[0]).toMatchObject({
      toolName: 'Bash',
      status: 'completed',
      input: 'ls',
      output: 'file1\nfile2',
    });
  });

  it('drops user messages with no text', () => {
    const empty: RawOpenCodeLine = {
      info: { role: 'user', time: { created: 1 } },
      parts: [{ type: 'step-start' }],
    };
    expect(normalizeOpenCodeLines([empty])).toHaveLength(0);
  });
});

describe('normalizeOpenCodeLines — CF-574 unknown surfacing', () => {
  it('surfaces an unrecognized message role as an unknown item', () => {
    const weird: RawOpenCodeLine = {
      info: { id: 'msg_x', role: 'orchestrator', time: { created: 5 } },
      parts: [{ type: 'text', text: 'hi' }],
    };
    const items = normalizeOpenCodeLines([weird]);
    expect(items).toHaveLength(1);
    expect(items[0]).toMatchObject({
      kind: 'unknown',
      reason: 'unrecognized message role',
      unrecognizedType: 'orchestrator',
    });
  });

  it('surfaces an unrecognized part type as an unknown item', () => {
    const asst: RawOpenCodeLine = {
      info: { id: 'msg_y', role: 'assistant', time: { created: 6 } },
      parts: [
        { type: 'text', text: 'done' },
        { id: 'prt_weird', type: 'future_part_type' },
      ],
    };
    const unknowns = normalizeOpenCodeLines([asst]).filter((i) => i.kind === 'unknown');
    expect(unknowns).toHaveLength(1);
    expect(unknowns[0]).toMatchObject({
      kind: 'unknown',
      reason: 'unrecognized part type',
      unrecognizedType: 'future_part_type',
    });
  });

  it('does NOT surface known-but-ignored part types or non-terminal tools as unknown', () => {
    const asst: RawOpenCodeLine = {
      info: { id: 'msg_z', role: 'assistant', time: { created: 7 } },
      parts: [
        { type: 'step-start' },
        { type: 'snapshot' },
        { type: 'tool', tool: 'Read', state: { status: 'pending' } },
      ],
    };
    expect(normalizeOpenCodeLines([asst]).filter((i) => i.kind === 'unknown')).toHaveLength(0);
  });

  it('surfaces a malformed line as an unknown item with the raw text', () => {
    const { rawLines } = parseOpenCodeJSONL('{not json');
    const items = normalizeOpenCodeLines(rawLines);
    expect(items).toHaveLength(1);
    expect(items[0]).toMatchObject({
      kind: 'unknown',
      reason: 'malformed line',
      rawLine: '{not json',
    });
  });

  it('gives unknown items stable ids based on stream position', () => {
    const { rawLines } = parseOpenCodeJSONL(['{bad1', '{bad2'].join('\n'));
    const items = normalizeOpenCodeLines(rawLines);
    const ids = items.map((i) => i.id);
    expect(new Set(ids).size).toBe(ids.length);
  });
});

describe('extractOpenCodeModel', () => {
  it('returns the first non-empty modelID', () => {
    const { rawLines } = parseOpenCodeJSONL([line(userLine), line(assistantLine)].join('\n'));
    expect(extractOpenCodeModel(rawLines)).toBe('claude-sonnet-4-20250514');
  });

  it('returns undefined when no model present', () => {
    expect(extractOpenCodeModel([])).toBeUndefined();
  });
});
