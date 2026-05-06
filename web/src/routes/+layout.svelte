<script lang="ts">
	import { onMount } from 'svelte';
	import favicon from '$lib/assets/favicon.svg';
	import '../app.css';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import { initLanguage, language, t } from '$lib/i18n';
	import { startRuntimePolling } from '$lib/stores/runtime';

	$: _lang = $language;

	onMount(() => {
		initLanguage();
		return startRuntimePolling();
	});
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>{t('page_title')}</title>
</svelte:head>

<div class="app-shell">
	<Sidebar />
	<main class="app-main">
		{#key $language}
			<slot />
		{/key}
	</main>
</div>
