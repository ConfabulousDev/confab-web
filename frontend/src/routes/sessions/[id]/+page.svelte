<script lang="ts">
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import { fetchWithCSRF } from '$lib/csrf';
	import type { SessionDetail, SessionShare } from '$lib/types';
	import { formatDate } from '$lib/utils';
	import RunCard from '$lib/components/RunCard.svelte';

	let session: SessionDetail | null = null;
	let loading = true;
	let error = '';
	let sessionId: string;

	// Share dialog state
	let showShareDialog = false;
	let shareVisibility: 'public' | 'private' = 'public';
	let invitedEmails: string[] = [];
	let newEmail = '';
	let expiresInDays: number | null = 7;
	let createdShareURL = '';
	let shares: SessionShare[] = [];
	let loadingShares = false;

	// Run selection state
	let selectedRunIndex = 0;

	$: sessionId = $page.params.id;

	$: selectedRun = session?.runs[selectedRunIndex];

	onMount(async () => {
		await fetchSession();
	});

	async function fetchSession() {
		loading = true;
		error = '';
		try {
			const response = await fetch(`/api/v1/sessions/${sessionId}`, {
				credentials: 'include'
			});

			if (response.status === 401) {
				window.location.href = '/';
				return;
			}

			if (response.status === 404) {
				error = 'Session not found';
				loading = false;
				return;
			}

			if (!response.ok) {
				throw new Error('Failed to fetch session');
			}

			session = await response.json();

			// Set initial selection to the latest run by timestamp
			if (session.runs && session.runs.length > 0) {
				const latestIndex = session.runs.reduce((latestIdx, run, idx) => {
					return new Date(run.end_timestamp) > new Date(session.runs[latestIdx].end_timestamp)
						? idx
						: latestIdx;
				}, 0);
				selectedRunIndex = latestIndex;
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load session';
		} finally {
			loading = false;
		}
	}

	async function fetchShares() {
		loadingShares = true;
		try {
			const response = await fetch(`/api/v1/sessions/${sessionId}/shares`, {
				credentials: 'include'
			});
			if (response.ok) {
				shares = await response.json();
			}
		} catch (err) {
			console.error('Failed to load shares:', err);
		} finally {
			loadingShares = false;
		}
	}

	function openShareDialog() {
		showShareDialog = true;
		createdShareURL = '';
		shareVisibility = 'public';
		invitedEmails = [];
		newEmail = '';
		expiresInDays = 7;
		fetchShares();
	}

	function addEmail() {
		const email = newEmail.trim();
		if (email && email.includes('@') && !invitedEmails.includes(email)) {
			invitedEmails = [...invitedEmails, email];
			newEmail = '';
		}
	}

	function removeEmail(email: string) {
		invitedEmails = invitedEmails.filter((e) => e !== email);
	}

	async function createShare() {
		error = '';
		try {
			const response = await fetchWithCSRF(`/api/v1/sessions/${sessionId}/share`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					visibility: shareVisibility,
					invited_emails: shareVisibility === 'private' ? invitedEmails : [],
					expires_in_days: expiresInDays
				})
			});

			if (!response.ok) {
				throw new Error('Failed to create share');
			}

			const result = await response.json();
			createdShareURL = result.share_url;
			await fetchShares();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create share';
		}
	}

	async function revokeShare(shareToken: string) {
		if (!confirm('Are you sure you want to revoke this share?')) {
			return;
		}

		try {
			const response = await fetchWithCSRF(`/api/v1/shares/${shareToken}`, {
				method: 'DELETE'
			});

			if (!response.ok) {
				throw new Error('Failed to revoke share');
			}

			await fetchShares();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to revoke share';
		}
	}

	function copyToClipboard(text: string) {
		navigator.clipboard.writeText(text);
		alert('Copied to clipboard!');
	}
</script>

