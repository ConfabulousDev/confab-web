<script lang="ts">
	import { onMount } from 'svelte';

	type Session = {
		session_id: string;
		first_seen: string;
		run_count: number;
		last_run_time: string;
	};

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

	function formatDate(dateStr: string): string {
		const date = new Date(dateStr);
		return date.toLocaleString();
	}

	function formatRelativeTime(dateStr: string): string {
		const date = new Date(dateStr);
		const now = new Date();
		const diff = now.getTime() - date.getTime();

		const seconds = Math.floor(diff / 1000);
		const minutes = Math.floor(seconds / 60);
		const hours = Math.floor(minutes / 60);
		const days = Math.floor(hours / 24);

		if (days > 0) return `${days}d ago`;
		if (hours > 0) return `${hours}h ago`;
		if (minutes > 0) return `${minutes}m ago`;
		return `${seconds}s ago`;
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
	.container {
		max-width: 1200px;
		margin: 0 auto;
		padding: 2rem;
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

	.btn-link {
		color: #666;
		text-decoration: none;
	}

	.btn-link:hover {
		color: #222;
	}

	.card {
		background: white;
		border-radius: 8px;
		box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
		padding: 2rem;
	}

	.loading,
	.empty {
		text-align: center;
		padding: 2rem;
		color: #666;
	}

	.alert {
		padding: 1rem;
		border-radius: 6px;
		margin-bottom: 1.5rem;
	}

	.alert-error {
		background: #f8d7da;
		border: 1px solid #f5c6cb;
		color: #721c24;
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

	.btn {
		display: inline-block;
		padding: 0.5rem 1rem;
		background: #24292e;
		color: white;
		text-decoration: none;
		border-radius: 6px;
		font-weight: 500;
		transition: background 0.2s;
		border: none;
		cursor: pointer;
		font-size: 0.9rem;
	}

	.btn:hover {
		background: #444;
	}

	.btn-sm {
		padding: 0.25rem 0.75rem;
		font-size: 0.85rem;
	}
</style>
