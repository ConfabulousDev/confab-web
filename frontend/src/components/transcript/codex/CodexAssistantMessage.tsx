// Renders a Codex assistant message. `phase: 'commentary'` styling is lighter
// weight than the default/final styling so commentary is visually subordinate
// to the final answer in the same turn.

import type { CodexAssistantItem } from '@/types/codexRenderItem';
import {
  buildCodexCostTooltip,
  formatCost,
  formatTokenCount,
} from '@/utils/tokenStats';
import { cx } from '@/utils/utils';
import { formatCodexTimestamp } from './codexFormat';
import CodexMessageBody from './CodexMessageBody';
import CodexMessageImages from './CodexMessageImages';
import CodexRowActions from './CodexRowActions';
import styles from './CodexMessage.module.css';

export interface CodexAssistantMessageProps {
  item: CodexAssistantItem;
  /**
   * Session ID for the per-row copy-link URL. Optional so the renderer can
   * be used in isolation; timeline always passes it in production.
   */
  sessionId?: string;
  /** Hover/click selection — adds the .selected ring. */
  isSelected?: boolean;
  /** Speaker kind differs from the previous speaker (tool_call doesn't count). */
  isNewSpeaker?: boolean;
  /** CF-360: this row is the deep-link landing target. */
  isDeepLinkTarget?: boolean;
  /** Skip to next same-kind row (CF-360). */
  onSkipToNext?: () => void;
  /** Skip to previous same-kind row (CF-360). */
  onSkipToPrevious?: () => void;
  /** Human-readable kind for aria-label (CF-360). */
  kindLabel?: string;
  /** CF-359: transcript search query — wraps matches in `<mark>` in the body. */
  searchQuery?: string;
  /** CF-359: this row is the active (n-of-N) search match — adds the amber ring. */
  isCurrentSearchMatch?: boolean;
  /** CF-362: cost mode toggle — when true, render $ / token / cache badges. */
  isCostMode?: boolean;
  /** CF-362: precomputed cost for this row. Badges suppress when 0/missing. */
  messageCost?: number;
}

export default function CodexAssistantMessage({
  item,
  sessionId,
  isSelected,
  isNewSpeaker,
  isDeepLinkTarget,
  onSkipToNext,
  onSkipToPrevious,
  kindLabel,
  searchQuery,
  isCurrentSearchMatch,
  isCostMode,
  messageCost,
}: CodexAssistantMessageProps) {
  const phaseClass = item.phase === 'commentary' ? styles.commentary : styles.final;
  const className = cx(
    styles.message,
    styles.assistant,
    phaseClass,
    isSelected && styles.selected,
    isNewSpeaker && styles.newSpeaker,
    isDeepLinkTarget && styles.deepLinkTarget,
    isCurrentSearchMatch && styles.searchMatch,
  );
  const defaultLabel =
    item.phase === 'commentary' ? 'assistant commentary' : 'assistant answer';

  // CF-362: badges render only when cost mode is on AND we have both usage
  // and a positive cost. Zero-cost rows / rows missing usage stay clean.
  const costBadges =
    isCostMode && item.usage !== undefined && messageCost !== undefined && messageCost > 0
      ? {
          usage: item.usage,
          cost: messageCost,
          tooltip: buildCodexCostTooltip(item.usage, messageCost),
          outputDisplay: item.usage.output_tokens + (item.usage.reasoning_output_tokens ?? 0),
          cachedHit: item.usage.cached_input_tokens ?? 0,
        }
      : null;

  return (
    <div
      className={className}
      data-kind="assistant"
      data-phase={item.phase}
    >
      <div className={styles.header}>
        <span className={styles.role}>
          {item.phase === 'commentary' ? 'Assistant (commentary)' : 'Assistant'}
        </span>
        <span className={styles.modelBadge}>{item.model}</span>
        <span className={styles.timestamp}>{formatCodexTimestamp(item.timestamp)}</span>
        {costBadges && (
          <>
            <span className={styles.costBadge} title={costBadges.tooltip}>
              {formatCost(costBadges.cost)}
            </span>
            <span className={styles.tokenPill} title={costBadges.tooltip}>
              {formatTokenCount(costBadges.usage.input_tokens)} in &middot;{' '}
              {formatTokenCount(costBadges.outputDisplay)} out
            </span>
            {costBadges.cachedHit > 0 && (
              <span className={styles.cachePill} title={costBadges.tooltip}>
                {formatTokenCount(costBadges.cachedHit)} hit
              </span>
            )}
          </>
        )}
        {sessionId && (
          <CodexRowActions
            sessionId={sessionId}
            lineId={item.lineId}
            copyText={item.text}
            onSkipToNext={onSkipToNext}
            onSkipToPrevious={onSkipToPrevious}
            kindLabel={kindLabel ?? defaultLabel}
          />
        )}
      </div>
      <div className={styles.body}>
        <CodexMessageBody
          text={item.text}
          searchQuery={searchQuery}
          isCurrentSearchMatch={isCurrentSearchMatch}
        />
        {item.images && (
          <CodexMessageImages images={item.images} altPrefix="Assistant-generated image" />
        )}
      </div>
    </div>
  );
}
