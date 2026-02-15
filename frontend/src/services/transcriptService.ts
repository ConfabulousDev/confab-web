// Service for fetching and parsing Claude Code transcripts
// All transcript data is validated with Zod schemas at parse time
import {
  type TranscriptLine,
  type TranscriptValidationError,
  type TranscriptParseResult,
  validateParsedTranscriptLine,
  formatValidationErrorsForLog,
} from '@/schemas/transcript';
import { syncFilesAPI } from './api';


/**
 * Parsed transcript with metadata
 */
interface ParsedTranscript {
  sessionId: string;
  messages: TranscriptLine[];
  agents: AgentNode[];
  /** Validation errors encountered while parsing (empty if all lines valid) */
  validationErrors: TranscriptValidationError[];
  /** Total number of non-empty lines in the file (for line_offset tracking) */
  totalLines: number;
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
interface AgentNode {
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
 * Fetch transcript content from backend API via sync file endpoint.
 * Uses canonical session endpoint which supports all access types
 * (owner, recipient share, system share, public share).
 * @param sessionId - The session UUID
 * @param fileName - Name of the transcript file
 * @param lineOffset - Optional: Return only lines after this line number
 */
async function fetchTranscriptContent(
  sessionId: string,
  fileName: string,
  lineOffset?: number
): Promise<string> {
  return syncFilesAPI.getContent(sessionId, fileName, lineOffset);
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
    // Parse JSON once and reuse the parsed object for validation
    let parsed: unknown;
    try {
      parsed = JSON.parse(line);
    } catch (e) {
      errors.push({
        line: index + 1,
        rawJson: line.length > 200 ? line.slice(0, 200) + '...' : line,
        errors: [{
          path: '(root)',
          message: `Invalid JSON: ${e instanceof Error ? e.message : 'parse error'}`,
        }],
      });
      return;
    }

    // Skip progress messages - they're streaming updates not needed in transcript view
    if (parsed !== null && typeof parsed === 'object' && 'type' in parsed && parsed.type === 'progress') {
      return;
    }

    const result = validateParsedTranscriptLine(parsed, line, index);

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

/** Cache entry includes messages, errors, and total line count */
interface CacheEntry {
  messages: TranscriptLine[];
  errors: TranscriptValidationError[];
  /** Total non-empty lines in file (for accurate line_offset tracking) */
  totalLines: number;
}

/** In-memory cache for parsed transcripts - keyed by sessionId-fileName */
const transcriptCacheV2 = new Map<string, CacheEntry>();

/**
 * Fetch and parse a transcript file with validation errors
 * Returns both successfully parsed messages and structured validation errors
 */
async function fetchTranscriptWithErrors(
  sessionId: string,
  fileName: string,
  options: { skipCache?: boolean } = {}
): Promise<CacheEntry> {
  const cacheKey = `${sessionId}-${fileName}`;

  // Check cache first
  if (!options.skipCache && transcriptCacheV2.has(cacheKey)) {
    const cached = transcriptCacheV2.get(cacheKey);
    if (cached) return cached;
  }

  // Fetch and parse
  const content = await fetchTranscriptContent(sessionId, fileName);
  const parseResult = parseJSONL(content);

  const entry: CacheEntry = {
    messages: parseResult.messages,
    errors: parseResult.errors,
    totalLines: parseResult.totalLines,
  };

  // Cache the result
  transcriptCacheV2.set(cacheKey, entry);

  return entry;
}

/**
 * Fetch and parse a complete transcript with metadata
 */
export async function fetchParsedTranscript(
  sessionId: string,
  fileName: string,
  skipCache?: boolean
): Promise<ParsedTranscript> {
  const { messages, errors, totalLines } = await fetchTranscriptWithErrors(sessionId, fileName, { skipCache });

  // Extract metadata - filter to messages with timestamp property
  const timestamps = messages
    .filter((m): m is typeof m & { timestamp: string } => 'timestamp' in m && typeof m.timestamp === 'string')
    .map((m) => m.timestamp);

  return {
    sessionId,
    messages,
    agents: [], // Will be populated by agent tree builder
    validationErrors: errors,
    totalLines,
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
 * Fetch new transcript messages incrementally.
 * Returns only messages that are new since the given line count.
 *
 * @param sessionId - The session UUID
 * @param fileName - Name of the transcript file
 * @param currentLineCount - Number of lines already loaded (fetch lines after this)
 * @returns Object with newMessages array and the new total line count
 */
export async function fetchNewTranscriptMessages(
  sessionId: string,
  fileName: string,
  currentLineCount: number
): Promise<{ newMessages: TranscriptLine[]; newTotalLineCount: number }> {
  // Fetch only lines after currentLineCount
  const content = await fetchTranscriptContent(sessionId, fileName, currentLineCount);

  // If content is empty, no new messages
  if (!content.trim()) {
    return { newMessages: [], newTotalLineCount: currentLineCount };
  }

  // Parse the new content
  const parseResult = parseJSONL(content);

  // New total is previous count plus total lines fetched (not just successful parses)
  // This ensures line_offset stays in sync with actual file line numbers
  const newTotalLineCount = currentLineCount + parseResult.totalLines;

  return {
    newMessages: parseResult.messages,
    newTotalLineCount,
  };
}

