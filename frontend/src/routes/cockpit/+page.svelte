<script lang="ts">
	import { onMount } from 'svelte';
	import {
		applyStrategyPreset,
		formatApiError,
		getCockpit,
		getWatchlist,
		getWatchlistById,
		getWatchlists,
		moveStrategyOrder,
		refreshWatchlistFeed,
		replaceWatchlist,
		resetCircuitBreaker,
		saveCockpit,
		toggleStrategyPin,
		type CockpitState,
		type CockpitStrategySlot,
		type CockpitStrategySummary,
		type CockpitWorkspace,
		type WatchlistGroup,
		type WatchlistItem,
		type WatchlistResponse,
		type WatchlistSummary
	} from '$lib/api';
	import { language, t } from '$lib/i18n';
	import { refreshRuntimeState, runtimeState } from '$lib/stores/runtime';
	import { launchTour, openHelpDrawer } from '$lib/stores/onboarding';
	import SlideToArm from '$lib/components/SlideToArm.svelte';
	import { Compass, LayoutGrid, Power, RefreshCcw, Search, ShieldAlert, Sparkles, Tags, X } from 'lucide-svelte';

	let cockpit: CockpitState | null = null;
	let masterWatchlist: WatchlistResponse | null = null;
	let availableWatchlists: WatchlistSummary[] = [];
	let activeWatchlistId = 'cobalt-telegram';
	let saving = false;
	let message = '';
	let collapsedGroups = new Set<string>();
	let tickerFilter = '';

	$: botState = $runtimeState;
	$: pnl = botState?.daily_pnl ?? 0;
	$: pnlPct = botState?.daily_pnl_percent ?? 0;
	$: lossLimit = botState?.daily_loss_limit ?? 0;
	$: nearBreach = lossLimit > 0 && pnl <= -lossLimit * 0.9;

	$: _lang = $language;
	$: tr = (key: string, params?: Record<string, string | number>) => {
		void _lang;
		return t(key, params);
	};
	$: activeWorkspace =
		cockpit?.workspaces.find((workspace) => workspace.id === cockpit?.active_workspace_id) ??
		cockpit?.workspaces[0] ??
		null;
	$: displayGroups = masterWatchlist?.groups ?? [];
	$: normalizedTickerFilter = tickerFilter.trim().toLowerCase();
	$: filteredGroups = displayGroups
		.map((group) => ({
			group,
			items: !normalizedTickerFilter
				? group.items
				: group.items.filter((item) => matchesTickerFilter(item, normalizedTickerFilter))
		}))
		.filter(({ items }) => items.length > 0);
	$: activeTickerCount = displayGroups.reduce(
		(count, group) => count + group.items.filter((item) => item.enabled).length,
		0
	);
	$: totalTickerCount = displayGroups.reduce((count, group) => count + group.items.length, 0);
	$: activeStrategySlot = activeWorkspace?.strategy_slots?.[0] ?? null;
	$: activeStrategyId = activeStrategySlot?.strategy_id || botState?.active_strategy_id || '';
	$: activeStrategySummary =
		cockpit?.strategy_library.find((strategy) => strategy.id === activeStrategyId) ?? null;
	$: sortedStrategies = [...(cockpit?.strategy_library ?? [])].sort((a, b) => {
		if (Boolean(a.pinned) !== Boolean(b.pinned)) return a.pinned ? -1 : 1;
		const orderA = typeof a.sort_order === 'number' ? a.sort_order : 0;
		const orderB = typeof b.sort_order === 'number' ? b.sort_order : 0;
		if (orderA !== orderB) return orderA - orderB;
		return a.name.localeCompare(b.name);
	});

	function cloneGroups(groups: WatchlistGroup[]): WatchlistGroup[] {
		return groups.map((group) => ({
			...group,
			items: group.items.map((item) => ({ ...item }))
		}));
	}

	function buildExclusiveStrategyWorkspaces(
		targetWorkspaceId: string,
		targetSlotId: string,
		nextEnabled: boolean,
		nextStrategyId?: string | null
	): CockpitWorkspace[] {
		if (!cockpit) return [];
		return cloneWorkspaces(cockpit.workspaces).map((workspace) => ({
			...workspace,
			strategy_slots: workspace.strategy_slots.map((candidate) => {
				const strategyId =
					workspace.id === targetWorkspaceId && candidate.id === targetSlotId
						? typeof nextStrategyId === 'undefined'
							? candidate.strategy_id ?? null
							: nextStrategyId
						: candidate.strategy_id ?? null;
				return {
					...candidate,
					strategy_id: strategyId,
					enabled: Boolean(
						workspace.id === targetWorkspaceId &&
							candidate.id === targetSlotId &&
							nextEnabled &&
							strategyId
					)
				};
			})
		}));
	}

	function cloneWorkspaces(workspaces: CockpitWorkspace[]): CockpitWorkspace[] {
		return workspaces.map((workspace) => ({
			...workspace,
			strategy_slots: workspace.strategy_slots.map((slot) => ({ ...slot }))
		}));
	}

	async function loadCockpit(force = false) {
		try {
			const [nextCockpit, nextWatchlists, nextWatchlist] = await Promise.all([
				getCockpit(force),
				getWatchlists(),
				getWatchlist(force)
			]);
			cockpit = nextCockpit;
			availableWatchlists = (nextWatchlists ?? []) as WatchlistSummary[];
			masterWatchlist = nextWatchlist;
			if (masterWatchlist?.id) {
				activeWatchlistId = masterWatchlist.id;
			} else if (availableWatchlists.length > 0) {
				activeWatchlistId = availableWatchlists[0].id;
			}
		} catch (err) {
			message = formatApiError(err);
		}
	}

	onMount(() => {
		void loadCockpit();
	});

	async function switchWatchlist(id: string) {
		try {
			saving = true;
			message = '';
			const nextWatchlist = await getWatchlistById(id, true);
			masterWatchlist = nextWatchlist;
			activeWatchlistId = nextWatchlist.id || id;
		} catch (err) {
			message = formatApiError(err);
		} finally {
			saving = false;
		}
	}

	async function handleRefreshFeed() {
		try {
			saving = true;
			message = '';
			const targetId = masterWatchlist?.id || activeWatchlistId;
			masterWatchlist = await refreshWatchlistFeed(targetId);
			cockpit = await getCockpit(true);
		} catch (err) {
			message = formatApiError(err);
		} finally {
			saving = false;
		}
	}

	async function persist(next: CockpitState) {
		saving = true;
		message = '';
		try {
			cockpit = await saveCockpit({
				global_enabled: next.global_enabled,
				active_workspace_id: next.active_workspace_id,
				workspaces: next.workspaces
			});
			await refreshRuntimeState(true);
		} catch (err) {
			message = formatApiError(err);
		} finally {
			saving = false;
		}
	}

	async function persistMasterGroups(nextGroups: WatchlistGroup[]) {
		if (!masterWatchlist) return;
		saving = true;
		message = '';
		try {
			masterWatchlist = await replaceWatchlist({
				watchlist_id: masterWatchlist.id || activeWatchlistId,
				groups: nextGroups,
				feed: masterWatchlist.feed ?? null
			});
			cockpit = await getCockpit(true);
		} catch (err) {
			message = formatApiError(err);
		} finally {
			saving = false;
		}
	}

	function itemKey(item: Pick<WatchlistItem, 'symbol' | 'exchange'>): string {
		const symbol = item.symbol.trim().toUpperCase();
		const exchange = (item.exchange ?? '').trim().toUpperCase();
		return exchange ? `${symbol}:${exchange}` : symbol;
	}

	function groupEnabled(group: WatchlistGroup): boolean {
		return group.items.length > 0 && group.items.every((item) => item.enabled);
	}

	async function setActiveWorkspace(workspaceId: string) {
		if (!cockpit) return;
		await persist({ ...cockpit, active_workspace_id: workspaceId });
	}

	async function setGlobalEnabled(enabled: boolean) {
		if (!cockpit) return;
		await persist({ ...cockpit, global_enabled: enabled });
	}

	async function setWorkspaceEnabled(enabled: boolean) {
		if (!cockpit || !activeWorkspace) return;
		const nextWorkspaces = cloneWorkspaces(cockpit.workspaces).map((workspace) =>
			workspace.id === activeWorkspace.id ? { ...workspace, enabled } : workspace
		);
		await persist({ ...cockpit, workspaces: nextWorkspaces });
	}

	async function setSlotEnabled(slot: CockpitStrategySlot, enabled: boolean) {
		if (!cockpit || !activeWorkspace) return;
		const nextWorkspaces = buildExclusiveStrategyWorkspaces(
			activeWorkspace.id,
			slot.id,
			enabled,
			slot.strategy_id ?? null
		);
		await persist({ ...cockpit, active_workspace_id: activeWorkspace.id, workspaces: nextWorkspaces });
	}

	async function armStrategy(strategyId: string) {
		if (!cockpit || !activeWorkspace || !strategyId) return;
		saving = true;
		message = '';
		try {
			await applyStrategyPreset(strategyId, activeWorkspace.id, activeStrategySlot?.id || 'slot_1');
			cockpit = await getCockpit(true);
			await refreshRuntimeState(true);
		} catch (err) {
			message = formatApiError(err);
		} finally {
			saving = false;
		}
	}

	async function handleTogglePin(strategyId: string) {
		if (!cockpit) return;
		saving = true;
		message = '';
		try {
			const updatedLibrary = await toggleStrategyPin(strategyId);
			cockpit = { ...cockpit, strategy_library: updatedLibrary };
		} catch (err) {
			message = formatApiError(err);
		} finally {
			saving = false;
		}
	}

	async function handleMoveOrder(strategyId: string, direction: 'up' | 'down') {
		if (!cockpit) return;
		saving = true;
		message = '';
		try {
			const updatedLibrary = await moveStrategyOrder(strategyId, direction);
			cockpit = { ...cockpit, strategy_library: updatedLibrary };
		} catch (err) {
			message = formatApiError(err);
		} finally {
			saving = false;
		}
	}

	function canMoveUp(index: number, list: CockpitStrategySummary[]): boolean {
		return index > 0 && Boolean(list[index - 1].pinned) === Boolean(list[index].pinned);
	}

	function canMoveDown(index: number, list: CockpitStrategySummary[]): boolean {
		return index < list.length - 1 && Boolean(list[index + 1].pinned) === Boolean(list[index].pinned);
	}

	async function setAllTickers(enabled: boolean) {
		if (!masterWatchlist) return;
		const nextGroups = cloneGroups(masterWatchlist.groups).map((group) => ({
			...group,
			items: group.items.map((item) => ({ ...item, enabled }))
		}));
		await persistMasterGroups(nextGroups);
	}

	async function setGroupEnabled(groupId: string, enabled: boolean) {
		if (!masterWatchlist) return;
		const nextGroups = cloneGroups(masterWatchlist.groups).map((group) =>
			group.id === groupId ? { ...group, items: group.items.map((item) => ({ ...item, enabled })) } : group
		);
		await persistMasterGroups(nextGroups);
	}

	async function setTickerEnabled(groupId: string, ticker: WatchlistItem, enabled: boolean) {
		if (!masterWatchlist) return;
		const nextGroups = cloneGroups(masterWatchlist.groups).map((group) => {
			if (group.id !== groupId) return group;
			return {
				...group,
				items: group.items.map((item) => (itemKey(item) === itemKey(ticker) ? { ...item, enabled } : item))
			};
		});
		await persistMasterGroups(nextGroups);
	}

	async function updateTickerSizing(groupId: string, ticker: WatchlistItem, mode: string, value: number) {
		if (!masterWatchlist) return;
		const nextGroups = cloneGroups(masterWatchlist.groups).map((group) => {
			if (group.id !== groupId) return group;
			return {
				...group,
				items: group.items.map((item) => {
					if (itemKey(item) === itemKey(ticker)) {
						return { ...item, size_mode: mode, size_value: value };
					}
					return item;
				})
			};
		});
		await persistMasterGroups(nextGroups);
	}

	async function handleResetBreaker() {
		try {
			await resetCircuitBreaker();
			if (cockpit) {
				await setGlobalEnabled(true);
			}
			await loadCockpit(true);
			await refreshRuntimeState(true);
		} catch (err) {
			message = formatApiError(err);
		}
	}

	function toggleGroup(groupId: string) {
		const next = new Set(collapsedGroups);
		if (next.has(groupId)) next.delete(groupId);
		else next.add(groupId);
		collapsedGroups = next;
	}

	$: strategyName = (strategyId?: string | null) => {
		if (!strategyId || !cockpit) return tr('cockpit_unassigned');
		return cockpit.strategy_library.find((item) => item.id === strategyId)?.name ?? tr('cockpit_unassigned');
	};

	function matchesTickerFilter(item: WatchlistItem, filterValue: string) {
		const haystack = [item.symbol, item.exchange ?? '', item.venue ?? '', item.instrument_id ?? '', item.name ?? '']
			.join(' ')
			.toLowerCase();
		return haystack.includes(filterValue);
	}

	$: formatTimestamp = (value?: string | null) => {
		if (!value) return tr('watchlist_never_refreshed');
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) return value;
		return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(date);
	};
