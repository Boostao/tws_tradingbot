<script lang="ts">
	import { t } from '$lib/i18n';
	import { GetTWSConnection, UpdateTWSConnection } from '../../../wailsjs/go/main/App';
    import { onMount } from 'svelte';

    let isExpanded = true;
    
    let twsHost = '127.0.0.1';
    let twsPort = 7497;
    let twsClient = 1;

    onMount(async () => {
        const config = await GetTWSConnection();
        twsHost = config.host;
        twsPort = config.port;
        twsClient = config.client_id;
    });

    async function saveTwsConnection() {
        try {
            await UpdateTWSConnection(twsHost, parseInt(twsPort, 10), parseInt(twsClient, 10));
            alert("TWS Connection Updated & Reconnected!");
        } catch(e) {
            alert("Failed: " + e);
        }
    }
</script>

<aside class="sidebar" class:expanded={isExpanded}>
	<div class="sidebar-header">
		<h2>TWS Config</h2>
		<button class="toggle-btn" on:click={() => isExpanded = !isExpanded}>☰</button>
	</div>

    {#if isExpanded}
	<div class="settings-block">
        <label>Gateway Host</label>
        <input type="text" bind:value={twsHost} />
        
        <label>Port (Live: 7496, Paper: 7497)</label>
        <input type="number" bind:value={twsPort} />
        
        <label>Client ID</label>
        <input type="number" bind:value={twsClient} />
        
        <button on:click={saveTwsConnection}>Connect to TWS</button>
	</div>
    {/if}
</aside>

<style>
.sidebar {
    width: 250px;
    background: #1e1e1e;
    border-right: 1px solid #333;
    display: flex;
    flex-direction: column;
    padding: 10px;
    transition: width 0.2s;
    height: 100vh;
}
.sidebar:not(.expanded) {
    width: 50px;
}
.sidebar-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    border-bottom: 1px solid #444;
    padding-bottom: 10px;
    margin-bottom: 15px;
}
.sidebar:not(.expanded) h2 {
    display: none;
}
.toggle-btn {
    background: none;
    border: none;
    color: white;
    cursor: pointer;
    font-size: 1.5rem;
}
label {
    display: block;
    margin-top: 10px;
    font-size: 0.85rem;
    color: #aaa;
}
input {
    width: 100%;
    padding: 5px;
    margin-top: 5px;
    background: #2a2a2a;
    border: 1px solid #444;
    color: white;
    border-radius: 4px;
}
button {
    width: 100%;
    margin-top: 15px;
    padding: 8px;
    background: #2196F3;
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
}
button:hover {
    background: #1976D2;
}
</style>
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

	$: botState = $runtimeState ?? emptyBotState;

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
				host: twsHost.trim(),
				port: Number(twsPort),
				client_id: Number(twsClientId),
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

	function applySidebarConfig(config: ConfigResponse): void {
		twsHost = config.ib?.host ?? twsHost;
		twsPort = String(config.ib?.port ?? twsPort);
		twsClientId = String(config.ib?.client_id ?? twsClientId);
		twsAccount = config.ib?.account ?? twsAccount;
		tradingMode = config.ib?.trading_mode ?? tradingMode;
		fixedNotional = String(config.runtime?.fixed_notional ?? fixedNotional);
		bracketEnabled = Boolean(config.runtime?.bracket_enabled ?? bracketEnabled);
		stopLossPct = String(config.runtime?.stop_loss_pct ?? stopLossPct);
		takeProfitPct = String(config.runtime?.take_profit_pct ?? takeProfitPct);
		lastPersistedConfig = sidebarConfigSnapshot();
	}

	function clearConfigPersistTimer(): void {
		if (configPersistTimer !== null) {
			clearTimeout(configPersistTimer);
			configPersistTimer = null;
		}
	}

	async function loadSidebarConfig(force = false) {
		try {
			const config = await getConfig(force);
			applySidebarConfig(config);
		} catch {
			return;
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
			const config = await updateConfig(currentSidebarConfig());
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
			if (botState.tws_connected) await disconnectTws();
			else await connectTws({ host: currentSidebarConfig().ib.host, port: currentSidebarConfig().ib.port, client_id: currentSidebarConfig().ib.client_id });
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
		void loadSidebarConfig(true);

		return () => {
			clearConfigPersistTimer();
		};
	});
</script>

