import { describe, it, expect } from 'vitest';
import type { AssistantMessage } from '@/types';
import { calculateMessageCost, formatTokenCount, formatCost } from './tokenStats';

// Helper to create a minimal assistant message with token usage
function createAssistantMessage(
  inputTokens: number,
  outputTokens: number,
  cacheCreated = 0,
  cacheRead = 0,
  model = 'claude-opus-4-5-20251101',
  extra?: {
    server_tool_use?: { web_search_requests?: number; web_fetch_requests?: number; code_execution_requests?: number };
    speed?: string;
  },
): AssistantMessage {
  return {
    type: 'assistant',
    uuid: 'test-uuid',
    timestamp: new Date().toISOString(),
    parentUuid: null,
    isSidechain: false,
    userType: 'external',
    cwd: '/test',
    sessionId: 'test-session',
    version: '1.0.0',
    requestId: 'req-test',
    message: {
      model,
      id: 'msg-test',
      type: 'message',
      role: 'assistant',
      content: [{ type: 'text', text: 'Test response' }],
      stop_reason: 'end_turn',
      stop_sequence: null,
      usage: {
        input_tokens: inputTokens,
        output_tokens: outputTokens,
        cache_creation_input_tokens: cacheCreated,
        cache_read_input_tokens: cacheRead,
        ...extra,
      },
    },
  };
}

describe('formatTokenCount', () => {
  it('should return raw number for values under 1000', () => {
    expect(formatTokenCount(0)).toBe('0');
    expect(formatTokenCount(1)).toBe('1');
    expect(formatTokenCount(999)).toBe('999');
  });

  it('should format values >= 1000 with k suffix', () => {
    expect(formatTokenCount(1000)).toBe('1.0k');
    expect(formatTokenCount(1500)).toBe('1.5k');
    expect(formatTokenCount(10000)).toBe('10.0k');
    expect(formatTokenCount(145892)).toBe('145.9k');
    expect(formatTokenCount(999999)).toBe('1000.0k');
  });

  it('should format values >= 1M with M suffix', () => {
    expect(formatTokenCount(1000000)).toBe('1.0M');
    expect(formatTokenCount(1500000)).toBe('1.5M');
    expect(formatTokenCount(10000000)).toBe('10.0M');
    expect(formatTokenCount(999999999)).toBe('1000.0M');
  });

  it('should format values >= 1B with B suffix', () => {
    expect(formatTokenCount(1000000000)).toBe('1.0B');
    expect(formatTokenCount(1500000000)).toBe('1.5B');
    expect(formatTokenCount(10000000000)).toBe('10.0B');
  });

  it('should round to one decimal place', () => {
    expect(formatTokenCount(1234)).toBe('1.2k');
    expect(formatTokenCount(1256)).toBe('1.3k');
    expect(formatTokenCount(1234567)).toBe('1.2M');
    expect(formatTokenCount(1256789012)).toBe('1.3B');
  });
});

describe('calculateMessageCost', () => {
  it('should return 0 for user messages', () => {
    const userMsg = {
      type: 'user' as const,
      uuid: 'user-uuid',
      timestamp: new Date().toISOString(),
      parentUuid: null,
      isSidechain: false,
      userType: 'external',
      cwd: '/test',
      sessionId: 'test-session',
      version: '1.0.0',
      message: { role: 'user' as const, content: 'Hello' },
    };
    expect(calculateMessageCost(userMsg)).toBe(0);
  });

  it('should calculate cost for a known model', () => {
    // Sonnet 4: input=$3/M, output=$15/M
    const msg = createAssistantMessage(100_000, 10_000, 0, 0, 'claude-sonnet-4-20250514');
    const cost = calculateMessageCost(msg);
    // input: 100k * $3/M = $0.30, output: 10k * $15/M = $0.15 → $0.45
    expect(cost).toBeCloseTo(0.45, 4);
  });

  it('should apply fast mode multiplier to token costs', () => {
    const msg = createAssistantMessage(1_000_000, 100_000, 0, 0, 'claude-opus-4-6-20260201', {
      speed: 'fast',
    });
    const cost = calculateMessageCost(msg);
    // Standard: $5 + $2.50 = $7.50, Fast: $7.50 * 6 = $45
    expect(cost).toBeCloseTo(45, 4);
  });

  it('should add web search costs without fast multiplier', () => {
    const msg = createAssistantMessage(0, 0, 0, 0, 'claude-sonnet-4-20250514', {
      speed: 'fast',
      server_tool_use: { web_search_requests: 10 },
    });
    const cost = calculateMessageCost(msg);
    // Token cost: $0, Web search: 10 * $0.01 = $0.10
    expect(cost).toBeCloseTo(0.10, 4);
  });

  it('should return 0 for unknown models', () => {
    const msg = createAssistantMessage(1_000_000, 0, 0, 0, 'claude-unknown-model');
    expect(calculateMessageCost(msg)).toBe(0);
  });
});

describe('formatCost', () => {
  it('should format costs with dollar sign and 2 decimal places', () => {
    expect(formatCost(0.50)).toBe('$0.50');
    expect(formatCost(4.23)).toBe('$4.23');
    expect(formatCost(10.00)).toBe('$10.00');
    expect(formatCost(123.45)).toBe('$123.45');
  });

  it('should show $0.00 for exactly zero cost', () => {
    expect(formatCost(0)).toBe('$0.00');
  });

  it('should show <$0.01 for very small non-zero costs', () => {
    expect(formatCost(0.001)).toBe('<$0.01');
    expect(formatCost(0.009)).toBe('<$0.01');
  });

  it('should round to 2 decimal places', () => {
    expect(formatCost(0.016)).toBe('$0.02');
    expect(formatCost(0.014)).toBe('$0.01');
    expect(formatCost(1.999)).toBe('$2.00');
  });
});
