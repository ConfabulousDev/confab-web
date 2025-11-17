<script lang="ts">
	import { onMount } from 'svelte';
	import { fetchWithCSRF } from '$lib/csrf';

	type APIKey = {
		id: number;
		name: string;
		created_at: string;
	};

	let keys: APIKey[] = [];
	let loading = true;
	let error = '';
	let newKeyName = '';
	let createdKey: { key: string; name: string } | null = null;
	let showCreateForm = false;

	onMount(async () => {
		await fetchKeys();
	});

	async function fetchKeys() {
		loading = true;
		error = '';
		try {
			const response = await fetch('/api/v1/keys', {
				credentials: 'include'
			});

			if (response.status === 401) {
				window.location.href = '/';
				return;
			}

			if (!response.ok) {
				throw new Error('Failed to fetch keys');
			}

			keys = await response.json();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load API keys';
		} finally {
			loading = false;
		}
	}

	async function createKey() {
		if (!newKeyName.trim()) {
			error = 'Please enter a key name';
			return;
		}

		error = '';
		try {
			const response = await fetchWithCSRF('/api/v1/keys', {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json'
				},
				body: JSON.stringify({ name: newKeyName })
			});

			if (!response.ok) {
				throw new Error('Failed to create key');
			}

			const result = await response.json();
			createdKey = { key: result.key, name: result.name };
			newKeyName = '';
			showCreateForm = false;
			await fetchKeys();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create key';
		}
	}

	async function deleteKey(id: number, name: string) {
		if (!confirm(`Are you sure you want to delete "${name}"? This cannot be undone.`)) {
			return;
		}

		error = '';
		try {
			const response = await fetchWithCSRF(`/api/v1/keys/${id}`, {
				method: 'DELETE'
			});

			if (!response.ok) {
				throw new Error('Failed to delete key');
			}

			await fetchKeys();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to delete key';
		}
	}

	function copyToClipboard(text: string) {
		navigator.clipboard.writeText(text);
		alert('API key copied to clipboard!');
	}

	function formatDate(dateStr: string): string {
		const date = new Date(dateStr);
		return date.toLocaleDateString() + ' ' + date.toLocaleTimeString();
	}
</script>

