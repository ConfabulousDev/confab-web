// Service for fetching and parsing Claude Code transcripts
// All transcript data is validated with Zod schemas at parse time
import {
  type TranscriptLine,
  type TranscriptValidationError,
  type TranscriptParseResult,
  parseTranscriptLineWithError,
  formatValidationErrorsForLog,
} from '@/schemas/transcript';

// Re-export types for consumers
export type { TranscriptLine, TranscriptValidationError, TranscriptParseResult } from '@/schemas/transcript';

/**
 * Parsed transcript with metadata
 */
export interface ParsedTranscript {
  sessionId: string;
  messages: TranscriptLine[];
  agents: AgentNode[];
  /** Validation errors encountered while parsing (empty if all lines valid) */
  validationErrors: TranscriptValidationError[];
  metadata: {
    version: string;
    messageCount: number;
    agentCount: number;
    firstTimestamp?: string;
    lastTimestamp?: string;
    /** Number of lines that failed validation */
    parseErrorCount: number;
  };
}

/**
 * Agent node for hierarchical transcript display
 */
export interface AgentNode {
  agentId: string;
  transcript: TranscriptLine[];
  parentToolUseId: string;
  parentMessageId: string;
  children: AgentNode[];
  metadata: {
    totalDurationMs?: number;
    totalTokens?: number;
    totalToolUseCount?: number;
    status?: string; // 'completed' | 'interrupted' | 'error' - use string for forward compat
  };
}

/**
 * In-memory cache for fetched transcripts
 * Key: `${runId}-${fileId}`
 */
const transcriptCache = new Map<string, TranscriptLine[]>();

/**
 * Options for fetching transcript content
 */
export interface FetchOptions {
  sessionId?: string;
  shareToken?: string;
}

/**
 * Fetch transcript content from backend API
 * Supports both authenticated and shared (public) access
 */
export async function fetchTranscriptContent(
  runId: number,
  fileId: number,
  options?: FetchOptions
): Promise<string> {
  let url: string;

  // Use shared endpoint if share token is provided
  if (options?.shareToken && options?.sessionId) {
    url = `/api/v1/sessions/${options.sessionId}/shared/${options.shareToken}/files/${fileId}/content`;
  } else {
    url = `/api/v1/runs/${runId}/files/${fileId}/content`;
  }

  const response = await fetch(url, {
    credentials: 'include',
  });

  if (!response.ok) {
    throw new Error(`Failed to fetch transcript: ${response.status} ${response.statusText}`);
  }

  return await response.text();
}

/**
 * Parse and validate JSONL content into transcript messages.
 * Each line is validated against the TranscriptLine schema.
 * Returns structured parse result with detailed errors for UI display.
 */
export function parseJSONL(jsonl: string): TranscriptParseResult {
  const lines = jsonl.split('\n').filter((line) => line.trim());
  const messages: TranscriptLine[] = [];
  const errors: TranscriptValidationError[] = [];

  lines.forEach((line, index) => {
    const result = parseTranscriptLineWithError(line, index);

    if (result.success) {
      messages.push(result.data);
    } else {
      errors.push(result.error);
    }
  });

  // Log summary if there were errors
  if (errors.length > 0) {
    console.warn(formatValidationErrorsForLog(errors));
  }

  return {
    messages,
    errors,
    totalLines: lines.length,
    successCount: messages.length,
    errorCount: errors.length,
  };
}

/**
 * Legacy parse function that returns only messages (for backward compatibility)
 * @deprecated Use parseJSONL which returns TranscriptParseResult with errors
 */
export function parseJSONLMessages(jsonl: string): TranscriptLine[] {
  return parseJSONL(jsonl).messages;
}

/** Cache entry includes both messages and errors */
interface CacheEntry {
  messages: TranscriptLine[];
  errors: TranscriptValidationError[];
}

/** In-memory cache for parsed transcripts */
const transcriptCacheV2 = new Map<string, CacheEntry>();

