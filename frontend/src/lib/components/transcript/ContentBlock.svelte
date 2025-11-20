<script lang="ts">
	import type { ContentBlock } from '$lib/types';
	import {
		isTextBlock,
		isThinkingBlock,
		isToolUseBlock,
		isToolResultBlock
	} from '$lib/types/transcript';
	import CodeBlock from './CodeBlock.svelte';
	import BashOutput from './BashOutput.svelte';

	export let block: ContentBlock;
	export let index: number = 0;
	export let toolName: string = ''; // Optional tool name from parent
	export let showThinking: boolean = true; // Global thinking visibility toggle
	export let expandAllTools: boolean = false;
	export let expandAllResults: boolean = true;

	let toolExpanded = expandAllTools;
	let toolResultExpanded = expandAllResults;
	let thinkingExpanded = false; // Default collapsed for thinking

	// React to changes in expand all controls
	$: toolExpanded = expandAllTools;
	$: toolResultExpanded = expandAllResults;

	// Auto-link URLs in text
	function linkify(text: string): string {
		const urlRegex = /(https?:\/\/[^\s]+)/g;
		return text.replace(urlRegex, '<a href="$1" target="_blank" rel="noopener noreferrer">$1</a>');
	}

	// Check if text contains markdown-like formatting
	function hasMarkdown(text: string): boolean {
		return /[*_`#\[\]]/. test(text);
	}

	// Detect language from tool name
	function getToolResultLanguage(toolName: string): string {
		const toolLanguages: Record<string, string> = {
			Bash: 'bash',
			Read: 'typescript',
			Write: 'typescript',
			Edit: 'typescript',
			Grep: 'bash',
			Glob: 'bash',
			WebFetch: 'html',
			WebSearch: 'json',
			NotebookEdit: 'python'
		};
		return toolLanguages[toolName] || 'plain';
	}

	// Format tool result content for display
	function formatToolResult(content: any): string {
		if (typeof content === 'string') {
			return content;
		}
		return JSON.stringify(content, null, 2);
	}

	// Detect if this is Bash-like output
	function isBashOutput(content: string, tool: string): boolean {
		if (tool === 'Bash') return true;
		// Heuristic: check for common bash patterns
		return (
			content.includes('$ ') ||
			content.match(/^[\w@-]+:/) !== null || // Typical bash prompt
			content.includes('\n$ ')
		);
	}

	// Track tool name from tool_use blocks
	$: if (isToolUseBlock(block)) {
		toolName = block.name;
	}
</script>

{#if isTextBlock(block)}
	<div class="text-block">
		<pre>{@html linkify(block.text)}</pre>
	</div>
{:else if isThinkingBlock(block)}
	{#if showThinking}
		<div class="thinking-block">
			<div class="thinking-header" on:click={() => (thinkingExpanded = !thinkingExpanded)}>
				<span class="thinking-icon">💭</span>
				<span class="thinking-label">Thinking</span>
				<button class="expand-btn">{thinkingExpanded ? '▼' : '▶'}</button>
			</div>
			{#if thinkingExpanded}
				<div class="thinking-content">
					<pre>{block.thinking}</pre>
				</div>
			{/if}
		</div>
	{/if}
{:else if isToolUseBlock(block)}
	<div class="tool-use-block">
		<div class="tool-header" on:click={() => (toolExpanded = !toolExpanded)}>
			<span class="tool-icon">🛠️</span>
			<span class="tool-name">{block.name}</span>
			<button class="expand-btn">{toolExpanded ? '▼' : '▶'}</button>
		</div>
		{#if toolExpanded}
			<div class="tool-input">
				<CodeBlock code={JSON.stringify(block.input, null, 2)} language="json" />
			</div>
		{/if}
	</div>
{:else if isToolResultBlock(block)}
	<div class="tool-result-block" class:error={block.is_error}>
		<div class="tool-result-header" on:click={() => (toolResultExpanded = !toolResultExpanded)}>
			<span class="result-icon">{block.is_error ? '❌' : '✅'}</span>
			<span>Tool Result</span>
			<button class="expand-btn">{toolResultExpanded ? '▼' : '▶'}</button>
		</div>
		{#if toolResultExpanded}
			<div class="tool-result-content">
				{#if typeof block.content === 'string'}
					{#if isBashOutput(block.content, toolName)}
						<BashOutput output={block.content} />
					{:else}
						<CodeBlock
							code={block.content}
							language="plain"
							maxHeight="500px"
							truncateLines={100}
						/>
					{/if}
				{:else}
					<!-- Recursive rendering for nested content blocks -->
					{#each block.content as nestedBlock}
						<svelte:self
							block={nestedBlock}
							{toolName}
							{showThinking}
							{expandAllTools}
							{expandAllResults}
						/>
					{/each}
				{/if}
			</div>
		{/if}
	</div>
{:else}
	<div class="unknown-block">
		<em>Unknown content block type</em>
	</div>
{/if}

<style>
	pre {
		margin: 0;
		white-space: pre-wrap;
		word-wrap: break-word;
		font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
		font-size: 0.9rem;
		line-height: 1.5;
	}

	.text-block {
		color: #212529;
	}

	.thinking-block {
		background: #f8f9fa;
		border: 1px solid #dee2e6;
		border-radius: 4px;
		padding: 0.75rem;
	}

	.thinking-header {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		font-weight: 600;
		color: #6c757d;
		font-size: 0.9rem;
		cursor: pointer;
		user-select: none;
		padding: 0.25rem 0;
		transition: opacity 0.2s ease, transform 0.1s ease;
	}

	.thinking-header:hover {
		opacity: 0.8;
	}

	.thinking-header:active {
		transform: scale(0.98);
	}

	.thinking-icon {
		font-size: 1rem;
	}

	.thinking-label {
		flex: 1;
	}

	.thinking-content {
		color: #495057;
		font-style: italic;
		margin-top: 0.5rem;
		animation: fadeIn 0.2s ease-out;
	}

	@keyframes fadeIn {
		from {
			opacity: 0;
		}
		to {
			opacity: 1;
		}
	}

	/* Style links in pre blocks */
	pre :global(a) {
		color: #007bff;
		text-decoration: underline;
	}

	pre :global(a:hover) {
		color: #0056b3;
	}

	.tool-use-block {
		background: #e7f3ff;
		border: 1px solid #007bff;
		border-radius: 4px;
		padding: 0.75rem;
	}

	.tool-header {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		font-weight: 600;
		color: #007bff;
		cursor: pointer;
		user-select: none;
		padding: 0.25rem 0;
		transition: opacity 0.2s ease, transform 0.1s ease;
	}

	.tool-header:hover {
		opacity: 0.8;
	}

	.tool-header:active {
		transform: scale(0.98);
	}

	.tool-name {
		font-size: 0.95rem;
		flex: 1;
	}

	.expand-btn {
		background: none;
		border: none;
		font-size: 0.8rem;
		color: inherit;
		cursor: pointer;
		padding: 0.25rem;
		line-height: 1;
	}

	.tool-input {
		margin-top: 0.5rem;
		animation: fadeIn 0.2s ease-out;
	}

	.tool-result-block {
		background: #d4edda;
		border: 1px solid #28a745;
		border-radius: 4px;
		padding: 0.75rem;
	}

	.tool-result-block.error {
		background: #f8d7da;
		border-color: #dc3545;
	}

	.tool-result-header {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		font-weight: 600;
		color: #155724;
		font-size: 0.9rem;
		cursor: pointer;
		user-select: none;
		padding: 0.25rem 0;
		transition: opacity 0.2s ease, transform 0.1s ease;
	}

	.tool-result-header:hover {
		opacity: 0.8;
	}

	.tool-result-header:active {
		transform: scale(0.98);
	}

	.tool-result-block.error .tool-result-header {
		color: #721c24;
	}

	.result-icon {
		font-size: 1rem;
	}

	.tool-result-header span:not(.result-icon):not(.expand-btn) {
		flex: 1;
	}

	.tool-result-content {
		margin-top: 0.5rem;
		animation: fadeIn 0.2s ease-out;
	}

	.unknown-block {
		color: #6c757d;
		font-style: italic;
	}

	/* Mobile responsive styles */
	@media (max-width: 768px) {
		.thinking-block {
			padding: 0.6rem;
		}

		.thinking-header,
		.tool-header,
		.tool-result-header {
			padding: 0.2rem 0;
			font-size: 0.85rem;
		}

		.thinking-icon,
		.tool-icon,
		.result-icon {
			font-size: 0.9rem;
		}

		.tool-name {
			font-size: 0.9rem;
		}

		.tool-use-block,
		.tool-result-block {
			padding: 0.6rem;
		}

		.tool-input,
		.tool-result-content {
			margin-top: 0.4rem;
		}

		pre {
			font-size: 0.85rem;
		}
	}

	@media (max-width: 480px) {
		.thinking-block,
		.tool-use-block,
		.tool-result-block {
			padding: 0.5rem;
		}

		.thinking-header,
		.tool-header,
		.tool-result-header {
			font-size: 0.8rem;
		}

		.thinking-icon,
		.tool-icon,
		.result-icon {
			font-size: 0.85rem;
		}

		.tool-name {
			font-size: 0.85rem;
		}

		.expand-btn {
			font-size: 0.75rem;
			padding: 0.2rem;
		}

		pre {
			font-size: 0.8rem;
		}
	}
</style>
