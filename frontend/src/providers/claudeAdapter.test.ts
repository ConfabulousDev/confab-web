// CF-417 spec: claudeAdapter satisfies the ProviderAdapter contract and
// delegates to the existing transcriptService / messageCategories APIs.

import { describe, expect, it, vi, beforeEach } from 'vitest';
import {
  fetchParsedTranscript,
  fetchNewTranscriptMessages,
} from '@/services/transcriptService';
import {
  DEFAULT_FILTER_STATE,
  messageMatchesFilter,
  countHierarchicalCategories,
} from '@/components/session/messageCategories';
import { computeSessionMeta } from '@/utils/sessionMeta';
import type { TokenUsage } from '@/utils/tokenStats';
import type { TranscriptLine, UserMessage, AssistantMessage } from '@/types';
import { claudeAdapter } from './claudeAdapter';

vi.mock('@/services/transcriptService', () => ({
  fetchParsedTranscript: vi.fn(),
  fetchNewTranscriptMessages: vi.fn(),
}));

beforeEach(() => {
  vi.clearAllMocks();
});

function userMessage(uuid: string, timestamp: string, text = 'hi'): UserMessage {
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
    message: { role: 'user', content: text },
  };
}

/**
 * Build an `AssistantMessage` for the contract-suite tests. The cost / tooltip
 * suites below have their own builders that stamp `tokenUsage` and accept
 * wire-extras (speed, service_tier, server_tool_use).
 */
function assistantMessage(
  uuid: string,
  timestamp: string,
  model = 'claude-sonnet-4-20250514',
): AssistantMessage {
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
      model,
      id: 'msg-123',
      type: 'message',
      role: 'assistant',
      content: [{ type: 'text', text: 'hello' }],
      stop_reason: 'end_turn',
      stop_sequence: null,
      usage: {
        input_tokens: 10,
        output_tokens: 5,
        cache_creation_input_tokens: 0,
        cache_read_input_tokens: 0,
      },
    },
  };
}

// Wire-payload extras that Claude renders separately from canonical TokenUsage.
interface ClaudeWireExtras {
  speed?: string;
  service_tier?: string | null;
  server_tool_use?: { web_search_requests?: number };
}

/**
 * Build an assistant message with canonical `tokenUsage` stamped AND the
 * matching wire shape on `message.usage`. Used by the cost + tooltip suites.
 */
function assistantMessageWithUsage(
  model: string,
  tokenUsage: TokenUsage,
  extras: ClaudeWireExtras = {},
): AssistantMessage & { tokenUsage: TokenUsage } {
  return {
    type: 'assistant',
    uuid: 'a-cost',
    timestamp: '2026-05-13T01:00:00Z',
    parentUuid: null,
    isSidechain: false,
    userType: 'human',
    cwd: '/test',
    sessionId: 'test-session',
    version: '1.0',
    requestId: 'req',
    message: {
      model,
      id: 'msg',
      type: 'message',
      role: 'assistant',
      content: [{ type: 'text', text: 'x' }],
      stop_reason: 'end_turn',
      stop_sequence: null,
      usage: {
        input_tokens: tokenUsage.input,
        output_tokens: tokenUsage.output,
        cache_creation_input_tokens: tokenUsage.cacheWrite,
        cache_read_input_tokens: tokenUsage.cacheRead,
        ...extras,
      },
    },
    tokenUsage,
  };
}

function zeroUsage(): TokenUsage {
  return { input: 0, output: 0, cacheWrite: 0, cacheRead: 0 };
}

