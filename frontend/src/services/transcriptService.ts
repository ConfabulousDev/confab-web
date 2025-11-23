// Service for fetching and parsing Claude Code transcripts
import type { TranscriptLine, ParsedTranscript } from '@/types/transcript';

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
	options: { skipCache?: boolean; sessionId?: string; shareToken?: string } = {}
): Promise<TranscriptLine[]> {
	const cacheKey = `${runId}-${fileId}`;

	// Check cache first
	if (!options.skipCache && transcriptCache.has(cacheKey)) {
		console.log(`    ⏱️ Using cached transcript`);
		return transcriptCache.get(cacheKey)!;
	}

	// Fetch and parse
	const t0 = performance.now();
	const content = await fetchTranscriptContent(runId, fileId, {
		sessionId: options.sessionId,
		shareToken: options.shareToken
	});
	const t1 = performance.now();
	console.log(`    ⏱️ Network fetch took ${Math.round(t1 - t0)}ms (${Math.round(content.length / 1024 / 1024 * 10) / 10}MB)`);

	const t2 = performance.now();
	const messages = parseJSONL(content);
	const t3 = performance.now();
	console.log(`    ⏱️ Parsing JSONL took ${Math.round(t3 - t2)}ms (${messages.length} messages)`);

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
	sessionId: string,
	shareToken?: string
): Promise<ParsedTranscript> {
	const t0 = performance.now();
	const messages = await fetchTranscript(runId, fileId, { sessionId, shareToken });
	const t1 = performance.now();
	console.log(`  ⏱️ fetchTranscript (network + parse) took ${Math.round(t1 - t0)}ms for ${messages.length} messages`);

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
