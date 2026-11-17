<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import {
		formatApiError,
		getConfig,
		getTwsConnection,
		updateConfig,
		updateTwsConnection,
		connectTws,
		disconnectTws,
		startBot,
		stopBot,
		type BotState,
		type ConfigResponse
	} from '$lib/api';
	import { language, setLanguage, t } from '$lib/i18n';
	import { refreshRuntimeState, runtimeState } from '$lib/stores/runtime';
	import {
		Activity,
		BookOpen,
		ChevronDown,
		ChevronRight,
		Compass,
		GitBranch,
		Languages,
		LayoutGrid,
		List,
		Plug,
		Power,
		ShieldCheck
	} from 'lucide-svelte';

	$: currentLang = $language;
	$: lang = $language;
	$: currentPath = $page.url.pathname;

	type SidebarConfigDraft = {
		ib: {
			host: string;
			port: number;
			client_id: number;
			account: string;
			trading_mode: string;
		};
		runtime: {
			fixed_notional: number;
			bracket_enabled: boolean;
			stop_loss_pct: number;
			take_profit_pct: number;
		};
	};

	const emptyBotState: BotState = {
		status: 'STOPPED',
		tws_connected: false,
		equity: 0,
		daily_pnl: 0,
		daily_pnl_percent: 0,
		total_pnl: 0,
		active_strategy: '',
		open_positions_count: 0,
		pending_orders_count: 0,
		trades_today: 0,
		win_rate_today: 0
	};

	let botState: BotState = emptyBotState;
	let twsHost = '127.0.0.1';
	let twsPort = '7497';
	let twsClientId = '1';
	let twsAccount = '';
	let tradingMode = 'paper';
	let fixedNotional = '10000';
	let bracketEnabled = false;
	let stopLossPct = '2';
	let takeProfitPct = '4';
	let sidebarMessage = '';
	let twsLoading = false;
	let botLoading = false;
	let configPersistTimer: ReturnType<typeof setTimeout> | null = null;
	let configPersistInFlight = false;
	let configPersistQueued = false;
	let lastPersistedConfig = '';
	let lastIbEndpoint = '127.0.0.1:7497:1';

	let brokerCollapsed = false;
	let riskCollapsed = false;
	let prevTwsConnected: boolean | null = null;

	$: botState = $runtimeState ?? emptyBotState;
	$: if (botState?.account_id && botState.account_id !== twsAccount) {
		twsAccount = botState.account_id;
	}
	$: {
		const isConnected = Boolean(botState?.tws_connected);
		if (prevTwsConnected === null) {
			brokerCollapsed = isConnected;
			riskCollapsed = isConnected;
			prevTwsConnected = isConnected;
		} else if (isConnected !== prevTwsConnected) {
			brokerCollapsed = isConnected;
			riskCollapsed = isConnected;
			prevTwsConnected = isConnected;
		}
	}

	$: effectiveAccount = botState?.account_id || twsAccount || '—';

	$: navItems = [
		{ href: '/cockpit', label: () => (lang && t('cockpit')) as string, icon: LayoutGrid },
		{ href: '/monitoring', label: () => (lang && t('monitoring')) as string, icon: Activity },
		{ href: '/watchlist', label: () => (lang && t('watchlist')) as string, icon: List },
		{ href: '/strategy', label: () => (lang && t('strategy_builder')) as string, icon: GitBranch },
		{ href: '/manual', label: () => (lang && t('user_manual')) as string || 'User Manual', icon: BookOpen }
	];

	function currentSidebarConfig(): SidebarConfigDraft {
		return {
			ib: {
				host: twsHost.trim() || '127.0.0.1',
				port: Number(twsPort) || 7497,
				client_id: Number(twsClientId) || 1,
				account: twsAccount.trim(),
				trading_mode: tradingMode
			},
			runtime: {
				fixed_notional: Number(fixedNotional),
				bracket_enabled: bracketEnabled,
				stop_loss_pct: Number(stopLossPct),
				take_profit_pct: Number(takeProfitPct)
			}
		};
	}

	function sidebarConfigSnapshot(): string {
		return JSON.stringify(currentSidebarConfig());
	}

	function applySidebarConfig(
		config: ConfigResponse,
		twsConn?: { host: string; port: number; client_id: number } | null
	): void {
		twsHost = config.ib?.host ?? twsConn?.host ?? twsHost;
		twsPort = String(config.ib?.port ?? twsConn?.port ?? twsPort);
		twsClientId = String(config.ib?.client_id ?? twsConn?.client_id ?? twsClientId);
		twsAccount = botState?.account_id || config.ib?.account || twsAccount;
		tradingMode = config.ib?.trading_mode ?? tradingMode;
		fixedNotional = String(config.runtime?.fixed_notional ?? fixedNotional);
		bracketEnabled = Boolean(config.runtime?.bracket_enabled ?? bracketEnabled);
		stopLossPct = String(config.runtime?.stop_loss_pct ?? stopLossPct);
		takeProfitPct = String(config.runtime?.take_profit_pct ?? takeProfitPct);
		lastPersistedConfig = sidebarConfigSnapshot();
		lastIbEndpoint = `${twsHost.trim()}:${Number(twsPort) || 7497}:${Number(twsClientId) || 1}`;
	}

	function clearConfigPersistTimer(): void {
		if (configPersistTimer !== null) {
			clearTimeout(configPersistTimer);
			configPersistTimer = null;
		}
	}

	async function loadSidebarConfig(force = false) {
		try {
			const [config, twsConn] = await Promise.all([
				getConfig(force),
				getTwsConnection().catch(() => null)
			]);
			applySidebarConfig(config, twsConn);
			sidebarMessage = '';
		} catch (err) {
			sidebarMessage = formatApiError(err);
		}
	}

	function scheduleSidebarConfigPersist(delayMs = 700): void {
		clearConfigPersistTimer();
		configPersistTimer = setTimeout(() => {
			configPersistTimer = null;
			void persistSidebarConfig();
		}, delayMs);
	}

	async function persistSidebarConfig(force = false) {
		clearConfigPersistTimer();
		const snapshot = sidebarConfigSnapshot();
		if (!force && snapshot === lastPersistedConfig) {
			return;
		}
		if (configPersistInFlight) {
			configPersistQueued = true;
			return;
		}

		configPersistInFlight = true;
		try {
			const nextCfg = currentSidebarConfig();
			const nextIbEndpoint = `${nextCfg.ib.host}:${nextCfg.ib.port}:${nextCfg.ib.client_id}`;
			const config = await updateConfig(nextCfg);
			if (botState.tws_connected && nextIbEndpoint !== lastIbEndpoint) {
				await updateTwsConnection(nextCfg.ib.host, nextCfg.ib.port, nextCfg.ib.client_id);
			}
			applySidebarConfig(config);
			sidebarMessage = '';
		} catch (err) {
			sidebarMessage = formatApiError(err);
		} finally {
			configPersistInFlight = false;
			if (configPersistQueued) {
				configPersistQueued = false;
				void persistSidebarConfig();
			}
		}
	}

	async function flushSidebarConfigPersist() {
		clearConfigPersistTimer();
		await persistSidebarConfig();
	}

	async function handleTwsToggle() {
		twsLoading = true;
		sidebarMessage = '';
		try {
			await flushSidebarConfigPersist();
			if (botState.tws_connected) {
				await disconnectTws();
			} else {
				const ib = currentSidebarConfig().ib;
				const res = await connectTws({ host: ib.host, port: ib.port, client_id: ib.client_id });
				if (res.status === 'error') {
					sidebarMessage = res.message || t('failed_to_connect_tws');
				}
			}
			await refreshRuntimeState(true);
			await loadSidebarConfig(true);
		} catch (err) {
			sidebarMessage = formatApiError(err);
		} finally {
			twsLoading = false;
		}
	}

	async function handleBotToggle() {
		botLoading = true;
		sidebarMessage = '';
		try {
			await flushSidebarConfigPersist();
			if (botState.status === 'RUNNING' || botState.status === 'STARTING') await stopBot();
			else await startBot();
			await refreshRuntimeState(true);
		} catch (err) {
			sidebarMessage = formatApiError(err);
		} finally {
			botLoading = false;
		}
	}

	onMount(() => {
		void loadSidebarConfig();

		return () => {
			clearConfigPersistTimer();
		};
	});