<div class="container">
	<div class="header">
		<h1>API Keys</h1>
		<a href="/" class="btn-link">← Back to Home</a>
	</div>

	{#if createdKey}
		<div class="alert alert-success">
			<h3>✓ API Key Created Successfully!</h3>
			<p><strong>Name:</strong> {createdKey.name}</p>
			<div class="key-display">
				<code>{createdKey.key}</code>
				<button class="btn btn-sm" on:click={() => copyToClipboard(createdKey!.key)}>
					Copy
				</button>
			</div>
			<p class="warning">
				⚠️ This is the only time you'll see this key. Save it securely!
			</p>
			<button class="btn" on:click={() => (createdKey = null)}>Close</button>
		</div>
	{/if}

	{#if error}
		<div class="alert alert-error">
			{error}
		</div>
	{/if}

	<div class="card">
		<div class="card-header">
			<h2>Your API Keys</h2>
			{#if !showCreateForm}
				<button class="btn btn-primary" on:click={() => (showCreateForm = true)}>
					+ Create New Key
				</button>
			{/if}
		</div>

		{#if showCreateForm}
			<div class="create-form">
				<h3>Create New API Key</h3>
				<input
					type="text"
					placeholder="Key name (e.g., Production Server, My Laptop)"
					bind:value={newKeyName}
					class="input"
				/>
				<div class="form-actions">
					<button class="btn btn-primary" on:click={createKey}>Create Key</button>
					<button class="btn btn-secondary" on:click={() => (showCreateForm = false)}>
						Cancel
					</button>
				</div>
			</div>
		{/if}

		{#if loading}
			<p class="loading">Loading...</p>
		{:else if keys.length === 0}
			<p class="empty">No API keys yet. Create one to get started!</p>
		{:else}
			<div class="keys-list">
				{#each keys as key (key.id)}
					<div class="key-item">
						<div class="key-info">
							<h3>{key.name}</h3>
							<p class="key-meta">Created: {formatDate(key.created_at)}</p>
						</div>
						<button class="btn btn-danger btn-sm" on:click={() => deleteKey(key.id, key.name)}>
							Delete
						</button>
					</div>
				{/each}
			</div>
		{/if}
	</div>

	<div class="help">
		<h3>Using API Keys</h3>
		<p>Use your API key with the Confab CLI:</p>
		<pre><code>confab configure --api-key &lt;your-key&gt;</code></pre>
		<p>Or use <code>confab login</code> for interactive authentication.</p>
	</div>
</div>

<style>
	.container {
		max-width: 900px;
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
		margin-bottom: 2rem;
	}

	.card-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 1.5rem;
	}

	.card-header h2 {
		font-size: 1.5rem;
		color: #222;
		margin: 0;
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

	.btn-primary {
		background: #28a745;
	}

	.btn-primary:hover {
		background: #218838;
	}

	.btn-secondary {
		background: #6c757d;
	}

	.btn-secondary:hover {
		background: #5a6268;
	}

	.btn-danger {
		background: #dc3545;
	}

	.btn-danger:hover {
		background: #c82333;
	}

	.btn-sm {
		padding: 0.25rem 0.75rem;
		font-size: 0.85rem;
	}

	.create-form {
		background: #f8f9fa;
		padding: 1.5rem;
		border-radius: 6px;
		margin-bottom: 1.5rem;
	}

	.create-form h3 {
		margin-bottom: 1rem;
		color: #222;
	}

	.input {
		width: 100%;
		padding: 0.75rem;
		border: 1px solid #ddd;
		border-radius: 6px;
		font-size: 1rem;
		margin-bottom: 1rem;
	}

	.input:focus {
		outline: none;
		border-color: #28a745;
	}

	.form-actions {
		display: flex;
		gap: 0.5rem;
	}

	.loading,
	.empty {
		text-align: center;
		padding: 2rem;
		color: #666;
	}

	.keys-list {
		display: flex;
		flex-direction: column;
		gap: 1rem;
	}

	.key-item {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 1rem;
		border: 1px solid #e1e4e8;
		border-radius: 6px;
	}

	.key-info h3 {
		font-size: 1.1rem;
		margin: 0 0 0.25rem 0;
		color: #222;
	}

	.key-meta {
		color: #666;
		font-size: 0.9rem;
		margin: 0;
	}

	.alert {
		padding: 1.5rem;
		border-radius: 6px;
		margin-bottom: 2rem;
	}

	.alert-success {
		background: #d4edda;
		border: 1px solid #c3e6cb;
		color: #155724;
	}

	.alert-error {
		background: #f8d7da;
		border: 1px solid #f5c6cb;
		color: #721c24;
	}

	.alert h3 {
		margin: 0 0 0.5rem 0;
	}

	.key-display {
		display: flex;
		align-items: center;
		gap: 1rem;
		margin: 1rem 0;
		padding: 1rem;
		background: white;
		border-radius: 4px;
	}

	.key-display code {
		flex: 1;
		font-family: monospace;
		word-break: break-all;
		font-size: 0.9rem;
	}

	.warning {
		color: #856404;
		background: #fff3cd;
		padding: 0.75rem;
		border-radius: 4px;
		margin: 1rem 0;
	}

	.help {
		background: #f8f9fa;
		padding: 1.5rem;
		border-radius: 8px;
	}

	.help h3 {
		margin: 0 0 1rem 0;
		color: #222;
	}

	.help p {
		margin: 0.5rem 0;
		color: #666;
	}

	.help pre {
		background: white;
		padding: 1rem;
		border-radius: 4px;
		overflow-x: auto;
		margin: 0.5rem 0;
	}

	.help code {
		font-family: monospace;
		font-size: 0.9rem;
	}
</style>
