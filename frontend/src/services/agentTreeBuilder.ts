// Service for building agent tree structure from transcripts
import { z } from 'zod';
import type {
	TranscriptLine,
	AgentNode,
	AssistantMessage,
	FileDetail
} from '@/types';
import {
	isAssistantMessage,
	isUserMessage,
	isToolUseBlock,
	getToolResults
} from '@/types';
import { fetchTranscript } from './transcriptService';

/**
 * Extract agent ID from filename
 * Format: agent-{8-char-hex}.jsonl -> "0da5686d"
 */
function extractAgentIdFromPath(filePath: string): string | null {
	const match = filePath.match(/agent-([a-f0-9]{8})\.jsonl$/);
	return match?.[1] ?? null;
}

interface AgentMetadata {
	status?: string; // 'completed' | 'interrupted' | 'error' - use string for forward compat
	totalDurationMs?: number;
	totalTokens?: number;
	totalToolUseCount?: number;
}

interface AgentReference {
	agentId: string;
	toolUseId: string;
	parentMessageId: string;
	prompt: string;
	metadata: AgentMetadata;
}

// Schema for agent tool use result embedded in content blocks
// Use passthrough() to preserve unknown fields for forward compatibility
const AgentToolUseResultSchema = z.object({
	agentId: z.string().optional(),
	prompt: z.string().optional(),
	status: z.string().optional(), // 'completed' | 'interrupted' | 'error' - use string for forward compat
	totalDurationMs: z.number().optional(),
	totalTokens: z.number().optional(),
	totalToolUseCount: z.number().optional(),
}).passthrough();

const BlockWithToolUseResultSchema = z.object({
	toolUseResult: AgentToolUseResultSchema,
});

type AgentToolUseResult = z.infer<typeof AgentToolUseResultSchema>;

/**
 * Extract agent data from a block that might have toolUseResult
 */
function extractAgentData(block: unknown): AgentToolUseResult | undefined {
	const result = BlockWithToolUseResultSchema.safeParse(block);
	if (!result.success) return undefined;
	return result.data.toolUseResult;
}

/**
 * Find agent references in a transcript
 * Returns map of agentId -> { toolUseId, messageId, prompt }
 */
function findAgentReferences(
	messages: TranscriptLine[]
): Map<string, AgentReference> {
	const references = new Map<string, AgentReference>();

	// Walk through messages looking for tool results with toolUseResult.agentId
	for (const message of messages) {
		if (!isUserMessage(message)) continue;

		const toolResults = getToolResults(message);
		for (const result of toolResults) {
			// Check if this is a Task tool result with agent metadata
			if (typeof result.content !== 'string' && Array.isArray(result.content)) {
				// Content might be an array with toolUseResult
				for (const block of result.content) {
					const agentData = extractAgentData(block);
					if (agentData?.agentId) {
						references.set(agentData.agentId, {
							agentId: agentData.agentId,
							toolUseId: result.tool_use_id,
							parentMessageId: message.uuid,
							prompt: agentData.prompt || '',
							metadata: {
								status: agentData.status,
								totalDurationMs: agentData.totalDurationMs,
								totalTokens: agentData.totalTokens,
								totalToolUseCount: agentData.totalToolUseCount
							}
						});
					}
				}
			}

			// Also check in the message's toolUseResult field (direct property)
			if (message.toolUseResult?.agentId) {
				const agentData = message.toolUseResult;
				references.set(agentData.agentId, {
					agentId: agentData.agentId,
					toolUseId: result.tool_use_id,
					parentMessageId: message.uuid,
					prompt: agentData.prompt || '',
					metadata: {
						status: agentData.status,
						totalDurationMs: agentData.totalDurationMs,
						totalTokens: agentData.totalTokens,
						totalToolUseCount: agentData.totalToolUseCount
					}
				});
			}
		}
	}

	return references;
}

/**
 * Build agent tree recursively
 */
