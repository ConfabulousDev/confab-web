// CF-360 spec tests for the per-tool copy-text composition.
//
// Locks the format decisions from the CF-360 interview:
//   - exec_command: `$ <cmd>\n<output>`
//   - apply_patch:  raw patch input only (no output, no summary)
//   - web_search:   queries joined by newlines; undefined if no queries
//   - generic:      stringified rawInput + rawOutput, joined by blank line

import { describe, it, expect } from 'vitest';
import { buildToolCallCopyText } from './codexToolCallHelpers';
import type { CodexToolCallItem } from '@/types/codexRenderItem';

const baseTs = '2026-05-13T18:00:00Z';

function tc(overrides: Partial<CodexToolCallItem>): CodexToolCallItem {
  return {
    kind: 'tool_call',
    lineId: '0',
    timestamp: baseTs,
    toolName: 'exec_command',
    callId: 'c1',
    rawInput: { cmd: 'pwd' },
    status: 'completed',
    ...overrides,
  };
}

describe('buildToolCallCopyText', () => {
  describe('exec_command', () => {
    it('joins `$ <cmd>` and output with a newline', () => {
      const item = tc({
        toolName: 'exec_command',
        rawInput: { cmd: 'pwd' },
        rawOutput: '/tmp\n',
      });
      expect(buildToolCallCopyText(item)).toBe('$ pwd\n/tmp\n');
    });

    it('returns just `$ <cmd>` when there is no output', () => {
      const item = tc({
        toolName: 'exec_command',
        rawInput: { cmd: 'pwd' },
        rawOutput: undefined,
      });
      expect(buildToolCallCopyText(item)).toBe('$ pwd');
    });

    it('returns just the output when there is no cmd', () => {
      const item = tc({
        toolName: 'exec_command',
        rawInput: {},
        rawOutput: 'standalone output',
      });
      expect(buildToolCallCopyText(item)).toBe('standalone output');
    });

    it('returns undefined when both cmd and output are absent', () => {
      const item = tc({
        toolName: 'exec_command',
        rawInput: {},
        rawOutput: undefined,
      });
      expect(buildToolCallCopyText(item)).toBeUndefined();
    });
  });

  describe('apply_patch', () => {
    it('returns the raw patch input string verbatim', () => {
      const patch = '*** Begin Patch\n*** Add File: a.ts\n+x\n*** End Patch';
      const item = tc({
        toolName: 'apply_patch',
        rawInput: patch,
        rawOutput: '{"output":"Success."}',
      });
      expect(buildToolCallCopyText(item)).toBe(patch);
    });

    it('does NOT include rawOutput in the copied text', () => {
      const patch = '*** Begin Patch\n*** End Patch';
      const item = tc({
        toolName: 'apply_patch',
        rawInput: patch,
        rawOutput: 'should not appear',
      });
      const out = buildToolCallCopyText(item);
      expect(out).toBe(patch);
      expect(out).not.toContain('should not appear');
    });

    it('does NOT include structuredOutput JSON', () => {
      const patch = '*** Begin Patch\n*** End Patch';
      const item = tc({
        toolName: 'apply_patch',
        rawInput: patch,
        structuredOutput: { success: true, changes: { '/a.ts': { type: 'add' } } },
      });
      const out = buildToolCallCopyText(item);
      expect(out).toBe(patch);
    });

    it('returns undefined when rawInput is non-string or empty', () => {
      expect(
        buildToolCallCopyText(tc({ toolName: 'apply_patch', rawInput: undefined })),
      ).toBeUndefined();
      expect(
        buildToolCallCopyText(tc({ toolName: 'apply_patch', rawInput: '' })),
      ).toBeUndefined();
      expect(
        buildToolCallCopyText(tc({ toolName: 'apply_patch', rawInput: { not: 'a string' } })),
      ).toBeUndefined();
    });
  });

  describe('web_search_call', () => {
    it('joins queries with newlines', () => {
      const item = tc({
        toolName: 'web_search_call',
        rawInput: { queries: ['a', 'b c', 'd'] },
      });
      expect(buildToolCallCopyText(item)).toBe('a\nb c\nd');
    });

    it('falls back to the singular `query` field when `queries` is absent', () => {
      const item = tc({
        toolName: 'web_search_call',
        rawInput: { query: 'lonely query' },
      });
      expect(buildToolCallCopyText(item)).toBe('lonely query');
    });

    it('returns undefined when there are no queries to copy', () => {
      const item = tc({ toolName: 'web_search_call', rawInput: {} });
      expect(buildToolCallCopyText(item)).toBeUndefined();
    });
  });

  describe('generic / unknown tool', () => {
    it('joins JSON-stringified input and string output with a blank line', () => {
      const item = tc({
        toolName: 'future_tool',
        rawInput: { k: 'v', n: 1 },
        rawOutput: 'plain result',
      });
      const out = buildToolCallCopyText(item) ?? '';
      expect(out).toContain('"k": "v"');
      expect(out).toContain('"n": 1');
      expect(out).toContain('plain result');
      expect(out.indexOf('plain result')).toBeGreaterThan(out.indexOf('"k"'));
    });

    it('passes through a string rawInput unchanged (no double-stringify)', () => {
      const item = tc({
        toolName: 'future_tool_str',
        rawInput: '{"k":"v"}',
        rawOutput: undefined,
      });
      expect(buildToolCallCopyText(item)).toBe('{"k":"v"}');
    });

    it('returns undefined when input is null/undefined and output is missing', () => {
      expect(
        buildToolCallCopyText(
          tc({ toolName: 'future_tool', rawInput: null, rawOutput: undefined }),
        ),
      ).toBeUndefined();
      expect(
        buildToolCallCopyText(
          tc({ toolName: 'future_tool', rawInput: undefined, rawOutput: undefined }),
        ),
      ).toBeUndefined();
    });
  });
});
