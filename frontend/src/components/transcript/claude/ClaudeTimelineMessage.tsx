import { useMemo } from 'react';
import type { TranscriptLine, ContentBlock, TextBlock } from '@/types';
import type { TIL } from '@/schemas/api';
import type { TokenUsage } from '@/utils/tokenStats';
import { isTextBlock, isToolUseBlock, isToolResultBlock, isFileHistorySnapshot, isUserMessage, isAssistantMessage, isSystemMessage, isSummaryMessage, isAttachmentMessage, isCommandExpansionMessage, getCommandExpansionSkillName, stripCommandExpansionTags } from '@/types';
import { useCopyToClipboard } from '@/hooks';
import ContentBlockComponent from '@/components/transcript/claude/ContentBlock';
import { AttachmentContent, AwaySummary } from '@/components/transcript/claude/attachments';
import TILBadge from '@/components/session/TILBadge';
import { formatCost, formatTokenCount, buildCostTooltip, normalizeClaudeUsage, computeMessageTokenSpeed, formatTokenSpeed } from '@/utils/tokenStats';
import { claudeAdapter } from '@/providers/claudeAdapter';
import { getClaudeRoleLabel } from '@/components/session/claudeCategories';
import styles from './ClaudeTimelineMessage.module.css';

interface ClaudeTimelineMessageProps {
  message: TranscriptLine;
  toolNameMap: Map<string, string>;
  previousMessage?: TranscriptLine;
  isSelected?: boolean;
  isDeepLinkTarget?: boolean;
  /** Whether this message is the currently active search match (drives both
   *  the amber box-shadow and the active highlight color on inline marks) */
  isCurrentSearchMatch?: boolean;
  searchQuery?: string;
  sessionId?: string;
  onSkipToNext?: () => void;
  onSkipToPrevious?: () => void;
  roleLabel?: string;
  isCostMode?: boolean;
  messageCost?: number;
  /** Corrected token usage from the final (last) line of a deduplicated message group.
   *  Used for tooltip display when available, since the raw message.usage may have
   *  intermediate output_tokens (not the final count). */
  correctedTokenUsage?: TokenUsage;
  /** TILs anchored to this message */
  tils?: TIL[];
}

/**
 * Get the CSS class for message type styling
 */
function getStyleClass(message: TranscriptLine): string {
  // `away_summary` system rows reuse the summary card chrome (per CF-346
  // decision #9) — distinguishing role label lives in getClaudeRoleLabel.
  if (isSystemMessage(message) && message.subtype === 'away_summary') return 'summary';
  // Map hyphenated types to camelCase CSS class names
  switch (message.type) {
    case 'file-history-snapshot':
      return 'fileHistorySnapshot';
    case 'queue-operation':
      return 'queueOperation';
    default:
      return message.type;
  }
}

/**
 * Format timestamp for display
 */
function formatTimestamp(timestamp: string): string {
  const date = new Date(timestamp);
  return date.toLocaleTimeString('en-US', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: true,
  });
}

// CF-418: canonical `TokenUsage` (input/output/cacheWrite/cacheRead) now
// drives the cost badges and is shared with Codex. Wire-shape extras
// (speed, service_tier, server_tool_use) live on `message.message.usage`
// and are read directly when rendering Claude-specific badges/tooltips.

interface ClaudeServerToolUsage {
  web_search_requests?: number;
  web_fetch_requests?: number;
  code_execution_requests?: number;
}

// (count, singular, plural) tuples for each server-tool counter on the
// Claude wire payload. Order drives the rendered badge order.
const SERVER_TOOL_LABELS: ReadonlyArray<readonly [keyof ClaudeServerToolUsage, string, string]> = [
  ['web_search_requests', 'search', 'searches'],
  ['web_fetch_requests', 'fetch', 'fetches'],
  ['code_execution_requests', 'exec', 'execs'],
];

