// CF-360 spec tests for the per-tool copy-text composition.
//
// Locks the format decisions from the CF-360 interview:
//   - exec_command: `$ <cmd>\n<output>`
//   - apply_patch:  raw patch input only (no output, no summary)
//   - web_search:   queries joined by newlines; undefined if no queries
//   - generic:      stringified rawInput + rawOutput, joined by blank line
//
// CF-368 added `readPlanSummary` for the `update_plan` body — tested below.

import { describe, it, expect } from 'vitest';
import { buildToolCallCopyText, readPlanSummary } from './codexToolCallHelpers';
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

  // CF-368: update_plan copies the rendered summary (Now: <step> · N/M complete),
  // never the raw JSON plan. The body renders the same summary.
  describe('update_plan', () => {
    it('returns the active-step summary when one step is in_progress', () => {
      const item = tc({
        toolName: 'update_plan',
        rawInput: {
          plan: [
            { step: 'Step one', status: 'completed' },
            { step: 'Step two', status: 'in_progress' },
            { step: 'Step three', status: 'pending' },
          ],
        },
      });
      expect(buildToolCallCopyText(item)).toBe('Now: Step two · 1/3 complete');
    });

    it('returns "Plan complete" when every step is completed', () => {
      const item = tc({
        toolName: 'update_plan',
        rawInput: {
          plan: [
            { step: 'a', status: 'completed' },
            { step: 'b', status: 'completed' },
          ],
        },
      });
      expect(buildToolCallCopyText(item)).toBe('Plan complete · 2/2 complete');
    });

    it('returns "Plan registered" when every step is pending', () => {
      const item = tc({
        toolName: 'update_plan',
        rawInput: {
          plan: [
            { step: 'a', status: 'pending' },
            { step: 'b', status: 'pending' },
          ],
        },
      });
      expect(buildToolCallCopyText(item)).toBe('Plan registered · 0/2 complete');
    });

    it('returns "Empty plan" when the plan is empty', () => {
      const item = tc({ toolName: 'update_plan', rawInput: { plan: [] } });
      expect(buildToolCallCopyText(item)).toBe('Empty plan');
    });

    it('returns undefined when the input is not a usable plan object', () => {
      expect(
        buildToolCallCopyText(tc({ toolName: 'update_plan', rawInput: null })),
      ).toBeUndefined();
      expect(
        buildToolCallCopyText(tc({ toolName: 'update_plan', rawInput: undefined })),
      ).toBeUndefined();
    });
  });
});

// CF-368: pure helper that classifies an update_plan payload into one of the
// five summary buckets. Drives both the body renderer and the copy text.
describe('readPlanSummary', () => {
  it('classifies an empty plan as bucket=empty with zero totals', () => {
    expect(readPlanSummary({ plan: [] })).toEqual({
      bucket: 'empty',
      completedCount: 0,
      totalCount: 0,
    });
  });

  it('classifies a missing plan field as bucket=empty (defensive)', () => {
    expect(readPlanSummary({})).toEqual({
      bucket: 'empty',
      completedCount: 0,
      totalCount: 0,
    });
    expect(readPlanSummary(null)).toEqual({
      bucket: 'empty',
      completedCount: 0,
      totalCount: 0,
    });
    expect(readPlanSummary('not an object')).toEqual({
      bucket: 'empty',
      completedCount: 0,
      totalCount: 0,
    });
  });

  it('classifies all-completed as bucket=complete', () => {
    expect(
      readPlanSummary({
        plan: [
          { step: 'a', status: 'completed' },
          { step: 'b', status: 'completed' },
          { step: 'c', status: 'completed' },
        ],
      }),
    ).toEqual({
      bucket: 'complete',
      completedCount: 3,
      totalCount: 3,
    });
  });

  it('classifies all-pending as bucket=pending', () => {
    expect(
      readPlanSummary({
        plan: [
          { step: 'a', status: 'pending' },
          { step: 'b', status: 'pending' },
        ],
      }),
    ).toEqual({
      bucket: 'pending',
      completedCount: 0,
      totalCount: 2,
    });
  });

  it('classifies a mix-with-active as bucket=in_progress with the first active step', () => {
    const result = readPlanSummary({
      plan: [
        { step: 'one', status: 'completed' },
        { step: 'two', status: 'in_progress' },
        { step: 'three', status: 'pending' },
      ],
    });
    expect(result.bucket).toBe('in_progress');
    expect(result.activeStep).toBe('two');
    expect(result.completedCount).toBe(1);
    expect(result.totalCount).toBe(3);
  });

  it('first in_progress wins when multiple are active', () => {
    const result = readPlanSummary({
      plan: [
        { step: 'first', status: 'in_progress' },
        { step: 'second', status: 'in_progress' },
      ],
    });
    expect(result.bucket).toBe('in_progress');
    expect(result.activeStep).toBe('first');
  });

  it('classifies completed+pending with no in_progress as bucket=paused', () => {
    expect(
      readPlanSummary({
        plan: [
          { step: 'a', status: 'completed' },
          { step: 'b', status: 'pending' },
          { step: 'c', status: 'pending' },
        ],
      }),
    ).toEqual({
      bucket: 'paused',
      completedCount: 1,
      totalCount: 3,
    });
  });

  it('counts unrecognized status values toward total but not completed', () => {
    const result = readPlanSummary({
      plan: [
        { step: 'a', status: 'completed' },
        { step: 'b', status: 'blocked' }, // unrecognized — forward-compat
        { step: 'c', status: 'pending' },
      ],
    });
    expect(result.completedCount).toBe(1);
    expect(result.totalCount).toBe(3);
    // No in_progress, has completed AND pending → paused.
    expect(result.bucket).toBe('paused');
  });

  it('skips steps with non-string `step` field cleanly (no crash, no count)', () => {
    const result = readPlanSummary({
      plan: [
        { step: 'valid', status: 'completed' },
        { step: 42, status: 'in_progress' }, // malformed, skipped
        { status: 'pending' }, // malformed, skipped
      ],
    });
    expect(result.totalCount).toBe(1);
    expect(result.completedCount).toBe(1);
    expect(result.bucket).toBe('complete');
  });
});
