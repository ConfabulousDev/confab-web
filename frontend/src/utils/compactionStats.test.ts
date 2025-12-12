import { describe, it, expect } from 'vitest';
import { calculateCompactionStats, formatResponseTime } from './compactionStats';
import type { TranscriptLine, SystemMessage, AssistantMessage, UserMessage } from '@/types';

// Helper to create a compact_boundary system message
function createCompactBoundary(
  uuid: string,
  trigger: 'auto' | 'manual' | string | undefined,
  options: { preTokens?: number; timestamp?: string; logicalParentUuid?: string } = {}
): SystemMessage {
  const { preTokens = 100000, timestamp, logicalParentUuid } = options;
  const base: SystemMessage = {
    type: 'system',
    uuid,
    timestamp: timestamp ?? new Date().toISOString(),
    parentUuid: null,
    isSidechain: false,
    userType: 'external',
    cwd: '/test',
    sessionId: 'test-session',
    version: '1.0.0',
    subtype: 'compact_boundary',
    content: 'Conversation compacted',
    isMeta: false,
    level: 'info',
  };

  if (logicalParentUuid) {
    base.logicalParentUuid = logicalParentUuid;
  }

  if (trigger !== undefined) {
    base.compactMetadata = { trigger, preTokens };
  }

  return base;
}

// Helper to create a non-compaction system message (e.g., api_error)
function createSystemMessage(uuid: string, subtype: string): SystemMessage {
  return {
    type: 'system',
    uuid,
    timestamp: new Date().toISOString(),
    parentUuid: null,
    isSidechain: false,
    userType: 'external',
    cwd: '/test',
    sessionId: 'test-session',
    version: '1.0.0',
    subtype,
    content: 'Some system message',
    isMeta: false,
    level: 'info',
  };
}

// Helper to create an assistant message
function createAssistantMessage(uuid: string, timestamp?: string): AssistantMessage {
  return {
    type: 'assistant',
    uuid,
    timestamp: timestamp ?? new Date().toISOString(),
    parentUuid: null,
    isSidechain: false,
    userType: 'external',
    cwd: '/test',
    sessionId: 'test-session',
    version: '1.0.0',
    requestId: `req-${uuid}`,
    message: {
      model: 'claude-sonnet-4-20250514',
      id: `msg-${uuid}`,
      type: 'message',
      role: 'assistant',
      content: [{ type: 'text', text: 'Test response' }],
      stop_reason: 'end_turn',
      stop_sequence: null,
      usage: {
        input_tokens: 1000,
        output_tokens: 500,
      },
    },
  };
}

// Helper to create a user message
function createUserMessage(uuid: string, timestamp?: string): UserMessage {
  return {
    type: 'user',
    uuid,
    timestamp: timestamp ?? new Date().toISOString(),
    parentUuid: null,
    isSidechain: false,
    userType: 'external',
    cwd: '/test',
    sessionId: 'test-session',
    version: '1.0.0',
    message: {
      role: 'user',
      content: 'Test message',
    },
  };
}

