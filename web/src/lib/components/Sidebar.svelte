<script lang="ts">
	import { language, setLanguage, t } from '$lib/i18n';
	import { Compass, GitBranch, Languages, List } from 'lucide-svelte';

	$: currentLang = $language;
	$: lang = $language;

	$: navItems = [
		{ href: '/watchlist', label: () => (lang && t('watchlist')) as string, icon: List },
		{ href: '/strategy', label: () => (lang && t('strategy_builder')) as string, icon: GitBranch }
	];
</script>

<aside class="sidebar">
	<div class="sidebar-brand">
		<div class="title">{lang && t('cobalt_title')}</div>
		<div class="subtitle">{lang && t('rule_based_trading_bot')}</div>
	</div>

	<div class="sidebar-section">
		<h4 class="heading">
			<span class="heading-icon"><Compass size={16} strokeWidth={1.6} /></span>
			{lang && t('navigation')}
		</h4>
		<nav class="sidebar-nav">
			{#each navItems as item}
				<a href={item.href} class="nav-item">
					<span class="nav-icon">
						<svelte:component this={item.icon} size={16} strokeWidth={1.6} />
					</span>
					{item.label()}
				</a>
			{/each}
		</nav>
	</div>

	<div class="sidebar-section">
		<h4 class="heading">
			<span class="heading-icon"><Languages size={16} strokeWidth={1.6} /></span>
			{lang && t('language')}
		</h4>
		<select
			value={currentLang}
			on:change={(event) => setLanguage((event.target as HTMLSelectElement).value as 'en' | 'fr')}
		>
			<option value="en">English</option>
			<option value="fr">Français</option>
		</select>
	</div>

	<div class="sidebar-footer">{lang && t('footer_text')}</div>
</aside>

<style>
	.sidebar {
		position: sticky;
		top: 0;
		height: 100vh;
		background: #0f1720;
		border-right: 1px solid #1f2937;
		padding: 20px 16px;
		display: flex;
		flex-direction: column;
		gap: 20px;
	}

	.sidebar-brand {
		text-align: center;
	}

	.title {
		font-size: 20px;
		font-weight: 700;
		color: #7aa2f7;
	}

	.subtitle {
		color: #94a3b8;
		font-size: 13px;
	}

	.sidebar-nav {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.sidebar-nav a {
		padding: 8px 12px;
		border-radius: 8px;
		background: #111827;
		border: 1px solid #1f2937;
		font-size: 13px;
	}

	.nav-item {
		display: flex;
		align-items: center;
		gap: 8px;
		color: inherit;
		text-decoration: none;
	}

	.nav-icon {
		display: inline-flex;
		align-items: center;
		color: #94a3b8;
	}

	.sidebar-section h4 {
		margin: 0 0 8px;
		font-size: 13px;
		color: #94a3b8;
		text-transform: uppercase;
		letter-spacing: 0.06em;
	}

	.sidebar-footer {
		margin-top: auto;
		font-size: 12px;
		color: #94a3b8;
		text-align: center;
	}

	select {
		width: 100%;
		background: #0b1320;
		border: 1px solid #1f2937;
		border-radius: 8px;
		padding: 6px 10px;
	}
</style>
