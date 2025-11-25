// Message parsing service
// Extracts display data from transcript messages
import type { TranscriptLine, ContentBlock } from '@/types/transcript';
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

export interface ParsedMessageData {
  role: 'user' | 'assistant' | 'system';
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
  let role: 'user' | 'assistant' | 'system' = 'user';
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
    timestamp = 'timestamp' in message ? (message as { timestamp?: string }).timestamp : undefined;
    content = [
      {
        type: 'text',
        text: `⚠️ Unknown message type\n\`\`\`json\n${JSON.stringify(message, null, 2)}\n\`\`\``,
      },
    ];
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
 * Build a map of tool use IDs to tool names from content blocks
 */
export function buildToolNameMap(content: ContentBlock[]): Map<string, string> {
  const map = new Map<string, string>();
  content.forEach((block) => {
    if (block.type === 'tool_use' && block.id && block.name) {
      map.set(block.id, block.name);
    }
  });
  return map;
}

/**
 * Get the tool name for a tool_result block
 */
export function getToolNameForResult(block: ContentBlock, toolNameMap: Map<string, string>): string {
  if (block.type === 'tool_result' && block.tool_use_id) {
    return toolNameMap.get(block.tool_use_id) || '';
  }
  return '';
}

/**
 * Extract plain text content from a message for copying
 */
export function extractTextContent(content: ContentBlock[]): string {
  const parts: string[] = [];

  for (const block of content) {
    if (block.type === 'text' && block.text) {
      parts.push(block.text);
    } else if (block.type === 'thinking' && block.thinking) {
      parts.push(`[Thinking]\n${block.thinking}`);
    } else if (block.type === 'tool_use') {
      parts.push(`[Tool: ${block.name}]\n${JSON.stringify(block.input, null, 2)}`);
    } else if (block.type === 'tool_result') {
      const resultContent =
        typeof block.content === 'string' ? block.content : JSON.stringify(block.content, null, 2);
      parts.push(`[Tool Result]\n${resultContent}`);
    }
  }

  return parts.join('\n\n');
}

/**
 * Format timestamp for display
 */
export function formatTimestamp(timestamp: string): string {
  const date = new Date(timestamp);
  return date.toLocaleTimeString('en-US', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
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
