<script lang="ts">
	import type { TranscriptLine, UserMessage, AssistantMessage, SystemMessage } from '$lib/types';
	import {
		isUserMessage,
		isAssistantMessage,
		isSystemMessage,
		isSummaryMessage,
		isToolResultMessage,
		hasThinking,
		usesTools
	} from '$lib/types/transcript';
	import ContentBlock from './ContentBlock.svelte';

	export let message: TranscriptLine;
	export let index: number;
	export let showThinking: boolean = true;
	export let expandAllTools: boolean = false;
	export let expandAllResults: boolean = true;

	// Determine message role for styling
	let role: 'user' | 'assistant' | 'system' = 'user';
	let timestamp: string | undefined;
	let content: any[] = [];
	let messageModel: string | undefined;
	let isToolResult = false;
	let hasThinkingContent = false;
	let hasToolUse = false;
	let copySuccess = false;

	// Build a map of tool_use_id -> tool name for linking results to their tools
	let toolNameMap = new Map<string, string>();

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
	}

	// Build tool name map from content blocks
	content.forEach((block: any) => {
		if (block.type === 'tool_use' && block.id && block.name) {
			toolNameMap.set(block.id, block.name);
		}
	});

	// Helper to get tool name for a tool_result block
	function getToolNameForResult(block: any): string {
		if (block.type === 'tool_result' && block.tool_use_id) {
			return toolNameMap.get(block.tool_use_id) || '';
		}
		return '';
	}

	// Format timestamp to be more readable
	function formatTimestamp(ts: string): string {
		const date = new Date(ts);
		return date.toLocaleTimeString('en-US', {
			hour: '2-digit',
			minute: '2-digit',
			second: '2-digit'
		});
	}

	// Get role icon
	function getRoleIcon(r: string): string {
		switch (r) {
			case 'user':
				return '👤';
			case 'assistant':
				return '🤖';
			case 'system':
				return 'ℹ️';
			default:
				return '•';
		}
	}

	// Get role label
	function getRoleLabel(r: string): string {
		if (r === 'user' && isToolResult) {
			return 'Tool Result';
		}
		return r.charAt(0).toUpperCase() + r.slice(1);
	}

	// Extract text content from message for copying
	function extractTextContent(): string {
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
					typeof block.content === 'string'
						? block.content
						: JSON.stringify(block.content, null, 2);
				parts.push(`[Tool Result]\n${resultContent}`);
			}
		}

		return parts.join('\n\n');
	}

	// Copy message content to clipboard
	async function copyMessage() {
		try {
			const text = extractTextContent();
			await navigator.clipboard.writeText(text);
			copySuccess = true;
			setTimeout(() => {
				copySuccess = false;
			}, 2000);
		} catch (err) {
			console.error('Failed to copy message:', err);
		}
	}
</script>