describe('claudeAdapter', () => {
  it('has id="claude-code" and supportsTILs=true', () => {
    expect(claudeAdapter.id).toBe('claude-code');
    expect(claudeAdapter.supportsTILs).toBe(true);
  });

  it('fetchInitial delegates to fetchParsedTranscript and reshapes the result', async () => {
    const messages: TranscriptLine[] = [userMessage('u1', '2026-05-13T01:00:00Z')];
    vi.mocked(fetchParsedTranscript).mockResolvedValue({
      sessionId: 's',
      messages,
      agents: [],
      validationErrors: [],
      totalLines: 1,
      metadata: {
        version: '1.0',
        messageCount: 1,
        agentCount: 0,
        parseErrorCount: 0,
      },
    });

    const result = await claudeAdapter.fetchInitial('s', 'transcript.jsonl', true);

    expect(fetchParsedTranscript).toHaveBeenCalledWith('s', 'transcript.jsonl', true);
    expect(result.items).toBe(messages);
    expect(result.raw).toBe(messages);
    expect(result.totalLines).toBe(1);
  });

  it('fetchIncremental delegates to fetchNewTranscriptMessages', async () => {
    const newMessages: TranscriptLine[] = [userMessage('u2', '2026-05-13T01:01:00Z')];
    vi.mocked(fetchNewTranscriptMessages).mockResolvedValue({
      newMessages,
      newTotalLineCount: 5,
    });

    const result = await claudeAdapter.fetchIncremental('s', 'transcript.jsonl', 3);

    expect(fetchNewTranscriptMessages).toHaveBeenCalledWith('s', 'transcript.jsonl', 3);
    expect(result.newItems).toBe(newMessages);
    expect(result.newRaw).toBe(newMessages);
    expect(result.newTotalLineCount).toBe(5);
  });

  it('normalize is the identity function', () => {
    const messages: TranscriptLine[] = [userMessage('u1', '2026-05-13T01:00:00Z')];
    expect(claudeAdapter.normalize(messages)).toBe(messages);
  });

  it('extractModel returns first assistant message model', () => {
    const messages: TranscriptLine[] = [
      userMessage('u1', '2026-05-13T01:00:00Z'),
      assistantMessage('a1', '2026-05-13T01:00:01Z', 'claude-opus-4-6'),
      assistantMessage('a2', '2026-05-13T01:00:02Z', 'claude-sonnet-4-6'),
    ];
    expect(claudeAdapter.extractModel(messages, messages)).toBe('claude-opus-4-6');
  });

  it('extractModel returns undefined when no assistant message present', () => {
    const messages: TranscriptLine[] = [userMessage('u1', '2026-05-13T01:00:00Z')];
    expect(claudeAdapter.extractModel(messages, messages)).toBeUndefined();
  });

  it('computeMeta delegates to computeSessionMeta over the items', () => {
    const messages: TranscriptLine[] = [
      userMessage('u1', '2026-05-13T01:00:00Z'),
      assistantMessage('a1', '2026-05-13T01:05:00Z'),
    ];
    const meta = claudeAdapter.computeMeta(messages, messages, {});
    const expected = computeSessionMeta(messages, {});
    expect(meta.durationMs).toBe(expected.durationMs);
    expect(meta.sessionDate?.toISOString()).toBe(expected.sessionDate?.toISOString());
  });

  it('countCategories delegates to countHierarchicalCategories', () => {
    const messages: TranscriptLine[] = [
      userMessage('u1', '2026-05-13T01:00:00Z'),
      assistantMessage('a1', '2026-05-13T01:05:00Z'),
    ];
    expect(claudeAdapter.countCategories(messages)).toEqual(
      countHierarchicalCategories(messages),
    );
  });

  it('itemMatchesFilter delegates to messageMatchesFilter', () => {
    const msg = userMessage('u1', '2026-05-13T01:00:00Z');
    expect(claudeAdapter.itemMatchesFilter(msg, DEFAULT_FILTER_STATE)).toBe(
      messageMatchesFilter(msg, DEFAULT_FILTER_STATE),
    );
  });

  it('exposes FilterDropdown and TranscriptPane as renderable components', () => {
    expect(typeof claudeAdapter.FilterDropdown).toBe('function');
    expect(typeof claudeAdapter.TranscriptPane).toBe('function');
  });

  it('exposes useFilters and useDeepLinkFilterReset as functions', () => {
    expect(typeof claudeAdapter.useFilters).toBe('function');
    expect(typeof claudeAdapter.useDeepLinkFilterReset).toBe('function');
  });
});