</script>

<div class="cockpit-page">
	<div class="cockpit-page-header">
		<h1 class="heading"><span class="heading-icon"><LayoutGrid size={20} strokeWidth={1.6} /></span>{tr('cockpit')}</h1>
		{#if message}
			<p class="cockpit-message">{message}</p>
		{/if}
		<div class="cockpit-header-actions">
			<button
				type="button"
				class="secondary cockpit-header-btn"
				on:click={() => launchTour(1)}
				title={tr('cockpit_launch_tour')}
			>
				<Compass size={13} strokeWidth={1.8} />
				<span>{tr('cockpit_launch_tour')}</span>
			</button>
			<button
				type="button"
				class="secondary cockpit-header-btn cockpit-help-btn"
				aria-label={tr('help_open_aria')}
				title={tr('help_open_aria')}
				on:click={openHelpDrawer}
			>
				?
			</button>
		</div>
	</div>

	{#if cockpit && activeWorkspace}
		<div class="cockpit-shell">
			<div class="cockpit-header card">
				<div class="cockpit-topbar">
					<div class="cockpit-tabs">
						{#each cockpit.workspaces as workspace}
							<button
								class:active-tab={workspace.id === activeWorkspace.id}
								class="secondary cockpit-tab"
								on:click={() => setActiveWorkspace(workspace.id)}
							>
								<span>{workspace.name}</span>
							</button>
						{/each}
					</div>
					<div class="cockpit-top-actions">
						<label class="toggle">
							<input
								type="checkbox"
								checked={cockpit.global_enabled}
								on:change={(event) => setGlobalEnabled((event.currentTarget as HTMLInputElement).checked)}
							/>
							<span class="toggle-slider"></span>
							<span class="toggle-label">{tr('cockpit_main_breaker')}</span>
						</label>
						<label class="toggle">
							<input
								type="checkbox"
								checked={activeWorkspace.enabled}
								on:change={(event) => setWorkspaceEnabled((event.currentTarget as HTMLInputElement).checked)}
							/>
							<span class="toggle-slider"></span>
							<span class="toggle-label">{tr('cockpit_workspace_enabled')}</span>
						</label>
						<button class="secondary" on:click={() => loadCockpit(true)}>
							<RefreshCcw size={14} strokeWidth={1.8} />
							<span>{tr('reload')}</span>
						</button>
					</div>
				</div>
				<div class="cockpit-meta-strip">
					<div class="cockpit-meta-chip">
						<span class="cockpit-meta-label">{tr('cockpit_workspace_mode')}</span>
						<strong>{activeWorkspace.kind}</strong>
					</div>
					<div class="cockpit-meta-chip">
						<span class="cockpit-meta-label">{tr('cockpit_ticker_status')}</span>
						<strong class="font-mono">{activeTickerCount} / {totalTickerCount}</strong>
					</div>
					<div class="cockpit-meta-chip cockpit-meta-chip-wide">
						<span class="cockpit-meta-label">{tr('cockpit_feed_source')}</span>
						<strong>{masterWatchlist?.feed?.title || masterWatchlist?.name || tr('ticker_source_default')}</strong>
						<span class="cockpit-meta-subtle">
							{tr('watchlist_last_refreshed', {
								value: formatTimestamp(
									masterWatchlist?.feed?.last_refreshed_at ?? masterWatchlist?.updated_at ?? cockpit.updated_at
								)
							})}
						</span>
					</div>
				</div>
				{#if availableWatchlists.length > 0}
					<div class="cockpit-watchlist-pillbar">
						<span class="cockpit-meta-label">{tr('watchlist')}:</span>
						{#each availableWatchlists as wl}
							<button
								class="secondary cockpit-pill"
								class:active-pill={wl.id === activeWatchlistId}
								on:click={() => switchWatchlist(wl.id)}
							>
								{wl.name}
							</button>
						{/each}
						<button class="secondary cockpit-pill-refresh" on:click={handleRefreshFeed} title={tr('reload')}>
							<RefreshCcw size={13} strokeWidth={1.8} />
						</button>
					</div>
				{/if}
			</div>

			<!-- Telemetry & Breaker HUD Strip (Locked Single-Bar Height) -->
			<div
				class="cockpit-hud-strip card"
				class:hud-breaker-tripped={botState?.circuit_breaker_tripped || !cockpit.global_enabled}
			>
				<!-- Broker Account Telemetry (status text owned by Sidebar) -->
				<div
					class="hud-item hud-connection"
					title={botState?.tws_connected ? tr('tws_connected_status') : tr('tws_disconnected')}
				>
					<span class="hud-dot" class:is-connected={botState?.tws_connected}></span>
					<span class="hud-value font-mono">{botState?.account_id ? `IBKR ${botState.account_id}` : 'IBKR TWS'}</span>
				</div>

				<!-- Heartbeat Pulse -->
				<div class="hud-item hud-heartbeat" title="Heartbeat">
					<span class="hud-pulse-glyph">●</span>
					<span class="hud-value font-mono">
						{botState?.last_heartbeat ? new Date(botState.last_heartbeat).toLocaleTimeString() : '--'}
					</span>
				</div>

				<!-- Portfolio Equity -->
				<div class="hud-item hud-equity">
					<span class="hud-label">{tr('equity')}</span>
					<strong class="hud-value font-mono">
						${(botState?.equity ?? 0).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
					</strong>
				</div>

				<!-- Daily P&L & P&L % -->
				<div class="hud-item hud-pnl" class:is-positive={pnl >= 0} class:is-negative={pnl < 0}>
					<span class="hud-label">{tr('daily_pnl')}</span>
					<strong class="hud-value font-mono">
						{pnl >= 0 ? '+' : ''}${pnl.toFixed(2)} ({pnlPct >= 0 ? '+' : ''}{pnlPct.toFixed(2)}%)
					</strong>
				</div>

				<!-- Daily Loss Limit -->
				<div class="hud-item hud-limit" class:is-near-breach={nearBreach}>
					<span class="hud-label">{tr('cockpit_hud_loss_limit')}</span>
					<span class="hud-value font-mono">-${lossLimit.toFixed(2)}</span>
					{#if nearBreach && !botState?.circuit_breaker_tripped}
						<span class="hud-warning-badge" title={tr('cockpit_hud_near_breach')}>⚠️ 90%</span>
					{/if}
				</div>

				<!-- Master Breaker Status Badge & Reset Action -->
				<div class="hud-item hud-breaker-action">
					{#if botState?.circuit_breaker_tripped || !cockpit.global_enabled}
						<span class="hud-badge breaker-tripped">
							<ShieldAlert size={14} />
							{botState?.circuit_breaker_tripped ? tr('cockpit_hud_breaker_tripped') : tr('cockpit_breaker_off')}
						</span>
						<button class="danger hud-reset-btn" on:click={handleResetBreaker}>
							<Power size={13} strokeWidth={2} />
							<span>{tr('cockpit_hud_reset_breaker')}</span>
						</button>
					{:else}
						<span class="hud-badge breaker-armed">
							<Power size={14} />
							{tr('cockpit_hud_breaker_armed')}
						</span>
					{/if}
				</div>
			</div>

			<div class="cockpit-grid">
				<section class="card cockpit-column">
					<div class="cockpit-section-header">
						<h2>
							<span class="heading-icon"><Sparkles size={18} strokeWidth={1.6} /></span>
							{tr('cockpit_strategy_slots')}
						</h2>
						<span class="muted font-mono">
							{activeStrategySlot?.enabled && activeStrategyId ? '1/1' : '0/1'}
						</span>
					</div>

					<div class="cockpit-slot-list">
						<!-- 1/1 Active Strategy Card -->
						{#if activeStrategySlot}
							<div class="cockpit-slot-card active-strategy-card" data-testid="active-strategy-card">
								<div class="cockpit-slot-header">
									<div class="cockpit-slot-summary">
										<div class="cockpit-slot-title">
											{activeStrategySummary?.name || strategyName(activeStrategyId)}
										</div>
										<div class="cockpit-slot-subtitle">
											{tr('cockpit_active_strategy')}
											{#if activeStrategySummary}
												· {tr('cockpit_rules_count', {
													enabled: activeStrategySummary.enabled_rule_count,
													total: activeStrategySummary.rule_count
												})}
											{/if}
										</div>
									</div>
									<label class="toggle">
										<input
											type="checkbox"
											checked={activeStrategySlot.enabled}
											on:change={(event) =>
												setSlotEnabled(activeStrategySlot, (event.currentTarget as HTMLInputElement).checked)}
										/>
										<span class="toggle-slider"></span>
										<span class="toggle-label">{tr('enabled')}</span>
									</label>
								</div>
								<div class="cockpit-slot-footnote">
									<span class="strategy-armed-badge" class:is-paused={!activeStrategySlot.enabled}>
										{activeStrategySlot.enabled ? tr('cockpit_armed_badge') : tr('cockpit_paused_badge')}
									</span>
									<span>{tr('cockpit_strategy_slot_help')}</span>
								</div>
							</div>
						{/if}

						<!-- Pinned & Reorderable Strategy Stack -->
						<div class="strategy-stack-header">
							<span>{tr('cockpit_strategy_stack')}</span>
							<span class="font-mono">{sortedStrategies.length}</span>
						</div>

						{#if sortedStrategies.length === 0}
							<p class="cockpit-filter-empty">{tr('cockpit_no_strategies')}</p>
						{:else}
							<ul class="strategy-stack-list">
								{#each sortedStrategies as strategy, idx (strategy.id)}
									{@const isArmed = strategy.id === activeStrategyId}
									<li
										class="strategy-stack-item"
										class:is-active={isArmed}
										class:is-pinned={Boolean(strategy.pinned)}
										data-strategy-id={strategy.id}
									>
										<div class="strategy-stack-main">
											<div class="strategy-stack-name-row">
												<span class="strategy-stack-name" title={strategy.name}>{strategy.name}</span>
											</div>
											<div class="strategy-stack-meta font-mono">
												<span>
													{tr('cockpit_rules_count', {
														enabled: strategy.enabled_rule_count,
														total: strategy.rule_count
													})}
												</span>
												{#if strategy.pinned}
													<span>· {tr('cockpit_pinned_badge')}</span>
												{/if}
											</div>
										</div>

										<div class="strategy-stack-actions">
											<button
												type="button"
												class="secondary strategy-pin-btn"
												class:is-pinned={Boolean(strategy.pinned)}
												aria-label={strategy.pinned ? tr('cockpit_unpin_strategy') : tr('cockpit_pin_strategy')}
												title={strategy.pinned ? tr('cockpit_unpin_strategy') : tr('cockpit_pin_strategy')}
												disabled={saving}
												on:click={() => handleTogglePin(strategy.id)}
											>
												📌
											</button>
											<button
												type="button"
												class="secondary strategy-move-btn"
												aria-label={tr('cockpit_move_up')}
												title={tr('cockpit_move_up')}
												disabled={saving || !canMoveUp(idx, sortedStrategies)}
												on:click={() => handleMoveOrder(strategy.id, 'up')}
											>
												↑
											</button>
											<button
												type="button"
												class="secondary strategy-move-btn"
												aria-label={tr('cockpit_move_down')}
												title={tr('cockpit_move_down')}
												disabled={saving || !canMoveDown(idx, sortedStrategies)}
												on:click={() => handleMoveOrder(strategy.id, 'down')}
											>
												↓
											</button>

											{#if isArmed}
												<span
													class="strategy-armed-badge"
													class:is-paused={!activeStrategySlot?.enabled}
												>
													{activeStrategySlot?.enabled ? tr('cockpit_armed_badge') : tr('cockpit_paused_badge')}
												</span>
											{:else}
												<SlideToArm
													label={tr('cockpit_slide_to_arm')}
													disabled={saving}
													on:arm={() => armStrategy(strategy.id)}
												/>
											{/if}
										</div>
									</li>
								{/each}
							</ul>
						{/if}
					</div>
				</section>

				<section class="card cockpit-column">
					<div class="cockpit-section-header">
						<h2><span class="heading-icon"><Tags size={18} strokeWidth={1.6} /></span>{tr('cockpit_ticker_groups')}</h2>
						<label class="toggle">
							<input
								type="checkbox"
								checked={activeTickerCount > 0 && activeTickerCount === totalTickerCount}
								on:change={(event) => setAllTickers((event.currentTarget as HTMLInputElement).checked)}
							/>
							<span class="toggle-slider"></span>
							<span class="toggle-label">{tr('watchlist_all_tickers')}</span>
						</label>
					</div>
					<div class="cockpit-filter-bar">
						<label class="cockpit-filter-input">
							<span class="cockpit-filter-icon"><Search size={15} strokeWidth={1.8} /></span>
							<input
								type="text"
								value={tickerFilter}
								on:input={(event) => (tickerFilter = (event.currentTarget as HTMLInputElement).value)}
								placeholder={tr('filter_tickers')}
								aria-label={tr('filter_tickers')}
							/>
						</label>
						<button
							type="button"
							class="secondary cockpit-filter-clear"
							on:click={() => (tickerFilter = '')}
							disabled={!tickerFilter}
							aria-label={tr('clear_ticker_filter')}
						>
							<X size={14} strokeWidth={2} />
						</button>
					</div>
					<div class="cockpit-group-list">
						{#if filteredGroups.length === 0}
							<p class="cockpit-filter-empty">{tr('no_tickers_match')}</p>
						{:else}
							{#each filteredGroups as filteredGroup}
								{@const group = filteredGroup.group}
								<div class="cockpit-group-card">
									<div class="cockpit-group-header">
										<button class="secondary" on:click={() => toggleGroup(group.id)}>
											{collapsedGroups.has(group.id) ? '+' : '-'} {group.name}
										</button>
										<div class="cockpit-group-actions">
											<span class="font-mono">
												{filteredGroup.items.filter((item) => item.enabled).length}/{filteredGroup.items.length}
											</span>
											<label class="toggle">
												<input
													type="checkbox"
													checked={groupEnabled(group)}
													on:change={(event) =>
														setGroupEnabled(group.id, (event.currentTarget as HTMLInputElement).checked)}
												/>
												<span class="toggle-slider"></span>
												<span class="toggle-label">{tr('enabled')}</span>
											</label>
										</div>
									</div>
									{#if !collapsedGroups.has(group.id)}
										<div class="cockpit-ticker-list">
											{#each filteredGroup.items as item}
												{@const isHeld =
													Boolean(item.is_holding) ||
													group.id === 'holding' ||
													(masterWatchlist?.id || activeWatchlistId) === 'positions-detenues'}
												<div
													class="cockpit-ticker-row {item.enabled ? 'is-enabled' : 'is-disabled'}"
													class:is-holding={isHeld}
												>
													<!-- 1. Normalized SYMBOL.VENUE badge & details -->
													<div class="ticker-identity">
														<span class="ticker-symbol font-mono">{item.symbol}</span>
														<span class="ticker-badge venue-badge font-mono">
															{item.venue || item.exchange || 'SMART'}
														</span>
														{#if item.instrument_id && item.instrument_id !== item.symbol}
															<span class="ticker-instrument-norm font-mono">{item.instrument_id}</span>
														{/if}
														{#if item.name}
															<span class="cockpit-ticker-subtle cockpit-ticker-name" title={item.name}>
																{item.name}
															</span>
														{/if}
													</div>

													<!-- 2. Méthode 8.8 Tier badge -->
													<div class="ticker-tier">
														{#if item.tier}
															<span class="ticker-badge tier-badge tier-{item.tier.slice(0, 1)}">
																{item.tier}
															</span>
														{/if}
													</div>

													<!-- 3. Holding indicator ($$$$) -->
													<div class="ticker-holding">
														{#if isHeld}
															<span
																class="ticker-badge holding-badge font-mono"
																title={tr('watchlist_holding_badge_title')}
															>
																$$$$
															</span>
														{/if}
													</div>

													<!-- 4. Inline position sizing control -->
													<div class="ticker-sizing">
														<select
															class="ticker-sizing-select font-mono"
															value={item.size_mode || 'shares'}
															on:change={(e) =>
																updateTickerSizing(
																	group.id,
																	item,
																	(e.currentTarget as HTMLSelectElement).value,
																	item.size_value || 100
																)}
															aria-label={tr('cockpit_sizing_mode')}
														>
															<option value="shares">#</option>
															<option value="dollar">$</option>
															<option value="percent">%</option>
														</select>
														<input
															type="number"
															class="ticker-sizing-input font-mono"
															value={item.size_value ?? 100}
															min="1"
															step={item.size_mode === 'percent' ? '0.5' : item.size_mode === 'dollar' ? '100' : '1'}
															on:change={(e) =>
																updateTickerSizing(
																	group.id,
																	item,
																	item.size_mode || 'shares',
																	parseFloat((e.currentTarget as HTMLInputElement).value) || 100
																)}
															aria-label={tr('cockpit_sizing_value')}
														/>
													</div>

													<!-- 5. Enable/Disable toggle -->
													<div class="ticker-toggle">
														<label class="toggle">
															<input
																type="checkbox"
																checked={item.enabled}
																on:change={(e) =>
																	setTickerEnabled(group.id, item, (e.currentTarget as HTMLInputElement).checked)}
															/>
															<span class="toggle-slider"></span>
															<span class="toggle-label sr-only">{tr('enabled')}</span>
														</label>
													</div>
												</div>
											{/each}
										</div>
									{/if}
								</div>
							{/each}
						{/if}
					</div>
				</section>
			</div>
		</div>
	{:else}
		<div class="card">
			<p>{tr('loading_chart_data')}</p>
		</div>
	{/if}
</div>

<style>
	.cockpit-header-actions {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		margin-left: auto;
	}

	.cockpit-header-btn {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		padding: 4px 10px;
		font-size: 12px;
		line-height: 1.2;
		border-color: var(--border-highlight, #2f3b52);
	}

	.cockpit-help-btn {
		min-width: 28px;
		justify-content: center;
		font-family: var(--font-mono);
		font-weight: 700;
		color: var(--cobalt-cyan, #38bdf8);
	}
</style>
