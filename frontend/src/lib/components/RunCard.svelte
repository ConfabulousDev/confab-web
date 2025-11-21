<script lang="ts">
	import type { RunDetail, GitInfo, TodoItem } from '$lib/types';
	import { formatDate, formatBytes } from '$lib/utils';
	import { onMount } from 'svelte';
	import TranscriptViewer from './transcript/TranscriptViewer.svelte';

	export let run: RunDetail;
	export let index: number;
	export let showGitInfo = true;
	export let shareToken: string | undefined = undefined;
	export let sessionId: string | undefined = undefined;

	let todos: { agent_id: string; items: TodoItem[] }[] = [];
	let loadingTodos = false;
	let showTranscript = false;

	function getRepoWebURL(repoUrl?: string): string | null {
		if (!repoUrl) return null;

		// Convert SSH URLs to HTTPS
		if (repoUrl.startsWith('git@github.com:')) {
			return repoUrl.replace('git@github.com:', 'https://github.com/').replace(/\.git$/, '');
		}
		if (repoUrl.startsWith('git@gitlab.com:')) {
			return repoUrl.replace('git@gitlab.com:', 'https://gitlab.com/').replace(/\.git$/, '');
		}

		// HTTPS URLs
		if (repoUrl.startsWith('https://github.com/') || repoUrl.startsWith('https://gitlab.com/')) {
			return repoUrl.replace(/\.git$/, '');
		}

		return null;
	}

	function getCommitURL(gitInfo?: GitInfo): string | null {
		const repoUrl = getRepoWebURL(gitInfo?.repo_url);
		if (!repoUrl || !gitInfo?.commit_sha) return null;

		if (repoUrl.includes('github.com')) {
			return `${repoUrl}/commit/${gitInfo.commit_sha}`;
		}
		if (repoUrl.includes('gitlab.com')) {
			return `${repoUrl}/-/commit/${gitInfo.commit_sha}`;
		}

		return null;
	}

	// Extract agent ID from todo file path
	// Format: {sessionID}-agent-{agentID}.json
	function extractAgentID(filePath: string): string {
		const fileName = filePath.split('/').pop() || '';
		const match = fileName.match(/-agent-([^.]+)\.json$/);
		return match ? match[1] : 'unknown';
	}

	async function loadTodos() {
		const todoFiles = run.files.filter((f) => f.file_type === 'todo');
		if (todoFiles.length === 0) return;

		loadingTodos = true;
		const loadedTodos: { agent_id: string; items: TodoItem[] }[] = [];

		for (const file of todoFiles) {
			try {
				// Fetch todo file content from backend
				// Use shared endpoint if shareToken is provided
				const url = shareToken && sessionId
					? `/api/v1/sessions/${sessionId}/shared/${shareToken}/files/${file.id}/content`
					: `/api/v1/runs/${run.id}/files/${file.id}/content`;
				const response = await fetch(url, {
					credentials: 'include'
				});

				if (!response.ok) continue;

				const content = await response.text();
				const items: TodoItem[] = JSON.parse(content);

				// Only add if there are actual todos
				if (items.length > 0) {
					loadedTodos.push({
						agent_id: extractAgentID(file.file_path),
						items
					});
				}
			} catch (err) {
				console.error('Failed to load todo file:', file.file_path, err);
			}
		}

		todos = loadedTodos;
		loadingTodos = false;
	}

	onMount(() => {
		loadTodos();
	});
</script>

