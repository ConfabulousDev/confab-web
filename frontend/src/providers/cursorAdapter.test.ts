// 6qwh/18n2: cursorAdapter satisfies the ProviderAdapter contract and delegates
// to cursorTranscriptService / cursorCategories. Mirrors opencodeAdapter.test.ts
// for parity. Cursor-specific facts (★): no model/token/cost on the wire, so
// extractModel returns undefined, calculateMessageCost degrades to 0 (empty
// pricing), and the cost-tooltip extras are absent.

import { describe, expect, it, vi, beforeEach } from 'vitest';
import {
  fetchParsedCursorTranscript,
  fetchNewCursorLines,
  normalizeCursorLines,
  extractCursorModel,
  parseCursorJSONL,
} from '@/services/cursorTranscriptService';
import {
  DEFAULT_CURSOR_FILTER_STATE,
  countCursorCategories,
  cursorItemMatchesFilter,
  type CursorRenderItem,
} from '@/components/session/cursorCategories';
import { cursorAdapter } from './cursorAdapter';

vi.mock('@/services/cursorTranscriptService', async () => {
  const actual = await vi.importActual<typeof import('@/services/cursorTranscriptService')>(
    '@/services/cursorTranscriptService',
  );
  return {
    ...actual,
    fetchParsedCursorTranscript: vi.fn(),
    fetchNewCursorLines: vi.fn(),
  };
});

beforeEach(() => {
  vi.clearAllMocks();
});

function line(obj: unknown): string {
  return JSON.stringify(obj);
}

const userLine = {
  role: 'user',
  message: { content: [{ type: 'text', text: 'Find all Go files' }] },
};

const assistantLine = {
  role: 'assistant',
  message: {
    content: [
      { type: 'text', text: 'I found 2 files.' },
      { type: 'tool_use', name: 'Glob', input: { glob_pattern: '**/*.go' } },
    ],
  },
};

function rawLines(...objs: unknown[]) {
  return parseCursorJSONL(objs.map(line).join('\n')).rawLines;
}

describe('cursorAdapter', () => {
  it('has id="cursor"', () => {
    expect(cursorAdapter.id).toBe('cursor');
  });

  it('fetchInitial delegates to fetchParsedCursorTranscript and reshapes the result', async () => {
    const raw = rawLines(userLine, assistantLine);
    const items = normalizeCursorLines(raw);
    vi.mocked(fetchParsedCursorTranscript).mockResolvedValue({
      sessionId: 's',
      items,
      rawLines: raw,
      totalLines: 2,
    });

    const result = await cursorAdapter.fetchInitial('s', 'session.jsonl', true);

    expect(fetchParsedCursorTranscript).toHaveBeenCalledWith('s', 'session.jsonl', true);
    expect(result.items).toBe(items);
    expect(result.totalLines).toBe(2);
    expect(result.raw).toBe(raw);
  });

  it('fetchIncremental delegates to fetchNewCursorLines and DERIVES newItems via normalize (★)', async () => {
    const raw = rawLines(assistantLine);
    vi.mocked(fetchNewCursorLines).mockResolvedValue({
      newRawLines: raw,
      newTotalLineCount: 7,
    });

    const result = await cursorAdapter.fetchIncremental('s', 'session.jsonl', 5);

    expect(fetchNewCursorLines).toHaveBeenCalledWith('s', 'session.jsonl', 5);
    expect(result.newRaw).toBe(raw);
    expect(result.newTotalLineCount).toBe(7);
    expect(result.newItems).toEqual(normalizeCursorLines(raw));
  });

  it('normalize delegates to normalizeCursorLines', () => {
    expect(cursorAdapter.normalize).toBe(normalizeCursorLines);
  });

  it('extractModel returns undefined — Cursor lines carry no model (★)', () => {
    const raw = rawLines(assistantLine);
    expect(cursorAdapter.extractModel(raw, [])).toBeUndefined();
    expect(cursorAdapter.extractModel(raw, [])).toBe(extractCursorModel());
  });

  it('computeMeta falls back to firstSeen/lastSyncAt (no per-line timestamp) (★)', () => {
    const meta = cursorAdapter.computeMeta([], [], {
      firstSeen: '2026-05-13T01:00:00Z',
      lastSyncAt: '2026-05-13T01:10:00Z',
    });
    expect(meta.durationMs).toBe(10 * 60 * 1000);
    expect(meta.sessionDate?.toISOString()).toBe('2026-05-13T01:00:00.000Z');
  });

  it('countCategories delegates to countCursorCategories', () => {
    const items = normalizeCursorLines(rawLines(userLine, assistantLine));
    expect(cursorAdapter.countCategories(items)).toEqual(countCursorCategories(items));
  });

  it('itemMatchesFilter delegates to cursorItemMatchesFilter', () => {
    const items = normalizeCursorLines(rawLines(userLine));
    if (items[0]) {
      expect(cursorAdapter.itemMatchesFilter(items[0], DEFAULT_CURSOR_FILTER_STATE)).toBe(
        cursorItemMatchesFilter(items[0], DEFAULT_CURSOR_FILTER_STATE),
      );
    }
  });

  it('exposes FilterDropdown and TranscriptPane as renderable components', () => {
    expect(typeof cursorAdapter.FilterDropdown).toBe('function');
    expect(typeof cursorAdapter.TranscriptPane).toBe('function');
  });

  it('exposes useFilters and useDeepLinkFilterReset as functions', () => {
    expect(typeof cursorAdapter.useFilters).toBe('function');
    expect(typeof cursorAdapter.useDeepLinkFilterReset).toBe('function');
  });
});

// Cursor has no usage/cost on any line, so calculateMessageCost degrades to 0
// (empty pricing). Non-assistant items are 0 too.
describe('cursorAdapter.calculateMessageCost', () => {
  const zeroUsage = { input: 0, output: 0, cacheWrite: 0, cacheWrite1h: 0, cacheRead: 0 };

  it('returns 0 for an assistant item (no pricing for cursor) (★)', () => {
    const item: CursorRenderItem = { kind: 'assistant', id: '1', text: 'x' };
    expect(cursorAdapter.calculateMessageCost('', zeroUsage, item)).toBe(0);
  });

  it('returns 0 for a non-assistant item (★)', () => {
    const item: CursorRenderItem = { kind: 'user', id: '0', text: 'hi' };
    expect(cursorAdapter.calculateMessageCost('', zeroUsage, item)).toBe(0);
  });
});

// Cursor defines its own tokensCostTooltip but, like OpenCode, neither
// extendCostTooltip nor tokensFastTooltip (no fast tier, no cost data).
describe('cursorAdapter Tokens-card tooltips', () => {
  it('defines a cost tooltip string', () => {
    expect(typeof cursorAdapter.tokensCostTooltip).toBe('string');
    expect(cursorAdapter.tokensCostTooltip.length).toBeGreaterThan(0);
  });

  it('does not define extendCostTooltip (★)', () => {
    expect(cursorAdapter.extendCostTooltip).toBeUndefined();
  });

  it('does not define tokensFastTooltip (★)', () => {
    expect(cursorAdapter.tokensFastTooltip).toBeUndefined();
  });
});
