<script lang="ts">
	import { onMount } from 'svelte';
	import {
		clearCache,
		formatApiError,
		getCachePolicy,
		getLastApiError,
		getRestMetric,
		getStrategy,
		saveStrategy,
		setCacheTtl,
		validateStrategy,
		importStrategy,
		importStrategyFile,
		exportStrategy
	} from '$lib/api';
	import { t, language } from '$lib/i18n';
	import { botState } from '$lib/stores/botState';
	import SymbolMultiSelect from '$lib/components/SymbolMultiSelect.svelte';
	import {
		GitBranch,
		Settings,
		ListChecks,
		PlusCircle,
		Sliders,
		Filter,
		Zap
	} from 'lucide-svelte';

	type Indicator = {
		type: string;
		length?: number | null;
		timeframe: string;
		source: string;
		symbol?: string | null;
		params: Record<string, any>;
		component?: string | null;
	};

	type Condition = {
		type: string;
		indicator_a: Indicator;
		indicator_b?: Indicator | null;
		threshold?: number | null;
		lookback_periods: number;
		range_start?: string | null;
		range_end?: string | null;
	};

	type Rule = {
		id: string;
		name: string;
		description?: string | null;
		scope: string;
		condition: Condition;
		action: string;
		enabled: boolean;
		priority: number;
	};

	type Strategy = {
		id: string;
		name: string;
		version: string;
		description?: string | null;
		tickers: string[];
		rules: Rule[];
		initial_capital: number;
		max_positions: number;
		position_size_mode: string;
		position_size_value: number;
	};

	let indicatorTypes: Array<{ label: string; value: string }> = [];
	let conditionOptions: Array<{ label: string; value: string }> = [];
	let actionOptions: Array<{ label: string; value: string }> = [];

	const timeframeOptions = ['1m', '5m', '15m', '30m', '1h', '4h', '1d'];
	const sourceOptions = ['close', 'open', 'high', 'low', 'hl2', 'hlc3', 'ohlc4'];

	let strategy: Strategy | null = null;
	let json = '';
	let status = '';
	let errors: string[] = [];
	let message = '';
	let showJson = false;
	let strategyCacheTtl = getCachePolicy().strategy;
	let restGetMs: number | null = null;
	let restSaveMs: number | null = null;
	let restValidateMs: number | null = null;
	let restLastError: string | null = null;

	let importFile: File | null = null;

	let ruleName = '';
	let ruleScope = 'per_ticker';
	let conditionType = 'crosses_above';
	let actionType = 'buy';
	let priority = 0;
	let lookbackPeriods = 1;
	let compareMode = 'indicator';
	let thresholdValue = 0;
	let rangeStart = '09:30';
	let rangeEnd = '16:00';
	let indicatorA: Indicator = createIndicator('ema');
	let indicatorB: Indicator = createIndicator('ema');

	$: _lang = $language;
	$: indicatorTypes = [
		{ label: t('EMA'), value: 'ema' },
		{ label: t('SMA'), value: 'sma' },
		{ label: t('Price'), value: 'price' },
		{ label: t('VIX'), value: 'vix' },
		{ label: t('RSI'), value: 'rsi' },
		{ label: t('Volume'), value: 'volume' },
		{ label: t('Time'), value: 'time' },
		{ label: t('MACD'), value: 'macd' },
		{ label: t('Bollinger Bands'), value: 'bollinger' },
		{ label: t('Stochastic'), value: 'stochastic' },
		{ label: t('On-Balance Volume'), value: 'obv' },
		{ label: t('Williams Alligator'), value: 'alligator' },
		{ label: t('Dividend Yield'), value: 'dividend_yield' },
		{ label: t('P/E Ratio'), value: 'pe_ratio' },
		{ label: t('Rel. Performance'), value: 'relative_performance' },
		{ label: t('ML Signal'), value: 'ml_signal' }
	];
	$: conditionOptions = [
		{ label: t('crosses above'), value: 'crosses_above' },
		{ label: t('crosses below'), value: 'crosses_below' },
		{ label: t('greater than (>)'), value: 'greater_than' },
		{ label: t('less than (<)'), value: 'less_than' },
		{ label: t('slope above (>)'), value: 'slope_above' },
		{ label: t('slope below (<)'), value: 'slope_below' },
		{ label: t('within time range'), value: 'within_range' }
	];
	$: actionOptions = [
		{ label: t('action_buy'), value: 'buy' },
		{ label: t('action_sell'), value: 'sell' },
		{ label: t('action_filter'), value: 'filter' }
	];

	const formatIndicator = (indicator: Indicator) => {
		const type = indicator.type;
		const tf = indicator.timeframe;
		const component = indicator.component ? `[${indicator.component}]` : '';
		const sym = indicator.symbol ? `${indicator.symbol}:` : '';
		
		if (type === 'price') {
			return `${sym}Price(${indicator.source})${component}`;
		}
		if (type === 'vix') {
			return `VIX(${tf})${component}`;
		}
		if (type === 'time') {
			return `Time${component}`;
		}
		if (type === 'ml_signal') {
			return `${sym}ML Signal${component}`;
		}
		if (indicator.length) {
			return `${sym}${type.toUpperCase()}(${indicator.length}, ${tf})${component}`;
		}
		return `${sym}${type.toUpperCase()}(${tf})${component}`;
	};

	const formatRulePreview = (rule: Rule) => {
		const cond = rule.condition;
		const indA = formatIndicator(cond.indicator_a);
		switch (cond.type) {
			case 'crosses_above':
				return `${indA} crosses above ${cond.indicator_b ? formatIndicator(cond.indicator_b) : '?'}`;
			case 'crosses_below':
				return `${indA} crosses below ${cond.indicator_b ? formatIndicator(cond.indicator_b) : '?'}`;
			case 'greater_than':
				return cond.indicator_b
					? `${indA} > ${formatIndicator(cond.indicator_b)}`
					: `${indA} > ${cond.threshold ?? '?'}`;
			case 'less_than':
				return cond.indicator_b
					? `${indA} < ${formatIndicator(cond.indicator_b)}`
					: `${indA} < ${cond.threshold ?? '?'}`;
			case 'slope_above':
				return `${indA} slope > ${cond.threshold ?? 0} (over ${cond.lookback_periods} periods)`;
			case 'slope_below':
				return `${indA} slope < ${cond.threshold ?? 0} (over ${cond.lookback_periods} periods)`;
			case 'within_range':
				return `Time within ${cond.range_start ?? '?'} - ${cond.range_end ?? '?'}`;
			default:
				return `${indA} ${cond.type}`;
		}
	};

	const formatMs = (value: number | null) =>
		value === null || Number.isNaN(value) ? '--' : `${Math.round(value)}ms`;

	function updateRestBadges() {
		restGetMs = getRestMetric('strategy.get')?.lastMs ?? null;
		restSaveMs = getRestMetric('strategy.save')?.lastMs ?? null;
		restValidateMs = getRestMetric('strategy.validate')?.lastMs ?? null;
		restLastError = getLastApiError()?.message ?? null;
	}

	function createIndicator(type: string): Indicator {
		return {
			type,
			length: type === 'ema' || type === 'sma' || type === 'rsi' ? 9 : null,
			timeframe: '5m',
			source: 'close',
			symbol: null,
			params: {},
			component: null
		};
	}

	function normalizeIndicator(indicator: Indicator): Indicator {
		const next = { ...indicator, params: { ...(indicator.params ?? {}) } };
		if (['ema', 'sma', 'rsi'].includes(next.type) && !next.length) {
			next.length = 9;
		}
		if (next.type === 'macd') {
			next.params.fast_period ??= 12;
			next.params.slow_period ??= 26;
			next.params.signal_period ??= 9;
		}
		if (next.type === 'bollinger') {
			next.length ??= 20;
			next.params.std_dev ??= 2;
			next.params.offset ??= 0;
		}
		if (next.type === 'stochastic') {
			next.params.k_period ??= 14;
			next.params.d_period ??= 3;
			next.params.smooth_k ??= 3;
		}
		if (next.type === 'alligator') {
			next.params.jaw_period ??= 13;
			next.params.jaw_shift ??= 8;
			next.params.teeth_period ??= 8;
			next.params.teeth_shift ??= 5;
			next.params.lips_period ??= 5;
			next.params.lips_shift ??= 3;
		}
		if (next.type === 'ml_signal') {
			next.params.column ??= 'signal';
		}
		if (next.type === 'ml_signal' && typeof next.params.feature_columns === 'string') {
			next.params.feature_columns = (next.params.feature_columns as string)
				.split(',')
				.map((entry) => entry.trim())
				.filter(Boolean);
		}
		return next;
	}

	function updateStrategy(next: Strategy) {
		strategy = next;
		json = JSON.stringify(next, null, 2);
	}

	function refreshJson() {
		if (strategy) {
			json = JSON.stringify(strategy, null, 2);
		}
	}

	function buildNewRule(): Rule {
		const condition: Condition = {
			type: conditionType,
			indicator_a: normalizeIndicator(indicatorA),
			indicator_b: null,
			threshold: null,
			lookback_periods: lookbackPeriods,
			range_start: null,
			range_end: null
		};

		const needsIndicatorB = ['crosses_above', 'crosses_below', 'greater_than', 'less_than'].includes(
			conditionType
		);
		const needsThreshold = ['greater_than', 'less_than', 'slope_above', 'slope_below'].includes(
			conditionType
		);

		if (conditionType === 'within_range') {
			condition.range_start = rangeStart;
			condition.range_end = rangeEnd;
		} else if (needsIndicatorB && (!needsThreshold || compareMode === 'indicator')) {
			condition.indicator_b = normalizeIndicator(indicatorB);
		} else if (needsThreshold) {
			condition.threshold = thresholdValue;
		}

		return {
			id: crypto.randomUUID(),
			name: ruleName || 'Unnamed Rule',
			scope: ruleScope,
			action: actionType,
			enabled: true,
			priority,
			condition
		};
	}

	async function loadStrategy(force = false) {
		try {
			const data = await getStrategy(force);
			updateStrategy(data as Strategy);
		} catch (err) {
			message = formatApiError(err);
		} finally {
			updateRestBadges();
		}
	}

	onMount(loadStrategy);

	async function handleValidate() {
		message = '';
		errors = [];
		try {
			if (!strategy) return;
			const result = await validateStrategy(strategy);
			status = result.valid ? 'valid' : 'invalid';
			errors = result.errors;
		} catch (err) {
			message = formatApiError(err);
		} finally {
			updateRestBadges();
		}
	}

	async function handleSave() {
		message = '';
		errors = [];
		try {
			if (!strategy) return;
			const saved = await saveStrategy(strategy);
			updateStrategy(saved as Strategy);
			status = 'saved';
		} catch (err) {
			message = formatApiError(err);
		} finally {
			updateRestBadges();
		}
	}

	async function handleExport() {
		message = '';
		try {
			// Trigger file download
			const link = document.createElement('a');
			link.href = `${API_BASE}/api/v1/strategy/export`;
			link.download = '';
			document.body.appendChild(link);
			link.click();
			document.body.removeChild(link);
			status = 'exported';
		} catch (err) {
			message = formatApiError(err);
		} finally {
			updateRestBadges();
		}
	}

	async function handleImport() {
		message = '';
		try {
			const parsed = JSON.parse(json) as Strategy;
			const data = await importStrategy(parsed);
			updateStrategy(data as Strategy);
			status = 'imported';
		} catch (err) {
			message = formatApiError(err);
		} finally {
			updateRestBadges();
		}
	}

	async function handleImportFile() {
		if (!importFile) {
			message = 'Please select a file to import';
			return;
		}
		message = '';
		try {
			const data = await importStrategyFile(importFile);
			updateStrategy(data as Strategy);
			status = 'imported from file';
			importFile = null; // Reset
		} catch (err) {
			message = formatApiError(err);
		} finally {
			updateRestBadges();
		}
	}

	function addRule() {
		if (!strategy) return;
		if (!ruleName.trim()) {
			message = t('rule_name_required');
			return;
		}
		const newRule = buildNewRule();
		const next = { ...strategy, rules: [...strategy.rules, newRule] };
		updateStrategy(next);
		message = t('rule_added_success', { name: newRule.name });
		ruleName = '';
	}

	function updateTickers(next: string[]) {
		if (!strategy) return;
		updateStrategy({ ...strategy, tickers: next });
	}

	function removeRule(id: string) {
		if (!strategy) return;
		const next = { ...strategy, rules: strategy.rules.filter((rule) => rule.id !== id) };
		updateStrategy(next);
	}

	function toggleRule(id: string) {
		if (!strategy) return;
		const next = {
			...strategy,
			rules: strategy.rules.map((rule) =>
				rule.id === id ? { ...rule, enabled: !rule.enabled } : rule
			)
		};
		updateStrategy(next);
	}

	$: needsIndicatorB = ['crosses_above', 'crosses_below', 'greater_than', 'less_than'].includes(
		conditionType
	);
	$: needsThreshold = ['greater_than', 'less_than', 'slope_above', 'slope_below'].includes(
		conditionType
	);
	$: needsRange = conditionType === 'within_range';

	$: previewRule = buildNewRule();
