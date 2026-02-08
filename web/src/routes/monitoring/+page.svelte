<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { createReconnectingWebSocket } from '$lib/ws';
	import { emergencyStop, formatApiError, getHealth, startBot, stopBot } from '$lib/api';
	import { t, language } from '$lib/i18n';
	import {
		Activity,
		Gamepad2,
		LineChart,
		ShieldCheck,
		Briefcase,
		ClipboardList,
		Code2,
		FileText,
		SquareChevronDown,
		SquareChevronRight
	} from 'lucide-svelte';

	type Position = {
		symbol?: string;
		quantity?: number;
		entry_price?: number;
		current_price?: number;
		unrealized_pnl?: number;
		entry_time?: string | null;
	};

	type Order = {
		order_id?: string;
		symbol?: string;
		side?: string;
		quantity?: number;
		price?: number | null;
		status?: string;
		order_type?: string;
		submitted_time?: string | null;
		filled_quantity?: number;
	};

	type BotState = {
		status?: string;
		tws_connected?: boolean;
		equity?: number;
		daily_pnl?: number;
		daily_pnl_percent?: number;
		total_pnl?: number;
		open_positions_count?: number;
		pending_orders_count?: number;
		trades_today?: number;
		win_rate_today?: number;
		last_update?: string | null;
		active_strategy?: string;
		error_message?: string;
		positions?: Position[];
		orders?: Order[];
		recent_logs?: string[];
		runner_active?: boolean;
	};

	let state: BotState | null = null;
	let logs: string[] = [];
	let error = '';
	let stateConnected = false;
	let apiStatus = 'checking';
	let apiError = '';
	$: _lang = $language;
	$: isRunning = state?.status === 'RUNNING';
	$: isTransitioning = state?.status === 'STARTING' || state?.status === 'STOPPING';
	$: canControl = Boolean(state?.runner_active) && !isTransitioning;
	$: primaryActionLabel = isRunning ? t('stop_bot') : t('start_bot');

	let stateConnection: { close: () => void } | null = null;
	let logsConnection: { close: () => void } | null = null;

	function connect() {
		stateConnection = createReconnectingWebSocket('/ws/state', {
			onMessage: (event) => {
				state = JSON.parse(event.data) as BotState;
			},
			onOpen: () => {
				stateConnected = true;
				error = '';
			},
			onClose: () => {
				stateConnected = false;
			},
			onError: () => {
				error = 'State stream error';
			}
		});

		logsConnection = createReconnectingWebSocket('/ws/logs', {
			onMessage: (event) => {
				const payload = JSON.parse(event.data) as { logs?: string[] };
				logs = payload.logs ?? [];
			},
			onOpen: () => {
				error = '';
			},
			onClose: () => {},
			onError: () => {
				error = 'Logs stream error';
			}
		});
	}

	const formatCurrency = (value?: number) =>
		typeof value === 'number' ? `$${value.toFixed(2)}` : '--';

	const formatNumber = (value?: number) =>
		typeof value === 'number' ? value.toFixed(2) : '--';

	const formatTimestamp = (value?: string | null) => {
		if (!value) return '--';
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) return value;
		return new Intl.DateTimeFormat('en-US', {
			year: 'numeric',
			month: 'short',
			day: '2-digit',
			hour: '2-digit',
			minute: '2-digit',
			second: '2-digit'
		}).format(date);
	};

	const refreshApiStatus = async () => {
		apiError = '';
		try {
			const result = await getHealth();
			apiStatus = result.status === 'ok' ? 'online' : result.status;
		} catch (err) {
			apiStatus = 'offline';
			apiError = err instanceof Error ? err.message : 'Unknown error';
		}
	};

	async function handlePrimaryAction() {
		apiError = '';
		try {
			if (isRunning) {
				await stopBot();
			} else {
				await startBot();
			}
		} catch (err) {
			apiError = formatApiError(err);
		}
	}

	async function handleEmergencyStop() {
		apiError = '';
		try {
			await emergencyStop();
		} catch (err) {
			apiError = formatApiError(err);
		}
	}

	onMount(() => {
		connect();
		refreshApiStatus();
	});

	onDestroy(() => {
		stateConnection?.close();
		logsConnection?.close();
	});
