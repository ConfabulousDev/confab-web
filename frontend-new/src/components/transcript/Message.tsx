import { useState, useMemo } from 'react';
import type { TranscriptLine, ContentBlock } from '@/types';
import {
  parseMessage,
  buildToolNameMap,
  getToolNameForResult as getToolName,
  extractTextContent,
  formatTimestamp,
  getRoleIcon,
  getRoleLabel,
} from '@/services/messageParser';
import ContentBlockComponent from './ContentBlock';
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
  showThinking = true,
  expandAllTools = false,
  expandAllResults = true,
}: MessageProps) {
  const [copySuccess, setCopySuccess] = useState(false);

  // Parse message into structured data using service
  const messageData = useMemo(() => parseMessage(message), [message]);

  // Build tool name map from content blocks
  const toolNameMap = useMemo(() => buildToolNameMap(messageData.content), [messageData.content]);

  // Copy message content to clipboard
  async function copyMessage() {
    try {
      const text = extractTextContent(messageData.content);
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
            <span className={styles.messageRole}>{getRoleLabel(messageData.role, messageData.isToolResult)}</span>
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
          {messageData.content.map((block: ContentBlock, i: number) => (
            <ContentBlockComponent
              key={i}
              block={block}
              index={i}
              toolName={getToolName(block, toolNameMap)}
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
