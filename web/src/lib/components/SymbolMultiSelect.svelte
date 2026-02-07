<script lang="ts">
	import SymbolSearch from './SymbolSearch.svelte';
	import type { SymbolRecord } from '$lib/api';
	import { t, language } from '$lib/i18n';

	export let selected: string[] = [];
	export let statusTone: 'online' | 'offline' = 'offline';
	export let includeNonStocks = false;
	export let onChange: ((next: string[]) => void) | undefined = undefined;

	let lastAdded: string | null = null;

	$: _lang = $language;

	function emitChange(next: string[]) {
		selected = next;
		onChange?.(next);
	}

	function addSymbol(item: SymbolRecord) {
		const sym = item.symbol.toUpperCase();
		if (selected.includes(sym)) return;
		lastAdded = sym;
		emitChange([...selected, sym]);
	}

	function removeSymbol(sym: string) {
		emitChange(selected.filter((s) => s !== sym));
	}
</script>

<div class="multi-select">
	<SymbolSearch
		onSelect={(item) => addSymbol(item)}
		exclude={selected}
		statusTone={statusTone}
		includeNonStocks={includeNonStocks}
		clearOnSelect={true}
	/>
	<div class="chips">
		{#if selected.length === 0}
			<span class="empty">{t('no_symbols_selected')}</span>
		{:else}
			{#each selected as sym (sym)}
				<span class="chip" class:highlight={sym === lastAdded}>
					{sym}
					<button type="button" on:click={() => removeSymbol(sym)}>×</button>
				</span>
			{/each}
		{/if}
	</div>
</div>

<style>
	.multi-select {
		display: grid;
		gap: 12px;
	}

	.chips {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		min-height: 34px;
	}

	.chip {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 6px 10px;
		background: #111827;
		border: 1px solid #1f2937;
		border-radius: 999px;
		font-size: 12px;
	}

	.chip button {
		background: transparent;
		border: none;
		color: #94a3b8;
		cursor: pointer;
		font-size: 16px;
		line-height: 1;
		padding: 0 2px;
	}

	.chip button:hover {
		color: #ef4444;
	}

	.chip.highlight {
		animation: fadeHighlight 3s ease-out forwards;
	}

	@keyframes fadeHighlight {
		0% {
			background-color: #f97316; /* Orange-500 */
		}
		100% {
			background-color: #111827;
		}
	}

	.empty {
		color: #94a3b8;
		font-style: italic;
	}
</style>
