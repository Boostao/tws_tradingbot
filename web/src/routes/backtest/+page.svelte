<script lang="ts">
	import { onDestroy } from 'svelte';
	import {
		clearCacheByPrefix,
		formatApiError,
		getBacktestResults,
		getBacktestStatus,
		getCachePolicy,
		getLastApiError,
		getRestMetric,
		runBacktest,
		setCacheTtl
	} from '$lib/api';
	import { t, language } from '$lib/i18n';
	import { botState } from '$lib/stores/botState';
	import SymbolMultiSelect from '$lib/components/SymbolMultiSelect.svelte';
	import { Beaker, PlayCircle, Plug, BarChart3 } from 'lucide-svelte';

	type EquityPoint = {
		timestamp?: string;
		equity?: number;
		cash?: number;
		position_value?: number;
		drawdown?: number;
		drawdown_pct?: number;
	};

	type Trade = {
		entry_time?: string;
		exit_time?: string | null;
		symbol?: string;
		side?: string;
		quantity?: number;
		entry_price?: number;
		exit_price?: number | null;
		pnl?: number;
		pnl_percent?: number;
	};

	let tickers: string[] = ['SPY', 'QQQ'];
	let startDate = new Date(Date.now() - 1000 * 60 * 60 * 24 * 90).toISOString().slice(0, 10);
	let endDate = new Date().toISOString().slice(0, 10);
	let timeframe = '5m';
	let initialCapital = 10000;
	let useTwsData = false;
	let useNautilus = false;

	let jobId = '';
	let status = '';
	let error = '';
	let result: Record<string, unknown> | null = null;
	let polling = false;
	let equityPath = '';
	let backtestStatusTtl = getCachePolicy().backtestStatus;
	let backtestResultsTtl = getCachePolicy().backtestResults;
	let restRunMs: number | null = null;
	let restStatusMs: number | null = null;
	let restResultsMs: number | null = null;
	let restLastError: string | null = null;
	let autoRefresh = false;
	let autoRefreshMs = 2000;
	let autoRefreshTimer: ReturnType<typeof setInterval> | null = null;

	$: _lang = $language;
	$: metrics = (result?.metrics ?? {}) as Record<string, number>;
	$: equityCurve = (result?.equity_curve ?? []) as EquityPoint[];
	$: trades = (result?.trades ?? []) as Trade[];
	$: equityPath = buildEquityPath(equityCurve);

	function formatCurrency(value?: number) {
		return typeof value === 'number' ? `$${value.toFixed(2)}` : '--';
	}

	function formatPercent(value?: number) {
		return typeof value === 'number' ? `${value.toFixed(2)}%` : '--';
	}

	const formatMs = (value: number | null) =>
		value === null || Number.isNaN(value) ? '--' : `${Math.round(value)}ms`;

	function updateRestBadges() {
		restRunMs = getRestMetric('backtest.run')?.lastMs ?? null;
		restStatusMs = getRestMetric('backtest.status')?.lastMs ?? null;
		restResultsMs = getRestMetric('backtest.results')?.lastMs ?? null;
		restLastError = getLastApiError()?.message ?? null;
	}

	function buildEquityPath(points: EquityPoint[]) {
		if (!points.length) return '';
		const values = points.map((p) => (typeof p.equity === 'number' ? p.equity : 0));
		const min = Math.min(...values);
		const max = Math.max(...values);
		const range = max - min || 1;
		return points
			.map((p, index) => {
				const value = typeof p.equity === 'number' ? p.equity : 0;
				const x = (index / Math.max(1, points.length - 1)) * 100;
				const y = 40 - ((value - min) / range) * 40;
				return `${index === 0 ? 'M' : 'L'}${x},${y}`;
			})
			.join(' ');
	}

	async function startBacktest() {
		error = '';
		result = null;
		status = 'running';
		const payload = {
			tickers,
			start_date: startDate,
			end_date: endDate,
			timeframe,
			initial_capital: initialCapital,
			use_tws_data: useTwsData,
			use_nautilus: useNautilus
		};

		try {
			const run = await runBacktest(payload);
			jobId = run.job_id;
			polling = true;
			await pollStatus();
		} catch (err) {
			error = formatApiError(err);
			status = 'failed';
		} finally {
			updateRestBadges();
		}
	}

	async function pollStatus() {
		while (polling && jobId) {
			const current = await getBacktestStatus(jobId);
			status = current.status;
			updateRestBadges();
			if (current.status === 'completed' || current.status === 'failed') {
				polling = false;
				const results = await getBacktestResults(jobId);
				result = results.result ?? null;
				error = results.error ?? '';
				updateRestBadges();
				break;
			}
			await new Promise((resolve) => setTimeout(resolve, 1000));
		}
	}

	async function refreshStatus(force = false) {
		if (!jobId) return;
		try {
			const current = await getBacktestStatus(jobId, force);
			status = current.status;
		} catch (err) {
			error = formatApiError(err);
		} finally {
			updateRestBadges();
		}
	}

	async function refreshResults(force = false) {
		if (!jobId) return;
		try {
			const results = await getBacktestResults(jobId, force);
			result = results.result ?? null;
			error = results.error ?? '';
		} catch (err) {
			error = formatApiError(err);
		} finally {
			updateRestBadges();
		}
	}

	$: if (autoRefresh) {
		if (autoRefreshTimer) clearInterval(autoRefreshTimer);
		autoRefreshTimer = setInterval(() => {
			void refreshStatus(true);
		}, autoRefreshMs);
	} else if (autoRefreshTimer) {
		clearInterval(autoRefreshTimer);
		autoRefreshTimer = null;
	}

	onDestroy(() => {
		if (autoRefreshTimer) clearInterval(autoRefreshTimer);
	});
