<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import type { SessionDetail } from '$lib/types';
	import { formatDate } from '$lib/utils';
	import RunCard from '$lib/components/RunCard.svelte';

	const sessionId = $page.params.sessionId;
	const shareToken = $page.params.token;

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
			<RunCard {run} {index} showGitInfo={false} {shareToken} sessionId={session.session_id} />
		{/each}
	{/if}
</div>

<style>
	/* Override container width for shared view */
	.container {
		max-width: 1200px;
	}

	/* Override loading style for this page */
	.loading {
		padding: 3rem;
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
</style>
