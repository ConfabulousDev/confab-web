// CF-417 spec: codexAdapter satisfies the ProviderAdapter contract and
// delegates to the existing codexTranscriptService / codexCategories APIs.

import { describe, expect, it, vi, beforeEach } from 'vitest';
import {
  fetchParsedCodexTranscript,
  fetchNewCodexLines,
  normalizeCodexLines,
  extractCodexModel,
  parseCodexJSONL,
} from '@/services/codexTranscriptService';
import {
  DEFAULT_CODEX_FILTER_STATE,
  codexItemMatchesFilter,
  countCodexCategories,
} from '@/components/session/codexCategories';
import type { RawCodexLine } from '@/schemas/codexTranscript';
import type { TokenUsage } from '@/utils/tokenStats';
import type { CodexAssistantItem } from '@/types/codexRenderItem';
import { codexAdapter } from './codexAdapter';

vi.mock('@/services/codexTranscriptService', async () => {
  const actual = await vi.importActual<typeof import('@/services/codexTranscriptService')>(
    '@/services/codexTranscriptService',
  );
  return {
    ...actual,
    fetchParsedCodexTranscript: vi.fn(),
    fetchNewCodexLines: vi.fn(),
  };
});

beforeEach(() => {
  vi.clearAllMocks();
});

function rawLine(jsonl: string): RawCodexLine {
  const parsed = parseCodexJSONL(jsonl);
  const line = parsed.rawLines[0];
  if (!line) {
    throw new Error(`rawLine helper: failed to parse ${jsonl} (errors=${JSON.stringify(parsed.errors)})`);
  }
  return line;
}

