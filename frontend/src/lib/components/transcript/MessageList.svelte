<script lang="ts">
	import type { TranscriptLine, AgentNode, RunDetail } from '$lib/types';
	import Message from './Message.svelte';
	import AgentPanel from './AgentPanel.svelte';
	import { getAgentInsertionIndex } from '$lib/services/agentTreeBuilder';
	import { onMount, afterUpdate } from 'svelte';
	import SvelteVirtualList from '@humanspeak/svelte-virtual-list';

	export let messages: TranscriptLine[];
	export let agents: AgentNode[];
	export let run: RunDetail;
	export let autoScroll: boolean = false;
	export let showThinking: boolean = true;
	export let expandAllAgents: boolean = true;
	export let expandAllTools: boolean = false;
	export let expandAllResults: boolean = true;

	let messageListElement: HTMLDivElement;
	let virtualListRef: any;
	let shouldScrollToBottom = false;

	// Item types for virtual list
	type VirtualItem =
		| { type: 'message'; message: TranscriptLine; index: number }
		| { type: 'separator'; timestamp: string }
		| { type: 'agent'; agent: AgentNode };

	// Build a map of where to insert agents (reactive)
	$: agentInsertionMap = (() => {
		const map = new Map<number, AgentNode[]>();
		agents.forEach((agent) => {
			const insertIndex = getAgentInsertionIndex(messages, agent.parentMessageId);
			const existing = map.get(insertIndex) || [];
			existing.push(agent);
			map.set(insertIndex, existing);
		});
		return map;
	})();

	// Flatten messages, separators, and agents into a single virtual list
	$: virtualItems = (() => {
		const items: VirtualItem[] = [];

		messages.forEach((message, index) => {
			// Add time separator if needed
			if (shouldShowTimeSeparator(message, index > 0 ? messages[index - 1] : null)) {
				if ('timestamp' in message) {
					items.push({ type: 'separator', timestamp: message.timestamp });
				}
			}

			// Add message
			items.push({ type: 'message', message, index });

			// Add agents after this message
			const agentsAtIndex = agentInsertionMap.get(index + 1);
			if (agentsAtIndex) {
				agentsAtIndex.forEach(agent => {
					items.push({ type: 'agent', agent });
				});
			}
		});

		return items;
	})();

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
		if (virtualListRef && virtualListRef.scroll) {
			virtualListRef.scroll({
				index: virtualItems.length - 1,
				align: 'end'
			});
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

{#if virtualItems.length === 0}
	<div class="empty-state">
		<div class="empty-icon">📋</div>
		<p>No messages in this session</p>
	</div>
{:else}
	<div class="message-list-wrapper">
		<SvelteVirtualList
			items={virtualItems}
			bind:this={virtualListRef}
			defaultEstimatedItemHeight={150}
			bufferSize={5}
		>
			{#snippet renderItem(item)}
				{#if item.type === 'separator'}
					<div class="time-separator">
						<span class="time-separator-line"></span>
						<span class="time-separator-text">{formatTimeSeparator(item.timestamp)}</span>
						<span class="time-separator-line"></span>
					</div>
				{:else if item.type === 'message'}
					<div class="message-wrapper">
						<Message
							message={item.message}
							index={item.index}
							{showThinking}
							{expandAllTools}
							{expandAllResults}
						/>
					</div>
				{:else if item.type === 'agent'}
					<div class="agent-wrapper">
						<AgentPanel
							agent={item.agent}
							{run}
							depth={0}
							{showThinking}
							{expandAllAgents}
							{expandAllTools}
							{expandAllResults}
						/>
					</div>
				{/if}
			{/snippet}
		</SvelteVirtualList>
	</div>
{/if}

<style>
	.message-list-wrapper {
		height: 80vh;
		width: 100%;
		overflow: hidden;
	}

	/* Item wrappers for proper spacing */
	.message-wrapper,
	.agent-wrapper {
		padding: 0 0.5rem;
		margin-bottom: 1rem;
	}

	/* Scrollbar styling for virtual list */
	.message-list-wrapper :global(::-webkit-scrollbar) {
		width: 8px;
	}

	.message-list-wrapper :global(::-webkit-scrollbar-track) {
		background: #f1f1f1;
		border-radius: 4px;
	}

	.message-list-wrapper :global(::-webkit-scrollbar-thumb) {
		background: #888;
		border-radius: 4px;
		transition: background 0.2s ease;
	}

	.message-list-wrapper :global(::-webkit-scrollbar-thumb:hover) {
		background: #555;
	}

	.message-list-wrapper :global(::-webkit-scrollbar-thumb:active) {
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
