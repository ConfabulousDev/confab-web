// Spec for `extractCodexItemText` (CF-359). Locks the per-kind search
// projection: what text becomes searchable for each `CodexRenderItem`
// variant. Tests use `.toContain` rather than equality so the extractor
// can include extra surrounding context (file paths, separators) without
// breaking the contract.

import { describe, it, expect } from 'vitest';
import type { CodexRenderItem } from '@/types/codexRenderItem';
import { extractCodexItemText } from './extractCodexItemText';

const ts = '2026-05-13T18:00:00Z';

describe('extractCodexItemText', () => {
  it('returns the user message text', () => {
    const item: CodexRenderItem = { kind: 'user', lineId: '0', timestamp: ts, text: 'hello world' };
    expect(extractCodexItemText(item)).toContain('hello world');
  });

  it('returns the assistant message text', () => {
    const item: CodexRenderItem = {
      kind: 'assistant', lineId: '0', timestamp: ts,
      text: 'reply body', phase: 'final', model: 'gpt-5',
    };
    expect(extractCodexItemText(item)).toContain('reply body');
  });

  describe('tool_call', () => {
    it('exec_command: returns both the command and the output', () => {
      const item: CodexRenderItem = {
        kind: 'tool_call', lineId: '0', timestamp: ts,
        toolName: 'exec_command', callId: 'c1',
        rawInput: { cmd: 'cat /etc/hosts' },
        rawOutput: '127.0.0.1 localhost',
        status: 'completed',
        execMetadata: { exitCode: 0, wallTimeMs: 10 },
      };
      const text = extractCodexItemText(item);
      expect(text).toContain('cat /etc/hosts');
      expect(text).toContain('127.0.0.1 localhost');
    });

    it('exec_command pending (no output): still includes the cmd', () => {
      const item: CodexRenderItem = {
        kind: 'tool_call', lineId: '0', timestamp: ts,
        toolName: 'exec_command', callId: 'c1',
        rawInput: { cmd: 'pwd' },
        status: 'pending',
      };
      expect(extractCodexItemText(item)).toContain('pwd');
    });

    it('apply_patch: returns the diff string and every changed file path', () => {
      const item: CodexRenderItem = {
        kind: 'tool_call', lineId: '0', timestamp: ts,
        toolName: 'apply_patch', callId: 'c2',
        rawInput: '*** Begin Patch\n*** Add File: docs/readme.md\n+# Title',
        rawOutput: '{"output":"Success."}',
        structuredOutput: {
          changes: {
            '/proj/docs/readme.md': { type: 'add', content: '# Title' },
            '/proj/src/index.ts': { type: 'update' },
          },
        },
        status: 'completed',
      };
      const text = extractCodexItemText(item);
      expect(text).toContain('*** Begin Patch');
      expect(text).toContain('/proj/docs/readme.md');
      expect(text).toContain('/proj/src/index.ts');
    });

    it('web_search_call: returns every query', () => {
      const item: CodexRenderItem = {
        kind: 'tool_call', lineId: '0', timestamp: ts,
        toolName: 'web_search_call', callId: 'c3',
        rawInput: { queries: ['anthropic prompt caching', 'codex rollout format'] },
        status: 'completed',
      };
      const text = extractCodexItemText(item);
      expect(text).toContain('anthropic prompt caching');
      expect(text).toContain('codex rollout format');
    });

    it('generic: returns stringified rawInput and rawOutput', () => {
      const item: CodexRenderItem = {
        kind: 'tool_call', lineId: '0', timestamp: ts,
        toolName: 'custom_thing', callId: 'c4',
        rawInput: { secret_token: 'abc123' },
        rawOutput: 'response payload',
        status: 'completed',
      };
      const text = extractCodexItemText(item);
      expect(text).toContain('secret_token');
      expect(text).toContain('abc123');
      expect(text).toContain('response payload');
    });
  });

  it('turn_separator: returns empty string', () => {
    const item: CodexRenderItem = {
      kind: 'turn_separator', lineId: '0', timestamp: ts,
      turnIndex: 1, durationMs: 1000,
    };
    expect(extractCodexItemText(item)).toBe('');
  });

  it('reasoning_hidden: returns empty string', () => {
    const item: CodexRenderItem = { kind: 'reasoning_hidden', lineId: '0', timestamp: ts };
    expect(extractCodexItemText(item)).toBe('');
  });

  describe('compacted', () => {
    it('returns the visible label (plural)', () => {
      const item: CodexRenderItem = {
        kind: 'compacted', lineId: '0', timestamp: ts, replacementCount: 5,
      };
      const text = extractCodexItemText(item);
      // The label rendered on screen is the searchable string. Using
      // .toLowerCase().includes mirrors the hook's matching.
      expect(text.toLowerCase()).toContain('compacted');
      expect(text).toContain('5');
      expect(text.toLowerCase()).toContain('earlier');
    });

    it('returns the visible label (singular for count=1)', () => {
      const item: CodexRenderItem = {
        kind: 'compacted', lineId: '0', timestamp: ts, replacementCount: 1,
      };
      const text = extractCodexItemText(item);
      expect(text.toLowerCase()).toContain('1 earlier message');
      // Must not include the plural-only token "messages"
      expect(text.toLowerCase()).not.toContain('messages');
    });
  });

  it('unknown: returns stringified rawLine so the expanded JSON is searchable', () => {
    const item: CodexRenderItem = {
      kind: 'unknown', lineId: '0', timestamp: ts,
      rawLine: { type: 'newfangled_event', payload: { ip: '192.0.2.1' } },
    };
    const text = extractCodexItemText(item);
    expect(text).toContain('newfangled_event');
    expect(text).toContain('192.0.2.1');
  });
});
