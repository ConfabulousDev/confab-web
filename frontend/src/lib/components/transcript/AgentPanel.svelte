<script lang="ts">
	import type { AgentNode, RunDetail } from '$lib/types';
	import MessageList from './MessageList.svelte';

	export let agent: AgentNode;
	export let run: RunDetail;
	export let depth: number = 0;
	export let showThinking: boolean = true;
	export let expandAllAgents: boolean = true;
	export let expandAllTools: boolean = false;
	export let expandAllResults: boolean = true;

	// React to expandAllAgents changes, but allow manual override
	let expanded = expandAllAgents && depth < 2; // Auto-expand first 2 levels
	$: expanded = expandAllAgents && depth < 2;

	function toggleExpanded() {
		expanded = !expanded;
	}

	// Get color based on depth
	const colors = ['#007bff', '#6f42c1', '#28a745', '#fd7e14', '#dc3545', '#17a2b8'];
	const borderColor = colors[depth % colors.length];

	// Calculate indentation (20px per level, max 100px)
	const indentation = Math.min(depth * 20, 100);

	// Determine if this is a deeply nested agent (depth > 3)
	const isDeeplyNested = depth > 3;
</script>

<div
	class="agent-panel"
	class:deeply-nested={isDeeplyNested}
	style="margin-left: {indentation}px; border-left-color: {borderColor}"
>
	<div class="agent-header" on:click={toggleExpanded}>
		<div class="agent-info">
			<span class="agent-icon">🤖</span>
			{#if depth > 0}
				<span class="depth-indicator" title="Nesting level {depth}">L{depth}</span>
			{/if}
			<span class="agent-label">Agent: {agent.agentId}</span>
			<span class="message-count">{agent.transcript.length} messages</span>
			{#if agent.children.length > 0}
				<span class="child-count" title="{agent.children.length} sub-agent(s)">
					{agent.children.length} sub-agent{agent.children.length === 1 ? '' : 's'}
				</span>
			{/if}
			{#if agent.metadata.status}
				<span class="agent-status status-{agent.metadata.status}">
					{agent.metadata.status}
				</span>
			{/if}
		</div>
		<button class="expand-btn">
			{expanded ? '▼' : '▶'}
		</button>
	</div>

	{#if expanded}
		<div class="agent-content">
			{#if agent.metadata.totalDurationMs}
				<div class="agent-meta">
					<span>Duration: {(agent.metadata.totalDurationMs / 1000).toFixed(1)}s</span>
					{#if agent.metadata.totalTokens}
						<span>Tokens: {agent.metadata.totalTokens.toLocaleString()}</span>
					{/if}
					{#if agent.metadata.totalToolUseCount}
						<span>Tools: {agent.metadata.totalToolUseCount}</span>
					{/if}
				</div>
			{/if}

			<MessageList
				messages={agent.transcript}
				agents={agent.children}
				{run}
				{showThinking}
				{expandAllAgents}
				{expandAllTools}
				{expandAllResults}
			/>

			<!-- Recursively render child agents -->
			{#if agent.children.length > 0}
				<div class="child-agents">
					{#each agent.children as child}
						<svelte:self
							agent={child}
							{run}
							depth={depth + 1}
							{showThinking}
							{expandAllAgents}
							{expandAllTools}
							{expandAllResults}
						/>
					{/each}
				</div>
			{/if}
		</div>
	{/if}
</div>

<style>
	.agent-panel {
		background: white;
		border: 1px solid #dee2e6;
		border-left: 3px solid;
		border-radius: 4px;
		margin-top: 1rem;
		overflow: hidden;
		transition: all 0.2s ease;
	}

	.agent-panel.deeply-nested {
		border-left-width: 4px;
		box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
	}

	.agent-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 0.75rem;
		background: #f8f9fa;
		cursor: pointer;
		user-select: none;
		transition: background 0.2s ease;
	}

	.agent-header:hover {
		background: #e9ecef;
	}

	.agent-header:active {
		transform: scale(0.99);
	}

	.agent-info {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		flex: 1;
	}

	.agent-icon {
		font-size: 1.2rem;
	}

	.depth-indicator {
		font-size: 0.7rem;
		font-weight: 700;
		background: rgba(0, 0, 0, 0.1);
		color: #495057;
		padding: 0.1rem 0.4rem;
		border-radius: 3px;
		font-family: monospace;
		letter-spacing: 0.5px;
	}

	.agent-label {
		font-weight: 600;
		color: #495057;
		font-size: 0.95rem;
	}

	.message-count {
		color: #6c757d;
		font-size: 0.85rem;
	}

	.child-count {
		color: #6c757d;
		font-size: 0.75rem;
		background: #e9ecef;
		padding: 0.2rem 0.5rem;
		border-radius: 4px;
		font-weight: 500;
	}

	.agent-status {
		padding: 0.25rem 0.5rem;
		border-radius: 4px;
		font-size: 0.75rem;
		font-weight: 600;
		text-transform: uppercase;
	}

	.status-completed {
		background: #d4edda;
		color: #155724;
	}

	.status-interrupted {
		background: #fff3cd;
		color: #856404;
	}

	.status-error {
		background: #f8d7da;
		color: #721c24;
	}

	.expand-btn {
		background: none;
		border: none;
		font-size: 0.9rem;
		color: #6c757d;
		cursor: pointer;
		padding: 0.25rem 0.5rem;
		transition: transform 0.2s ease, color 0.2s ease;
	}

	.expand-btn:hover {
		color: #495057;
		transform: scale(1.1);
	}

	.agent-content {
		padding: 1rem;
		animation: slideDown 0.2s ease-out;
	}

	@keyframes slideDown {
		from {
			opacity: 0;
			transform: translateY(-10px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	.agent-meta {
		display: flex;
		gap: 1rem;
		padding: 0.5rem;
		background: #f8f9fa;
		border-radius: 4px;
		margin-bottom: 1rem;
		font-size: 0.85rem;
		color: #6c757d;
	}

	.child-agents {
		margin-top: 1rem;
	}

	/* Mobile responsive styles */
	@media (max-width: 768px) {
		.agent-panel {
			margin-top: 0.75rem;
		}

		.agent-header {
			padding: 0.6rem;
		}

		.agent-info {
			gap: 0.5rem;
			flex-wrap: wrap;
		}

		.agent-icon {
			font-size: 1rem;
		}

		.agent-label {
			font-size: 0.9rem;
		}

		.message-count,
		.child-count {
			font-size: 0.75rem;
		}

		.agent-meta {
			padding: 0.4rem;
			gap: 0.75rem;
			font-size: 0.8rem;
		}

		.agent-content {
			padding: 0.75rem;
		}

		/* Reduce indentation on mobile */
		.agent-panel[style*='margin-left'] {
			margin-left: calc(var(--indent, 0px) * 0.5) !important;
		}
	}

	@media (max-width: 480px) {
		.agent-header {
			padding: 0.5rem;
		}

		.agent-info {
			gap: 0.4rem;
		}

		.agent-label {
			font-size: 0.85rem;
		}

		.depth-indicator {
			font-size: 0.65rem;
			padding: 0.05rem 0.3rem;
		}

		.message-count,
		.child-count {
			font-size: 0.7rem;
		}

		.agent-status {
			font-size: 0.7rem;
			padding: 0.2rem 0.4rem;
		}

		.agent-content {
			padding: 0.5rem;
		}

		.agent-meta {
			font-size: 0.75rem;
			padding: 0.3rem;
			gap: 0.5rem;
		}
	}
</style>
