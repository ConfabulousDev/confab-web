// Service tests for Codex transcript parsing + normalization.
//
// Tests are spec-derived from CF-349. They lock the contract that
// `parseCodexJSONL` validates input and `normalizeCodexLines` transforms
// validated raw lines into a clean render-item stream.

import { describe, it, expect, beforeEach } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import {
  parseCodexJSONL,
  normalizeCodexLines,
  _resetReportedCodexSessions,
} from './codexTranscriptService';
import type { CodexRenderItem } from '@/types/codexRenderItem';

const FIXTURE_PATH = resolve(__dirname, '../test-fixtures/codex-rollout.jsonl');
const fixtureJsonl = readFileSync(FIXTURE_PATH, 'utf-8');

beforeEach(() => {
  _resetReportedCodexSessions();
});

// ---------------------------------------------------------------------------
// parseCodexJSONL
// ---------------------------------------------------------------------------

describe('parseCodexJSONL', () => {
  it('parses every fixture line without errors', () => {
    const result = parseCodexJSONL(fixtureJsonl);
    expect(result.errorCount).toBe(0);
    expect(result.successCount).toBe(result.totalLines);
    expect(result.rawLines.length).toBeGreaterThan(0);
  });

  it('totalLines reflects non-empty line count', () => {
    const result = parseCodexJSONL(fixtureJsonl);
    const nonEmpty = fixtureJsonl.split('\n').filter((l) => l.trim().length > 0).length;
    expect(result.totalLines).toBe(nonEmpty);
  });

  it('records a malformed line in errors without aborting the parse', () => {
    const jsonl = [
      '{"timestamp":"2026-05-13T01:00:00Z","type":"session_meta","payload":{"id":"x"}}',
      'not valid json',
      '{"timestamp":"2026-05-13T01:00:01Z","type":"compacted","payload":{"message":"","replacement_history":[]}}',
    ].join('\n');
    const result = parseCodexJSONL(jsonl);
    expect(result.totalLines).toBe(3);
    expect(result.errorCount).toBe(1);
    expect(result.successCount).toBe(2);
    expect(result.errors[0]?.line).toBe(2);
  });

  it('skips empty lines without counting them as errors', () => {
    const jsonl = [
      '{"timestamp":"2026-05-13T01:00:00Z","type":"session_meta","payload":{"id":"x"}}',
      '',
      '   ',
      '{"timestamp":"2026-05-13T01:00:01Z","type":"compacted","payload":{"message":"","replacement_history":[]}}',
    ].join('\n');
    const result = parseCodexJSONL(jsonl);
    expect(result.errorCount).toBe(0);
    expect(result.totalLines).toBe(2);
  });
});

// ---------------------------------------------------------------------------
// normalizeCodexLines
// ---------------------------------------------------------------------------

function items(jsonl: string): CodexRenderItem[] {
  const { rawLines } = parseCodexJSONL(jsonl);
  return normalizeCodexLines(rawLines);
}