describe('codexAdapter', () => {
  it('has id="codex" and supportsTILs=false', () => {
    expect(codexAdapter.id).toBe('codex');
    expect(codexAdapter.supportsTILs).toBe(false);
  });

  it('fetchInitial delegates to fetchParsedCodexTranscript and reshapes the result', async () => {
    const raw: RawCodexLine[] = [
      rawLine(
        '{"timestamp":"2026-05-13T01:00:00Z","type":"session_meta","payload":{"id":"x","model":"gpt-5-codex"}}',
      ),
    ];
    vi.mocked(fetchParsedCodexTranscript).mockResolvedValue({
      sessionId: 's',
      items: [],
      rawLines: raw,
      validationErrors: [],
      totalLines: 1,
      metadata: { itemCount: 0, rawLineCount: 1, parseErrorCount: 0 },
    });

    const result = await codexAdapter.fetchInitial('s', 'rollout.jsonl', true);

    expect(fetchParsedCodexTranscript).toHaveBeenCalledWith('s', 'rollout.jsonl', true);
    expect(result.raw).toBe(raw);
    expect(result.totalLines).toBe(1);
    expect(Array.isArray(result.items)).toBe(true);
  });

  it('fetchIncremental delegates to fetchNewCodexLines', async () => {
    const raw: RawCodexLine[] = [
      rawLine(
        '{"timestamp":"2026-05-13T01:01:00Z","type":"compacted","payload":{"message":"","replacement_history":[]}}',
      ),
    ];
    vi.mocked(fetchNewCodexLines).mockResolvedValue({
      newRawLines: raw,
      newTotalLineCount: 7,
    });

    const result = await codexAdapter.fetchIncremental('s', 'rollout.jsonl', 5);

    expect(fetchNewCodexLines).toHaveBeenCalledWith('s', 'rollout.jsonl', 5);
    expect(result.newRaw).toBe(raw);
    expect(result.newTotalLineCount).toBe(7);
    expect(Array.isArray(result.newItems)).toBe(true);
  });

  it('normalize delegates to normalizeCodexLines', () => {
    const raw: RawCodexLine[] = [
      rawLine(
        '{"timestamp":"2026-05-13T01:00:00Z","type":"compacted","payload":{"message":"hi","replacement_history":[]}}',
      ),
    ];
    expect(codexAdapter.normalize(raw)).toEqual(normalizeCodexLines(raw));
  });

  it('extractModel delegates to extractCodexModel', () => {
    const raw: RawCodexLine[] = [
      rawLine(
        '{"timestamp":"2026-05-13T01:00:00Z","type":"session_meta","payload":{"id":"x","model":"gpt-5-codex"}}',
      ),
    ];
    expect(codexAdapter.extractModel(raw, [])).toBe('gpt-5-codex');
    expect(codexAdapter.extractModel(raw, [])).toBe(extractCodexModel(raw));
  });

  it('computeMeta walks rawLines for min/max timestamp', () => {
    const raw: RawCodexLine[] = [
      rawLine(
        '{"timestamp":"2026-05-13T01:00:00Z","type":"session_meta","payload":{"id":"x","model":"gpt-5-codex"}}',
      ),
      rawLine(
        '{"timestamp":"2026-05-13T01:05:00Z","type":"compacted","payload":{"message":"","replacement_history":[]}}',
      ),
    ];

    const meta = codexAdapter.computeMeta([], raw, {});

    expect(meta.durationMs).toBe(5 * 60 * 1000);
    expect(meta.sessionDate?.toISOString()).toBe('2026-05-13T01:00:00.000Z');
  });

  it('computeMeta falls back to firstSeen/lastSyncAt when rawLines is empty', () => {
    const meta = codexAdapter.computeMeta([], [], {
      firstSeen: '2026-05-13T01:00:00Z',
      lastSyncAt: '2026-05-13T01:10:00Z',
    });
    expect(meta.durationMs).toBe(10 * 60 * 1000);
    expect(meta.sessionDate?.toISOString()).toBe('2026-05-13T01:00:00.000Z');
  });

  it('countCategories delegates to countCodexCategories', () => {
    const items = normalizeCodexLines([
      rawLine(
        '{"timestamp":"2026-05-13T01:00:00Z","type":"compacted","payload":{"message":"hi","replacement_history":[]}}',
      ),
    ]);
    expect(codexAdapter.countCategories(items)).toEqual(countCodexCategories(items));
  });

  it('itemMatchesFilter delegates to codexItemMatchesFilter', () => {
    const items = normalizeCodexLines([
      rawLine(
        '{"timestamp":"2026-05-13T01:00:00Z","type":"compacted","payload":{"message":"hi","replacement_history":[]}}',
      ),
    ]);
    if (items[0]) {
      expect(codexAdapter.itemMatchesFilter(items[0], DEFAULT_CODEX_FILTER_STATE)).toBe(
        codexItemMatchesFilter(items[0], DEFAULT_CODEX_FILTER_STATE),
      );
    }
  });

  it('exposes FilterDropdown and TranscriptPane as renderable components', () => {
    expect(typeof codexAdapter.FilterDropdown).toBe('function');
    expect(typeof codexAdapter.TranscriptPane).toBe('function');
  });

  it('exposes useFilters and useDeepLinkFilterReset as functions', () => {
    expect(typeof codexAdapter.useFilters).toBe('function');
    expect(typeof codexAdapter.useDeepLinkFilterReset).toBe('function');
  });
});

