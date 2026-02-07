<script lang="ts">
	import { onMount } from 'svelte';
	import { formatApiError, getSymbols, getWatchlist, replaceWatchlist } from '$lib/api';
	import { t, language } from '$lib/i18n';
	import { botState } from '$lib/stores/botState';
	import SymbolSearch from '$lib/components/SymbolSearch.svelte';
	import { List, Trash2 } from 'lucide-svelte';

	type WatchlistRow = {
		ticker: string;
		description: string;
		market: string;
	};

	type WatchlistEntry = {
		ticker: string;
		market: string;
	};

	let rows: WatchlistRow[] = [{ ticker: '', description: '', market: '' }];
	let message = '';
	let saving = false;
	let fileInput: HTMLInputElement | null = null;
	let editingRow: number | null = null;

	$: _lang = $language;
	$: twsConnected = $botState.tws_connected;

	function normalizeRows(next: WatchlistRow[]) {
		const cleaned = next.filter((row) => row.ticker.trim().length > 0 || row.description || row.market);
		if (cleaned.length === 0 || cleaned[cleaned.length - 1].ticker.trim().length > 0) {
			cleaned.push({ ticker: '', description: '', market: '' });
		}
		rows = cleaned;
	}

	function parseEntry(value: string): WatchlistEntry {
		const [rawTicker, rawMarket] = value.split(':', 2);
		return {
			ticker: (rawTicker ?? '').trim().toUpperCase(),
			market: (rawMarket ?? '').trim().toUpperCase()
		};
	}

	function entryToString(entry: WatchlistEntry): string {
		return entry.market ? `${entry.ticker}:${entry.market}` : entry.ticker;
	}

	function rowKey(row: WatchlistRow): string {
		return entryToString({
			ticker: row.ticker.trim().toUpperCase(),
			market: row.market.trim().toUpperCase()
		});
	}

	function toSymbols(list: WatchlistRow[]) {
		const seen = new Set<string>();
		const result: string[] = [];
		for (const row of list) {
			const ticker = row.ticker.trim().toUpperCase();
			if (!ticker) continue;
			const key = rowKey(row);
			if (seen.has(key)) continue;
			seen.add(key);
			result.push(
				entryToString({ ticker, market: row.market.trim().toUpperCase() })
			);
		}
		return result;
	}

	async function hydrateRow(index: number, ticker: string, market = '') {
		try {
			const exchange = market.trim().toUpperCase();
			const { symbols } = await getSymbols({
				q: ticker,
				exchange: exchange || undefined,
				limit: 200
			});
			let match = symbols.find((item) => {
				if (item.symbol.toUpperCase() !== ticker.toUpperCase()) return false;
				if (!exchange) return true;
				return (item.exchange ?? '').toUpperCase() === exchange;
			});
			if (!match && exchange) {
				const retry = await getSymbols({ q: ticker, limit: 200 });
				match = retry.symbols.find(
					(item) => item.symbol.toUpperCase() === ticker.toUpperCase()
				);
			}
			if (!match) return;
			rows[index] = {
				...rows[index],
				ticker: match.symbol.toUpperCase(),
				description: match.name ?? rows[index].description,
				market: (match.exchange ?? rows[index].market ?? '').toUpperCase()
			};
			normalizeRows([...rows]);
		} catch {
			// ignore symbol lookup errors
		}
	}

	async function loadWatchlist(force = false) {
		try {
			const result = await getWatchlist(force);
			const nextRows = result.symbols.map((entry) => {
				const parsed = parseEntry(entry);
				return {
					ticker: parsed.ticker,
					description: '',
					market: parsed.market
				};
			});
			normalizeRows(nextRows);
			result.symbols.forEach((entry, idx) => {
				const parsed = parseEntry(entry);
				void hydrateRow(idx, parsed.ticker, parsed.market);
			});
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
			if (!entry.ticker) continue;
			const key = entryToString(entry);
			if (seen.has(key)) continue;
			seen.add(key);
			result.push(key);
		}
		return result;
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
			const result = await replaceWatchlist(next);
			const nextRows = result.symbols.map((entry) => {
				const parsed = parseEntry(entry);
				return {
					ticker: parsed.ticker,
					description: '',
					market: parsed.market
				};
			});
			normalizeRows(nextRows);
			result.symbols.forEach((entry, idx) => {
				const parsed = parseEntry(entry);
				void hydrateRow(idx, parsed.ticker, parsed.market);
			});
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
			const result = await replaceWatchlist([]);
			const nextRows = result.symbols.map((entry) => {
				const parsed = parseEntry(entry);
				return {
					ticker: parsed.ticker,
					description: '',
					market: parsed.market
				};
			});
			normalizeRows(nextRows);
			message = t('clear_all');
		} catch (err) {
			message = formatApiError(err);
		} finally {
			saving = false;
		}
	}

	function handleDownload() {
		const symbols = toSymbols(rows);
		const payload = symbols.join('\n');
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

	async function saveRows(nextRows: WatchlistRow[]) {
		const nextSymbols = toSymbols(nextRows);
		saving = true;
		try {
			const result = await replaceWatchlist(nextSymbols);
			const rowsFromResult = result.symbols.map((entry) => {
				const parsed = parseEntry(entry);
				return {
					ticker: parsed.ticker,
					description: '',
					market: parsed.market
				};
			});
			normalizeRows(rowsFromResult);
			result.symbols.forEach((entry, idx) => {
				const parsed = parseEntry(entry);
				void hydrateRow(idx, parsed.ticker, parsed.market);
			});
		} catch (err) {
			message = formatApiError(err);
		} finally {
			saving = false;
		}
	}

	function handleRowClick(index: number) {
		editingRow = index;
	}

	function handleCancel(index: number) {
		if (editingRow === index) editingRow = null;
	}

	function handleRemove(index: number) {
		message = '';
		const removed = rows[index];
		const updated = rows.filter((_, idx) => idx !== index);
		normalizeRows(updated);
		if (editingRow !== null) {
			if (editingRow === index) editingRow = null;
			else if (editingRow > index) editingRow -= 1;
		}
		if (removed?.ticker?.trim()) {
			void saveRows(updated);
		}
	}

	function handleSelect(index: number, event: CustomEvent<{ symbol: string; name?: string; exchange?: string }>) {
		const item = event.detail;
		const nextSymbol = item.symbol.toUpperCase();
		const nextMarket = (item.exchange ?? '').toUpperCase();
		const nextKey = entryToString({ ticker: nextSymbol, market: nextMarket });
		if (rows.some((row, idx) => idx !== index && rowKey(row) === nextKey)) {
			message = t('symbol_already_added') || 'Symbol already added.';
			editingRow = null;
			return;
		}
		const updated = [...rows];
		updated[index] = {
			ticker: nextSymbol,
			description: item.name ?? '',
			market: nextMarket
		};
		normalizeRows(updated);
		editingRow = null;
		void saveRows(updated);
		if (!item.name || !item.exchange) {
			void hydrateRow(index, nextSymbol, nextMarket);
		}
	}
</script>

<h1 class="heading"><span class="heading-icon"><List size={20} strokeWidth={1.6} /></span>{t('watchlist')}</h1>

{#if message}
	<p style="color: #f87171;">{message}</p>
{/if}

<div class="card">
	<div style="display: flex; gap: 8px; flex-wrap: wrap; align-items: center;">
		<input
			type="file"
			accept=".txt,.csv"
			bind:this={fileInput}
			on:change={handleFileChange}
			style="display: none;"
		/>
		<button class="secondary" on:click={() => fileInput?.click()}>{t('reload_from_file')}</button>
		<button class="secondary" on:click={handleDownload}>{t('download')}</button>
		<button
			class="secondary"
			on:click={handleClearAll}
		>
			{t('clear_all')}
		</button>
		{#if saving}
			<span class="muted">{t('loading_chart_data')}</span>
		{/if}
	</div>

	<div style="margin-top: 16px;">
		<table class="table">
			<thead>
				<tr>
					<th style="width: 25%;">Ticker</th>
					<th>Description</th>
					<th style="width: 15%;">Market</th>
					<th style="width: 44px;"></th>
				</tr>
			</thead>
			<tbody>
				{#each rows as row, index}
					<tr>
						<td>
							{#if editingRow === index}
								<SymbolSearch
									placeholder="Search symbol"
									statusTone={twsConnected ? 'online' : 'offline'}
									includeNonStocks={true}
									autoFocus={true}
									initialValue={row.ticker}
									on:select={(event) => handleSelect(index, event)}
									on:cancel={() => handleCancel(index)}
								/>
							{:else}
								<button
									class="secondary"
									type="button"
									on:click={() => handleRowClick(index)}
									style="width: 100%; text-align: left;"
								>
									{row.ticker || '—'}
								</button>
							{/if}
						</td>
						<td>{row.description || '—'}</td>
						<td>{row.market || '—'}</td>
						<td style="text-align: right;">
							<button
								class="secondary"
								type="button"
								on:click={() => handleRemove(index)}
								aria-label={t('remove_row')}
								title={t('remove_row')}
								style="padding: 6px; display: inline-flex; align-items: center; justify-content: center; color: #ef4444;"
							>
								<Trash2 size={16} strokeWidth={1.6} />
							</button>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
</div>
