<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { createReconnectingWebSocket } from '$lib/ws';
	import { emergencyStop, formatApiError, getHealth, getRestMetric, startBot, stopBot } from '$lib/api';
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
		Server,
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
	};

	let state: BotState | null = null;
	let logs: string[] = [];
	let error = '';
	let commandStatus = '';
	let stateConnected = false;
	let logsConnected = false;
	let lastStateAt: number | null = null;
	let lastLogsAt: number | null = null;
	let stateReconnects = 0;
	let logsReconnects = 0;
	let stateCloseInfo: string | null = null;
	let logsCloseInfo: string | null = null;
	let lastErrorAt: number | null = null;
	let restLastMs: number | null = null;
	let restLastAt: number | null = null;
	let apiStatus = 'checking';
	let apiError = '';
	$: _lang = $language;

	let stateConnection: { close: () => void } | null = null;
	let logsConnection: { close: () => void } | null = null;

	function connect() {
		stateConnection = createReconnectingWebSocket('/ws/state', {
			onMessage: (event) => {
				state = JSON.parse(event.data) as BotState;
				lastStateAt = Date.now();
			},
			onOpen: () => {
				stateConnected = true;
				error = '';
			},
			onClose: (event) => {
				stateConnected = false;
				stateReconnects += 1;
				stateCloseInfo = formatClose(event);
			},
			onError: () => {
				error = 'State stream error';
				lastErrorAt = Date.now();
			}
		});

		logsConnection = createReconnectingWebSocket('/ws/logs', {
			onMessage: (event) => {
				const payload = JSON.parse(event.data) as { logs?: string[] };
				logs = payload.logs ?? [];
				lastLogsAt = Date.now();
			},
			onOpen: () => {
				logsConnected = true;
				error = '';
			},
			onClose: (event) => {
				logsConnected = false;
				logsReconnects += 1;
				logsCloseInfo = formatClose(event);
			},
			onError: () => {
				error = 'Logs stream error';
				lastErrorAt = Date.now();
			}
		});
	}

	const formatCurrency = (value?: number) =>
		typeof value === 'number' ? `$${value.toFixed(2)}` : '--';

	const formatNumber = (value?: number) =>
		typeof value === 'number' ? value.toFixed(2) : '--';

	const formatLatency = (timestamp: number | null) => {
		if (!timestamp) return '--';
		const delta = Math.max(0, Date.now() - timestamp);
		return `${Math.round(delta / 1000)}s`; // seconds since last message
	};

	const formatMs = (value: number | null) =>
		value === null || Number.isNaN(value) ? '--' : `${Math.round(value)}ms`;

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

	const captureRestMetric = (key: string) => {
		const metric = getRestMetric(key);
		if (!metric) return;
		restLastMs = metric.lastMs;
		restLastAt = metric.lastAt;
	};

	const formatClose = (event?: CloseEvent) => {
		if (!event) return '--';
		const reason = event.reason ? ` - ${event.reason}` : '';
		return `${event.code}${reason}`;
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

	async function handleStart() {
		commandStatus = '';
		try {
			const result = await startBot();
			commandStatus = result.status;
		} catch (err) {
			commandStatus = formatApiError(err);
		} finally {
			captureRestMetric('bot.start');
		}
	}

	async function handleStop() {
		commandStatus = '';
		try {
			const result = await stopBot();
			commandStatus = result.status;
		} catch (err) {
			commandStatus = formatApiError(err);
		} finally {
			captureRestMetric('bot.stop');
		}
	}

	async function handleEmergencyStop() {
		commandStatus = '';
		try {
			const result = await emergencyStop();
			commandStatus = result.status;
		} catch (err) {
			commandStatus = formatApiError(err);
		} finally {
			captureRestMetric('bot.emergency_stop');
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

<div class="card">
	<h2 class="heading"><span class="heading-icon"><Server size={18} strokeWidth={1.6} /></span>{t('api_status')}</h2>
	<span class="badge">
		<span class={`status-dot ${apiStatus === 'online' ? '' : 'offline'}`}></span>
		{apiStatus}
	</span>
	{#if apiError}
		<p style="color: #f87171; margin-top: 12px;">{apiError}</p>
	{/if}
</div>

<div class="card">
	<h2 class="heading"><span class="heading-icon"><Gamepad2 size={18} strokeWidth={1.6} /></span>{t('bot_controls')}</h2>
	<div style="display: flex; flex-wrap: wrap; gap: 10px;">
		<button on:click={handleStart}>{t('start_bot')}</button>
		<button class="secondary" on:click={handleStop}>{t('stop_bot')}</button>
		<button class="danger" on:click={handleEmergencyStop}>{t('emergency_stop')}</button>
	</div>
	{#if commandStatus}
		<p class="muted" style="margin-top: 8px;">{commandStatus}</p>
	{/if}
</div>

<div class="card">
	<h2 class="heading"><span class="heading-icon"><ShieldCheck size={18} strokeWidth={1.6} /></span>Status</h2>
	<div style="display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 10px;">
		<span class="badge">
			<span class={`status-dot ${stateConnected ? '' : 'offline'}`}></span>
			State {stateConnected ? 'connected' : 'disconnected'}
		</span>
		<span class="badge">
			<span class={`status-dot ${logsConnected ? '' : 'offline'}`}></span>
			Logs {logsConnected ? 'connected' : 'disconnected'}
		</span>
		<span class="badge">State lag: {formatLatency(lastStateAt)}</span>
		<span class="badge">Logs lag: {formatLatency(lastLogsAt)}</span>
		<span class="badge">Reconnects: {stateReconnects + logsReconnects}</span>
		<span class="badge">State close: {stateCloseInfo ?? '--'}</span>
		<span class="badge">Logs close: {logsCloseInfo ?? '--'}</span>
		<span class="badge">Last error: {formatLatency(lastErrorAt)}</span>
		<span class="badge">REST last: {formatMs(restLastMs)}</span>
		<span class="badge">REST age: {formatLatency(restLastAt)}</span>
	</div>
	<div class="metric-grid">
		<div>
			<p>Status</p>
			<strong>{state?.status ?? 'unknown'}</strong>
		</div>
		<div>
			<p>TWS</p>
			<strong>{state?.tws_connected ? 'connected' : 'disconnected'}</strong>
		</div>
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
</style>
