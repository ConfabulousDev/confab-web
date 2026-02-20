// Shared TypeScript types for Confab frontend
// Types are derived from Zod schemas for runtime validation

// Re-export API types from schemas (these are validated at runtime)
export type {
  GitInfo,
  Session,
  SessionDetail,
  SessionShare,
} from '@/schemas/api';

// Re-export transcript types from schemas (these are validated at runtime)
export type {
  TranscriptLine,
  ContentBlock,
  TextBlock,
  UserMessage,
  AssistantMessage,
  SystemMessage,
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
  isToolResultMessage,
} from '@/schemas/transcript';
