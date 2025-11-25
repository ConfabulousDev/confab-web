import { useMemo } from 'react';
import type { TranscriptLine, ContentBlock } from '@/types';
import {
  parseMessage,
  buildToolNameMap,
  getToolNameForResult as getToolName,
  extractTextContent,
  formatTimestamp,
  getRoleLabel,
} from '@/services/messageParser';
import { useCopyToClipboard } from '@/hooks';
import ContentBlockComponent from './ContentBlock';
import styles from './Message.module.css';

interface MessageProps {
  message: TranscriptLine;
  index: number;
  previousMessage?: TranscriptLine;
}

function Message({ message, previousMessage }: MessageProps) {
  const { copy, copied } = useCopyToClipboard();

  // Parse message into structured data using service
  const messageData = useMemo(() => parseMessage(message), [message]);
  const prevMessageData = useMemo(() => previousMessage ? parseMessage(previousMessage) : null, [previousMessage]);

  // Check if this message is from a different speaker than the previous
  const isDifferentSpeaker = prevMessageData && prevMessageData.role !== messageData.role;

  // Build tool name map from content blocks
  const toolNameMap = useMemo(() => buildToolNameMap(messageData.content), [messageData.content]);

  // Copy message content to clipboard
  function copyMessage() {
    const text = extractTextContent(messageData.content);
    copy(text);
  }

  return (
    <div className={`${styles.message} ${styles[`message-${messageData.role}`]} ${messageData.isToolResult ? styles.isToolResult : ''} ${isDifferentSpeaker ? styles.differentSpeaker : ''}`}>
      <div className={styles.messageHeader}>
        <span className={styles.messageRole}>{getRoleLabel(messageData.role, messageData.isToolResult)}</span>
        <div className={styles.headerRight}>
          {messageData.messageModel && (
            <span className={styles.messageModel}>{messageData.messageModel.split('-').slice(-1)[0]}</span>
          )}
          {messageData.timestamp && <span className={styles.messageTimestamp}>{formatTimestamp(messageData.timestamp)}</span>}
          <button className={styles.copyBtn} onClick={copyMessage} title="Copy message">
            {copied ? '✓' : '⎘'}
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
          />
        ))}
      </div>
    </div>
  );
}

export default Message;
