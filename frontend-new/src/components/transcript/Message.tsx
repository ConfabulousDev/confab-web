import { useState, useMemo } from 'react';
import type { TranscriptLine } from '@/types';
import {
  isUserMessage,
  isAssistantMessage,
  isSystemMessage,
  isSummaryMessage,
  isFileHistorySnapshot,
  isQueueOperationMessage,
  isToolResultMessage,
  hasThinking,
  usesTools,
} from '@/types/transcript';
import ContentBlock from './ContentBlock';
import styles from './Message.module.css';

interface MessageProps {
  message: TranscriptLine;
  index: number;
  showThinking?: boolean;
  expandAllTools?: boolean;
  expandAllResults?: boolean;
}

function Message({
  message,
  // index, // Reserved for future use
  showThinking = true,
  expandAllTools = false,
  expandAllResults = true,
}: MessageProps) {
  const [copySuccess, setCopySuccess] = useState(false);

  // Parse message into structured data
  const messageData = useMemo(() => {
    let role: 'user' | 'assistant' | 'system' = 'user';
    let timestamp: string | undefined;
    let content: any[] = [];
    let messageModel: string | undefined;
    let isToolResult = false;
    let hasThinkingContent = false;
    let hasToolUse = false;

    if (isUserMessage(message)) {
      role = 'user';
      timestamp = message.timestamp;
      const msgContent = message.message.content;
      content = typeof msgContent === 'string' ? [{ type: 'text', text: msgContent }] : msgContent;
      isToolResult = isToolResultMessage(message);
    } else if (isAssistantMessage(message)) {
      role = 'assistant';
      timestamp = message.timestamp;
      content = message.message.content;
      messageModel = message.message.model;
      hasThinkingContent = hasThinking(message);
      hasToolUse = usesTools(message);
    } else if (isSystemMessage(message)) {
      role = 'system';
      timestamp = message.timestamp;
      content = [{ type: 'text', text: message.content }];
    } else if (isSummaryMessage(message)) {
      role = 'system';
      content = [{ type: 'text', text: `📋 ${message.summary}` }];
    } else if (isFileHistorySnapshot(message)) {
      role = 'system';
      const fileCount = Object.keys(message.snapshot.trackedFileBackups).length;
      const fileList = Object.entries(message.snapshot.trackedFileBackups)
        .map(([path, backup]) => `  • ${path} (v${backup.version})`)
        .join('\n');
      const snapshotText = `📸 File Snapshot (${fileCount} ${fileCount === 1 ? 'file' : 'files'})\n${fileList}`;
      content = [{ type: 'text', text: snapshotText }];
    } else if (isQueueOperationMessage(message)) {
      role = 'system';
      timestamp = message.timestamp;
      const operationEmoji = message.operation === 'enqueue' ? '➕' : '➖';
      const operationText = message.operation === 'enqueue' ? 'Added to queue' : 'Removed from queue';
      content = [{ type: 'text', text: `${operationEmoji} ${operationText}` }];
    } else {
      // Fallback for unknown message types
      console.warn('Unknown message type encountered:', message);
      role = 'system';
      timestamp = 'timestamp' in message ? (message as any).timestamp : undefined;
      content = [{ type: 'text', text: `⚠️ Unknown message type\n\`\`\`json\n${JSON.stringify(message, null, 2)}\n\`\`\`` }];
    }

    return {
      role,
      timestamp,
      content,
      messageModel,
      isToolResult,
      hasThinkingContent,
      hasToolUse,
    };
  }, [message]);

  // Build tool name map from content blocks
  const toolNameMap = useMemo(() => {
    const map = new Map<string, string>();
    messageData.content.forEach((block: any) => {
      if (block.type === 'tool_use' && block.id && block.name) {
        map.set(block.id, block.name);
      }
    });
    return map;
  }, [messageData.content]);

  // Helper to get tool name for a tool_result block
  function getToolNameForResult(block: any): string {
    if (block.type === 'tool_result' && block.tool_use_id) {
      return toolNameMap.get(block.tool_use_id) || '';
    }
    return '';
  }

  // Format timestamp
  function formatTimestamp(ts: string): string {
    const date = new Date(ts);
    return date.toLocaleTimeString('en-US', {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
  }

  // Get role icon
  function getRoleIcon(r: string): string {
    switch (r) {
      case 'user':
        return '👤';
      case 'assistant':
        return '🤖';
      case 'system':
        return 'ℹ️';
      default:
        return '•';
    }
  }

  // Get role label
  function getRoleLabel(r: string): string {
    if (r === 'user' && messageData.isToolResult) {
      return 'Tool Result';
    }
    return r.charAt(0).toUpperCase() + r.slice(1);
  }

  // Extract text content from message for copying
  function extractTextContent(): string {
    const parts: string[] = [];

    for (const block of messageData.content) {
      if (block.type === 'text' && block.text) {
        parts.push(block.text);
      } else if (block.type === 'thinking' && block.thinking) {
        parts.push(`[Thinking]\n${block.thinking}`);
      } else if (block.type === 'tool_use') {
        parts.push(`[Tool: ${block.name}]\n${JSON.stringify(block.input, null, 2)}`);
      } else if (block.type === 'tool_result') {
        const resultContent = typeof block.content === 'string' ? block.content : JSON.stringify(block.content, null, 2);
        parts.push(`[Tool Result]\n${resultContent}`);
      }
    }

    return parts.join('\n\n');
  }

  // Copy message content to clipboard
  async function copyMessage() {
    try {
      const text = extractTextContent();
      await navigator.clipboard.writeText(text);
      setCopySuccess(true);
      setTimeout(() => {
        setCopySuccess(false);
      }, 2000);
    } catch (err) {
      console.error('Failed to copy message:', err);
    }
  }

  return (
    <div className={`${styles.message} ${styles[`message-${messageData.role}`]} ${messageData.isToolResult ? styles.isToolResult : ''}`}>
      <div className={styles.messageSidebar}>
        <div className={styles.messageIcon}>{getRoleIcon(messageData.role)}</div>
      </div>

      <div className={styles.messageBody}>
        <div className={styles.messageHeader}>
          <div className={styles.messageMeta}>
            <span className={styles.messageRole}>{getRoleLabel(messageData.role)}</span>
            {messageData.timestamp && <span className={styles.messageTimestamp}>{formatTimestamp(messageData.timestamp)}</span>}
            {messageData.messageModel && (
              <span className={styles.messageModel}>{messageData.messageModel.split('-').slice(-1)[0]}</span>
            )}
          </div>
          <div className={styles.messageActions}>
            <div className={styles.messageBadges}>
              {messageData.hasThinkingContent && <span className={`${styles.badge} ${styles.badgeThinking}`}>💭 Thinking</span>}
              {messageData.hasToolUse && <span className={`${styles.badge} ${styles.badgeTools}`}>🛠️ Tools</span>}
            </div>
            <button className={styles.copyMessageBtn} onClick={copyMessage} title="Copy message">
              {copySuccess ? '✓' : '📋'}
            </button>
          </div>
        </div>

        <div className={styles.messageContent}>
          {messageData.content.map((block: any, i: number) => (
            <ContentBlock
              key={i}
              block={block}
              index={i}
              toolName={getToolNameForResult(block)}
              showThinking={showThinking}
              expandAllTools={expandAllTools}
              expandAllResults={expandAllResults}
            />
          ))}
        </div>
      </div>
    </div>
  );
}

export default Message;
