// Schema tests for Codex rollout JSONL.
//
// Validates that every documented `type` and nested `payload.type` parses
// cleanly, that unknowns fall through to forward-compat catch-alls, and that
// extra fields survive `.passthrough()`.

import { describe, it, expect } from 'vitest';
import { RawCodexLineSchema } from './codexTranscript';

function parse(input: unknown) {
  return RawCodexLineSchema.safeParse(input);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object';
}

/** Read `value[parent][key]` as a string without `as` casts. */
function readNestedString(value: unknown, parent: string, key: string): string | undefined {
  if (!isRecord(value)) return undefined;
  const inner = value[parent];
  if (!isRecord(inner)) return undefined;
  const v = inner[key];
  return typeof v === 'string' ? v : undefined;
}

describe('RawCodexLineSchema', () => {
  describe('top-level types', () => {
    it('accepts session_meta', () => {
      const result = parse({
        timestamp: '2026-05-13T01:00:00Z',
        type: 'session_meta',
        payload: {
          id: '019e-fixture',
          cwd: '/tmp/proj',
          originator: 'codex-tui',
          model_provider: 'openai',
          model: 'gpt-5',
        },
      });
      expect(result.success).toBe(true);
    });

    it('accepts turn_context', () => {
      const result = parse({
        timestamp: '2026-05-13T01:00:00Z',
        type: 'turn_context',
        payload: { turn_id: 't1', cwd: '/tmp/proj', approval_policy: 'on-request' },
      });
      expect(result.success).toBe(true);
    });

    it('accepts compacted', () => {
      const result = parse({
        timestamp: '2026-05-13T01:00:00Z',
        type: 'compacted',
        payload: { message: '', replacement_history: [] },
      });
      expect(result.success).toBe(true);
    });

    it('accepts unknown top-level type via catch-all', () => {
      const result = parse({
        timestamp: '2026-05-13T01:00:00Z',
        type: 'future_top_level_type',
        payload: { anything: 'goes' },
      });
      expect(result.success).toBe(true);
    });
  });

  describe('response_item payload variants', () => {
    function responseItem(payload: unknown) {
      return parse({
        timestamp: '2026-05-13T01:00:00Z',
        type: 'response_item',
        payload,
      });
    }

    it('accepts message[role=user]', () => {
      const result = responseItem({
        type: 'message',
        role: 'user',
        content: [{ type: 'input_text', text: 'hello' }],
      });
      expect(result.success).toBe(true);
    });

    it('accepts message[role=assistant] with phase commentary', () => {
      const result = responseItem({
        type: 'message',
        role: 'assistant',
        content: [{ type: 'output_text', text: 'thinking out loud' }],
        phase: 'commentary',
      });
      expect(result.success).toBe(true);
    });

    it('accepts message[role=assistant] with phase final', () => {
      const result = responseItem({
        type: 'message',
        role: 'assistant',
        content: [{ type: 'output_text', text: 'final answer' }],
        phase: 'final',
      });
      expect(result.success).toBe(true);
    });

    it('accepts message[role=developer]', () => {
      const result = responseItem({
        type: 'message',
        role: 'developer',
        content: [{ type: 'input_text', text: 'sandbox instructions' }],
      });
      expect(result.success).toBe(true);
    });

    it('accepts function_call', () => {
      const result = responseItem({
        type: 'function_call',
        name: 'exec_command',
        arguments: '{"cmd":"pwd"}',
        call_id: 'call_abc',
      });
      expect(result.success).toBe(true);
    });

    it('accepts function_call_output', () => {
      const result = responseItem({
        type: 'function_call_output',
        call_id: 'call_abc',
        output: 'Output:\n/tmp/proj',
      });
      expect(result.success).toBe(true);
    });

    it('accepts custom_tool_call (apply_patch)', () => {
      const result = responseItem({
        type: 'custom_tool_call',
        status: 'completed',
        call_id: 'call_patch',
        name: 'apply_patch',
        input: '*** Begin Patch\n*** End Patch',
      });
      expect(result.success).toBe(true);
    });

    it('accepts custom_tool_call_output', () => {
      const result = responseItem({
        type: 'custom_tool_call_output',
        call_id: 'call_patch',
        output: '{"output":"ok"}',
      });
      expect(result.success).toBe(true);
    });

    it('accepts reasoning (encrypted)', () => {
      const result = responseItem({
        type: 'reasoning',
        summary: [],
        content: null,
        encrypted_content: 'opaque-blob',
      });
      expect(result.success).toBe(true);
    });

    it('accepts web_search_call', () => {
      const result = responseItem({
        type: 'web_search_call',
        status: 'completed',
        action: {
          type: 'search',
          query: 'codex jsonl',
          queries: ['codex jsonl', 'rollout schema'],
        },
      });
      expect(result.success).toBe(true);
    });

    it('accepts unknown response_item payload.type via catch-all', () => {
      const result = responseItem({
        type: 'future_response_item_payload',
        anything: 'goes',
      });
      expect(result.success).toBe(true);
    });
  });

  describe('event_msg payload variants', () => {
    function eventMsg(payload: unknown) {
      return parse({
        timestamp: '2026-05-13T01:00:00Z',
        type: 'event_msg',
        payload,
      });
    }

    it('accepts user_message', () => {
      const result = eventMsg({ type: 'user_message', message: 'hello', images: [] });
      expect(result.success).toBe(true);
    });

    it('accepts agent_message', () => {
      const result = eventMsg({
        type: 'agent_message',
        message: 'reply',
        phase: 'commentary',
      });
      expect(result.success).toBe(true);
    });

    it('accepts task_started', () => {
      const result = eventMsg({
        type: 'task_started',
        turn_id: 't1',
        started_at: 1778634000,
        model: 'gpt-5',
      });
      expect(result.success).toBe(true);
    });

    it('accepts task_complete', () => {
      const result = eventMsg({
        type: 'task_complete',
        turn_id: 't1',
        last_agent_message: 'done',
        completed_at: 1778634011,
        duration_ms: 11000,
        time_to_first_token_ms: 1700,
      });
      expect(result.success).toBe(true);
    });

    it('accepts token_count', () => {
      const result = eventMsg({
        type: 'token_count',
        info: null,
        rate_limits: { limit_id: 'codex' },
      });
      expect(result.success).toBe(true);
    });

    it('accepts patch_apply_end', () => {
      const result = eventMsg({
        type: 'patch_apply_end',
        call_id: 'call_patch',
        turn_id: 't1',
        stdout: 'Success.',
        stderr: '',
        success: true,
        changes: {},
      });
      expect(result.success).toBe(true);
    });

    it('accepts unknown event_msg payload.type via catch-all', () => {
      const result = eventMsg({ type: 'future_event_msg_payload', any: 'thing' });
      expect(result.success).toBe(true);
    });
  });

  describe('forward compatibility', () => {
    it('preserves extra fields on session_meta via passthrough', () => {
      const input = {
        timestamp: '2026-05-13T01:00:00Z',
        type: 'session_meta',
        payload: { id: 'x', cwd: '/x', future_field: 'preserved' },
      };
      const result = parse(input);
      expect(result.success).toBe(true);
      if (result.success) {
        // The exact field name on the parsed object should still be present.
        expect(readNestedString(result.data, 'payload', 'future_field')).toBe('preserved');
      }
    });

    it('preserves extra fields on response_item.message via passthrough', () => {
      const input = {
        timestamp: '2026-05-13T01:00:00Z',
        type: 'response_item',
        payload: {
          type: 'message',
          role: 'assistant',
          content: [{ type: 'output_text', text: 'hi' }],
          future_phase: 'reasoning',
        },
      };
      const result = parse(input);
      expect(result.success).toBe(true);
      if (result.success) {
        expect(readNestedString(result.data, 'payload', 'future_phase')).toBe('reasoning');
      }
    });
  });
});