<div class="container">
	<div class="header">
		<div>
			<h1>Session Detail</h1>
			{#if session}
				<p class="session-id">
					<strong>Session ID:</strong> <code>{session.session_id}</code>
				</p>
			{/if}
		</div>
		<div class="header-actions">
			<button class="btn btn-share" on:click={openShareDialog}>📤 Share</button>
			<a href="/sessions" class="btn-link">← Back to Sessions</a>
		</div>
	</div>

	{#if error}
		<div class="alert alert-error">
			{error}
		</div>
	{:else if loading}
		<div class="card">
			<p class="loading">Loading session...</p>
		</div>
	{:else if session}
		<div class="meta-card">
			<div class="meta-item">
				<span class="meta-label">First Seen:</span>
				<span class="meta-value">{formatDate(session.first_seen)}</span>
			</div>

			<!-- Version selector dropdown (only show if multiple runs) -->
			{#if session.runs.length > 1}
				<div class="meta-item">
					<span class="meta-label">Select Version:</span>
					<select id="run-select" bind:value={selectedRunIndex} class="version-select">
						{#each session.runs as run, index}
							{@const isLatestRun = session.runs.every(
								(r) => new Date(run.end_timestamp) >= new Date(r.end_timestamp)
							)}
							{@const isOldestRun = session.runs.every(
								(r) => new Date(run.end_timestamp) <= new Date(r.end_timestamp)
							)}
							{@const label = isLatestRun ? 'latest' : isOldestRun ? 'started' : 'updated'}
							<option value={index}>
								#{index + 1} {label} @ {formatDate(run.end_timestamp)}
							</option>
						{/each}
					</select>
				</div>
			{/if}
		</div>

		<!-- Display the selected run -->
		{#if selectedRun}
			<RunCard run={selectedRun} index={selectedRunIndex} />
		{/if}
	{/if}
</div>

<!-- Share Dialog Modal -->
{#if showShareDialog}
	<div class="modal-overlay" on:click={() => (showShareDialog = false)}>
		<div class="modal" on:click|stopPropagation>
			<div class="modal-header">
				<h2>Share Session</h2>
				<button class="close-btn" on:click={() => (showShareDialog = false)}>×</button>
			</div>

			<div class="modal-body">
				{#if createdShareURL}
					<div class="success-message">
						<h3>✓ Share Link Created</h3>
						<div class="share-url-box">
							<input type="text" readonly value={createdShareURL} class="share-url-input" />
							<button class="btn btn-sm" on:click={() => copyToClipboard(createdShareURL)}>
								Copy
							</button>
						</div>
					</div>
				{:else}
					<div class="form-group">
						<label>
							<input
								type="radio"
								bind:group={shareVisibility}
								value="public"
							/>
							<strong>Public</strong> - Anyone with link
						</label>
						<label>
							<input
								type="radio"
								bind:group={shareVisibility}
								value="private"
							/>
							<strong>Private</strong> - Invite specific people
						</label>
					</div>

					{#if shareVisibility === 'private'}
						<div class="form-group">
							<label>Invite by email:</label>
							<div class="email-input-group">
								<input
									type="email"
									bind:value={newEmail}
									placeholder="email@example.com"
									on:keydown={(e) => e.key === 'Enter' && addEmail()}
								/>
								<button class="btn btn-sm" on:click={addEmail}>Add</button>
							</div>
							{#if invitedEmails.length > 0}
								<div class="email-list">
									{#each invitedEmails as email}
										<span class="email-tag">
											{email}
											<button class="remove-btn" on:click={() => removeEmail(email)}>×</button>
										</span>
									{/each}
								</div>
							{/if}
						</div>
					{/if}

					<div class="form-group">
						<label>Expires:</label>
						<select bind:value={expiresInDays}>
							<option value={1}>1 day</option>
							<option value={7}>7 days</option>
							<option value={30}>30 days</option>
							<option value={null}>Never</option>
						</select>
					</div>

					<div class="modal-footer">
						<button class="btn btn-primary" on:click={createShare}>Create Share Link</button>
						<button class="btn btn-secondary" on:click={() => (showShareDialog = false)}>
							Cancel
						</button>
					</div>
				{/if}

				<div class="shares-list">
					<h3>Active Shares</h3>
					{#if loadingShares}
						<p>Loading...</p>
					{:else if shares.length === 0}
						<p class="empty">No active shares</p>
					{:else}
						{#each shares as share}
							<div class="share-item">
								<div class="share-info">
									<span class="visibility-badge {share.visibility}">
										{share.visibility}
									</span>
									{#if share.visibility === 'private' && share.invited_emails}
										<span class="invited">
											{share.invited_emails.join(', ')}
										</span>
									{/if}
									{#if share.expires_at}
										<span class="expires">Expires: {formatDate(share.expires_at)}</span>
									{:else}
										<span class="never-expires">Never expires</span>
									{/if}
								</div>
								<button class="btn btn-danger btn-sm" on:click={() => revokeShare(share.share_token)}>
									Revoke
								</button>
							</div>
						{/each}
					{/if}
				</div>
			</div>
		</div>
	</div>
{/if}

<style>
	.container {
		max-width: 1200px;
		margin: 0 auto;
		padding: 2rem;
	}

	.header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
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

	.loading {
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

	.meta-card {
		background: white;
		border-radius: 8px;
		box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
		padding: 1rem;
		margin-bottom: 1rem;
		display: flex;
		gap: 1.5rem;
	}

	.meta-item {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}

	.version-select {
		padding: 0.5rem 0.75rem;
		border: 1px solid #ced4da;
		border-radius: 6px;
		font-size: 0.875rem;
		color: #495057;
		background-color: white;
		cursor: pointer;
		transition: border-color 0.2s, box-shadow 0.2s;
		width: auto;
		min-width: 250px;
	}

	.version-select:hover {
		border-color: #80bdff;
	}

	.version-select:focus {
		outline: none;
		border-color: #007bff;
		box-shadow: 0 0 0 0.2rem rgba(0, 123, 255, 0.25);
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

	.git-info-section {
		margin-top: 1.5rem;
		padding-top: 1.5rem;
		border-top: 1px solid #dee2e6;
	}

	.git-info-section h4 {
		font-size: 1rem;
		color: #495057;
		margin: 0 0 1rem 0;
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.git-info {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
		background: #f8f9fa;
		padding: 1rem;
		border-radius: 6px;
	}

	.value.link {
		color: #007bff;
		text-decoration: none;
	}

	.value.link:hover {
		text-decoration: underline;
	}

	.dirty-badge {
		display: inline-block;
		font-size: 0.75rem;
		padding: 0.25rem 0.5rem;
		background: #fff3cd;
		color: #856404;
		border-radius: 4px;
		margin-left: 0.5rem;
		font-weight: 600;
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

	/* Share button and header */
	.header-actions {
		display: flex;
		gap: 1rem;
		align-items: center;
	}

	.btn-share {
		background: #28a745;
	}

	.btn-share:hover {
		background: #218838;
	}

	/* Modal */
	.modal-overlay {
		position: fixed;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
		background: rgba(0, 0, 0, 0.5);
		display: flex;
		justify-content: center;
		align-items: center;
		z-index: 1000;
	}

	.modal {
		background: white;
		border-radius: 8px;
		max-width: 600px;
		width: 90%;
		max-height: 90vh;
		overflow-y: auto;
		box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
	}

	.modal-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 1.5rem;
		border-bottom: 1px solid #dee2e6;
	}

	.modal-header h2 {
		margin: 0;
		font-size: 1.5rem;
	}

	.close-btn {
		background: none;
		border: none;
		font-size: 2rem;
		line-height: 1;
		cursor: pointer;
		color: #6c757d;
	}

	.close-btn:hover {
		color: #212529;
	}

	.modal-body {
		padding: 1.5rem;
	}

	.modal-footer {
		display: flex;
		gap: 0.5rem;
		justify-content: flex-end;
		margin-top: 1.5rem;
	}

	.form-group {
		margin-bottom: 1.5rem;
	}

	.form-group label {
		display: block;
		margin-bottom: 0.5rem;
		font-weight: 500;
	}

	.form-group input[type='radio'] {
		margin-right: 0.5rem;
	}

	.form-group select,
	.form-group input[type='email'] {
		width: 100%;
		padding: 0.5rem;
		border: 1px solid #ced4da;
		border-radius: 6px;
		font-size: 1rem;
	}

	.email-input-group {
		display: flex;
		gap: 0.5rem;
	}

	.email-input-group input {
		flex: 1;
	}

	.email-list {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		margin-top: 0.5rem;
	}

	.email-tag {
		background: #e9ecef;
		padding: 0.25rem 0.5rem;
		border-radius: 4px;
		font-size: 0.9rem;
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.remove-btn {
		background: none;
		border: none;
		cursor: pointer;
		font-size: 1.2rem;
		line-height: 1;
		color: #6c757d;
	}

	.remove-btn:hover {
		color: #dc3545;
	}

	.success-message {
		background: #d4edda;
		border: 1px solid #c3e6cb;
		padding: 1rem;
		border-radius: 6px;
		margin-bottom: 1.5rem;
	}

	.success-message h3 {
		color: #155724;
		margin: 0 0 0.5rem 0;
	}

	.share-url-box {
		display: flex;
		gap: 0.5rem;
	}

	.share-url-input {
		flex: 1;
		padding: 0.5rem;
		border: 1px solid #c3e6cb;
		border-radius: 6px;
		font-family: monospace;
		font-size: 0.9rem;
	}

	.shares-list {
		margin-top: 1.5rem;
	}

	.shares-list h3 {
		font-size: 1.1rem;
		margin-bottom: 1rem;
	}

	.share-item {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 1rem;
		background: #f8f9fa;
		border-radius: 6px;
		margin-bottom: 0.5rem;
	}

	.share-info {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}

	.visibility-badge {
		display: inline-block;
		padding: 0.25rem 0.5rem;
		border-radius: 4px;
		font-size: 0.75rem;
		font-weight: 600;
		text-transform: uppercase;
		width: fit-content;
	}

	.visibility-badge.public {
		background: #d1ecf1;
		color: #0c5460;
	}

	.visibility-badge.private {
		background: #fff3cd;
		color: #856404;
	}

	.invited,
	.expires,
	.never-expires {
		font-size: 0.85rem;
		color: #6c757d;
	}

	.btn-secondary {
		background: #6c757d;
	}

	.btn-secondary:hover {
		background: #5a6268;
	}

	.empty {
		color: #6c757d;
		font-style: italic;
	}
</style>
