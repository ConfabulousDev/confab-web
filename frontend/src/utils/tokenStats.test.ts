import { describe, it, expect } from 'vitest';
import type { TranscriptLine, AssistantMessage } from '@/types';
import { calculateTokenStats, calculateEstimatedCost, formatTokenCount, formatCost } from './tokenStats';

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

describe('calculateTokenStats', () => {
  it('should return zeros for empty messages array', () => {
    const stats = calculateTokenStats([]);
    expect(stats).toEqual({
      input: 0,
      output: 0,
      cacheCreated: 0,
      cacheRead: 0,
      webSearchRequests: 0,
      webFetchRequests: 0,
      codeExecutionRequests: 0,
    });
  });

  it('should sum tokens from a single assistant message', () => {
    const messages: TranscriptLine[] = [
      createAssistantMessage(1000, 500, 200, 100),
    ];
    const stats = calculateTokenStats(messages);
    expect(stats).toEqual({
      input: 1000,
      output: 500,
      cacheCreated: 200,
      cacheRead: 100,
      webSearchRequests: 0,
      webFetchRequests: 0,
      codeExecutionRequests: 0,
    });
  });

  it('should sum tokens from multiple assistant messages', () => {
    const messages: TranscriptLine[] = [
      createAssistantMessage(1000, 500, 200, 100),
      createAssistantMessage(2000, 800, 300, 150),
      createAssistantMessage(1500, 600, 0, 500),
    ];
    const stats = calculateTokenStats(messages);
    expect(stats).toEqual({
      input: 4500,
      output: 1900,
      cacheCreated: 500,
      cacheRead: 750,
      webSearchRequests: 0,
      webFetchRequests: 0,
      codeExecutionRequests: 0,
    });
  });

  it('should handle messages without cache tokens', () => {
    const message = createAssistantMessage(1000, 500, 0, 0);
    // Remove optional cache fields to simulate real-world data
    delete message.message.usage.cache_creation_input_tokens;
    delete message.message.usage.cache_read_input_tokens;

    const messages: TranscriptLine[] = [message];
    const stats = calculateTokenStats(messages);
    expect(stats).toEqual({
      input: 1000,
      output: 500,
      cacheCreated: 0,
      cacheRead: 0,
      webSearchRequests: 0,
      webFetchRequests: 0,
      codeExecutionRequests: 0,
    });
  });

  it('should ignore non-assistant messages', () => {
    const assistantMsg = createAssistantMessage(1000, 500, 200, 100);
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
      message: {
        role: 'user' as const,
        content: 'Hello',
      },
    };

    const messages: TranscriptLine[] = [userMsg, assistantMsg];
    const stats = calculateTokenStats(messages);
    expect(stats).toEqual({
      input: 1000,
      output: 500,
      cacheCreated: 200,
      cacheRead: 100,
      webSearchRequests: 0,
      webFetchRequests: 0,
      codeExecutionRequests: 0,
    });
  });

  it('should accumulate server tool request counts', () => {
    const messages: TranscriptLine[] = [
      createAssistantMessage(1000, 500, 0, 0, 'claude-sonnet-4-20250514', {
        server_tool_use: { web_search_requests: 3, web_fetch_requests: 1 },
      }),
      createAssistantMessage(1000, 500, 0, 0, 'claude-sonnet-4-20250514', {
        server_tool_use: { web_search_requests: 2, code_execution_requests: 1 },
      }),
    ];
    const stats = calculateTokenStats(messages);
    expect(stats.webSearchRequests).toBe(5);
    expect(stats.webFetchRequests).toBe(1);
    expect(stats.codeExecutionRequests).toBe(1);
  });
});

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

