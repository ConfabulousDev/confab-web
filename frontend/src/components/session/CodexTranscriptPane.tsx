// Renders the transcript-tab content for Codex sessions.
//
// CF-386: presentational. Fetch + poll moved up to `SessionViewer` so the
// session header can derive the model on the Summary tab too.
// CF-361: render items + filter inputs are now also lifted to `SessionViewer`
// so the same `items` stream can drive both the category counts in the
// header and the row list here.

import CodexMessageTimeline from '@/components/transcript/codex/CodexMessageTimeline';
import type { CodexRenderItem } from '@/types/codexRenderItem';
import TranscriptPaneStatus from './TranscriptPaneStatus';

export interface CodexTranscriptPaneProps {
  sessionId: string;
  /** Unfiltered render items — drives the timeline bar's segment layout. */
  items: CodexRenderItem[];
  /** Post-filter render items — drives the row list. Equals `items` when no filter. */
  filteredItems: CodexRenderItem[];
  /**
   * CF-361: indices into `items` whose category passes the active filter.
   * Forwarded to the timeline bar so filtered segments render greyed.
   * `undefined` means no filter is active.
   */
  visibleIndices?: Set<number>;
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
  /** CF-362: cost mode (per-message badges + CostBar side rail). */
  isCostMode?: boolean;
}

export default function CodexTranscriptPane({
  sessionId,
  items,
  filteredItems,
  visibleIndices,
  loading,
  error,
  targetLineId,
  isCostMode,
}: CodexTranscriptPaneProps) {
  if (loading || error) {
    return <TranscriptPaneStatus loading={loading} error={error} />;
  }

  return (
    <CodexMessageTimeline
      items={items}
      filteredItems={filteredItems}
      visibleIndices={visibleIndices}
      sessionId={sessionId}
      targetLineId={targetLineId}
      isCostMode={isCostMode}
    />
  );
}
