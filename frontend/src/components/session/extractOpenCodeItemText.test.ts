// Spec for `extractOpenCodeItemText` (5p9j). Locks the per-kind search
// projection: what text becomes searchable for each `OpenCodeRenderItem`
// variant. Uses `.toContain` so the extractor can add surrounding context
// (separators) without breaking the contract.

import { describe, it, expect } from 'vitest';
import type { OpenCodeRenderItem } from './opencodeCategories';
import { extractOpenCodeItemText } from './extractOpenCodeItemText';

const t = 1_717_689_500_000;

describe('extractOpenCodeItemText', () => {
  it('user: returns the message text', () => {
    const item: OpenCodeRenderItem = { kind: 'user', id: 'u1', text: 'hello world', timeCreated: t };
    expect(extractOpenCodeItemText(item)).toContain('hello world');
  });

  it('assistant: joins reasoning + visible text', () => {
    const item: OpenCodeRenderItem = {
      kind: 'assistant',
      id: 'a1',
      text: 'final reply body',
      reasoning: 'internal deliberation here',
      timeCreated: t,
    };
    const text = extractOpenCodeItemText(item);
    expect(text).toContain('internal deliberation here');
    expect(text).toContain('final reply body');
  });

  it('assistant: works with no reasoning present', () => {
    const item: OpenCodeRenderItem = {
      kind: 'assistant',
      id: 'a2',
      text: 'just the body',
      timeCreated: t,
    };
    expect(extractOpenCodeItemText(item)).toContain('just the body');
  });

  it('tool: includes both input and output (decision 4)', () => {
    const item: OpenCodeRenderItem = {
      kind: 'tool',
      id: 'prt1',
      toolName: 'Bash',
      status: 'completed',
      input: 'grep -rn needle src',
      output: 'src/main.go:42: needle found',
      timeCreated: t,
    };
    const text = extractOpenCodeItemText(item);
    expect(text).toContain('grep -rn needle src');
    expect(text).toContain('src/main.go:42: needle found');
  });

  it('tool: input-only (no output yet) still searchable', () => {
    const item: OpenCodeRenderItem = {
      kind: 'tool',
      id: 'prt2',
      toolName: 'Glob',
      status: 'pending',
      input: '**/*.ts',
      timeCreated: t,
    };
    expect(extractOpenCodeItemText(item)).toContain('**/*.ts');
  });

  it('unknown: returns the stringified rawLine the row displays', () => {
    const item: OpenCodeRenderItem = {
      kind: 'unknown',
      id: 'x1',
      reason: 'unrecognized-part-type',
      unrecognizedType: 'newfangled',
      rawLine: { type: 'newfangled', payload: { ip: '192.0.2.1' } },
      timeCreated: t,
    };
    const text = extractOpenCodeItemText(item);
    expect(text).toContain('newfangled');
    expect(text).toContain('192.0.2.1');
  });

  it('unknown: passes a raw string through verbatim', () => {
    const item: OpenCodeRenderItem = {
      kind: 'unknown',
      id: 'x2',
      reason: 'malformed-line',
      unrecognizedType: 'string',
      rawLine: 'totally raw garbage line',
      timeCreated: t,
    };
    expect(extractOpenCodeItemText(item)).toBe('totally raw garbage line');
  });
});
