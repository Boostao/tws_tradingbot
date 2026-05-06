<script lang="ts">
	import { onMount } from 'svelte';
	import { dryRunBot, formatApiError, getDiagnostics, type BotDryRun, type DiagnosticsResponse } from '$lib/api';
	import { t } from '$lib/i18n';
	import { refreshRuntimeState, runtimeState, setRuntimeState } from '$lib/stores/runtime';
	import { Activity, RefreshCcw } from 'lucide-svelte';

	let diagnostics: DiagnosticsResponse | null = null;
	let dryRunResult: BotDryRun | null = null;
	let loading = true;
	let refreshing = false;
	let runningDryRun = false;
	let message = '';

	$: botState = $runtimeState;
	$: recentTrades = Array.isArray(botState?.recent_trades) ? botState?.recent_trades ?? [] : [];
	$: latestDryRun = dryRunResult ?? parseDryRun(botState?.last_dry_run);
	$: latestDryRunOrders = Array.isArray(latestDryRun?.planned_orders) ? latestDryRun?.planned_orders ?? [] : [];
	$: latestDryRunSubscriptions = Array.isArray(latestDryRun?.subscriptions) ? latestDryRun?.subscriptions ?? [] : [];
	$: lastRuntimeReloadAt = botState?.last_runtime_reload_at ?? diagnostics?.runtime.last_runtime_reload_at ?? null;
	$: lastRuntimeReloadReason = botState?.last_runtime_reload_reason ?? diagnostics?.runtime.last_runtime_reload_reason ?? null;
	$: lastDisconnectAt = botState?.last_disconnect_at ?? diagnostics?.runtime.last_disconnect_at ?? null;
	$: lastDisconnectReason = botState?.last_disconnect_reason ?? diagnostics?.runtime.last_disconnect_reason ?? null;

	async function loadMonitoring(force = false) {
		if (force) refreshing = true;
		message = '';
		try {
			const [nextDiagnostics, nextState] = await Promise.all([getDiagnostics(force), refreshRuntimeState(force)]);
			diagnostics = nextDiagnostics;
			dryRunResult = parseDryRun(nextState?.last_dry_run);
		} catch (err) {
			message = formatApiError(err);
		} finally {
			loading = false;
			refreshing = false;
		}
	}

	async function refreshDiagnosticsTick() {
		try {
			diagnostics = await getDiagnostics(true);
		} catch {
			return;
		}
	}

	async function runDryRun() {
		runningDryRun = true;
		message = '';
		try {
			const result = await dryRunBot();
			dryRunResult = result;
			setRuntimeState(result.state);
			diagnostics = await getDiagnostics(true);
		} catch (err) {
			message = formatApiError(err);
		} finally {
			runningDryRun = false;
		}
	}

	onMount(() => {
		void loadMonitoring();
		const intervalId = setInterval(() => {
			void refreshDiagnosticsTick();
		}, 5000);
		return () => clearInterval(intervalId);
	});

	function parseDryRun(value: Record<string, unknown> | null | undefined): BotDryRun | null {
		if (!value || typeof value !== 'object') return null;
		return value as BotDryRun;
	}

	function numberValue(value: unknown): number | null {
		if (typeof value === 'number' && Number.isFinite(value)) return value;
		if (typeof value === 'string') {
			const parsed = Number(value);
			return Number.isFinite(parsed) ? parsed : null;
		}
		return null;
	}

	function stringValue(value: unknown, fallback = '—'): string {
		if (typeof value === 'string' && value.trim()) return value;
		if (typeof value === 'number' && Number.isFinite(value)) return String(value);
		return fallback;
	}

	function formatTimestamp(value?: string | null) {
		if (!value) return t('watchlist_never_refreshed');
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) return value;
		return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(date);
	}

	function formatCurrency(value: unknown) {
		const amount = numberValue(value);
		if (amount === null) return '—';
		return new Intl.NumberFormat(undefined, { style: 'currency', currency: 'USD', maximumFractionDigits: 2 }).format(amount);
	}

	function formatCompactNumber(value: unknown) {
		const amount = numberValue(value);
		if (amount === null) return '—';
		return new Intl.NumberFormat(undefined, { maximumFractionDigits: 2 }).format(amount);
	}

	function formatPercent(value: unknown) {
		const amount = numberValue(value);
		if (amount === null) return '—';
		return `${amount.toFixed(1)}%`;
	}

	function compactPath(value?: string | null) {
		if (!value) return '—';
		const normalized = value.replaceAll('\\', '/');
		const parts = normalized.split('/').filter(Boolean);
		return parts.length <= 2 ? normalized : parts.slice(-2).join('/');
	}

	function formatReloadReason(value?: string | null) {
		if (!value) return 'Not yet reloaded';
		if (value === 'startup') return 'Startup';
		if (value === 'cycle') return 'Execution cycle';
		if (value === 'dry_run') return 'Dry run';
		return value;
	}

	function formatSymbolWarning(value?: string | null) {
		if (!value) return '—';
		const key = `symbol_warning_${value}`;
		const translated = t(key);
		return translated === key ? value : translated;
	}

	function tradeSymbol(trade: Record<string, unknown>) {
		return stringValue(trade.symbol ?? trade.instrument_id);
	}

	function tradePnlClass(trade: Record<string, unknown>) {
		const pnl = numberValue(trade.pnl);
		if (pnl === null || pnl === 0) return '';
		return pnl > 0 ? 'is-positive' : 'is-negative';
	}

	function formatStatus(value?: string | null) {
		const normalized = String(value ?? 'unknown').trim().replaceAll('_', ' ').toLowerCase();
		if (!normalized) return 'Unknown';
		return normalized.charAt(0).toUpperCase() + normalized.slice(1);
	}

	function statusTone(value?: string | null) {
		const normalized = String(value ?? '').trim().toUpperCase();
		if (normalized === 'RUNNING') return 'is-live';
		if (normalized === 'STARTING' || normalized === 'STOPPING') return 'is-warm';
		if (normalized === 'ERROR' || normalized === 'DISCONNECTED') return 'is-alert';
		return 'is-idle';
	}

	function runtimeSummary() {
		if (botState?.error_message) return botState.error_message;
		if (diagnostics?.symbols?.last_warning) {
			return `Symbol source warning: ${formatSymbolWarning(diagnostics.symbols.last_warning)}.`;
		}
		if (!botState?.tws_connected) {
			return 'Connect TWS to unlock live account state, market data flow, and execution diagnostics.';
		}
		if (botState?.runner_active) {
			return 'Runner is active. Watch heartbeat, pending orders, and recent fills as the session evolves.';
		}
		return 'Configuration is loaded. Run a preview before the next paper or live start.';
	}
