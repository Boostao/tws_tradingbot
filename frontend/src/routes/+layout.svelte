<script lang="ts">
	import { onMount } from 'svelte';
	import { browser } from '$app/environment';
	import { page } from '$app/stores';
	import favicon from '$lib/assets/favicon.svg';
	import '../app.css';
	import { initLanguage, language, t } from '$lib/i18n';
	import { startRuntimePolling } from '$lib/stores/runtime';
	import {
		closeHelpDrawer,
		completeOrDismissTour,
		helpDrawerOpen,
		initOnboardingOnCockpit,
		toggleHelpDrawer,
		tourStore
	} from '$lib/stores/onboarding';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import HelpDrawer from '$lib/components/HelpDrawer.svelte';
	import OnboardingTour from '$lib/components/OnboardingTour.svelte';

	let onboardingChecked = false;

	$: lang = $language;
	$: if (browser && !onboardingChecked && $page.url.pathname === '/cockpit') {
		onboardingChecked = true;
		initOnboardingOnCockpit();
	}

	function handleGlobalKeydown(event: KeyboardEvent) {
		if (event.key === 'F1') {
			event.preventDefault();
			if (!$tourStore.open) {
				toggleHelpDrawer();
			}
			return;
		}
		if (event.key === 'Escape') {
			if ($tourStore.open) {
				event.preventDefault();
				event.stopImmediatePropagation();
				completeOrDismissTour(false);
			} else if ($helpDrawerOpen) {
				event.preventDefault();
				event.stopImmediatePropagation();
				closeHelpDrawer();
			}
		}
	}

	onMount(() => {
		initLanguage();
		return startRuntimePolling();
	});
</script>

<svelte:window on:keydown={handleGlobalKeydown} />

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>{lang && t('page_title')}</title>
</svelte:head>

<div class="app-shell">
	<Sidebar />
	<main class="app-main">
		<slot />
	</main>
</div>

<HelpDrawer />
<OnboardingTour />
