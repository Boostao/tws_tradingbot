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

<h1>{t('watchlist')}</h1>
<p class="muted">{status}</p>
{#if error}
	<p style="color: #f87171;">{error}</p>
{/if}