</script>

<h1 class="heading"><span class="heading-icon"><Beaker size={20} strokeWidth={1.6} /></span>{t('backtest_configuration')}</h1>

<div class="card">
	<h2 class="heading"><span class="heading-icon"><PlayCircle size={18} strokeWidth={1.6} /></span>{t('run_backtest')}</h2>
	<div style="display: grid; gap: 12px; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));">
		<div>
			<span class="muted" style="display: block; margin-bottom: 6px;">
				{t('select_tickers').replaceAll('**', '')}
			</span>
			<SymbolMultiSelect
				selected={tickers}
				statusTone={$botState.tws_connected ? 'online' : 'offline'}
				on:change={(event) => (tickers = event.detail)}
			/>
		</div>
		<label>
			{t('start_date')}
			<input type="date" bind:value={startDate} />
		</label>
		<label>
			{t('end_date')}
			<input type="date" bind:value={endDate} />
		</label>
		<label>
			{t('timeframe')}
			<select bind:value={timeframe}>
				<option value="5m">5m</option>
				<option value="15m">15m</option>
				<option value="30m">30m</option>
				<option value="1h">1h</option>
				<option value="1d">1d</option>
			</select>
		</label>
		<label>
			{t('initial_capital')}
			<input type="number" bind:value={initialCapital} min="1000" step="1000" />
		</label>
		<label>
			<input type="checkbox" bind:checked={useTwsData} />
			{t('use_real_tws_data')}
		</label>
		<label>
			<input type="checkbox" bind:checked={useNautilus} />
			{t('use_nautilus_backtest')}
		</label>
	</div>
	<button style="margin-top: 16px;" on:click={startBacktest}>{t('run_backtest')}</button>
</div>