<div class="message message-{role}" class:is-tool-result={isToolResult}>
	<div class="message-sidebar">
		<div class="message-icon">{getRoleIcon(role)}</div>
	</div>

	<div class="message-body">
		<div class="message-header">
			<div class="message-meta">
				<span class="message-role">{getRoleLabel(role)}</span>
				{#if timestamp}
					<span class="message-timestamp">{formatTimestamp(timestamp)}</span>
				{/if}
				{#if messageModel}
					<span class="message-model">{messageModel.split('-').slice(-1)[0]}</span>
				{/if}
			</div>
			<div class="message-actions">
				<div class="message-badges">
					{#if hasThinkingContent}
						<span class="badge badge-thinking">💭 Thinking</span>
					{/if}
					{#if hasToolUse}
						<span class="badge badge-tools">🛠️ Tools</span>
					{/if}
				</div>
				<button class="copy-message-btn" on:click={copyMessage} title="Copy message">
					{#if copySuccess}
						✓
					{:else}
						📋
					{/if}
				</button>
			</div>
		</div>

		<div class="message-content">
			{#each content as block, i}
				<ContentBlock
					{block}
					index={i}
					toolName={getToolNameForResult(block)}
					{showThinking}
					{expandAllTools}
					{expandAllResults}
				/>
			{/each}
		</div>
	</div>
</div>

<style>
	.message {
		display: flex;
		gap: 1rem;
		padding: 1rem;
		border-radius: 8px;
		border: 1px solid #dee2e6;
		background: white;
		transition: box-shadow 0.2s ease, transform 0.2s ease;
		animation: fadeInUp 0.3s ease-out;
	}

	@keyframes fadeInUp {
		from {
			opacity: 0;
			transform: translateY(10px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	.message:hover {
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
		transform: translateY(-1px);
	}

	.message-user {
		border-left: 3px solid #007bff;
		background: #f8f9ff;
	}

	.message-assistant {
		border-left: 3px solid #6c757d;
		background: #f8f9fa;
	}

	.message-system {
		border-left: 3px solid #ffc107;
		background: #fffbf0;
	}

	.message.is-tool-result {
		border-left-color: #28a745;
		background: #f0fdf4;
	}

	.message-sidebar {
		flex-shrink: 0;
	}

	.message-icon {
		width: 40px;
		height: 40px;
		display: flex;
		align-items: center;
		justify-content: center;
		background: white;
		border-radius: 50%;
		font-size: 1.5rem;
		border: 2px solid #dee2e6;
	}

	.message-user .message-icon {
		border-color: #007bff;
	}

	.message-assistant .message-icon {
		border-color: #6c757d;
	}

	.message-system .message-icon {
		border-color: #ffc107;
	}

	.message-body {
		flex: 1;
		min-width: 0;
	}

	.message-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 0.75rem;
		padding-bottom: 0.5rem;
		border-bottom: 1px solid rgba(0, 0, 0, 0.05);
		flex-wrap: wrap;
		gap: 0.5rem;
	}

	.message-meta {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		flex-wrap: wrap;
	}

	.message-role {
		font-weight: 600;
		font-size: 0.95rem;
		color: #212529;
	}

	.message-timestamp {
		font-size: 0.8rem;
		color: #6c757d;
		font-family: 'Monaco', 'Menlo', monospace;
	}

	.message-model {
		font-size: 0.75rem;
		color: #6c757d;
		background: #e9ecef;
		padding: 0.2rem 0.5rem;
		border-radius: 3px;
		font-family: monospace;
	}

	.message-actions {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.message-badges {
		display: flex;
		gap: 0.5rem;
	}

	.badge {
		font-size: 0.75rem;
		padding: 0.2rem 0.5rem;
		border-radius: 4px;
		font-weight: 500;
		transition: transform 0.2s ease;
		animation: fadeIn 0.3s ease-out;
	}

	@keyframes fadeIn {
		from {
			opacity: 0;
			transform: scale(0.9);
		}
		to {
			opacity: 1;
			transform: scale(1);
		}
	}

	.badge:hover {
		transform: scale(1.05);
	}

	.copy-message-btn {
		background: rgba(255, 255, 255, 0.8);
		border: 1px solid #dee2e6;
		border-radius: 4px;
		padding: 0.25rem 0.5rem;
		font-size: 0.85rem;
		cursor: pointer;
		transition: all 0.2s;
		color: #495057;
		display: flex;
		align-items: center;
		justify-content: center;
		min-width: 28px;
	}

	.copy-message-btn:hover {
		background: white;
		border-color: #007bff;
		color: #007bff;
		transform: scale(1.05);
	}

	.copy-message-btn:active {
		transform: scale(0.95);
	}

	.badge-thinking {
		background: #e7f3ff;
		color: #0066cc;
	}

	.badge-tools {
		background: #fff3cd;
		color: #856404;
	}

	.message-content {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
	}

	/* Mobile responsive styles */
	@media (max-width: 768px) {
		.message {
			gap: 0.75rem;
			padding: 0.75rem;
		}

		.message-icon {
			width: 32px;
			height: 32px;
			font-size: 1.2rem;
		}

		.message-role {
			font-size: 0.9rem;
		}

		.message-timestamp {
			font-size: 0.75rem;
		}

		.message-model {
			font-size: 0.7rem;
		}

		.badge {
			font-size: 0.7rem;
			padding: 0.15rem 0.4rem;
		}

		.copy-message-btn {
			padding: 0.2rem 0.4rem;
			font-size: 0.75rem;
			min-width: 24px;
		}
	}

	@media (max-width: 480px) {
		.message {
			gap: 0.5rem;
			padding: 0.5rem;
		}

		.message-icon {
			width: 28px;
			height: 28px;
			font-size: 1rem;
		}

		.message-header {
			margin-bottom: 0.5rem;
			padding-bottom: 0.4rem;
		}

		.message-meta {
			gap: 0.5rem;
		}

		.copy-message-btn {
			padding: 0.15rem 0.3rem;
			font-size: 0.7rem;
			min-width: 22px;
		}
	}
</style>
