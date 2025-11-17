<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';

	const sessionId = $page.params.sessionId;
	const shareToken = $page.params.token;

	type FileDetail = {
		id: number;
		file_path: string;
		file_type: string;
		size_bytes: number;
		s3_key?: string;
		s3_uploaded_at?: string;
	};

	type RunDetail = {
		id: number;
		end_timestamp: string;
		cwd: string;
		reason: string;
		transcript_path: string;
		s3_uploaded: boolean;
		files: FileDetail[];
	};

	type SessionDetail = {
		session_id: string;
		first_seen: string;
		runs: RunDetail[];
	};

	let session: SessionDetail | null = null;
	let loading = true;
	let error = '';
	let errorType: 'not_found' | 'expired' | 'unauthorized' | 'forbidden' | 'general' | null = null;

	onMount(async () => {
		try {
			const response = await fetch(`/api/v1/sessions/${sessionId}/shared/${shareToken}`, {
				credentials: 'include'
			});

			if (!response.ok) {
				if (response.status === 404) {
					errorType = 'not_found';
					error = 'Share not found';
				} else if (response.status === 410) {
					errorType = 'expired';
					error = 'This share link has expired';
				} else if (response.status === 401) {
					errorType = 'unauthorized';
					const text = await response.text();
					error = text || 'Please log in to view this private share';
				} else if (response.status === 403) {
					errorType = 'forbidden';
					error = 'You are not authorized to view this share';
				} else {
					errorType = 'general';
					error = 'Failed to load shared session';
				}
				loading = false;
				return;
			}

			session = await response.json();
			loading = false;
		} catch (err) {
			error = 'Failed to load shared session';
			errorType = 'general';
			loading = false;
		}
	});

	function formatDate(dateString: string): string {
		return new Date(dateString).toLocaleString();
	}

	function formatBytes(bytes: number): string {
		if (bytes === 0) return '0 B';
		const k = 1024;
		const sizes = ['B', 'KB', 'MB', 'GB'];
		const i = Math.floor(Math.log(bytes) / Math.log(k));
		return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`;
	}
</script>

<div class="container">
	{#if loading}
		<div class="loading">Loading shared session...</div>
	{:else if error}
		<div class="error-container">
			<div class="error-icon">
				{#if errorType === 'not_found'}
					🔍
				{:else if errorType === 'expired'}
					⏰
				{:else if errorType === 'unauthorized'}
					🔒
				{:else if errorType === 'forbidden'}
					🚫
				{:else}
					⚠️
				{/if}
			</div>
			<h2>{error}</h2>
			{#if errorType === 'unauthorized'}
				<p>This is a private share. Please <a href="/auth/github/login">log in</a> to view it.</p>
			{:else if errorType === 'forbidden'}
				<p>This share is only accessible to invited users.</p>
			{:else if errorType === 'expired'}
				<p>Please request a new share link from the session owner.</p>
			{/if}
		</div>
	{:else if session}
		<!-- Share Banner -->
		<div class="share-banner">
			<span class="share-icon">📤</span>
			<span><strong>Shared Session</strong></span>
		</div>

		<!-- Session Header -->
		<div class="header">
			<div>
				<h1>Session Detail</h1>
				<p class="session-id">
					<strong>Session ID:</strong> <code>{session.session_id}</code>
				</p>
			</div>
		</div>

		<!-- Session Metadata -->
		<div class="meta-card">
			<div class="meta-item">
				<span class="meta-label">First Seen:</span>
				<span class="meta-value">{formatDate(session.first_seen)}</span>
			</div>
			<div class="meta-item">
				<span class="meta-label">Total Runs:</span>
				<span class="meta-value">{session.runs.length}</span>
			</div>
		</div>

		<!-- Runs -->
		<h2>Runs</h2>

		{#each session.runs as run, index}
			<div class="run-card">
				<div class="run-header">
					<h3>Run #{index + 1}</h3>
					<span class="timestamp">{formatDate(run.end_timestamp)}</span>
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
			</div>
		{/each}
	{/if}
</div>

<style>
	.container {
		max-width: 1200px;
		margin: 0 auto;
		padding: 2rem;
	}

	.loading {
		text-align: center;
		padding: 3rem;
		color: #666;
		font-size: 1.1rem;
	}

	.error-container {
		text-align: center;
		padding: 3rem;
		max-width: 500px;
		margin: 0 auto;
	}

	.error-icon {
		font-size: 4rem;
		margin-bottom: 1rem;
	}

	.error-container h2 {
		color: #d32f2f;
		margin-bottom: 1rem;
	}

	.error-container p {
		color: #666;
		margin-bottom: 1rem;
	}

	.error-container a {
		color: #1976d2;
		text-decoration: underline;
	}

	.share-banner {
		background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
		color: white;
		padding: 1rem 1.5rem;
		border-radius: 8px;
		margin-bottom: 2rem;
		display: flex;
		align-items: center;
		gap: 0.75rem;
		font-size: 1.05rem;
	}

	.share-icon {
		font-size: 1.5rem;
	}

	.header {
		margin-bottom: 2rem;
	}

	.header h1 {
		font-size: 2rem;
		color: #222;
		margin: 0 0 0.5rem 0;
	}

	.session-id {
		color: #666;
		margin: 0;
	}

	.session-id code {
		background: #f8f9fa;
		padding: 0.25rem 0.5rem;
		border-radius: 4px;
		font-size: 0.9rem;
	}

	.meta-card {
		background: white;
		border-radius: 8px;
		box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
		padding: 1.5rem;
		margin-bottom: 2rem;
		display: flex;
		gap: 2rem;
	}

	.meta-item {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}

	.meta-label {
		font-size: 0.875rem;
		color: #6c757d;
		font-weight: 500;
	}

	.meta-value {
		font-size: 1.25rem;
		color: #212529;
		font-weight: 600;
	}

	h2 {
		font-size: 1.5rem;
		color: #222;
		margin: 2rem 0 1rem 0;
	}

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
</style>
