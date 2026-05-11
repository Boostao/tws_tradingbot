<script lang="ts">
	import { onMount } from 'svelte';
	import favicon from '$lib/assets/favicon.svg';
	import '../app.css';
	import { initLanguage, language, t } from '$lib/i18n';
	import { startRuntimePolling } from '$lib/stores/runtime';
    import Sidebar from '$lib/components/Sidebar.svelte';

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

<style>
.app-shell {
    display: flex;
    height: 100vh;
    width: 100vw;
    overflow: hidden;
    background: #121212;
    color: #e0e0e0;
}
.app-main {
    flex: 1;
    overflow: auto;
    padding: 10px;
}
</style>
