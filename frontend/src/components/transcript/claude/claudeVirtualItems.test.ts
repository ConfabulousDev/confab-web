// 6h7m: unit tests for the Claude virtual-item builder, extracted from
// ClaudeMessageTimeline.tsx. Mirrors CodexMessageTimeline.test.tsx's
// `buildVirtualItems` coverage. Fixtures for day-boundary cases use
// local-time `Date` construction (not hardcoded 'Z' literals) so the tests
// are deterministic regardless of the runner's timezone.

import { describe, it, expect } from 'vitest';
import type { AssistantMessage, TranscriptLine } from '@/types';
import { buildVirtualItems } from './claudeVirtualItems';

function assistantAt(timestamp: string, uuid = 'a1'): AssistantMessage {
  return {
    type: 'assistant',
    uuid,
    timestamp,
    parentUuid: null,
    isSidechain: false,
    userType: 'external',
    cwd: '/test',
    sessionId: 'session-1',
    version: '1.0.0',
    requestId: 'req-1',
    message: {
      model: 'claude-sonnet-4-20250514',
      id: 'msg-1',
      type: 'message',
      role: 'assistant',
      content: [{ type: 'text', text: 'hello' }],
      stop_reason: 'end_turn',
      stop_sequence: null,
      usage: {
        input_tokens: 10,
        output_tokens: 10,
        cache_creation_input_tokens: 0,
        cache_read_input_tokens: 0,
      },
    },
  };
}

// `summary` lines carry no timestamp field at all (SummaryMessageSchema).
function summaryLine(leafUuid = 'leaf-1'): TranscriptLine {
  return { type: 'summary', summary: 'a summary', leafUuid };
}

function buildAndAllIndex(messages: TranscriptLine[]) {
  const messageToAllIndex = new Map<TranscriptLine, number>();
  messages.forEach((m, i) => messageToAllIndex.set(m, i));
  return buildVirtualItems(messages, messageToAllIndex);
}

describe('buildVirtualItems (Claude)', () => {
  describe('time-gap separator', () => {
    it('injects a separator entry between messages >5min apart', () => {
      const messages = [assistantAt('2026-05-13T18:00:00Z', 'a'), assistantAt('2026-05-13T18:06:00Z', 'b')];
      const result = buildAndAllIndex(messages);
      expect(result).toHaveLength(3);
      expect(result[0]?.type).toBe('message');
      expect(result[1]?.type).toBe('separator');
      expect(result[2]?.type).toBe('message');
    });

    it('does not inject a separator for messages <=5min apart', () => {
      const messages = [assistantAt('2026-05-13T18:00:00Z', 'a'), assistantAt('2026-05-13T18:04:59Z', 'b')];
      const result = buildAndAllIndex(messages);
      expect(result).toHaveLength(2);
      expect(result.every((v) => v.type === 'message')).toBe(true);
    });

    it('does not inject a separator before the first message', () => {
      const result = buildAndAllIndex([assistantAt('2026-05-13T18:00:00Z', 'a')]);
      expect(result).toHaveLength(1);
      expect(result[0]?.type).toBe('message');
    });
  });

  describe('day-boundary divider', () => {
    it('injects a separator across a calendar-day change even with a <5min gap', () => {
      const messages = [
        assistantAt(new Date(2026, 4, 13, 23, 59, 0).toISOString(), 'a'),
        assistantAt(new Date(2026, 4, 14, 0, 1, 0).toISOString(), 'b'),
      ];
      const result = buildAndAllIndex(messages);
      expect(result).toHaveLength(3);
      expect(result[1]?.type).toBe('separator');
    });

    it('does not inject a separator for a same-day gap under 5min, even late at night', () => {
      const messages = [
        assistantAt(new Date(2026, 4, 13, 23, 50, 0).toISOString(), 'a'),
        assistantAt(new Date(2026, 4, 13, 23, 54, 0).toISOString(), 'b'),
      ];
      const result = buildAndAllIndex(messages);
      expect(result).toHaveLength(2);
    });
  });

  describe('last-known-timestamp fix (Decision 7)', () => {
    it('still detects a day-boundary crossing a timestamp-less summary line', () => {
      const messages = [
        assistantAt(new Date(2026, 4, 13, 23, 59, 0).toISOString(), 'a'),
        summaryLine(),
        assistantAt(new Date(2026, 4, 14, 0, 1, 0).toISOString(), 'b'),
      ];
      const result = buildAndAllIndex(messages);
      // Layout: message(a), message(summary), separator, message(b)
      expect(result.map((v) => v.type)).toEqual(['message', 'message', 'separator', 'message']);
    });

    it('does not crash on a summary line and does not treat it as a timestamp boundary', () => {
      const messages = [summaryLine(), assistantAt('2026-05-13T18:00:00Z', 'a')];
      const result = buildAndAllIndex(messages);
      expect(result.map((v) => v.type)).toEqual(['message', 'message']);
    });
  });
});
