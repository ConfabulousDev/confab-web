import { describe, it, expect, vi, afterEach } from 'vitest';
import { getDefaultDateRange, getDatePresets, parseDateRangeFromURL } from './dateRange';

// Pin "today" so tests are deterministic
// Wednesday, 2025-07-16
const FAKE_NOW = new Date(2025, 6, 16, 10, 30, 0).getTime();

describe('getDefaultDateRange', () => {
  afterEach(() => vi.useRealTimers());

  it('returns a "Last 7 Days" range ending today', () => {
    vi.useFakeTimers({ now: FAKE_NOW });
    const range = getDefaultDateRange();
    expect(range.label).toBe('Last 7 Days');
    expect(range.endDate).toBe('2025-07-16');
    expect(range.startDate).toBe('2025-07-10');
  });
});

describe('getDatePresets', () => {
  afterEach(() => vi.useRealTimers());

  it('returns 6 presets with expected labels', () => {
    vi.useFakeTimers({ now: FAKE_NOW });
    const presets = getDatePresets();
    expect(presets.map((p) => p.label)).toEqual([
      'This Week',
      'Last Week',
      'This Month',
      'Last Month',
      'Last 30 Days',
      'Last 90 Days',
    ]);
  });

  it('computes correct dates for a mid-week Wednesday', () => {
    // Wednesday 2025-07-16
    vi.useFakeTimers({ now: FAKE_NOW });
    const presets = getDatePresets();
    const byLabel = Object.fromEntries(presets.map((p) => [p.label, p]));

    // This Week: Monday Jul 14 - today Jul 16
    expect(byLabel['This Week']?.startDate).toBe('2025-07-14');
    expect(byLabel['This Week']?.endDate).toBe('2025-07-16');

    // Last Week: Mon Jul 7 - Sun Jul 13
    expect(byLabel['Last Week']?.startDate).toBe('2025-07-07');
    expect(byLabel['Last Week']?.endDate).toBe('2025-07-13');

    // This Month: Jul 1 - today
    expect(byLabel['This Month']?.startDate).toBe('2025-07-01');
    expect(byLabel['This Month']?.endDate).toBe('2025-07-16');

    // Last Month: Jun 1 - Jun 30
    expect(byLabel['Last Month']?.startDate).toBe('2025-06-01');
    expect(byLabel['Last Month']?.endDate).toBe('2025-06-30');

    // Last 30 Days: Jun 17 - Jul 16
    expect(byLabel['Last 30 Days']?.startDate).toBe('2025-06-17');
    expect(byLabel['Last 30 Days']?.endDate).toBe('2025-07-16');

    // Last 90 Days: Apr 18 - Jul 16
    expect(byLabel['Last 90 Days']?.startDate).toBe('2025-04-18');
    expect(byLabel['Last 90 Days']?.endDate).toBe('2025-07-16');
  });

  it('handles Sunday correctly (week starts on previous Monday)', () => {
    // Sunday 2025-07-20
    vi.useFakeTimers({ now: new Date(2025, 6, 20, 10, 0, 0).getTime() });
    const presets = getDatePresets();
    const thisWeek = presets.find((p) => p.label === 'This Week');
    // Monday of that week is Jul 14
    expect(thisWeek?.startDate).toBe('2025-07-14');
    expect(thisWeek?.endDate).toBe('2025-07-20');
  });

  it('handles Monday correctly (week starts today)', () => {
    // Monday 2025-07-14
    vi.useFakeTimers({ now: new Date(2025, 6, 14, 10, 0, 0).getTime() });
    const presets = getDatePresets();
    const thisWeek = presets.find((p) => p.label === 'This Week');
    expect(thisWeek?.startDate).toBe('2025-07-14');
    expect(thisWeek?.endDate).toBe('2025-07-14');
  });

  it('handles January (last month crosses year boundary)', () => {
    // Wednesday 2025-01-15
    vi.useFakeTimers({ now: new Date(2025, 0, 15, 10, 0, 0).getTime() });
    const presets = getDatePresets();
    const lastMonth = presets.find((p) => p.label === 'Last Month');
    expect(lastMonth?.startDate).toBe('2024-12-01');
    expect(lastMonth?.endDate).toBe('2024-12-31');
  });

  it('all presets have startDate <= endDate', () => {
    vi.useFakeTimers({ now: FAKE_NOW });
    for (const preset of getDatePresets()) {
      expect(preset.startDate <= preset.endDate).toBe(true);
    }
  });
});

describe('parseDateRangeFromURL', () => {
  afterEach(() => vi.useRealTimers());

  it('returns null when params are missing', () => {
    expect(parseDateRangeFromURL(new URLSearchParams())).toBeNull();
    expect(parseDateRangeFromURL(new URLSearchParams('start=2025-01-01'))).toBeNull();
    expect(parseDateRangeFromURL(new URLSearchParams('end=2025-01-07'))).toBeNull();
  });

  it('returns null for invalid date formats', () => {
    expect(parseDateRangeFromURL(new URLSearchParams('start=2025-1-1&end=2025-1-7'))).toBeNull();
    expect(parseDateRangeFromURL(new URLSearchParams('start=bad&end=2025-01-07'))).toBeNull();
  });

  it('parses valid date params and infers label', () => {
    vi.useFakeTimers({ now: FAKE_NOW });
    const params = new URLSearchParams('start=2025-07-10&end=2025-07-16');
    const range = parseDateRangeFromURL(params);
    expect(range).toEqual({
      startDate: '2025-07-10',
      endDate: '2025-07-16',
      label: 'Last 7 Days',
    });
  });

  it('falls back to date range string when no preset matches', () => {
    vi.useFakeTimers({ now: FAKE_NOW });
    const params = new URLSearchParams('start=2025-05-01&end=2025-05-15');
    const range = parseDateRangeFromURL(params);
    expect(range?.label).toBe('2025-05-01 - 2025-05-15');
  });
});
