<script lang="ts">
	import type { RunDetail, TranscriptLine, AgentNode } from '$lib/types';
	import { onMount } from 'svelte';
	import { fetchParsedTranscript } from '$lib/services/transcriptService';
	import { buildAgentTree } from '$lib/services/agentTreeBuilder';
	import MessageList from './MessageList.svelte';

	export let run: RunDetail;
	export let shareToken: string | undefined = undefined;
	export let sessionId: string | undefined = undefined;

	let loading = true;
	let error: string | null = null;
	let messages: TranscriptLine[] = [];
	let agents: AgentNode[] = [];
	let expanded = true;
	let showThinking = true;

	// Batched rendering state
	let allMessages: TranscriptLine[] = [];
	let renderingBatch = false;
	let renderProgress = { current: 0, total: 0 };

	// Expand/collapse all controls
	let expandAllAgents = true;
	let expandAllTools = false;
	let expandAllResults = true;

	onMount(async () => {
		await loadTranscript();
	});

	async function loadTranscript() {
		loading = true;
		error = null;

		const t0 = performance.now();
		try {
			// Find transcript file
			const transcriptFile = run.files.find((f) => f.file_type === 'transcript');
			if (!transcriptFile) {
				throw new Error('No transcript file found');
			}

			// Fetch and parse transcript
			// Use provided sessionId or fall back to transcript_path (for non-shared views)
			const effectiveSessionId = sessionId || run.transcript_path;

			const t1 = performance.now();
			const parsed = await fetchParsedTranscript(
				run.id,
				transcriptFile.id,
				effectiveSessionId,
				shareToken
			);
			const t2 = performance.now();
			console.log(`⏱️ fetchParsedTranscript took ${Math.round(t2 - t1)}ms`);

			// Store all messages
			allMessages = parsed.messages;

			// Build agent tree
			const shareOptions = shareToken && sessionId ? { sessionId, shareToken } : undefined;
			const t3 = performance.now();
			agents = await buildAgentTree(run.id, allMessages, run.files, shareOptions);
			const t4 = performance.now();
			console.log(`⏱️ buildAgentTree took ${Math.round(t4 - t3)}ms`);

			const total = performance.now() - t0;
			console.log(`⏱️ Data load complete: ${Math.round(total)}ms`, {
				messageCount: allMessages.length,
				agentCount: agents.length
			});

			// Start batched rendering
			loading = false;
			if (allMessages.length > 0) {
				renderingBatch = true;
				renderProgress = { current: 0, total: allMessages.length };
				renderNextBatch();
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load transcript';
			console.error('Failed to load transcript:', e);
			loading = false;
		}
	}

	function renderNextBatch() {
		const BATCH_SIZE = 100;
		const start = messages.length;
		const end = Math.min(start + BATCH_SIZE, allMessages.length);

		// Add next batch of messages
		messages = [...messages, ...allMessages.slice(start, end)];
		renderProgress = { current: end, total: renderProgress.total };

		// Continue if there are more messages
		if (end < allMessages.length) {
			setTimeout(renderNextBatch, 0); // Yield to browser
		} else {
			// Rendering complete - delay slightly to show 100% completion
			setTimeout(() => {
				renderingBatch = false;
				console.log(`⏱️ Rendering complete: ${messages.length} messages`);
			}, 300);
		}
	}

	function toggleExpanded() {
		expanded = !expanded;
	}

	function toggleExpandAllAgents() {
		expandAllAgents = !expandAllAgents;
	}

	function toggleExpandAllTools() {
		expandAllTools = !expandAllTools;
	}

	function toggleExpandAllResults() {
		expandAllResults = !expandAllResults;
	}
</script>

<div class="transcript-viewer">
	<div class="transcript-header">
		<h3>Transcript</h3>
		<div class="header-controls">
			<button
				class="toggle-btn thinking-toggle"
				class:active={showThinking}
				on:click={() => (showThinking = !showThinking)}
				title={showThinking ? 'Hide thinking blocks' : 'Show thinking blocks'}
			>
				💭 {showThinking ? 'Hide' : 'Show'} Thinking
			</button>
			<button
				class="toggle-btn"
				on:click={toggleExpandAllAgents}
				title={expandAllAgents ? 'Collapse all agents' : 'Expand all agents'}
			>
				🤖 {expandAllAgents ? 'Collapse' : 'Expand'} Agents
			</button>
			<button
				class="toggle-btn"
				on:click={toggleExpandAllTools}
				title={expandAllTools ? 'Collapse all tool blocks' : 'Expand all tool blocks'}
			>
				🛠️ {expandAllTools ? 'Collapse' : 'Expand'} Tools
			</button>
			<button
				class="toggle-btn"
				on:click={toggleExpandAllResults}
				title={expandAllResults ? 'Collapse all results' : 'Expand all results'}
			>
				✅ {expandAllResults ? 'Collapse' : 'Expand'} Results
			</button>
			<button class="toggle-btn" on:click={toggleExpanded}>
				{expanded ? 'Collapse' : 'Expand'} All
			</button>
		</div>
	</div>

	{#if loading}
		<div class="loading">Loading transcript...</div>
	{:else if error}
		<div class="error">
			<strong>Error:</strong>
			{error}
		</div>
	{:else if expanded}
		<div class="transcript-content">
			{#if renderingBatch}
				<div class="rendering-progress">
					<div class="progress-text">
						Rendering messages: {renderProgress.current.toLocaleString()} / {renderProgress.total.toLocaleString()}
						({Math.round((renderProgress.current / renderProgress.total) * 100)}%)
					</div>
					<div class="progress-bar">
						<div
							class="progress-fill"
							style="width: {(renderProgress.current / renderProgress.total) * 100}%"
						></div>
					</div>
				</div>
			{:else}
				<div class="transcript-meta">
					<span>{messages.length} messages</span>
					{#if agents.length > 0}
						<span>{agents.length} agent{agents.length === 1 ? '' : 's'}</span>
					{/if}
				</div>
			{/if}

			<MessageList
				{messages}
				{agents}
				{run}
				{showThinking}
				{expandAllAgents}
				{expandAllTools}
				{expandAllResults}
			/>
		</div>
	{/if}
</div>

<style>
	.transcript-viewer {
		background: white;
		border: 1px solid #dee2e6;
		border-radius: 8px;
		overflow: hidden;
	}

	.transcript-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 1rem;
		background: #f8f9fa;
		border-bottom: 1px solid #dee2e6;
	}

	.transcript-header h3 {
		margin: 0;
		font-size: 1.1rem;
		color: #212529;
	}

	.header-controls {
		display: flex;
		gap: 0.5rem;
		flex-wrap: wrap;
	}

	.toggle-btn {
		padding: 0.5rem 1rem;
		background: white;
		border: 1px solid #dee2e6;
		border-radius: 4px;
		cursor: pointer;
		font-size: 0.9rem;
		color: #495057;
		transition: all 0.2s;
		white-space: nowrap;
	}

	/* Mobile responsive styles */
	@media (max-width: 768px) {
		.transcript-header {
			flex-direction: column;
			align-items: flex-start;
			gap: 0.75rem;
		}

		.transcript-header h3 {
			font-size: 1rem;
		}

		.header-controls {
			width: 100%;
			justify-content: flex-start;
		}

		.toggle-btn {
			padding: 0.4rem 0.75rem;
			font-size: 0.85rem;
			flex: 0 0 auto;
		}
	}

	@media (max-width: 480px) {
		.toggle-btn {
			padding: 0.35rem 0.6rem;
			font-size: 0.8rem;
		}

		.transcript-header {
			padding: 0.75rem;
		}
	}

	.toggle-btn:hover {
		background: #e9ecef;
	}

	.thinking-toggle {
		display: flex;
		align-items: center;
		gap: 0.25rem;
	}

	.thinking-toggle.active {
		background: #e7f3ff;
		border-color: #007bff;
		color: #007bff;
	}

	.thinking-toggle.active:hover {
		background: #cce5ff;
	}

	.loading,
	.error {
		padding: 2rem;
		text-align: center;
	}

	.error {
		color: #dc3545;
	}

	.transcript-content {
		padding: 1rem;
	}

	.rendering-progress {
		padding: 1rem;
		background: #f8f9fa;
		border-radius: 8px;
		margin-bottom: 1rem;
	}

	.progress-text {
		font-size: 0.9rem;
		color: #495057;
		margin-bottom: 0.75rem;
		text-align: center;
		font-weight: 500;
	}

	.progress-bar {
		width: 100%;
		height: 24px;
		background: #e9ecef;
		border-radius: 12px;
		overflow: hidden;
		position: relative;
	}

	.progress-fill {
		position: absolute;
		top: 0;
		left: 0;
		height: 100%;
		background: linear-gradient(90deg, #28a745, #20c997);
		transition: width 0.2s ease;
		will-change: width;
		min-width: 0;
		max-width: 100%;
	}

	.transcript-meta {
		display: flex;
		gap: 1rem;
		padding: 0.5rem 0 1rem 0;
		font-size: 0.85rem;
		color: #6c757d;
		border-bottom: 1px solid #dee2e6;
		margin-bottom: 1rem;
	}
</style>
