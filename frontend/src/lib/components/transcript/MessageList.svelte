<script lang="ts">
	import type { TranscriptLine, AgentNode, RunDetail } from '$lib/types';
	import {
		isUserMessage,
		isAssistantMessage,
		isSystemMessage,
		isSummaryMessage
	} from '$lib/types/transcript';
	import Message from './Message.svelte';
	import AgentPanel from './AgentPanel.svelte';
	import { getAgentInsertionIndex } from '$lib/services/agentTreeBuilder';
	import { onMount, afterUpdate } from 'svelte';

	export let messages: TranscriptLine[];
	export let agents: AgentNode[];
	export let run: RunDetail;
	export let autoScroll: boolean = false;
	export let showThinking: boolean = true;
	export let expandAllAgents: boolean = true;
	export let expandAllTools: boolean = false;
	export let expandAllResults: boolean = true;

	let messageListElement: HTMLDivElement;
	let shouldScrollToBottom = false;

	// Build a map of where to insert agents
	const agentInsertionMap = new Map<number, AgentNode[]>();
	agents.forEach((agent) => {
		const insertIndex = getAgentInsertionIndex(messages, agent.parentMessageId);
		const existing = agentInsertionMap.get(insertIndex) || [];
		existing.push(agent);
		agentInsertionMap.set(insertIndex, existing);
	});

	// Filter out non-displayable messages
	const displayableMessages = messages.filter(
		(msg) =>
			isUserMessage(msg) || isAssistantMessage(msg) || isSystemMessage(msg) || isSummaryMessage(msg)
	);

	onMount(() => {
		if (autoScroll) {
			scrollToBottom();
		}
	});

	afterUpdate(() => {
		if (shouldScrollToBottom && autoScroll) {
			scrollToBottom();
			shouldScrollToBottom = false;
		}
	});

	function scrollToBottom() {
		if (messageListElement) {
			messageListElement.scrollTop = messageListElement.scrollHeight;
		}
	}

	// Manually trigger scroll to bottom
	export function scrollToEnd() {
		shouldScrollToBottom = true;
		scrollToBottom();
	}

	// Check if we should show a time separator
	function shouldShowTimeSeparator(current: TranscriptLine, previous: TranscriptLine | null): boolean {
		if (!previous) return false;

		const currentTime = 'timestamp' in current ? new Date(current.timestamp) : null;
		const previousTime = 'timestamp' in previous ? new Date(previous.timestamp) : null;

		if (!currentTime || !previousTime) return false;

		// Show separator if more than 5 minutes between messages
		const diff = currentTime.getTime() - previousTime.getTime();
		return diff > 5 * 60 * 1000; // 5 minutes in milliseconds
	}

	function formatTimeSeparator(timestamp: string): string {
		const date = new Date(timestamp);
		const now = new Date();
		const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
		const messageDate = new Date(date.getFullYear(), date.getMonth(), date.getDate());

		if (messageDate.getTime() === today.getTime()) {
			return date.toLocaleTimeString('en-US', {
				hour: '2-digit',
				minute: '2-digit'
			});
		}

		return date.toLocaleString('en-US', {
			month: 'short',
			day: 'numeric',
			hour: '2-digit',
			minute: '2-digit'
		});
	}
</script>

<div class="message-list" bind:this={messageListElement}>
	{#each displayableMessages as message, index}
		<!-- Time separator -->
		{#if shouldShowTimeSeparator(message, index > 0 ? displayableMessages[index - 1] : null)}
			{#if 'timestamp' in message}
				<div class="time-separator">
					<span class="time-separator-line"></span>
					<span class="time-separator-text">{formatTimeSeparator(message.timestamp)}</span>
					<span class="time-separator-line"></span>
				</div>
			{/if}
		{/if}

		<Message {message} {index} {showThinking} {expandAllTools} {expandAllResults} />

		<!-- Insert agents after their parent message -->
		{#if agentInsertionMap.has(index + 1)}
			{#each agentInsertionMap.get(index + 1) as agent}
				<AgentPanel {agent} {run} depth={0} {showThinking} {expandAllAgents} {expandAllTools} {expandAllResults} />
			{/each}
		{/if}
	{/each}

	{#if displayableMessages.length === 0}
		<div class="empty-state">
			<div class="empty-icon">📋</div>
			<p>No conversation messages</p>
			{#if messages.length > 0}
				<p class="empty-detail">This session contains {messages.length} metadata {messages.length === 1 ? 'entry' : 'entries'} (file snapshots, etc.) but no user/assistant messages.</p>
			{/if}
		</div>
	{/if}
</div>

<style>
	.message-list {
		display: flex;
		flex-direction: column;
		gap: 1rem;
		max-height: 80vh;
		overflow-y: auto;
		padding: 0.5rem;
		scroll-behavior: smooth;
	}

	/* Scrollbar styling */
	.message-list::-webkit-scrollbar {
		width: 8px;
	}

	.message-list::-webkit-scrollbar-track {
		background: #f1f1f1;
		border-radius: 4px;
	}

	.message-list::-webkit-scrollbar-thumb {
		background: #888;
		border-radius: 4px;
		transition: background 0.2s ease;
	}

	.message-list::-webkit-scrollbar-thumb:hover {
		background: #555;
	}

	.message-list::-webkit-scrollbar-thumb:active {
		background: #333;
	}

	.time-separator {
		display: flex;
		align-items: center;
		gap: 1rem;
		margin: 1rem 0;
	}

	.time-separator-line {
		flex: 1;
		height: 1px;
		background: #dee2e6;
	}

	.time-separator-text {
		font-size: 0.8rem;
		color: #6c757d;
		font-weight: 500;
		white-space: nowrap;
		padding: 0.25rem 0.75rem;
		background: #f8f9fa;
		border-radius: 12px;
		border: 1px solid #dee2e6;
	}

	.empty-state {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: 4rem 2rem;
		text-align: center;
		color: #6c757d;
	}

	.empty-icon {
		font-size: 3rem;
		margin-bottom: 1rem;
		opacity: 0.5;
	}

	.empty-state p {
		margin: 0;
		font-size: 1.1rem;
	}

	.empty-detail {
		margin-top: 0.5rem !important;
		font-size: 0.9rem !important;
		color: #868e96;
		max-width: 400px;
	}

	/* Mobile responsive styles */
	@media (max-width: 768px) {
		.message-list {
			gap: 0.75rem;
			padding: 0.4rem;
			max-height: 85vh;
		}

		.time-separator {
			margin: 0.75rem 0;
			gap: 0.75rem;
		}

		.time-separator-text {
			font-size: 0.75rem;
			padding: 0.2rem 0.6rem;
		}

		.empty-icon {
			font-size: 2.5rem;
		}

		.empty-state p {
			font-size: 1rem;
		}
	}

	@media (max-width: 480px) {
		.message-list {
			gap: 0.5rem;
			padding: 0.25rem;
			max-height: 90vh;
		}

		.time-separator {
			margin: 0.5rem 0;
			gap: 0.5rem;
		}

		.time-separator-text {
			font-size: 0.7rem;
			padding: 0.15rem 0.5rem;
		}

		.empty-state {
			padding: 3rem 1rem;
		}

		.empty-icon {
			font-size: 2rem;
		}

		.empty-state p {
			font-size: 0.95rem;
		}
	}
</style>