// CF-418: Claude adapter applies the fast-mode multiplier (6x) and adds
// per-request web-search dollars on top of the base arithmetic from
// `calculateCost`. Both adjustments are Claude-specific.
describe('claudeAdapter.calculateMessageCost', () => {
  it('returns base calculateCost result when speed is not fast and no server tools used', () => {
    const msg = assistantMessageWithUsage('claude-sonnet-4-20250514', {
      ...zeroUsage(),
      input: 100_000,
      output: 10_000,
    });
    // sonnet-4: 100k * $3/M + 10k * $15/M = $0.45
    expect(claudeAdapter.calculateMessageCost(msg.message.model, msg.tokenUsage, msg))
      .toBeCloseTo(0.45, 4);
  });

  it('applies the 6x fast multiplier when usage.speed === "fast"', () => {
    const msg = assistantMessageWithUsage(
      'claude-opus-4-6-20260201',
      { ...zeroUsage(), input: 1_000_000, output: 100_000 },
      { speed: 'fast' },
    );
    // opus-4-6: $5 + $2.50 = $7.50 base → $45 with 6x fast
    expect(claudeAdapter.calculateMessageCost(msg.message.model, msg.tokenUsage, msg))
      .toBeCloseTo(45, 4);
  });

  it('adds web-search dollars per request, not multiplied by fast', () => {
    const msg = assistantMessageWithUsage(
      'claude-sonnet-4-20250514',
      zeroUsage(),
      { speed: 'fast', server_tool_use: { web_search_requests: 10 } },
    );
    // Token cost = 0; web search = 10 * $0.01 = $0.10. Not multiplied by 6.
    expect(claudeAdapter.calculateMessageCost(msg.message.model, msg.tokenUsage, msg))
      .toBeCloseTo(0.1, 4);
  });

  it('combines fast multiplier and web-search add-on correctly', () => {
    const msg = assistantMessageWithUsage(
      'claude-opus-4-6-20260201',
      { ...zeroUsage(), input: 100_000, output: 10_000 },
      { speed: 'fast', server_tool_use: { web_search_requests: 2 } },
    );
    // base = 100k*$5 + 10k*$25 = $0.75; fast = $4.50; web = $0.02 → $4.52
    expect(claudeAdapter.calculateMessageCost(msg.message.model, msg.tokenUsage, msg))
      .toBeCloseTo(4.52, 4);
  });
});

// CF-418: Claude adapter's tooltip appends Speed / Tier / Web-search lines
// to the base cost-tooltip output. Codex-only labels never appear.
describe('claudeAdapter.extendCostTooltip', () => {
  const baseLines = ['$0.10', '', 'Input tokens (in): 0', 'Output tokens (out): 0'];

  it('appends Speed (fast) line when usage.speed === "fast"', () => {
    const msg = assistantMessageWithUsage('claude-sonnet-4-20250514', zeroUsage(), { speed: 'fast' });
    const out = claudeAdapter.extendCostTooltip!(baseLines, zeroUsage(), msg);
    expect(out.some((l) => /Speed:\s*fast/.test(l))).toBe(true);
    expect(out.some((l) => l.includes('6x'))).toBe(true);
  });

  it('appends Cache write / Cache read lines when present in canonical usage', () => {
    const msg = assistantMessageWithUsage('claude-sonnet-4-20250514', zeroUsage());
    const usage: TokenUsage = { input: 0, output: 0, cacheWrite: 100, cacheRead: 50 };
    const out = claudeAdapter.extendCostTooltip!(baseLines, usage, msg);
    expect(out.some((l) => l.includes('Cache write tokens (write): 100'))).toBe(true);
    expect(out.some((l) => l.includes('Cache read tokens (hit): 50'))).toBe(true);
  });

  it('appends Web searches line when web_search_requests > 0', () => {
    const msg = assistantMessageWithUsage('claude-sonnet-4-20250514', zeroUsage(), {
      server_tool_use: { web_search_requests: 3 },
    });
    const out = claudeAdapter.extendCostTooltip!(baseLines, zeroUsage(), msg);
    expect(out.some((l) => /Web searches:\s*3/.test(l))).toBe(true);
  });

  it('does not emit Cached (hit) or Reasoning sub-lines (those are Codex-only)', () => {
    const msg = assistantMessageWithUsage('claude-sonnet-4-20250514', zeroUsage(), { speed: 'fast' });
    const usage: TokenUsage = { input: 100, output: 100, cacheWrite: 10, cacheRead: 10 };
    const out = claudeAdapter.extendCostTooltip!(baseLines, usage, msg);
    expect(out.every((l) => !l.includes('Cached (hit)'))).toBe(true);
    expect(out.every((l) => !l.includes('Reasoning'))).toBe(true);
  });
});

// CF-436: Per-provider tooltip strings for the Tokens summary card live on
// the adapter as static fields. Claude defines both cost + fast tooltips.
describe('claudeAdapter Tokens-card tooltips (CF-436)', () => {
  it('defines tokensCostTooltip mentioning 5-minute prompt caching', () => {
    expect(claudeAdapter.tokensCostTooltip).toBe(
      'Estimated API cost based on token usage and model pricing (assumes 5-minute prompt caching)',
    );
  });

  it('defines tokensFastTooltip naming Anthropic priority tier', () => {
    expect(claudeAdapter.tokensFastTooltip).toBe(
      'Cost from turns using Anthropic priority tier (~6x base rate)',
    );
  });
});
