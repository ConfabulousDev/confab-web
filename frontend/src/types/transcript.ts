// TypeScript types for Claude Code transcript format
// Based on Claude Code v2.0.42 transcript schema
// See: docs/claude-code-transcript-format.md

// ============================================================================
// Base Message Structure
// ============================================================================

export type TranscriptLine =
	| UserMessage
	| AssistantMessage
	| FileHistorySnapshot
	| SystemMessage
	| SummaryMessage
	| QueueOperationMessage;

// ============================================================================
// Common Fields
// ============================================================================

interface BaseMessage {
	uuid: string;
	timestamp: string; // ISO 8601
	parentUuid: string | null;
	isSidechain: boolean;
	userType: string; // Usually "external"
	cwd: string;
	sessionId: string;
	version: string; // e.g., "2.0.42"
	gitBranch?: string;
}

// ============================================================================
// User Message
// ============================================================================

export interface UserMessage extends BaseMessage {
	type: 'user';
	thinkingMetadata?: ThinkingMetadata;
	message: {
		role: 'user';
		content: string | ContentBlock[];
	};
	// Present in tool results from agent tasks
	toolUseResult?: ToolUseResult;
}

export interface ThinkingMetadata {
	level: 'high' | 'medium' | 'low' | 'off';
	disabled: boolean;
	triggers: string[];
}

export interface ToolUseResult {
	status: 'completed' | 'interrupted' | 'error';
	prompt: string;
	agentId: string; // 8-character hex ID
	content: ContentBlock[];
	totalDurationMs: number;
	totalTokens: number;
	totalToolUseCount: number;
	usage: TokenUsage;
}

// ============================================================================
// Assistant Message
// ============================================================================

export interface AssistantMessage extends BaseMessage {
	type: 'assistant';
	requestId: string;
	agentId?: string; // Present in agent transcripts
	message: {
		model: string; // e.g., "claude-sonnet-4-5-20250929"
		id: string; // API message ID
		type: 'message';
		role: 'assistant';
		content: ContentBlock[];
		stop_reason: StopReason;
		stop_sequence: string | null;
		usage: TokenUsage;
	};
}

export type StopReason = 'end_turn' | 'tool_use' | 'max_tokens' | 'stop_sequence';

export interface TokenUsage {
	input_tokens: number;
	cache_creation_input_tokens?: number;
	cache_read_input_tokens?: number;
	cache_creation?: {
		ephemeral_5m_input_tokens: number;
		ephemeral_1h_input_tokens: number;
	};
	output_tokens: number;
	service_tier?: string; // e.g., "standard"
}

// ============================================================================
// Content Blocks
// ============================================================================

export type ContentBlock = TextBlock | ThinkingBlock | ToolUseBlock | ToolResultBlock;

export interface TextBlock {
	type: 'text';
	text: string;
}

export interface ThinkingBlock {
	type: 'thinking';
	thinking: string;
	signature: string; // Cryptographic signature
}

export interface ToolUseBlock {
	type: 'tool_use';
	id: string; // e.g., "toolu_01NHDUYBGs52pSNJxuamKqsY"
	name: ToolName;
	input: Record<string, any>;
}

export type ToolName =
	| 'Bash'
	| 'Read'
	| 'Write'
	| 'Edit'
	| 'Grep'
	| 'Glob'
	| 'Task'
	| 'WebFetch'
	| 'WebSearch'
	| 'NotebookEdit'
	| string; // Allow unknown tools

export interface ToolResultBlock {
	type: 'tool_result';
	tool_use_id: string;
	content: string | ContentBlock[];
	is_error?: boolean;
}

// ============================================================================
// File History Snapshot
// ============================================================================

export interface FileHistorySnapshot {
	type: 'file-history-snapshot';
	messageId: string;
	isSnapshotUpdate: boolean;
	snapshot: {
		messageId: string;
		timestamp: string;
		trackedFileBackups: Record<string, FileBackup>;
	};
}

export interface FileBackup {
	backupFileName: string | null;
	version: number;
	backupTime: string;
}

// ============================================================================
// System Message
// ============================================================================

export interface SystemMessage extends BaseMessage {
	type: 'system';
	logicalParentUuid?: string;
	subtype: SystemSubtype;
	content: string;
	isMeta: boolean;
	level: 'info' | 'warning' | 'error';
	compactMetadata?: CompactMetadata;
}

export type SystemSubtype = 'compact_boundary' | string; // Allow unknown subtypes

export interface CompactMetadata {
	trigger: 'auto' | 'manual';
	preTokens: number;
}

// ============================================================================
// Summary Message
// ============================================================================

export interface SummaryMessage {
	type: 'summary';
	summary: string;
	leafUuid: string;
}

// ============================================================================
// Queue Operation Message
// ============================================================================

