// Shared formatting helpers for Codex transcript rendering.

/** Format an ISO timestamp into a short time-of-day for per-message rows. */
export function formatCodexTimestamp(timestamp: string): string {
  const date = new Date(timestamp);
  return date.toLocaleTimeString('en-US', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: true,
  });
}

/** Format a duration in milliseconds as `Ns` or `M:SS` for turn separators. */
export function formatDurationMs(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  const seconds = Math.round(ms / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  const remSec = seconds % 60;
  return `${minutes}:${remSec.toString().padStart(2, '0')}`;
}

/** Pull the leaf file name from a POSIX-style absolute path. */
export function leafFileName(path: string): string {
  const idx = path.lastIndexOf('/');
  return idx === -1 ? path : path.slice(idx + 1);
}

/**
 * Stringify a value for display. Returns strings as-is, JSON-encodes objects
 * with 2-space indent, and falls back to `String(value)` if serialization
 * fails (e.g. circular structures).
 */
export function stringifyForDisplay(value: unknown): string {
  if (typeof value === 'string') return value;
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}
