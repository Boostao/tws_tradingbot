<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { GetAutopilotState } from '../../../bindings/tws_traderbot/app';
	import type { AutopilotState } from '../../../bindings/tws_traderbot/backend/models/models';

	let state: AutopilotState | null = null;
	let loading = true;
	let error: string | null = null;
	let interval: ReturnType<typeof setInterval> | null = null;

	async function updateState() {
		try {
			state = await GetAutopilotState();
			error = null;
		} catch (e) {
			error = 'Failed to fetch autopilot state';
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		void updateState();
		interval = setInterval(updateState, 2000);
		return () => {
			if (interval) clearInterval(interval);
		};
	});
</script>

<div class="autopilot-panel">
	<h3 class="heading">Autopilot HUD</h3>
	{#if loading}
		<p>Loading...</p>
	{:else if error}
		<p class="error">{error}</p>
	{:else if state}
		<div class="status-grid">
			<div class="status-item">
				<span class="label">Exec Mode</span>
				<span class="value">{state.exec_mode}</span>
			</div>
			<div class="status-item">
				<span class="label">Active Orders</span>
				<span class="value">{state.active_orders.length}</span>
			</div>
			<div class="status-item">
				<span class="label">Session</span>
				<span class="value">{state.session?.is_active ? 'Active' : 'Inactive'}</span>
			</div>
		</div>
	{:else}
		<p>No data available</p>
	{/if}
</div>

<style>
	.autopilot-panel {
		padding: 16px;
		background: var(--bg-card, #0f1726);
		border: 1px solid var(--border-subtle, #1f2937);
		border-radius: 8px;
		color: var(--text-primary, #e6edf3);
	}
	.heading {
		margin: 0 0 12px;
		font-size: 14px;
		color: var(--text-secondary, #94a3b8);
	}
	.status-grid {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 12px;
	}
	.status-item {
		display: flex;
		flex-direction: column;
	}
	.label {
		font-size: 11px;
		color: var(--text-muted, #64748b);
	}
	.value {
		font-size: 14px;
		font-weight: 600;
	}
	.error {
		color: var(--sem-bear-text, #ef5350);
	}
</style>
