<script lang="ts">
	import { onMount } from 'svelte';
	import {
		formatApiError,
		getWatchlist,
		getWatchlistById,
		getWatchlists,
		importTradingViewWatchlist,
		refreshWatchlistFeed,
		replaceWatchlist,
		exportWatchlistFile,
		importWatchlistFile,
		type SymbolRecord,
		type WatchlistFeed,
		type WatchlistGroup,
		type WatchlistItem,
		type WatchlistResponse,
		type WatchlistSummary
	} from '$lib/api';
	import { t, language } from '$lib/i18n';
	import { watchlistDraftUrl } from '$lib/stores/ui';
	import SymbolSearch from '$lib/components/SymbolSearch.svelte';
	import { ChevronDown, ChevronRight, Download, List, Plus, RefreshCcw, Trash2, Upload } from 'lucide-svelte';

	let availableWatchlists: WatchlistSummary[] = [];
	let activeWatchlistId = '';
	let watchlist: WatchlistResponse = { symbols: [], groups: [], feed: null, updated_at: null };
	let message = '';
	let saving = false;
	let addingToManual = false;
	let collapsedGroups = new Set<string>();

	$: _lang = $language;
	$: tr = (key: string, params?: Record<string, string | number>) => {
		void _lang;
		return t(key, params);
	};
	$: totalTickers = watchlist.groups.reduce((count, group) => count + group.items.length, 0);
	$: activeTickers = watchlist.symbols.length;
	$: allEnabled =
		totalTickers > 0 && watchlist.groups.every((group) => group.items.every((item) => item.enabled));
	$: currentWatchlistId = activeWatchlistId || watchlist.id || 'cobalt-telegram';
	$: isHoldingsWatchlist = currentWatchlistId === 'positions-detenues';

	function cloneGroups(groups: WatchlistGroup[]): WatchlistGroup[] {
		return groups.map((group) => ({
			...group,
			items: group.items.map((item) => ({ ...item }))
		}));
	}

	function itemKey(item: Pick<WatchlistItem, 'symbol' | 'exchange'>): string {
		const symbol = item.symbol.trim().toUpperCase();
		const exchange = (item.exchange ?? '').trim().toUpperCase();
		return exchange ? `${symbol}:${exchange}` : symbol;
	}

	function createManualGroup(): WatchlistGroup {
		return {
			id: 'manual',
			name: t('watchlist_manual_group'),
			source: 'manual',
			items: []
		};
	}

	function ensureManualGroup(groups: WatchlistGroup[], targetId?: string): WatchlistGroup[] {
		if (targetId === 'positions-detenues') {
			return groups;
		}
		if (groups.some((group) => group.source === 'manual' || group.id === 'manual')) {
			return groups;
		}
		return [createManualGroup(), ...groups];
	}

	function normalizeWatchlist(result: WatchlistResponse) {
		const resolvedId = result.id || activeWatchlistId || 'cobalt-telegram';
		activeWatchlistId = resolvedId;
		watchlist = {
			...result,
			id: resolvedId,
			groups: ensureManualGroup(result.groups ?? [], resolvedId),
			feed: result.feed ?? null
		};
		watchlistDraftUrl.set(result.feed?.url ?? '');
	}

	async function refreshSummaries() {
		try {
			availableWatchlists = await getWatchlists();
		} catch {
			// keep existing summaries
		}
	}

	async function loadWatchlist(force = false) {
		try {
			const [summaries, result] = await Promise.all([
				getWatchlists(),
				activeWatchlistId ? getWatchlistById(activeWatchlistId, force) : getWatchlist(force)
			]);
			availableWatchlists = summaries;
			normalizeWatchlist(result);
		} catch (err) {
			message = formatApiError(err);
		}
	}

	async function switchWatchlist(id: string) {
		if (!id) return;
		saving = true;
		message = '';
		try {
			activeWatchlistId = id;
			const result = await getWatchlistById(id, true);
			normalizeWatchlist(result);
		} catch (err) {
			message = formatApiError(err);
		} finally {
			saving = false;
		}
	}

	onMount(() => {
		void loadWatchlist();
	});

	async function persistGroups(nextGroups: WatchlistGroup[], nextFeed: WatchlistFeed | null = watchlist.feed ?? null) {
		saving = true;
		message = '';
		try {
			const updated = await replaceWatchlist({
				watchlist_id: currentWatchlistId,
				groups: nextGroups,
				feed: nextFeed
			});
			normalizeWatchlist(updated);
			await refreshSummaries();
		} catch (err) {
			message = formatApiError(err);
		} finally {
			saving = false;
		}
	}

	async function handleExportFile() {
		message = '';
		saving = true;
		try {
			await exportWatchlistFile();
		} catch (err) {
			message = formatApiError(err);
		} finally {
			saving = false;
		}
	}

	async function handleImportFile() {
		message = '';
		saving = true;
		try {
			const result = await importWatchlistFile();
			if (result) {
				normalizeWatchlist(result);
				await refreshSummaries();
				message = t('saved');
			}
		} catch (err) {
			message = formatApiError(err);
		} finally {
			saving = false;
		}
	}

	async function handleImportTradingView() {
		const url = $watchlistDraftUrl.trim();
		if (!url) return;
		message = '';
		saving = true;
		try {
			const targetId = isHoldingsWatchlist ? undefined : currentWatchlistId;
			const imported = await importTradingViewWatchlist(url, targetId);
			normalizeWatchlist(imported);
			await refreshSummaries();
			message = t('saved');
		} catch (err) {
			message = formatApiError(err);
		} finally {
			saving = false;
		}
	}

	async function handleClearAll() {
		message = '';
		saving = true;
		try {
			const nextGroups = isHoldingsWatchlist ? [] : [createManualGroup()];
			const updated = await replaceWatchlist({
				watchlist_id: currentWatchlistId,
				groups: nextGroups,
				feed: null
			});
			normalizeWatchlist(updated);
			await refreshSummaries();
			message = t('clear_all');
		} catch (err) {
			message = formatApiError(err);
		} finally {
			saving = false;
		}
	}

	async function handleRefreshFeed() {
		message = '';
		saving = true;
		try {
			const refreshed = await refreshWatchlistFeed(currentWatchlistId);
			normalizeWatchlist(refreshed);
			await refreshSummaries();
			message = t('saved');
		} catch (err) {
			message = formatApiError(err);
		} finally {
			saving = false;
		}
	}

	function toggleGroupCollapse(groupId: string) {
		const next = new Set(collapsedGroups);
		if (next.has(groupId)) next.delete(groupId);
		else next.add(groupId);
		collapsedGroups = next;
	}

	function groupEnabled(group: WatchlistGroup): boolean {
		return group.items.length > 0 && group.items.every((item) => item.enabled);
	}

	function findManualGroupIndex(groups: WatchlistGroup[]): number {
		return groups.findIndex((group) => group.source === 'manual' || group.id === 'manual');
	}

	async function handleAddSymbol(item: SymbolRecord) {
		const updated = cloneGroups(ensureManualGroup(watchlist.groups));
		let manualIndex = findManualGroupIndex(updated);
		if (manualIndex === -1) {
			updated.unshift(createManualGroup());
			manualIndex = 0;
		}
		const nextItem: WatchlistItem = {
			symbol: item.symbol.toUpperCase(),
			exchange: (item.exchange ?? '').toUpperCase(),
			name: item.name ?? '',
			enabled: true
		};
		const nextKey = itemKey(nextItem);
		if (updated.some((group) => group.items.some((existing) => itemKey(existing) === nextKey))) {
			message = t('symbol_already_added');
			addingToManual = false;
			return;
		}
		updated[manualIndex].items = [...updated[manualIndex].items, nextItem];
		addingToManual = false;
		await persistGroups(updated);
	}

	async function handleItemToggle(groupId: string, itemValue: WatchlistItem, enabled: boolean) {
		const updated = cloneGroups(watchlist.groups).map((group) => {
			if (group.id !== groupId) return group;
			return {
				...group,
				items: group.items.map((item) =>
					itemKey(item) === itemKey(itemValue) ? { ...item, enabled } : item
				)
			};
		});
		await persistGroups(updated);
	}

	async function handleGroupToggle(groupId: string, enabled: boolean) {
		const updated = cloneGroups(watchlist.groups).map((group) =>
			group.id === groupId
				? { ...group, items: group.items.map((item) => ({ ...item, enabled })) }
				: group
		);
		await persistGroups(updated);
	}

	async function handleAllToggle(enabled: boolean) {
		const updated = cloneGroups(watchlist.groups).map((group) => ({
			...group,
			items: group.items.map((item) => ({ ...item, enabled }))
		}));
		await persistGroups(updated);
	}

	async function handleRemoveItem(groupId: string, itemValue: WatchlistItem) {
		const updated = cloneGroups(watchlist.groups).map((group) => {
			if (group.id !== groupId) return group;
			return {
				...group,
				items: group.items.filter((item) => itemKey(item) !== itemKey(itemValue))
			};
		});
		await persistGroups(updated);
	}

	$: formatTimestamp = (value?: string | null) => {
		if (!value) return tr('watchlist_never_refreshed');
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) return value;
		return new Intl.DateTimeFormat(undefined, {
			dateStyle: 'medium',
			timeStyle: 'short'
		}).format(date);
	};
