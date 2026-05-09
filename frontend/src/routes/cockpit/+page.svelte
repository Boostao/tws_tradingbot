<script lang="ts">
	import { onMount } from 'svelte';
	import {
		formatApiError,
		getCockpit,
		getWatchlist,
		replaceWatchlist,
		saveCockpit,
		type CockpitState,
		type CockpitStrategySlot,
		type CockpitWorkspace,
		type WatchlistGroup,
		type WatchlistItem,
		type WatchlistResponse
	} from '$lib/api';
	import { language, t } from '$lib/i18n';
	import { runtimeState } from '$lib/stores/runtime';
	import { LayoutGrid, Power, RefreshCcw, Search, Settings2, ShieldAlert, Sparkles, Tags, X } from 'lucide-svelte';

	let cockpit: CockpitState | null = null;
	let masterWatchlist: WatchlistResponse | null = null;
	let saving = false;
	let message = '';
	let collapsedGroups = new Set<string>();
	let tickerFilter = '';

	$: botState = $runtimeState;

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
	$: activeStrategySlot = activeWorkspace?.strategy_slots[0] ?? null;

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
			const [nextCockpit, nextWatchlist] = await Promise.all([
				getCockpit(force),
				getWatchlist(force)
			]);
			cockpit = nextCockpit;
			masterWatchlist = nextWatchlist;
		} catch (err) {
			message = formatApiError(err);
		}
	}

	onMount(() => {
		void loadCockpit();
	});

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
								placeholder="Filter tickers"
								aria-label="Filter tickers"
							/>
						</label>
						<button
							type="button"
							class="secondary cockpit-filter-clear"
							on:click={() => (tickerFilter = '')}
							disabled={!tickerFilter}
							aria-label="Clear ticker filter"
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
											<div class="cockpit-ticker-row">
												<div class="cockpit-ticker-line">
													<span class="cockpit-ticker-symbol">{item.symbol}</span>
													<span class="cockpit-ticker-subtle">{item.exchange || '—'}</span>
													{#if item.name}
														<span class="cockpit-ticker-subtle cockpit-ticker-name">{item.name}</span>
													{/if}
												</div>
												<label class="toggle">
													<input type="checkbox" checked={item.enabled} on:change={(event) => setTickerEnabled(group.id, item, (event.currentTarget as HTMLInputElement).checked)} />
													<span class="toggle-slider"></span>
													<span class="toggle-label">{t('enabled')}</span>
												</label>
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