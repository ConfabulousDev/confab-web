import type { TranscriptLine } from '@/types';

interface SessionMetaInput {
  firstSeen?: string;
  lastSyncAt?: string | null;
}

interface SessionMeta {
  durationMs: number | undefined;
  sessionDate: Date | undefined;
  firstTimestamp: number | undefined;
  lastTimestamp: number | undefined;
}

/**
 * Compute session metadata (duration, date) from message timestamps.
 *
 * This matches how the backend analytics computes duration_ms - by finding
 * the earliest and latest timestamps across all messages. This is more accurate
 * than using session.first_seen/last_sync_at, especially for resumed sessions
 * where those metadata fields may not reflect the full session history.
 *
 * Falls back to session metadata timestamps if no messages are available.
 */
export function computeSessionMeta(
  messages: TranscriptLine[],
  session: SessionMetaInput
): SessionMeta {
  let firstTimestamp: number | undefined;
  let lastTimestamp: number | undefined;

  // Find earliest and latest timestamps from messages
  for (const msg of messages) {
    if (msg.type === 'user' || msg.type === 'assistant') {
      const ts = new Date(msg.timestamp).getTime();
      if (!isNaN(ts)) {
        if (firstTimestamp === undefined || ts < firstTimestamp) {
          firstTimestamp = ts;
        }
        if (lastTimestamp === undefined || ts > lastTimestamp) {
          lastTimestamp = ts;
        }
      }
    }
  }

  // Compute duration from message timestamps
  let durationMs: number | undefined;
  if (firstTimestamp !== undefined && lastTimestamp !== undefined && lastTimestamp > firstTimestamp) {
    durationMs = lastTimestamp - firstTimestamp;
  }

  // Fall back to session timestamps if no messages
  if (durationMs === undefined && session.firstSeen && session.lastSyncAt) {
    const start = new Date(session.firstSeen).getTime();
    const end = new Date(session.lastSyncAt).getTime();
    if (end > start) {
      durationMs = end - start;
    }
  }

  // Get session date from earliest message timestamp or session.firstSeen
  let sessionDate: Date | undefined;
  if (firstTimestamp !== undefined) {
    sessionDate = new Date(firstTimestamp);
  } else if (session.firstSeen) {
    sessionDate = new Date(session.firstSeen);
  }

  return { durationMs, sessionDate, firstTimestamp, lastTimestamp };
}