</script>

<h1 class="heading"><span class="heading-icon"><List size={20} strokeWidth={1.6} /></span>{tr('watchlist')}</h1>

{#if message}
	<p style="color: #f87171;">{message}</p>
{/if}

<div class="card">
	<div class="watchlist-toolbar">
		<select
			class="watchlist-selector"
			aria-label={tr('watchlist_selector_label')}
			value={currentWatchlistId}
			disabled={saving}
			on:change={(e) => switchWatchlist((e.currentTarget as HTMLSelectElement).value)}
		>
			{#each availableWatchlists as wl}
				<option value={wl.id}>{wl.name}</option>
			{/each}
		</select>

		<input
			type="url"
			value={$watchlistDraftUrl}
			on:input={(e) => watchlistDraftUrl.set((e.currentTarget as HTMLInputElement).value)}
			placeholder={tr('watchlist_feed_url_placeholder')}
			style="min-width: min(100%, 340px); flex: 1 1 280px;"
		/>
		<button class="secondary" on:click={handleImportTradingView} disabled={saving}>
			{tr('watchlist_import_tradingview')}
		</button>
		<button class="secondary" on:click={handleRefreshFeed} disabled={saving || !watchlist.feed?.url}>
			<RefreshCcw size={14} strokeWidth={1.8} />
			<span>{tr('refresh')}</span>
		</button>
		<button class="secondary" on:click={handleImportFile} disabled={saving}>
			<Upload size={14} strokeWidth={1.8} />
			<span>{tr('watchlist_import_file')}</span>
		</button>
		<button class="secondary" on:click={handleExportFile} disabled={saving}>
			<Download size={14} strokeWidth={1.8} />
			<span>{tr('watchlist_export_file')}</span>
		</button>
		<button class="secondary" on:click={() => (addingToManual = !addingToManual)} disabled={saving || isHoldingsWatchlist}>
			<Plus size={14} strokeWidth={1.8} />
			<span>{tr('watchlist_add_ticker')}</span>
		</button>
		<button class="secondary" on:click={handleClearAll} disabled={saving || isHoldingsWatchlist}>{tr('clear_all')}</button>
		{#if saving}
			<span class="muted">{tr('loading_chart_data')}</span>
		{/if}
	</div>

	{#if isHoldingsWatchlist}
		<p class="watchlist-holding-hint">{tr('watchlist_holding_readonly_hint')}</p>
	{/if}

	<div class="watchlist-summary">
		<div>
			<strong class="font-mono">{tr('watchlist_active_summary', { active: activeTickers, total: totalTickers })}</strong>
		</div>
		<div>
			{tr('watchlist_last_refreshed', {
				value: formatTimestamp(watchlist.feed?.last_refreshed_at ?? watchlist.updated_at)
			})}
		</div>
		<label class="toggle">
			<input
				type="checkbox"
				checked={allEnabled}
				on:change={(event) => handleAllToggle((event.currentTarget as HTMLInputElement).checked)}
			/>
			<span class="toggle-slider"></span>
			<span class="toggle-label">{tr('watchlist_all_tickers')}</span>
		</label>
	</div>

	{#if addingToManual && !isHoldingsWatchlist}
		<div class="watchlist-add-panel">
			<SymbolSearch
				placeholder={tr('watchlist_add_ticker_placeholder')}
				statusTone={'online'}
				includeNonStocks={true}
				autoFocus={true}
				onSelect={handleAddSymbol}
				onCancel={() => (addingToManual = false)}
			/>
		</div>
	{/if}

	<div class="watchlist-groups">
		{#each watchlist.groups as group}
			<div class="watchlist-group">
				<div class="watchlist-group-header">
					<button class="secondary" type="button" on:click={() => toggleGroupCollapse(group.id)}>
						{#if collapsedGroups.has(group.id)}
							<ChevronRight size={14} strokeWidth={1.8} />
						{:else}
							<ChevronDown size={14} strokeWidth={1.8} />
						{/if}
						<span>{group.name}</span>
					</button>
					<div class="watchlist-group-meta">
						<span class="font-mono">{group.items.filter((item) => item.enabled).length}/{group.items.length}</span>
						<label class="toggle">
							<input
								type="checkbox"
								checked={groupEnabled(group)}
								on:change={(event) => handleGroupToggle(group.id, (event.currentTarget as HTMLInputElement).checked)}
							/>
							<span class="toggle-slider"></span>
							<span class="toggle-label">{tr('enabled')}</span>
						</label>
					</div>
				</div>

				{#if !collapsedGroups.has(group.id)}
					{#if group.items.length === 0}
						<p class="watchlist-empty">{tr('watchlist_no_tickers')}</p>
					{:else}
						<div class="watchlist-group-items">
							{#each group.items as item}
								{@const isHeld = Boolean(item.is_holding) || group.id === 'holding' || isHoldingsWatchlist}
								<div class="watchlist-item-row" class:is-holding={isHeld}>
									<div class="watchlist-item-main">
										<div class="watchlist-item-title-row">
											<span class="watchlist-item-symbol font-mono">{item.symbol}</span>
											<span class="ticker-badge venue-badge font-mono">
												{item.venue || item.exchange || 'SMART'}
											</span>
											{#if item.instrument_id && item.instrument_id !== item.symbol}
												<span class="ticker-instrument-norm font-mono">{item.instrument_id}</span>
											{/if}
											{#if item.tier}
												<span class="ticker-badge tier-badge tier-{item.tier.slice(0, 1)}">
													{item.tier}
												</span>
											{/if}
											{#if isHeld}
												<span
													class="ticker-badge holding-badge font-mono"
													title={tr('watchlist_holding_badge_title')}
												>
													$$$$
												</span>
											{/if}
										</div>
										<div class="watchlist-item-subtitle">
											<span>{item.exchange || item.venue || '—'}</span>
											{#if item.name}
												<span>• {item.name}</span>
											{/if}
										</div>
									</div>
									<div class="watchlist-item-actions">
										<label class="toggle">
											<input
												type="checkbox"
												checked={item.enabled}
												on:change={(event) =>
													handleItemToggle(group.id, item, (event.currentTarget as HTMLInputElement).checked)}
											/>
											<span class="toggle-slider"></span>
											<span class="toggle-label">{tr('enabled')}</span>
										</label>
										{#if (group.source === 'manual' || group.id === 'manual') && !isHoldingsWatchlist}
											<button
												class="secondary"
												type="button"
												on:click={() => handleRemoveItem(group.id, item)}
												aria-label={tr('remove_row')}
												title={tr('remove_row')}
											>
												<Trash2 size={16} strokeWidth={1.6} />
											</button>
										{/if}
									</div>
								</div>
							{/each}
						</div>
					{/if}
				{/if}
			</div>
		{/each}
	</div>
</div>
