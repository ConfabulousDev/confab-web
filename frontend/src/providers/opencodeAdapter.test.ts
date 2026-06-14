// CF-557: opencodeAdapter satisfies the ProviderAdapter contract and delegates
// to the existing opencodeTranscriptService / opencodeCategories APIs. Mirrors
// claudeAdapter.test.ts / codexAdapter.test.ts for test parity. OpenCode-specific
// behaviors (★) are asserted directly: fetchIncremental derives newItems via
// normalize (not pass-through), computeMeta walks render-items' epoch-ms
// timeCreated, and calculateMessageCost is a hybrid (prefer info.cost, else the
// pricing-table fallback).

import { describe, expect, it, vi, beforeEach, beforeAll } from 'vitest';
import {
  fetchParsedOpenCodeTranscript,
  fetchNewOpenCodeLines,
  normalizeOpenCodeLines,
  extractOpenCodeModel,
  parseOpenCodeJSONL,
} from '@/services/opencodeTranscriptService';
import {
  DEFAULT_OPENCODE_FILTER_STATE,
  countOpenCodeCategories,
  opencodeItemMatchesFilter,
  type OpenCodeRenderItem,
} from '@/components/session/opencodeCategories';
import { calculateCost, setPricingTable, type TokenUsage } from '@/utils/tokenStats';
import { PRICING_FIXTURE } from '@/test/pricingFixture';
import { opencodeAdapter } from './opencodeAdapter';

// Mock ONLY the two fetch functions; importActual for the pure delegates
// (normalize / extractModel / parse builder), exactly as the codex sibling does.
vi.mock('@/services/opencodeTranscriptService', async () => {
  const actual = await vi.importActual<typeof import('@/services/opencodeTranscriptService')>(
    '@/services/opencodeTranscriptService',
  );
  return {
    ...actual,
    fetchParsedOpenCodeTranscript: vi.fn(),
    fetchNewOpenCodeLines: vi.fn(),
  };
});

beforeEach(() => {
  vi.clearAllMocks();
});

// Reuse the wire fixtures + builder from opencodeTranscriptService.test.ts.
function line(obj: unknown): string {
  return JSON.stringify(obj);
}

const userLine = {
  info: { id: 'msg_user', role: 'user', time: { created: 1717689500000 } },
  parts: [{ id: 'prt_1', type: 'text', text: 'Find all Go files' }],
};

const assistantLine = {
  info: {
    id: 'msg_asst',
    role: 'assistant',
    modelID: 'claude-sonnet-4-20250514',
    providerID: 'anthropic',
    cost: 0.015,
    tokens: { input: 10000, output: 5000, cache: { read: 3000, write: 2000 } },
    time: { created: 1717689600000 },
  },
  parts: [
    { id: 'prt_2', type: 'reasoning', text: 'Let me check the files...' },
    { id: 'prt_4', type: 'text', text: 'I found 2 files.' },
  ],
};

function rawLines(...objs: unknown[]) {
  return parseOpenCodeJSONL(objs.map(line).join('\n')).rawLines;
}

describe('opencodeAdapter', () => {
  it('has id="opencode"', () => {
    expect(opencodeAdapter.id).toBe('opencode');
  });

  it('fetchInitial delegates to fetchParsedOpenCodeTranscript and reshapes the result', async () => {
    const raw = rawLines(userLine, assistantLine);
    const items = normalizeOpenCodeLines(raw);
    vi.mocked(fetchParsedOpenCodeTranscript).mockResolvedValue({
      sessionId: 's',
      items,
      rawLines: raw,
      totalLines: 2,
    });

    const result = await opencodeAdapter.fetchInitial('s', 'session.jsonl', true);

    expect(fetchParsedOpenCodeTranscript).toHaveBeenCalledWith('s', 'session.jsonl', true);
    expect(result.items).toBe(items);
    expect(result.totalLines).toBe(2);
    expect(result.raw).toBe(raw); // raw === parsed.rawLines
  });

  it('fetchIncremental delegates to fetchNewOpenCodeLines and DERIVES newItems via normalize (★)', async () => {
    const raw = rawLines(assistantLine);
    vi.mocked(fetchNewOpenCodeLines).mockResolvedValue({
      newRawLines: raw,
      newTotalLineCount: 7,
    });

    const result = await opencodeAdapter.fetchIncremental('s', 'session.jsonl', 5);

    expect(fetchNewOpenCodeLines).toHaveBeenCalledWith('s', 'session.jsonl', 5);
    expect(result.newRaw).toBe(raw);
    expect(result.newTotalLineCount).toBe(7);
    // newItems is normalize-derived render items, NOT a pass-through of the raw
    // wire lines — toEqual against the normalizer output proves the derivation.
    expect(result.newItems).toEqual(normalizeOpenCodeLines(raw));
  });

  it('normalize delegates to normalizeOpenCodeLines', () => {
    expect(opencodeAdapter.normalize).toBe(normalizeOpenCodeLines);
  });

  it('extractModel delegates to extractOpenCodeModel', () => {
    const raw = rawLines(assistantLine);
    expect(opencodeAdapter.extractModel(raw, [])).toBe('claude-sonnet-4-20250514');
    expect(opencodeAdapter.extractModel(raw, [])).toBe(extractOpenCodeModel(raw));
  });

  it('computeMeta walks render-items timeCreated (epoch ms) for min/max (★)', () => {
    const items = normalizeOpenCodeLines(rawLines(userLine, assistantLine));
    const meta = opencodeAdapter.computeMeta(items, [], {});
    // user created 1717689500000, assistant 1717689600000 → 100s span.
    expect(meta.durationMs).toBe(100_000);
    expect(meta.sessionDate?.getTime()).toBe(1717689500000);
  });

  it('computeMeta falls back to firstSeen/lastSyncAt when items is empty (★)', () => {
    const meta = opencodeAdapter.computeMeta([], [], {
      firstSeen: '2026-05-13T01:00:00Z',
      lastSyncAt: '2026-05-13T01:10:00Z',
    });
    expect(meta.durationMs).toBe(10 * 60 * 1000);
    expect(meta.sessionDate?.toISOString()).toBe('2026-05-13T01:00:00.000Z');
  });

  it('countCategories delegates to countOpenCodeCategories', () => {
    const items = normalizeOpenCodeLines(rawLines(userLine, assistantLine));
    expect(opencodeAdapter.countCategories(items)).toEqual(countOpenCodeCategories(items));
  });

  it('itemMatchesFilter delegates to opencodeItemMatchesFilter', () => {
    const items = normalizeOpenCodeLines(rawLines(userLine));
    if (items[0]) {
      expect(opencodeAdapter.itemMatchesFilter(items[0], DEFAULT_OPENCODE_FILTER_STATE)).toBe(
        opencodeItemMatchesFilter(items[0], DEFAULT_OPENCODE_FILTER_STATE),
      );
    }
  });

  it('exposes FilterDropdown and TranscriptPane as renderable components', () => {
    expect(typeof opencodeAdapter.FilterDropdown).toBe('function');
    expect(typeof opencodeAdapter.TranscriptPane).toBe('function');
  });

  it('exposes useFilters and useDeepLinkFilterReset as functions', () => {
    expect(typeof opencodeAdapter.useFilters).toBe('function');
    expect(typeof opencodeAdapter.useDeepLinkFilterReset).toBe('function');
  });
});