async function buildAgentNodeRecursive(
	runId: number,
	agentId: string,
	agentFile: FileDetail,
	parentToolUseId: string,
	parentMessageId: string,
	metadata: AgentMetadata,
	agentFileMap: Map<string, FileDetail>,
	depth: number = 0,
	maxDepth: number = 10,
	shareOptions?: { sessionId: string; shareToken: string }
): Promise<AgentNode> {
	// Prevent infinite recursion
	if (depth >= maxDepth) {
		console.warn(`Max nesting depth ${maxDepth} reached for agent ${agentId}`);
		return {
			agentId,
			transcript: [],
			parentToolUseId,
			parentMessageId,
			children: [],
			metadata
		};
	}

	// Load agent transcript
	const transcript = await fetchTranscript(runId, agentFile.id, shareOptions);

	// Find sub-agents spawned by this agent
	const subAgentRefs = findAgentReferences(transcript);

	// Recursively load sub-agents in parallel for better performance
	const childPromises: Promise<AgentNode | null>[] = [];

	for (const [subAgentId, refData] of subAgentRefs) {
		const subAgentFile = agentFileMap.get(subAgentId);
		if (!subAgentFile) {
			console.warn(`Sub-agent file not found for agent ${subAgentId}`);
			childPromises.push(Promise.resolve(null));
			continue;
		}

		const promise = buildAgentNodeRecursive(
			runId,
			subAgentId,
			subAgentFile,
			refData.toolUseId,
			refData.parentMessageId,
			refData.metadata,
			agentFileMap,
			depth + 1,
			maxDepth,
			shareOptions
		).catch((e) => {
			console.error(`Failed to load sub-agent ${subAgentId}:`, e);
			return null;
		});

		childPromises.push(promise);
	}

	// Wait for all children to load in parallel
	const childResults = await Promise.all(childPromises);
	const children = childResults.filter((node): node is AgentNode => node !== null);

	return {
		agentId,
		transcript,
		parentToolUseId,
		parentMessageId,
		children,
		metadata: {
			totalDurationMs: metadata.totalDurationMs,
			totalTokens: metadata.totalTokens,
			totalToolUseCount: metadata.totalToolUseCount,
			status: metadata.status
		}
	};
}

/**
 * Build complete agent tree from run files
 */
export async function buildAgentTree(
	runId: number,
	mainTranscript: TranscriptLine[],
	allFiles: FileDetail[],
	shareOptions?: { sessionId: string; shareToken: string }
): Promise<AgentNode[]> {
	// Find all agent files
	const agentFiles = allFiles.filter((f) => f.file_type === 'agent');

	// Create map of agentId -> FileDetail
	const agentFileMap = new Map<string, FileDetail>();
	for (const file of agentFiles) {
		const agentId = extractAgentIdFromPath(file.file_path);
		if (agentId) {
			agentFileMap.set(agentId, file);
		}
	}

	// Find agent references in main transcript
	const agentRefs = findAgentReferences(mainTranscript);

	// Build top-level agents in parallel (those spawned by main session)
	const topLevelPromises: Promise<AgentNode | null>[] = [];

	for (const [agentId, refData] of agentRefs) {
		const agentFile = agentFileMap.get(agentId);
		if (!agentFile) {
			console.warn(`Agent file not found for agent ${agentId}`);
			topLevelPromises.push(Promise.resolve(null));
			continue;
		}

		const promise = buildAgentNodeRecursive(
			runId,
			agentId,
			agentFile,
			refData.toolUseId,
			refData.parentMessageId,
			refData.metadata,
			agentFileMap, // Pass the map for recursive lookups
			0, // Start at depth 0
			10, // Max depth
			shareOptions
		).catch((e) => {
			console.error(`Failed to load agent ${agentId}:`, e);
			return null;
		});

		topLevelPromises.push(promise);
	}

	// Wait for all top-level agents to load in parallel
	const topLevelResults = await Promise.all(topLevelPromises);
	const topLevelAgents = topLevelResults.filter((node): node is AgentNode => node !== null);

	console.log('Built agent tree:', {
		topLevelCount: topLevelAgents.length,
		totalCount: getAgentCount(topLevelAgents),
		maxDepth: getMaxDepth(topLevelAgents)
	});

	return topLevelAgents;
}

/**
 * Find the parent message for an agent
 * Returns the assistant message that spawned this agent
 */
export function findAgentParentMessage(
	messages: TranscriptLine[],
	toolUseId: string
): AssistantMessage | null {
	// Walk backwards to find the assistant message with this tool_use_id
	for (let i = messages.length - 1; i >= 0; i--) {
		const msg = messages[i];
		if (!msg) continue;
		if (isAssistantMessage(msg)) {
			const hasToolUse = msg.message.content.some(
				(block) => isToolUseBlock(block) && block.id === toolUseId
			);
			if (hasToolUse) {
				return msg;
			}
		}
	}
	return null;
}

/**
 * Get insertion point for agent in message list
 * Agent should be displayed after the tool result that spawned it
 */
export function getAgentInsertionIndex(
	messages: TranscriptLine[],
	parentMessageId: string
): number {
	const index = messages.findIndex((m) => 'uuid' in m && m.uuid === parentMessageId);
	return index >= 0 ? index + 1 : messages.length;
}

/**
 * Flatten agent tree into a list for debugging
 */
export function flattenAgentTree(agents: AgentNode[]): AgentNode[] {
	const flat: AgentNode[] = [];

	function walk(node: AgentNode) {
		flat.push(node);
		node.children.forEach(walk);
	}

	agents.forEach(walk);
	return flat;
}

/**
 * Get total agent count (including nested)
 */
export function getAgentCount(agents: AgentNode[]): number {
	return flattenAgentTree(agents).length;
}

/**
 * Get max nesting depth
 */
export function getMaxDepth(agents: AgentNode[]): number {
	function depth(node: AgentNode): number {
		if (node.children.length === 0) return 1;
		return 1 + Math.max(...node.children.map(depth));
	}

	if (agents.length === 0) return 0;
	return Math.max(...agents.map(depth));
}
