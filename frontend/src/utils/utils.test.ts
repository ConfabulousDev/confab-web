import { describe, it, expect, vi, afterEach } from 'vitest';
import { clampToViewportX, formatBytes, stripAnsi } from './utils';
import { formatDateString, formatRelativeTime } from './formatting';

describe('formatBytes', () => {
  it('should format 0 bytes', () => {
    expect(formatBytes(0)).toBe('0 B');
  });

  it('should format bytes correctly', () => {
    expect(formatBytes(1024)).toBe('1 KB');
    expect(formatBytes(1024 * 1024)).toBe('1 MB');
    expect(formatBytes(1024 * 1024 * 1024)).toBe('1 GB');
  });

  it('should handle decimal values', () => {
    expect(formatBytes(1536)).toBe('1.5 KB');
    expect(formatBytes(1024 * 1.5)).toBe('1.5 KB');
  });
});

describe('formatDateString', () => {
  it('should format ISO date string', () => {
    const date = '2024-01-15T10:30:00Z';
    const result = formatDateString(date);
    expect(result).toContain('2024');
  });

  it('should handle date without Z suffix', () => {
    const date = '2024-01-15T10:30:00';
    const result = formatDateString(date);
    expect(result).toBeTruthy();
  });
});

describe('formatRelativeTime', () => {
  const NOW = new Date('2025-06-15T12:00:00Z').getTime();

  afterEach(() => {
    vi.useRealTimers();
  });

  it('should format time just now', () => {
    vi.useFakeTimers({ now: NOW });
    expect(formatRelativeTime(new Date(NOW).toISOString())).toBe('just now');
  });

  it('should format seconds ago', () => {
    vi.useFakeTimers({ now: NOW });
    expect(formatRelativeTime(new Date(NOW - 30000).toISOString())).toBe('30s ago');
  });

  it('should format minutes ago', () => {
    vi.useFakeTimers({ now: NOW });
    expect(formatRelativeTime(new Date(NOW - 120000).toISOString())).toBe('2m ago');
  });

  it('should format hours ago', () => {
    vi.useFakeTimers({ now: NOW });
    expect(formatRelativeTime(new Date(NOW - 7200000).toISOString())).toBe('2h ago');
  });

  it('should format days ago', () => {
    vi.useFakeTimers({ now: NOW });
    expect(formatRelativeTime(new Date(NOW - 172800000).toISOString())).toBe('2d ago');
  });

  it('should format future seconds', () => {
    vi.useFakeTimers({ now: NOW });
    expect(formatRelativeTime(new Date(NOW + 30000).toISOString())).toBe('in 30s');
  });

  it('should format future minutes', () => {
    vi.useFakeTimers({ now: NOW });
    expect(formatRelativeTime(new Date(NOW + 120000).toISOString())).toBe('in 2m');
  });

  it('should format future hours', () => {
    vi.useFakeTimers({ now: NOW });
    expect(formatRelativeTime(new Date(NOW + 7200000).toISOString())).toBe('in 2h');
  });

  it('should format future days', () => {
    vi.useFakeTimers({ now: NOW });
    expect(formatRelativeTime(new Date(NOW + 172800000).toISOString())).toBe('in 2d');
  });
});

describe('stripAnsi', () => {
  it('should strip color codes', () => {
    expect(stripAnsi('\x1b[31mred\x1b[0m')).toBe('red');
    expect(stripAnsi('\x1b[1;32mbold green\x1b[0m')).toBe('bold green');
  });

  it('should strip cursor movement codes', () => {
    expect(stripAnsi('\x1b[2Jcleared\x1b[H')).toBe('cleared');
    expect(stripAnsi('\x1b[10Aup 10 lines')).toBe('up 10 lines');
  });

  it('should handle text without ANSI codes', () => {
    expect(stripAnsi('plain text')).toBe('plain text');
    expect(stripAnsi('')).toBe('');
  });

  it('should handle multiple ANSI codes in sequence', () => {
    expect(stripAnsi('\x1b[1m\x1b[31m\x1b[44mbold red on blue\x1b[0m')).toBe('bold red on blue');
  });

  it('should handle unicode escape notation', () => {
    // This is how it appears in JSON strings
    const input = 'prefix\x1b[32mgreen\x1b[0msuffix';
    expect(stripAnsi(input)).toBe('prefixgreensuffix');
  });
});

describe('clampToViewportX', () => {
  // 7grf: popovers and tooltips must stay inside a phone-width viewport.
  it('keeps a left edge that already fits', () => {
    expect(clampToViewportX(20, 100, 375, 8)).toBe(20);
  });

  it('pulls a box that passes the right edge back inside the margin', () => {
    expect(clampToViewportX(100, 300, 375, 8)).toBe(67);
  });

  it('pushes a box that starts left of the margin to the margin', () => {
    expect(clampToViewportX(-30, 100, 375, 8)).toBe(8);
  });

  it('pins a box wider than the viewport to the left margin', () => {
    expect(clampToViewportX(50, 500, 375, 8)).toBe(8);
  });

  it('accepts a box exactly as wide as the space between the margins', () => {
    expect(clampToViewportX(100, 359, 375, 8)).toBe(8);
  });
});