export interface QueueOperationMessage {
	type: 'queue-operation';
	operation: 'enqueue' | 'dequeue' | string;
	timestamp: string;
	content: string;
	sessionId: string;
}

// ============================================================================
// Agent Tree Structure
// ============================================================================

export interface AgentNode {
	agentId: string;
	transcript: TranscriptLine[];
	parentToolUseId: string; // Links to parent message's tool_use block
	parentMessageId: string; // UUID of parent message
	children: AgentNode[];
	metadata: {
		totalDurationMs?: number;
		totalTokens?: number;
		totalToolUseCount?: number;
		status?: 'completed' | 'interrupted' | 'error';
	};
}

// ============================================================================
// Parsed Transcript Structure
// ============================================================================

export interface ParsedTranscript {
	sessionId: string;
	messages: TranscriptLine[];
	agents: AgentNode[];
	metadata: {
		version: string;
		messageCount: number;
		agentCount: number;
		firstTimestamp?: string;
		lastTimestamp?: string;
	};
}

// ============================================================================
// Helper Types for UI
// ============================================================================

// Message types for rendering decisions
export type MessageRole = 'user' | 'assistant' | 'system';

// Simplified message for display
export interface DisplayMessage {
	id: string; // uuid
	role: MessageRole;
	timestamp: string;
	content: ContentBlock[];
	isToolResult: boolean;
	isFromAgent: boolean;
	agentId?: string;
	parentId?: string;
}

// Tool use display
export interface DisplayToolUse {
	id: string;
	name: ToolName;
	input: Record<string, any>;
	result?: DisplayToolResult;
	status: 'pending' | 'success' | 'error';
}

export interface DisplayToolResult {
	content: string | ContentBlock[];
	isError: boolean;
	agentId?: string; // If this was a Task tool
}

// Thinking display
export interface DisplayThinking {
	content: string;
	signature: string;
	expanded: boolean;
}

// ============================================================================
// Type Guards
// ============================================================================

export function isUserMessage(line: TranscriptLine): line is UserMessage {
	return line.type === 'user';
}

export function isAssistantMessage(line: TranscriptLine): line is AssistantMessage {
	return line.type === 'assistant';
}

export function isSystemMessage(line: TranscriptLine): line is SystemMessage {
	return line.type === 'system';
}

export function isFileHistorySnapshot(line: TranscriptLine): line is FileHistorySnapshot {
	return line.type === 'file-history-snapshot';
}

export function isSummaryMessage(line: TranscriptLine): line is SummaryMessage {
	return line.type === 'summary';
}

export function isQueueOperationMessage(line: TranscriptLine): line is QueueOperationMessage {
	return line.type === 'queue-operation';
}

export function isTextBlock(block: ContentBlock): block is TextBlock {
	return block.type === 'text';
}

export function isThinkingBlock(block: ContentBlock): block is ThinkingBlock {
	return block.type === 'thinking';
}

export function isToolUseBlock(block: ContentBlock): block is ToolUseBlock {
	return block.type === 'tool_use';
}

export function isToolResultBlock(block: ContentBlock): block is ToolResultBlock {
	return block.type === 'tool_result';
}

// ============================================================================
// Utility Functions
// ============================================================================

/**
 * Extract agent ID from tool result if it was a Task tool
 */
export function extractAgentId(toolResult: ToolResultBlock): string | null {
	if (typeof toolResult.content === 'object' && 'toolUseResult' in toolResult.content) {
		return (toolResult.content as any).toolUseResult?.agentId || null;
	}
	return null;
}

/**
 * Get message content as plain text (strips formatting)
 */
export function getPlainTextContent(content: ContentBlock[]): string {
	return content
		.filter(isTextBlock)
		.map((block) => block.text)
		.join('\n');
}

/**
 * Check if assistant message contains thinking
 */
export function hasThinking(message: AssistantMessage): boolean {
	return message.message.content.some(isThinkingBlock);
}

/**
 * Check if assistant message uses tools
 */
export function usesTools(message: AssistantMessage): boolean {
	return message.message.content.some(isToolUseBlock);
}

/**
 * Get all tool uses from an assistant message
 */
export function getToolUses(message: AssistantMessage): ToolUseBlock[] {
	return message.message.content.filter(isToolUseBlock);
}

/**
 * Check if user message is a tool result
 */
export function isToolResultMessage(message: UserMessage): boolean {
	const content = message.message.content;
	if (typeof content === 'string') return false;
	return content.some(isToolResultBlock);
}

/**
 * Get tool results from user message
 */
export function getToolResults(message: UserMessage): ToolResultBlock[] {
	const content = message.message.content;
	if (typeof content === 'string') return [];
	return content.filter(isToolResultBlock);
}
