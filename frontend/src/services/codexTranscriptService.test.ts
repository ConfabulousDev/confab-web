// Service tests for Codex transcript parsing + normalization.
//
// Tests are spec-derived from CF-349. They lock the contract that
// `parseCodexJSONL` validates input and `normalizeCodexLines` transforms
// validated raw lines into a clean render-item stream.

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import {
  parseCodexJSONL,
  normalizeCodexLines,
  extractCodexModel,
  reportCodexTranscriptErrors,
  _resetReportedCodexSessions,
} from './codexTranscriptService';
import type { CodexRenderItem } from '@/types/codexRenderItem';
import type { RawCodexLine, TranscriptValidationError } from '@/schemas/codexTranscript';

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

  it('event_msg.mcp_tool_call_end enriches the paired tool_call with mcpInvocation (CF-368)', () => {
    const jsonl = [
      {
        timestamp: '2026-05-13T01:00:00Z',
        type: 'response_item',
        payload: {
          type: 'function_call',
          call_id: 'call_mcp_001',
          name: 'save_issue',
          namespace: 'mcp__linear__',
          arguments: '{"team":"Confabulous"}',
        },
      },
      {
        timestamp: '2026-05-13T01:00:01Z',
        type: 'event_msg',
        payload: {
          type: 'mcp_tool_call_end',
          call_id: 'call_mcp_001',
          invocation: {
            server: 'linear',
            tool: 'save_issue',
            arguments: { team: 'Confabulous' },
          },
          duration: { secs: 1, nanos: 250000000 },
          result: {
            Ok: {
              content: [{ type: 'text', text: '{"id":"CF-404"}' }],
            },
          },
        },
      },
      {
        timestamp: '2026-05-13T01:00:02Z',
        type: 'response_item',
        payload: {
          type: 'function_call_output',
          call_id: 'call_mcp_001',
          output: 'Wall time: 1.2500 seconds\nOutput:\n[{"type":"text","text":"{\\"id\\":\\"CF-404\\"}"}]',
        },
      },
    ]
      .map((line) => JSON.stringify(line))
      .join('\n');

    const result = items(jsonl);
    // Only the paired function_call row should be emitted — mcp_tool_call_end
    // mutates it in-place rather than producing its own item.
    expect(result).toHaveLength(1);
    const toolCall = result[0];
    expect(toolCall?.kind).toBe('tool_call');
    if (toolCall?.kind === 'tool_call') {
      expect(toolCall.callId).toBe('call_mcp_001');
      expect(toolCall.status).toBe('completed');
      expect(toolCall.rawOutput).toContain('CF-404');
      expect(toolCall.mcpInvocation).toEqual({ server: 'linear', tool: 'save_issue' });
    }
  });

  it('event_msg.mcp_tool_call_end with no server/tool drops the enrichment (CF-368)', () => {
    const jsonl = [
      {
        timestamp: '2026-05-13T01:00:00Z',
        type: 'response_item',
        payload: {
          type: 'function_call',
          call_id: 'call_mcp_empty',
          name: 'unlabeled',
          arguments: '{}',
        },
      },
      {
        timestamp: '2026-05-13T01:00:01Z',
        type: 'event_msg',
        payload: {
          type: 'mcp_tool_call_end',
          call_id: 'call_mcp_empty',
          // invocation omitted entirely
        },
      },
      {
        timestamp: '2026-05-13T01:00:02Z',
        type: 'response_item',
        payload: {
          type: 'function_call_output',
          call_id: 'call_mcp_empty',
          output: '',
        },
      },
    ]
      .map((line) => JSON.stringify(line))
      .join('\n');

    const result = items(jsonl);
    expect(result).toHaveLength(1);
    const toolCall = result[0];
    if (toolCall?.kind === 'tool_call') {
      expect(toolCall.mcpInvocation).toBeUndefined();
    }
  });

  it('drops event_msg.web_search_end as redundant with response_item.web_search_call (CF-368)', () => {
    const jsonl = [
      {
        timestamp: '2026-05-13T01:00:00Z',
        type: 'response_item',
        payload: {
          type: 'web_search_call',
          status: 'completed',
          action: { type: 'search', query: 'codex jsonl format' },
        },
      },
      {
        timestamp: '2026-05-13T01:00:01Z',
        type: 'event_msg',
        payload: {
          type: 'web_search_end',
          call_id: 'ws_abc',
          query: 'codex jsonl format',
          action: { type: 'search', query: 'codex jsonl format', queries: ['codex jsonl format'] },
        },
      },
    ]
      .map((line) => JSON.stringify(line))
      .join('\n');

    const result = items(jsonl);
    // Only the response_item.web_search_call survives as a tool_call.
    // The event_msg.web_search_end must NOT produce an additional row,
    // and crucially must NOT fall through to a CodexUnknownItem.
    expect(result).toHaveLength(1);
    expect(result[0]?.kind).toBe('tool_call');
    expect(result.find((i) => i.kind === 'unknown')).toBeUndefined();
  });

  it('drops event_msg.context_compacted as redundant with top-level compacted line (CF-368)', () => {
    const jsonl = [
      {
        timestamp: '2026-05-13T01:00:00Z',
        type: 'event_msg',
        payload: { type: 'context_compacted' },
      },
      {
        timestamp: '2026-05-13T01:00:01Z',
        type: 'compacted',
        payload: {
          message: 'summary',
          replacement_history: [{ a: 1 }, { b: 2 }],
        },
      },
    ]
      .map((line) => JSON.stringify(line))
      .join('\n');

    const result = items(jsonl);
    // Only the top-level `compacted` line produces a divider; the
    // event_msg.context_compacted preview is noise and must be dropped.
    expect(result).toHaveLength(1);
    expect(result[0]?.kind).toBe('compacted');
    expect(result.find((i) => i.kind === 'unknown')).toBeUndefined();
  });

  it('emits CodexTurnAbortedItem for event_msg.turn_aborted (CF-368)', () => {
    const jsonl = [
      {
        timestamp: '2026-05-13T01:00:00Z',
        type: 'event_msg',
        payload: {
          type: 'turn_aborted',
          turn_id: '019e3bb4-2150-70f3-a356-599189a0292c',
          reason: 'interrupted',
          completed_at: 1779118150,
          duration_ms: 29367,
        },
      },
    ]
      .map((line) => JSON.stringify(line))
      .join('\n');

    const result = items(jsonl);
    expect(result).toHaveLength(1);
    const aborted = result[0];
    expect(aborted?.kind).toBe('turn_aborted');
    if (aborted?.kind === 'turn_aborted') {
      expect(aborted.reason).toBe('interrupted');
      expect(aborted.durationMs).toBe(29367);
      expect(aborted.timestamp).toBe('2026-05-13T01:00:00Z');
      expect(aborted.lineId).toBe('0');
    }
    expect(result.find((i) => i.kind === 'unknown')).toBeUndefined();
  });

  it('CodexTurnAbortedItem tolerates missing reason and duration_ms (CF-368)', () => {
    const jsonl = [
      {
        timestamp: '2026-05-13T01:00:00Z',
        type: 'event_msg',
        payload: { type: 'turn_aborted' },
      },
    ]
      .map((line) => JSON.stringify(line))
      .join('\n');

    const result = items(jsonl);
    expect(result).toHaveLength(1);
    const aborted = result[0];
    if (aborted?.kind === 'turn_aborted') {
      expect(aborted.reason).toBe('');
      expect(aborted.durationMs).toBe(0);
    }
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

  // Codex CLI ~0.130+ (CF-379) moved the per-turn model out of session_meta /
  // task_started and into turn_context. Without picking it up here, the
  // assistant render items fall back to the initial 'unknown' literal and the
  // transcript view badges every assistant message "unknown".
  it('picks up the model from turn_context when session_meta and task_started omit it', () => {
    const jsonl = [
      '{"timestamp":"2026-05-13T01:00:00Z","type":"session_meta","payload":{"id":"x","cwd":"/x"}}',
      '{"timestamp":"2026-05-13T01:00:00.100Z","type":"turn_context","payload":{"turn_id":"t1","model":"gpt-5.5"}}',
      '{"timestamp":"2026-05-13T01:00:00.200Z","type":"event_msg","payload":{"type":"task_started","turn_id":"t1"}}',
      '{"timestamp":"2026-05-13T01:00:00.300Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}}',
    ].join('\n');
    const result = items(jsonl);
    const assistants = result.filter((i) => i.kind === 'assistant');
    expect(assistants.length).toBe(1);
    if (assistants[0]?.kind === 'assistant') {
      expect(assistants[0].model).toBe('gpt-5.5');
    }
  });

  it('updates the running model on each new turn_context envelope', () => {
    const jsonl = [
      '{"timestamp":"2026-05-13T01:00:00Z","type":"session_meta","payload":{"id":"x","cwd":"/x"}}',
      '{"timestamp":"2026-05-13T01:00:00.100Z","type":"turn_context","payload":{"turn_id":"t1","model":"gpt-5"}}',
      '{"timestamp":"2026-05-13T01:00:00.300Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"first"}]}}',
      '{"timestamp":"2026-05-13T01:01:00Z","type":"turn_context","payload":{"turn_id":"t2","model":"gpt-5.5"}}',
      '{"timestamp":"2026-05-13T01:01:00.300Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"second"}]}}',
    ].join('\n');
    const result = items(jsonl);
    const assistants = result.filter((i) => i.kind === 'assistant');
    expect(assistants.length).toBe(2);
    const models = assistants.map((a) => (a.kind === 'assistant' ? a.model : null));
    expect(models).toEqual(['gpt-5', 'gpt-5.5']);
  });

  // Pins the last-wins ordering for rollouts in transition that carry both
  // envelopes: turn_context is the more specific per-turn signal and must
  // override whatever session_meta set earlier in the stream.
  it('turn_context.model overrides session_meta.model for subsequent assistant items', () => {
    const jsonl = [
      '{"timestamp":"2026-05-13T01:00:00Z","type":"session_meta","payload":{"id":"x","cwd":"/x","model":"gpt-5"}}',
      '{"timestamp":"2026-05-13T01:00:00.100Z","type":"turn_context","payload":{"turn_id":"t1","model":"gpt-5.5"}}',
      '{"timestamp":"2026-05-13T01:00:00.300Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}}',
    ].join('\n');
    const result = items(jsonl);
    const assistants = result.filter((i) => i.kind === 'assistant');
    expect(assistants.length).toBe(1);
    if (assistants[0]?.kind === 'assistant') {
      expect(assistants[0].model).toBe('gpt-5.5');
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

  // -------------------------------------------------------------------------
  // CF-388: image content blocks
  //
  // Codex injects `<image name=[Image #N]>` / `</image>` sentinel wrappers
  // around each `input_image` block. The normalizer strips those wrappers
  // and surfaces image_url values onto `images` on the user/assistant item.
  // -------------------------------------------------------------------------

  describe('image content blocks', () => {
    const DATA_URL_1 = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkAAIAAAoAAv/lxKUAAAAASUVORK5CYII=';
    const DATA_URL_2 = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAcAAc6POE4AAAAASUVORK5CYII=';

    function userMessageJsonl(content: unknown[]): string {
      return JSON.stringify({
        timestamp: '2026-05-13T01:00:00Z',
        type: 'response_item',
        payload: { type: 'message', role: 'user', content },
      });
    }

    function assistantMessageJsonl(content: unknown[]): string {
      return JSON.stringify({
        timestamp: '2026-05-13T01:00:00Z',
        type: 'response_item',
        payload: {
          type: 'message',
          role: 'assistant',
          content,
          phase: 'final',
        },
      });
    }

    it('extracts input_image.image_url onto images on the user item', () => {
      const jsonl = userMessageJsonl([
        { type: 'input_text', text: 'look at this screenshot' },
        { type: 'input_image', image_url: DATA_URL_1, detail: 'high' },
      ]);
      const result = items(jsonl);
      const user = result.find((i) => i.kind === 'user');
      expect(user).toBeDefined();
      if (user?.kind === 'user') {
        expect(user.images).toEqual([DATA_URL_1]);
        expect(user.text).toBe('look at this screenshot');
      }
    });

    it('strips <image name=[Image #1]> sentinel wrappers from input_text', () => {
      const jsonl = userMessageJsonl([
        { type: 'input_text', text: '<image name=[Image #1]>' },
        { type: 'input_image', image_url: DATA_URL_1 },
        { type: 'input_text', text: '</image>' },
        { type: 'input_text', text: 'what do you see here?' },
      ]);
      const result = items(jsonl);
      const user = result.find((i) => i.kind === 'user');
      expect(user).toBeDefined();
      if (user?.kind === 'user') {
        expect(user.text).not.toContain('<image');
        expect(user.text).not.toContain('</image>');
        expect(user.text).toContain('what do you see here?');
      }
    });

    it('strips bare <image> / </image> sentinel variants (no name attribute)', () => {
      const jsonl = userMessageJsonl([
        { type: 'input_text', text: '<image>' },
        { type: 'input_image', image_url: DATA_URL_1 },
        { type: 'input_text', text: '</image>\n\nfollow-up question' },
      ]);
      const result = items(jsonl);
      const user = result.find((i) => i.kind === 'user');
      expect(user).toBeDefined();
      if (user?.kind === 'user') {
        expect(user.text).not.toContain('<image');
        expect(user.text).not.toContain('</image>');
        expect(user.text).toContain('follow-up question');
      }
    });

    it('preserves bare [Image #1] references inside ordinary user prose', () => {
      const jsonl = userMessageJsonl([
        { type: 'input_text', text: 'as you can see in [Image #1] the layout is broken' },
        { type: 'input_image', image_url: DATA_URL_1 },
      ]);
      const result = items(jsonl);
      const user = result.find((i) => i.kind === 'user');
      expect(user).toBeDefined();
      if (user?.kind === 'user') {
        // Bare `[Image #1]` references must survive — only `<image …>` /
        // `</image>` wrappers are stripped.
        expect(user.text).toContain('[Image #1]');
      }
    });

    it('emits an image-only user item when text strips to empty but images is non-empty', () => {
      const jsonl = userMessageJsonl([
        { type: 'input_text', text: '<image name=[Image #1]>' },
        { type: 'input_image', image_url: DATA_URL_1 },
        { type: 'input_text', text: '</image>' },
      ]);
      const result = items(jsonl);
      const user = result.find((i) => i.kind === 'user');
      expect(user).toBeDefined();
      if (user?.kind === 'user') {
        expect(user.text).toBe('');
        expect(user.images).toEqual([DATA_URL_1]);
      }
    });

    it('text-only user message produces an item with no images field', () => {
      const jsonl = userMessageJsonl([
        { type: 'input_text', text: 'plain user message, no attachments' },
      ]);
      const result = items(jsonl);
      const user = result.find((i) => i.kind === 'user');
      expect(user).toBeDefined();
      if (user?.kind === 'user') {
        expect(user.images).toBeUndefined();
        expect(user.text).toBe('plain user message, no attachments');
      }
    });

    it('attaches images to the assistant item from output_image', () => {
      const jsonl = assistantMessageJsonl([
        { type: 'output_text', text: 'here is a rendered chart' },
        { type: 'output_image', image_url: DATA_URL_1 },
      ]);
      const result = items(jsonl);
      const assistant = result.find((i) => i.kind === 'assistant');
      expect(assistant).toBeDefined();
      if (assistant?.kind === 'assistant') {
        expect(assistant.images).toEqual([DATA_URL_1]);
        expect(assistant.text).toBe('here is a rendered chart');
      }
    });

    it('strips sentinel wrappers even when they appear inside fenced code blocks', () => {
      // Per CF-388 interview: regex runs on raw text before markdown render,
      // and strips unconditionally. A user documenting the sentinel inside
      // a code fence is a knowingly-accepted false positive — Codex emits the
      // sentinel itself, real users do not.
      const jsonl = userMessageJsonl([
        {
          type: 'input_text',
          text: '```\n<image name=[Image #1]>\n```\nwhat does this tag mean?',
        },
        { type: 'input_image', image_url: DATA_URL_1 },
      ]);
      const result = items(jsonl);
      const user = result.find((i) => i.kind === 'user');
      expect(user).toBeDefined();
      if (user?.kind === 'user') {
        expect(user.text).not.toContain('<image');
        expect(user.text).toContain('what does this tag mean?');
      }
    });

    it('extracts multiple input_image blocks in order onto a single images array', () => {
      const jsonl = userMessageJsonl([
        { type: 'input_text', text: 'two screenshots:' },
        { type: 'input_image', image_url: DATA_URL_1 },
        { type: 'input_image', image_url: DATA_URL_2 },
      ]);
      const result = items(jsonl);
      const user = result.find((i) => i.kind === 'user');
      expect(user).toBeDefined();
      if (user?.kind === 'user') {
        expect(user.images).toEqual([DATA_URL_1, DATA_URL_2]);
      }
    });
  });
});

// ---------------------------------------------------------------------------
// extractCodexModel (CF-386)
//
// CF-383 added `fetchCodexSessionMeta` that read only the rollout's first
// line. Real Codex rollouts often miss `payload.model` on `session_meta` — the
// canonical source per CF-379 is `turn_context.model`. CF-386 lifts Codex
// transcript state up into SessionViewer (mirroring Claude) and replaces the
// line-1 helper with this scan-everything fallback chain, matching the backend
// parser at `backend/internal/codex/parser.go:170-177`.
// ---------------------------------------------------------------------------

// Build a RawCodexLine by parsing a single JSONL snippet — keeps tests in
// terms of the wire shape rather than constructing schema-validated objects
// by hand. Throws if the snippet failed to validate so test setup errors
// don't masquerade as assertion failures.
function rawLine(jsonl: string): RawCodexLine {
  const line = parseCodexJSONL(jsonl).rawLines[0];
  if (!line) throw new Error(`rawLine helper: failed to parse ${jsonl}`);
  return line;
}

describe('extractCodexModel', () => {
  it('returns model from a session_meta line', () => {
    const lines = [
      rawLine(
        '{"timestamp":"2026-05-13T01:00:00Z","type":"session_meta","payload":{"id":"x","model":"gpt-5-codex"}}',
      ),
    ];
    expect(extractCodexModel(lines)).toBe('gpt-5-codex');
  });

  it('returns model from a turn_context line when session_meta has no model', () => {
    const lines = [
      rawLine(
        '{"timestamp":"2026-05-13T01:00:00Z","type":"session_meta","payload":{"id":"x"}}',
      ),
      rawLine(
        '{"timestamp":"2026-05-13T01:00:01Z","type":"turn_context","payload":{"turn_id":"t1","model":"gpt-5"}}',
      ),
    ];
    expect(extractCodexModel(lines)).toBe('gpt-5');
  });

  it('falls through to a later turn_context when the first has no model', () => {
    const lines = [
      rawLine(
        '{"timestamp":"2026-05-13T01:00:00Z","type":"session_meta","payload":{"id":"x"}}',
      ),
      rawLine(
        '{"timestamp":"2026-05-13T01:00:01Z","type":"turn_context","payload":{"turn_id":"t1"}}',
      ),
      rawLine(
        '{"timestamp":"2026-05-13T01:00:02Z","type":"turn_context","payload":{"turn_id":"t2","model":"gpt-5"}}',
      ),
    ];
    expect(extractCodexModel(lines)).toBe('gpt-5');
  });

  it('prefers the earliest non-empty model encountered', () => {
    const lines = [
      rawLine(
        '{"timestamp":"2026-05-13T01:00:00Z","type":"session_meta","payload":{"id":"x","model":"first"}}',
      ),
      rawLine(
        '{"timestamp":"2026-05-13T01:00:01Z","type":"turn_context","payload":{"turn_id":"t1","model":"second"}}',
      ),
    ];
    expect(extractCodexModel(lines)).toBe('first');
  });

  it('returns undefined when no session_meta or turn_context line carries a model', () => {
    const lines = [
      rawLine(
        '{"timestamp":"2026-05-13T01:00:00Z","type":"session_meta","payload":{"id":"x"}}',
      ),
      rawLine(
        '{"timestamp":"2026-05-13T01:00:01Z","type":"turn_context","payload":{"turn_id":"t1"}}',
      ),
    ];
    expect(extractCodexModel(lines)).toBeUndefined();
  });

  it('returns undefined for an empty rawLines array', () => {
    expect(extractCodexModel([])).toBeUndefined();
  });

  it('ignores response_item and event_msg lines (only scans session_meta / turn_context)', () => {
    const lines = [
      rawLine(
        '{"timestamp":"2026-05-13T01:00:00Z","type":"event_msg","payload":{"type":"task_started","turn_id":"t1","model":"event-msg-model"}}',
      ),
    ];
    expect(extractCodexModel(lines)).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// CF-362 — token_count → assistant item usage attribution.
//
// `event_msg.token_count` carries `info.last_token_usage` (per-call delta).
// On each occurrence, attach the delta to the most-recent assistant render-item
// whose `usage` is still undefined — walking backwards from the end of the
// items array. Multi-call turns yield multiple token_count events; each
// attributes to its own assistant item.
// ---------------------------------------------------------------------------

describe('normalizeCodexLines — token_count attribution (CF-362)', () => {
  const USAGE_1000_500 = `{"input_tokens":1000,"cached_input_tokens":0,"output_tokens":500,"reasoning_output_tokens":0,"total_tokens":1500}`;
  const USAGE_2000_700 = `{"input_tokens":2000,"cached_input_tokens":300,"output_tokens":700,"reasoning_output_tokens":100,"total_tokens":2800}`;

  it('attaches last_token_usage to the preceding assistant final', () => {
    const jsonl = [
      '{"timestamp":"2026-05-13T01:00:00Z","type":"session_meta","payload":{"model":"gpt-5"}}',
      '{"timestamp":"2026-05-13T01:00:01Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}}',
      '{"timestamp":"2026-05-13T01:00:02Z","type":"response_item","payload":{"type":"message","role":"assistant","phase":"final","content":[{"type":"output_text","text":"hello"}]}}',
      `{"timestamp":"2026-05-13T01:00:03Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":${USAGE_1000_500},"total_token_usage":${USAGE_1000_500}}}}`,
    ].join('\n');
    const result = items(jsonl);
    const assistantItems = result.filter((i) => i.kind === 'assistant');
    expect(assistantItems).toHaveLength(1);
    const a = assistantItems[0];
    if (a?.kind !== 'assistant') throw new Error('expected assistant');
    // CF-418 + CF-471: parse layer normalizes to canonical TokenUsage.
    // input=1000 - cached=0 → uncached=1000; output passes the wire value
    // through (reasoning is a subset of output_tokens, never added).
    expect(a.usage).toEqual({
      input: 1000,
      output: 500,
      cacheWrite: 0,
      cacheRead: 0,
    });
  });

  it('attaches usage to the most-recent assistant of ANY phase when no final exists yet', () => {
    // First API call ends with commentary + a function_call_output, then a
    // token_count event. That commentary should carry the usage.
    const jsonl = [
      '{"timestamp":"2026-05-13T01:00:00Z","type":"session_meta","payload":{"model":"gpt-5"}}',
      '{"timestamp":"2026-05-13T01:00:01Z","type":"response_item","payload":{"type":"message","role":"assistant","phase":"commentary","content":[{"type":"output_text","text":"thinking out loud"}]}}',
      `{"timestamp":"2026-05-13T01:00:02Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":${USAGE_1000_500}}}}`,
    ].join('\n');
    const result = items(jsonl);
    const a = result.find((i) => i.kind === 'assistant');
    if (a?.kind !== 'assistant') throw new Error('expected assistant');
    expect(a.phase).toBe('commentary');
    // CF-418: canonical `.input` is uncached (1000 - 0 = 1000).
    expect(a.usage?.input).toBe(1000);
  });

  it('gives each multi-call assistant item its own usage (commentary then final)', () => {
    const jsonl = [
      '{"timestamp":"2026-05-13T01:00:00Z","type":"session_meta","payload":{"model":"gpt-5"}}',
      '{"timestamp":"2026-05-13T01:00:01Z","type":"response_item","payload":{"type":"message","role":"assistant","phase":"commentary","content":[{"type":"output_text","text":"first call"}]}}',
      `{"timestamp":"2026-05-13T01:00:02Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":${USAGE_1000_500}}}}`,
      '{"timestamp":"2026-05-13T01:00:03Z","type":"response_item","payload":{"type":"message","role":"assistant","phase":"final","content":[{"type":"output_text","text":"second call"}]}}',
      `{"timestamp":"2026-05-13T01:00:04Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":${USAGE_2000_700}}}}`,
    ].join('\n');
    const result = items(jsonl);
    const [first, second] = result.filter((i) => i.kind === 'assistant');
    if (first?.kind !== 'assistant' || second?.kind !== 'assistant') {
      throw new Error('expected two assistant items');
    }
    expect(first.phase).toBe('commentary');
    // CF-418 + CF-471: canonical `.input` is uncached; `.cacheRead` is the
    // cache hit. `.output` is the wire output_tokens; reasoning is a subset.
    // first: 1000 - 0 = 1000 uncached, 0 cacheRead, output 500.
    expect(first.usage?.input).toBe(1000);
    expect(second.phase).toBe('final');
    // second: 2000 - 300 = 1700 uncached, 300 cacheRead, output 700.
    expect(second.usage?.input).toBe(1700);
    expect(second.usage?.cacheRead).toBe(300);
    expect(second.usage?.output).toBe(700);
  });

  it('is a no-op when token_count arrives before any assistant message', () => {
    const jsonl = [
      '{"timestamp":"2026-05-13T01:00:00Z","type":"session_meta","payload":{"model":"gpt-5"}}',
      `{"timestamp":"2026-05-13T01:00:01Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":${USAGE_1000_500}}}}`,
      '{"timestamp":"2026-05-13T01:00:02Z","type":"response_item","payload":{"type":"message","role":"assistant","phase":"final","content":[{"type":"output_text","text":"hi"}]}}',
    ].join('\n');
    // Must not throw; later assistant must NOT inherit the orphan usage.
    const result = items(jsonl);
    const a = result.find((i) => i.kind === 'assistant');
    if (a?.kind !== 'assistant') throw new Error('expected assistant');
    expect(a.usage).toBeUndefined();
  });

  it('is a no-op when info or last_token_usage is null/missing', () => {
    const jsonl = [
      '{"timestamp":"2026-05-13T01:00:00Z","type":"session_meta","payload":{"model":"gpt-5"}}',
      '{"timestamp":"2026-05-13T01:00:01Z","type":"response_item","payload":{"type":"message","role":"assistant","phase":"final","content":[{"type":"output_text","text":"hi"}]}}',
      '{"timestamp":"2026-05-13T01:00:02Z","type":"event_msg","payload":{"type":"token_count","info":null}}',
      `{"timestamp":"2026-05-13T01:00:03Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":${USAGE_1000_500}}}}`,
    ].join('\n');
    const result = items(jsonl);
    const a = result.find((i) => i.kind === 'assistant');
    if (a?.kind !== 'assistant') throw new Error('expected assistant');
    expect(a.usage).toBeUndefined();
  });

  it('does NOT emit a render item for the token_count event itself', () => {
    // token_count is a side-channel — it should never produce a row.
    const jsonl = [
      '{"timestamp":"2026-05-13T01:00:00Z","type":"session_meta","payload":{"model":"gpt-5"}}',
      '{"timestamp":"2026-05-13T01:00:01Z","type":"response_item","payload":{"type":"message","role":"assistant","phase":"final","content":[{"type":"output_text","text":"hi"}]}}',
      `{"timestamp":"2026-05-13T01:00:02Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":${USAGE_1000_500}}}}`,
    ].join('\n');
    const result = items(jsonl);
    // Only the assistant item; no unknown / extra row from the token_count.
    expect(result.map((i) => i.kind)).toEqual(['assistant']);
  });
});

// ---------------------------------------------------------------------------
// CF-471 regression — `reasoning_output_tokens` is a SUBSET of `output_tokens`
// on the OpenAI wire, not an additive bucket. `applyTokenUsageToLastAssistant`
// must surface the wire `output_tokens` unchanged on the canonical TokenUsage
// and never add reasoning to it. The raw reasoning count is preserved on the
// assistant item (`reasoningTokens`) so the cost tooltip can show it as a
// parenthetical sub-line.
// ---------------------------------------------------------------------------

describe('normalizeCodexLines — reasoning is informational, never additive (CF-471)', () => {
  // One user → one assistant, then a single token_count event whose usage
  // JSON varies per case. All four cases share this scaffold, so we build
  // the JSONL through a helper rather than repeating the boilerplate.
  const ASSISTANT_LINES = [
    '{"timestamp":"2026-05-13T01:00:00Z","type":"session_meta","payload":{"model":"gpt-5"}}',
    '{"timestamp":"2026-05-13T01:00:01Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}}',
    '{"timestamp":"2026-05-13T01:00:02Z","type":"response_item","payload":{"type":"message","role":"assistant","phase":"final","content":[{"type":"output_text","text":"hello"}]}}',
  ] as const;

  function buildJsonl(usage: {
    input: number;
    cached: number;
    output: number;
    reasoning: number;
  }): string {
    const total = usage.input + usage.output;
    const usageJSON = `{"input_tokens":${usage.input},"cached_input_tokens":${usage.cached},"output_tokens":${usage.output},"reasoning_output_tokens":${usage.reasoning},"total_tokens":${total}}`;
    return [
      ...ASSISTANT_LINES,
      `{"timestamp":"2026-05-13T01:00:03Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":${usageJSON},"total_token_usage":${usageJSON}}}}`,
    ].join('\n');
  }

  function assistantFrom(jsonl: string) {
    const a = items(jsonl).find((i) => i.kind === 'assistant');
    if (a?.kind !== 'assistant') throw new Error('expected assistant');
    return a;
  }

  it.each([0, 1, 100, 999_999])(
    'usage.output equals wire output_tokens when reasoning_output_tokens=%d',
    (reasoning) => {
      const wireOutput = 500;
      const a = assistantFrom(
        buildJsonl({ input: 1000, cached: 0, output: wireOutput, reasoning }),
      );
      expect(a.usage?.output).toBe(wireOutput);
    },
  );

  it('preserves the raw reasoning count on the assistant item even though output is unchanged', () => {
    const a = assistantFrom(
      buildJsonl({ input: 1000, cached: 0, output: 500, reasoning: 120 }),
    );
    // Output is the wire value, untouched.
    expect(a.usage?.output).toBe(500);
    // Reasoning is informational, stamped separately for the tooltip.
    expect(a.reasoningTokens).toBe(120);
  });
});

// ---------------------------------------------------------------------------
// reportCodexTranscriptErrors
// ---------------------------------------------------------------------------
// Mirrors transcriptService.test.ts's reportTranscriptErrors block. Locks the
// fire-and-forget POST contract to /api/v1/client-errors under the
// `codex_transcript_validation` category so the two providers stay
// triageable independently in observability tooling.

describe('reportCodexTranscriptErrors', () => {
  let fetchSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    _resetReportedCodexSessions();
    fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('{"status":"ok"}'));
  });

  afterEach(() => {
    fetchSpy.mockRestore();
  });

  /** Extract the parsed JSON body from a fetch spy call */
  const parseFetchBody = (spy: ReturnType<typeof vi.spyOn>, callIndex = 0) =>
    JSON.parse(String(spy.mock.calls[callIndex]![1]?.body ?? ''));

  const makeError = (line: number, messageType?: string): TranscriptValidationError => ({
    line,
    rawJson: `{"type":"${messageType ?? 'unknown'}","bad":"data"}`,
    messageType,
    errors: [
      { path: 'payload.type', message: 'Invalid type', expected: 'message', received: 'new_type' },
    ],
  });

  it('sends errors to the backend with correct payload structure', () => {
    const errors = [makeError(42, 'response_item')];
    reportCodexTranscriptErrors('session-abc', errors);

    expect(fetchSpy).toHaveBeenCalledOnce();
    const [url, options] = fetchSpy.mock.calls[0]!;
    expect(url).toBe('/api/v1/client-errors');
    expect(options?.method).toBe('POST');
    expect(options?.credentials).toBe('include');

    const body = parseFetchBody(fetchSpy);
    expect(body.category).toBe('codex_transcript_validation');
    expect(body.session_id).toBe('session-abc');
    expect(body.errors).toHaveLength(1);
    expect(body.errors[0].line).toBe(42);
    expect(body.errors[0].message_type).toBe('response_item');
    expect(body.errors[0].details).toHaveLength(1);
    expect(body.errors[0].details[0].path).toBe('payload.type');
    expect(body.errors[0].details[0].expected).toBe('message');
    expect(body.errors[0].details[0].received).toBe('new_type');
  });

  it('truncates raw_json_preview to 500 chars', () => {
    const longJson = 'x'.repeat(1000);
    const errors: TranscriptValidationError[] = [{
      line: 1,
      rawJson: longJson,
      errors: [{ path: 'root', message: 'bad' }],
    }];

    reportCodexTranscriptErrors('session-long', errors);

    const body = parseFetchBody(fetchSpy);
    expect(body.errors[0].raw_json_preview).toHaveLength(500);
  });

  it('limits to 50 errors per report', () => {
    const errors = Array.from({ length: 100 }, (_, i) => makeError(i + 1));
    reportCodexTranscriptErrors('session-many', errors);

    const body = parseFetchBody(fetchSpy);
    expect(body.errors).toHaveLength(50);
  });

  it('silently ignores fetch failures', () => {
    fetchSpy.mockRejectedValue(new Error('Network error'));

    // Should not throw
    expect(() => reportCodexTranscriptErrors('session-fail', [makeError(1)])).not.toThrow();
  });
});
