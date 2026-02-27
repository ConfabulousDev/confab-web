import { useMemo } from 'react';
import type { TranscriptLine, ContentBlock, TextBlock } from '@/types';
import { isTextBlock, isToolUseBlock, isToolResultBlock, isFileHistorySnapshot, isUserMessage, isAssistantMessage, isSystemMessage, isSummaryMessage, isCommandExpansionMessage, getCommandExpansionSkillName, stripCommandExpansionTags } from '@/types';
import { useCopyToClipboard } from '@/hooks';
import ContentBlockComponent from '@/components/transcript/ContentBlock';
import { getRoleLabel } from './messageCategories';
import styles from './TimelineMessage.module.css';

interface TimelineMessageProps {
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
}

/**
 * Get the CSS class for message type styling
 */
function getStyleClass(type: TranscriptLine['type']): string {
  // Map hyphenated types to camelCase CSS class names
  switch (type) {
    case 'file-history-snapshot':
      return 'fileHistorySnapshot';
    case 'queue-operation':
      return 'queueOperation';
    default:
      return type;
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

interface ServerToolUsage {
  web_search_requests?: number;
  web_fetch_requests?: number;
  code_execution_requests?: number;
}

interface TokenUsage {
  input_tokens: number;
  output_tokens: number;
  cache_creation_input_tokens?: number;
  cache_read_input_tokens?: number;
  service_tier?: string | null;
  server_tool_use?: ServerToolUsage;
  speed?: string;
}

/**
 * Format token count for display (total of input + output)
 */
function formatTokens(usage: TokenUsage): string {
  const total = usage.input_tokens + usage.output_tokens;
  if (total >= 1000) {
    return `${(total / 1000).toFixed(1)}k tokens`;
  }
  return `${total} ${total === 1 ? 'token' : 'tokens'}`;
}

/**
 * Build detailed tooltip for token usage
 */
function buildTokenTooltip(usage: TokenUsage): string {
  const lines: string[] = [];

  lines.push(`Input: ${usage.input_tokens.toLocaleString()}`);
  lines.push(`Output: ${usage.output_tokens.toLocaleString()}`);

  if (usage.cache_creation_input_tokens) {
    lines.push(`Cache created: ${usage.cache_creation_input_tokens.toLocaleString()}`);
  }
  if (usage.cache_read_input_tokens) {
    lines.push(`Cache read: ${usage.cache_read_input_tokens.toLocaleString()}`);
  }
  if (usage.service_tier) {
    lines.push(`Tier: ${usage.service_tier}`);
  }
  if (usage.speed) {
    lines.push(`Speed: ${usage.speed}`);
  }

  const stu = usage.server_tool_use;
  if (stu) {
    if (stu.web_search_requests) {
      lines.push(`Web searches: ${stu.web_search_requests}`);
    }
    if (stu.web_fetch_requests) {
      lines.push(`Web fetches: ${stu.web_fetch_requests}`);
    }
    if (stu.code_execution_requests) {
      lines.push(`Code executions: ${stu.code_execution_requests}`);
    }
  }

  return lines.join('\n');
}

/**
 * Build server tool badge labels from usage
 */
function getServerToolBadges(usage: TokenUsage): string[] {
  const badges: string[] = [];
  const stu = usage.server_tool_use;
  if (!stu) return badges;

  if (stu.web_search_requests) {
    badges.push(`${stu.web_search_requests} ${stu.web_search_requests === 1 ? 'search' : 'searches'}`);
  }
  if (stu.web_fetch_requests) {
    badges.push(`${stu.web_fetch_requests} ${stu.web_fetch_requests === 1 ? 'fetch' : 'fetches'}`);
  }
  if (stu.code_execution_requests) {
    badges.push(`${stu.code_execution_requests} ${stu.code_execution_requests === 1 ? 'exec' : 'execs'}`);
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

function TimelineMessage({ message, toolNameMap, previousMessage, isSelected, isDeepLinkTarget, isCurrentSearchMatch, searchQuery, sessionId, onSkipToNext, onSkipToPrevious, roleLabel: roleLabelProp }: TimelineMessageProps) {
  const { copy: copyText, copied: textCopied } = useCopyToClipboard();
  const { copy: copyLink, copied: linkCopied } = useCopyToClipboard();

  const styleClass = getStyleClass(message.type);
  const roleLabel = getRoleLabel(message);
  const contentBlocks = useMemo(() => getContentBlocks(message), [message]);

  // Get timestamp if available
  const timestamp = 'timestamp' in message && typeof message.timestamp === 'string' ? message.timestamp : undefined;

  // Get token usage for assistant messages
  const tokenUsage = isAssistantMessage(message) ? message.message.usage : undefined;

  // Get model for assistant messages
  const model = isAssistantMessage(message) ? message.message.model : undefined;

  // Get agent ID for sub-agent messages
  const agentId = isAssistantMessage(message) ? message.agentId : undefined;

  // Get skill name for command-expansion messages
  const skillName = isUserMessage(message) && isCommandExpansionMessage(message)
    ? getCommandExpansionSkillName(message)
    : null;

  // Check if this is from a different role than the previous message
  const previousRole = previousMessage ? getRoleLabel(previousMessage) : null;
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
          {timestamp && <span className={styles.timestamp}>{formatTimestamp(timestamp)}</span>}
        </div>
        <div className={styles.headerRight}>
          {tokenUsage && (
            <span className={styles.tokens} title={buildTokenTooltip(tokenUsage)}>
              {formatTokens(tokenUsage)}
            </span>
          )}
          {tokenUsage?.speed === 'fast' && (
            <span className={styles.fastBadge} title="Fast mode (6x pricing)">
              <svg width="10" height="10" viewBox="0 0 16 16" fill="currentColor">
                <path d="M8.5 1L3 9h4.5L6.5 15 13 7H8.5L10 1H8.5z" />
              </svg>
              fast
            </span>
          )}
          {tokenUsage && getServerToolBadges(tokenUsage).map((badge) => (
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

export default TimelineMessage;