// CF-557: hybrid cost — prefer the message-reported info.cost, else fall back to
// the pricing-table arithmetic. Fallback assertions are SELF-REFERENTIAL to the
// live calculateCost('opencode', …) so the suite is order-independent w.r.t.
// rd9v's pricing-table change.
describe('opencodeAdapter.calculateMessageCost', () => {
  beforeAll(() => setPricingTable(PRICING_FIXTURE));

  type OCAssistant = Extract<OpenCodeRenderItem, { kind: 'assistant' }>;
  function assistantItem(overrides: Partial<OCAssistant> = {}): OCAssistant {
    return {
      kind: 'assistant',
      id: 'msg_asst',
      text: 'x',
      model: 'gemini-2.5-pro',
      timeCreated: 1717689600000,
      ...overrides,
    };
  }

  function usage(overrides: Partial<TokenUsage> = {}): TokenUsage {
    return { input: 0, output: 0, cacheWrite: 0, cacheWrite1h: 0, cacheRead: 0, ...overrides };
  }

  it('prefers the message-reported info.cost when it is numeric (★)', () => {
    // A cost that does NOT match the table arithmetic, to prove precedence.
    const u = usage({ input: 1_000_000 });
    const item = assistantItem({ cost: 0.42, usage: u });
    expect(opencodeAdapter.calculateMessageCost(item.model!, u, item)).toBe(0.42);
  });

  it('falls back to calculateCost(opencode, …) when the message has no cost (★)', () => {
    const u = usage({ input: 1_000_000 });
    const item = assistantItem({ usage: u }); // no cost
    expect(opencodeAdapter.calculateMessageCost(item.model!, u, item)).toBe(
      calculateCost('opencode', item.model!, u),
    );
  });

  it('returns 0 for a non-assistant item (★)', () => {
    const userItem: OpenCodeRenderItem = { kind: 'user', id: 'u', text: 'hi', timeCreated: 1 };
    expect(
      opencodeAdapter.calculateMessageCost('gemini-2.5-pro', usage({ input: 1_000_000 }), userItem),
    ).toBe(0);
  });

  it('falls back to calculateCost for a zero-usage assistant without cost (boundary, ★)', () => {
    const zero = usage();
    const item = assistantItem({ usage: zero });
    expect(opencodeAdapter.calculateMessageCost(item.model!, zero, item)).toBe(
      calculateCost('opencode', item.model!, zero),
    );
  });
});

// CF-436: OpenCode defines its own tokensCostTooltip and, unlike Claude/Codex,
// defines neither extendCostTooltip nor tokensFastTooltip.
describe('opencodeAdapter Tokens-card tooltips', () => {
  it('defines the OpenCode cost tooltip string', () => {
    expect(opencodeAdapter.tokensCostTooltip).toBe(
      'Cost reported by OpenCode per message across all providers used in this session.',
    );
  });

  it('does not define extendCostTooltip (★)', () => {
    expect(opencodeAdapter.extendCostTooltip).toBeUndefined();
  });

  it('does not define tokensFastTooltip (★)', () => {
    expect(opencodeAdapter.tokensFastTooltip).toBeUndefined();
  });
});