describe('calculateCompactionStats', () => {
  it('returns zeros for empty messages array', () => {
    const result = calculateCompactionStats([]);
    expect(result).toEqual({ total: 0, auto: 0, manual: 0, avgCompactionTimeMs: null });
  });

  it('returns zeros when no compaction events exist', () => {
    const messages: TranscriptLine[] = [
      createUserMessage('1'),
      createAssistantMessage('2'),
      createUserMessage('3'),
      createAssistantMessage('4'),
    ];

    const result = calculateCompactionStats(messages);
    expect(result).toEqual({ total: 0, auto: 0, manual: 0, avgCompactionTimeMs: null });
  });

  it('counts a single auto compaction', () => {
    const messages: TranscriptLine[] = [
      createUserMessage('1'),
      createAssistantMessage('2'),
      createCompactBoundary('3', 'auto'),
      createAssistantMessage('4'),
    ];

    const result = calculateCompactionStats(messages);
    expect(result.total).toBe(1);
    expect(result.auto).toBe(1);
    expect(result.manual).toBe(0);
  });

  it('counts a single manual compaction', () => {
    const messages: TranscriptLine[] = [
      createUserMessage('1'),
      createAssistantMessage('2'),
      createCompactBoundary('3', 'manual'),
      createAssistantMessage('4'),
    ];

    const result = calculateCompactionStats(messages);
    expect(result.total).toBe(1);
    expect(result.auto).toBe(0);
    expect(result.manual).toBe(1);
  });

  it('counts mixed auto and manual compactions', () => {
    const messages: TranscriptLine[] = [
      createUserMessage('1'),
      createCompactBoundary('2', 'auto'),
      createAssistantMessage('3'),
      createCompactBoundary('4', 'manual'),
      createAssistantMessage('5'),
      createCompactBoundary('6', 'auto'),
      createAssistantMessage('7'),
    ];

    const result = calculateCompactionStats(messages);
    expect(result.total).toBe(3);
    expect(result.auto).toBe(2);
    expect(result.manual).toBe(1);
  });

  it('counts toward total when compactMetadata is missing', () => {
    const messages: TranscriptLine[] = [
      createCompactBoundary('1', undefined),
    ];

    const result = calculateCompactionStats(messages);
    expect(result).toEqual({ total: 1, auto: 0, manual: 0, avgCompactionTimeMs: null });
  });

  it('counts toward total when trigger has unknown value', () => {
    const messages: TranscriptLine[] = [
      createCompactBoundary('1', 'unknown_trigger'),
    ];

    const result = calculateCompactionStats(messages);
    expect(result).toEqual({ total: 1, auto: 0, manual: 0, avgCompactionTimeMs: null });
  });

  it('ignores system messages with different subtypes', () => {
    const messages: TranscriptLine[] = [
      createSystemMessage('1', 'api_error'),
      createSystemMessage('2', 'warning'),
      createCompactBoundary('3', 'auto'),
      createSystemMessage('4', 'api_error'),
    ];

    const result = calculateCompactionStats(messages);
    expect(result.total).toBe(1);
    expect(result.auto).toBe(1);
    expect(result.manual).toBe(0);
  });

  it('correctly counts compactions mixed with various message types', () => {
    const messages: TranscriptLine[] = [
      createUserMessage('1'),
      createAssistantMessage('2'),
      createSystemMessage('3', 'api_error'),
      createCompactBoundary('4', 'auto', { preTokens: 150000 }),
      createUserMessage('5'),
      createAssistantMessage('6'),
      createCompactBoundary('7', 'manual', { preTokens: 160000 }),
      createSystemMessage('8', 'api_error'),
      createAssistantMessage('9'),
    ];

    const result = calculateCompactionStats(messages);
    expect(result.total).toBe(2);
    expect(result.auto).toBe(1);
    expect(result.manual).toBe(1);
  });

  // Compaction time tests (logicalParent → compact_boundary)
  // Only auto compactions are included in timing; manual compactions include user think time
  it('calculates avgCompactionTimeMs for a single auto compaction', () => {
    const messages: TranscriptLine[] = [
      createAssistantMessage('parent-1', '2025-12-11T16:53:39.000Z'),
      createCompactBoundary('boundary-1', 'auto', {
        timestamp: '2025-12-11T16:54:33.000Z',
        logicalParentUuid: 'parent-1',
      }),
    ];

    const result = calculateCompactionStats(messages);
    // 16:54:33 - 16:53:39 = 54 seconds
    expect(result.avgCompactionTimeMs).toBe(54000);
  });

  it('calculates average across multiple auto compactions', () => {
    const messages: TranscriptLine[] = [
      createAssistantMessage('parent-1', '2025-12-11T16:00:00.000Z'),
      createCompactBoundary('boundary-1', 'auto', {
        timestamp: '2025-12-11T16:00:10.000Z', // 10 seconds
        logicalParentUuid: 'parent-1',
      }),
      createAssistantMessage('parent-2', '2025-12-11T17:00:00.000Z'),
      createCompactBoundary('boundary-2', 'auto', {
        timestamp: '2025-12-11T17:00:30.000Z', // 30 seconds
        logicalParentUuid: 'parent-2',
      }),
    ];

    const result = calculateCompactionStats(messages);
    expect(result.avgCompactionTimeMs).toBe(20000); // (10000 + 30000) / 2
  });

  it('excludes manual compactions from timing calculation', () => {
    const messages: TranscriptLine[] = [
      createAssistantMessage('parent-1', '2025-12-11T16:00:00.000Z'),
      createCompactBoundary('boundary-1', 'manual', {
        timestamp: '2025-12-11T16:10:00.000Z', // 10 minutes - user think time
        logicalParentUuid: 'parent-1',
      }),
    ];

    const result = calculateCompactionStats(messages);
    expect(result.total).toBe(1);
    expect(result.manual).toBe(1);
    expect(result.avgCompactionTimeMs).toBeNull(); // Manual excluded from timing
  });

  it('only includes auto compactions in timing average, not manual', () => {
    const messages: TranscriptLine[] = [
      createAssistantMessage('parent-1', '2025-12-11T16:00:00.000Z'),
      createCompactBoundary('boundary-1', 'auto', {
        timestamp: '2025-12-11T16:00:20.000Z', // 20 seconds
        logicalParentUuid: 'parent-1',
      }),
      createAssistantMessage('parent-2', '2025-12-11T17:00:00.000Z'),
      createCompactBoundary('boundary-2', 'manual', {
        timestamp: '2025-12-11T17:05:00.000Z', // 5 minutes - user think time, should be excluded
        logicalParentUuid: 'parent-2',
      }),
    ];

    const result = calculateCompactionStats(messages);
    expect(result.total).toBe(2);
    expect(result.auto).toBe(1);
    expect(result.manual).toBe(1);
    expect(result.avgCompactionTimeMs).toBe(20000); // Only the auto compaction's 20 seconds
  });

  it('returns null avgCompactionTimeMs when logicalParentUuid is missing', () => {
    const messages: TranscriptLine[] = [
      createCompactBoundary('boundary-1', 'auto', {
        timestamp: '2025-12-11T16:54:33.000Z',
        // no logicalParentUuid
      }),
    ];

    const result = calculateCompactionStats(messages);
    expect(result.avgCompactionTimeMs).toBeNull();
  });

  it('returns null avgCompactionTimeMs when logicalParent message not found', () => {
    const messages: TranscriptLine[] = [
      createCompactBoundary('boundary-1', 'auto', {
        timestamp: '2025-12-11T16:54:33.000Z',
        logicalParentUuid: 'nonexistent-uuid',
      }),
    ];

    const result = calculateCompactionStats(messages);
    expect(result.avgCompactionTimeMs).toBeNull();
  });

  it('only counts compactions with valid logicalParent in average', () => {
    const messages: TranscriptLine[] = [
      createAssistantMessage('parent-1', '2025-12-11T16:00:00.000Z'),
      createCompactBoundary('boundary-1', 'auto', {
        timestamp: '2025-12-11T16:00:20.000Z', // 20 seconds
        logicalParentUuid: 'parent-1',
      }),
      createCompactBoundary('boundary-2', 'auto', {
        timestamp: '2025-12-11T17:00:00.000Z',
        logicalParentUuid: 'nonexistent-uuid', // won't be counted
      }),
    ];

    const result = calculateCompactionStats(messages);
    expect(result.total).toBe(2);
    expect(result.avgCompactionTimeMs).toBe(20000); // Only counts the first one
  });
});