describe('normalizeCodexLines', () => {
  // -------------------------------------------------------------------------
  // Drop noise
  // -------------------------------------------------------------------------

  it('drops session_meta, turn_context, event_msg.token_count, event_msg.task_started', () => {
    const result = items(fixtureJsonl);
    for (const item of result) {
      expect(item.kind).not.toBe('session_meta');
      expect(item.kind).not.toBe('turn_context');
      expect(item.kind).not.toBe('token_count');
      expect(item.kind).not.toBe('task_started');
    }
  });

  it('drops event_msg.user_message and event_msg.agent_message (redundant with response_item.message)', () => {
    const userText = 'add the linear mcp to my codex config';
    const result = items(fixtureJsonl);
    const userOccurrences = result.filter((i) => i.kind === 'user' && i.text === userText);
    // The fixture has one response_item.message[role=user] AND one
    // event_msg.user_message with identical text. Normalization should
    // emit only the response_item version.
    expect(userOccurrences.length).toBe(1);
  });

  it('drops response_item.message[role=developer]', () => {
    const result = items(fixtureJsonl);
    // Developer messages start with `<permissions instructions>` in the fixture.
    const developerLeakage = result.filter(
      (i) => i.kind === 'user' && i.text.includes('permissions instructions'),
    );
    expect(developerLeakage.length).toBe(0);
  });

  // -------------------------------------------------------------------------
  // User messages
  // -------------------------------------------------------------------------

  it('strips <environment_context>...</environment_context> from user message text', () => {
    const result = items(fixtureJsonl);
    for (const item of result) {
      if (item.kind === 'user') {
        expect(item.text).not.toContain('<environment_context>');
        expect(item.text).not.toContain('</environment_context>');
      }
    }
  });

  it('emits user items in order with raw text', () => {
    const result = items(fixtureJsonl);
    const userItems = result.filter((i) => i.kind === 'user');
    expect(userItems.length).toBe(2);
    expect(userItems[0]?.kind === 'user' && userItems[0]?.text).toBe(
      'add the linear mcp to my codex config',
    );
    expect(userItems[1]?.kind === 'user' && userItems[1]?.text).toBe('look at CF-342');
  });

  // -------------------------------------------------------------------------
  // Assistant messages
  // -------------------------------------------------------------------------

  it('emits assistant items with phase from response_item.message.phase', () => {
    const result = items(fixtureJsonl);
    const assistants = result.filter((i) => i.kind === 'assistant');
    // Two assistant messages with phase: 'commentary' and 'final' for turn 1,
    // one 'final' for turn 2.
    const phases = assistants.map((a) => (a.kind === 'assistant' ? a.phase : null));
    expect(phases).toContain('commentary');
    expect(phases.filter((p) => p === 'final').length).toBe(2);
  });

  it('attaches model name from session_meta / task_started to assistant items', () => {
    const result = items(fixtureJsonl);
    const assistants = result.filter((i) => i.kind === 'assistant');
    for (const a of assistants) {
      if (a.kind === 'assistant') {
        expect(a.model).toBe('gpt-5');
      }
    }
  });

  // -------------------------------------------------------------------------
  // Tool calls
  // -------------------------------------------------------------------------

  it('pairs function_call with function_call_output by call_id', () => {
    const result = items(fixtureJsonl);
    const pwdCall = result.find(
      (i) => i.kind === 'tool_call' && i.callId === 'call_fixture_pwd_0001',
    );
    expect(pwdCall).toBeDefined();
    if (pwdCall && pwdCall.kind === 'tool_call') {
      expect(pwdCall.toolName).toBe('exec_command');
      expect(pwdCall.status).toBe('completed');
      expect(pwdCall.rawOutput).toBeDefined();
    }
  });

  it('strips exec output preamble and surfaces exit code + wall time as execMetadata', () => {
    const result = items(fixtureJsonl);
    const pwdCall = result.find(
      (i) => i.kind === 'tool_call' && i.callId === 'call_fixture_pwd_0001',
    );
    expect(pwdCall).toBeDefined();
    if (pwdCall && pwdCall.kind === 'tool_call') {
      expect(pwdCall.execMetadata?.exitCode).toBe(0);
      expect(pwdCall.execMetadata?.wallTimeMs).toBeGreaterThanOrEqual(0);
      // The displayed output should NOT contain the preamble metadata lines.
      expect(pwdCall.rawOutput).not.toContain('Chunk ID:');
      expect(pwdCall.rawOutput).not.toContain('Wall time:');
      expect(pwdCall.rawOutput).not.toContain('Process exited with code');
      expect(pwdCall.rawOutput).not.toContain('Original token count:');
      // The actual command output IS present.
      expect(pwdCall.rawOutput).toContain('/Users/dev/example-project');
    }
  });

  it('pairs custom_tool_call with both custom_tool_call_output and event_msg.patch_apply_end', () => {
    const result = items(fixtureJsonl);
    const patchCall = result.find(
      (i) => i.kind === 'tool_call' && i.callId === 'call_fixture_patch_0001',
    );
    expect(patchCall).toBeDefined();
    if (patchCall && patchCall.kind === 'tool_call') {
      expect(patchCall.toolName).toBe('apply_patch');
      // Raw output from custom_tool_call_output.
      expect(patchCall.rawOutput).toBeDefined();
      // Structured output from event_msg.patch_apply_end.
      expect(patchCall.structuredOutput).toBeDefined();
    }
  });

  it('emits a pending tool call when no matching output has arrived', () => {
    const result = items(fixtureJsonl);
    const pending = result.find(
      (i) => i.kind === 'tool_call' && i.callId === 'call_fixture_pending_0099',
    );
    expect(pending).toBeDefined();
    if (pending && pending.kind === 'tool_call') {
      expect(pending.status).toBe('pending');
      expect(pending.rawOutput).toBeUndefined();
    }
  });

  // -------------------------------------------------------------------------
  // Reasoning, turn separators, compaction
  // -------------------------------------------------------------------------

  it('emits a CodexReasoningHiddenItem for each reasoning line', () => {
    const result = items(fixtureJsonl);
    const reasoning = result.filter((i) => i.kind === 'reasoning_hidden');
    expect(reasoning.length).toBe(1);
  });

  it('emits CodexTurnSeparatorItem per task_complete with durationMs and turnIndex', () => {
    const result = items(fixtureJsonl);
    const separators = result.filter((i) => i.kind === 'turn_separator');
    expect(separators.length).toBe(2);
    if (separators[0]?.kind === 'turn_separator') {
      expect(separators[0].turnIndex).toBe(1);
      expect(separators[0].durationMs).toBe(11000);
      expect(separators[0].timeToFirstTokenMs).toBe(1704);
    }
    if (separators[1]?.kind === 'turn_separator') {
      expect(separators[1].turnIndex).toBe(2);
      expect(separators[1].durationMs).toBe(6000);
    }
  });

  it('emits CodexCompactedItem with replacementCount', () => {
    const result = items(fixtureJsonl);
    const compacted = result.filter((i) => i.kind === 'compacted');
    expect(compacted.length).toBe(1);
    if (compacted[0]?.kind === 'compacted') {
      expect(compacted[0].replacementCount).toBe(2);
    }
  });

  // -------------------------------------------------------------------------
  // Forward compat
  // -------------------------------------------------------------------------

  it('emits CodexUnknownItem for unrecognized top-level type', () => {
    const result = items(fixtureJsonl);
    const unknown = result.filter((i) => i.kind === 'unknown');
    // Fixture has 3 unknown lines: future_top_level_type,
    // response_item.future_payload_type, event_msg.future_event_payload.
    expect(unknown.length).toBeGreaterThanOrEqual(1);
  });

  // -------------------------------------------------------------------------
  // Ordering and timing
  // -------------------------------------------------------------------------

  it('preserves chronological order via timestamps', () => {
    const result = items(fixtureJsonl);
    for (let i = 1; i < result.length; i++) {
      const cur = result[i];
      const prev = result[i - 1];
      if (cur && prev) {
        expect(cur.timestamp >= prev.timestamp).toBe(true);
      }
    }
  });

  // -------------------------------------------------------------------------
  // CF-360: stable line identity (lineId)
  // -------------------------------------------------------------------------

  describe('lineId', () => {
    it('assigns a non-empty string lineId to every emitted item', () => {
      const result = items(fixtureJsonl);
      expect(result.length).toBeGreaterThan(0);
      for (const item of result) {
        expect(typeof item.lineId).toBe('string');
        expect(item.lineId.length).toBeGreaterThan(0);
      }
    });

    it('lineId is unique across all emitted items in the fixture', () => {
      const result = items(fixtureJsonl);
      const ids = result.map((i) => i.lineId);
      expect(new Set(ids).size).toBe(result.length);
    });

    it('user item lineId equals the String() of its rawLines index', () => {
      // Single-line JSONL: one response_item.message[role=user] at index 0.
      const jsonl =
        '{"timestamp":"2026-05-13T01:00:00Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}}';
      const result = items(jsonl);
      const userItem = result.find((i) => i.kind === 'user');
      expect(userItem).toBeDefined();
      expect(userItem?.lineId).toBe('0');
    });

    it('compacted item lineId equals its rawLines index', () => {
      const jsonl = [
        '{"timestamp":"2026-05-13T01:00:00Z","type":"session_meta","payload":{"id":"x","model":"gpt-5"}}',
        '{"timestamp":"2026-05-13T01:00:01Z","type":"compacted","payload":{"message":"summary","replacement_history":[{"x":1},{"x":2},{"x":3}]}}',
      ].join('\n');
      const result = items(jsonl);
      const compacted = result.find((i) => i.kind === 'compacted');
      expect(compacted).toBeDefined();
      // session_meta is at rawLines[0], compacted at rawLines[1].
      expect(compacted?.lineId).toBe('1');
    });

    it('turn_separator lineId equals the task_complete line index', () => {
      const jsonl = [
        '{"timestamp":"2026-05-13T01:00:00Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}}',
        '{"timestamp":"2026-05-13T01:00:01Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}}',
        '{"timestamp":"2026-05-13T01:00:02Z","type":"event_msg","payload":{"type":"task_complete","duration_ms":2000,"time_to_first_token_ms":500}}',
      ].join('\n');
      const result = items(jsonl);
      const sep = result.find((i) => i.kind === 'turn_separator');
      expect(sep).toBeDefined();
      expect(sep?.lineId).toBe('2');
    });

    it('reasoning_hidden lineId equals its rawLines index', () => {
      const jsonl = [
        '{"timestamp":"2026-05-13T01:00:00Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}}',
        '{"timestamp":"2026-05-13T01:00:01Z","type":"response_item","payload":{"type":"reasoning","encrypted_content":"…"}}',
      ].join('\n');
      const result = items(jsonl);
      const reasoning = result.find((i) => i.kind === 'reasoning_hidden');
      expect(reasoning).toBeDefined();
      expect(reasoning?.lineId).toBe('1');
    });

    it('tool_call lineId tracks the initial function_call line, not its output line', () => {
      // function_call at rawLines[0], function_call_output at rawLines[1].
      const jsonl = [
        '{"timestamp":"2026-05-13T01:00:00Z","type":"response_item","payload":{"type":"function_call","call_id":"c_pair_01","name":"exec_command","arguments":"{\\"cmd\\":\\"pwd\\"}"}}',
        '{"timestamp":"2026-05-13T01:00:01Z","type":"response_item","payload":{"type":"function_call_output","call_id":"c_pair_01","output":"Output:\\n/tmp\\n"}}',
      ].join('\n');
      const result = items(jsonl);
      const toolCall = result.find((i) => i.kind === 'tool_call');
      expect(toolCall).toBeDefined();
      // Initial function_call is at index 0; output mutates in place.
      expect(toolCall?.lineId).toBe('0');
      // And the call resolved (output was paired in).
      if (toolCall?.kind === 'tool_call') {
        expect(toolCall.status).not.toBe('pending');
      }
    });

    it('custom_tool_call_output and patch_apply_end do not overwrite the call lineId', () => {
      // custom_tool_call at [0], output at [1], patch_apply_end at [2].
      const jsonl = [
        '{"timestamp":"2026-05-13T01:00:00Z","type":"response_item","payload":{"type":"custom_tool_call","call_id":"c_patch_01","name":"apply_patch","input":"*** Begin Patch\\n*** End Patch","status":"in_progress"}}',
        '{"timestamp":"2026-05-13T01:00:01Z","type":"response_item","payload":{"type":"custom_tool_call_output","call_id":"c_patch_01","output":"{}"}}',
        '{"timestamp":"2026-05-13T01:00:02Z","type":"event_msg","payload":{"type":"patch_apply_end","call_id":"c_patch_01","success":true,"changes":{}}}',
      ].join('\n');
      const result = items(jsonl);
      const toolCall = result.find((i) => i.kind === 'tool_call');
      expect(toolCall).toBeDefined();
      expect(toolCall?.lineId).toBe('0');
    });

    it('web_search_call lineId equals its source line index', () => {
      const jsonl = [
        '{"timestamp":"2026-05-13T01:00:00Z","type":"session_meta","payload":{"id":"x","model":"gpt-5"}}',
        '{"timestamp":"2026-05-13T01:00:01Z","type":"response_item","payload":{"type":"web_search_call","status":"completed","action":{"query":"codex"}}}',
      ].join('\n');
      const result = items(jsonl);
      const ws = result.find((i) => i.kind === 'tool_call' && i.toolName === 'web_search_call');
      expect(ws).toBeDefined();
      expect(ws?.lineId).toBe('1');
    });
  });
});
