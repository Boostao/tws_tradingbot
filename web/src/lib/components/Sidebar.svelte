<script lang="ts">
	import { onMount } from 'svelte';
	import { botState, refreshBotState } from '$lib/stores/botState';
	import { language, setLanguage, t } from '$lib/i18n';
	import { connectTws, disconnectTws, formatApiError, getConfig, updateConfig } from '$lib/api';
	import {
		Plug,
		ClipboardList,
		BarChart3,
		Languages,
		Compass,
		Activity,
		GitBranch,
		Beaker,
		List,
		Bell,
		BookOpen,
		Settings
	} from 'lucide-svelte';

	$: state = $botState;
	$: currentLang = $language;
	$: lang = $language;

	let twsHost = '';
	let twsPort = '';
	let twsClientId = '';
	let twsError = '';
	let twsLoading = false;

	$: navItems = [
		{ href: '/watchlist', label: () => (lang && t('watchlist')) as string, icon: List },
		{ href: '/strategy', label: () => (lang && t('strategy_builder')) as string, icon: GitBranch },
		{ href: '/backtest', label: () => (lang && t('backtest_configuration')) as string, icon: Beaker },
		{ href: '/monitoring', label: () => (lang && t('monitoring_logging')) as string, icon: Activity },
		{ href: '/notifications', label: () => (lang && t('notifications')) as string, icon: Bell },
		{ href: '/docs/quickstart', label: () => (lang && t('documentation')) as string, icon: BookOpen }
	];

	const formatCurrency = (value: number) =>
		Number.isFinite(value) ? `$${value.toFixed(2)}` : '--';

	onMount(async () => {
		try {
			const config = await getConfig();
			if (config.ib?.host) twsHost = config.ib.host;
			if (typeof config.ib?.port === 'number') twsPort = String(config.ib.port);
			if (typeof config.ib?.client_id === 'number') twsClientId = String(config.ib.client_id);
		} catch (err) {
			console.warn(err);
		}
	});

	const handleToggleConnection = async () => {
		twsError = '';
		twsLoading = true;
		try {
			if (state.tws_connected) {
				await disconnectTws();
			} else {
				const { nextHost, nextPort, nextClientId } = await persistIbConfig();
				await connectTws({
					host: nextHost || undefined,
					port: Number.isFinite(nextPort) ? nextPort : undefined,
					client_id: Number.isFinite(nextClientId) ? nextClientId : undefined
				});
			}
			await refreshBotState();
		} catch (err) {
			twsError = formatApiError(err);
		} finally {
			twsLoading = false;
		}
	};

	const buildIbConfigUpdate = () => {
		const ibUpdates: Record<string, unknown> = {};
		const nextHost = twsHost.trim();
		const nextPort = twsPort ? Number(twsPort) : undefined;
		const nextClientId = twsClientId ? Number(twsClientId) : undefined;
		if (nextHost) ibUpdates.host = nextHost;
		if (Number.isFinite(nextPort)) ibUpdates.port = nextPort;
		if (Number.isFinite(nextClientId)) ibUpdates.client_id = nextClientId;
		return { ibUpdates, nextHost, nextPort, nextClientId };
	};

	const persistIbConfig = async () => {
		const { ibUpdates, nextHost, nextPort, nextClientId } = buildIbConfigUpdate();
		if (Object.keys(ibUpdates).length === 0) {
			return { nextHost, nextPort, nextClientId };
		}
		try {
			await updateConfig({ ib: ibUpdates });
		} catch (err) {
			twsError = formatApiError(err);
		}
		return { nextHost, nextPort, nextClientId };
	};
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
			<span class="heading-icon"><Plug size={16} strokeWidth={1.6} /></span>
			{lang && t('connection')}
		</h4>
		<div class="status-row">
			<span>{lang && t('tws')}</span>
			<span class:online={state.tws_connected} class:offline={!state.tws_connected}>
				{state.tws_connected ? (lang && t('connected')) : (lang && t('disconnected'))}
			</span>
		</div>
		<div class="connection-form">
			<label>
				<span>{lang && t('tws_host')}</span>
				<input
					type="text"
					bind:value={twsHost}
					placeholder="127.0.0.1"
					on:blur={() => void persistIbConfig()}
				/>
			</label>
			<label>
				<span>{lang && t('tws_port')}</span>
				<input
					type="number"
					min="1"
					bind:value={twsPort}
					placeholder="7497"
					on:blur={() => void persistIbConfig()}
				/>
			</label>
			<label>
				<span>{lang && t('client_id')}</span>
				<input
					type="number"
					min="0"
					bind:value={twsClientId}
					placeholder="1"
					on:blur={() => void persistIbConfig()}
				/>
			</label>
			<div class="connection-actions">
				<button
					on:click={handleToggleConnection}
					disabled={twsLoading}
					class:primary={!state.tws_connected}
				>
					{twsLoading
						? (lang && t('connecting_tws'))
						: state.tws_connected
							? (lang && t('disconnect'))
							: (lang && t('connect'))}
				</button>
			</div>
			{#if twsError}
				<p class="connection-error">{twsError}</p>
			{/if}
		</div>
		<div class="status-row">
			<span>{lang && t('bot')}</span>
			<span class:online={state.status === 'RUNNING'} class:offline={state.status !== 'RUNNING'}>
				{state.status === 'RUNNING' ? (lang && t('running')) : (lang && t('stopped'))}
			</span>
		</div>
	</div>

	<div class="sidebar-section">
		<h4 class="heading">
			<span class="heading-icon"><ClipboardList size={16} strokeWidth={1.6} /></span>
			{lang && t('active_strategy_header')}
		</h4>
		{#if state.active_strategy}
			<div class="status-row strategy-name">
				<em>{state.active_strategy}</em>
			</div>
		{:else}
			<div class="status-row">
				<span>{lang && t('name')}</span>
				<span>{lang && t('none')}</span>
			</div>
		{/if}
		<div class="status-row">
			<span>{lang && t('open_positions')}</span>
			<span>{state.open_positions_count ?? 0}</span>
		</div>
		<div class="status-row">
			<span>{lang && t('pending_orders')}</span>
			<span>{state.pending_orders_count ?? 0}</span>
		</div>
	</div>

	<div class="sidebar-section">
		<h4 class="heading">
			<span class="heading-icon"><BarChart3 size={16} strokeWidth={1.6} /></span>
			{lang && t('quick_stats')}
		</h4>
		<div class="stat">
			<span>{lang && t('equity')}</span>
			<strong>{formatCurrency(state.equity)}</strong>
		</div>
		<div class="stat">
			<span>{lang && t('daily_pnl')}</span>
			<strong>{formatCurrency(state.daily_pnl)}</strong>
		</div>
	</div>

	<div class="sidebar-section">
		<h4 class="heading">
			<span class="heading-icon"><Languages size={16} strokeWidth={1.6} /></span>
			{lang && t('language')}
		</h4>
		<select
			value={currentLang}
			on:change={(event) => setLanguage((event.target as HTMLSelectElement).value as any)}
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

	.status-row {
		display: flex;
		justify-content: space-between;
		font-size: 13px;
		padding: 4px 0;
	}

	.status-row.strategy-name {
		justify-content: flex-start;
	}

	.status-row.strategy-name em {
		font-style: italic;
		font-weight: 600;
	}

	.status-row span:last-child {
		font-weight: 600;
	}

	.status-row .online {
		color: #4ade80;
	}

	.status-row .offline {
		color: #f59e0b;
	}

	.stat {
		display: flex;
		justify-content: space-between;
		font-size: 13px;
		padding: 4px 0;
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

	.connection-form {
		display: flex;
		flex-direction: column;
		gap: 8px;
		margin: 10px 0 6px;
	}

	.connection-form label {
		display: flex;
		flex-direction: column;
		gap: 4px;
		font-size: 12px;
		color: #94a3b8;
	}

	.connection-form input {
		background: #0b1320;
		border: 1px solid #1f2937;
		border-radius: 8px;
		padding: 6px 10px;
		color: #e2e8f0;
		font-size: 13px;
	}

	.connection-actions {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.connection-actions button {
		border: 1px solid #1f2937;
		background: #111827;
		color: #e2e8f0;
		border-radius: 8px;
		padding: 6px 10px;
		font-size: 12px;
		cursor: pointer;
	}

	.connection-actions button.primary {
		background: #1d4ed8;
		border-color: #1e3a8a;
	}

	.connection-actions button:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.connection-error {
		margin: 0;
		font-size: 12px;
		color: #f87171;
	}
</style>
