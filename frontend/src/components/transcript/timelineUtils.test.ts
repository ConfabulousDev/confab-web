// 6h7m: unit tests for the shared day-boundary/idle-gap divider decision and
// label functions. These are the single seam all 4 providers' *VirtualItems
// builders funnel through, so correctness here is load-bearing for every
// provider's divider placement.

import { describe, it, expect } from 'vitest';
import { shouldShowDivider, formatDividerLabel, TIME_GAP_THRESHOLD_MS } from './timelineUtils';

// Constructed with the local-time Date(year, month, day, ...) form (NOT ISO
// 'Z' strings) throughout this file: day-boundary math is local-calendar-day
// based (matching the existing formatTimeSeparator convention), so a fixture
// built from a UTC string would shift days depending on the test runner's TZ.
const MAY_13_18_00 = new Date(2026, 4, 13, 18, 0, 0).getTime();

describe('shouldShowDivider', () => {
  it('does not show a divider when there is no previous known timestamp (first item)', () => {
    const result = shouldShowDivider(MAY_13_18_00, undefined);
    expect(result).toEqual({ show: false, dayChanged: false });
  });

  it('does not show a divider for a same-day gap under the 5min threshold', () => {
    const result = shouldShowDivider(MAY_13_18_00, MAY_13_18_00 - 60_000);
    expect(result).toEqual({ show: false, dayChanged: false });
  });

  it('shows a divider (no day change) for a same-day gap over the 5min threshold', () => {
    const result = shouldShowDivider(MAY_13_18_00, MAY_13_18_00 - (TIME_GAP_THRESHOLD_MS + 1));
    expect(result).toEqual({ show: true, dayChanged: false });
  });

  it('does not show a divider for a same-day gap exactly at the 5min threshold', () => {
    const result = shouldShowDivider(MAY_13_18_00, MAY_13_18_00 - TIME_GAP_THRESHOLD_MS);
    expect(result).toEqual({ show: false, dayChanged: false });
  });

  it('shows a divider with dayChanged=true when the calendar day changes even with a tiny gap', () => {
    // 11:59pm -> 12:01am: a 2-minute gap that crosses midnight.
    const may13_2359 = new Date(2026, 4, 13, 23, 59, 0).getTime();
    const may14_0001 = new Date(2026, 4, 14, 0, 1, 0).getTime();
    const result = shouldShowDivider(may14_0001, may13_2359);
    expect(result).toEqual({ show: true, dayChanged: true });
  });

  it('shows a divider when the day changes even with a very large gap', () => {
    const dayLater = MAY_13_18_00 + 24 * 60 * 60 * 1000;
    const result = shouldShowDivider(dayLater, MAY_13_18_00);
    expect(result).toEqual({ show: true, dayChanged: true });
  });

  it('does not show a divider for two timestamps on the same day far apart intraday', () => {
    const morning = new Date(2026, 4, 13, 0, 5, 0).getTime();
    const evening = new Date(2026, 4, 13, 0, 10, 0).getTime();
    const result = shouldShowDivider(evening, morning);
    expect(result).toEqual({ show: false, dayChanged: false });
  });
});

describe('formatDividerLabel', () => {
  it('returns a full weekday+month+day label when dayChanged is true', () => {
    const label = formatDividerLabel(new Date(2026, 6, 7, 0, 1, 0).getTime(), true);
    // e.g. "Tuesday, July 7" - locale-dependent exact format, so check the pieces.
    expect(label).toMatch(/\w+day/); // contains a weekday name
    expect(label).toContain('July');
    expect(label).toContain('7');
  });

  it('does not include a weekday/full-date format when dayChanged is false', () => {
    const label = formatDividerLabel(new Date().getTime(), false);
    // Falls back to the existing idle-gap time-only/short-date text, which
    // never contains a full weekday name.
    expect(label).not.toMatch(/day,/);
  });
});
