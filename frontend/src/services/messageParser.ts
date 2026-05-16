// Message parsing service
// Extracts display data from transcript messages
import type { TranscriptLine, ContentBlock } from '@/types';
import {
  isUserMessage,
  isAssistantMessage,
  isSystemMessage,
  isSummaryMessage,
  isFileHistorySnapshot,
  isQueueOperationMessage,
  isPRLinkMessage,
  isToolResultMessage,
  isTextBlock,
  isThinkingBlock,
  isToolUseBlock,
  isToolResultBlock,
  hasThinking,
  usesTools,
} from '@/types';

interface ParsedMessageData {
  role: 'user' | 'assistant' | 'system' | 'unknown';
  timestamp?: string;
  content: ContentBlock[];
  messageModel?: string;
  isToolResult: boolean;
  hasThinkingContent: boolean;
  hasToolUse: boolean;
}

/**
 * Parse a transcript line into display-ready message data
 */
export function parseMessage(message: TranscriptLine): ParsedMessageData {
  let role: 'user' | 'assistant' | 'system' | 'unknown' = 'user';
  let timestamp: string | undefined;
  let content: ContentBlock[] = [];
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
    content = message.content ? [{ type: 'text', text: message.content }] : [];
  } else if (isSummaryMessage(message)) {
    role = 'system';
    content = [{ type: 'text', text: `📋 ${message.summary}` }];
  } else if (isFileHistorySnapshot(message)) {
    role = 'system';
    const backups = message.snapshot.trackedFileBackups;
    const fileCount = Object.keys(backups).length;
    const fileList = Object.entries(backups)
      .map(([path, backup]: [string, { version: number }]) => `  • ${path} (v${backup.version})`)
      .join('\n');
    const snapshotText = `📸 File Snapshot (${fileCount} ${fileCount === 1 ? 'file' : 'files'})\n${fileList}`;
    content = [{ type: 'text', text: snapshotText }];
  } else if (isQueueOperationMessage(message)) {
    role = 'system';
    timestamp = message.timestamp;
    const operationEmoji = message.operation === 'enqueue' ? '➕' : '➖';
    const operationText = message.operation === 'enqueue' ? 'Added to queue' : 'Removed from queue';
    content = [{ type: 'text', text: `${operationEmoji} ${operationText}` }];
  } else if (isPRLinkMessage(message)) {
    role = 'system';
    timestamp = message.timestamp;
    content = [{ type: 'text', text: `🔗 PR #${message.prNumber} — [${message.prRepository}](${message.prUrl})` }];
  } else {
    // Unknown message type — forward compatibility catch-all
    role = 'unknown';
    timestamp = 'timestamp' in message && typeof message.timestamp === 'string' ? message.timestamp : undefined;
    content = [{ type: 'text', text: `Unknown message type: ${message.type}` }];
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
}

/**
 * Plain-text search projection for one transcript message — bridges a
 * `TranscriptLine` to the generic `useTranscriptSearch` hook (which
 * lowercases when building its index, so callers don't have to).
 */
export function extractMessageText(message: TranscriptLine): string {
  return extractTextContent(parseMessage(message).content);
}

/**
 * Extract plain text content from a message for copying
 */
export function extractTextContent(content: ContentBlock[]): string {
  const parts: string[] = [];

  for (const block of content) {
    if (isTextBlock(block)) {
      parts.push(block.text);
    } else if (isThinkingBlock(block)) {
      parts.push(`[Thinking]\n${block.thinking}`);
    } else if (isToolUseBlock(block)) {
      parts.push(`[Tool: ${block.name}]\n${JSON.stringify(block.input, null, 2)}`);
    } else if (isToolResultBlock(block)) {
      const resultContent =
        typeof block.content === 'string' ? block.content : JSON.stringify(block.content, null, 2);
      parts.push(`[Tool Result]\n${resultContent}`);
    }
  }

  return parts.join('\n\n');
}

/**
 * Get role label for display
 */
export function getRoleLabel(role: string, isToolResult: boolean): string {
  if (role === 'user' && isToolResult) {
    return 'Tool Result';
  }
  return role.charAt(0).toUpperCase() + role.slice(1);
}
