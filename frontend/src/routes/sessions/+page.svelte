<script lang="ts">
	import { onMount } from 'svelte';
	import type { Session } from '$lib/types';
	import { formatDate, formatRelativeTime } from '$lib/utils';

	let sessions: Session[] = [];
	let loading = true;
	let error = '';
	let sortColumn: 'title' | 'session_id' | 'last_run_time' = 'last_run_time';
	let sortDirection: 'asc' | 'desc' = 'desc'; // Default: most recent first

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

	function handleSort(column: 'title' | 'session_id' | 'last_run_time') {
		if (sortColumn === column) {
			// Toggle direction if clicking the same column
			sortDirection = sortDirection === 'asc' ? 'desc' : 'asc';
		} else {
			// New column: default to ascending (except last_run_time defaults to descending)
			sortColumn = column;
			sortDirection = column === 'last_run_time' ? 'desc' : 'asc';
		}
	}

	// Sorted sessions (reactive) - filter out empty sessions (0 byte transcripts)
	$: sortedSessions = (() => {
		// Filter out sessions where all runs have 0-byte transcripts
		const filtered = sessions.filter(s => s.max_transcript_size > 0);

		const sorted = [...filtered];
		sorted.sort((a, b) => {
			let aVal, bVal;

			switch (sortColumn) {
				case 'title':
					aVal = a.title || 'Untitled Session';
					bVal = b.title || 'Untitled Session';
					break;
				case 'session_id':
					aVal = a.session_id;
					bVal = b.session_id;
					break;
				case 'last_run_time':
					aVal = new Date(a.last_run_time).getTime();
					bVal = new Date(b.last_run_time).getTime();
					break;
			}

			if (aVal < bVal) return sortDirection === 'asc' ? -1 : 1;
			if (aVal > bVal) return sortDirection === 'asc' ? 1 : -1;
			return 0;
		});
		return sorted;
	})();
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
							<th class="sortable" on:click={() => handleSort('title')}>
								Title
								{#if sortColumn === 'title'}
									<span class="sort-indicator">{sortDirection === 'asc' ? '↑' : '↓'}</span>
								{/if}
							</th>
							<th class="sortable" on:click={() => handleSort('session_id')}>
								Session ID
								{#if sortColumn === 'session_id'}
									<span class="sort-indicator">{sortDirection === 'asc' ? '↑' : '↓'}</span>
								{/if}
							</th>
							<th class="sortable" on:click={() => handleSort('last_run_time')}>
								Last Activity
								{#if sortColumn === 'last_run_time'}
									<span class="sort-indicator">{sortDirection === 'asc' ? '↑' : '↓'}</span>
								{/if}
							</th>
						</tr>
					</thead>
					<tbody>
						{#each sortedSessions as session (session.session_id)}
							<tr class="clickable-row" on:click={() => window.location.href = `/sessions/${session.session_id}`}>
								<td class:session-title={!session.title}>{session.title || 'Untitled Session'}</td>
								<td>
									<code class="session-id">{session.session_id.substring(0, 8)}</code>
								</td>
								<td>{formatDate(session.last_run_time)}</td>
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
		margin-bottom: 1.5rem;
	}

	.header h1 {
		font-size: 1.75rem;
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
		padding: 0.5rem 0.75rem;
		font-weight: 600;
		color: #495057;
		border-bottom: 2px solid #dee2e6;
	}

	th.sortable {
		cursor: pointer;
		user-select: none;
		transition: background-color 0.2s ease;
	}

	th.sortable:hover {
		background: #e9ecef;
	}

	.sort-indicator {
		margin-left: 0.25rem;
		font-size: 0.85rem;
		color: #007bff;
	}

	td {
		padding: 0.5rem 0.75rem;
		border-bottom: 1px solid #dee2e6;
		color: #212529;
		font-size: 0.9rem;
	}

	.clickable-row {
		cursor: pointer;
		transition: background-color 0.2s ease;
	}

	.clickable-row:hover {
		background: #e9ecef;
	}

	.session-title {
		color: #6c757d;
		font-style: italic;
	}

	.session-type {
		color: #495057;
		font-weight: 500;
	}

	.session-id {
		font-family: monospace;
		background: #e9ecef;
		padding: 0.2rem 0.4rem;
		border-radius: 4px;
		font-size: 0.85rem;
	}

	.relative-time {
		color: #6c757d;
		font-size: 0.9rem;
	}
</style>
