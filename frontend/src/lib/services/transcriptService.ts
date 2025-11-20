// Service for fetching and parsing Claude Code transcripts
import type { TranscriptLine, ParsedTranscript } from '$lib/types/transcript';

/**
 * In-memory cache for fetched transcripts
 * Key: `${runId}-${fileId}`
 */
const transcriptCache = new Map<string, TranscriptLine[]>();

/**
 * Fetch transcript content from backend API
 */
export async function fetchTranscriptContent(
	runId: number,
	fileId: number
): Promise<string> {
	const response = await fetch(`/api/v1/runs/${runId}/files/${fileId}/content`, {
		credentials: 'include'
	});

	if (!response.ok) {
		throw new Error(`Failed to fetch transcript: ${response.status} ${response.statusText}`);
	}

	return await response.text();
}

/**
 * Parse JSONL content into transcript messages
 * Each line is a separate JSON object
 */
export function parseJSONL(jsonl: string): TranscriptLine[] {
	const lines = jsonl.split('\n').filter((line) => line.trim());
	const messages: TranscriptLine[] = [];
	const errors: Array<{ line: number; error: string }> = [];

	lines.forEach((line, index) => {
		try {
			const parsed = JSON.parse(line);
			messages.push(parsed as TranscriptLine);
		} catch (e) {
			const errorMsg = e instanceof Error ? e.message : 'Unknown error';
			errors.push({ line: index + 1, error: errorMsg });
			console.error(`Failed to parse line ${index + 1}:`, errorMsg);
		}
	});

	// Log summary if there were errors
	if (errors.length > 0) {
		console.warn(
			`Parsed ${messages.length} messages with ${errors.length} errors:`,
			errors.slice(0, 5) // Show first 5 errors
		);
	}

	return messages;
}

/**
 * Fetch and parse a transcript file
 * Results are cached to avoid re-fetching
 */
export async function fetchTranscript(
	runId: number,
	fileId: number,
	options: { skipCache?: boolean } = {}
): Promise<TranscriptLine[]> {
	const cacheKey = `${runId}-${fileId}`;

	// Check cache first
	if (!options.skipCache && transcriptCache.has(cacheKey)) {
		return transcriptCache.get(cacheKey)!;
	}

	// Fetch and parse
	const content = await fetchTranscriptContent(runId, fileId);
	const messages = parseJSONL(content);

	// Cache the result
	transcriptCache.set(cacheKey, messages);

	return messages;
}

/**
 * Fetch and parse a complete transcript with metadata
 */
export async function fetchParsedTranscript(
	runId: number,
	fileId: number,
	sessionId: string
): Promise<ParsedTranscript> {
	const messages = await fetchTranscript(runId, fileId);

	// Extract metadata
	const timestamps = messages
		.filter((m) => 'timestamp' in m)
		.map((m) => (m as any).timestamp)
		.filter(Boolean);

	return {
		sessionId,
		messages,
		agents: [], // Will be populated by agent tree builder
		metadata: {
			version: messages.find((m) => 'version' in m)?.version || 'unknown',
			messageCount: messages.length,
			agentCount: 0, // Will be updated by agent tree builder
			firstTimestamp: timestamps[0],
			lastTimestamp: timestamps[timestamps.length - 1]
		}
	};
}

/**
 * Clear the transcript cache
 * Useful for forcing a refresh
 */
export function clearTranscriptCache(): void {
	transcriptCache.clear();
}

/**
 * Clear a specific transcript from cache
 */
export function clearTranscriptFromCache(runId: number, fileId: number): void {
	const cacheKey = `${runId}-${fileId}`;
	transcriptCache.delete(cacheKey);
}

/**
 * Get cache statistics
 */
export function getCacheStats(): { size: number; keys: string[] } {
	return {
		size: transcriptCache.size,
		keys: Array.from(transcriptCache.keys())
	};
}