</script>

<h1 class="heading"><span class="heading-icon"><GitBranch size={20} strokeWidth={1.6} /></span>{t('strategy_builder')}</h1>

{#if message}
	<p style="color: #f87171;">{message}</p>
{/if}

{#if strategy}
	<div class="card">
		<h2 class="heading"><span class="heading-icon"><Settings size={18} strokeWidth={1.6} /></span>{t('configuration')}</h2>
		<div style="display: grid; gap: 12px; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));">
			<label>
				{t('strategy_name')}
				<input bind:value={strategy.name} />
			</label>
			<label>
				{t('description')}
				<input bind:value={strategy.description} />
			</label>
			<label>
				{t('version')}
				<input bind:value={strategy.version} />
			</label>
		</div>
		<div style="margin-top: 12px;">
			<span class="muted" style="display: block; margin-bottom: 6px;">
				{t('select_tickers').replaceAll('**', '')}
			</span>
			<SymbolMultiSelect
				selected={strategy.tickers}
				statusTone={$botState.tws_connected ? 'online' : 'offline'}
				on:change={(event) => updateTickers(event.detail)}
			/>
		</div>
		<p class="muted" style="margin-top: 8px;">
			{t('rules')}: {strategy.rules.length} • {t('scope_global')}:
			{strategy.rules.filter((r) => r.scope === 'global').length} • {t('scope_per_ticker')}:
			{strategy.rules.filter((r) => r.scope === 'per_ticker').length}
		</p>
	</div>

	<div class="card">
		<h2 class="heading"><span class="heading-icon"><ListChecks size={18} strokeWidth={1.6} /></span>{t('trading_rules')}</h2>
		{#if strategy.rules.length === 0}
			<p class="muted">{t('no_rules_defined')}</p>
		{:else}
			{#each strategy.rules as rule}
				<div style="border: 1px solid #1f2937; border-radius: 10px; padding: 12px; margin-bottom: 10px;">
					<div style="display: flex; justify-content: space-between; gap: 12px; flex-wrap: wrap;">
						<div>
							<strong>{rule.name}</strong>
							<p class="muted" style="margin: 4px 0;">
								{rule.scope === 'global' ? t('scope_badge_global') : t('scope_badge_per_ticker')} •
								{rule.action.toUpperCase()}
							</p>
							<p class="muted" style="margin: 0;">{formatRulePreview(rule)}</p>
						</div>
						<div style="display: flex; gap: 8px; align-items: center;">
							<label class="toggle">
								<input type="checkbox" checked={rule.enabled} on:change={() => toggleRule(rule.id)} />
								<span class="toggle-slider"></span>
								<span class="toggle-label">{t('enabled')}</span>
							</label>
							<button class="secondary" on:click={() => removeRule(rule.id)}>{t('delete_rule')}</button>
						</div>
					</div>
				</div>
			{/each}
		{/if}
	</div>

	<div class="card">
		<h2 class="heading"><span class="heading-icon"><PlusCircle size={18} strokeWidth={1.6} /></span>{t('add_new_rule')}</h2>
		<div style="display: grid; gap: 12px; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));">
			<label>
				{t('rule_name')}
				<input bind:value={ruleName} placeholder={t('rule_name_placeholder')} />
			</label>
			<label>
				{t('scope')}
				<select bind:value={ruleScope}>
					<option value="per_ticker">{t('scope_per_ticker')}</option>
					<option value="global">{t('scope_global')}</option>
				</select>
			</label>
			<label>
				{t('action')}
				<select bind:value={actionType}>
					{#each actionOptions as opt}
						<option value={opt.value}>{opt.label}</option>
					{/each}
				</select>
			</label>
			<label>
				{t('priority')}
				<input type="number" bind:value={priority} min="0" max="100" />
			</label>
		</div>

		<hr style="margin: 16px 0; border-color: #1f2937;" />

		<h3 class="heading"><span class="heading-icon"><Sliders size={16} strokeWidth={1.6} /></span>{t('indicator_a')}</h3>
		<div style="display: grid; gap: 12px; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));">
			<label>
				{t('indicator_type')}
				<select bind:value={indicatorA.type}>
					{#each indicatorTypes as opt}
						<option value={opt.value}>{opt.label}</option>
					{/each}
				</select>
			</label>
			<label>
				{t('timeframe')}
				<select bind:value={indicatorA.timeframe}>
					{#each timeframeOptions as tf}
						<option value={tf}>{tf}</option>
					{/each}
				</select>
			</label>
			<label>
				{t('source')}
				<select bind:value={indicatorA.source}>
					{#each sourceOptions as src}
						<option value={src}>{src}</option>
					{/each}
				</select>
			</label>
			<label>
				{t('symbol')}
				<input type="text" bind:value={indicatorA.symbol} placeholder="Default (Current)" />
			</label>
			{#if ['ema', 'sma', 'rsi'].includes(indicatorA.type)}
				<label>
					{t('length')}
					<input type="number" bind:value={indicatorA.length} min="1" max="500" />
				</label>
			{/if}
			{#if indicatorA.type === 'macd'}
				<label>
					{t('fast_length')}
					<input type="number" bind:value={indicatorA.params.fast_period} />
				</label>
				<label>
					{t('slow_length')}
					<input type="number" bind:value={indicatorA.params.slow_period} />
				</label>
				<label>
					{t('signal_length')}
					<input type="number" bind:value={indicatorA.params.signal_period} />
				</label>
				<label>
					{t('output')}
					<select bind:value={indicatorA.component}>
						<option value="macd">macd</option>
						<option value="signal">signal</option>
						<option value="histogram">histogram</option>
					</select>
				</label>
			{/if}
			{#if indicatorA.type === 'bollinger'}
				<label>
					{t('length')}
					<input type="number" bind:value={indicatorA.length} />
				</label>
				<label>
					{t('std_dev')}
					<input type="number" bind:value={indicatorA.params.std_dev} step="0.1" />
				</label>
				<label>
					{t('offset')}
					<input type="number" bind:value={indicatorA.params.offset} />
				</label>
				<label>
					{t('band')}
					<select bind:value={indicatorA.component}>
						<option value="upper">upper</option>
						<option value="middle">middle</option>
						<option value="lower">lower</option>
					</select>
				</label>
			{/if}
			{#if indicatorA.type === 'stochastic'}
				<label>
					{t('k_length')}
					<input type="number" bind:value={indicatorA.params.k_period} />
				</label>
				<label>
					{t('d_smoothing')}
					<input type="number" bind:value={indicatorA.params.d_period} />
				</label>
				<label>
					{t('k_smoothing')}
					<input type="number" bind:value={indicatorA.params.smooth_k} />
				</label>
				<label>
					{t('line')}
					<select bind:value={indicatorA.component}>
						<option value="k">k</option>
						<option value="d">d</option>
					</select>
				</label>
			{/if}
			{#if indicatorA.type === 'alligator'}
				<label>
					{t('jaw_length')}
					<input type="number" bind:value={indicatorA.params.jaw_period} />
				</label>
				<label>
					{t('jaw_offset')}
					<input type="number" bind:value={indicatorA.params.jaw_shift} />
				</label>
				<label>
					{t('teeth_length')}
					<input type="number" bind:value={indicatorA.params.teeth_period} />
				</label>
				<label>
					{t('teeth_offset')}
					<input type="number" bind:value={indicatorA.params.teeth_shift} />
				</label>
				<label>
					{t('lips_length')}
					<input type="number" bind:value={indicatorA.params.lips_period} />
				</label>
				<label>
					{t('lips_offset')}
					<input type="number" bind:value={indicatorA.params.lips_shift} />
				</label>
				<label>
					{t('line')}
					<select bind:value={indicatorA.component}>
						<option value="jaw">jaw</option>
						<option value="teeth">teeth</option>
						<option value="lips">lips</option>
					</select>
				</label>
			{/if}
			{#if indicatorA.type === 'ml_signal'}
				<label>
					{t('model_path')}
					<input type="text" bind:value={indicatorA.params.model_path} placeholder="models/signal.onnx" />
				</label>
				<label>
					{t('signal_column')}
					<input type="text" bind:value={indicatorA.params.column} placeholder="signal" />
				</label>
				<label>
					{t('feature_columns')}
					<input type="text" bind:value={indicatorA.params.feature_columns} placeholder="open,high,low" />
				</label>
			{/if}
		</div>

		<hr style="margin: 16px 0; border-color: #1f2937;" />

		<h3 class="heading"><span class="heading-icon"><Filter size={16} strokeWidth={1.6} /></span>{t('condition_type')}</h3>
		<div style="display: grid; gap: 12px; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));">
			<label>
				{t('condition_type')}
				<select bind:value={conditionType}>
					{#each conditionOptions as opt}
						<option value={opt.value}>{opt.label}</option>
					{/each}
				</select>
			</label>
			<label>
				{t('lookback_periods')}
				<input type="number" bind:value={lookbackPeriods} min="1" max="100" />
			</label>
			{#if needsThreshold && needsIndicatorB}
				<label>
					{t('compare_to')}
					<select bind:value={compareMode}>
						<option value="indicator">{t('compare_to_indicator')}</option>
						<option value="threshold">{t('compare_to_threshold')}</option>
					</select>
				</label>
			{/if}
		</div>

		{#if needsIndicatorB && (!needsThreshold || compareMode === 'indicator')}
			<hr style="margin: 16px 0; border-color: #1f2937;" />
			<h3 class="heading"><span class="heading-icon"><Sliders size={16} strokeWidth={1.6} /></span>{t('indicator_b')}</h3>
			<div style="display: grid; gap: 12px; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));">
				<label>
					{t('indicator_type')}
					<select bind:value={indicatorB.type}>
						{#each indicatorTypes as opt}
							<option value={opt.value}>{opt.label}</option>
						{/each}
					</select>
				</label>
				<label>
					{t('timeframe')}
					<select bind:value={indicatorB.timeframe}>
						{#each timeframeOptions as tf}
							<option value={tf}>{tf}</option>
						{/each}
					</select>
				</label>
				<label>
					{t('source')}
					<select bind:value={indicatorB.source}>
						{#each sourceOptions as src}
							<option value={src}>{src}</option>
						{/each}
					</select>
				</label>
				<label>
					{t('symbol')}
					<input type="text" bind:value={indicatorB.symbol} placeholder="Default (Current)" />
				</label>
				{#if ['ema', 'sma', 'rsi'].includes(indicatorB.type)}
					<label>
						{t('length')}
						<input type="number" bind:value={indicatorB.length} min="1" max="500" />
					</label>
				{/if}
				{#if indicatorB.type === 'macd'}
					<label>
						{t('fast_length')}
						<input type="number" bind:value={indicatorB.params.fast_period} />
					</label>
					<label>
						{t('slow_length')}
						<input type="number" bind:value={indicatorB.params.slow_period} />
					</label>
					<label>
						{t('signal_length')}
						<input type="number" bind:value={indicatorB.params.signal_period} />
					</label>
					<label>
						{t('output')}
						<select bind:value={indicatorB.component}>
							<option value="macd">macd</option>
							<option value="signal">signal</option>
							<option value="histogram">histogram</option>
						</select>
					</label>
				{/if}
				{#if indicatorB.type === 'bollinger'}
					<label>
						{t('length')}
						<input type="number" bind:value={indicatorB.length} />
					</label>
					<label>
						{t('std_dev')}
						<input type="number" bind:value={indicatorB.params.std_dev} step="0.1" />
					</label>
					<label>
						{t('offset')}
						<input type="number" bind:value={indicatorB.params.offset} />
					</label>
					<label>
						{t('band')}
						<select bind:value={indicatorB.component}>
							<option value="upper">upper</option>
							<option value="middle">middle</option>
							<option value="lower">lower</option>
						</select>
					</label>
				{/if}
				{#if indicatorB.type === 'stochastic'}
					<label>
						{t('k_length')}
						<input type="number" bind:value={indicatorB.params.k_period} />
					</label>
					<label>
						{t('d_smoothing')}
						<input type="number" bind:value={indicatorB.params.d_period} />
					</label>
					<label>
						{t('k_smoothing')}
						<input type="number" bind:value={indicatorB.params.smooth_k} />
					</label>
					<label>
						{t('line')}
						<select bind:value={indicatorB.component}>
							<option value="k">k</option>
							<option value="d">d</option>
						</select>
					</label>
				{/if}
				{#if indicatorB.type === 'alligator'}
					<label>
						{t('jaw_length')}
						<input type="number" bind:value={indicatorB.params.jaw_period} />
					</label>
					<label>
						{t('jaw_offset')}
						<input type="number" bind:value={indicatorB.params.jaw_shift} />
					</label>
					<label>
						{t('teeth_length')}
						<input type="number" bind:value={indicatorB.params.teeth_period} />
					</label>
					<label>
						{t('teeth_offset')}
						<input type="number" bind:value={indicatorB.params.teeth_shift} />
					</label>
					<label>
						{t('lips_length')}
						<input type="number" bind:value={indicatorB.params.lips_period} />
					</label>
					<label>
						{t('lips_offset')}
						<input type="number" bind:value={indicatorB.params.lips_shift} />
					</label>
					<label>
						{t('line')}
						<select bind:value={indicatorB.component}>
							<option value="jaw">jaw</option>
							<option value="teeth">teeth</option>
							<option value="lips">lips</option>
						</select>
					</label>
				{/if}
				{#if indicatorB.type === 'ml_signal'}
					<label>
						{t('model_path')}
						<input type="text" bind:value={indicatorB.params.model_path} placeholder="models/signal.onnx" />
					</label>
					<label>
						{t('signal_column')}
						<input type="text" bind:value={indicatorB.params.column} placeholder="signal" />
					</label>
					<label>
						{t('feature_columns')}
						<input type="text" bind:value={indicatorB.params.feature_columns} placeholder="open,high,low" />
					</label>
				{/if}
			</div>
		{/if}

		{#if needsThreshold && (!needsIndicatorB || compareMode === 'threshold')}
			<div style="margin-top: 12px;">
				<label>
					{t('threshold_value')}
					<input type="number" bind:value={thresholdValue} step="0.01" />
				</label>
			</div>
		{/if}

		{#if needsRange}
			<div style="display: grid; gap: 12px; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); margin-top: 12px;">
				<label>
					{t('start_time')}
					<input type="text" bind:value={rangeStart} />
				</label>
				<label>
					{t('end_time')}
					<input type="text" bind:value={rangeEnd} />
				</label>
			</div>
		{/if}

		<div style="margin-top: 16px;">
			<p class="muted">{t('preview')} {formatRulePreview(previewRule)}</p>
			<button on:click={addRule}>{t('add_rule')}</button>
		</div>
	</div>

	<div class="card">
		<h2 class="heading"><span class="heading-icon"><Zap size={18} strokeWidth={1.6} /></span>{t('actions')}</h2>
		<div style="display: flex; flex-wrap: wrap; gap: 8px;">
			<button on:click={handleValidate}>{t('validate')}</button>
			<button on:click={handleSave}>{t('save')}</button>
			<button class="secondary" on:click={handleExport}>{t('export')}</button>
			<button class="secondary" on:click={() => loadStrategy(true)}>{t('reload')}</button>
			<button
				class="secondary"
				on:click={() => {
					clearCache('strategy');
					status = 'cache cleared';
				}}
			>
				{t('clear_all')}
			</button>
			<button
				class="secondary"
				on:click={() => {
					showJson = !showJson;
					if (showJson) refreshJson();
				}}
			>
				{showJson ? t('hide_json') : t('show_json')}
			</button>
		</div>
		<div style="display: flex; flex-wrap: wrap; gap: 8px; margin-top: 8px;">
			<span class="badge">REST get: {formatMs(restGetMs)}</span>
			<span class="badge">REST save: {formatMs(restSaveMs)}</span>
			<span class="badge">REST validate: {formatMs(restValidateMs)}</span>
			<span class="badge">Last API error: {restLastError ?? '--'}</span>
		</div>
		<div style="display: flex; flex-wrap: wrap; gap: 8px; margin-top: 8px; align-items: center;">
			<label class="muted" style="display: flex; gap: 6px; align-items: center;">
				Strategy cache TTL (ms)
				<input
					type="number"
					min="0"
					bind:value={strategyCacheTtl}
					on:change={(event) => {
						const next = Number((event.target as HTMLInputElement).value);
						strategyCacheTtl = next;
						setCacheTtl('strategy', next);
						status = 'cache ttl updated';
					}}
				/>
			</label>
		</div>
		{#if status}
			<p class="muted" style="margin-top: 8px;">{t('status_label')}: {status}</p>
		{/if}
		{#if errors.length}
			<ul>
				{#each errors as err}
					<li style="color: #f87171;">{err}</li>
				{/each}
			</ul>
		{/if}
	</div>

	{#if showJson}
		<div class="card">
			<h2>{t('strategy_json')}</h2>
			<div style="margin-bottom: 12px;">
				<label>
					{t('import_from_file')}
					<input type="file" accept=".json" on:change={(e) => importFile = (e.target as HTMLInputElement).files?.[0] || null} />
				</label>
				<button on:click={handleImportFile} disabled={!importFile}>{t('import_file')}</button>
			</div>
			<textarea rows="16" style="width: 100%;" bind:value={json}></textarea>
			<div style="margin-top: 12px; display: flex; gap: 8px;">
				<button on:click={handleImport}>{t('import_strategy')}</button>
				<button class="secondary" on:click={handleExport}>{t('refresh')}</button>
			</div>
		</div>
	{/if}
{:else}
	<p class="muted">Loading strategy...</p>
{/if}
