import { describe, it, expect } from 'vitest';
import { formatDuration, formatRelativeTime, formatModelName, formatRepoName, formatDateString, formatDateTime } from './formatting';

describe('formatDuration', () => {
  describe('without decimalSeconds option', () => {
    it('formats seconds', () => {
      expect(formatDuration(0)).toBe('0s');
      expect(formatDuration(1000)).toBe('1s');
      expect(formatDuration(45000)).toBe('45s');
      expect(formatDuration(59000)).toBe('59s');
    });

    it('formats minutes', () => {
      expect(formatDuration(60000)).toBe('1m');
      expect(formatDuration(120000)).toBe('2m');
      expect(formatDuration(300000)).toBe('5m');
    });

    it('formats minutes with seconds', () => {
      expect(formatDuration(90000)).toBe('1m 30s');
      expect(formatDuration(125000)).toBe('2m 5s');
      expect(formatDuration(179000)).toBe('2m 59s');
    });

    it('formats hours', () => {
      expect(formatDuration(3600000)).toBe('1h');
      expect(formatDuration(7200000)).toBe('2h');
    });

    it('formats hours with minutes', () => {
      expect(formatDuration(3900000)).toBe('1h 5m'); // 65 minutes
      expect(formatDuration(5400000)).toBe('1h 30m'); // 90 minutes
      expect(formatDuration(10800000)).toBe('3h'); // 180 minutes
    });

    it('formats days', () => {
      expect(formatDuration(86400000)).toBe('1d'); // 24 hours
      expect(formatDuration(172800000)).toBe('2d'); // 48 hours
    });

    it('formats days with hours', () => {
      expect(formatDuration(90000000)).toBe('1d 1h'); // 25 hours
      expect(formatDuration(129600000)).toBe('1d 12h'); // 36 hours
    });

    it('rounds to nearest second', () => {
      expect(formatDuration(500)).toBe('1s'); // 0.5s rounds to 1s
      expect(formatDuration(1499)).toBe('1s'); // 1.499s rounds to 1s
      expect(formatDuration(1500)).toBe('2s'); // 1.5s rounds to 2s
    });

    it('handles edge case of 59.5 seconds rounding to 1 minute', () => {
      // 59500ms = 59.5s, rounds to 60s = 1m
      expect(formatDuration(59500)).toBe('1m');
    });

    it('handles edge case of 119.5 seconds', () => {
      // 119500ms = 119.5s, rounds to 120s = 2m
      expect(formatDuration(119500)).toBe('2m');
    });
  });

  describe('with decimalSeconds option', () => {
    it('shows decimal for sub-minute durations', () => {
      expect(formatDuration(0, { decimalSeconds: true })).toBe('0.0s');
      expect(formatDuration(500, { decimalSeconds: true })).toBe('0.5s');
      expect(formatDuration(1000, { decimalSeconds: true })).toBe('1.0s');
      expect(formatDuration(1234, { decimalSeconds: true })).toBe('1.2s');
      expect(formatDuration(4200, { decimalSeconds: true })).toBe('4.2s');
      expect(formatDuration(59000, { decimalSeconds: true })).toBe('59.0s');
    });

    it('still uses integer seconds for minute+ durations', () => {
      expect(formatDuration(60000, { decimalSeconds: true })).toBe('1m');
      expect(formatDuration(90000, { decimalSeconds: true })).toBe('1m 30s');
      expect(formatDuration(3600000, { decimalSeconds: true })).toBe('1h');
      expect(formatDuration(3900000, { decimalSeconds: true })).toBe('1h 5m');
    });
  });
});

describe('formatRelativeTime', () => {
  it('formats recent times', () => {
    const now = new Date();
    const fiveMinAgo = new Date(now.getTime() - 5 * 60 * 1000).toISOString();
    expect(formatRelativeTime(fiveMinAgo)).toBe('5m ago');
  });

  it('formats hours ago', () => {
    const now = new Date();
    const twoHoursAgo = new Date(now.getTime() - 2 * 60 * 60 * 1000).toISOString();
    expect(formatRelativeTime(twoHoursAgo)).toBe('2h ago');
  });

  it('formats days ago', () => {
    const now = new Date();
    const threeDaysAgo = new Date(now.getTime() - 3 * 24 * 60 * 60 * 1000).toISOString();
    expect(formatRelativeTime(threeDaysAgo)).toBe('3d ago');
  });

  it('formats just now', () => {
    const now = new Date().toISOString();
    expect(formatRelativeTime(now)).toBe('just now');
  });
});

describe('formatModelName', () => {
  it('formats claude-3-5-sonnet model names', () => {
    expect(formatModelName('claude-3-5-sonnet-20241022')).toBe('claude-sonnet-3.5');
  });

  it('formats claude-opus-4 model names', () => {
    expect(formatModelName('claude-opus-4-20250514')).toBe('claude-opus-4');
  });

  it('formats claude-opus-4-5 model names', () => {
    expect(formatModelName('claude-opus-4-5-20251101')).toBe('claude-opus-4.5');
  });

  it('formats claude-sonnet-4-5 model names', () => {
    expect(formatModelName('claude-sonnet-4-5-20250514')).toBe('claude-sonnet-4.5');
  });

  it('handles simple variant names', () => {
    expect(formatModelName('opus')).toBe('claude-opus');
    expect(formatModelName('sonnet')).toBe('claude-sonnet');
    expect(formatModelName('haiku')).toBe('claude-haiku');
  });

  it('handles unknown models', () => {
    expect(formatModelName('gpt-4-turbo')).toBe('gpt-4-turbo');
    expect(formatModelName('custom-model-name-v1')).toBe('custom-model-name');
  });
});

describe('formatRepoName', () => {
  it('extracts repo name from GitHub HTTPS URL', () => {
    expect(formatRepoName('https://github.com/user/repo.git')).toBe('user/repo');
    expect(formatRepoName('https://github.com/org/project')).toBe('org/project');
  });

  it('extracts repo name from GitLab HTTPS URL', () => {
    expect(formatRepoName('https://gitlab.com/user/repo.git')).toBe('user/repo');
  });

  it('extracts repo name from SSH URL', () => {
    expect(formatRepoName('git@github.com:user/repo.git')).toBe('user/repo');
  });

  it('handles URLs without .git suffix', () => {
    expect(formatRepoName('https://github.com/user/repo')).toBe('user/repo');
  });

  it('returns fallback for unknown formats', () => {
    expect(formatRepoName('/local/path/to/repo')).toBe('to/repo');
    expect(formatRepoName('repo')).toBe('repo');
  });
});

describe('formatDateString', () => {
  it('formats ISO date strings', () => {
    // Just verify it returns a non-empty string (locale-dependent)
    const result = formatDateString('2024-01-15T10:30:00Z');
    expect(result).toBeTruthy();
    expect(typeof result).toBe('string');
  });

  it('handles dates without Z suffix', () => {
    const result = formatDateString('2024-01-15T10:30:00');
    expect(result).toBeTruthy();
  });
});

describe('formatDateTime', () => {
  it('formats Date objects', () => {
    const date = new Date('2024-01-15T10:30:00Z');
    const result = formatDateTime(date);
    expect(result).toBeTruthy();
    expect(typeof result).toBe('string');
    // Should include date parts
    expect(result).toMatch(/Jan|15|2024/);
  });
});
