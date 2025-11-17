<script lang="ts">
	import { onMount } from 'svelte';
	import { initCSRF } from '$lib/csrf';
	import '../app.css';

	// Initialize CSRF token on app load
	onMount(() => {
		initCSRF();
	});
</script>

<slot />