// CF-418: Codex adapter's calculateMessageCost is base arithmetic only —
// no fast multiplier (Codex has no `speed`), no web-search add-on.
// Caller (parse layer) has already split `input` into uncached + cacheRead
// and folded reasoning into `output`.
describe('codexAdapter.calculateMessageCost', () => {
  function item(model: string, usage: TokenUsage): CodexAssistantItem {
    return {
      kind: 'assistant',
      lineId: '0',
      timestamp: '2026-05-13T01:00:00Z',
      text: 'x',
      phase: 'final',
      model,
      usage,
    };
  }

  function usage(overrides: Partial<TokenUsage> = {}): TokenUsage {
    return { input: 0, output: 0, cacheWrite: 0, cacheRead: 0, ...overrides };
  }

  it('bills gpt-5 input + output at documented rates', () => {
    // gpt-5: input=$1.25/M, output=$10/M → 1M*$1.25 + 100k*$10 = $2.25
    const it = item('gpt-5', usage({ input: 1_000_000, output: 100_000 }));
    expect(codexAdapter.calculateMessageCost(it.model, it.usage!, it))
      .toBeCloseTo(2.25, 4);
  });

  it('charges cache reads at the documented cache-read rate', () => {
    // gpt-5 cacheRead=$0.125/M → 200k*$0.125 = $0.025
    const it = item('gpt-5', usage({ cacheRead: 200_000 }));
    expect(codexAdapter.calculateMessageCost(it.model, it.usage!, it))
      .toBeCloseTo(0.025, 6);
  });

  it('does NOT charge for cache writes (table rate is 0 for Codex models)', () => {
    const it = item('gpt-5', usage({ cacheWrite: 999_999_999 }));
    expect(codexAdapter.calculateMessageCost(it.model, it.usage!, it)).toBe(0);
  });

  it('returns 0 for zero usage', () => {
    const it = item('gpt-5', usage());
    expect(codexAdapter.calculateMessageCost(it.model, it.usage!, it)).toBe(0);
  });

  it('returns 0 for unknown Codex model (no throw)', () => {
    const it = item('unknown-gpt', usage({ input: 1_000_000, output: 100_000 }));
    expect(codexAdapter.calculateMessageCost(it.model, it.usage!, it)).toBe(0);
  });
});

// CF-418: Codex adapter's tooltip appends Cached (hit) / Reasoning sub-lines.
// Claude-only labels (Speed, Tier, Web searches) never appear.
describe('codexAdapter.extendCostTooltip', () => {
  // Reasoning total is preserved on the item via `reasoningTokens` so the
  // tooltip can show it even though it's already folded into `usage.output`
  // for billing.
  function item(usage: TokenUsage, reasoningTokens = 0): CodexAssistantItem {
    return {
      kind: 'assistant',
      lineId: '0',
      timestamp: '2026-05-13T01:00:00Z',
      text: 'x',
      phase: 'final',
      model: 'gpt-5',
      usage,
      reasoningTokens,
    };
  }

  const baseLines = ['$0.10', '', 'Input tokens (in): 0', 'Output tokens (out): 0'];

  it('appends Cached (hit) sub-line when cacheRead > 0', () => {
    const i = item({ input: 100, output: 0, cacheWrite: 0, cacheRead: 25 });
    const out = codexAdapter.extendCostTooltip!(baseLines, i.usage!, i);
    expect(out.some((l) => /Cached \(hit\):\s*25/.test(l))).toBe(true);
  });

  it('omits Cached (hit) sub-line when cacheRead is 0', () => {
    const i = item({ input: 100, output: 0, cacheWrite: 0, cacheRead: 0 });
    const out = codexAdapter.extendCostTooltip!(baseLines, i.usage!, i);
    expect(out.every((l) => !l.includes('Cached (hit)'))).toBe(true);
  });

  it('does NOT emit Claude-only labels (Speed, Tier, Web searches)', () => {
    const i = item({ input: 1000, output: 1000, cacheWrite: 0, cacheRead: 100 }, 50);
    const out = codexAdapter.extendCostTooltip!(baseLines, i.usage!, i);
    expect(out.every((l) => !/^Speed:/.test(l))).toBe(true);
    expect(out.every((l) => !/^Tier:/.test(l))).toBe(true);
    expect(out.every((l) => !/Web searches:/.test(l))).toBe(true);
  });
});

// CF-436: Codex defines tokensCostTooltip with OpenAI-flavored copy, and
// does NOT define tokensFastTooltip (no fast/priority tier on OpenAI).
describe('codexAdapter Tokens-card tooltips (CF-436)', () => {
  it('defines tokensCostTooltip referencing OpenAI model pricing', () => {
    expect(codexAdapter.tokensCostTooltip).toBe(
      'Estimated API cost based on token usage and OpenAI model pricing.',
    );
  });

  it('does not define tokensFastTooltip (no fast tier on OpenAI)', () => {
    expect(codexAdapter.tokensFastTooltip).toBeUndefined();
  });
});
