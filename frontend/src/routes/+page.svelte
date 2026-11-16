<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';

	// Single-Page Cockpit Redirect
	// Listens for pulse_tick telemetry and redirects root to /cockpit
	onMount(() => {
		void goto('/cockpit', { replaceState: true });
	});
</script>

<div class="cockpit-redirect">
	<p>Redirecting to Cockpit...</p>
</div>

<style>
	.cockpit-redirect {
		display: flex;
		align-items: center;
		justify-content: center;
		min-height: 100vh;
		background: #000000;
		color: #ffffff;
		font-family: 'IoskeleyMono', monospace;
		font-size: 14px;
	}
</style>
