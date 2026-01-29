<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import favicon from '$lib/assets/favicon.svg';
	import '../app.css';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import { initLanguage, language, t } from '$lib/i18n';
	import { startBotStatePolling, stopBotStatePolling } from '$lib/stores/botState';

	$: _lang = $language;

	onMount(() => {
		initLanguage();
		startBotStatePolling();
	});

	onDestroy(() => {
		stopBotStatePolling();
	});
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>{t('page_title')}</title>
</svelte:head>

<div class="app-shell">
	{#key $language}
		<Sidebar />
	{/key}
	<main class="app-main">
		{#key $language}
			<slot />
		{/key}
	</main>
</div>
