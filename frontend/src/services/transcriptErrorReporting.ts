import type { TranscriptValidationError } from '@/schemas/claudeTranscript';

const MAX_ERRORS_PER_REPORT = 50;

/**
 * Report transcript validation errors to the backend for observability.
 * Shared by the per-provider transcript services (Claude, Codex), which pass
 * their own `category` (e.g. `transcript_validation` vs
 * `codex_transcript_validation`) so the two can be triaged independently (c8gn).
 *
 * Uses raw fetch (bypasses APIClient) so 401s don't redirect the user.
 * Fire-and-forget: errors are silently ignored. Per-session dedup is the
 * caller's responsibility (each service keeps its own reported-sessions set).
 */
export function reportTranscriptErrors(
  sessionId: string,
  errors: TranscriptValidationError[],
  category: string,
): void {
  const payload = {
    category,
    session_id: sessionId,
    errors: errors.slice(0, MAX_ERRORS_PER_REPORT).map((e) => ({
      line: e.line,
      message_type: e.messageType,
      details: e.errors.map((d) => ({
        path: d.path,
        message: d.message,
        expected: d.expected,
        received: d.received,
      })),
      raw_json_preview: e.rawJson.slice(0, 500),
    })),
    context: {
      url: typeof window !== 'undefined' ? window.location.pathname : undefined,
      user_agent: typeof navigator !== 'undefined' ? navigator.userAgent : undefined,
    },
  };

  fetch('/api/v1/client-errors', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify(payload),
  }).catch(() => {}); // Fire-and-forget
}