/** Server-tool badge labels read straight from the wire payload. */
function getServerToolBadges(stu: ClaudeServerToolUsage | undefined): string[] {
  if (!stu) return [];
  const badges: string[] = [];
  for (const [key, singular, plural] of SERVER_TOOL_LABELS) {
    const count = stu[key];
    if (count) {
      badges.push(`${count} ${count === 1 ? singular : plural}`);
    }
  }
  return badges;
}

/**
 * Get content blocks from a message
 */
function getContentBlocks(message: TranscriptLine): ContentBlock[] {
  if (isUserMessage(message)) {
    const content = message.message.content;
    if (typeof content === 'string') {
      const text = isCommandExpansionMessage(message)
        ? stripCommandExpansionTags(content)
        : content;
      return [{ type: 'text', text }];
    }
    return content;
  }
  if (isAssistantMessage(message)) {
    return message.message.content;
  }
  if (isSystemMessage(message)) {
    return message.content ? [{ type: 'text', text: message.content }] : [];
  }
  if (isSummaryMessage(message)) {
    return [{ type: 'text', text: message.summary }];
  }
  return [];
}

/**
 * Extract plain text for copying
 */
function extractTextContent(blocks: ContentBlock[]): string {
  return blocks
    .filter(isTextBlock)
    .map((block: TextBlock) => block.text)
    .join('\n');
}

/**
 * Get tool name for a tool result block
 */
function getToolNameForResult(block: ContentBlock, toolNameMap: Map<string, string>): string {
  if (isToolResultBlock(block)) {
    return toolNameMap.get(block.tool_use_id) || '';
  }
  if (isToolUseBlock(block)) {
    return block.name;
  }
  return '';
}

/**
 * Render file history snapshot content
 */
