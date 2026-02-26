import { describe, it, expect } from 'vitest';
import { validateParsedTranscriptLine } from '@/schemas/transcript';
import { countHierarchicalCategories, messageMatchesFilter, DEFAULT_FILTER_STATE } from './messageCategories';
import type { FilterState } from './messageCategories';

function parseLine(obj: unknown) {
  const result = validateParsedTranscriptLine(obj, JSON.stringify(obj), 0);
  if (!result.success) throw new Error(`Validation failed: ${JSON.stringify(result.error)}`);
  return result.data;
}

const prLinkMessage = parseLine({
  type: 'pr-link',
  prNumber: 22,
  prRepository: 'ConfabulousDev/confab-web',
  prUrl: 'https://github.com/ConfabulousDev/confab-web/pull/22',
  sessionId: 'session-123',
  timestamp: '2026-02-22T08:00:41.865Z',
});

describe('messageCategories', () => {
  describe('countHierarchicalCategories', () => {
    it('counts pr-link messages', () => {
      const counts = countHierarchicalCategories([prLinkMessage]);
      expect(counts['pr-link']).toBe(1);
    });

    it('counts pr-link alongside other types', () => {
      const queueOp = parseLine({
        type: 'queue-operation',
        operation: 'enqueue',
        timestamp: '2026-02-22T08:00:00Z',
        sessionId: 'session-123',
      });

      const counts = countHierarchicalCategories([prLinkMessage, queueOp]);
      expect(counts['pr-link']).toBe(1);
      expect(counts['queue-operation']).toBe(1);
    });
  });

  describe('messageMatchesFilter', () => {
    it('hides pr-link by default', () => {
      expect(messageMatchesFilter(prLinkMessage, DEFAULT_FILTER_STATE)).toBe(false);
    });

    it('shows pr-link when filter is enabled', () => {
      const filterState: FilterState = {
        ...DEFAULT_FILTER_STATE,
        'pr-link': true,
      };
      expect(messageMatchesFilter(prLinkMessage, filterState)).toBe(true);
    });

    it('hides system messages by default (deep-link filter reset scenario)', () => {
      const systemMessage = parseLine({
        type: 'system',
        uuid: 'sys-uuid-1',
        timestamp: '2026-02-22T08:00:00Z',
        parentUuid: null,
        isSidechain: false,
        userType: 'external',
        cwd: '/test',
        sessionId: 'session-123',
        version: '1.0.0',
        subtype: 'info',
        content: 'System info message',
      });
      expect(messageMatchesFilter(systemMessage, DEFAULT_FILTER_STATE)).toBe(false);
    });

    it('shows system messages after filter reset with system enabled', () => {
      const systemMessage = parseLine({
        type: 'system',
        uuid: 'sys-uuid-1',
        timestamp: '2026-02-22T08:00:00Z',
        parentUuid: null,
        isSidechain: false,
        userType: 'external',
        cwd: '/test',
        sessionId: 'session-123',
        version: '1.0.0',
        subtype: 'info',
        content: 'System info message',
      });
      const resetState: FilterState = { ...DEFAULT_FILTER_STATE, system: true };
      expect(messageMatchesFilter(systemMessage, resetState)).toBe(true);
    });

    it('shows user messages with DEFAULT_FILTER_STATE (deep-link targets visible after reset)', () => {
      const userMessage = parseLine({
        type: 'user',
        uuid: 'user-uuid-1',
        timestamp: '2026-02-22T08:00:00Z',
        parentUuid: null,
        isSidechain: false,
        userType: 'external',
        cwd: '/test',
        sessionId: 'session-123',
        version: '1.0.0',
        message: { role: 'user', content: 'Hello' },
      });
      expect(messageMatchesFilter(userMessage, DEFAULT_FILTER_STATE)).toBe(true);
    });

    it('shows assistant messages with DEFAULT_FILTER_STATE (deep-link targets visible after reset)', () => {
      const assistantMessage = parseLine({
        type: 'assistant',
        uuid: 'asst-uuid-1',
        timestamp: '2026-02-22T08:00:00Z',
        parentUuid: null,
        isSidechain: false,
        userType: 'external',
        cwd: '/test',
        sessionId: 'session-123',
        version: '1.0.0',
        requestId: 'req-1',
        message: {
          model: 'claude-sonnet-4-20250514',
          id: 'msg-1',
          type: 'message',
          role: 'assistant',
          content: [{ type: 'text', text: 'Hello!' }],
          stop_reason: 'end_turn',
          stop_sequence: null,
          usage: { input_tokens: 100, output_tokens: 50 },
        },
      });
      expect(messageMatchesFilter(assistantMessage, DEFAULT_FILTER_STATE)).toBe(true);
    });
  });
});
