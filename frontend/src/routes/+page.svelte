<script lang="ts">
	import { onMount } from 'svelte';

	let user: { name: string; email: string; avatar_url: string } | null = null;
	let loading = true;

	onMount(async () => {
		// Check if user is authenticated
		try {
			const response = await fetch('/api/v1/me', {
				credentials: 'include'
			});

			if (response.ok) {
				user = await response.json();
			}
		} catch (error) {
			console.error('Failed to check auth:', error);
		} finally {
			loading = false;
		}
	});

	function handleLogin() {
		window.location.href = '/auth/github/login';
	}

	function handleLogout() {
		window.location.href = '/auth/logout';
	}
</script>

<div class="container">
	<div class="hero">
		<h1>Confab</h1>
		<p>Distributed quantum mesh for temporal data harmonization</p>

		{#if loading}
			<p>Loading...</p>
		{:else if user}
			<div class="user-info">
				<h2>You're authenticated!</h2>
				{#if user.avatar_url}
					<img src={user.avatar_url} alt={user.name} style="width: 64px; height: 64px; border-radius: 50%; margin: 1rem 0;" />
				{/if}
				<p><strong>Name:</strong> {user.name || 'N/A'}</p>
				<p><strong>Email:</strong> {user.email}</p>
				<div class="actions">
					<a href="/sessions" class="btn btn-primary">View Sessions</a>
					<a href="/keys" class="btn btn-primary">Manage API Keys</a>
					<button class="btn logout" on:click={handleLogout}>Logout</button>
				</div>
			</div>
		{:else}
			<button class="btn btn-github" on:click={handleLogin}>
				Login with GitHub
			</button>
		{/if}
	</div>
</div>