</script>

<h1 class="heading"><span class="heading-icon"><Activity size={20} strokeWidth={1.6} /></span>{t('monitoring_logging')}</h1>

{#if error}
	<p style="color: #f87171;">{error}</p>
{/if}

{#if state?.error_message}
	<p style="color: #f87171;">{state.error_message}</p>
{/if}

{#if state && stateConnected && state.runner_active === false}
	<div class="banner warning">
		<strong>{t('monitoring_bot_runner_disconnected_title')}</strong>
		<p>
			{t('monitoring_bot_runner_disconnected_prefix')}
			<code>{t('run_bot_command')}</code>
			{t('monitoring_bot_runner_disconnected_suffix')}
		</p>
	</div>
{/if}

<div class="card">
	<h2 class="heading"><span class="heading-icon"><ShieldCheck size={18} strokeWidth={1.6} /></span>{t('status_label')}</h2>
	<div style="display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 10px;">
		<span class="badge">
			<span
				class={`status-dot ${
					state?.status === 'RUNNING'
						? ''
						: state?.status === 'STARTING'
							? 'starting'
							: 'offline'
				}`}
			></span>
			{t('status_bot')}: {state?.status ?? 'unknown'}
		</span>
		<span class="badge">
			<span class={`status-dot ${apiStatus === 'online' ? '' : 'offline'}`}></span>
			{t('status_api')}: {apiStatus}
		</span>
		<span class="badge">
			<span class={`status-dot ${state?.tws_connected ? '' : 'offline'}`}></span>
			{t('status_tws')}: {state?.tws_connected ? 'connected' : 'disconnected'}
		</span>
	</div>
	{#if apiError}
		<p style="color: #f87171; margin-bottom: 12px;">{apiError}</p>
	{/if}
	<div class="metric-grid">
		<div>
			<p>Active Strategy</p>
			<strong>{state?.active_strategy || 'none'}</strong>
		</div>
		<div>
			<p>Last Update</p>
			<strong>{formatTimestamp(state?.last_update)}</strong>
		</div>
	</div>
</div>

<div class="card">
	<h2 class="heading"><span class="heading-icon"><Gamepad2 size={18} strokeWidth={1.6} /></span>{t('bot_controls')}</h2>
	<div style="display: flex; flex-wrap: wrap; gap: 10px;">
		<button on:click={handlePrimaryAction} disabled={!canControl}>
			{primaryActionLabel}
		</button>
		{#if isRunning}
			<button class="danger" on:click={handleEmergencyStop} disabled={!state?.runner_active}>
				{t('emergency_stop')}
			</button>
		{/if}
	</div>
</div>

<div class="card">
	<h2 class="heading"><span class="heading-icon"><LineChart size={18} strokeWidth={1.6} /></span>Key Metrics</h2>
	<div class="metric-grid">
		<div>
			<p>Equity</p>
			<strong>{formatCurrency(state?.equity)}</strong>
		</div>
		<div>
			<p>Daily PnL</p>
			<strong>{formatCurrency(state?.daily_pnl)}</strong>
			<p class="muted" style="margin: 0;">{state?.daily_pnl_percent?.toFixed(2) ?? '0.00'}%</p>
		</div>
		<div>
			<p>Total PnL</p>
			<strong>{formatCurrency(state?.total_pnl)}</strong>
		</div>
		<div>
			<p>Open Positions</p>
			<strong>{state?.open_positions_count ?? 0}</strong>
		</div>
		<div>
			<p>Pending Orders</p>
			<strong>{state?.pending_orders_count ?? 0}</strong>
		</div>
		<div>
			<p>Trades Today</p>
			<strong>{state?.trades_today ?? 0}</strong>
			<p class="muted" style="margin: 0;">Win rate: {state?.win_rate_today ?? 0}%</p>
		</div>
	</div>
</div>

<div class="card">
	<h2 class="heading"><span class="heading-icon"><Briefcase size={18} strokeWidth={1.6} /></span>Positions</h2>
	{#if !state?.positions || state.positions.length === 0}
		<p class="muted">No open positions.</p>
	{:else}
		<table class="table">
			<thead>
				<tr>
					<th>Symbol</th>
					<th>Qty</th>
					<th>Entry</th>
					<th>Current</th>
					<th>Unrealized PnL</th>
					<th>Entry Time</th>
				</tr>
			</thead>
			<tbody>
				{#each state.positions as pos}
					<tr>
						<td>{pos.symbol}</td>
						<td>{formatNumber(pos.quantity)}</td>
						<td>{formatCurrency(pos.entry_price)}</td>
						<td>{formatCurrency(pos.current_price)}</td>
						<td>{formatCurrency(pos.unrealized_pnl)}</td>
						<td>{formatTimestamp(pos.entry_time)}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
</div>

<div class="card">
	<h2 class="heading"><span class="heading-icon"><ClipboardList size={18} strokeWidth={1.6} /></span>Orders</h2>
	{#if !state?.orders || state.orders.length === 0}
		<p class="muted">No recent orders.</p>
	{:else}
		<table class="table">
			<thead>
				<tr>
					<th>ID</th>
					<th>Symbol</th>
					<th>Side</th>
					<th>Qty</th>
					<th>Price</th>
					<th>Status</th>
					<th>Type</th>
					<th>Submitted</th>
				</tr>
			</thead>
			<tbody>
				{#each state.orders as order}
					<tr>
						<td>{order.order_id}</td>
						<td>{order.symbol}</td>
						<td>{order.side}</td>
						<td>{formatNumber(order.quantity)}</td>
						<td>{order.price != null ? formatCurrency(order.price) : '--'}</td>
						<td>{order.status}</td>
						<td>{order.order_type}</td>
						<td>{formatTimestamp(order.submitted_time)}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
</div>

<div class="card">
	<details class="raw-state">
		<summary class="raw-summary">
			<div class="raw-summary-row">
				<h2 class="heading">
					<span class="heading-icon"><Code2 size={18} strokeWidth={1.6} /></span>Raw State
				</h2>
				<span class="raw-toggle" aria-hidden="true">
					<span class="raw-toggle-closed">
						<SquareChevronRight size={18} strokeWidth={1.6} />
					</span>
					<span class="raw-toggle-open">
						<SquareChevronDown size={18} strokeWidth={1.6} />
					</span>
				</span>
			</div>
		</summary>
		<pre>{JSON.stringify(state, null, 2)}</pre>
	</details>
</div>

<div class="card">
	<h2 class="heading"><span class="heading-icon"><FileText size={18} strokeWidth={1.6} /></span>Recent Logs</h2>
	{#if logs.length === 0 && (!state?.recent_logs || state.recent_logs.length === 0)}
		<p class="muted">No logs yet.</p>
	{:else}
		<ul>
			{#each (logs.length ? logs : state?.recent_logs ?? []) as line}
				<li>{line}</li>
			{/each}
		</ul>
	{/if}
</div>

<style>
	.raw-summary {
		list-style: none;
		cursor: pointer;
	}

	.raw-summary::-webkit-details-marker {
		display: none;
	}

	.raw-summary-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
	}

	.raw-summary h2 {
		margin: 0;
	}

	.raw-toggle {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		color: #94a3b8;
	}

	.raw-toggle-open {
		display: none;
	}

	.raw-state[open] .raw-toggle-open {
		display: inline;
	}

	.raw-state[open] .raw-toggle-closed {
		display: none;
	}

	.banner {
		padding: 12px 16px;
		border-radius: 8px;
		margin-bottom: 20px;
		border: 1px solid transparent;
	}

	.banner.warning {
		background: #451a03;
		border-color: #78350f;
		color: #fcd34d;
	}
	
	.banner strong {
		display: block;
		margin-bottom: 4px;
	}
	
	.banner p {
		margin: 0;
		font-size: 0.9em;
		opacity: 0.9;
	}
	
	.banner code {
		background: rgba(0,0,0,0.2);
		padding: 2px 4px;
		border-radius: 4px;
		font-family: monospace;
	}
</style>
