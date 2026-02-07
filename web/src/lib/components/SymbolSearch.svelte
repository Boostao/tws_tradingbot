<script lang="ts">
	import { onMount, tick, onDestroy } from 'svelte';
	import type { SymbolRecord } from '$lib/api';
	import { getSymbols } from '$lib/api';
	import { t, language } from '$lib/i18n';

	export let placeholder = '';
	export let exclude: string[] = [];
	export let maxResults = 15;
	export let statusLabel = '';
	export let statusTone: 'online' | 'offline' = 'offline';
	export let includeNonStocks = false;
	export let autoFocus = false;
	export let initialValue = '';
	export let clearOnSelect = false;
	export let onSelect: ((item: SymbolRecord) => void) | undefined = undefined;
	export let onCancel: (() => void) | undefined = undefined;

	let query = '';
	let source: string | null = null;
	let results: Array<SymbolRecord & { score: number; matchType: 'symbol' | 'name'; indices: number[] }> = [];
	let selectedIndex = -1;
	let open = false;
	let debounceTimer: ReturnType<typeof setTimeout> | null = null;
	let blurTimer: ReturnType<typeof setTimeout> | null = null;
	let loading = false;
	let inputEl: HTMLInputElement | null = null;

	$: _lang = $language;
	$: displayPlaceholder = placeholder || t('symbol_search_placeholder');
	$: statusText =
		statusLabel ||
		(source === 'tradingview'
			? t('symbol_source_live')
			: source === 'cache'
				? t('symbol_source_cached')
				: t('local_database_label'));
	$: statusBadgeTone = source === 'tradingview' ? 'online' : statusTone;

	function normalize(text?: string): string {
		return (text || '').toLowerCase();
	}

	function fuzzyMatch(text: string, q: string) {
		if (!text || !q) return { match: false, score: 0, indices: [] as number[] };
		const t = normalize(text);
		const queryLower = normalize(q);

		if (t === queryLower) return { match: true, score: 1000, indices: [...Array(queryLower.length).keys()] };
		if (t.startsWith(queryLower))
			return { match: true, score: 900 - t.length, indices: [...Array(queryLower.length).keys()] };

		const idx = t.indexOf(queryLower);
		if (idx !== -1) {
			const indices = [...Array(queryLower.length).keys()].map((i) => i + idx);
			return { match: true, score: 500 - idx, indices };
		}

		let queryIdx = 0;
		const indices: number[] = [];
		let consecutiveBonus = 0;
		for (let i = 0; i < t.length && queryIdx < queryLower.length; i++) {
			if (t[i] === queryLower[queryIdx]) {
				indices.push(i);
				if (indices.length > 1 && indices[indices.length - 1] === indices[indices.length - 2] + 1) {
					consecutiveBonus += 10;
				}
				queryIdx += 1;
			}
		}
		if (queryIdx === queryLower.length) {
			const spread = indices[indices.length - 1] - indices[0];
			const score = 100 - spread + consecutiveBonus;
			return { match: true, score: Math.max(score, 1), indices };
		}
		return { match: false, score: 0, indices: [] as number[] };
	}

	function buildResults(items: SymbolRecord[], nextQuery: string) {
		if (!nextQuery.trim()) return [];
		const upper = nextQuery.toUpperCase();
		const scored: typeof results = [];
		const excludeSet = new Set(exclude.map((s) => s.toUpperCase()));

		for (const item of items) {
			if (excludeSet.has(item.symbol.toUpperCase())) continue;
			const symbolMatch = fuzzyMatch(item.symbol, upper);
			const nameMatch = fuzzyMatch(item.name || '', upper);
			let bestScore = 0;
			let matchType: 'symbol' | 'name' = 'symbol';
			let indices: number[] = [];

			if (symbolMatch.match) {
				bestScore = symbolMatch.score + 500;
				matchType = 'symbol';
				indices = symbolMatch.indices;
			}
			if (nameMatch.match && nameMatch.score + 200 > bestScore) {
				bestScore = nameMatch.score + 200;
				matchType = 'name';
				indices = nameMatch.indices;
			}
			if (bestScore > 0) {
				if ((item.type || '').toLowerCase() === 'stock') {
					bestScore += 250;
				}
				scored.push({ ...item, score: bestScore, matchType, indices });
			}
		}

		scored.sort((a, b) => b.score - a.score);
		if (!includeNonStocks) {
			const stockMatches = scored.filter((item) => (item.type || '').toLowerCase() === 'stock');
			if (stockMatches.length) {
				return stockMatches.slice(0, maxResults);
			}
		}
		return scored.slice(0, maxResults);
	}

	function escapeHtml(value: string) {
		return value
			.replaceAll('&', '&amp;')
			.replaceAll('<', '&lt;')
			.replaceAll('>', '&gt;')
			.replaceAll('"', '&quot;')
			.replaceAll("'", '&#39;');
	}

	function highlight(text: string, indices: number[], active: boolean) {
		if (!active || indices.length === 0) return escapeHtml(text);
		let result = '';
		let lastIdx = 0;
		for (const idx of indices) {
			if (idx < text.length) {
				result += escapeHtml(text.slice(lastIdx, idx));
				result += `<span class="highlight">${escapeHtml(text[idx])}</span>`;
				lastIdx = idx + 1;
			}
		}
		result += escapeHtml(text.slice(lastIdx));
		return result;
	}

	async function fetchRemote(nextQuery: string) {
		if (!nextQuery.trim()) {
			results = [];
			selectedIndex = -1;
			open = false;
			return;
		}
		loading = true;
		try {
			const data = await getSymbols({
				q: nextQuery,
				type: includeNonStocks ? undefined : 'stock',
				limit: Math.max(50, maxResults * 4)
			});
			source = data.source ?? null;
			results = buildResults(data.symbols ?? [], nextQuery);
			selectedIndex = results.length ? 0 : -1;
			open = true;
		} finally {
			loading = false;
		}
	}

	function handleInput() {
		if (debounceTimer) clearTimeout(debounceTimer);
		debounceTimer = setTimeout(() => {
			void fetchRemote(query);
		}, 120);
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			open = false;
			query = initialValue;
			onCancel?.();
			return;
		}

		if (!open && (event.key === 'ArrowDown' || event.key === 'Enter')) {
			void fetchRemote(query);
			return;
		}
		if (!open) return;
		switch (event.key) {
			case 'ArrowDown':
				event.preventDefault();
				selectedIndex = Math.min(selectedIndex + 1, results.length - 1);
				break;
			case 'ArrowUp':
				event.preventDefault();
				selectedIndex = Math.max(selectedIndex - 1, 0);
				break;
			case 'Enter':
				event.preventDefault();
				if (selectedIndex >= 0) selectItem(results[selectedIndex]);
				break;
		}
	}

	function selectItem(item: SymbolRecord) {
		if (blurTimer) clearTimeout(blurTimer);
		query = clearOnSelect ? '' : item.symbol;
		open = false;
		onSelect?.(item);
	}

	function handleBlur() {
		blurTimer = setTimeout(() => {
			open = false;
			onCancel?.();
		}, 120);
	}

	onDestroy(() => {
		if (debounceTimer) clearTimeout(debounceTimer);
		if (blurTimer) clearTimeout(blurTimer);
	});

	onMount(async () => {
		loading = false;
		query = initialValue;
		if (autoFocus) {
			await tick();
			inputEl?.focus();
			inputEl?.select();
		}
	});
