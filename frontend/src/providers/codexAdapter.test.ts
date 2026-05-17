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