describe('formatResponseTime', () => {
  it('returns "-" for null', () => {
    expect(formatResponseTime(null)).toBe('-');
  });

  it('formats sub-second values with decimal', () => {
    expect(formatResponseTime(500)).toBe('0.5s');
    expect(formatResponseTime(1234)).toBe('1.2s');
  });

  it('formats seconds under a minute', () => {
    expect(formatResponseTime(4200)).toBe('4.2s');
    expect(formatResponseTime(59000)).toBe('59.0s');
  });

  it('formats exactly 60 seconds as 1m', () => {
    expect(formatResponseTime(60000)).toBe('1m');
  });

  it('formats minutes with remaining seconds', () => {
    expect(formatResponseTime(90000)).toBe('1m 30s');
    expect(formatResponseTime(125000)).toBe('2m 5s');
  });

  it('formats minutes without remaining seconds', () => {
    expect(formatResponseTime(120000)).toBe('2m');
    expect(formatResponseTime(300000)).toBe('5m');
  });

  it('handles edge case near minute boundary (avoids "1m 60s")', () => {
    // 119.5 seconds should round to 120 seconds = 2m, not "1m 60s"
    expect(formatResponseTime(119500)).toBe('2m');
  });

  it('formats hours with remaining minutes', () => {
    expect(formatResponseTime(3900000)).toBe('1h 5m'); // 65 minutes
    expect(formatResponseTime(5400000)).toBe('1h 30m'); // 90 minutes
  });

  it('formats hours without remaining minutes', () => {
    expect(formatResponseTime(3600000)).toBe('1h'); // 60 minutes
    expect(formatResponseTime(7200000)).toBe('2h'); // 120 minutes
  });
});
