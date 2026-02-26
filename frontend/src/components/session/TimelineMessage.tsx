import { useMemo } from 'react';
import type { TranscriptLine, ContentBlock, TextBlock } from '@/types';
import { isTextBlock, isFileHistorySnapshot, isUserMessage } from '@/types';
import { isCommandExpansionMessage, getCommandExpansionSkillName, stripCommandExpansionTags } from '@/schemas/transcript';
import { useCopyToClipboard } from '@/hooks';
import ContentBlockComponent from '@/components/transcript/ContentBlock';
import styles from './TimelineMessage.module.css';

interface TimelineMessageProps {
  message: TranscriptLine;
  toolNameMap: Map<string, string>;
  previousMessage?: TranscriptLine;
  isSelected?: boolean;
  isDeepLinkTarget?: boolean;
  isSearchMatch?: boolean;
  sessionId?: string;
}

/**
 * Get role label for display
 */
function getRoleLabel(message: TranscriptLine): string {
  switch (message.type) {
    case 'user': {
      const content = message.message.content;
      if (Array.isArray(content)) {
        const hasToolResult = content.some((block) => block.type === 'tool_result');
        if (hasToolResult) return 'Tool';
      }
      return 'User';
    }
    case 'assistant':
      return 'Assistant';
    case 'system':
      return 'System';
    case 'summary':
      return 'Summary';
    case 'file-history-snapshot':
      return 'File Snapshot';
    case 'queue-operation':
      return 'Queue';
    default:
      return 'Unknown';
  }
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

interface TokenUsage {
  input_tokens: number;
  output_tokens: number;
  cache_creation_input_tokens?: number;
  cache_read_input_tokens?: number;
  service_tier?: string | null;
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

  return lines.join('\n');
}

/**
 * Get content blocks from a message
 */
function getContentBlocks(message: TranscriptLine): ContentBlock[] {
  switch (message.type) {
    case 'user': {
      const content = message.message.content;
      if (typeof content === 'string') {
        // Strip command-expansion XML tags for clean display
        const text = isUserMessage(message) && isCommandExpansionMessage(message)
          ? stripCommandExpansionTags(content)
          : content;
        return [{ type: 'text', text }];
      }
      return content;
    }
    case 'assistant':
      return message.message.content;
    case 'system':
      return message.content ? [{ type: 'text', text: message.content }] : [];
    case 'summary':
      return [{ type: 'text', text: message.summary }];
    default:
      return [];
  }
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
  if (block.type === 'tool_result') {
    return toolNameMap.get(block.tool_use_id) || '';
  }
  if (block.type === 'tool_use') {
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

function TimelineMessage({ message, toolNameMap, previousMessage, isSelected, isDeepLinkTarget, isSearchMatch, sessionId }: TimelineMessageProps) {
  const { copy: copyText, copied: textCopied } = useCopyToClipboard();
  const { copy: copyLink, copied: linkCopied } = useCopyToClipboard();

  const styleClass = getStyleClass(message.type);
  const roleLabel = getRoleLabel(message);
  const contentBlocks = useMemo(() => getContentBlocks(message), [message]);

  // Get timestamp if available
  const timestamp = 'timestamp' in message ? message.timestamp : undefined;

  // Get token usage for assistant messages
  const tokenUsage = message.type === 'assistant' ? message.message.usage : undefined;

  // Get model for assistant messages
  const model = message.type === 'assistant' ? message.message.model : undefined;

  // Get agent ID for sub-agent messages
  const agentId = message.type === 'assistant' ? message.agentId : undefined;

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
    isSearchMatch && styles.searchMatch,
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
          {model && <span className={styles.model}>{extractModelVariant(model)}</span>}
          <button
            className={styles.copyBtn}
            onClick={handleCopyText}
            title="Copy message"
            aria-label="Copy message"
          >
            {textCopied ? '✓' : '⎘'}
          </button>
          {messageUuid && sessionId && (
            <button
              className={styles.copyBtn}
              onClick={handleCopyLink}
              title="Copy link to message"
              aria-label="Copy link to message"
            >
              {linkCopied ? '✓' : '🔗'}
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
