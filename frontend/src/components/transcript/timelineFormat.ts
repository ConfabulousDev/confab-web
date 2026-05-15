/**
 * Format a duration in milliseconds for timeline tooltips on both the Claude
 * `TimelineBar` and the Codex `CodexTimelineBar`. Renders as `1h 15m`,
 * `5m 30s`, `42s`, or `500ms` depending on magnitude — preferred over the
 * `M:SS` colon style in `codexFormat.ts` for tooltip prose.
 */
export function formatDuration(ms: number): string {
  const seconds = Math.floor(ms / 1000);
  if (seconds < 1) return `${ms}ms`;

  const minutes = Math.floor(seconds / 60);
  if (minutes < 1) return `${seconds}s`;

  const hours = Math.floor(minutes / 60);
  if (hours < 1) {
    const rs = seconds % 60;
    return rs > 0 ? `${minutes}m ${rs}s` : `${minutes}m`;
  }

  const rm = minutes % 60;
  return rm > 0 ? `${hours}h ${rm}m` : `${hours}h`;
}
