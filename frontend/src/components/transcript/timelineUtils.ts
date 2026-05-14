// Helpers shared between the Claude and Codex timeline views. Kept narrow:
// only formatting + scroll-retry logic that has identical semantics on both
// sides. The per-stream `shouldShowTimeSeparator` predicates stay near their
// own data shapes (TranscriptLine vs CodexRenderItem).

/**
 * Format a timestamp for the >5min idle-gap divider. Today → time-of-day;
 * otherwise short date + time. Used by both `MessageTimeline` (Claude) and
 * `CodexMessageTimeline` (Codex) so the divider reads the same in both views.
 */
export function formatTimeSeparator(timestamp: string): string {
  const date = new Date(timestamp);
  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const messageDate = new Date(date.getFullYear(), date.getMonth(), date.getDate());

  if (messageDate.getTime() === today.getTime()) {
    return date.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' });
  }
  return date.toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

/**
 * Repeatedly call `action` across animation frames until `shouldStop` returns
 * true or `maxAttempts` is reached. Used by both timeline views for virtual
 * scroll positioning, where item sizes are estimated until measured and a
 * single `scrollToIndex` call lands short of the target.
 */
export function retryOnAnimationFrame(
  action: () => void,
  shouldStop: () => boolean,
  maxAttempts = 5,
): void {
  function attempt(n: number): void {
    action();
    if (n < maxAttempts && !shouldStop()) {
      requestAnimationFrame(() => attempt(n + 1));
    }
  }
  attempt(0);
}
