<script lang="ts">
	import { onMount } from 'svelte';
	import {
		applyStrategyPreset,
		formatApiError,
		getCockpit,
		getWatchlist,
		getWatchlistById,
		getWatchlists,
		refreshWatchlistFeed,
		replaceWatchlist,
		resetCircuitBreaker,
		saveCockpit,
		type CockpitState,
		type CockpitStrategySlot,
		type CockpitWorkspace,
		type WatchlistGroup,
		type WatchlistItem,
		type WatchlistResponse,
		type WatchlistSummary
	} from '$lib/api';
	import { language, t } from '$lib/i18n';
	import { runtimeState } from '$lib/stores/runtime';
	import { LayoutGrid, Power, RefreshCcw, Search, Settings2, ShieldAlert, Sparkles, Tags, X } from 'lucide-svelte';

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
	$: lossLimit = botState?.daily_loss_limit ?? 500;
	$: nearBreach = pnl <= -lossLimit * 0.9;

	$: _lang = $language;
	$: activeWorkspace = cockpit?.workspaces.find((workspace) => workspace.id === cockpit?.active_workspace_id) ?? cockpit?.workspaces[0] ?? null;
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
	$: activeTickerCount = displayGroups.reduce((count, group) => count + group.items.filter((item) => item.enabled).length, 0);
	$: totalTickerCount = displayGroups.reduce((count, group) => count + group.items.length, 0);
	$: activeStrategySlot = activeWorkspace?.strategy_slots?.[0] ?? null;

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
						? (typeof nextStrategyId === 'undefined' ? candidate.strategy_id ?? null : nextStrategyId)
						: candidate.strategy_id ?? null;
				return {
					...candidate,
					strategy_id: strategyId,
					enabled: Boolean(
						workspace.id === targetWorkspaceId &&
						candidate.id === targetSlotId &&
						nextEnabled &&
						strategyId
					),
				};
			}),
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
			if (masterWatchlist?.feed?.external_id) {
				activeWatchlistId = masterWatchlist.feed.external_id;
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
			activeWatchlistId = id;
			masterWatchlist = await getWatchlistById(id);
		} catch (err) {
			message = formatApiError(err);
		} finally {
			saving = false;
		}
	}

	async function handleRefreshFeed() {
		try {
			saving = true;
			masterWatchlist = await refreshWatchlistFeed(activeWatchlistId);
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
			masterWatchlist = await replaceWatchlist({ groups: nextGroups, feed: masterWatchlist.feed ?? null });
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
		const nextWorkspaces = buildExclusiveStrategyWorkspaces(activeWorkspace.id, slot.id, enabled, slot.strategy_id ?? null);
		await persist({ ...cockpit, active_workspace_id: activeWorkspace.id, workspaces: nextWorkspaces });
	}

	async function setSlotStrategy(slot: CockpitStrategySlot, strategyId: string) {
		if (!cockpit || !activeWorkspace) return;
		const nextWorkspaces = buildExclusiveStrategyWorkspaces(activeWorkspace.id, slot.id, Boolean(strategyId), strategyId || null);
		await persist({ ...cockpit, active_workspace_id: activeWorkspace.id, workspaces: nextWorkspaces });
		if (strategyId) {
			try {
				await applyStrategyPreset(strategyId, activeWorkspace.id, slot.id);
			} catch (err) {
				message = formatApiError(err);
			}
		}
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

	function strategyName(strategyId?: string | null) {
		if (!strategyId || !cockpit) return t('cockpit_unassigned');
		return cockpit.strategy_library.find((item) => item.id === strategyId)?.name ?? t('cockpit_unassigned');
	}

	function matchesTickerFilter(item: WatchlistItem, filterValue: string) {
		const haystack = [item.symbol, item.exchange ?? '', item.name ?? ''].join(' ').toLowerCase();
		return haystack.includes(filterValue);
	}

	function formatTimestamp(value?: string | null) {
		if (!value) return t('watchlist_never_refreshed');
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) return value;
		return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(date);
	}
</script>

<div class="cockpit-page">
	<div class="cockpit-page-header">
		<h1 class="heading"><span class="heading-icon"><LayoutGrid size={20} strokeWidth={1.6} /></span>{t('cockpit')}</h1>
		{#if message}
			<p class="cockpit-message">{message}</p>
		{/if}
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
							<input type="checkbox" checked={cockpit.global_enabled} on:change={(event) => setGlobalEnabled((event.currentTarget as HTMLInputElement).checked)} />
							<span class="toggle-slider"></span>
							<span class="toggle-label">{t('cockpit_main_breaker')}</span>
						</label>
						<label class="toggle">
							<input type="checkbox" checked={activeWorkspace.enabled} on:change={(event) => setWorkspaceEnabled((event.currentTarget as HTMLInputElement).checked)} />
							<span class="toggle-slider"></span>
							<span class="toggle-label">{t('cockpit_workspace_enabled')}</span>
						</label>
						<button class="secondary" on:click={() => loadCockpit(true)}>
							<RefreshCcw size={14} strokeWidth={1.8} />
							<span>{t('reload')}</span>
						</button>
					</div>
				</div>
				<div class="cockpit-meta-strip">
					<div class="cockpit-meta-chip">
						<span class="cockpit-meta-label">{t('cockpit_workspace_mode')}</span>
						<strong>{activeWorkspace.kind}</strong>
					</div>
					<div class="cockpit-meta-chip">
						<span class="cockpit-meta-label">{t('cockpit_ticker_status')}</span>
						<strong>{activeTickerCount} / {totalTickerCount}</strong>
					</div>
					<div class="cockpit-meta-chip cockpit-meta-chip-wide">
						<span class="cockpit-meta-label">{t('cockpit_feed_source')}</span>
						<strong>{masterWatchlist?.feed?.title || t('ticker_source_default')}</strong>
						<span class="cockpit-meta-subtle">{t('watchlist_last_refreshed', { value: formatTimestamp(masterWatchlist?.feed?.last_refreshed_at ?? masterWatchlist?.updated_at ?? cockpit.updated_at) })}</span>
					</div>
				</div>
				{#if availableWatchlists.length > 0}
					<div class="cockpit-watchlist-pillbar">
						<span class="cockpit-meta-label">{t('watchlist')}:</span>
						{#each availableWatchlists as wl}
							<button
								class="secondary cockpit-pill"
								class:active-pill={wl.id === activeWatchlistId}
								on:click={() => switchWatchlist(wl.id)}
							>
								{wl.name}
							</button>
						{/each}
						<button class="secondary cockpit-pill-refresh" on:click={handleRefreshFeed} title={t('reload')}>
							<RefreshCcw size={13} strokeWidth={1.8} />
						</button>
					</div>
				{/if}
			</div>

			<!-- Telemetry & Breaker HUD Strip -->
			<div class="cockpit-hud-strip card" class:hud-breaker-tripped={botState?.circuit_breaker_tripped || !cockpit.global_enabled}>
				<!-- TWS Connection Indicator -->
				<div class="hud-item hud-connection" title="IBKR TWS Socket">
					<span class="hud-dot" class:is-connected={botState?.tws_connected}></span>
					<span class="hud-label">{botState?.tws_connected ? t('tws_connected_status') : (t('tws_disconnected') || 'TWS Déconnecté')}</span>
				</div>

				<!-- Heartbeat Pulse -->
				<div class="hud-item hud-heartbeat" title="Heartbeat">
					<span class="hud-pulse-glyph">●</span>
					<span class="hud-value">{botState?.last_heartbeat ? new Date(botState.last_heartbeat).toLocaleTimeString() : '--'}</span>
				</div>

				<!-- Portfolio Equity -->
				<div class="hud-item hud-equity">
					<span class="hud-label">{t('equity')}</span>
					<strong class="hud-value font-mono">
						${(botState?.equity ?? 100000).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
					</strong>
				</div>

				<!-- Daily P&L & P&L % -->
				<div class="hud-item hud-pnl" class:is-positive={pnl >= 0} class:is-negative={pnl < 0}>
					<span class="hud-label">{t('daily_pnl')}</span>
					<strong class="hud-value font-mono">
						{pnl >= 0 ? '+' : ''}${pnl.toFixed(2)} ({pnlPct >= 0 ? '+' : ''}{pnlPct.toFixed(2)}%)
					</strong>
				</div>

				<!-- Daily Loss Limit -->
				<div class="hud-item hud-limit" class:is-near-breach={nearBreach}>
					<span class="hud-label">{t('cockpit_hud_loss_limit') || 'Limite Perte'}</span>
					<span class="hud-value font-mono">-${lossLimit.toFixed(2)}</span>
					{#if nearBreach}
						<span class="hud-warning-badge" title="Perte proche de la limite (90%)">⚠️ 90%</span>
					{/if}
				</div>

				<!-- Master Breaker Status Badge & Reset Action -->
				<div class="hud-item hud-breaker-action">
					{#if botState?.circuit_breaker_tripped || !cockpit.global_enabled}
						<span class="hud-badge breaker-tripped">
							<ShieldAlert size={14} />
							{botState?.circuit_breaker_tripped ? (t('cockpit_hud_breaker_tripped') || 'COUPE-CIRCUIT DÉCLENCHÉ') : (t('cockpit_breaker_off') || 'COUPÉ')}
						</span>
						<button class="danger hud-reset-btn" on:click={handleResetBreaker}>
							<Power size={13} strokeWidth={2} />
							<span>{t('cockpit_hud_reset_breaker') || 'Réarmer coupe-circuit'}</span>
						</button>
					{:else}
						<span class="hud-badge breaker-armed">
							<Power size={14} />
							{t('cockpit_hud_breaker_armed') || 'ARMÉ'}
						</span>
					{/if}
				</div>
			</div>

			<div class="cockpit-grid">
				<section class="card cockpit-column">
					<div class="cockpit-section-header">
						<h2><span class="heading-icon"><Sparkles size={18} strokeWidth={1.6} /></span>{t('cockpit_strategy_slots')}</h2>
						<span class="muted">{activeStrategySlot?.enabled && activeStrategySlot?.strategy_id ? '1/1' : '0/1'}</span>
					</div>
					<div class="cockpit-slot-list">
						{#if activeStrategySlot}
							<div class="cockpit-slot-card">
								<div class="cockpit-slot-header">
									<div class="cockpit-slot-summary">
										<div class="cockpit-slot-title">{activeStrategySlot.label}</div>
										<div class="cockpit-slot-subtitle">{strategyName(activeStrategySlot.strategy_id)}</div>
									</div>
									<label class="toggle">
										<input type="checkbox" checked={activeStrategySlot.enabled} on:change={(event) => setSlotEnabled(activeStrategySlot, (event.currentTarget as HTMLInputElement).checked)} />
										<span class="toggle-slider"></span>
										<span class="toggle-label">{t('enabled')}</span>
									</label>
								</div>
								<div class="cockpit-slot-controls">
									<select value={activeStrategySlot.strategy_id ?? ''} on:change={(event) => setSlotStrategy(activeStrategySlot, (event.currentTarget as HTMLSelectElement).value)}>
										<option value="">{t('cockpit_unassigned')}</option>
										{#each cockpit.strategy_library as strategy}
											<option value={strategy.id}>{strategy.name}</option>
										{/each}
									</select>
									<div class="cockpit-slot-footnote">
									<Settings2 size={14} strokeWidth={1.6} />
									<span>{t('cockpit_strategy_slot_help')}</span>
									</div>
								</div>
							</div>
						{/if}
					</div>
				</section>

				<section class="card cockpit-column">
					<div class="cockpit-section-header">
						<h2><span class="heading-icon"><Tags size={18} strokeWidth={1.6} /></span>{t('cockpit_ticker_groups')}</h2>
						<label class="toggle">
							<input type="checkbox" checked={activeTickerCount > 0 && activeTickerCount === totalTickerCount} on:change={(event) => setAllTickers((event.currentTarget as HTMLInputElement).checked)} />
							<span class="toggle-slider"></span>
							<span class="toggle-label">{t('watchlist_all_tickers')}</span>
						</label>
					</div>
					<div class="cockpit-filter-bar">
						<label class="cockpit-filter-input">
							<span class="cockpit-filter-icon"><Search size={15} strokeWidth={1.8} /></span>
							<input
								type="text"
								value={tickerFilter}
								on:input={(event) => (tickerFilter = (event.currentTarget as HTMLInputElement).value)}
								placeholder={t('filter_tickers')}
								aria-label={t('filter_tickers')}
							/>
						</label>
						<button
							type="button"
							class="secondary cockpit-filter-clear"
							on:click={() => (tickerFilter = '')}
							disabled={!tickerFilter}
							aria-label={t('clear_ticker_filter')}
						>
							<X size={14} strokeWidth={2} />
						</button>
					</div>
					<div class="cockpit-group-list">
						{#if filteredGroups.length === 0}
							<p class="cockpit-filter-empty">{t('no_tickers_match')}</p>
						{:else}
							{#each filteredGroups as filteredGroup}
								{@const group = filteredGroup.group}
							<div class="cockpit-group-card">
								<div class="cockpit-group-header">
									<button class="secondary" on:click={() => toggleGroup(group.id)}>{collapsedGroups.has(group.id) ? '+' : '-'} {group.name}</button>
									<div class="cockpit-group-actions">
										<span>{filteredGroup.items.filter((item) => item.enabled).length}/{filteredGroup.items.length}</span>
										<label class="toggle">
											<input type="checkbox" checked={groupEnabled(group)} on:change={(event) => setGroupEnabled(group.id, (event.currentTarget as HTMLInputElement).checked)} />
											<span class="toggle-slider"></span>
											<span class="toggle-label">{t('enabled')}</span>
										</label>
									</div>
								</div>
								{#if !collapsedGroups.has(group.id)}
									<div class="cockpit-ticker-list">
										{#each filteredGroup.items as item}
											<div class="cockpit-ticker-row {item.enabled ? 'is-enabled' : 'is-disabled'}" class:is-holding={item.is_holding || group.id === 'holding'}>
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
														<span class="cockpit-ticker-subtle cockpit-ticker-name" title={item.name}>{item.name}</span>
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
													{#if item.is_holding || group.id === 'holding' || (item.instrument_id && masterWatchlist?.feed?.external_id === 'positions-detenues')}
														<span class="ticker-badge holding-badge font-mono" title="Position Détenue ($$$$)">$$$$</span>
													{/if}
												</div>

												<!-- 4. Inline position sizing control -->
												<div class="ticker-sizing">
													<select
														class="ticker-sizing-select font-mono"
														value={item.size_mode || 'shares'}
														on:change={(e) => updateTickerSizing(group.id, item, (e.currentTarget as HTMLSelectElement).value, item.size_value || 100)}
														aria-label="Mode de dimensionnement"
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
														on:change={(e) => updateTickerSizing(group.id, item, item.size_mode || 'shares', parseFloat((e.currentTarget as HTMLInputElement).value) || 100)}
														aria-label="Valeur de dimensionnement"
													/>
												</div>

												<!-- 5. Enable/Disable toggle -->
												<div class="ticker-toggle">
													<label class="toggle">
														<input type="checkbox" checked={item.enabled} on:change={(e) => setTickerEnabled(group.id, item, (e.currentTarget as HTMLInputElement).checked)} />
														<span class="toggle-slider"></span>
														<span class="toggle-label sr-only">{t('enabled')}</span>
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

			<div class="card cockpit-safety {cockpit.global_enabled ? '' : 'is-off'}">
				<div class="cockpit-section-header">
					<h2><span class="heading-icon"><ShieldAlert size={18} strokeWidth={1.6} /></span>{t('cockpit_safety_title')}</h2>
					<span class="cockpit-breaker-state">
						<Power size={16} strokeWidth={1.6} />
						{cockpit.global_enabled ? t('cockpit_breaker_on') : t('cockpit_breaker_off')}
					</span>
				</div>
				<p>{t('cockpit_safety_copy')}</p>
			</div>
		</div>
	{:else}
		<div class="card">
			<p>{t('loading_chart_data')}</p>
		</div>
	{/if}
</div>
