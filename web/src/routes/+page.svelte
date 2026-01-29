<script lang="ts">
	import { onMount } from 'svelte';
	import { getHealth } from '$lib/api';
	import { t, language } from '$lib/i18n';

	let status = 'checking';
	let error = '';
	$: _lang = $language;

	onMount(async () => {
		try {
			const result = await getHealth();
			status = result.status === 'ok' ? 'online' : result.status;
		} catch (err) {
			status = 'offline';
			error = err instanceof Error ? err.message : 'Unknown error';
		}
	});
</script>

<h1>{t('refactor_in_progress')}</h1>
<p>{t('backend_health_check')}</p>

<div class="card">
	<span class="badge">
		<span class={`status-dot ${status === 'online' ? '' : 'offline'}`}></span>
		{status}
	</span>
	{#if error}
		<p style="color: #f87171; margin-top: 12px;">{error}</p>
	{/if}
</div>

<div class="card">
	<h2>{t('next_up')}</h2>
	<ul>
		<li>{t('next_wire_monitoring')}</li>
		<li>{t('next_strategy_editor')}</li>
		<li>{t('next_backtest_runner')}</li>
	</ul>
</div>