<div class="run-card">
	<div class="run-header">
		<div class="header-left">
			<h3>Run #{index + 1}</h3>
			<span class="timestamp">{formatDate(run.end_timestamp)}</span>
		</div>
		<button class="view-transcript-btn" on:click={() => (showTranscript = !showTranscript)}>
			{showTranscript ? 'Hide' : 'View'} Transcript
		</button>
	</div>

	<div class="run-info">
		<div class="info-row">
			<span class="label">Working Directory:</span>
			<code class="value">{run.cwd}</code>
		</div>
		<div class="info-row">
			<span class="label">End Reason:</span>
			<span class="value">{run.reason}</span>
		</div>
		<div class="info-row">
			<span class="label">Transcript:</span>
			<code class="value">{run.transcript_path}</code>
		</div>
		<div class="info-row">
			<span class="label">Cloud Backup:</span>
			<span class="value {run.s3_uploaded ? 'success' : 'muted'}">
				{run.s3_uploaded ? '✓ Uploaded' : '✗ Not uploaded'}
			</span>
		</div>
	</div>

	{#if showGitInfo && run.git_info}
		<div class="git-info-section">
			<h4>Git Information</h4>
			<div class="git-info">
				{#if run.git_info.repo_url}
					<div class="info-row">
						<span class="label">Repository:</span>
						{#if getRepoWebURL(run.git_info.repo_url)}
							<a
								href={getRepoWebURL(run.git_info.repo_url)}
								target="_blank"
								rel="noopener"
								class="value link"
							>
								{run.git_info.repo_url}
							</a>
						{:else}
							<code class="value">{run.git_info.repo_url}</code>
						{/if}
					</div>
				{/if}

				{#if run.git_info.branch}
					<div class="info-row">
						<span class="label">Branch:</span>
						<code class="value">{run.git_info.branch}</code>
						{#if run.git_info.is_dirty}
							<span class="dirty-badge">⚠ Uncommitted changes</span>
						{/if}
					</div>
				{/if}

				{#if run.git_info.commit_sha}
					<div class="info-row">
						<span class="label">Commit:</span>
						{#if getCommitURL(run.git_info)}
							<a
								href={getCommitURL(run.git_info)}
								target="_blank"
								rel="noopener"
								class="value link"
							>
								<code>{run.git_info.commit_sha.substring(0, 7)}</code>
							</a>
						{:else}
							<code class="value">{run.git_info.commit_sha.substring(0, 7)}</code>
						{/if}
					</div>
				{/if}

				{#if run.git_info.commit_message}
					<div class="info-row">
						<span class="label">Message:</span>
						<span class="value">{run.git_info.commit_message}</span>
					</div>
				{/if}

				{#if run.git_info.author}
					<div class="info-row">
						<span class="label">Author:</span>
						<span class="value">{run.git_info.author}</span>
					</div>
				{/if}
			</div>
		</div>
	{/if}

	{#if run.files && run.files.length > 0}
		<div class="files-section">
			<h4>Files ({run.files.length})</h4>
			<div class="files-list">
				{#each run.files as file}
					<div class="file-item">
						<div class="file-info">
							<span class="file-type {file.file_type}">{file.file_type}</span>
							<code class="file-path">{file.file_path}</code>
						</div>
						<span class="file-size">{formatBytes(file.size_bytes)}</span>
					</div>
				{/each}
			</div>
		</div>
	{/if}

	{#if todos.length > 0}
		<div class="todos-section">
			<h4>Todo Lists ({todos.length})</h4>
			{#each todos as todoGroup}
				<div class="todo-group">
					<h5>Agent: {todoGroup.agent_id}</h5>
					<div class="todo-list">
						{#each todoGroup.items as item}
							<div class="todo-item status-{item.status}">
								<span class="todo-status-icon">
									{#if item.status === 'completed'}
										✓
									{:else if item.status === 'in_progress'}
										⟳
									{:else}
										○
									{/if}
								</span>
								<span class="todo-content">{item.content}</span>
							</div>
						{/each}
					</div>
				</div>
			{/each}
		</div>
	{/if}

	{#if showTranscript}
		<div class="transcript-section">
			<TranscriptViewer {run} {shareToken} {sessionId} />
		</div>
	{/if}
</div>

<style>
	.run-card {
		background: white;
		border-radius: 8px;
		box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
		padding: 1.5rem;
		margin-bottom: 1.5rem;
	}

	.run-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 1rem;
		padding-bottom: 1rem;
		border-bottom: 1px solid #dee2e6;
	}

	.header-left {
		display: flex;
		align-items: center;
		gap: 1rem;
	}

	.view-transcript-btn {
		padding: 0.5rem 1rem;
		background: #007bff;
		color: white;
		border: none;
		border-radius: 4px;
		cursor: pointer;
		font-size: 0.9rem;
		font-weight: 500;
		transition: background 0.2s;
	}

	.view-transcript-btn:hover {
		background: #0056b3;
	}

	.run-header h3 {
		font-size: 1.25rem;
		color: #222;
		margin: 0;
	}

	.timestamp {
		color: #6c757d;
		font-size: 0.9rem;
	}

	.run-info {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
		margin-bottom: 1rem;
	}

	.info-row {
		display: flex;
		gap: 1rem;
	}

	.info-row .label {
		font-weight: 600;
		color: #495057;
		min-width: 150px;
	}

	.info-row .value {
		color: #212529;
	}

	.info-row code.value {
		background: #f8f9fa;
		padding: 0.25rem 0.5rem;
		border-radius: 4px;
		font-size: 0.9rem;
	}

	.success {
		color: #28a745;
	}

	.muted {
		color: #6c757d;
	}

	.git-info-section {
		margin-top: 1.5rem;
		padding-top: 1.5rem;
		border-top: 1px solid #dee2e6;
	}

	.git-info-section h4 {
		font-size: 1rem;
		color: #495057;
		margin: 0 0 1rem 0;
	}

	.git-info {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
	}

	.link {
		color: #1976d2;
		text-decoration: none;
	}

	.link:hover {
		text-decoration: underline;
	}

	.dirty-badge {
		font-size: 0.75rem;
		background: #fff3cd;
		color: #856404;
		padding: 0.2rem 0.5rem;
		border-radius: 4px;
		margin-left: 0.5rem;
	}

	.files-section {
		margin-top: 1.5rem;
		padding-top: 1.5rem;
		border-top: 1px solid #dee2e6;
	}

	.files-section h4 {
		font-size: 1rem;
		color: #495057;
		margin: 0 0 1rem 0;
	}

	.files-list {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.file-item {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 0.75rem;
		background: #f8f9fa;
		border-radius: 6px;
	}

	.file-info {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		flex: 1;
	}

	.file-type {
		font-size: 0.75rem;
		font-weight: 600;
		padding: 0.25rem 0.5rem;
		border-radius: 4px;
		text-transform: uppercase;
	}

	.file-type.transcript {
		background: #d1ecf1;
		color: #0c5460;
	}

	.file-type.agent {
		background: #d4edda;
		color: #155724;
	}

	.file-type.todo {
		background: #fff3cd;
		color: #856404;
	}

	.file-path {
		font-family: monospace;
		font-size: 0.85rem;
		color: #495057;
	}

	.file-size {
		color: #6c757d;
		font-size: 0.85rem;
		white-space: nowrap;
	}

	.todos-section {
		margin-top: 1.5rem;
		padding-top: 1.5rem;
		border-top: 1px solid #dee2e6;
	}

	.todos-section h4 {
		font-size: 1rem;
		color: #495057;
		margin: 0 0 1rem 0;
	}

	.todo-group {
		margin-bottom: 1.5rem;
	}

	.todo-group:last-child {
		margin-bottom: 0;
	}

	.todo-group h5 {
		font-size: 0.9rem;
		color: #6c757d;
		margin: 0 0 0.75rem 0;
		font-weight: 600;
	}

	.todo-list {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.todo-item {
		display: flex;
		align-items: flex-start;
		gap: 0.75rem;
		padding: 0.75rem;
		background: #f8f9fa;
		border-radius: 6px;
		border-left: 3px solid transparent;
	}

	.todo-item.status-pending {
		border-left-color: #6c757d;
	}

	.todo-item.status-in_progress {
		border-left-color: #007bff;
		background: #e7f3ff;
	}

	.todo-item.status-completed {
		border-left-color: #28a745;
		background: #e8f5e9;
	}

	.todo-status-icon {
		font-size: 1rem;
		line-height: 1.5;
		min-width: 1.25rem;
		text-align: center;
	}

	.todo-item.status-pending .todo-status-icon {
		color: #6c757d;
	}

	.todo-item.status-in_progress .todo-status-icon {
		color: #007bff;
	}

	.todo-item.status-completed .todo-status-icon {
		color: #28a745;
	}

	.todo-content {
		color: #212529;
		font-size: 0.9rem;
		line-height: 1.5;
		flex: 1;
	}

	.todo-item.status-completed .todo-content {
		color: #6c757d;
		text-decoration: line-through;
	}

	.transcript-section {
		margin-top: 1.5rem;
		padding-top: 1.5rem;
		border-top: 1px solid #dee2e6;
	}
</style>
