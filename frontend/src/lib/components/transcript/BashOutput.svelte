<script lang="ts">
	export let output: string;
	export let command: string = '';
	export let exitCode: number | null = null;
	export let maxHeight: string = '400px';

	let copySuccess = false;

	// Parse ANSI color codes (basic support)
	function parseANSI(text: string): string {
		// Remove ANSI escape sequences for now
		// In the future, we could convert them to HTML colors
		return text.replace(/\x1b\[[0-9;]*m/g, '');
	}

	// Format the output
	const cleanOutput = parseANSI(output);
	const hasError = exitCode !== null && exitCode !== 0;

	async function copyToClipboard() {
		try {
			await navigator.clipboard.writeText(output);
			copySuccess = true;
			setTimeout(() => {
				copySuccess = false;
			}, 2000);
		} catch (err) {
			console.error('Failed to copy:', err);
		}
	}
</script>

<div class="bash-output" class:error={hasError}>
	<button class="copy-btn" on:click={copyToClipboard} title="Copy output to clipboard">
		{#if copySuccess}
			✓
		{:else}
			📋
		{/if}
	</button>
	{#if command}
		<div class="bash-prompt">
			<span class="prompt-symbol">$</span>
			<span class="command">{command}</span>
		</div>
	{/if}
	<div class="bash-content" style="max-height: {maxHeight};">
		<pre>{cleanOutput}</pre>
	</div>
	{#if exitCode !== null && exitCode !== 0}
		<div class="exit-code">
			<span class="exit-label">Exit code:</span>
			<span class="exit-value">{exitCode}</span>
		</div>
	{/if}
</div>

<style>
	.bash-output {
		background: #1e1e1e;
		color: #d4d4d4;
		border-radius: 4px;
		overflow: hidden;
		font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', 'Courier New', monospace;
		font-size: 0.85rem;
		line-height: 1.5;
		position: relative;
	}

	.bash-output.error {
		border: 2px solid #dc3545;
	}

	.copy-btn {
		position: absolute;
		top: 0.5rem;
		right: 0.5rem;
		background: rgba(45, 45, 45, 0.9);
		border: 1px solid #555;
		border-radius: 4px;
		padding: 0.25rem 0.5rem;
		font-size: 0.75rem;
		cursor: pointer;
		z-index: 10;
		transition: all 0.2s;
		font-weight: 500;
		color: #d4d4d4;
	}

	.copy-btn:hover {
		background: #555;
		border-color: #4ec9b0;
		color: #4ec9b0;
	}

	.copy-btn:active {
		transform: scale(0.95);
	}

	.bash-prompt {
		background: #2d2d2d;
		padding: 0.5rem 0.75rem;
		border-bottom: 1px solid #3e3e3e;
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.prompt-symbol {
		color: #4ec9b0;
		font-weight: bold;
	}

	.command {
		color: #ce9178;
		flex: 1;
	}

	.bash-content {
		padding: 0.75rem;
		overflow-x: auto;
		overflow-y: auto;
	}

	.bash-content pre {
		margin: 0;
		white-space: pre-wrap;
		word-wrap: break-word;
		font-family: inherit;
		font-size: inherit;
		line-height: inherit;
		color: inherit;
	}

	/* Custom scrollbar for bash output */
	.bash-content::-webkit-scrollbar {
		width: 8px;
		height: 8px;
	}

	.bash-content::-webkit-scrollbar-track {
		background: #2d2d2d;
	}

	.bash-content::-webkit-scrollbar-thumb {
		background: #555;
		border-radius: 4px;
	}

	.bash-content::-webkit-scrollbar-thumb:hover {
		background: #666;
	}

	.exit-code {
		background: #2d2d2d;
		padding: 0.5rem 0.75rem;
		border-top: 1px solid #3e3e3e;
		display: flex;
		align-items: center;
		gap: 0.5rem;
		font-size: 0.8rem;
	}

	.exit-label {
		color: #858585;
	}

	.exit-value {
		color: #f48771;
		font-weight: bold;
	}

	/* Mobile responsive styles */
	@media (max-width: 768px) {
		.copy-btn {
			padding: 0.2rem 0.4rem;
			font-size: 0.7rem;
		}

		.bash-prompt {
			padding: 0.4rem 0.6rem;
			font-size: 0.85rem;
		}

		.bash-content {
			padding: 0.6rem;
		}

		.bash-content pre {
			font-size: 0.8rem;
		}

		.exit-code {
			padding: 0.4rem 0.6rem;
			font-size: 0.75rem;
		}
	}

	@media (max-width: 480px) {
		.copy-btn {
			top: 0.25rem;
			right: 0.25rem;
			padding: 0.15rem 0.3rem;
			font-size: 0.65rem;
		}

		.bash-prompt {
			padding: 0.3rem 0.5rem;
			font-size: 0.8rem;
		}

		.bash-content {
			padding: 0.5rem;
		}

		.bash-content pre {
			font-size: 0.75rem;
		}

		.exit-code {
			padding: 0.3rem 0.5rem;
			font-size: 0.7rem;
		}
	}
</style>
