<script lang="ts">
	import { onMount } from 'svelte';
	import type { Session } from '$lib/types';
	import { formatDate, formatRelativeTime } from '$lib/utils';

	let sessions: Session[] = [];
	let loading = true;
	let error = '';

	onMount(async () => {
		await fetchSessions();
	});

	async function fetchSessions() {
		loading = true;
		error = '';
		try {
			const response = await fetch('/api/v1/sessions', {
				credentials: 'include'
			});

			if (response.status === 401) {
				window.location.href = '/';
				return;
			}

			if (!response.ok) {
				throw new Error('Failed to fetch sessions');
			}

			sessions = await response.json();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load sessions';
		} finally {
			loading = false;
		}
	}
</script>

<div class="container">
	<div class="header">
		<h1>Sessions</h1>
		<a href="/" class="btn-link">← Back to Home</a>
	</div>

	{#if error}
		<div class="alert alert-error">
			{error}
		</div>
	{/if}

	<div class="card">
		{#if loading}
			<p class="loading">Loading sessions...</p>
		{:else if sessions.length === 0}
			<p class="empty">No sessions yet. Sessions will appear here after you use confab.</p>
		{:else}
			<div class="sessions-table">
				<table>
					<thead>
						<tr>
							<th>Session ID</th>
							<th>First Seen</th>
							<th>Runs</th>
							<th>Last Activity</th>
							<th></th>
						</tr>
					</thead>
					<tbody>
						{#each sessions as session (session.session_id)}
							<tr>
								<td>
									<code class="session-id">{session.session_id.substring(0, 8)}</code>
								</td>
								<td>{formatDate(session.first_seen)}</td>
								<td>{session.run_count}</td>
								<td>
									<span class="relative-time">{formatRelativeTime(session.last_run_time)}</span>
								</td>
								<td>
									<a href="/sessions/{session.session_id}" class="btn btn-sm">View</a>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	</div>
</div>

<style>
	/* Override container width for sessions list */
	.container {
		max-width: 1200px;
	}

	.header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 2rem;
	}

	.header h1 {
		font-size: 2rem;
		color: #222;
	}

	.sessions-table {
		overflow-x: auto;
	}

	table {
		width: 100%;
		border-collapse: collapse;
	}

	thead {
		background: #f8f9fa;
	}

	th {
		text-align: left;
		padding: 0.75rem;
		font-weight: 600;
		color: #495057;
		border-bottom: 2px solid #dee2e6;
	}

	td {
		padding: 0.75rem;
		border-bottom: 1px solid #dee2e6;
		color: #212529;
	}

	tbody tr:hover {
		background: #f8f9fa;
	}

	.session-id {
		font-family: monospace;
		background: #e9ecef;
		padding: 0.25rem 0.5rem;
		border-radius: 4px;
		font-size: 0.9rem;
	}

	.relative-time {
		color: #6c757d;
		font-size: 0.9rem;
	}
</style>
