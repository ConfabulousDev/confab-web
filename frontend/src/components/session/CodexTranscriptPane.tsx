// Renders the transcript-tab content for Codex sessions.
//
// CF-386: presentational. Fetch + poll moved up to `SessionViewer` so the
// session header can derive the model on the Summary tab too (mirrors how
// Claude state is owned by `SessionViewer`). This component now just
// normalizes the raw lines into render items and hands them to the timeline.

import { useMemo } from 'react';
import { normalizeCodexLines } from '@/services/codexTranscriptService';
import type { RawCodexLine } from '@/schemas/codexTranscript';
import CodexMessageTimeline from '@/components/transcript/codex/CodexMessageTimeline';
import styles from './CodexTranscriptPane.module.css';

export interface CodexTranscriptPaneProps {
  sessionId: string;
  /** Parsed rollout lines owned by `SessionViewer`. */
  rawLines: RawCodexLine[];
  /** True while the initial rollout fetch is in flight. */
  loading: boolean;
  /** Error message from the rollout fetch, if any. */
  error: string | null;
  /**
   * Deep-link target row, addressed by its `lineId` (CF-360). The same URL
   * `?msg=` parameter that Claude uses for UUID-based deep links; for Codex,
   * the string is reinterpreted as a lineId.
   */
  targetLineId?: string;
  /**
   * RESERVED placeholder for CF-361 — no consumer yet. Pass-through to
   * `CodexMessageTimeline`; see its prop doc for the planned semantics.
   */
  targetLineIdHidden?: boolean;
}

export default function CodexTranscriptPane({
  sessionId,
  rawLines,
  loading,
  error,
  targetLineId,
  targetLineIdHidden,
}: CodexTranscriptPaneProps) {
  // Re-derive render items whenever raw lines change. Pure, cheap inside useMemo.
  const items = useMemo(() => normalizeCodexLines(rawLines), [rawLines]);

  if (loading) {
    return <div className={styles.loading}>Loading transcript...</div>;
  }
  if (error) {
    return (
      <div className={styles.error}>
        <strong>Error:</strong> {error}
      </div>
    );
  }

  return (
    <CodexMessageTimeline
      items={items}
      sessionId={sessionId}
      targetLineId={targetLineId}
      targetLineIdHidden={targetLineIdHidden}
    />
  );
}
