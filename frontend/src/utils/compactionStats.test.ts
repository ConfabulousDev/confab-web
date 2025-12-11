import { describe, it, expect } from 'vitest';
import { calculateCompactionStats } from './compactionStats';
import type { TranscriptLine, SystemMessage, AssistantMessage, UserMessage } from '@/types';

// Helper to create a compact_boundary system message
function createCompactBoundary(
  uuid: string,
  trigger: 'auto' | 'manual' | string | undefined,
  preTokens = 100000
): SystemMessage {
  const base: SystemMessage = {
    type: 'system',
    uuid,
    timestamp: new Date().toISOString(),
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
function createAssistantMessage(uuid: string): AssistantMessage {
  return {
    type: 'assistant',
    uuid,
    timestamp: new Date().toISOString(),
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
function createUserMessage(uuid: string): UserMessage {
  return {
    type: 'user',
    uuid,
    timestamp: new Date().toISOString(),
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
    expect(result).toEqual({ total: 0, auto: 0, manual: 0 });
  });

  it('returns zeros when no compaction events exist', () => {
    const messages: TranscriptLine[] = [
      createUserMessage('1'),
      createAssistantMessage('2'),
      createUserMessage('3'),
      createAssistantMessage('4'),
    ];

    const result = calculateCompactionStats(messages);
    expect(result).toEqual({ total: 0, auto: 0, manual: 0 });
  });

  it('counts a single auto compaction', () => {
    const messages: TranscriptLine[] = [
      createUserMessage('1'),
      createAssistantMessage('2'),
      createCompactBoundary('3', 'auto'),
      createAssistantMessage('4'),
    ];

    const result = calculateCompactionStats(messages);
    expect(result).toEqual({ total: 1, auto: 1, manual: 0 });
  });

  it('counts a single manual compaction', () => {
    const messages: TranscriptLine[] = [
      createUserMessage('1'),
      createAssistantMessage('2'),
      createCompactBoundary('3', 'manual'),
      createAssistantMessage('4'),
    ];

    const result = calculateCompactionStats(messages);
    expect(result).toEqual({ total: 1, auto: 0, manual: 1 });
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
    expect(result).toEqual({ total: 3, auto: 2, manual: 1 });
  });

  it('counts toward total when compactMetadata is missing', () => {
    const messages: TranscriptLine[] = [
      createCompactBoundary('1', undefined),
    ];

    const result = calculateCompactionStats(messages);
    expect(result).toEqual({ total: 1, auto: 0, manual: 0 });
  });

  it('counts toward total when trigger has unknown value', () => {
    const messages: TranscriptLine[] = [
      createCompactBoundary('1', 'unknown_trigger'),
    ];

    const result = calculateCompactionStats(messages);
    expect(result).toEqual({ total: 1, auto: 0, manual: 0 });
  });

  it('ignores system messages with different subtypes', () => {
    const messages: TranscriptLine[] = [
      createSystemMessage('1', 'api_error'),
      createSystemMessage('2', 'warning'),
      createCompactBoundary('3', 'auto'),
      createSystemMessage('4', 'api_error'),
    ];

    const result = calculateCompactionStats(messages);
    expect(result).toEqual({ total: 1, auto: 1, manual: 0 });
  });

  it('correctly counts compactions mixed with various message types', () => {
    const messages: TranscriptLine[] = [
      createUserMessage('1'),
      createAssistantMessage('2'),
      createSystemMessage('3', 'api_error'),
      createCompactBoundary('4', 'auto', 150000),
      createUserMessage('5'),
      createAssistantMessage('6'),
      createCompactBoundary('7', 'manual', 160000),
      createSystemMessage('8', 'api_error'),
      createAssistantMessage('9'),
    ];

    const result = calculateCompactionStats(messages);
    expect(result).toEqual({ total: 2, auto: 1, manual: 1 });
  });
});