</script>

<aside class="sidebar">
	<a class="sidebar-brand" href="/cockpit">
		<svg class="sidebar-emblem" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100" width="36" height="36" aria-hidden="true">
			<title>Cobalt</title>
			<defs>
				<linearGradient id="sidebar-cobalt-bg" x1="0%" y1="0%" x2="100%" y2="100%">
					<stop offset="0%" stop-color="#1D367A" />
					<stop offset="55%" stop-color="#0e0c28" />
					<stop offset="100%" stop-color="#05070B" />
				</linearGradient>
				<linearGradient id="sidebar-cobalt-wing" x1="0%" y1="0%" x2="0%" y2="100%">
					<stop offset="0%" stop-color="#38BDF8" />
					<stop offset="100%" stop-color="#1D63ED" />
				</linearGradient>
			</defs>
			<rect x="2" y="2" width="96" height="96" rx="22" fill="url(#sidebar-cobalt-bg)" stroke="#263c8a" stroke-width="3" />
			<g fill="url(#sidebar-cobalt-wing)">
				<path d="M 14,36 L 26,20 L 31,37 L 20,46 Z" />
				<path d="M 86,36 L 74,20 L 69,37 L 80,46 Z" />
				<path d="M 10,52 L 24,50 L 26,61 L 14,64 Z" />
				<path d="M 90,52 L 76,50 L 74,61 L 86,64 Z" />
				<path d="M 20,73 L 31,67 L 36,78 L 26,82 Z" />
				<path d="M 80,73 L 69,67 L 64,78 L 74,82 Z" />
				<path d="M 50,80 L 59,75 L 50,90 L 41,75 Z" />
			</g>
			<path d="M 65.7,36.8 A 20.5,20.5 0 1,0 65.7,63.2 L 57.7,56.4 A 10,10 0 1,1 57.7,43.6 Z" fill="#ffffff" />
			<polygon points="50,11 55,19.5 50,25 45,19.5" fill="#faa031" />
		</svg>
		<div class="sidebar-brand-text">
			<div class="title">{lang && t('cobalt_title')}</div>
			<div class="subtitle">{lang && t('rule_based_trading_bot')}</div>
		</div>
	</a>

	<div class="sidebar-section">
		<h4 class="heading">
			<span class="heading-icon"><Compass size={15} strokeWidth={1.6} /></span>
			{lang && t('navigation')}
		</h4>
		<nav class="sidebar-nav">
			{#each navItems as item}
				{@const isActive = currentPath === item.href || currentPath.startsWith(item.href + '/')}
				<a
					href={item.href}
					class="nav-item"
					class:active={isActive}
					aria-current={isActive ? 'page' : undefined}
				>
					<span class="nav-icon">
						<svelte:component this={item.icon} size={16} strokeWidth={1.6} />
					</span>
					<span>{item.label()}</span>
				</a>
			{/each}
		</nav>
	</div>

	<div class="sidebar-section sidebar-status-panel">
		<div class="status-row sidebar-status-badge" data-testid="status-badge">
			<span class="status-badge-tws">
				<span class="status-dot" class:offline={!botState.tws_connected}></span>
				<span class:online={botState.tws_connected} class:offline={!botState.tws_connected}>
					{botState.tws_connected ? (lang && t('tws_connected_status')) : (lang && t('tws_disconnected'))}
				</span>
			</span>
			<span
				class="bot-state-pill"
				class:is-running={botState.status === 'RUNNING'}
				class:is-armed={botState.status === 'ARMED'}
			>
				{botState.status === 'RUNNING'
					? (lang && t('running'))
					: botState.status === 'ARMED'
						? (lang && t('sidebar_status_armed'))
						: (lang && t('stopped'))}
			</span>
		</div>

		<button
			type="button"
			class="secondary bot-toggle"
			on:click={handleBotToggle}
			disabled={botLoading || !botState.tws_connected}
		>
			<span class="nav-icon"><Power size={15} strokeWidth={1.6} /></span>
			{#if botLoading}
				{botState.status === 'RUNNING' || botState.status === 'STARTING' ? (lang && t('stopping')) : (lang && t('starting'))}
			{:else}
				{botState.status === 'RUNNING' || botState.status === 'STARTING' ? (lang && t('stop_bot')) : (lang && t('start_bot'))}
			{/if}
		</button>

		{#if botState.active_strategy}
			<div class="sidebar-strategy-chip">
				<span class="muted">{lang && t('active_strategy_header')}</span>
				<strong>{botState.active_strategy}</strong>
			</div>
		{/if}
	</div>

	<!-- Collapsible Section 1: Broker Connection -->
	<div class="sidebar-section sidebar-collapsible">
		<button
			type="button"
			class="sidebar-collapse-btn"
			aria-expanded={!brokerCollapsed}
			on:click={() => (brokerCollapsed = !brokerCollapsed)}
		>
			<span class="collapse-btn-left">
				<span class="heading-icon"><Plug size={14} strokeWidth={1.7} /></span>
				<span>{lang && t('sidebar_broker_section')}</span>
			</span>
			<span class="collapse-btn-chevron" aria-hidden="true">
				{#if brokerCollapsed}
					<ChevronRight size={14} strokeWidth={2} />
				{:else}
					<ChevronDown size={14} strokeWidth={2} />
				{/if}
			</span>
		</button>

		{#if brokerCollapsed}
			<div class="sidebar-summary-chip font-mono" title={`${twsHost}:${twsPort} · ${effectiveAccount}`}>
				{twsHost}:{twsPort} · {effectiveAccount}
			</div>
		{:else}
			<div class="connection-form">
				<label>
					<span>{lang && t('tws_host')}</span>
					<input type="text" bind:value={twsHost} on:blur={() => scheduleSidebarConfigPersist()} />
				</label>
				<div class="sidebar-two-col">
					<label>
						<span>{lang && t('tws_port')}</span>
						<input type="number" min="1" bind:value={twsPort} on:blur={() => scheduleSidebarConfigPersist()} />
					</label>
					<label>
						<span>{lang && t('client_id')}</span>
						<input type="number" min="1" bind:value={twsClientId} on:blur={() => scheduleSidebarConfigPersist()} />
					</label>
				</div>
				<label>
					<span>{lang && t('account_id')}</span>
					<input type="text" bind:value={twsAccount} on:blur={() => scheduleSidebarConfigPersist()} />
				</label>
				<label>
					<span>{lang && t('trading_mode')}</span>
					<select bind:value={tradingMode} on:change={() => scheduleSidebarConfigPersist()}>
						<option value="paper">{lang && t('paper')}</option>
						<option value="live">{lang && t('live')}</option>
					</select>
				</label>
				<button type="button" on:click={handleTwsToggle} disabled={twsLoading} class:primary={!botState.tws_connected}>
					{#if twsLoading}
						{botState.tws_connected ? (lang && t('disconnecting_tws')) : (lang && t('connecting_tws'))}
					{:else}
						{botState.tws_connected ? (lang && t('disconnect')) : (lang && t('connect'))}
					{/if}
				</button>
			</div>
		{/if}
	</div>

	<!-- Collapsible Section 2: Portfolio & Risk Management -->
	<div class="sidebar-section sidebar-collapsible">
		<button
			type="button"
			class="sidebar-collapse-btn"
			aria-expanded={!riskCollapsed}
			on:click={() => (riskCollapsed = !riskCollapsed)}
		>
			<span class="collapse-btn-left">
				<span class="heading-icon"><ShieldCheck size={14} strokeWidth={1.7} /></span>
				<span>{lang && t('sidebar_risk_section')}</span>
			</span>
			<span class="collapse-btn-chevron" aria-hidden="true">
				{#if riskCollapsed}
					<ChevronRight size={14} strokeWidth={2} />
				{:else}
					<ChevronDown size={14} strokeWidth={2} />
				{/if}
			</span>
		</button>

		{#if riskCollapsed}
			<div class="sidebar-summary-chip font-mono">
				${fixedNotional} · {bracketEnabled ? `${stopLossPct}% / ${takeProfitPct}%` : (lang && t('sidebar_bracket_off'))}
			</div>
		{:else}
			<div class="connection-form">
				<label>
					<span>{lang && t('fixed_notional')}</span>
					<input type="number" min="0" step="100" bind:value={fixedNotional} on:blur={() => scheduleSidebarConfigPersist()} />
				</label>
				<label class="checkbox-row">
					<input type="checkbox" bind:checked={bracketEnabled} on:change={() => scheduleSidebarConfigPersist()} />
					<span>{lang && t('bracket_enabled')}</span>
				</label>
				<div class="sidebar-two-col">
					<label>
						<span>{lang && t('stop_loss_pct')}</span>
						<input type="number" min="0" step="0.1" bind:value={stopLossPct} on:blur={() => scheduleSidebarConfigPersist()} disabled={!bracketEnabled} />
					</label>
					<label>
						<span>{lang && t('take_profit_pct')}</span>
						<input type="number" min="0" step="0.1" bind:value={takeProfitPct} on:blur={() => scheduleSidebarConfigPersist()} disabled={!bracketEnabled} />
					</label>
				</div>
			</div>
		{/if}
	</div>

	{#if sidebarMessage}
		<p class="sidebar-message">{sidebarMessage}</p>
	{/if}

	<div class="sidebar-section">
		<h4 class="heading">
			<span class="heading-icon"><Languages size={15} strokeWidth={1.6} /></span>
			{lang && t('language')}
		</h4>
		<select
			aria-label={lang && t('language')}
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
		height: 100dvh;
		overflow-y: auto;
		background: linear-gradient(180deg, var(--cobalt-navy, #0e0c28) 0%, var(--bg-surface, #0b111e) 100%);
		border-right: 1px solid var(--border-subtle, #1f2937);
		padding: 16px 14px;
		display: flex;
		flex-direction: column;
		gap: 14px;
	}

	.sidebar-brand {
		display: flex;
		align-items: center;
		gap: 10px;
		text-decoration: none;
		color: inherit;
		padding-bottom: 12px;
		border-bottom: 1px solid var(--border-subtle, #1f2937);
	}

	.sidebar-emblem {
		flex-shrink: 0;
		display: block;
	}

	.sidebar-brand-text {
		min-width: 0;
	}

	.title {
		font-size: 18px;
		font-weight: 800;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		color: var(--text-primary, #e6edf3);
		line-height: 1.1;
	}

	.subtitle {
		color: var(--cobalt-slate, #7a80b4);
		font-size: 11px;
		margin-top: 2px;
	}

	.sidebar-nav {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.sidebar-nav a {
		padding: 8px 10px;
		border-radius: 8px;
		background: rgba(15, 23, 38, 0.72);
		border: 1px solid var(--border-subtle, #1f2937);
		font-size: 13px;
		font-weight: 600;
		transition: border-color 0.15s ease, background-color 0.15s ease;
	}

	.sidebar-nav a:hover {
		border-color: var(--border-highlight, #2f3b52);
		background: var(--bg-card-elevated, #141e30);
	}

	.sidebar-nav a.active {
		border-color: var(--cobalt-electric, #1d63ed);
		background: linear-gradient(90deg, rgba(29, 99, 237, 0.24), rgba(38, 60, 138, 0.16));
		color: var(--text-primary, #e6edf3);
	}

	.sidebar-nav a.active .nav-icon {
		color: var(--cobalt-cyan, #38bdf8);
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
		color: var(--text-secondary, #94a3b8);
	}

	.sidebar-section h4 {
		margin: 0 0 8px;
		font-size: 11px;
		color: var(--text-secondary, #94a3b8);
		text-transform: uppercase;
		letter-spacing: 0.08em;
	}

	.sidebar-status-panel {
		display: grid;
		grid-template-columns: minmax(0, 1fr);
		min-width: 0;
		gap: 8px;
		padding: 10px;
		border-radius: 10px;
		background: var(--bg-card, #0f1726);
		border: 1px solid var(--border-subtle, #1f2937);
	}

	.status-row {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 8px;
		min-width: 0;
		overflow: hidden;
		font-size: 12px;
		color: var(--text-primary, #e6edf3);
	}

	.status-badge-tws {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-weight: 700;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.bot-state-pill {
		flex-shrink: 0;
		font-size: 10px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		padding: 2px 7px;
		border-radius: 999px;
		background: rgba(100, 116, 139, 0.2);
		color: var(--text-secondary, #94a3b8);
		border: 1px solid var(--border-subtle, #1f2937);
	}

	.bot-state-pill.is-running {
		background: rgba(16, 185, 129, 0.16);
		color: var(--sem-bull-text, #4caf50);
		border-color: rgba(16, 185, 129, 0.45);
	}

	.bot-state-pill.is-armed {
		background: rgba(56, 189, 248, 0.14);
		color: var(--cobalt-cyan, #38bdf8);
		border-color: rgba(56, 189, 248, 0.4);
	}

	.sidebar-strategy-chip {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 8px;
		min-width: 0;
		overflow: hidden;
		font-size: 11px;
		padding-top: 6px;
		border-top: 1px solid var(--border-subtle, #1f2937);
	}

	.sidebar-strategy-chip span {
		flex-shrink: 0;
	}

	.sidebar-strategy-chip strong {
		color: var(--text-primary, #e6edf3);
		min-width: 0;
		flex: 1 1 auto;
		text-align: right;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.sidebar-collapsible {
		display: grid;
		gap: 8px;
		padding: 8px 10px;
		border-radius: 10px;
		background: var(--bg-card, #0f1726);
		border: 1px solid var(--border-subtle, #1f2937);
	}

	.sidebar-collapse-btn {
		width: 100%;
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		padding: 2px 0;
		background: transparent;
		border: none;
		color: var(--text-secondary, #94a3b8);
		font-size: 11px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		cursor: pointer;
		text-align: left;
	}

	.sidebar-collapse-btn:hover {
		color: var(--text-primary, #e6edf3);
	}

	.collapse-btn-left {
		display: inline-flex;
		align-items: center;
		gap: 6px;
	}

	.collapse-btn-chevron {
		display: inline-flex;
		align-items: center;
		color: var(--text-muted, #64748b);
	}

	.sidebar-summary-chip {
		font-size: 11px;
		color: var(--cobalt-link, #8ab4f8);
		background: var(--bg-void, #05070b);
		border: 1px solid var(--border-subtle, #1f2937);
		border-radius: 6px;
		padding: 5px 8px;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.connection-form {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.sidebar-two-col {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 8px;
	}

	.connection-form label,
	.checkbox-row {
		display: flex;
		flex-direction: column;
		gap: 4px;
		font-size: 11px;
		color: var(--text-secondary, #94a3b8);
	}

	.checkbox-row {
		flex-direction: row;
		align-items: center;
		gap: 8px;
	}

	.checkbox-row input[type='checkbox'] {
		width: auto;
	}

	.online {
		color: var(--sem-bull-text, #4caf50);
	}

	.offline {
		color: var(--sem-bear-text, #ef5350);
	}

	.connection-form button,
	.bot-toggle {
		background: var(--bg-input, #0b1320);
		border: 1px solid var(--border-subtle, #1f2937);
		border-radius: 8px;
		padding: 7px 10px;
		color: var(--text-primary, #e6edf3);
		cursor: pointer;
		font-size: 12px;
	}

	.connection-form button.primary {
		background: var(--cobalt-electric, #1d63ed);
		border-color: var(--cobalt-electric, #1d63ed);
	}

	button.secondary {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 6px;
	}

	button:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.bot-toggle {
		width: 100%;
	}

	.sidebar-message {
		margin: 0;
		font-size: 12px;
		color: #fca5a5;
	}

	select {
		width: 100%;
	}

	input {
		width: 100%;
		background-color: var(--bg-input, #0b1320);
		border: 1px solid var(--border-subtle, #1f2937);
		border-radius: 8px;
		padding: 6px 8px;
		color: var(--text-primary, #e6edf3);
		font-size: 13px;
	}

	.sidebar-footer {
		margin-top: auto;
		font-size: 11px;
		color: var(--text-muted, #64748b);
		text-align: center;
		padding-top: 8px;
	}
</style>
