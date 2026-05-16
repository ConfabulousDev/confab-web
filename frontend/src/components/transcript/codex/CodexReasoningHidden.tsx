// Minimal "(reasoning hidden)" marker for opaque encrypted reasoning lines.
//
// Forward-compat (CF-358): if a future Codex `reasoning` line ever carries
// plaintext, extend `CodexReasoningHiddenItem` with a `{ decoded: true;
// text: string }` discriminator and render a 💭 collapsible block here,
// mirroring `ContentBlock.tsx` thinking-block treatment. Today's normalizer
// only emits the encrypted/hidden case, so this component stays minimal.

import type { CodexReasoningHiddenItem } from '@/types/codexRenderItem';
import { cx } from '@/utils/utils';
import { formatCodexTimestamp } from './codexFormat';
import CodexRowActions from './CodexRowActions';
import styles from './CodexDividers.module.css';

export interface CodexReasoningHiddenProps {
  item: CodexReasoningHiddenItem;
  /** Session ID for the per-row copy-link URL (CF-360). Optional in tests. */
  sessionId?: string;
  /** Hover/click selection — adds the .selected ring. */
  isSelected?: boolean;
  /** Never fires for reasoning_hidden (not a speaker). Accepted for shape uniformity. */
  isNewSpeaker?: boolean;
  /** CF-360: this row is the deep-link landing target. */
  isDeepLinkTarget?: boolean;
  /** CF-359: accepted for shape uniformity. Extractor returns "" for this
   *  kind so the row never matches; no highlighting branch needed. */
  searchQuery?: string;
  /** CF-359: accepted for shape uniformity. See `searchQuery` note above. */
  isCurrentSearchMatch?: boolean;
}

export default function CodexReasoningHidden({
  item,
  sessionId,
  isSelected,
  isDeepLinkTarget,
  isCurrentSearchMatch,
}: CodexReasoningHiddenProps) {
  const className = cx(
    styles.reasoningHidden,
    isSelected && styles.selected,
    isDeepLinkTarget && styles.deepLinkTarget,
    isCurrentSearchMatch && styles.searchMatch,
  );
  return (
    <div className={className} data-kind="reasoning_hidden">
      <span className={styles.reasoningIcon} aria-hidden="true">
        🔒
      </span>
      <span>reasoning hidden</span>
      <span className={styles.reasoningTimestamp}>
        {formatCodexTimestamp(item.timestamp)}
      </span>
      {sessionId && (
        <CodexRowActions
          sessionId={sessionId}
          lineId={item.lineId}
          kindLabel="reasoning marker"
        />
      )}
    </div>
  );
}