</script>

<div class="symbol-search">
	<div class="search-input">
		<span class="icon">🔍</span>
		<input
			bind:this={inputEl}
			bind:value={query}
			placeholder={displayPlaceholder}
			on:input={handleInput}
			on:keydown={handleKeydown}
			on:blur={handleBlur}
			autocomplete="off"
			spellcheck="false"
		/>
	</div>
	{#if open}
		<div class="dropdown">
			<div class="status-bar">
				<span>
					{#if loading}
						{t('loading_chart_data')}
					{:else if query.trim().length === 0}
						{t('type_to_search')}
					{:else if results.length === 0}
						{t('no_matches')}
					{:else}
						{results.length} {t('results')}{results.length !== 1 ? 's' : ''}
					{/if}
				</span>
				<span class:online={statusBadgeTone === 'online'} class:offline={statusBadgeTone !== 'online'}>
					{statusText}
				</span>
			</div>
			<div class="results">
				{#if results.length === 0 && query.trim().length > 0}
					<div class="empty">{t('no_results_for')} “{query.trim()}”</div>
				{:else}
					{#each results as item, index}
						<button
							type="button"
							class:selected={index === selectedIndex}
							on:click={() => selectItem(item)}
						>
							<span class="symbol">{@html highlight(item.symbol, item.indices, item.matchType === 'symbol')}</span>
							<span class="name">{@html highlight(item.name || '', item.indices, item.matchType === 'name')}</span>
							<span class="exchange">{item.exchange ?? 'US'}</span>
						</button>
					{/each}
				{/if}
			</div>
		</div>
	{/if}
</div>

<style>
	.symbol-search {
		position: relative;
		width: 100%;
	}

	.search-input {
		display: flex;
		align-items: center;
		gap: 8px;
		background: #0b1320;
		border: 1px solid #1f2937;
		border-radius: 10px;
		padding: 4px 12px;
	}

	.search-input input {
		flex: 1;
		border: none;
		background: transparent;
		padding: 4px;
	}

	.search-input input:focus {
		outline: none;
	}

	.icon {
		font-size: 14px;
		opacity: 0.7;
	}

	.dropdown {
		position: absolute;
		left: 0;
		right: 0;
		margin-top: 6px;
		background: #0f1720;
		border: 1px solid #1f2937;
		border-radius: 10px;
		z-index: 100;
		box-shadow: 0 10px 24px rgba(0, 0, 0, 0.35);
	}

	.status-bar {
		display: flex;
		justify-content: space-between;
		padding: 8px 12px;
		font-size: 12px;
		color: #94a3b8;
		border-bottom: 1px solid #1f2937;
	}

	.status-bar .online {
		color: #4ade80;
	}

	.status-bar .offline {
		color: #f59e0b;
	}

	.results {
		max-height: 280px;
		overflow-y: auto;
	}

	.results button {
		width: 100%;
		background: transparent;
		border: none;
		color: inherit;
		display: grid;
		grid-template-columns: 72px 1fr auto;
		gap: 12px;
		padding: 10px 12px;
		text-align: left;
		border-bottom: 1px solid #1f2937;
		cursor: pointer;
	}

	.results button:last-child {
		border-bottom: none;
	}

	.results button:hover,
	.results button.selected {
		background: rgba(148, 163, 184, 0.12);
	}

	.symbol {
		font-weight: 600;
		color: #e2e8f0;
	}

	.name {
		color: #94a3b8;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.exchange {
		font-size: 11px;
		color: #94a3b8;
		background: #111827;
		padding: 2px 6px;
		border-radius: 6px;
	}

	:global(.highlight) {
		color: #7aa2f7;
		font-weight: 600;
	}

	.empty {
		padding: 12px;
		color: #94a3b8;
		font-size: 13px;
	}
</style>
