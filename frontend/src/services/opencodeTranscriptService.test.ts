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
  it('parses valid lines, skips blank and malformed', () => {
    const jsonl = [line(userLine), '', '   ', '{not json', line(assistantLine)].join('\n');
    const { rawLines, totalLines } = parseOpenCodeJSONL(jsonl);
    // 3 non-empty lines (user, malformed, assistant); malformed dropped.
    expect(totalLines).toBe(3);
    expect(rawLines).toHaveLength(2);
    expect(rawLines[0]?.info.role).toBe('user');
    expect(rawLines[1]?.info.role).toBe('assistant');
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
      expect(asst.usage).toEqual({ input: 10000, output: 5000, cacheWrite: 2000, cacheRead: 3000 });
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

describe('extractOpenCodeModel', () => {
  it('returns the first non-empty modelID', () => {
    const { rawLines } = parseOpenCodeJSONL([line(userLine), line(assistantLine)].join('\n'));
    expect(extractOpenCodeModel(rawLines)).toBe('claude-sonnet-4-20250514');
  });

  it('returns undefined when no model present', () => {
    expect(extractOpenCodeModel([])).toBeUndefined();
  });
});
