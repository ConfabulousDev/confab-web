// et0r: subagent card rendered in place of an Agent tool_result (`launch`) or a
// `<task-notification>` completion (`finished`), with an "Open transcript"
// action that switches to the subagent's subtab.

import type { ReactNode } from 'react';
import { useOpenOnRisingEdge } from '@/hooks/useOpenOnRisingEdge';
import { formatDuration } from '@/components/transcript/timelineFormat';
import type { TranscriptThreadStatus } from '@/providers/types';
import { formatTokenCount } from '@/utils/tokenStats';
import { cx } from '@/utils/utils';
import { agentDisplayName, normalizeAgentStatus, type ClaudeAgentInfo } from './claudeAgentIndex';
import styles from './SubagentCard.module.css';

interface SubagentCardProps {
  variant: 'launch' | 'finished';
  agent: ClaudeAgentInfo;
  /** `finished` only: this notification's own `<status>` (repeats can differ). */
  status?: string;
  /** `finished` only: the notification `<summary>`. */
  summary?: string;
  /** Label for the collapsed raw-content disclosure. */
  rawLabel: string;
  /** Raw content (tool_result block / notification XML), collapsed by default. */
  children: ReactNode;
  onOpenThread?: (agentId: string) => void;
  isCurrentSearchMatch?: boolean;
}

// we3k D3: the pill shows the normalized status word, styled per status.
const STATUS_CLASS: Record<TranscriptThreadStatus, string | undefined> = {
  running: styles.statusRunning,
  completed: styles.statusCompleted,
  failed: styles.statusError,
  stopped: styles.statusStopped,
  unknown: undefined,
};

function plural(count: number, singular: string, pluralForm: string): string {
  return `${count} ${count === 1 ? singular : pluralForm}`;
}

function foregroundStats(agent: ClaudeAgentInfo): string[] {
  const stats: string[] = [];
  if (agent.totalDurationMs !== undefined) stats.push(formatDuration(agent.totalDurationMs));
  if (agent.totalTokens !== undefined) stats.push(`${formatTokenCount(agent.totalTokens)} tokens`);
  if (agent.totalToolUseCount !== undefined) {
    stats.push(plural(agent.totalToolUseCount, 'tool use', 'tool uses'));
  }
  return stats;
}

export default function SubagentCard({
  variant,
  agent,
  status,
  summary,
  rawLabel,
  children,
  onOpenThread,
  isCurrentSearchMatch,
}: SubagentCardProps) {
  const [rawOpen, setRawOpen] = useOpenOnRisingEdge(!!isCurrentSearchMatch);
  const isFinished = variant === 'finished';
  const shownStatus = normalizeAgentStatus(isFinished ? (status ?? agent.status) : agent.status);
  // The type chip only adds information when a description took the title.
  const title = agentDisplayName(agent);
  const showTypeChip = !!agent.description && !!agent.subagentType;
  const stats = isFinished ? [] : foregroundStats(agent);

  return (
    <div className={styles.card}>
      <div className={styles.header}>
        <span className={styles.badge}>{isFinished ? 'Subagent finished' : 'Subagent'}</span>
        <span className={styles.title}>{title}</span>
        {showTypeChip && <span className={styles.chip}>{agent.subagentType}</span>}
        {agent.model && <span className={styles.chip}>{agent.model}</span>}
        <span className={cx(styles.status, STATUS_CLASS[shownStatus])}>{shownStatus}</span>
      </div>
      {isFinished && summary && <div className={styles.summary}>{summary}</div>}
      {stats.length > 0 && (
        <div className={styles.stats}>
          {stats.map((stat) => (
            <span key={stat}>{stat}</span>
          ))}
        </div>
      )}
      {onOpenThread && (
        <button type="button" className={styles.openButton} onClick={() => onOpenThread(agent.agentId)}>
          Open transcript →
        </button>
      )}
      <details className={styles.raw} open={rawOpen} onToggle={(e) => setRawOpen(e.currentTarget.open)}>
        <summary>{rawLabel}</summary>
        {children}
      </details>
    </div>
  );
}
