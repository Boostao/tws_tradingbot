<script lang="ts">
	import { onMount } from 'svelte';
	import favicon from '$lib/assets/favicon.svg';
	import '../app.css';
	import { initLanguage, language, t } from '$lib/i18n';
	import { startRuntimePolling } from '$lib/stores/runtime';
    import { FailsafeStop } from '../../wailsjs/go/main/App';

	$: _lang = $language;

	onMount(() => {
		initLanguage();
		return startRuntimePolling();
	});

    function triggerKillSwitch() {
        FailsafeStop();
        alert("FAILSAFE INVOKED - ALL TRADING STOPPED");
    }
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>{t('page_title')}</title>
</svelte:head>

<div class="app-shell">
	<main class="app-main">
		{#key $language}
			<slot />
		{/key}
	</main>
</div>

<button class="global-kill-switch" on:click={triggerKillSwitch}>
    GLOBAL DISABLE
</button>

<style>
.global-kill-switch {
    position: fixed;
    bottom: 20px;
    right: 20px;
    background-color: #ff0000;
    color: white;
    font-weight: bold;
    padding: 15px 30px;
    border-radius: 50px;
    box-shadow: 0px 4px 10px rgba(0,0,0,0.5);
    z-index: 9999;
    border: 3px solid darkred;
    cursor: pointer;
    font-size: 1.2rem;
}
.global-kill-switch:active {
    background-color: darkred;
}
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
