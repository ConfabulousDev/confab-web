// CF-360 spec tests for `skipNavKey`.
//
// Locks the granularity decisions from the CF-360 interview:
//   - user → "user"
//   - assistant → "assistant:" + phase  (commentary vs final split)
//   - tool_call → "tool_call:" + toolName  (exec/patch/web_search split)
//   - turn_separator / reasoning_hidden / compacted / unknown → null

import { describe, it, expect } from 'vitest';
import { skipNavKey } from './codexVirtualItems';
import type { CodexRenderItem } from '@/types/codexRenderItem';

const ts = '2026-05-13T18:00:00Z';

describe('skipNavKey', () => {
  it('returns "user" for a user item', () => {
    const item: CodexRenderItem = { kind: 'user', lineId: '0', timestamp: ts, text: 'hi' };
    expect(skipNavKey(item)).toBe('user');
  });

  it('returns "assistant:commentary" for a commentary assistant', () => {
    const item: CodexRenderItem = {
      kind: 'assistant',
      lineId: '0',
      timestamp: ts,
      text: 'thinking…',
      phase: 'commentary',
      model: 'gpt-5',
    };
    expect(skipNavKey(item)).toBe('assistant:commentary');
  });

  it('returns "assistant:final" for a final-phase assistant', () => {
    const item: CodexRenderItem = {
      kind: 'assistant',
      lineId: '0',
      timestamp: ts,
      text: 'answer',
      phase: 'final',
      model: 'gpt-5',
    };
    expect(skipNavKey(item)).toBe('assistant:final');
  });

  it('returns "tool_call:exec_command" for exec_command', () => {
    const item: CodexRenderItem = {
      kind: 'tool_call',
      lineId: '0',
      timestamp: ts,
      toolName: 'exec_command',
      callId: 'c1',
      rawInput: { cmd: 'pwd' },
      status: 'completed',
    };
    expect(skipNavKey(item)).toBe('tool_call:exec_command');
  });

  it('returns "tool_call:apply_patch" for apply_patch', () => {
    const item: CodexRenderItem = {
      kind: 'tool_call',
      lineId: '0',
      timestamp: ts,
      toolName: 'apply_patch',
      callId: 'c1',
      rawInput: '*** End Patch',
      status: 'completed',
    };
    expect(skipNavKey(item)).toBe('tool_call:apply_patch');
  });

  it('returns "tool_call:web_search_call" for web_search_call', () => {
    const item: CodexRenderItem = {
      kind: 'tool_call',
      lineId: '0',
      timestamp: ts,
      toolName: 'web_search_call',
      callId: 'c1',
      rawInput: {},
      status: 'completed',
    };
    expect(skipNavKey(item)).toBe('tool_call:web_search_call');
  });

  it('returns "tool_call:<future>" for unknown tools (pass-through)', () => {
    const item: CodexRenderItem = {
      kind: 'tool_call',
      lineId: '0',
      timestamp: ts,
      toolName: 'future_tool',
      callId: 'c1',
      rawInput: {},
      status: 'completed',
    };
    expect(skipNavKey(item)).toBe('tool_call:future_tool');
  });

  it('returns null for turn_separator (no skip nav)', () => {
    const item: CodexRenderItem = {
      kind: 'turn_separator',
      lineId: '0',
      timestamp: ts,
      turnIndex: 1,
      durationMs: 1000,
    };
    expect(skipNavKey(item)).toBeNull();
  });

  it('returns null for reasoning_hidden', () => {
    expect(skipNavKey({ kind: 'reasoning_hidden', lineId: '0', timestamp: ts })).toBeNull();
  });

  it('returns null for compacted', () => {
    expect(
      skipNavKey({ kind: 'compacted', lineId: '0', timestamp: ts, replacementCount: 1 }),
    ).toBeNull();
  });

  it('returns null for unknown', () => {
    expect(
      skipNavKey({ kind: 'unknown', lineId: '0', timestamp: ts, rawLine: {} }),
    ).toBeNull();
  });
});
