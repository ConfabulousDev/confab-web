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
  UnknownBlock,
  UnknownMessage,
  UserMessage,
  AssistantMessage,
  SystemMessage,
  PRLinkMessage,
} from '@/schemas/transcript';

// Re-export type guards
export {
  isTextBlock,
  isThinkingBlock,
  isToolUseBlock,
  isToolResultBlock,
  isImageBlock,
  isUnknownBlock,
  isUserMessage,
  isAssistantMessage,
  isSystemMessage,
  isFileHistorySnapshot,
  isSummaryMessage,
  isQueueOperationMessage,
  isPRLinkMessage,
  isUnknownMessage,
} from '@/schemas/transcript';

// Re-export utility functions
export {
  hasThinking,
  usesTools,
  isToolResultMessage,
  isSkillExpansionMessage,
  isCommandExpansionMessage,
  getCommandExpansionSkillName,
  stripCommandExpansionTags,
  warnIfKnownTypeCaughtByCatchall,
} from '@/schemas/transcript';
