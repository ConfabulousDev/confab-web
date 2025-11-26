// Shared TypeScript types for Confab frontend
// Types are derived from Zod schemas for runtime validation

// Re-export API types from schemas (these are validated at runtime)
export type {
  GitInfo,
  FileDetail,
  RunDetail,
  Session,
  SessionDetail,
  SessionShare,
  User,
  APIKey,
  CreateAPIKeyResponse,
  CreateShareResponse,
} from '@/schemas/api';

// Re-export transcript types from schemas (these are validated at runtime)
export type {
  TranscriptLine,
  ContentBlock,
  TextBlock,
  ThinkingBlock,
  ToolUseBlock,
  ToolResultBlock,
  ImageBlock,
  UserMessage,
  AssistantMessage,
  SystemMessage,
  FileHistorySnapshot,
  SummaryMessage,
  QueueOperationMessage,
} from '@/schemas/transcript';

// Re-export type guards
export {
  isTextBlock,
  isThinkingBlock,
  isToolUseBlock,
  isToolResultBlock,
  isImageBlock,
  isUserMessage,
  isAssistantMessage,
  isSystemMessage,
  isFileHistorySnapshot,
  isSummaryMessage,
  isQueueOperationMessage,
} from '@/schemas/transcript';

// Re-export utility functions
export {
  hasThinking,
  usesTools,
  getToolUses,
  isToolResultMessage,
  getToolResults,
  getPlainTextContent,
} from '@/schemas/transcript';

// Todo item from Claude Code todo list (local type, not from API)
export type TodoItem = {
  content: string;
  status: 'pending' | 'in_progress' | 'completed';
  activeForm: string;
};

// Agent tree node structure (used locally, not from API)
export type { AgentNode, ParsedTranscript } from '@/services/transcriptService';
