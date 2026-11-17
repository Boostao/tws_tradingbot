<script lang="ts">
	import {
		Activity,
		BookOpen,
		Compass,
		ExternalLink,
		GitBranch,
		HelpCircle,
		LayoutGrid,
		List,
		Plug
	} from 'lucide-svelte';
	import { language, t } from '$lib/i18n';
	import { launchTour, openHelpDrawer } from '$lib/stores/onboarding';

	$: lang = $language;
</script>

<div class="manual-page">
	<header class="manual-hero card">
		<div class="manual-hero-top">
			<div class="manual-hero-copy">
				<span class="manual-kicker font-mono">COBALT V2 · {lang && t('manual_hero_kicker')}</span>
				<h1 class="heading">
					<span class="heading-icon"><BookOpen size={20} strokeWidth={1.6} /></span>
					{lang && t('user_manual')}
				</h1>
				<p class="manual-hero-subtitle">{lang && t('manual_hero_subtitle')}</p>
			</div>
			<button type="button" class="secondary manual-help-trigger" on:click={openHelpDrawer}>
				<HelpCircle size={15} strokeWidth={1.8} />
				<span>{lang && t('manual_quick_help_btn')}</span>
			</button>
		</div>

		<div class="manual-tour-grid">
			<div class="manual-tour-card is-primary">
				<div class="manual-tour-meta">
					<span class="manual-tour-badge font-mono">{lang && t('tour_track1_duration')}</span>
					<h2>{lang && t('manual_track1_title')}</h2>
					<p>{lang && t('manual_track1_desc')}</p>
				</div>
				<button type="button" class="manual-tour-btn" on:click={() => launchTour(1)}>
					<Compass size={14} strokeWidth={1.8} />
					<span>{lang && t('manual_launch_track1')}</span>
				</button>
			</div>

			<div class="manual-tour-card">
				<div class="manual-tour-meta">
					<span class="manual-tour-badge font-mono">{lang && t('tour_track2_duration')}</span>
					<h2>{lang && t('manual_track2_title')}</h2>
					<p>{lang && t('manual_track2_desc')}</p>
				</div>
				<button type="button" class="secondary manual-tour-btn" on:click={() => launchTour(2)}>
					<Compass size={14} strokeWidth={1.8} />
					<span>{lang && t('manual_launch_track2')}</span>
				</button>
			</div>

			<div class="manual-tour-card">
				<div class="manual-tour-meta">
					<span class="manual-tour-badge font-mono">{lang && t('tour_track3_duration')}</span>
					<h2>{lang && t('manual_track3_title')}</h2>
					<p>{lang && t('manual_track3_desc')}</p>
				</div>
				<button type="button" class="secondary manual-tour-btn" on:click={() => launchTour(3)}>
					<Compass size={14} strokeWidth={1.8} />
					<span>{lang && t('manual_launch_track3')}</span>
				</button>
			</div>
		</div>
	</header>

	<div class="manual-sections">
		<!-- 1. TWS Connection & Account Telemetry -->
		<section class="card manual-card">
			<div class="manual-card-header">
				<span class="manual-card-icon"><Plug size={18} strokeWidth={1.7} /></span>
				<h2>{lang && t('manual_tws_config_title')}</h2>
			</div>
			<p class="manual-lead">{lang && t('manual_tws_config_p1')}</p>
			<ul class="manual-list">
				<li>
					<strong class="font-mono">{lang && t('manual_tws_config_host')}</strong>
					<span>{lang && t('manual_tws_config_host_desc')}</span>
				</li>
				<li>
					<strong class="font-mono">{lang && t('manual_tws_config_port')}</strong>
					<span>{lang && t('manual_tws_config_port_desc')}</span>
				</li>
				<li>
					<strong class="font-mono">{lang && t('manual_tws_config_client_id')}</strong>
					<span>{lang && t('manual_tws_config_client_id_desc')}</span>
				</li>
				<li>
					<strong class="font-mono">{lang && t('manual_tws_config_trading_mode')}</strong>
					<span>{lang && t('manual_tws_config_trading_mode_desc')}</span>
				</li>
			</ul>
			<div class="manual-callout">
				<strong>{lang && t('manual_tws_config_settings_title')}</strong>
				<span>{lang && t('manual_tws_config_settings_desc')}</span>
			</div>
			<a
				href="https://www.interactivebrokers.com/campus/ibkr-api-page/twsapi-doc/#tws-config-api"
				target="_blank"
				rel="noopener noreferrer"
				class="manual-ext-link"
			>
				<span>{lang && t('manual_tws_config_read_docs')}</span>
				<ExternalLink size={14} />
			</a>
		</section>

		<!-- 2. Cockpit Execution & 4-Level Safety Gate -->
		<section class="card manual-card">
			<div class="manual-card-header">
				<span class="manual-card-icon"><LayoutGrid size={18} strokeWidth={1.7} /></span>
				<h2>{lang && t('manual_cockpit_title')}</h2>
			</div>
			<p class="manual-lead">{lang && t('manual_cockpit_p1')}</p>
			<ul class="manual-list">
				<li>
					<strong>{lang && t('manual_cockpit_hud')}</strong>
					<span>{lang && t('manual_cockpit_hud_desc')}</span>
				</li>
				<li>
					<strong>{lang && t('manual_cockpit_workspaces')}</strong>
					<span>{lang && t('manual_cockpit_workspaces_desc')}</span>
				</li>
				<li>
					<strong>{lang && t('manual_cockpit_stack')}</strong>
					<span>{lang && t('manual_cockpit_stack_desc')}</span>
				</li>
				<li>
					<strong>{lang && t('manual_cockpit_tickers')}</strong>
					<span>{lang && t('manual_cockpit_tickers_desc')}</span>
				</li>
				<li>
					<strong>{lang && t('manual_cockpit_global')}</strong>
					<span>{lang && t('manual_cockpit_global_desc')}</span>
				</li>
			</ul>
		</section>

		<!-- 3. Monitoring (4 Trader Panels) & Dry-Run Preflight -->
		<section class="card manual-card">
			<div class="manual-card-header">
				<span class="manual-card-icon"><Activity size={18} strokeWidth={1.7} /></span>
				<h2>{lang && t('manual_monitoring_title')}</h2>
			</div>
			<p class="manual-lead">{lang && t('manual_monitoring_p1')}</p>
			<ul class="manual-list">
				<li>
					<strong>{lang && t('manual_monitoring_panel1')}</strong>
					<span>{lang && t('manual_monitoring_panel1_desc')}</span>
				</li>
				<li>
					<strong>{lang && t('manual_monitoring_panel2')}</strong>
					<span>{lang && t('manual_monitoring_panel2_desc')}</span>
				</li>
				<li>
					<strong>{lang && t('manual_monitoring_panel3')}</strong>
					<span>{lang && t('manual_monitoring_panel3_desc')}</span>
				</li>
				<li>
					<strong>{lang && t('manual_monitoring_panel4')}</strong>
					<span>{lang && t('manual_monitoring_panel4_desc')}</span>
				</li>
			</ul>
		</section>

		<!-- 4. Watchlists, Live $$$$ Holdings & Méthode 8.8 -->
		<section class="card manual-card">
			<div class="manual-card-header">
				<span class="manual-card-icon"><List size={18} strokeWidth={1.7} /></span>
				<h2>{lang && t('manual_watchlist_title')}</h2>
			</div>
			<p class="manual-lead">{lang && t('manual_watchlist_p1')}</p>
			<ul class="manual-list">
				<li>
					<strong>{lang && t('manual_watchlist_holdings')}</strong>
					<span>{lang && t('manual_watchlist_holdings_desc')}</span>
				</li>
				<li>
					<strong>{lang && t('manual_watchlist_feeds')}</strong>
					<span>{lang && t('manual_watchlist_feeds_desc')}</span>
				</li>
				<li>
					<strong>{lang && t('manual_watchlist_tiers')}</strong>
					<span>{lang && t('manual_watchlist_tiers_desc')}</span>
				</li>
				<li>
					<strong>{lang && t('manual_watchlist_suspend')}</strong>
					<span>{lang && t('manual_watchlist_suspend_desc')}</span>
				</li>
			</ul>
		</section>

		<!-- 5. 3-Pane Strategy Workbench & Shortcuts -->
		<section class="card manual-card manual-card-wide">
			<div class="manual-card-header">
				<span class="manual-card-icon"><GitBranch size={18} strokeWidth={1.7} /></span>
				<h2>{lang && t('manual_strategy_title')}</h2>
			</div>
			<p class="manual-lead">{lang && t('manual_strategy_p1')}</p>
			<ul class="manual-list">
				<li>
					<strong>{lang && t('manual_strategy_presets')}</strong>
					<span>{lang && t('manual_strategy_presets_desc')}</span>
				</li>
				<li>
					<strong>{lang && t('manual_strategy_pipeline')}</strong>
					<span>{lang && t('manual_strategy_pipeline_desc')}</span>
				</li>
				<li>
					<strong>{lang && t('manual_strategy_inspector')}</strong>
					<span>{lang && t('manual_strategy_inspector_desc')}</span>
				</li>
				<li>
					<strong>{lang && t('manual_strategy_validation')}</strong>
					<span>{lang && t('manual_strategy_validation_desc')}</span>
				</li>
			</ul>
			<div class="manual-shortcuts-bar">
				<span class="font-mono">{lang && t('manual_shortcuts_line')}</span>
			</div>
		</section>
	</div>