describe('calculateEstimatedCost', () => {
  it('should return 0 for empty messages array', () => {
    expect(calculateEstimatedCost([])).toBe(0);
  });

  it('should calculate cost for opus-4-5 model', () => {
    // Opus 4.5: input=$5/M, output=$25/M, cacheWrite=$6.25/M, cacheRead=$0.50/M
    const messages: TranscriptLine[] = [
      createAssistantMessage(1_000_000, 100_000, 50_000, 200_000, 'claude-opus-4-5-20251101'),
    ];
    const cost = calculateEstimatedCost(messages);
    // input: 1M * $5/M = $5
    // output: 100k * $25/M = $2.50
    // cacheWrite: 50k * $6.25/M = $0.3125
    // cacheRead: 200k * $0.50/M = $0.10
    // Total: $7.9125
    expect(cost).toBeCloseTo(7.9125, 4);
  });

  it('should calculate cost for sonnet-4 model', () => {
    // Sonnet 4: input=$3/M, output=$15/M, cacheWrite=$3.75/M, cacheRead=$0.30/M
    const messages: TranscriptLine[] = [
      createAssistantMessage(1_000_000, 100_000, 50_000, 200_000, 'claude-sonnet-4-20250514'),
    ];
    const cost = calculateEstimatedCost(messages);
    // input: 1M * $3/M = $3
    // output: 100k * $15/M = $1.50
    // cacheWrite: 50k * $3.75/M = $0.1875
    // cacheRead: 200k * $0.30/M = $0.06
    // Total: $4.7475
    expect(cost).toBeCloseTo(4.7475, 4);
  });

  it('should calculate cost for haiku-3-5 model', () => {
    // Haiku 3.5: input=$0.80/M, output=$4/M, cacheWrite=$1.00/M, cacheRead=$0.08/M
    const messages: TranscriptLine[] = [
      createAssistantMessage(1_000_000, 100_000, 50_000, 200_000, 'claude-haiku-3-5-20241022'),
    ];
    const cost = calculateEstimatedCost(messages);
    // input: 1M * $0.80/M = $0.80
    // output: 100k * $4/M = $0.40
    // cacheWrite: 50k * $1.00/M = $0.05
    // cacheRead: 200k * $0.08/M = $0.016
    // Total: $1.266
    expect(cost).toBeCloseTo(1.266, 4);
  });

  it('should sum costs across multiple messages with different models', () => {
    const messages: TranscriptLine[] = [
      createAssistantMessage(100_000, 10_000, 0, 0, 'claude-opus-4-5-20251101'),
      createAssistantMessage(100_000, 10_000, 0, 0, 'claude-sonnet-4-20250514'),
    ];
    const cost = calculateEstimatedCost(messages);
    // Opus 4.5: input 100k * $5/M + output 10k * $25/M = $0.50 + $0.25 = $0.75
    // Sonnet 4: input 100k * $3/M + output 10k * $15/M = $0.30 + $0.15 = $0.45
    // Total: $1.20
    expect(cost).toBeCloseTo(1.20, 4);
  });

  it('should use zero pricing for unknown models', () => {
    const messages: TranscriptLine[] = [
      createAssistantMessage(1_000_000, 0, 0, 0, 'claude-unknown-model'),
    ];
    const cost = calculateEstimatedCost(messages);
    // Unknown models contribute $0 rather than silently defaulting
    expect(cost).toBe(0);
  });

  it('should add web search per-request costs', () => {
    const messages: TranscriptLine[] = [
      createAssistantMessage(100_000, 10_000, 0, 0, 'claude-sonnet-4-20250514', {
        server_tool_use: { web_search_requests: 5, web_fetch_requests: 3 },
      }),
    ];
    const cost = calculateEstimatedCost(messages);
    // Tokens: input 100k * $3/M + output 10k * $15/M = $0.30 + $0.15 = $0.45
    // Web search: 5 * $0.01 = $0.05
    // Web fetch: free
    // Total: $0.50
    expect(cost).toBeCloseTo(0.50, 4);
  });

  it('should apply 6x multiplier for fast mode', () => {
    const messages: TranscriptLine[] = [
      createAssistantMessage(1_000_000, 100_000, 0, 0, 'claude-opus-4-6-20260201', {
        speed: 'fast',
      }),
    ];
    const cost = calculateEstimatedCost(messages);
    // Standard: input 1M * $5/M + output 100k * $25/M = $5 + $2.50 = $7.50
    // Fast: $7.50 * 6 = $45
    expect(cost).toBeCloseTo(45, 4);
  });

  it('should apply fast mode multiplier to cache costs', () => {
    const messages: TranscriptLine[] = [
      createAssistantMessage(0, 0, 1_000_000, 1_000_000, 'claude-opus-4-6-20260201', {
        speed: 'fast',
      }),
    ];
    const cost = calculateEstimatedCost(messages);
    // Standard: cacheWrite 1M * $6.25/M + cacheRead 1M * $0.50/M = $6.75
    // Fast: $6.75 * 6 = $40.50
    expect(cost).toBeCloseTo(40.50, 4);
  });

  it('should not apply fast mode multiplier to web search costs', () => {
    const messages: TranscriptLine[] = [
      createAssistantMessage(1_000_000, 0, 0, 0, 'claude-opus-4-6-20260201', {
        speed: 'fast',
        server_tool_use: { web_search_requests: 10 },
      }),
    ];
    const cost = calculateEstimatedCost(messages);
    // Token cost: 1M * $5/M = $5, fast: $5 * 6 = $30
    // Web search: 10 * $0.01 = $0.10 (NOT multiplied by fast mode)
    // Total: $30.10
    expect(cost).toBeCloseTo(30.10, 4);
  });

  it('should not change cost for standard speed', () => {
    const messages: TranscriptLine[] = [
      createAssistantMessage(1_000_000, 0, 0, 0, 'claude-opus-4-5-20251101', {
        speed: 'standard',
      }),
    ];
    const cost = calculateEstimatedCost(messages);
    // Standard speed: no multiplier
    // input 1M * $5/M = $5
    expect(cost).toBeCloseTo(5, 4);
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