/**
 * Fetch and parse a transcript file
 * Results are cached to avoid re-fetching
 * Returns only messages for backward compatibility - use fetchTranscriptWithErrors for full result
 */
export async function fetchTranscript(
  runId: number,
  fileId: number,
  options: { skipCache?: boolean; sessionId?: string; shareToken?: string } = {}
): Promise<TranscriptLine[]> {
  const result = await fetchTranscriptWithErrors(runId, fileId, options);
  return result.messages;
}

/**
 * Fetch and parse a transcript file with validation errors
 * Returns both successfully parsed messages and structured validation errors
 */
export async function fetchTranscriptWithErrors(
  runId: number,
  fileId: number,
  options: { skipCache?: boolean; sessionId?: string; shareToken?: string } = {}
): Promise<CacheEntry> {
  const cacheKey = `${runId}-${fileId}`;

  // Check cache first
  if (!options.skipCache && transcriptCacheV2.has(cacheKey)) {
    console.log(`    ⏱️ Using cached transcript`);
    const cached = transcriptCacheV2.get(cacheKey);
    if (cached) return cached;
  }

  // Fetch and parse
  const t0 = performance.now();
  const content = await fetchTranscriptContent(runId, fileId, {
    sessionId: options.sessionId,
    shareToken: options.shareToken,
  });
  const t1 = performance.now();
  console.log(
    `    ⏱️ Network fetch took ${Math.round(t1 - t0)}ms (${Math.round((content.length / 1024 / 1024) * 10) / 10}MB)`
  );

  const t2 = performance.now();
  const parseResult = parseJSONL(content);
  const t3 = performance.now();
  console.log(`    ⏱️ Parsing JSONL took ${Math.round(t3 - t2)}ms (${parseResult.successCount} messages, ${parseResult.errorCount} errors)`);

  const entry: CacheEntry = {
    messages: parseResult.messages,
    errors: parseResult.errors,
  };

  // Cache the result
  transcriptCacheV2.set(cacheKey, entry);

  // Also update legacy cache for backward compat
  transcriptCache.set(cacheKey, parseResult.messages);

  return entry;
}

/**
 * Fetch and parse a complete transcript with metadata
 */
export async function fetchParsedTranscript(
  runId: number,
  fileId: number,
  sessionId: string,
  shareToken?: string
): Promise<ParsedTranscript> {
  const t0 = performance.now();
  const { messages, errors } = await fetchTranscriptWithErrors(runId, fileId, { sessionId, shareToken });
  const t1 = performance.now();
  console.log(`  ⏱️ fetchTranscript (network + parse) took ${Math.round(t1 - t0)}ms for ${messages.length} messages`);

  // Extract metadata - filter to messages with timestamp property
  const timestamps = messages
    .filter((m): m is typeof m & { timestamp: string } => 'timestamp' in m && typeof m.timestamp === 'string')
    .map((m) => m.timestamp);

  return {
    sessionId,
    messages,
    agents: [], // Will be populated by agent tree builder
    validationErrors: errors,
    metadata: {
      version: messages.find((m) => 'version' in m)?.version || 'unknown',
      messageCount: messages.length,
      agentCount: 0, // Will be updated by agent tree builder
      firstTimestamp: timestamps[0],
      lastTimestamp: timestamps[timestamps.length - 1],
      parseErrorCount: errors.length,
    },
  };
}

/**
 * Clear the transcript cache
 * Useful for forcing a refresh
 */
export function clearTranscriptCache(): void {
  transcriptCache.clear();
  transcriptCacheV2.clear();
}

/**
 * Clear a specific transcript from cache
 */
export function clearTranscriptFromCache(runId: number, fileId: number): void {
  const cacheKey = `${runId}-${fileId}`;
  transcriptCache.delete(cacheKey);
  transcriptCacheV2.delete(cacheKey);
}

/**
 * Get cache statistics
 */
export function getCacheStats(): { size: number; keys: string[] } {
  return {
    size: transcriptCacheV2.size,
    keys: Array.from(transcriptCacheV2.keys()),
  };
}