<div class="card">
	<h2 class="heading"><span class="heading-icon"><Plug size={18} strokeWidth={1.6} /></span>{t('connection_status')}</h2>
	<p>{status || 'idle'}</p>
	<div style="display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 8px;">
		<span class="badge">REST run: {formatMs(restRunMs)}</span>
		<span class="badge">REST status: {formatMs(restStatusMs)}</span>
		<span class="badge">REST results: {formatMs(restResultsMs)}</span>
		<span class="badge">Last API error: {restLastError ?? '--'}</span>
	</div>
	<div style="display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 8px; align-items: center;">
		<label style="display: flex; gap: 6px; align-items: center;">
			<input type="checkbox" bind:checked={autoRefresh} />
			{t('auto_refresh')}
		</label>
		<label style="display: flex; gap: 6px; align-items: center;">
			<span class="muted">Interval (ms)</span>
			<input type="number" min="500" step="500" bind:value={autoRefreshMs} />
		</label>
	</div>
	<div style="display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 8px; align-items: center;">
		<label class="muted" style="display: flex; gap: 6px; align-items: center;">
			Status cache TTL (ms)
			<input
				type="number"
				min="0"
				bind:value={backtestStatusTtl}
				on:change={(event) => {
					const next = Number((event.target as HTMLInputElement).value);
					backtestStatusTtl = next;
					setCacheTtl('backtestStatus', next);
					status = 'cache ttl updated';
				}}
			/>
		</label>
		<label class="muted" style="display: flex; gap: 6px; align-items: center;">
			Results cache TTL (ms)
			<input
				type="number"
				min="0"
				bind:value={backtestResultsTtl}
				on:change={(event) => {
					const next = Number((event.target as HTMLInputElement).value);
					backtestResultsTtl = next;
					setCacheTtl('backtestResults', next);
					status = 'cache ttl updated';
				}}
			/>
		</label>
	</div>
	<div style="display: flex; gap: 8px; flex-wrap: wrap;">
		<button class="secondary" on:click={() => refreshStatus(true)} disabled={!jobId}>
			{t('refresh')}
		</button>
		<button class="secondary" on:click={() => refreshResults(true)} disabled={!jobId}>
			{t('refresh')}
		</button>
		<button
			class="secondary"
			on:click={() => {
				clearCacheByPrefix('backtest_');
				status = 'cache cleared';
			}}
		>
			{t('clear_all')}
		</button>
	</div>
	{#if error}
		<p style="color: #f87171;">{error}</p>
	{/if}
</div>

{#if result}
	<div class="card">
		<h2 class="heading"><span class="heading-icon"><BarChart3 size={18} strokeWidth={1.6} /></span>{t('backtest_results')}</h2>
		<div class="metric-grid">
			<div>
				<p>{t('total_return')}</p>
				<strong>{formatCurrency(metrics.total_return)}</strong>
				<p class="muted" style="margin: 0;">{formatPercent(metrics.total_return_percent)}</p>
			</div>
			<div>
				<p>{t('sharpe_ratio')}</p>
				<strong>{metrics.sharpe_ratio?.toFixed?.(2) ?? '--'}</strong>
			</div>
			<div>
				<p>{t('max_drawdown')}</p>
				<strong>{formatCurrency(metrics.max_drawdown)}</strong>
				<p class="muted" style="margin: 0;">{formatPercent(metrics.max_drawdown_percent)}</p>
			</div>
			<div>
				<p>{t('win_rate')}</p>
				<strong>{formatPercent(metrics.win_rate)}</strong>
			</div>
			<div>
				<p>{t('total_trades')}</p>
				<strong>{metrics.total_trades ?? 0}</strong>
			</div>
			<div>
				<p>{t('profit_factor')}</p>
				<strong>{metrics.profit_factor?.toFixed?.(2) ?? '--'}</strong>
			</div>
		</div>
	</div>

	<div class="card">
		<h2>{t('equity_curve')}</h2>
		{#if equityCurve.length === 0}
			<p class="muted">{t('no_equity_data')}</p>
		{:else}
			<svg viewBox="0 0 100 40" width="100%" height="200" preserveAspectRatio="none">
				<path d={equityPath} fill="none" stroke="#38bdf8" stroke-width="1.5" />
			</svg>
		{/if}
	</div>

	<div class="card">
		<h2>{t('trade_history', { count: trades.length })}</h2>
		{#if trades.length === 0}
			<p class="muted">{t('no_trades_executed')}</p>
		{:else}
			<table class="table">
				<thead>
					<tr>
						<th>{t('symbol')}</th>
						<th>{t('side')}</th>
						<th>{t('entry')}</th>
						<th>{t('exit_price')}</th>
						<th>{t('quantity_short')}</th>
						<th>{t('pnl')}</th>
						<th>{t('pnl_percent')}</th>
					</tr>
				</thead>
				<tbody>
					{#each trades as trade}
						<tr>
							<td>{trade.symbol}</td>
							<td>{trade.side}</td>
							<td>{trade.entry_price != null ? formatCurrency(trade.entry_price) : '--'}</td>
							<td>{trade.exit_price != null ? formatCurrency(trade.exit_price) : '--'}</td>
							<td>{trade.quantity?.toFixed?.(2) ?? '--'}</td>
							<td>{formatCurrency(trade.pnl)}</td>
							<td>{formatPercent(trade.pnl_percent)}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}
	</div>
{/if}
