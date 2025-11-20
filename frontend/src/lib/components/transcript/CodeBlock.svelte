<script lang="ts">
	import { onMount } from 'svelte';
	import Prism from 'prismjs';

	// Import core languages
	import 'prismjs/components/prism-bash';
	import 'prismjs/components/prism-typescript';
	import 'prismjs/components/prism-javascript';
	import 'prismjs/components/prism-json';
	import 'prismjs/components/prism-python';
	import 'prismjs/components/prism-go';
	import 'prismjs/components/prism-markdown';
	import 'prismjs/components/prism-yaml';
	import 'prismjs/components/prism-sql';
	import 'prismjs/components/prism-css';
	import 'prismjs/components/prism-markup'; // HTML/XML

	// Import a clean theme
	import 'prismjs/themes/prism.css';

	export let code: string;
	export let language: string = 'plain';
	export let showLineNumbers: boolean = false;
	export let maxHeight: string = 'none';
	export let truncateLines: number = 0; // 0 = no truncation

	let highlightedCode = '';
	let isTruncated = false;
	let showingFull = false;
	let displayCode = code;
	let copySuccess = false;

	// Check if code needs truncation
	$: {
		if (truncateLines > 0 && !showingFull) {
			const lines = code.split('\n');
			if (lines.length > truncateLines) {
				isTruncated = true;
				displayCode = lines.slice(0, truncateLines).join('\n');
			} else {
				isTruncated = false;
				displayCode = code;
			}
		} else {
			displayCode = code;
		}
	}

	function toggleFullView() {
		showingFull = !showingFull;
	}

	async function copyToClipboard() {
		try {
			await navigator.clipboard.writeText(code);
			copySuccess = true;
			setTimeout(() => {
				copySuccess = false;
			}, 2000);
		} catch (err) {
			console.error('Failed to copy:', err);
		}
	}

	// Map common aliases to Prism language names
	const languageMap: Record<string, string> = {
		js: 'javascript',
		ts: 'typescript',
		py: 'python',
		sh: 'bash',
		shell: 'bash',
		yml: 'yaml',
		html: 'markup',
		xml: 'markup',
		txt: 'plain',
		text: 'plain'
	};

	function normalizeLanguage(lang: string): string {
		const normalized = lang.toLowerCase().trim();
		return languageMap[normalized] || normalized;
	}

	function highlightCode() {
		const lang = normalizeLanguage(language);

		// Check if language is supported
		if (lang === 'plain' || !Prism.languages[lang]) {
			highlightedCode = escapeHtml(displayCode);
			return;
		}

		try {
			highlightedCode = Prism.highlight(displayCode, Prism.languages[lang], lang);
		} catch (e) {
			console.warn(`Failed to highlight code with language '${lang}':`, e);
			highlightedCode = escapeHtml(displayCode);
		}
	}

	function escapeHtml(text: string): string {
		const div = document.createElement('div');
		div.textContent = text;
		return div.innerHTML;
	}

	onMount(() => {
		highlightCode();
	});

	// Re-highlight when displayCode or language changes
	$: if (displayCode || language) {
		highlightCode();
	}
</script>

<div class="code-block" class:line-numbers={showLineNumbers}>
	<button class="copy-btn" on:click={copyToClipboard} title="Copy to clipboard">
		{#if copySuccess}
			✓ Copied
		{:else}
			📋 Copy
		{/if}
	</button>
	<pre style="max-height: {maxHeight};"><code class="language-{normalizeLanguage(language)}">{@html highlightedCode}</code></pre>
	{#if isTruncated}
		<div class="truncate-notice">
			<span class="truncate-text">
				{showingFull ? '' : `Showing first ${truncateLines} lines...`}
			</span>
			<button class="expand-toggle" on:click={toggleFullView}>
				{showingFull ? 'Show less' : 'Show all'}
			</button>
		</div>
	{/if}
</div>

<style>
	.code-block {
		position: relative;
		background: #f8f9fa;
		border-radius: 4px;
		overflow: hidden;
	}

	.copy-btn {
		position: absolute;
		top: 0.5rem;
		right: 0.5rem;
		background: rgba(255, 255, 255, 0.9);
		border: 1px solid #dee2e6;
		border-radius: 4px;
		padding: 0.25rem 0.5rem;
		font-size: 0.75rem;
		cursor: pointer;
		z-index: 10;
		transition: all 0.2s;
		font-weight: 500;
		color: #495057;
		backdrop-filter: blur(4px);
	}

	.copy-btn:hover {
		background: white;
		border-color: #007bff;
		color: #007bff;
	}

	.copy-btn:active {
		transform: scale(0.95);
	}

	.copy-btn:has(*:first-child:not(:empty)) {
		animation: pulse 0.5s ease;
	}

	@keyframes pulse {
		0% {
			transform: scale(1);
		}
		50% {
			transform: scale(1.1);
		}
		100% {
			transform: scale(1);
		}
	}

	pre {
		margin: 0;
		padding: 1rem;
		overflow-x: auto;
		overflow-y: auto;
		font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
		font-size: 0.85rem;
		line-height: 1.5;
		background: transparent !important;
	}

	code {
		font-family: inherit;
		font-size: inherit;
		background: transparent !important;
	}

	/* Custom scrollbar for code blocks */
	pre::-webkit-scrollbar {
		width: 8px;
		height: 8px;
	}

	pre::-webkit-scrollbar-track {
		background: #e9ecef;
		border-radius: 4px;
	}

	pre::-webkit-scrollbar-thumb {
		background: #adb5bd;
		border-radius: 4px;
	}

	pre::-webkit-scrollbar-thumb:hover {
		background: #868e96;
	}

	/* Line numbers support */
	.line-numbers pre {
		counter-reset: line-numbering;
	}

	.line-numbers code {
		display: block;
	}

	.line-numbers code::before {
		counter-increment: line-numbering;
		content: counter(line-numbering);
		display: inline-block;
		width: 3em;
		margin-right: 1em;
		padding-right: 0.5em;
		text-align: right;
		color: #6c757d;
		border-right: 1px solid #dee2e6;
		user-select: none;
	}

	/* Truncation notice */
	.truncate-notice {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 0.5rem;
		background: #e9ecef;
		border-top: 1px solid #dee2e6;
		font-size: 0.85rem;
	}

	.truncate-text {
		color: #6c757d;
		font-style: italic;
	}

	.expand-toggle {
		background: #007bff;
		color: white;
		border: none;
		border-radius: 4px;
		padding: 0.25rem 0.75rem;
		font-size: 0.85rem;
		font-weight: 500;
		cursor: pointer;
		transition: background 0.2s;
	}

	.expand-toggle:hover {
		background: #0056b3;
	}

	/* Mobile responsive styles */
	@media (max-width: 768px) {
		.copy-btn {
			padding: 0.2rem 0.4rem;
			font-size: 0.7rem;
		}

		pre {
			padding: 0.75rem;
			font-size: 0.8rem;
		}

		.truncate-notice {
			padding: 0.4rem;
			font-size: 0.8rem;
		}

		.expand-toggle {
			padding: 0.2rem 0.6rem;
			font-size: 0.8rem;
		}
	}

	@media (max-width: 480px) {
		.copy-btn {
			top: 0.25rem;
			right: 0.25rem;
			padding: 0.15rem 0.3rem;
			font-size: 0.65rem;
		}

		pre {
			padding: 0.5rem;
			font-size: 0.75rem;
		}

		.truncate-notice {
			padding: 0.3rem;
			font-size: 0.75rem;
		}
	}
</style>
