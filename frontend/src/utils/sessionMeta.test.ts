import { describe, it, expect } from 'vitest';
import { computeSessionMeta } from './sessionMeta';
import type { TranscriptLine, UserMessage, AssistantMessage } from '@/types';

// Helper to create a minimal user message with a timestamp
function createUserMessage(timestamp: string, uuid = 'test-uuid'): UserMessage {
  return {
    type: 'user',
    uuid,
    timestamp,
    parentUuid: null,
    isSidechain: false,
    userType: 'human',
    cwd: '/test',
    sessionId: 'test-session',
    version: '1.0',
    message: {
      role: 'user',
      content: 'test message',
    },
  };
}

// Helper to create a minimal assistant message with a timestamp
function createAssistantMessage(timestamp: string, uuid = 'test-uuid'): AssistantMessage {
  return {
    type: 'assistant',
    uuid,
    timestamp,
    parentUuid: null,
    isSidechain: false,
    userType: 'human',
    cwd: '/test',
    sessionId: 'test-session',
    version: '1.0',
    requestId: 'req-123',
    message: {
      model: 'claude-sonnet-4-20250514',
      id: 'msg-123',
      type: 'message',
      role: 'assistant',
      content: [],
      stop_reason: 'end_turn',
      stop_sequence: null,
      usage: {
        input_tokens: 100,
        output_tokens: 50,
        cache_creation_input_tokens: 0,
        cache_read_input_tokens: 0,
      },
    },
  };
}

describe('computeSessionMeta', () => {
  describe('duration calculation from messages', () => {
    it('computes duration from first to last message timestamp', () => {
      const messages: TranscriptLine[] = [
        createUserMessage('2024-01-15T10:00:00Z'),
        createAssistantMessage('2024-01-15T10:05:00Z'),
        createUserMessage('2024-01-15T10:10:00Z'),
        createAssistantMessage('2024-01-15T10:15:00Z'),
      ];

      const result = computeSessionMeta(messages, {});

      // 15 minutes = 900,000 ms
      expect(result.durationMs).toBe(15 * 60 * 1000);
    });

    it('handles messages in non-chronological order', () => {
      // Messages might not be in timestamp order in the array
      const messages: TranscriptLine[] = [
        createUserMessage('2024-01-15T10:10:00Z'), // middle
        createAssistantMessage('2024-01-15T10:00:00Z'), // earliest
        createUserMessage('2024-01-15T10:20:00Z'), // latest
      ];

      const result = computeSessionMeta(messages, {});

      // Should find min and max correctly regardless of order
      expect(result.durationMs).toBe(20 * 60 * 1000);
      expect(result.firstTimestamp).toBe(new Date('2024-01-15T10:00:00Z').getTime());
      expect(result.lastTimestamp).toBe(new Date('2024-01-15T10:20:00Z').getTime());
    });

    it('returns undefined duration for single message', () => {
      const messages: TranscriptLine[] = [
        createUserMessage('2024-01-15T10:00:00Z'),
      ];

      const result = computeSessionMeta(messages, {});

      // Can't compute duration with just one timestamp
      expect(result.durationMs).toBeUndefined();
    });

    it('returns undefined duration for empty messages array', () => {
      const result = computeSessionMeta([], {});

      expect(result.durationMs).toBeUndefined();
      expect(result.firstTimestamp).toBeUndefined();
      expect(result.lastTimestamp).toBeUndefined();
    });

    it('ignores non-user/assistant message types', () => {
      // Create a summary message inline (not user/assistant, so won't have timestamp used)
      const summaryMessage: TranscriptLine = {
        type: 'summary',
        summary: 'test summary',
        leafUuid: 'leaf-123',
      };

      const messages: TranscriptLine[] = [
        createUserMessage('2024-01-15T10:00:00Z'),
        summaryMessage,
        createAssistantMessage('2024-01-15T10:30:00Z'),
      ];

      const result = computeSessionMeta(messages, {});

      // Should only use user/assistant timestamps
      expect(result.durationMs).toBe(30 * 60 * 1000);
    });
  });

  describe('fallback to session metadata', () => {
    it('falls back to session timestamps when no messages', () => {
      const result = computeSessionMeta([], {
        firstSeen: '2024-01-15T09:00:00Z',
        lastSyncAt: '2024-01-15T17:00:00Z',
      });

      // 8 hours = 28,800,000 ms
      expect(result.durationMs).toBe(8 * 60 * 60 * 1000);
    });

    it('prefers message timestamps over session metadata', () => {
      const messages: TranscriptLine[] = [
        createUserMessage('2024-01-15T10:00:00Z'),
        createAssistantMessage('2024-01-15T18:00:00Z'), // 8 hours span
      ];

      const result = computeSessionMeta(messages, {
        // Session metadata suggests only 1 hour (should be ignored)
        firstSeen: '2024-01-15T17:00:00Z',
        lastSyncAt: '2024-01-15T18:00:00Z',
      });

      // Should use message timestamps (8 hours), not session metadata (1 hour)
      expect(result.durationMs).toBe(8 * 60 * 60 * 1000);
    });

    it('handles missing lastSyncAt in session metadata', () => {
      const result = computeSessionMeta([], {
        firstSeen: '2024-01-15T09:00:00Z',
        lastSyncAt: null,
      });

      expect(result.durationMs).toBeUndefined();
    });
  });

  describe('sessionDate calculation', () => {
    it('uses earliest message timestamp for sessionDate', () => {
      const messages: TranscriptLine[] = [
        createUserMessage('2024-01-15T10:30:00Z'), // not the earliest
        createAssistantMessage('2024-01-15T10:00:00Z'), // earliest
        createUserMessage('2024-01-15T11:00:00Z'),
      ];

      const result = computeSessionMeta(messages, {});

      expect(result.sessionDate?.toISOString()).toBe('2024-01-15T10:00:00.000Z');
    });

    it('falls back to session.firstSeen for sessionDate when no messages', () => {
      const result = computeSessionMeta([], {
        firstSeen: '2024-01-15T09:00:00Z',
      });

      expect(result.sessionDate?.toISOString()).toBe('2024-01-15T09:00:00.000Z');
    });

    it('returns undefined sessionDate when no data available', () => {
      const result = computeSessionMeta([], {});

      expect(result.sessionDate).toBeUndefined();
    });
  });

  describe('resumed session scenario', () => {
    it('computes correct duration for resumed sessions with full transcript history', () => {
      // Scenario: User started session yesterday, resumed today
      // Session metadata might only reflect the resume time,
      // but transcript has the full history
      const messages: TranscriptLine[] = [
        // Original session - yesterday
        createUserMessage('2024-01-14T10:00:00Z'),
        createAssistantMessage('2024-01-14T10:05:00Z'),
        createUserMessage('2024-01-14T18:00:00Z'),
        createAssistantMessage('2024-01-14T18:30:00Z'),
        // Resumed today
        createUserMessage('2024-01-15T09:00:00Z'),
        createAssistantMessage('2024-01-15T09:05:00Z'),
      ];

      const result = computeSessionMeta(messages, {
        // Session metadata reflects resume time (wrong for duration)
        firstSeen: '2024-01-15T09:00:00Z',
        lastSyncAt: '2024-01-15T09:05:00Z',
      });

      // Duration should span from yesterday 10:00 to today 09:05
      // = 23 hours and 5 minutes = 83,100,000 ms
      const expectedMs = (23 * 60 + 5) * 60 * 1000;
      expect(result.durationMs).toBe(expectedMs);

      // Session date should be the original start
      expect(result.sessionDate?.toISOString()).toBe('2024-01-14T10:00:00.000Z');
    });
  });
});
