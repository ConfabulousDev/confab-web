// Forward-compat fallback row for unrecognized Codex line shapes.
// Renders a small chip with the raw JSON behind a click-to-expand (shared
// UnknownRawDetails shell) so a new line type lands somewhere visible instead
// of being silently dropped, plus a CF-574 "Report this message" affordance.

import { useMemo } from 'react';
import {
  CODEX_UNKNOWN_REASON_LABELS,
  type CodexUnknownItem as CodexUnknownItemType,
} from '@/types/codexRenderItem';
import { computeKeyFingerprint } from '@/utils/reportUnknown';
import ReportUnknownButton from '@/components/transcript/ReportUnknownButton';
import UnknownRawDetails from '@/components/transcript/UnknownRawDetails';
import { formatCodexTimestamp, stringifyForDisplay } from './codexFormat';
import CodexRowActions from './CodexRowActions';
import styles from './CodexDividers.module.css';

export interface CodexUnknownItemProps {
  item: CodexUnknownItemType;
  /** Session ID for the per-row copy-link URL (CF-360). Optional in tests. */
  sessionId?: string;
  /** Hover/click selection — adds the .selected ring. */
  isSelected?: boolean;
  /** Never fires for unknown (not a speaker). Accepted for shape uniformity. */
  isNewSpeaker?: boolean;
  /** CF-360: this row is the deep-link landing target. */
  isDeepLinkTarget?: boolean;
  /** CF-359: search query — highlights matches inside the raw-JSON `<pre>`. */
  searchQuery?: string;
  /** CF-359: this row is the active (n-of-N) search match — adds the amber ring. */
  isCurrentSearchMatch?: boolean;
}

export default function CodexUnknownItem({
  item,
  sessionId,
  isSelected,
  isDeepLinkTarget,
  searchQuery,
  isCurrentSearchMatch,
}: CodexUnknownItemProps) {
  const raw = useMemo(() => stringifyForDisplay(item.rawLine), [item.rawLine]);

  return (
    <UnknownRawDetails
      label="Unrecognized line"
      rawText={raw}
      isSelected={isSelected}
      isDeepLinkTarget={isDeepLinkTarget}
      searchQuery={searchQuery}
      isCurrentSearchMatch={isCurrentSearchMatch}
      summaryAside={
        <span className={styles.unknownTimestamp}>{formatCodexTimestamp(item.timestamp)}</span>
      }
      actions={
        <>
          <ReportUnknownButton
            descriptor={{
              provider: 'codex',
              surface: 'line',
              type: item.unrecognizedType,
              reason: CODEX_UNKNOWN_REASON_LABELS[item.reason],
              keyFingerprint: computeKeyFingerprint(item.rawLine),
            }}
          />
          {sessionId && (
            <CodexRowActions
              sessionId={sessionId}
              timestamp={item.timestamp}
              copyText={raw}
              kindLabel="unrecognized row"
            />
          )}
        </>
      }
    />
  );
}