</script>

<div class="cockpit-page monitoring-page">
	<div class="cockpit-page-header monitoring-page-header">
		<div class="monitoring-heading-block">
			<span class="monitoring-kicker">Mission control</span>
			<h1 class="heading"><span class="heading-icon"><Activity size={20} strokeWidth={1.6} /></span>{t('monitoring')}</h1>
		</div>
		<div class="cockpit-top-actions">
			<button class="secondary" on:click={() => loadMonitoring(true)} disabled={refreshing || runningDryRun}>
				<RefreshCcw size={14} strokeWidth={1.8} />
				<span>{t('reload')}</span>
			</button>
		</div>
		{#if message}
			<p class="cockpit-message">{message}</p>
		{/if}
	</div>

	{#if loading && !diagnostics && !botState}
		<div class="card">
			<p>{t('loading_chart_data')}</p>
		</div>
	{:else}
		<section class="card monitoring-hero">
			<div class="monitoring-hero-main">
				<span class={`monitoring-hero-status ${statusTone(botState?.status)}`}>{formatStatus(botState?.status)}</span>
				<h2>{botState?.active_strategy || 'No active strategy loaded'}</h2>
				<p>{runtimeSummary()}</p>
			</div>
			<div class="monitoring-hero-strip">
				<div class="monitoring-hero-chip">
					<span>Mode</span>
					<strong>{diagnostics?.startup.trading_mode ?? '—'}</strong>
				</div>
				<div class="monitoring-hero-chip">
					<span>Endpoint</span>
					<strong>{diagnostics ? `${diagnostics.startup.ib_host}:${diagnostics.startup.ib_port}` : '—'}</strong>
				</div>
				<div class="monitoring-hero-chip">
					<span>Account</span>
					<strong>{diagnostics?.startup.account ?? '—'}</strong>
				</div>
				<div class="monitoring-hero-chip">
					<span>Heartbeat</span>
					<strong>{formatTimestamp(botState?.last_heartbeat ?? botState?.last_update)}</strong>
				</div>
			</div>
		</section>

		<div class="monitoring-main-grid">
			<section class="card cockpit-runtime-card monitoring-panel monitoring-runtime-card">
				<div class="monitoring-card-head">
					<div>
						<span class="monitoring-card-kicker">Execution surface</span>
						<h2>Runtime</h2>
					</div>
					<span class={`monitoring-badge ${statusTone(botState?.status)}`}>{botState?.tws_connected ? 'TWS linked' : 'TWS offline'}</span>
				</div>
				<div class="monitoring-runtime-primary">
					<div class="monitoring-stat-tile monitoring-stat-tile-strong">
						<span class="monitoring-stat-label">Equity</span>
						<strong>{formatCurrency(botState?.equity)}</strong>
						<span class="monitoring-stat-meta">Account snapshot</span>
					</div>
					<div class="monitoring-stat-tile">
						<span class="monitoring-stat-label">Day PnL</span>
						<strong>{formatCurrency(botState?.daily_pnl)}</strong>
						<span class="monitoring-stat-meta">{formatPercent(botState?.daily_pnl_percent)}</span>
					</div>
					<div class="monitoring-stat-tile">
						<span class="monitoring-stat-label">Total PnL</span>
						<strong>{formatCurrency(botState?.total_pnl)}</strong>
						<span class="monitoring-stat-meta">{formatStatus(botState?.status)}</span>
					</div>
				</div>
				<div class="monitoring-runtime-secondary">
					<div class="monitoring-mini-stat">
						<span class="monitoring-stat-label">Trades today</span>
						<strong>{formatCompactNumber(botState?.trades_today)}</strong>
					</div>
					<div class="monitoring-mini-stat">
						<span class="monitoring-stat-label">Win rate</span>
						<strong>{formatPercent(botState?.win_rate_today)}</strong>
					</div>
					<div class="monitoring-mini-stat">
						<span class="monitoring-stat-label">Open positions</span>
						<strong>{formatCompactNumber(botState?.open_positions_count)}</strong>
					</div>
					<div class="monitoring-mini-stat">
						<span class="monitoring-stat-label">Pending orders</span>
						<strong>{formatCompactNumber(botState?.pending_orders_count)}</strong>
					</div>
				</div>
				<div class="monitoring-pill-row">
					<span class={`monitoring-mini-pill ${botState?.tws_connected ? 'is-live' : 'is-idle'}`}>
						{botState?.tws_connected ? 'Broker connected' : 'Broker disconnected'}
					</span>
					<span class="monitoring-mini-pill">Reload {formatReloadReason(lastRuntimeReloadReason)}</span>
					<span class={`monitoring-mini-pill ${botState?.runner_active ? 'is-live' : 'is-idle'}`}>
						{botState?.runner_active ? 'Runner active' : 'Runner idle'}
					</span>
					{#if diagnostics?.symbols?.last_warning}
						<span class="monitoring-mini-pill is-alert">{formatSymbolWarning(diagnostics.symbols.last_warning)}</span>
					{:else}
						<span class="monitoring-mini-pill is-live">Symbol cache healthy</span>
					{/if}
				</div>
				<div class="cockpit-runtime-footnote">
					<span>Heartbeat {formatTimestamp(botState?.last_heartbeat ?? botState?.last_update)}</span>
					{#if botState?.error_message}
						<span class="monitoring-alert-inline">{botState.error_message}</span>
					{/if}
				</div>
			</section>

			<section class="card cockpit-runtime-card monitoring-panel monitoring-diagnostics-card">
				<div class="monitoring-card-head">
					<div>
						<span class="monitoring-card-kicker">Startup + health</span>
						<h2>Diagnostics</h2>
					</div>
					<span class="muted">{formatTimestamp(diagnostics?.symbols?.last_checked_at ?? lastRuntimeReloadAt)}</span>
				</div>
				{#if diagnostics}
					<div class="monitoring-startup-grid">
						<div class="monitoring-mini-stat">
							<span class="monitoring-stat-label">Mode</span>
							<strong>{diagnostics.startup.trading_mode}</strong>
						</div>
						<div class="monitoring-mini-stat">
							<span class="monitoring-stat-label">Endpoint</span>
							<strong>{diagnostics.startup.ib_host}:{diagnostics.startup.ib_port}</strong>
						</div>
						<div class="monitoring-mini-stat">
							<span class="monitoring-stat-label">Client</span>
							<strong>{formatCompactNumber(diagnostics.startup.client_id)}</strong>
						</div>
						<div class="monitoring-mini-stat">
							<span class="monitoring-stat-label">Logs</span>
							<strong>{diagnostics.startup.log_level}</strong>
						</div>
					</div>
					<div class="monitoring-path-grid">
						<div class="monitoring-path-tile" title={diagnostics.startup.watchlist_path}>
							<span class="monitoring-stat-label">Watchlist path</span>
							<strong>{compactPath(diagnostics.startup.watchlist_path)}</strong>
						</div>
						<div class="monitoring-path-tile" title={diagnostics.startup.strategy_path}>
							<span class="monitoring-stat-label">Strategy path</span>
							<strong>{compactPath(diagnostics.startup.strategy_path)}</strong>
						</div>
						<div class="monitoring-path-tile" title={diagnostics.startup.symbol_cache_path}>
							<span class="monitoring-stat-label">Symbol cache</span>
							<strong>{compactPath(diagnostics.startup.symbol_cache_path)}</strong>
						</div>
						<div class="monitoring-path-tile" title={diagnostics.startup.log_file}>
							<span class="monitoring-stat-label">Log file</span>
							<strong>{compactPath(diagnostics.startup.log_file)}</strong>
						</div>
					</div>
					<div class="monitoring-event-grid">
						<div class="monitoring-event-tile">
							<span class="monitoring-stat-label">Runtime reload</span>
							<strong>{formatTimestamp(lastRuntimeReloadAt)}</strong>
							<span class="monitoring-stat-meta">{formatReloadReason(lastRuntimeReloadReason)}</span>
						</div>
						<div class="monitoring-event-tile">
							<span class="monitoring-stat-label">Broker disconnect</span>
							<strong>{formatTimestamp(lastDisconnectAt)}</strong>
							<span class="monitoring-stat-meta">{lastDisconnectReason || 'None recorded'}</span>
						</div>
						<div class="monitoring-event-tile">
							<span class="monitoring-stat-label">Symbol health</span>
							<strong>{stringValue(diagnostics.symbols.source, 'local').toUpperCase()}</strong>
							<span class="monitoring-stat-meta">Checked {formatTimestamp(diagnostics.symbols.last_checked_at)}</span>
						</div>
						<div class="monitoring-event-tile">
							<span class="monitoring-stat-label">Environment</span>
							<strong>{diagnostics.startup.environment}</strong>
							<span class="monitoring-stat-meta">Account {diagnostics.startup.account}</span>
						</div>
					</div>
					{#if diagnostics.symbols.last_warning}
						<p class="monitoring-alert">{formatSymbolWarning(diagnostics.symbols.last_warning)}</p>
					{/if}
				{:else}
					<p class="cockpit-runtime-empty">Diagnostics snapshot unavailable.</p>
				{/if}
			</section>
		</div>

		<div class="monitoring-secondary-grid">
			<section class="card cockpit-runtime-card monitoring-panel monitoring-dry-run-card">
				<div class="monitoring-card-head">
					<div>
						<span class="monitoring-card-kicker">Preflight</span>
						<h2>Dry Run</h2>
					</div>
					<button class="secondary" on:click={runDryRun} disabled={runningDryRun || refreshing}>
						<RefreshCcw size={14} strokeWidth={1.8} />
						<span>{runningDryRun ? 'Running…' : 'Run preview'}</span>
					</button>
				</div>
				{#if latestDryRun}
					<div class="cockpit-runtime-metrics cockpit-runtime-metrics-compact">
						<div class="cockpit-runtime-metric">
							<span class="cockpit-meta-label">Generated</span>
							<strong>{formatTimestamp(latestDryRun.generated_at)}</strong>
						</div>
						<div class="cockpit-runtime-metric">
							<span class="cockpit-meta-label">Subscriptions</span>
							<strong>{latestDryRunSubscriptions.length}</strong>
						</div>
						<div class="cockpit-runtime-metric">
							<span class="cockpit-meta-label">Planned orders</span>
							<strong>{latestDryRunOrders.length}</strong>
						</div>
						<div class="cockpit-runtime-metric">
							<span class="cockpit-meta-label">Open orders</span>
							<strong>{latestDryRun.open_orders?.length ?? 0}</strong>
						</div>
					</div>
					{#if latestDryRunOrders.length > 0}
						<div class="cockpit-activity-list">
							{#each latestDryRunOrders.slice(0, 5) as order}
								<div class="cockpit-activity-row">
									<div class="cockpit-activity-main">
										<strong>{stringValue(order.instrument_id)}</strong>
										<span class="cockpit-ticker-subtle">{stringValue(order.side)} · {formatCompactNumber(order.quantity)} shares</span>
									</div>
									<span class="cockpit-ticker-subtle">{stringValue(order.reason)}</span>
								</div>
							{/each}
						</div>
					{:else}
						<p class="cockpit-runtime-empty">No orders are currently planned for the active strategy.</p>
					{/if}
				{:else}
					<p class="cockpit-runtime-empty">Run a preview to inspect current subscriptions, positions, and planned orders before live submission.</p>
				{/if}
			</section>

			<section class="card cockpit-runtime-card monitoring-panel monitoring-trades-card">
				<div class="monitoring-card-head">
					<div>
						<span class="monitoring-card-kicker">Ledger</span>
						<h2>Recent Trades</h2>
					</div>
					<span class="muted">{recentTrades.length}</span>
				</div>
				{#if recentTrades.length > 0}
					<div class="cockpit-activity-list">
						{#each recentTrades.slice(0, 8) as trade}
							<div class="cockpit-activity-row">
								<div class="cockpit-activity-main">
									<strong>{tradeSymbol(trade)}</strong>
									<span class="cockpit-ticker-subtle">{stringValue(trade.side)} · {formatCompactNumber(trade.quantity)} @ {formatCompactNumber(trade.price)}</span>
								</div>
								<div class="cockpit-activity-side">
									<span class:cockpit-pnl-positive={tradePnlClass(trade) === 'is-positive'} class:cockpit-pnl-negative={tradePnlClass(trade) === 'is-negative'}>{formatCurrency(trade.pnl)}</span>
									<span class="cockpit-ticker-subtle">{formatTimestamp(stringValue(trade.closed_at, ''))}</span>
								</div>
							</div>
						{/each}
					</div>
				{:else}
					<p class="cockpit-runtime-empty">Closed fills will appear here once the runner receives executions from TWS.</p>
				{/if}
			</section>
		</div>
	{/if}
</div>