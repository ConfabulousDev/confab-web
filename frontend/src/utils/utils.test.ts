import { describe, it, expect } from 'vitest';
import { formatBytes, formatDate, formatRelativeTime, stripAnsi } from './utils';

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

describe('formatDate', () => {
  it('should format ISO date string', () => {
    const date = '2024-01-15T10:30:00Z';
    const result = formatDate(date);
    expect(result).toContain('2024');
  });

  it('should handle date without Z suffix', () => {
    const date = '2024-01-15T10:30:00';
    const result = formatDate(date);
    expect(result).toBeTruthy();
  });
});

describe('formatRelativeTime', () => {
  it('should format time just now', () => {
    const now = new Date().toISOString();
    expect(formatRelativeTime(now)).toBe('just now');
  });

  it('should format seconds ago', () => {
    const past = new Date(Date.now() - 30000).toISOString();
    expect(formatRelativeTime(past)).toBe('30s ago');
  });

  it('should format minutes ago', () => {
    const past = new Date(Date.now() - 120000).toISOString();
    expect(formatRelativeTime(past)).toBe('2m ago');
  });

  it('should format hours ago', () => {
    const past = new Date(Date.now() - 7200000).toISOString();
    expect(formatRelativeTime(past)).toBe('2h ago');
  });

  it('should format days ago', () => {
    const past = new Date(Date.now() - 172800000).toISOString();
    expect(formatRelativeTime(past)).toBe('2d ago');
  });

  it('should format future seconds', () => {
    const future = new Date(Date.now() + 30000).toISOString();
    expect(formatRelativeTime(future)).toBe('in 30s');
  });

  it('should format future minutes', () => {
    const future = new Date(Date.now() + 120000).toISOString();
    expect(formatRelativeTime(future)).toBe('in 2m');
  });

  it('should format future hours', () => {
    const future = new Date(Date.now() + 7200000).toISOString();
    expect(formatRelativeTime(future)).toBe('in 2h');
  });

  it('should format future days', () => {
    const future = new Date(Date.now() + 172800000).toISOString();
    expect(formatRelativeTime(future)).toBe('in 2d');
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