</div>

<style>
	.manual-page {
		display: grid;
		gap: 16px;
		padding-bottom: 20px;
		font-family: var(--font-sans);
	}

	.manual-hero {
		background: linear-gradient(180deg, rgba(38, 60, 138, 0.16), var(--bg-card, #0f1726));
		border: 1px solid var(--cobalt-brand, #263c8a);
		display: grid;
		gap: 14px;
		margin-top: 0;
	}

	.manual-hero-top {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 12px;
		flex-wrap: wrap;
	}

	.manual-hero-copy {
		display: grid;
		gap: 5px;
	}

	.manual-kicker {
		font-size: 11px;
		font-weight: 700;
		letter-spacing: 0.1em;
		text-transform: uppercase;
		color: var(--cobalt-orange, #faa031);
	}

	.manual-hero-copy h1 {
		margin: 0;
		font-size: 21px;
	}

	.manual-hero-subtitle {
		margin: 0;
		font-size: 13px;
		line-height: 1.5;
		color: var(--text-secondary, #94a3b8);
		max-width: 72ch;
	}

	.manual-help-trigger {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		border-color: var(--border-highlight, #2f3b52);
		color: var(--cobalt-cyan, #38bdf8);
		font-size: 13px;
	}

	.manual-tour-grid {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(230px, 1fr));
		gap: 12px;
	}

	.manual-tour-card {
		background: var(--bg-surface, #0b111e);
		border: 1px solid var(--border-subtle, #1f2937);
		border-radius: 10px;
		padding: 13px 14px;
		display: flex;
		flex-direction: column;
		justify-content: space-between;
		gap: 12px;
	}

	.manual-tour-card.is-primary {
		border-color: var(--cobalt-cyan, #38bdf8);
		background: linear-gradient(180deg, rgba(29, 99, 237, 0.14), var(--bg-surface, #0b111e));
	}

	.manual-tour-meta {
		display: grid;
		gap: 5px;
	}

	.manual-tour-badge {
		font-size: 11px;
		font-weight: 700;
		color: var(--cobalt-orange, #faa031);
		text-transform: uppercase;
		letter-spacing: 0.06em;
	}

	.manual-tour-meta h2 {
		margin: 0;
		font-size: 15px;
		font-weight: 700;
		color: var(--text-primary, #e6edf3);
	}

	.manual-tour-meta p {
		margin: 0;
		font-size: 12px;
		line-height: 1.45;
		color: var(--text-secondary, #94a3b8);
	}

	.manual-tour-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 6px;
		font-size: 13px;
		padding: 8px 12px;
	}

	.manual-sections {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(360px, 1fr));
		gap: 14px;
	}

	.manual-card {
		margin-top: 0;
		background: var(--bg-card, #0f1726);
		border: 1px solid var(--border-subtle, #1f2937);
		display: grid;
		gap: 10px;
		align-content: start;
	}

	.manual-card-wide {
		grid-column: 1 / -1;
	}

	.manual-card-header {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.manual-card-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		color: var(--cobalt-cyan, #38bdf8);
	}

	.manual-card-header h2 {
		margin: 0;
		font-size: 16px;
		font-weight: 700;
		color: var(--text-primary, #e6edf3);
	}

	.manual-lead {
		margin: 0;
		font-size: 13px;
		line-height: 1.55;
		color: var(--text-secondary, #94a3b8);
	}

	.manual-list {
		margin: 0;
		padding-left: 18px;
		display: grid;
		gap: 7px;
	}

	.manual-list li {
		font-size: 13px;
		line-height: 1.5;
		color: var(--text-secondary, #94a3b8);
	}

	.manual-list strong {
		color: var(--text-primary, #e6edf3);
		margin-right: 4px;
	}

	.manual-callout {
		padding: 9px 11px;
		border-radius: 8px;
		background: var(--bg-card-elevated, #141e30);
		border: 1px solid var(--border-highlight, #2f3b52);
		font-size: 12px;
		line-height: 1.5;
		color: var(--text-secondary, #94a3b8);
	}

	.manual-callout strong {
		color: var(--cobalt-gold, #edc44e);
		margin-right: 4px;
	}

	.manual-ext-link {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		font-size: 12px;
		font-weight: 600;
		color: var(--cobalt-link, #8ab4f8);
		text-decoration: underline;
		width: fit-content;
	}

	.manual-shortcuts-bar {
		padding: 8px 11px;
		border-radius: 8px;
		background: var(--bg-void, #05070b);
		border: 1px solid var(--border-subtle, #1f2937);
		font-size: 12px;
		color: var(--sem-bull-text, #4caf50);
	}
</style>