function FileSnapshotContent({ message }: { message: TranscriptLine }) {
  if (!isFileHistorySnapshot(message)) return null;

  const files = Object.keys(message.snapshot.trackedFileBackups);
  const fileCount = files.length;

  if (fileCount === 0) {
    return <div className={styles.snapshotEmpty}>No files tracked</div>;
  }

  return (
    <div className={styles.snapshotContent}>
      <div className={styles.snapshotSummary}>
        {fileCount} {fileCount === 1 ? 'file' : 'files'} tracked
      </div>
      <div className={styles.snapshotFiles}>
        {files.map((filePath) => {
          const backup = message.snapshot.trackedFileBackups[filePath];
          return (
            <div key={filePath} className={styles.snapshotFile}>
              <span className={styles.snapshotFilePath}>{filePath}</span>
              {backup && backup.version > 0 && (
                <span className={styles.snapshotFileVersion}>v{backup.version}</span>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

function ClaudeTimelineMessage({ message, toolNameMap, previousMessage, isSelected, isDeepLinkTarget, isCurrentSearchMatch, searchQuery, sessionId, onSkipToNext, onSkipToPrevious, roleLabel: roleLabelProp, isCostMode, messageCost, correctedTokenUsage, tils }: ClaudeTimelineMessageProps) {
  const { copy: copyText, copied: textCopied } = useCopyToClipboard();
  const { copy: copyLink, copied: linkCopied } = useCopyToClipboard();

  const styleClass = getStyleClass(message);
  const roleLabel = getClaudeRoleLabel(message);
  const isAwaySummary = isSystemMessage(message) && message.subtype === 'away_summary';
  // Attachment rows have no obvious text representation, so skip the
  // getContentBlocks work entirely — the dedicated AttachmentContent
  // component renders the body instead. Away-summary rows fall through to
  // the system branch of getContentBlocks so the Copy button can extract
  // the markdown content even though AwaySummary renders the visual body.
  const hasCustomBody = isAttachmentMessage(message);
  const contentBlocks = useMemo(() => (hasCustomBody ? [] : getContentBlocks(message)), [message, hasCustomBody]);

  // Get timestamp if available
  const timestamp = 'timestamp' in message && typeof message.timestamp === 'string' ? message.timestamp : undefined;

  // CF-418: canonical TokenUsage stamped by the Claude transcript service.
  // `correctedTokenUsage` (from the final line of a dedup'd group) overrides
  // when present, since intermediate streamed lines carry partial counts.
  // Fall back to live normalization for older cached entries that pre-date
  // the parse-time stamping.
  const wireUsage = isAssistantMessage(message) ? message.message.usage : undefined;
  const messageUsage = isAssistantMessage(message)
    ? message.tokenUsage ?? normalizeClaudeUsage(message.message.usage)
    : undefined;
  const displayUsage: TokenUsage | undefined = correctedTokenUsage ?? messageUsage;

  const tooltipText = isAssistantMessage(message) && displayUsage && messageCost != null
    ? buildCostTooltip(claudeAdapter, displayUsage, messageCost, message)
    : '';

  // CF-525: approximate per-message output speed. Duration is estimated as the
  // gap to the immediately preceding entry's timestamp; the shared helper owns
  // the omission rules (no predecessor, zero output, non-positive/garbled gap →
  // null → badge hidden). Marked "~" because it's an estimate, not measured.
  const prevTimestamp =
    previousMessage && 'timestamp' in previousMessage && typeof previousMessage.timestamp === 'string'
      ? previousMessage.timestamp
      : undefined;
  const messageSpeed =
    displayUsage && timestamp
      ? computeMessageTokenSpeed(displayUsage.output, prevTimestamp, timestamp)
      : null;

  // Get model for assistant messages
  const model = isAssistantMessage(message) ? message.message.model : undefined;

  // Get agent ID for sub-agent messages
  const agentId = isAssistantMessage(message) ? message.agentId : undefined;

  // Get skill name for command-expansion messages
  const skillName = isUserMessage(message) && isCommandExpansionMessage(message)
    ? getCommandExpansionSkillName(message)
    : null;

  // Check if this is from a different role than the previous message
  const previousRole = previousMessage ? getClaudeRoleLabel(previousMessage) : null;
  const isDifferentRole = previousRole !== roleLabel;

  // Get message UUID if available (user, assistant, system messages have it)
  const messageUuid = 'uuid' in message && typeof message.uuid === 'string' ? message.uuid : undefined;

  function handleCopyText() {
    copyText(extractTextContent(contentBlocks));
  }

  function handleCopyLink() {
    if (!messageUuid || !sessionId) return;
    copyLink(`${window.location.origin}/sessions/${sessionId}?tab=transcript&msg=${messageUuid}`);
  }

  const className = [
    styles.message,
    styles[styleClass],
    isDifferentRole && styles.newSpeaker,
    isSelected && styles.selected,
    isDeepLinkTarget && styles.deepLinkTarget,
    isCurrentSearchMatch && styles.searchMatch,
  ].filter(Boolean).join(' ');

  return (
    <div className={className}>
      <div className={styles.header}>
        <div className={styles.headerLeft}>
          <span className={styles.role}>{roleLabel}</span>
          {agentId && <span className={styles.agentBadge}>{agentId}</span>}
          {skillName && <span className={styles.skillBadge}>/{skillName}</span>}
          {tils && tils.length > 0 && <TILBadge tils={tils} />}
          {timestamp && <span className={styles.timestamp}>{formatTimestamp(timestamp)}</span>}
        </div>
        <div className={styles.headerRight}>
          {displayUsage && isCostMode && messageCost != null && (
            <>
              <span className={styles.costBadge} title={tooltipText}>
                {formatCost(messageCost)}
              </span>
              <span className={styles.tokenPill} title={tooltipText}>
                {formatTokenCount(displayUsage.input)} in · {formatTokenCount(displayUsage.output)} out
              </span>
              {(displayUsage.cacheWrite || displayUsage.cacheRead) ? (
                <span className={styles.cachePill} title={tooltipText}>
                  {displayUsage.cacheWrite ? `${formatTokenCount(displayUsage.cacheWrite)} write` : null}
                  {displayUsage.cacheWrite && displayUsage.cacheRead ? ' · ' : null}
                  {displayUsage.cacheRead ? `${formatTokenCount(displayUsage.cacheRead)} hit` : null}
                </span>
              ) : null}
              {messageSpeed != null && (
                <span
                  className={styles.tokenPill}
                  title="Estimated output speed — tokens/sec from time since the previous entry"
                >
                  ~{formatTokenSpeed(messageSpeed)}
                </span>
              )}
            </>
          )}
          {wireUsage?.speed === 'fast' && (
            <span className={styles.fastBadge} title="Fast mode (6x pricing)">
              <svg width="10" height="10" viewBox="0 0 16 16" fill="currentColor">
                <path d="M8.5 1L3 9h4.5L6.5 15 13 7H8.5L10 1H8.5z" />
              </svg>
              fast
            </span>
          )}
          {getServerToolBadges(wireUsage?.server_tool_use).map((badge) => (
            <span key={badge} className={styles.serverToolBadge}>{badge}</span>
          ))}
          {model && <span className={styles.model}>{extractModelVariant(model)}</span>}
          {onSkipToPrevious && (
            <button
              className={styles.skipBtn}
              onClick={onSkipToPrevious}
              title={`Previous ${roleLabelProp ?? roleLabel} message`}
              aria-label={`Previous ${roleLabelProp ?? roleLabel} message`}
            >
              <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                <polyline points="4 10 8 6 12 10" />
              </svg>
            </button>
          )}
          {onSkipToNext && (
            <button
              className={styles.skipBtn}
              onClick={onSkipToNext}
              title={`Next ${roleLabelProp ?? roleLabel} message`}
              aria-label={`Next ${roleLabelProp ?? roleLabel} message`}
            >
              <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                <polyline points="4 6 8 10 12 6" />
              </svg>
            </button>
          )}
          {!hasCustomBody && (
            <button
              className={`${styles.copyBtn} ${textCopied ? styles.copied : ''}`}
              onClick={handleCopyText}
              title="Copy message"
              aria-label="Copy message"
            >
              {textCopied ? (
                <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <polyline points="3.5 8.5 6.5 11.5 12.5 4.5" />
                </svg>
              ) : (
                <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                  <rect x="5.5" y="5.5" width="8" height="8" rx="1.5" />
                  <path d="M10.5 5.5V3.5a1.5 1.5 0 0 0-1.5-1.5H3.5A1.5 1.5 0 0 0 2 3.5V9a1.5 1.5 0 0 0 1.5 1.5h2" />
                </svg>
              )}
            </button>
          )}
          {messageUuid && sessionId && (
            <button
              className={`${styles.copyBtn} ${linkCopied ? styles.copied : ''}`}
              onClick={handleCopyLink}
              title="Copy link to message"
              aria-label="Copy link to message"
            >
              {linkCopied ? (
                <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <polyline points="3.5 8.5 6.5 11.5 12.5 4.5" />
                </svg>
              ) : (
                <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M6.5 9.5a3 3 0 0 0 4.24 0l2-2a3 3 0 0 0-4.24-4.24l-1 1" />
                  <path d="M9.5 6.5a3 3 0 0 0-4.24 0l-2 2a3 3 0 0 0 4.24 4.24l1-1" />
                </svg>
              )}
            </button>
          )}
        </div>
      </div>

      <div className={styles.content}>
        {message.type === 'file-history-snapshot' ? (
          <FileSnapshotContent message={message} />
        ) : isAttachmentMessage(message) ? (
          <AttachmentContent message={message} />
        ) : isAwaySummary ? (
          <AwaySummary message={message} />
        ) : (
          contentBlocks.map((block, i) => (
            <ContentBlockComponent
              key={i}
              block={block}
              toolName={getToolNameForResult(block, toolNameMap)}
              searchQuery={searchQuery}
              isCurrentSearchMatch={isCurrentSearchMatch}
            />
          ))
        )}
      </div>
    </div>
  );
}

/**
 * Extract short model variant from full model name
 */
function extractModelVariant(model: string): string {
  const variants = ['sonnet', 'opus', 'haiku'];
  for (const variant of variants) {
    if (model.toLowerCase().includes(variant)) {
      return variant;
    }
  }
  // Return last segment
  const parts = model.split('-');
  return parts[parts.length - 1] || model;
}

export default ClaudeTimelineMessage;