<aside class="sidebar">
	<a class="sidebar-brand" href="/cockpit">
		<div class="title">{lang && t('cobalt_title')}</div>
		<div class="subtitle">{lang && t('rule_based_trading_bot')}</div>
	</a>

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
			<span class="heading-icon"><Plug size={16} strokeWidth={1.6} /></span>
			{lang && t('connection')}
		</h4>
		<div class="status-row">
			<span>{lang && t('tws')}</span>
			<span class:online={botState.tws_connected} class:offline={!botState.tws_connected}>
				{botState.tws_connected ? (lang && t('connected')) : (lang && t('disconnected'))}
			</span>
		</div>
		<div class="connection-form">
			<label>
				<span>{lang && t('tws_host')}</span>
				<input type="text" bind:value={twsHost} on:blur={() => scheduleSidebarConfigPersist()} />
			</label>
			<label>
				<span>{lang && t('tws_port')}</span>
				<input type="number" min="1" bind:value={twsPort} on:blur={() => scheduleSidebarConfigPersist()} />
			</label>
			<label>
				<span>{lang && t('client_id')}</span>
				<input type="number" min="1" bind:value={twsClientId} on:blur={() => scheduleSidebarConfigPersist()} />
			</label>
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
			<label>
				<span>{lang && t('fixed_notional')}</span>
				<input type="number" min="0" step="100" bind:value={fixedNotional} on:blur={() => scheduleSidebarConfigPersist()} />
			</label>
			<label class="checkbox-row">
				<input type="checkbox" bind:checked={bracketEnabled} on:change={() => scheduleSidebarConfigPersist()} />
				<span>{lang && t('bracket_enabled')}</span>
			</label>
			<label>
				<span>{lang && t('stop_loss_pct')}</span>
				<input type="number" min="0" step="0.1" bind:value={stopLossPct} on:blur={() => scheduleSidebarConfigPersist()} disabled={!bracketEnabled} />
			</label>
			<label>
				<span>{lang && t('take_profit_pct')}</span>
				<input type="number" min="0" step="0.1" bind:value={takeProfitPct} on:blur={() => scheduleSidebarConfigPersist()} disabled={!bracketEnabled} />
			</label>
			<button on:click={handleTwsToggle} disabled={twsLoading} class:primary={!botState.tws_connected}>
				{#if twsLoading}
					{botState.tws_connected ? (lang && t('disconnecting_tws')) : (lang && t('connecting_tws'))}
				{:else}
					{botState.tws_connected ? (lang && t('disconnect')) : (lang && t('connect'))}
				{/if}
			</button>
		</div>
		<div class="status-row">
			<span>{lang && t('status_bot')}</span>
			<span class:online={botState.status === 'RUNNING'} class:offline={botState.status !== 'RUNNING'}>
				{botState.status === 'RUNNING' ? (lang && t('running')) : (lang && t('stopped'))}
			</span>
		</div>
		<button class="secondary bot-toggle" on:click={handleBotToggle} disabled={botLoading || !botState.tws_connected}>
			<span class="nav-icon"><Power size={16} strokeWidth={1.6} /></span>
			{#if botLoading}
				{botState.status === 'RUNNING' || botState.status === 'STARTING' ? (lang && t('stopping')) : (lang && t('starting'))}
			{:else}
				{botState.status === 'RUNNING' || botState.status === 'STARTING' ? (lang && t('stop_bot')) : (lang && t('start_bot'))}
			{/if}
		</button>
		{#if botState.active_strategy}
			<div class="status-row strategy-name">
				<span>{lang && t('active_strategy_header')}</span>
				<strong>{botState.active_strategy}</strong>
			</div>
		{/if}
		{#if sidebarMessage}
			<p class="sidebar-message">{sidebarMessage}</p>
		{/if}
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
		text-decoration: none;
		color: inherit;
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

	.status-row,
	.strategy-name {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 8px;
		font-size: 13px;
		color: #cbd5e1;
		margin-bottom: 8px;
	}

	.connection-form {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}

	.connection-form label,
	.checkbox-row {
		display: flex;
		flex-direction: column;
		gap: 4px;
		font-size: 12px;
		color: #94a3b8;
	}

	.checkbox-row {
		flex-direction: row;
		align-items: center;
	}

	.online {
		color: #4ade80;
	}

	.offline {
		color: #f87171;
	}

	button {
		background: #0b1320;
		border: 1px solid #1f2937;
		border-radius: 8px;
		padding: 8px 10px;
		color: #e2e8f0;
		cursor: pointer;
	}

	button.primary {
		background: #1d4ed8;
		border-color: #1d4ed8;
	}

	button.secondary {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
	}

	button:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.bot-toggle {
		width: 100%;
		margin-top: 4px;
	}

	.sidebar-message {
		margin: 0;
		font-size: 12px;
		color: #fca5a5;
	}

	select {
		width: 100%;
		background: #0b1320;
		border: 1px solid #1f2937;
		border-radius: 8px;
		padding: 6px 10px;
	}

	input {
		width: 100%;
		background: #0b1320;
		border: 1px solid #1f2937;
		border-radius: 8px;
		padding: 6px 10px;
		color: #e2e8f0;
	}
</style>
