<script lang="ts">
	import { onMount } from 'svelte';
	import {
		formatApiError,
		getWatchlist,
		importTradingViewWatchlist,
		refreshWatchlistFeed,
		replaceWatchlist,
		type SymbolRecord,
		type WatchlistFeed,
		type WatchlistGroup,
		type WatchlistItem,
		type WatchlistResponse
	} from '$lib/api';
	import { t, language } from '$lib/i18n';
	import SymbolSearch from '$lib/components/SymbolSearch.svelte';
	import { ChevronDown, ChevronRight, List, Plus, RefreshCcw, Trash2 } from 'lucide-svelte';

	let watchlist: WatchlistResponse = { symbols: [], groups: [], feed: null, updated_at: null };
	let message = '';
	let saving = false;
	let fileInput: HTMLInputElement | null = null;
	import { watchlistDraftUrl } from "$lib/stores/ui";
	$: tradingViewUrl = $watchlistDraftUrl;
	function updateDraft(e: any) { watchlistDraftUrl.set(e.target.value); }
	let addingToManual = false;
	let collapsedGroups = new Set<string>();

	$: _lang = $language;
	$: totalTickers = watchlist.groups.reduce((count, group) => count + group.items.length, 0);
	$: activeTickers = watchlist.symbols.length;
	$: allEnabled = totalTickers > 0 && watchlist.groups.every((group) => group.items.every((item) => item.enabled));

	function cloneGroups(groups: WatchlistGroup[]): WatchlistGroup[] {
		return groups.map((group) => ({
			...group,
			items: group.items.map((item) => ({ ...item }))
		}));
	}

	function parseEntry(value: string): WatchlistItem {
		const [rawTicker, rawMarket] = value.split(':', 2);
		return {
			symbol: (rawTicker ?? '').trim().toUpperCase(),
			exchange: (rawMarket ?? '').trim().toUpperCase(),
			name: '',
			enabled: true
		};
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

	function ensureManualGroup(groups: WatchlistGroup[]): WatchlistGroup[] {
		if (groups.some((group) => group.source === 'manual' || group.id === 'manual')) {
			return groups;
		}
		return [createManualGroup(), ...groups];
	}

	function normalizeWatchlist(result: WatchlistResponse) {
		watchlist = {
			...result,
			groups: ensureManualGroup(result.groups ?? []),
			feed: result.feed ?? null
		};
		tradingViewUrl = result.feed?.url ?? tradingViewUrl;
	}

	async function loadWatchlist(force = false) {
		try {
			normalizeWatchlist(await getWatchlist(force));
		} catch (err) {
			message = formatApiError(err);
		}
	}

	onMount(() => {
		loadWatchlist();
	});

	function parseSymbols(text: string): string[] {
		const entries = text
			.split(/\r?\n|,|\t|;/)
			.map((value) => value.trim())
			.filter(Boolean)
			.map((value) => parseEntry(value));
		const seen = new Set<string>();
		const result: string[] = [];
		for (const entry of entries) {
			if (!entry.symbol) continue;
			const key = itemKey(entry);
			if (seen.has(key)) continue;
			seen.add(key);
			result.push(key);
		}
		return result;
	}

	async function persistGroups(nextGroups: WatchlistGroup[], nextFeed: WatchlistFeed | null = watchlist.feed ?? null) {
		saving = true;
		message = '';
		try {
			normalizeWatchlist(await replaceWatchlist({ groups: nextGroups, feed: nextFeed }));
		} catch (err) {
			message = formatApiError(err);
		} finally {
			saving = false;
		}
	}

	async function handleFileChange(event: Event) {
		const target = event.target as HTMLInputElement;
		const file = target.files?.[0];
		if (!file) return;
		message = '';
		saving = true;
		try {
			const text = await file.text();
			const next = parseSymbols(text);
			const manualGroup = createManualGroup();
			manualGroup.items = next.map((entry) => parseEntry(entry));
			normalizeWatchlist(await replaceWatchlist({ groups: [manualGroup], feed: null }));
			message = t('saved');
		} catch (err) {
			message = formatApiError(err);
		} finally {
			saving = false;
			if (fileInput) fileInput.value = '';
		}
	}

	async function handleClearAll() {
		message = '';
		saving = true;
		try {
			normalizeWatchlist(await replaceWatchlist({ groups: [createManualGroup()], feed: null }));
			message = t('clear_all');
		} catch (err) {
			message = formatApiError(err);
		} finally {
			saving = false;
		}
	}

	function handleDownload() {
		const payload = watchlist.symbols.join('\n');
		const blob = new Blob([payload], { type: 'text/plain' });
		const url = URL.createObjectURL(blob);
		const link = document.createElement('a');
		link.href = url;
		link.download = 'watchlist.txt';
		document.body.appendChild(link);
		link.click();
		link.remove();
		URL.revokeObjectURL(url);
	}

	async function handleImportTradingView() {
		if (!tradingViewUrl.trim()) return;
		message = '';
		saving = true;
		try {
			normalizeWatchlist(await importTradingViewWatchlist(tradingViewUrl.trim()));
			message = t('saved');
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
			normalizeWatchlist(await refreshWatchlistFeed());
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

	function formatTimestamp(value?: string | null) {
		if (!value) return t('watchlist_never_refreshed');
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) return value;
		return new Intl.DateTimeFormat(undefined, {
			dateStyle: 'medium',
			timeStyle: 'short'
		}).format(date);
	}
</script>

<h1 class="heading"><span class="heading-icon"><List size={20} strokeWidth={1.6} /></span>{t('watchlist')}</h1>

{#if message}
	<p style="color: #f87171;">{message}</p>
{/if}

<div class="card">
	<div class="watchlist-toolbar">
		<input
			type="file"
			accept=".txt,.csv"
			bind:this={fileInput}
			on:change={handleFileChange}
			style="display: none;"
		/>
		<input
			type="url"
			value={tradingViewUrl} on:input={updateDraft}
			placeholder={t('watchlist_feed_url_placeholder')}
			style="min-width: min(100%, 420px); flex: 1 1 320px;"
		/>
		<button class="secondary" on:click={handleImportTradingView}>{t('watchlist_import_tradingview')}</button>
		<button class="secondary" on:click={handleRefreshFeed} disabled={!watchlist.feed?.url}>
			<RefreshCcw size={14} strokeWidth={1.8} />
			<span>{t('refresh')}</span>
		</button>
		<button class="secondary" on:click={() => fileInput?.click()}>{t('reload_from_file')}</button>
		<button class="secondary" on:click={handleDownload}>{t('download')}</button>
		<button class="secondary" on:click={() => (addingToManual = !addingToManual)}>
			<Plus size={14} strokeWidth={1.8} />
			<span>{t('watchlist_add_ticker')}</span>
		</button>
		<button class="secondary" on:click={handleClearAll}>{t('clear_all')}</button>
		{#if saving}
			<span class="muted">{t('loading_chart_data')}</span>
		{/if}
	</div>

	<div class="watchlist-summary">
		<div>
			<strong>{t('watchlist_active_summary', { active: activeTickers, total: totalTickers })}</strong>
		</div>
		<div>{t('watchlist_last_refreshed', { value: formatTimestamp(watchlist.feed?.last_refreshed_at ?? watchlist.updated_at) })}</div>
		<label class="toggle">
			<input
				type="checkbox"
				checked={allEnabled}
				on:change={(event) => handleAllToggle((event.currentTarget as HTMLInputElement).checked)}
			/>
			<span class="toggle-slider"></span>
			<span class="toggle-label">{t('watchlist_all_tickers')}</span>
		</label>
	</div>

	{#if addingToManual}
		<div class="watchlist-add-panel">
			<SymbolSearch
				placeholder={t('watchlist_add_ticker_placeholder')}
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
						<span>{group.items.filter((item) => item.enabled).length}/{group.items.length}</span>
						<label class="toggle">
							<input
								type="checkbox"
								checked={groupEnabled(group)}
								on:change={(event) => handleGroupToggle(group.id, (event.currentTarget as HTMLInputElement).checked)}
							/>
							<span class="toggle-slider"></span>
							<span class="toggle-label">{t('enabled')}</span>
						</label>
					</div>
				</div>

				{#if !collapsedGroups.has(group.id)}
					{#if group.items.length === 0}
						<p class="watchlist-empty">{t('watchlist_no_tickers')}</p>
					{:else}
						<div class="watchlist-group-items">
							{#each group.items as item}
								<div class="watchlist-item-row">
									<div class="watchlist-item-main">
										<div class="watchlist-item-symbol">{item.symbol}</div>
										<div class="watchlist-item-subtitle">
											<span>{item.exchange || '—'}</span>
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
												on:change={(event) => handleItemToggle(group.id, item, (event.currentTarget as HTMLInputElement).checked)}
											/>
											<span class="toggle-slider"></span>
											<span class="toggle-label">{t('enabled')}</span>
										</label>
										{#if group.source === 'manual' || group.id === 'manual'}
											<button
												class="secondary"
												type="button"
												on:click={() => handleRemoveItem(group.id, item)}
												aria-label={t('remove_row')}
												title={t('remove_row')}
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
