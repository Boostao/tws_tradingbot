<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import {
		dryRunBot,
		formatApiError,
		getCockpit,
		getDiagnostics,
		getRecentTrades,
		resetCircuitBreaker,
		type BotDryRun,
		type BrokerAlert,
		type CockpitState,
		type DiagnosticsResponse,
		type TradeRecord
	} from '$lib/api';
	import { language, t } from '$lib/i18n';
	import { refreshRuntimeState, runtimeState, setRuntimeState } from '$lib/stores/runtime';
	import {
		Activity,
		AlertTriangle,
		Clock,
		DollarSign,
		Play,
		RefreshCcw,
		ShieldAlert,
		TrendingUp,
		Wallet
	} from 'lucide-svelte';

	let diagnostics: DiagnosticsResponse | null = null;
	let cockpit: CockpitState | null = null;
	let recentTradesApi: TradeRecord[] = [];
	let dryRunResult: BotDryRun | null = null;
	let loading = true;
	let refreshing = false;
	let runningDryRun = false;
	let resettingBreaker = false;
	let message = '';
	let pollTimer: ReturnType<typeof setInterval> | null = null;

	$: _lang = $language;
	$: tr = (key: string, params?: Record<string, string | number>) => {
		void _lang;
		return t(key, params);
	};

	$: botState = $runtimeState;

	$: activeWorkspace =
		cockpit?.workspaces?.find((w) => w.id === cockpit?.active_workspace_id) ??
		cockpit?.workspaces?.[0] ??
		null;
	$: activeSlot =
		activeWorkspace?.strategy_slots?.find((s) => s.enabled && s.strategy_id) ??
		activeWorkspace?.strategy_slots?.find((s) => s.strategy_id) ??
		null;
	$: cockpitStrategyName =
		cockpit?.strategy_library?.find((s) => s.id === activeSlot?.strategy_id)?.name ?? '';
	$: resolvedStrategyName =
		botState?.active_strategy || diagnostics?.active_strategy_name || cockpitStrategyName || '';

	$: operationalState = (() => {
		const rawStatus = String(botState?.status ?? '').trim().toUpperCase();
		if (botState?.circuit_breaker_tripped || rawStatus === 'CIRCUIT_BREAKER') {
			return 'CIRCUIT_BREAKER';
		}
		if (botState?.running || botState?.runner_active || rawStatus === 'RUNNING') {
			return 'RUNNING';
		}
		if (
			rawStatus === 'ARMED' ||
			Boolean(cockpit?.global_enabled && activeWorkspace?.enabled && activeSlot?.enabled)
		) {
			return 'ARMED';
		}
		if (resolvedStrategyName) {
			return 'PAUSED';
		}
		return 'STOPPED';
	})();

	$: operationalStateLabel = (() => {
		switch (operationalState) {
			case 'CIRCUIT_BREAKER':
				return tr('monitoring_state_breaker');
			case 'RUNNING':
				return tr('monitoring_state_running');
			case 'ARMED':
				return tr('monitoring_state_armed');
			case 'PAUSED':
				return tr('monitoring_state_paused');
			default:
				return tr('monitoring_state_stopped');
		}
	})();

	$: operationalTone = (() => {
		if (operationalState === 'CIRCUIT_BREAKER') return 'is-alert';
		if (operationalState === 'RUNNING') return 'is-live';
		if (operationalState === 'ARMED') return 'is-armed';
		if (operationalState === 'PAUSED') return 'is-warm';
		return 'is-idle';
	})();

	$: heroSummaryText = (() => {
		if (botState?.error_message) return botState.error_message;
		if (operationalState === 'CIRCUIT_BREAKER') return tr('monitoring_summary_breaker');
		if (!botState?.tws_connected) return tr('monitoring_summary_disconnected');
		if (operationalState === 'RUNNING') return tr('monitoring_summary_running');
		if (operationalState === 'ARMED') return tr('monitoring_summary_armed');
		return tr('monitoring_summary_paused');
	})();

	$: resolvedAccount =
		botState?.account_id || diagnostics?.account || diagnostics?.startup?.account || '—';
	$: resolvedMode = (diagnostics?.startup?.trading_mode ?? 'paper').toUpperCase();
	$: resolvedEndpoint = diagnostics?.startup
		? `${diagnostics.startup.ib_host}:${diagnostics.startup.ib_port}`
		: '—';
	$: heroTimestamp = formatTimestamp(botState?.last_heartbeat ?? botState?.last_update, _lang);

	// Panel 1: Portfolio & Currency Readiness + Breaker Headroom
	$: dailyLossLimit = numberValue(botState?.daily_loss_limit) ?? 0;
	$: dailyPnlVal = numberValue(botState?.daily_pnl) ?? 0;
	$: dailyLossUsed = dailyPnlVal < 0 ? Math.abs(dailyPnlVal) : 0;
	$: breakerUsedPct =
		dailyLossLimit > 0 ? Math.min(100, (dailyLossUsed / dailyLossLimit) * 100) : 0;
	$: breakerRemaining = dailyLossLimit > 0 ? Math.max(0, dailyLossLimit - dailyLossUsed) : 0;
	$: breakerNearBreach = dailyLossLimit > 0 && breakerUsedPct >= 90;

	// Panel 2: Open Positions
	$: openPositions = Array.isArray(botState?.open_positions) ? botState.open_positions : [];

	// Panel 3: Working Orders & Dry-Run Scanner
	$: openOrders = Array.isArray(botState?.open_orders) ? botState.open_orders : [];
	$: lastDryRunSummary =
		botState?.last_dry_run && typeof botState.last_dry_run === 'object'
			? (botState.last_dry_run as Record<string, unknown>)
			: null;
	$: dryRunSubscriptionsCount = dryRunResult
		? Array.isArray(dryRunResult.subscriptions)
			? dryRunResult.subscriptions.length
			: 0
		: numberValue(lastDryRunSummary?.subscriptions) ?? 0;
	$: dryRunPlannedOrders = Array.isArray(dryRunResult?.planned_orders)
		? dryRunResult.planned_orders
		: [];
	$: dryRunPlannedCount = dryRunResult
		? dryRunPlannedOrders.length
		: numberValue(lastDryRunSummary?.planned_orders) ?? 0;

	// Panel 4: Session, Broker Alerts & Execution Ledger
	$: sessionStatus = botState?.session_status || diagnostics?.session_status || '—';
	$: lastDisconnectAt = botState?.last_disconnect_at ?? diagnostics?.runtime?.last_disconnect_at ?? null;
	$: lastDisconnectReason =
		botState?.last_disconnect_reason ?? diagnostics?.runtime?.last_disconnect_reason ?? null;
	$: brokerAlerts = ((diagnostics?.broker_alerts?.length
		? diagnostics.broker_alerts
		: botState?.broker_alerts) ?? []) as BrokerAlert[];

	$: combinedTrades = (() => {
		const fromState = Array.isArray(botState?.recent_trades) ? botState.recent_trades : [];
		const merged: Array<Record<string, unknown>> = [];
		const seen = new Set<string>();
		for (const item of [...recentTradesApi, ...fromState]) {
			const rec = item as unknown as Record<string, unknown>;
			const idPart = stringValue(rec.id ?? rec.order_id, '');
			const tsPart = stringValue(rec.created_at ?? rec.closed_at, '');
			const symPart = stringValue(rec.symbol ?? rec.instrument_id, '');
			const key = `${idPart}|${symPart}|${tsPart}`;
			if (!seen.has(key)) {
				seen.add(key);
				merged.push(rec);
			}
		}
		return merged;
	})();

	async function loadMonitoring(force = false) {
		if (force) refreshing = true;
		message = '';
		try {
			const [nextDiagnostics, , nextCockpit, nextTrades] = await Promise.all([
				getDiagnostics(force),
				refreshRuntimeState(force),
				getCockpit(force),
				getRecentTrades()
			]);
			diagnostics = nextDiagnostics;
			cockpit = nextCockpit;
			recentTradesApi = Array.isArray(nextTrades) ? nextTrades : [];
		} catch (err) {
			message = formatApiError(err);
		} finally {
			loading = false;
			refreshing = false;
		}
	}

	async function refreshMonitoringTick() {
		try {
			const [nextDiagnostics, , nextTrades] = await Promise.all([
				getDiagnostics(true),
				refreshRuntimeState(true),
				getRecentTrades()
			]);
			diagnostics = nextDiagnostics;
			recentTradesApi = Array.isArray(nextTrades) ? nextTrades : [];
		} catch {
			// background tick fails silently
		}
	}

	async function runDryRun() {
		runningDryRun = true;
		message = '';
		try {
			const result = await dryRunBot();
			dryRunResult = result;
			if (result?.state) {
				setRuntimeState(result.state);
			}
			diagnostics = await getDiagnostics(true);
		} catch (err) {
			message = formatApiError(err);
		} finally {
			runningDryRun = false;
		}
	}

	async function handleResetBreaker() {
		resettingBreaker = true;
		message = '';
		try {
			await resetCircuitBreaker();
			await loadMonitoring(true);
		} catch (err) {
			message = formatApiError(err);
		} finally {
			resettingBreaker = false;
		}
	}

	onMount(() => {
		void loadMonitoring(true);
		pollTimer = setInterval(() => {
			void refreshMonitoringTick();
		}, 5000);
	});

	onDestroy(() => {
		if (pollTimer) {
			clearInterval(pollTimer);
			pollTimer = null;
		}
	});

	function numberValue(value: unknown): number | null {
		if (typeof value === 'number' && Number.isFinite(value)) return value;
		if (typeof value === 'string' && value.trim()) {
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

	function formatTimestamp(value?: string | null, lang?: string): string {
		void lang;
		if (!value) return tr('watchlist_never_refreshed');
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) return value;
		return new Intl.DateTimeFormat(undefined, { dateStyle: 'short', timeStyle: 'short' }).format(
			date
		);
	}

	function formatTimeShort(value: unknown): string {
		const raw = stringValue(value, '');
		if (!raw) return '—';
		const date = new Date(raw);
		if (Number.isNaN(date.getTime())) return raw;
		return new Intl.DateTimeFormat(undefined, {
			hour: '2-digit',
			minute: '2-digit',
			second: '2-digit'
		}).format(date);
	}

	function formatMoney(value: unknown, currency = 'USD'): string {
		const amount = numberValue(value);
		if (amount === null) return '—';
		const ccy = currency && currency.trim() ? currency.trim().toUpperCase() : 'USD';
		try {
			return new Intl.NumberFormat(undefined, {
				style: 'currency',
				currency: ccy,
				maximumFractionDigits: 2
			}).format(amount);
		} catch {
			return `${amount.toFixed(2)} ${ccy}`;
		}
	}

	function formatSignedMoney(value: unknown, currency = 'USD'): string {
		const amount = numberValue(value);
		if (amount === null) return '—';
		const formatted = formatMoney(Math.abs(amount), currency);
		if (amount > 0) return `+${formatted}`;
		if (amount < 0) return `-${formatted}`;
		return formatted;
	}

	function formatCompactNumber(value: unknown): string {
		const amount = numberValue(value);
		if (amount === null) return '—';
		return new Intl.NumberFormat(undefined, { maximumFractionDigits: 4 }).format(amount);
	}

	function formatPercent(value: unknown): string {
		const amount = numberValue(value);
		if (amount === null) return '—';
		const sign = amount > 0 ? '+' : '';
		return `${sign}${amount.toFixed(2)}%`;
	}

	function pnlClass(value: unknown): string {
		const amount = numberValue(value);
		if (amount === null || amount === 0) return '';
		return amount > 0 ? 'is-positive' : 'is-negative';
	}
</script>

<div class="cockpit-page monitoring-page">
	<div class="cockpit-page-header monitoring-page-header">
		<div class="monitoring-heading-block">
			<span class="monitoring-kicker">{tr('monitoring_kicker')}</span>
			<h1 class="heading">
				<span class="heading-icon"><Activity size={20} strokeWidth={1.7} /></span>
				{tr('monitoring')}
			</h1>
		</div>
		<div class="cockpit-top-actions">
			<button
				class="secondary"
				on:click={() => loadMonitoring(true)}
				disabled={refreshing || runningDryRun}
			>
				<RefreshCcw size={14} strokeWidth={1.8} />
				<span>{tr('reload')}</span>
			</button>
		</div>
	</div>

	{#if message}
		<p class="monitoring-error-banner">{message}</p>
	{/if}

	{#if loading && !diagnostics && !botState}
		<div class="card">
			<p class="muted">{tr('loading_chart_data')}</p>
		</div>
	{:else}
		<section class="card monitoring-hero">
			<div class="monitoring-hero-main">
				<div class="monitoring-hero-status-row">
					<span class={`monitoring-hero-status ${operationalTone}`}>{operationalStateLabel}</span>
					<span class="monitoring-hero-mode font-mono">{resolvedMode}</span>
				</div>
				<h2>{resolvedStrategyName || tr('monitoring_no_strategy')}</h2>
				<p>{heroSummaryText}</p>
			</div>
			<div class="monitoring-hero-strip">
				<div class="monitoring-hero-chip">
					<span class="monitoring-stat-label">{tr('monitoring_account_chip')}</span>
					<strong class="font-mono">{resolvedAccount}</strong>
				</div>
				<div class="monitoring-hero-chip">
					<span class="monitoring-stat-label">{tr('monitoring_mode_chip')}</span>
					<strong class="font-mono">{resolvedMode}</strong>
				</div>
				<div class="monitoring-hero-chip">
					<span class="monitoring-stat-label">{tr('monitoring_endpoint_chip')}</span>
					<strong class="font-mono">{resolvedEndpoint}</strong>
				</div>
				<div class="monitoring-hero-chip">
					<span class="monitoring-stat-label">{tr('monitoring_heartbeat_chip')}</span>
					<strong class="font-mono">{heroTimestamp}</strong>
				</div>
			</div>
		</section>

		<div class="monitoring-panels-grid">
			<!-- Panel 1: Live Portfolio & Currency Readiness -->
			<section class="card monitoring-panel" data-testid="monitoring-panel-1">
				<div class="monitoring-card-head">
					<div>
						<span class="monitoring-card-kicker">{tr('monitoring_panel1_kicker')}</span>
						<h2>
							<Wallet size={16} strokeWidth={1.7} />
							<span>{tr('monitoring_panel1_title')}</span>
						</h2>
					</div>
					<span class="monitoring-fxconv-tag font-mono">FXCONV</span>
				</div>

				<div class="monitoring-kpi-row">
					<div class="monitoring-stat-tile monitoring-stat-tile-strong">
						<span class="monitoring-stat-label">{tr('net_liquidation')}</span>
						<strong class="font-mono">{formatMoney(botState?.equity)}</strong>
					</div>
					<div class="monitoring-stat-tile">
						<span class="monitoring-stat-label">{tr('monitoring_buying_power')}</span>
						<strong class="font-mono">{formatMoney(botState?.buying_power)}</strong>
					</div>
				</div>

				<div class="monitoring-currency-row">
					<div class="monitoring-mini-stat">
						<span class="monitoring-stat-label">{tr('monitoring_cash_usd')}</span>
						<strong class="font-mono">{formatMoney(botState?.cash_usd, 'USD')}</strong>
						<span class="monitoring-ccy-code font-mono">USD</span>
					</div>
					<div class="monitoring-mini-stat">
						<span class="monitoring-stat-label">{tr('monitoring_cash_cad')}</span>
						<strong class="font-mono">{formatMoney(botState?.cash_cad, 'CAD')}</strong>
						<span class="monitoring-ccy-code font-mono">CAD</span>
					</div>
					<div class="monitoring-mini-stat">
						<span class="monitoring-stat-label">{tr('monitoring_cash_base')}</span>
						<strong class="font-mono">{formatMoney(botState?.cash_base, 'USD')}</strong>
						<span class="monitoring-ccy-code font-mono">BASE</span>
					</div>
				</div>
				<p class="monitoring-subnote">{tr('monitoring_fxconv_note')}</p>

				<div class="monitoring-pnl-row">
					<div class="monitoring-mini-stat">
						<span class="monitoring-stat-label">{tr('monitoring_realized_pnl')}</span>
						<strong class={`font-mono ${pnlClass(botState?.realized_pnl)}`}>
							{formatSignedMoney(botState?.realized_pnl)}
						</strong>
					</div>
					<div class="monitoring-mini-stat">
						<span class="monitoring-stat-label">{tr('unrealized_pnl')}</span>
						<strong class={`font-mono ${pnlClass(botState?.unrealized_pnl)}`}>
							{formatSignedMoney(botState?.unrealized_pnl)}
						</strong>
					</div>
					<div class="monitoring-mini-stat">
						<span class="monitoring-stat-label">{tr('daily_pnl')}</span>
						<strong class={`font-mono ${pnlClass(botState?.daily_pnl)}`}>
							{formatSignedMoney(botState?.daily_pnl)}
						</strong>
						<span class="monitoring-stat-meta font-mono">
							{formatPercent(botState?.daily_pnl_percent)}
						</span>
					</div>
				</div>

				<div
					class="monitoring-breaker-box"
					class:is-tripped={botState?.circuit_breaker_tripped}
					class:is-near={breakerNearBreach && !botState?.circuit_breaker_tripped}
				>
					<div class="monitoring-breaker-head">
						<span class="monitoring-stat-label">{tr('monitoring_breaker_headroom')}</span>
						{#if botState?.circuit_breaker_tripped}
							<span class="monitoring-mini-pill is-alert">{tr('cockpit_hud_breaker_tripped')}</span>
						{:else if dailyLossLimit > 0}
							<span class="monitoring-stat-meta font-mono">
								{tr('monitoring_breaker_used', { pct: breakerUsedPct.toFixed(0) })}
							</span>
						{/if}
					</div>

					{#if dailyLossLimit > 0}
						<div class="monitoring-gauge-track" role="progressbar" aria-valuenow={Math.round(breakerUsedPct)} aria-valuemin={0} aria-valuemax={100}>
							<div
								class="monitoring-gauge-fill"
								class:is-tripped={botState?.circuit_breaker_tripped}
								class:is-near={breakerNearBreach && !botState?.circuit_breaker_tripped}
								style={`width: ${breakerUsedPct.toFixed(1)}%`}
							></div>
						</div>
					{/if}
					<div class="monitoring-breaker-foot">
						{#if dailyLossLimit > 0}
							<span class="monitoring-stat-meta font-mono">
								{tr('monitoring_breaker_remaining', {
									remaining: formatMoney(breakerRemaining),
									limit: formatMoney(dailyLossLimit)
								})}
							</span>
						{:else}
							<p class="monitoring-subnote">{tr('monitoring_breaker_no_limit')}</p>
						{/if}
						{#if botState?.circuit_breaker_tripped}
							<button
								class="danger monitoring-reset-btn"
								on:click={handleResetBreaker}
								disabled={resettingBreaker}
							>
								<ShieldAlert size={13} strokeWidth={1.8} />
								<span>{tr('cockpit_hud_reset_breaker')}</span>
							</button>
						{/if}
					</div>
				</div>
			</section>

			<!-- Panel 2: Live Open Positions ($$$$) -->
			<section class="card monitoring-panel" data-testid="monitoring-panel-2">
				<div class="monitoring-card-head">
					<div>
						<span class="monitoring-card-kicker">{tr('monitoring_panel2_kicker')}</span>
						<h2>
							<DollarSign size={16} strokeWidth={1.7} />
							<span>{tr('monitoring_panel2_title')}</span>
						</h2>
					</div>
					<span class="monitoring-mini-pill font-mono">
						{tr('monitoring_positions_count', { count: openPositions.length })}
					</span>
				</div>

				{#if openPositions.length > 0}
					<div class="monitoring-table-wrap">
						<table class="table monitoring-table">
							<thead>
								<tr>
									<th>{tr('monitoring_col_instrument')}</th>
									<th>{tr('monitoring_col_venue')}</th>
									<th>{tr('monitoring_col_ccy')}</th>
									<th class="num">{tr('monitoring_col_qty')}</th>
									<th class="num">{tr('monitoring_col_avg_cost')}</th>
									<th class="num">{tr('monitoring_col_mkt_price')}</th>
									<th class="num">{tr('monitoring_col_mkt_value')}</th>
									<th class="num">{tr('monitoring_col_unrealized')}</th>
								</tr>
							</thead>
							<tbody>
								{#each openPositions as pos}
									<tr>
										<td class="font-mono">
											<strong>{pos.instrument_id || pos.symbol}</strong>
										</td>
										<td class="font-mono muted">{pos.venue || '—'}</td>
										<td class="font-mono">{pos.currency || 'USD'}</td>
										<td class="num font-mono">{formatCompactNumber(pos.position)}</td>
										<td class="num font-mono">{formatMoney(pos.avg_cost, pos.currency || 'USD')}</td>
										<td class="num font-mono">
											{formatMoney(pos.market_price, pos.currency || 'USD')}
										</td>
										<td class="num font-mono">
											{formatMoney(pos.market_value, pos.currency || 'USD')}
										</td>
										<td class={`num font-mono ${pnlClass(pos.unrealized_pnl)}`}>
											{formatSignedMoney(pos.unrealized_pnl, pos.currency || 'USD')}
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				{:else}
					<p class="cockpit-runtime-empty">{tr('monitoring_no_positions')}</p>
				{/if}
			</section>

			<!-- Panel 3: Active / Working Orders & Live Dry-Run Scanner -->
			<section class="card monitoring-panel" data-testid="monitoring-panel-3">
				<div class="monitoring-card-head">
					<div>
						<span class="monitoring-card-kicker">{tr('monitoring_panel3_kicker')}</span>
						<h2>
							<TrendingUp size={16} strokeWidth={1.7} />
							<span>{tr('monitoring_panel3_title')}</span>
						</h2>
					</div>
					<button
						on:click={runDryRun}
						disabled={runningDryRun || refreshing}
					>
						<Play size={14} strokeWidth={1.8} />
						<span>
							{runningDryRun ? tr('monitoring_running_dry_run') : tr('monitoring_run_dry_run')}
						</span>
					</button>
				</div>

				<div class="monitoring-subsection">
					<div class="monitoring-subhead">
						<span class="monitoring-stat-label">{tr('monitoring_working_orders')}</span>
						<span class="monitoring-stat-meta font-mono">{openOrders.length}</span>
					</div>
					{#if openOrders.length > 0}
						<div class="monitoring-table-wrap">
							<table class="table monitoring-table">
								<thead>
									<tr>
										<th>{tr('monitoring_col_order_id')}</th>
										<th>{tr('monitoring_col_instrument')}</th>
										<th>{tr('side')}</th>
										<th>{tr('monitoring_col_type')}</th>
										<th class="num">{tr('monitoring_col_fill')}</th>
										<th class="num">{tr('monitoring_col_limit')}</th>
										<th>{tr('status')}</th>
									</tr>
								</thead>
								<tbody>
									{#each openOrders as order}
										<tr>
											<td class="font-mono">#{order.order_id}</td>
											<td class="font-mono">
												<strong>{order.instrument_id || order.symbol}</strong>
											</td>
											<td class="font-mono">{order.action}</td>
											<td class="font-mono">{order.order_type}</td>
											<td class="num font-mono">
												{formatCompactNumber(order.filled_qty)} / {formatCompactNumber(order.total_qty)}
											</td>
											<td class="num font-mono">{order.limit_price > 0 ? formatMoney(order.limit_price) : '—'}</td>
											<td>
												<span class="monitoring-mini-pill">
													{order.status}{order.outside_rth ? ` · ${tr('monitoring_col_outside_rth')}` : ''}
												</span>
											</td>
										</tr>
									{/each}
								</tbody>
							</table>
						</div>
					{:else}
						<p class="cockpit-runtime-empty">{tr('monitoring_no_working_orders')}</p>
					{/if}
				</div>

				<div class="monitoring-subsection">
					<div class="monitoring-subhead">
						<span class="monitoring-stat-label">{tr('monitoring_dryrun_section')}</span>
						<span class="monitoring-stat-meta font-mono">
							{tr('monitoring_dryrun_subscriptions')}: {dryRunSubscriptionsCount} · {tr('monitoring_dryrun_planned')}: {dryRunPlannedCount}
						</span>
					</div>
					{#if dryRunResult}
						{#if dryRunPlannedOrders.length > 0}
							<div class="cockpit-activity-list">
								{#each dryRunPlannedOrders as planned}
									{@const plannedSym = stringValue(planned.instrument_id ?? planned.symbol)}
									{@const plannedAction = stringValue(planned.action ?? planned.side)}
									{@const plannedQty = planned.quantity ?? planned.qty}
									{@const plannedReason =
										stringValue(planned.gate_reason, '') ||
										stringValue(planned.reason, '') ||
										stringValue(planned.rule, '')}
									{@const canExec = planned.can_execute !== false}
									<div class="cockpit-activity-row">
										<div class="cockpit-activity-main">
											<strong class="font-mono">{plannedSym}</strong>
											<span class="cockpit-ticker-subtle font-mono">
												{plannedAction} · {formatCompactNumber(plannedQty)} @ {formatMoney(planned.price)}
											</span>
										</div>
										<div class="cockpit-activity-side">
											<span class={`monitoring-mini-pill ${canExec ? 'is-live' : 'is-warm'}`}>
												{canExec ? tr('monitoring_dryrun_ready') : tr('monitoring_dryrun_blocked')}
											</span>
											{#if plannedReason}
												<span class="cockpit-ticker-subtle">{plannedReason}</span>
											{/if}
										</div>
									</div>
								{/each}
							</div>
						{:else}
							<p class="cockpit-runtime-empty">{tr('monitoring_dryrun_no_planned')}</p>
						{/if}
					{:else}
						<p class="cockpit-runtime-empty">{tr('monitoring_dryrun_empty')}</p>
					{/if}
				</div>
			</section>

			<!-- Panel 4: Execution & Gate-Diagnostics Ledger -->
			<section class="card monitoring-panel" data-testid="monitoring-panel-4">
				<div class="monitoring-card-head">
					<div>
						<span class="monitoring-card-kicker">{tr('monitoring_panel4_kicker')}</span>
						<h2>
							<Clock size={16} strokeWidth={1.7} />
							<span>{tr('monitoring_panel4_title')}</span>
						</h2>
					</div>
					<span class="monitoring-mini-pill is-live font-mono">{sessionStatus}</span>
				</div>

				<div class="monitoring-session-row">
					<div class="monitoring-mini-stat">
						<span class="monitoring-stat-label">{tr('monitoring_session_status')}</span>
						<strong class="font-mono">{sessionStatus}</strong>
					</div>
					{#if lastDisconnectReason || lastDisconnectAt}
						<div class="monitoring-mini-stat">
							<span class="monitoring-stat-label">{tr('monitoring_last_disconnect')}</span>
							<strong class="font-mono">{lastDisconnectReason || '—'}</strong>
						</div>
					{/if}
				</div>

				<div class="monitoring-subsection">
					<div class="monitoring-subhead">
						<span class="monitoring-stat-label">{tr('monitoring_broker_alerts')}</span>
						<span class="monitoring-stat-meta font-mono">{brokerAlerts.length}</span>
					</div>
					{#if brokerAlerts.length > 0}
						<div class="monitoring-alerts-list">
							{#each brokerAlerts.slice(0, 6) as alert}
								{@const sev = String(alert.severity ?? '').toLowerCase()}
								<div
									class="monitoring-alert-item"
									class:is-error={sev === 'error'}
									class:is-warn={sev === 'warning' || sev === 'warn'}
								>
									<div class="monitoring-alert-top">
										<span class="font-mono monitoring-alert-code">
											<AlertTriangle size={13} strokeWidth={1.8} />
											[Code {alert.code}]
										</span>
										<span class="monitoring-alert-msg">{alert.message}</span>
									</div>
									{#if alert.hint}
										<p class="monitoring-alert-hint">{alert.hint}</p>
									{/if}
								</div>
							{/each}
						</div>
					{:else}
						<p class="cockpit-runtime-empty">{tr('monitoring_no_broker_alerts')}</p>
					{/if}
				</div>

				<div class="monitoring-subsection">
					<div class="monitoring-subhead">
						<span class="monitoring-stat-label">{tr('monitoring_executions_title')}</span>
						<span class="monitoring-stat-meta font-mono">{combinedTrades.length}</span>
					</div>
					{#if combinedTrades.length > 0}
						<div class="monitoring-table-wrap">
							<table class="table monitoring-table">
								<thead>
									<tr>
										<th>{tr('symbol')}</th>
										<th>{tr('side')}</th>
										<th class="num">{tr('quantity_short')}</th>
										<th class="num">{tr('price')}</th>
										<th>{tr('status')}</th>
										<th>{tr('monitoring_col_mode')}</th>
										<th>{tr('monitoring_col_oca')}</th>
										<th class="num">{tr('monitoring_col_time')}</th>
									</tr>
								</thead>
								<tbody>
									{#each combinedTrades.slice(0, 12) as trade}
										<tr>
											<td class="font-mono">
												<strong>{stringValue(trade.symbol ?? trade.instrument_id)}</strong>
											</td>
											<td class="font-mono">{stringValue(trade.action ?? trade.side)}</td>
											<td class="num font-mono">{formatCompactNumber(trade.quantity)}</td>
											<td class="num font-mono">{formatMoney(trade.price)}</td>
											<td class="font-mono">{stringValue(trade.status)}</td>
											<td class="font-mono">{stringValue(trade.mode)}</td>
											<td class="font-mono muted">{stringValue(trade.oca_group)}</td>
											<td class="num font-mono muted">
												{formatTimeShort(trade.created_at ?? trade.closed_at)}
											</td>
										</tr>
									{/each}
								</tbody>
							</table>
						</div>
					{:else}
						<p class="cockpit-runtime-empty">{tr('monitoring_no_executions')}</p>
					{/if}
				</div>
			</section>
		</div>
	{/if}
</div>

<style>
	.monitoring-page {
		height: auto;
		min-height: 100%;
		overflow-x: clip;
		overflow-y: visible;
		display: flex;
		flex-direction: column;
		gap: 14px;
	}

	.monitoring-page-header {
		align-items: center;
	}

	.monitoring-heading-block {
		display: grid;
		gap: 4px;
	}

	.monitoring-error-banner {
		margin: 0;
		padding: 10px 14px;
		border-radius: 10px;
		border: 1px solid rgba(239, 68, 68, 0.45);
		background: rgba(127, 29, 29, 0.25);
		color: var(--sem-bear-text);
		font-size: 13px;
	}

	.monitoring-hero {
		display: grid;
		grid-template-columns: minmax(0, 1.15fr) minmax(300px, 0.85fr);
		gap: 14px;
		padding: 16px 18px;
		border-color: var(--border-highlight);
		background: linear-gradient(135deg, rgba(38, 60, 138, 0.16), var(--bg-card));
	}

	.monitoring-hero-status-row {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
	}

	.monitoring-hero-mode {
		font-size: 11px;
		font-weight: 700;
		padding: 4px 8px;
		border-radius: 999px;
		border: 1px solid var(--border-highlight);
		background: var(--bg-surface);
		color: var(--cobalt-cyan);
	}

	.monitoring-hero-status.is-armed {
		border-color: rgba(56, 189, 248, 0.45);
		background: rgba(29, 99, 237, 0.2);
		color: var(--cobalt-cyan);
	}

	.monitoring-hero-main h2 {
		margin: 0;
		max-width: none;
		font-size: 22px;
		line-height: 1.15;
		letter-spacing: -0.02em;
	}

	.monitoring-hero-main p {
		margin: 0;
		font-size: 13px;
		color: var(--text-secondary);
	}

	.monitoring-panels-grid {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 14px;
		align-items: start;
	}

	.monitoring-panel {
		display: grid;
		gap: 12px;
		padding: 16px;
		border: 1px solid var(--border-subtle);
		background: var(--bg-card);
		min-width: 0;
	}

	.monitoring-card-head h2 {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		font-size: 16px;
		margin: 4px 0 0;
	}

	.monitoring-fxconv-tag {
		font-size: 11px;
		font-weight: 700;
		padding: 4px 8px;
		border-radius: 6px;
		border: 1px solid rgba(56, 189, 248, 0.35);
		background: rgba(56, 189, 248, 0.1);
		color: var(--cobalt-cyan);
	}

	.monitoring-kpi-row {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 10px;
	}

	.monitoring-currency-row,
	.monitoring-pnl-row {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		gap: 10px;
	}

	.monitoring-ccy-code {
		font-size: 11px;
		color: var(--cobalt-cyan);
		font-weight: 700;
	}

	.monitoring-subnote {
		margin: 0;
		font-size: 12px;
		color: var(--text-secondary);
	}

	.is-positive {
		color: var(--sem-bull-text);
	}

	.is-negative {
		color: var(--sem-bear-text);
	}

	.monitoring-breaker-box {
		display: grid;
		gap: 8px;
		padding: 12px;
		border-radius: 10px;
		border: 1px solid var(--border-subtle);
		background: var(--bg-surface);
	}

	.monitoring-breaker-box.is-near {
		border-color: var(--sem-warn);
	}

	.monitoring-breaker-box.is-tripped {
		border-color: var(--sem-bear);
		background: rgba(239, 68, 68, 0.1);
	}

	.monitoring-breaker-head,
	.monitoring-breaker-foot {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
		flex-wrap: wrap;
	}

	.monitoring-gauge-track {
		width: 100%;
		height: 8px;
		border-radius: 999px;
		background: var(--bg-void);
		border: 1px solid var(--border-subtle);
		overflow: hidden;
	}

	.monitoring-gauge-fill {
		height: 100%;
		border-radius: 999px;
		background: var(--sem-bull);
		transition: width 0.2s ease;
	}

	.monitoring-gauge-fill.is-near {
		background: var(--sem-warn);
	}

	.monitoring-gauge-fill.is-tripped {
		background: var(--sem-bear);
	}

	.monitoring-reset-btn {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 5px 10px;
		font-size: 12px;
	}

	.monitoring-subsection {
		display: grid;
		gap: 8px;
	}

	.monitoring-subhead {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
	}

	.monitoring-table-wrap {
		overflow-x: auto;
		border: 1px solid var(--border-subtle);
		border-radius: 10px;
		background: var(--bg-surface);
	}

	.monitoring-table {
		font-size: 12px;
	}

	.monitoring-table th,
	.monitoring-table td {
		padding: 7px 9px;
		white-space: nowrap;
	}

	.monitoring-table th.num,
	.monitoring-table td.num {
		text-align: right;
	}

	.monitoring-session-row {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
		gap: 10px;
	}

	.monitoring-alerts-list {
		display: grid;
		gap: 8px;
	}

	.monitoring-alert-item {
		display: grid;
		gap: 4px;
		padding: 10px 12px;
		border-radius: 10px;
		border: 1px solid var(--border-highlight);
		background: var(--bg-surface);
		font-size: 12px;
	}

	.monitoring-alert-item.is-warn {
		border-color: rgba(250, 160, 49, 0.45);
		background: rgba(250, 160, 49, 0.08);
	}

	.monitoring-alert-item.is-error {
		border-color: rgba(239, 68, 68, 0.45);
		background: rgba(239, 68, 68, 0.1);
	}

	.monitoring-alert-top {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
	}

	.monitoring-alert-code {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		font-weight: 700;
		color: var(--sem-warn-text);
	}

	.monitoring-alert-item.is-error .monitoring-alert-code {
		color: var(--sem-bear-text);
	}

	.monitoring-alert-msg {
		font-weight: 600;
		color: var(--text-primary);
	}

	.monitoring-alert-hint {
		margin: 0;
		color: var(--text-secondary);
		line-height: 1.4;
	}

	@media (max-width: 1120px) {
		.monitoring-panels-grid,
		.monitoring-hero {
			grid-template-columns: 1fr;
		}
	}

	@media (max-width: 720px) {
		.monitoring-kpi-row,
		.monitoring-currency-row,
		.monitoring-pnl-row {
			grid-template-columns: 1fr;
		}
	}
</style>
