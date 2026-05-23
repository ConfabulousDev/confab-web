// CF-475 spec tests for the Codex deep-link target resolver.
//
// Contract:
//   - Empty target string => null.
//   - Unparseable target (Date.parse returns NaN) => null. Includes bare
//     digits like "42" (which some JS engines silently parse as year 42 AD)
//     — we treat as garbage to lock the policy.
//   - Empty items array => null regardless of target.
//   - Target before every item => index 0 ("closest row, never not-found").
//   - Target exactly matches an item's timestamp => that item's index.
//   - Target between two items => the earlier index (latest with
//     `timestamp <= target`).
//   - Target after the last item => last index.
//   - Items whose own timestamps are unparseable are skipped, not crashing
//     the scan.

import { describe, it, expect } from 'vitest';
import { resolveCodexDeepLinkTarget } from './resolveCodexDeepLinkTarget';
import type { CodexRenderItem } from '@/types/codexRenderItem';

function user(timestamp: string, lineId = '0'): CodexRenderItem {
  return { kind: 'user', lineId, timestamp, text: 'x' };
}

const T1 = '2026-05-13T18:00:00Z';
const T2 = '2026-05-13T18:01:00Z';
const T3 = '2026-05-13T18:02:00Z';
const T4 = '2026-05-13T18:03:00Z';

describe('resolveCodexDeepLinkTarget', () => {
  it('returns null when target is empty', () => {
    expect(resolveCodexDeepLinkTarget([user(T1)], '')).toBeNull();
  });

  it('returns null when target is unparseable garbage', () => {
    expect(resolveCodexDeepLinkTarget([user(T1)], 'not-a-date')).toBeNull();
  });

  it('returns null when target is a bare digit string', () => {
    // `Date.parse("42")` is implementation-defined; treat bare digits as
    // garbage so legacy lineId-style URLs never accidentally land somewhere.
    expect(resolveCodexDeepLinkTarget([user(T1)], '42')).toBeNull();
  });

  it('returns null when items array is empty', () => {
    expect(resolveCodexDeepLinkTarget([], T1)).toBeNull();
  });

  it('returns index 0 when target precedes every item', () => {
    const items = [user(T2), user(T3), user(T4)];
    expect(resolveCodexDeepLinkTarget(items, T1)).toBe(0);
  });

  it('returns the matched index on exact timestamp match', () => {
    const items = [user(T1), user(T2), user(T3)];
    expect(resolveCodexDeepLinkTarget(items, T2)).toBe(1);
  });

  it('returns the earlier index when target falls between two items', () => {
    const items = [user(T1), user(T3)];
    expect(resolveCodexDeepLinkTarget(items, T2)).toBe(0);
  });

  it('returns the last index when target is after every item', () => {
    const items = [user(T1), user(T2), user(T3)];
    expect(resolveCodexDeepLinkTarget(items, T4)).toBe(2);
  });

  it('handles a single item correctly (target at exact match)', () => {
    expect(resolveCodexDeepLinkTarget([user(T2)], T2)).toBe(0);
  });

  it('handles a single item correctly (target after)', () => {
    expect(resolveCodexDeepLinkTarget([user(T1)], T2)).toBe(0);
  });

  it('handles a single item correctly (target before)', () => {
    // Target precedes the only item — clamp to index 0.
    expect(resolveCodexDeepLinkTarget([user(T2)], T1)).toBe(0);
  });

  it('skips items with unparseable timestamps without crashing', () => {
    const items = [user(T1), user('garbage'), user(T3)];
    expect(resolveCodexDeepLinkTarget(items, T2)).toBe(0);
    expect(resolveCodexDeepLinkTarget(items, T3)).toBe(2);
  });

  it('accepts millisecond-precision ISO strings', () => {
    const items = [
      user('2026-05-13T18:00:00.000Z'),
      user('2026-05-13T18:00:00.500Z'),
      user('2026-05-13T18:00:01.000Z'),
    ];
    expect(
      resolveCodexDeepLinkTarget(items, '2026-05-13T18:00:00.750Z'),
    ).toBe(1);
  });
});
