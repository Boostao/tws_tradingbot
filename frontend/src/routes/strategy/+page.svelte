<script lang="ts">
	import { onMount } from 'svelte';
	import { ExportStrategyFile, ImportStrategyFile } from '../../../bindings/tws_traderbot/app';
	import {
		applyStrategyPreset,
		deleteStrategyPreset,
		formatApiError,
		getStrategy,
		getStrategyLibrary,
		getStrategyPreset,
		importStrategy,
		importStrategyFile,
		moveStrategyOrder,
		saveStrategy,
		saveStrategyPreset,
		toggleStrategyPin,
		updateStrategyPreset,
		validateStrategy,
		type StrategyLibraryEntry
	} from '$lib/api';
	import { language, t } from '$lib/i18n';
	import { helpDrawerOpen, tourStore } from '$lib/stores/onboarding';
	import {
		ArrowDown,
		ArrowUp,
		CheckCircle2,
		Code2,
		Download,
		Filter,
		GitBranch,
		Pin,
		Play,
		Plus,
		Save,
		Sliders,
		Trash2,
		Upload,
		X
	} from 'lucide-svelte';

	type Indicator = {
		type: string;
		length?: number | null;
		period?: number | null;
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
		tickers?: string[];
		rules: Rule[];
		initial_capital: number;
		max_positions: number;
		position_size_mode: string;
		position_size_value: number;
	};

	type WorkspaceFilter = 'all' | 'achat' | 'short' | 'swing' | 'pinned';

	const timeframeOptions = ['1m', '5m', '15m', '30m', '1h', '4h', '1d', '1w', '1M'];
	const sourceOptions = ['close', 'open', 'high', 'low', 'hl2', 'hlc3', 'ohlc4', 'volume'];

	const SWING_PRESET_IDS = new Set([
		'style_08',
		'style_10',
		'style_11',
		'style_12',
		'style_f',
		'screener_golden_cross',
		'cours_weinstein_p2',
		'cours_revirement_30m'
	]);
	const SHORT_PRESET_IDS = new Set(['screener_10_30_bas']);

	let strategy: Strategy | null = null;
	let json = '';
	let validationStatus: 'idle' | 'valid' | 'invalid' = 'idle';
	let errors: string[] = [];
	let message = '';
	let showJson = false;
	let presetName = '';
	let strategyLibrary: StrategyLibraryEntry[] = [];
	let currentPresetId: string | null = null;
	let librarySearch = '';
	let workspaceFilter: WorkspaceFilter = 'all';
	let tickersInput = '';
	let fileInputEl: HTMLInputElement | null = null;

	// Rule inspector form state
	let editingRuleId: string | null = null;
	let ruleName = '';
	let ruleScope = 'per_ticker';
	let conditionType = 'crosses_above';
	let actionType = 'buy';
	let priority = 50;
	let lookbackPeriods = 1;
	let compareMode = 'indicator';
	let thresholdValue = 0;
	let rangeStart = '09:30';
	let rangeEnd = '16:00';
	let indicatorA: Indicator = createIndicator('ema');
	let indicatorB: Indicator = createIndicator('ema');

	$: _lang = $language;
	$: tr = (key: string, params?: Record<string, string | number>) => {
		void _lang;
		return t(key, params);
	};

	$: if (strategy && !strategy.rules) {
		strategy.rules = [];
	}

	$: indicatorTypes = [
		{ label: tr('Price'), value: 'price' },
		{ label: tr('Volume'), value: 'volume' },
		{ label: tr('EMA'), value: 'ema' },
		{ label: tr('SMA'), value: 'sma' },
		{ label: tr('RSI'), value: 'rsi' },
		{ label: tr('MACD'), value: 'macd' },
		{ label: tr('VIX'), value: 'vix' },
		{ label: tr('Time'), value: 'time' },
		{ label: tr('Bollinger Bands'), value: 'bollinger' },
		{ label: tr('Stochastic'), value: 'stochastic' },
		{ label: tr('On-Balance Volume'), value: 'obv' },
		{ label: tr('Williams Alligator'), value: 'alligator' },
		{ label: tr('Heikin-Ashi'), value: 'heikin_ashi' },
		{ label: tr('RVOL'), value: 'rvol' },
		{ label: tr('Guppy MMA'), value: 'guppy' },
		{ label: tr('ATR'), value: 'atr' },
		{ label: tr('Relative Strength'), value: 'relative_strength' },
		{ label: tr('52W High'), value: 'high_52w' },
		{ label: tr('Dividend Yield'), value: 'dividend_yield' },
		{ label: tr('P/E Ratio'), value: 'pe_ratio' },
		{ label: tr('Rel. Performance'), value: 'relative_performance' },
		{ label: tr('ML Signal'), value: 'ml_signal' }
	];

	$: conditionOptions = [
		{ label: tr('crosses above'), value: 'crosses_above' },
		{ label: tr('crosses below'), value: 'crosses_below' },
		{ label: tr('greater than (>)'), value: 'greater_than' },
		{ label: tr('less than (<)'), value: 'less_than' },
		{ label: tr('slope above (>)'), value: 'slope_above' },
		{ label: tr('slope below (<)'), value: 'slope_below' },
		{ label: tr('within time range'), value: 'within_range' }
	];

	$: actionOptions = [
		{ label: tr('action_filter'), value: 'filter' },
		{ label: tr('action_buy'), value: 'buy' },
		{ label: tr('action_sell'), value: 'sell' }
	];

	$: sortedLibrary = [...strategyLibrary].sort((a, b) => {
		if (Boolean(a.pinned) !== Boolean(b.pinned)) return a.pinned ? -1 : 1;
		const oa = typeof a.sort_order === 'number' ? a.sort_order : 0;
		const ob = typeof b.sort_order === 'number' ? b.sort_order : 0;
		if (oa !== ob) return oa - ob;
		return a.name.localeCompare(b.name);
	});

	$: normalizedSearch = librarySearch.trim().toLowerCase();
	$: filteredLibrary = sortedLibrary.filter((entry) => {
		if (normalizedSearch && !entry.name.toLowerCase().includes(normalizedSearch)) {
			return false;
		}
		if (workspaceFilter === 'all') return true;
		if (workspaceFilter === 'pinned') return Boolean(entry.pinned);
		return classifyPresetWorkspace(entry) === workspaceFilter;
	});

	$: pipelineStages = [
		{ action: 'filter', title: tr('strategy_stage_filter_title') },
		{ action: 'buy', title: tr('strategy_stage_buy_title') },
		{ action: 'sell', title: tr('strategy_stage_sell_title') }
	];

	$: needsIndicatorB = ['crosses_above', 'crosses_below', 'greater_than', 'less_than'].includes(
		conditionType
	);
	$: needsThreshold = ['greater_than', 'less_than', 'slope_above', 'slope_below'].includes(
		conditionType
	);
	$: needsRange = conditionType === 'within_range';

	function classifyPresetWorkspace(entry: StrategyLibraryEntry): 'achat' | 'short' | 'swing' {
		const rawWs = String((entry as Record<string, unknown>).workspace ?? '')
			.trim()
			.toLowerCase();
		if (rawWs === 'short') return 'short';
		if (rawWs === 'swing') return 'swing';
		if (rawWs === 'achat' || rawWs === 'long' || rawWs === 'buy') return 'achat';

		const id = entry.id.toLowerCase();
		const name = entry.name.toLowerCase();
		if (
			SHORT_PRESET_IDS.has(id) ||
			name.includes('short') ||
			name.includes('baissier') ||
			name.includes('découvert')
		) {
			return 'short';
		}
		if (
			SWING_PRESET_IDS.has(id) ||
			name.includes('swing') ||
			name.includes('weinstein') ||
			name.includes('hebdo') ||
			name.includes('phase 2') ||
			name.includes('guppy') ||
			name.includes('30 min')
		) {
			return 'swing';
		}
		return 'achat';
	}

	function canMoveInGroup(entry: StrategyLibraryEntry, direction: 'up' | 'down'): boolean {
		const group = sortedLibrary.filter((item) => Boolean(item.pinned) === Boolean(entry.pinned));
		const idx = group.findIndex((item) => item.id === entry.id);
		if (idx === -1) return false;
		return direction === 'up' ? idx > 0 : idx < group.length - 1;
	}

	function indicatorDisplayLabel(type: string): string {
		const found = indicatorTypes.find((it) => it.value === type);
		if (found) return found.label;
		return type.toUpperCase();
	}

	function formatIndicator(indicator: Indicator): string {
		const type = indicator.type;
		const tf = indicator.timeframe || '1d';
		const component = indicator.component ? `[${indicator.component}]` : '';
		const sym = indicator.symbol ? `${indicator.symbol}:` : '';
		const len = indicator.length ?? indicator.period;

		if (type === 'price') {
			return `${sym}${tr('Price')}(${indicator.source || 'close'}, ${tf})${component}`;
		}
		if (type === 'vix') {
			return `VIX(${tf})${component}`;
		}
		if (type === 'time') {
			return `${tr('Time')}${component}`;
		}
		if (type === 'ml_signal') {
			return `${sym}${tr('ML Signal')}${component}`;
		}
		const label = indicatorDisplayLabel(type);
		if (len) {
			return `${sym}${label}(${len}, ${tf})${component}`;
		}
		return `${sym}${label}(${tf})${component}`;
	}

	function formatRulePreview(rule: Rule): string {
		const cond = rule.condition;
		if (!cond || !cond.indicator_a) return rule.name;
		const indA = formatIndicator(cond.indicator_a);
		switch (cond.type) {
			case 'crosses_above':
			case 'cross_above':
				return `${indA} ${tr('strategy_cond_crosses_above')} ${
					cond.indicator_b ? formatIndicator(cond.indicator_b) : cond.threshold ?? '?'
				}`;
			case 'crosses_below':
			case 'cross_below':
				return `${indA} ${tr('strategy_cond_crosses_below')} ${
					cond.indicator_b ? formatIndicator(cond.indicator_b) : cond.threshold ?? '?'
				}`;
			case 'greater_than':
				return cond.indicator_b
					? `${indA} > ${formatIndicator(cond.indicator_b)}`
					: `${indA} > ${cond.threshold ?? '?'}`;
			case 'less_than':
				return cond.indicator_b
					? `${indA} < ${formatIndicator(cond.indicator_b)}`
					: `${indA} < ${cond.threshold ?? '?'}`;
			case 'slope_above':
				return `${indA} ${tr('strategy_cond_slope_over', {
					op: '>',
					threshold: cond.threshold ?? 0,
					n: cond.lookback_periods ?? 1
				})}`;
			case 'slope_below':
				return `${indA} ${tr('strategy_cond_slope_over', {
					op: '<',
					threshold: cond.threshold ?? 0,
					n: cond.lookback_periods ?? 1
				})}`;
			case 'within_range':
				return `${tr('Time')} ${tr('strategy_cond_within', {
					start: cond.range_start ?? '?',
					end: cond.range_end ?? '?'
				})}`;
			default:
				return `${indA}`;
		}
	}

	function defaultLengthForType(type: string): number | null {
		if (type === 'ema' || type === 'sma') return 9;
		if (type === 'rsi' || type === 'atr') return 14;
		if (type === 'bollinger') return 20;
		if (type === 'rvol' || type === 'relative_strength' || type === 'relative_performance') {
			return 100;
		}
		if (type === 'high_52w') return 252;
		return null;
	}

	function createIndicator(type: string): Indicator {
		return normalizeIndicator({
			type,
			length: defaultLengthForType(type),
			period: defaultLengthForType(type),
			timeframe: '1d',
			source: type === 'volume' || type === 'rvol' ? 'volume' : 'close',
			symbol: null,
			params: {},
			component: null
		});
	}

	function normalizeIndicator(indicator: Indicator): Indicator {
		const rawParams = { ...(indicator.params ?? {}) };
		const resolvedLen =
			indicator.length ??
			indicator.period ??
			(typeof rawParams.period === 'number' ? rawParams.period : null) ??
			(typeof rawParams.length === 'number' ? rawParams.length : null) ??
			defaultLengthForType(indicator.type);

		const next: Indicator = {
			...indicator,
			timeframe: indicator.timeframe || '1d',
			source: indicator.source || 'close',
			length: resolvedLen,
			period: resolvedLen,
			params: rawParams
		};

		if (next.type === 'macd') {
			const fast = next.params.fast ?? next.params.fast_period ?? 12;
			const slow = next.params.slow ?? next.params.slow_period ?? 26;
			const sig = next.params.signal ?? next.params.signal_period ?? 9;
			next.params = { fast, slow, signal: sig };
			if (!next.component) next.component = 'macd';
		} else if (next.type === 'stochastic') {
			const kLen = next.params.k_length ?? next.params.k_period ?? 15;
			const kSm = next.params.k_smooth ?? next.params.smooth_k ?? 5;
			const dSm = next.params.d_smooth ?? next.params.d_period ?? 5;
			next.params = { k_length: kLen, k_smooth: kSm, d_smooth: dSm };
			if (!next.component) next.component = 'k';
		} else if (next.type === 'bollinger') {
			const len = next.length ?? 20;
			next.length = len;
			next.period = len;
			next.params.std_dev = next.params.std_dev ?? next.params.multiplier ?? 2.0;
			if (next.component === 'middle') next.component = 'basis';
			if (!next.component) next.component = 'basis';
		} else if (next.type === 'heikin_ashi') {
			if (!next.component) next.component = 'close';
		} else if (next.type === 'guppy') {
			if (!next.component) next.component = 'fast_max';
		} else if (next.type === 'atr') {
			const len = next.length ?? 14;
			next.length = len;
			next.period = len;
			if (!next.component) next.component = 'atr';
		} else if (next.type === 'alligator') {
			if (!next.component) next.component = 'jaw';
		} else if (next.type === 'ml_signal') {
			next.params.column ??= 'signal';
			if (typeof next.params.feature_columns === 'string') {
				next.params.feature_columns = (next.params.feature_columns as string)
					.split(',')
					.map((entry) => entry.trim())
					.filter(Boolean);
			}
		}

		if (resolvedLen && !['macd', 'stochastic', 'heikin_ashi', 'guppy', 'alligator', 'price', 'volume', 'vix', 'time', 'obv', 'ml_signal', 'dividend_yield', 'pe_ratio'].includes(next.type)) {
			next.length = resolvedLen;
			next.period = resolvedLen;
		}
		return next;
	}

	function handleIndicatorTypeChange(which: 'A' | 'B') {
		if (which === 'A') {
			indicatorA = normalizeIndicator({
				...indicatorA,
				length: defaultLengthForType(indicatorA.type),
				period: defaultLengthForType(indicatorA.type),
				component: null,
				params: {}
			});
		} else {
			indicatorB = normalizeIndicator({
				...indicatorB,
				length: defaultLengthForType(indicatorB.type),
				period: defaultLengthForType(indicatorB.type),
				component: null,
				params: {}
			});
		}
	}

	function createBlankStrategy(): Strategy {
		return {
			id: crypto.randomUUID(),
			name: tr('new_strategy'),
			version: '1.0.0',
			description: tr('created_with_strategy_builder'),
			tickers: [],
			rules: [],
			initial_capital: 10000,
			max_positions: 10,
			position_size_mode: 'equal',
			position_size_value: 0.1
		};
	}

	function setPresetLink(nextPresetId: string | null, nextPresetName: string) {
		currentPresetId = nextPresetId;
		presetName = nextPresetName;
	}

	function updateStrategy(next: Strategy) {
		const normalized: Strategy = {
			...next,
			rules: Array.isArray(next.rules) ? next.rules : [],
			tickers: Array.isArray(next.tickers) ? next.tickers : [],
			initial_capital: typeof next.initial_capital === 'number' ? next.initial_capital : 10000,
			max_positions: typeof next.max_positions === 'number' ? next.max_positions : 10,
			position_size_mode: next.position_size_mode || 'equal',
			position_size_value:
				typeof next.position_size_value === 'number' ? next.position_size_value : 0.1
		};
		strategy = normalized;
		tickersInput = (normalized.tickers ?? []).join(', ');
		json = JSON.stringify(normalized, null, 2);
	}

	function syncTickersFromInput() {
		if (!strategy) return;
		strategy.tickers = tickersInput
			.split(',')
			.map((s) => s.trim().toUpperCase())
			.filter(Boolean);
		refreshJson();
	}

	function refreshJson() {
		if (strategy) {
			json = JSON.stringify(strategy, null, 2);
		}
	}

	function buildNewRule(): Rule {
		const normalizedA = normalizeIndicator(indicatorA);
		const condition: Condition = {
			type: conditionType,
			indicator_a: normalizedA,
			indicator_b: null,
			threshold: null,
			lookback_periods: lookbackPeriods,
			range_start: null,
			range_end: null
		};

		if (conditionType === 'within_range') {
			condition.range_start = rangeStart;
			condition.range_end = rangeEnd;
		} else if (needsIndicatorB && (!needsThreshold || compareMode === 'indicator')) {
			condition.indicator_b = normalizeIndicator(indicatorB);
		} else if (needsThreshold) {
			condition.threshold = thresholdValue;
		}

		return {
			id: editingRuleId || crypto.randomUUID(),
			name: ruleName.trim() || tr('strategy_inspector_new'),
			scope: ruleScope,
			action: actionType,
			enabled: true,
			priority,
			condition
		};
	}

	function autoSelectFirstRule(next: Strategy) {
		if (next.rules && next.rules.length > 0) {
			editRule(next.rules[0]);
		} else {
			resetForm();
		}
	}

	async function loadStrategy(force = false) {
		try {
			const data = (await getStrategy(force)) as Strategy;
			updateStrategy(data);
			setPresetLink(data.id ?? null, data.name ?? '');
			autoSelectFirstRule(data);
		} catch (err) {
			message = formatApiError(err);
		}
	}

	async function loadStrategyLibraryState() {
		try {
			const library = await getStrategyLibrary();
			strategyLibrary = library;
			if (currentPresetId && !library.some((entry) => entry.id === currentPresetId)) {
				currentPresetId = null;
			}
		} catch (err) {
			message = formatApiError(err);
		}
	}

	onMount(async () => {
		await Promise.all([loadStrategy(), loadStrategyLibraryState()]);
	});

	async function handleValidate() {
		message = '';
		errors = [];
		try {
			if (!strategy) return;
			const result = await validateStrategy(strategy);
			validationStatus = result.valid ? 'valid' : 'invalid';
			errors = result.errors ?? [];
			if (result.valid) {
				message = tr('strategy_validation_passed');
			}
		} catch (err) {
			message = formatApiError(err);
		}
	}

	async function handleSave() {
		message = '';
		errors = [];
		try {
			if (!strategy) return;
			syncTickersFromInput();
			const saved = (await saveStrategy(strategy)) as Strategy;
			updateStrategy(saved);
			if (currentPresetId) {
				await updateStrategyPreset(currentPresetId, saved, saved.name);
				await loadStrategyLibraryState();
			}
			message = tr('strategy_saved_success');
		} catch (err) {
			message = formatApiError(err);
		}
	}

	async function handleSavePreset() {
		message = '';
		try {
			if (!strategy) return;
			syncTickersFromInput();
			const targetName = (strategy.name || '').trim() || presetName.trim();
			if (targetName) {
				strategy = { ...strategy, name: targetName };
				refreshJson();
			}
			const savedPreset = await saveStrategyPreset(strategy, targetName);
			setPresetLink(savedPreset.id, savedPreset.name);
			await loadStrategyLibraryState();
			message = tr('strategy_preset_saved', { name: savedPreset.name });
		} catch (err) {
			message = formatApiError(err);
		}
	}

	async function handleExport() {
		message = '';
		try {
			if (!strategy) return;
			syncTickersFromInput();
			refreshJson();
			try {
				await ExportStrategyFile(strategy as any);
			} catch {
				// Browser / Playwright fallback download
				const blob = new Blob([JSON.stringify(strategy, null, 2)], {
					type: 'application/json'
				});
				const url = URL.createObjectURL(blob);
				const a = document.createElement('a');
				a.href = url;
				a.download = `${(strategy.name || 'strategy').replace(/[^a-z0-9_-]+/gi, '_')}.json`;
				a.click();
				URL.revokeObjectURL(url);
			}
			message = tr('strategy_exported_success');
		} catch (err) {
			message = formatApiError(err);
		}
	}

	async function handleImportJsonText() {
		message = '';
		try {
			const parsed = JSON.parse(json) as Strategy;
			const data = (await importStrategy(parsed)) as Strategy;
			updateStrategy(data);
			setPresetLink(null, data.name ?? '');
			autoSelectFirstRule(data);
			message = tr('strategy_imported_success', { name: data.name || '' });
		} catch (err) {
			message = formatApiError(err);
		}
	}

	async function handleFileInputChange(event: Event) {
		const target = event.target as HTMLInputElement;
		const file = target.files?.[0];
		if (!file) return;
		message = '';
		try {
			const data = (await importStrategyFile(file)) as Strategy;
			updateStrategy(data);
			setPresetLink(null, data.name ?? '');
			autoSelectFirstRule(data);
			message = tr('strategy_imported_success', { name: data.name || '' });
		} catch (err) {
			message = formatApiError(err);
		} finally {
			target.value = '';
		}
	}

	async function handleNativeImportFile() {
		message = '';
		try {
			const data = (await ImportStrategyFile()) as unknown as Strategy | null;
			if (!data) return;
			updateStrategy(data);
			setPresetLink(null, data.name ?? '');
			autoSelectFirstRule(data);
			message = tr('strategy_imported_success', { name: data.name || '' });
		} catch {
			fileInputEl?.click();
		}
	}

	function handleNewStrategy() {
		message = '';
		errors = [];
		validationStatus = 'idle';
		const next = createBlankStrategy();
		updateStrategy(next);
		setPresetLink(null, next.name);
		resetForm();
	}

	async function handleLoadPreset(strategyId: string) {
		message = '';
		errors = [];
		validationStatus = 'idle';
		try {
			const data = (await getStrategyPreset(strategyId)) as Strategy;
			updateStrategy(data);
			setPresetLink(strategyId, data.name ?? '');
			autoSelectFirstRule(data);
		} catch (err) {
			message = formatApiError(err);
		}
	}

	async function handleActivatePreset(strategyId: string) {
		message = '';
		try {
			const data = (await applyStrategyPreset(strategyId)) as Strategy;
			updateStrategy(data);
			setPresetLink(strategyId, data.name ?? '');
			autoSelectFirstRule(data);
			await loadStrategyLibraryState();
			message = tr('strategy_applied_success');
		} catch (err) {
			message = formatApiError(err);
		}
	}

	async function handleDeletePreset(strategyId: string) {
		message = '';
		try {
			await deleteStrategyPreset(strategyId);
			if (currentPresetId === strategyId) {
				currentPresetId = null;
			}
			await loadStrategyLibraryState();
		} catch (err) {
			message = formatApiError(err);
		}
	}

	async function handleTogglePin(strategyId: string) {
		message = '';
		try {
			strategyLibrary = await toggleStrategyPin(strategyId);
		} catch (err) {
			message = formatApiError(err);
		}
	}

	async function handleMoveOrder(strategyId: string, direction: 'up' | 'down') {
		message = '';
		try {
			strategyLibrary = await moveStrategyOrder(strategyId, direction);
		} catch (err) {
			message = formatApiError(err);
		}
	}

	function resetForm(defaultAction = 'buy') {
		editingRuleId = null;
		ruleName = '';
		ruleScope = 'per_ticker';
		conditionType = 'crosses_above';
		actionType = defaultAction;
		priority = 50;
		lookbackPeriods = 1;
		compareMode = 'indicator';
		thresholdValue = 0;
		rangeStart = '09:30';
		rangeEnd = '16:00';
		indicatorA = createIndicator('ema');
		indicatorB = createIndicator('ema');
	}

	function startNewRuleInStage(stage: string) {
		resetForm(stage);
	}

	function editRule(rule: Rule) {
		editingRuleId = rule.id;
		ruleName = rule.name;
		ruleScope = rule.scope || 'per_ticker';
		actionType = rule.action || 'buy';
		priority = rule.priority ?? 50;
		const cond = rule.condition;
		const rawCondType = cond?.type || 'crosses_above';
		conditionType =
			rawCondType === 'cross_above'
				? 'crosses_above'
				: rawCondType === 'cross_below'
					? 'crosses_below'
					: rawCondType;
		lookbackPeriods = cond?.lookback_periods ?? 1;

		if (cond?.threshold !== null && cond?.threshold !== undefined) {
			thresholdValue = cond.threshold;
		} else {
			thresholdValue = 0;
		}
		if (cond?.range_start) rangeStart = cond.range_start;
		if (cond?.range_end) rangeEnd = cond.range_end;

		if (cond?.indicator_a) {
			indicatorA = normalizeIndicator(JSON.parse(JSON.stringify(cond.indicator_a)));
		} else {
			indicatorA = createIndicator('ema');
		}

		if (cond?.indicator_b) {
			indicatorB = normalizeIndicator(JSON.parse(JSON.stringify(cond.indicator_b)));
			compareMode = 'indicator';
		} else {
			indicatorB = createIndicator('ema');
			if (['greater_than', 'less_than', 'slope_above', 'slope_below'].includes(conditionType)) {
				compareMode = 'threshold';
			} else {
				compareMode = 'indicator';
			}
		}

		if (['greater_than', 'less_than'].includes(conditionType)) {
			compareMode = cond?.indicator_b ? 'indicator' : 'threshold';
		}
	}

	function saveRule() {
		if (!strategy) return;
		if (!ruleName.trim()) {
			message = tr('rule_name_required');
			return;
		}
		const newRule = buildNewRule();

		if (editingRuleId) {
			const existing = strategy.rules.find((r) => r.id === editingRuleId);
			if (existing) {
				newRule.enabled = existing.enabled;
			}
			const next: Strategy = {
				...strategy,
				rules: strategy.rules.map((r) => (r.id === editingRuleId ? newRule : r))
			};
			updateStrategy(next);
			editRule(newRule);
			message = tr('rule_updated_success', { name: newRule.name });
		} else {
			const next: Strategy = { ...strategy, rules: [...strategy.rules, newRule] };
			updateStrategy(next);
			editRule(newRule);
			message = tr('rule_added_success', { name: newRule.name });
		}
	}

	function removeRule(id: string) {
		if (!strategy) return;
		const nextRules = strategy.rules.filter((rule) => rule.id !== id);
		const next: Strategy = { ...strategy, rules: nextRules };
		updateStrategy(next);
		if (editingRuleId === id) {
			autoSelectFirstRule(next);
		}
		message = tr('strategy_rule_deleted');
	}

	function toggleRule(id: string) {
		if (!strategy) return;
		const next: Strategy = {
			...strategy,
			rules: strategy.rules.map((rule) =>
				rule.id === id ? { ...rule, enabled: !rule.enabled } : rule
			)
		};
		updateStrategy(next);
	}
</script>

<svelte:window
	on:keydown={(e) => {
		if (e.key === 'Escape' && !e.defaultPrevented && showJson && !$tourStore.open && !$helpDrawerOpen) {
			showJson = false;
		}
	}}
/>

<div class="strategy-page">
	<div class="strategy-topbar">
		<h1 class="heading">
			<span class="heading-icon"><GitBranch size={18} strokeWidth={1.7} /></span>
			<span>{tr('strategy_builder')}</span>
		</h1>
		{#if message}
			<span class="strategy-status-toast">{message}</span>
		{/if}
	</div>

	<input
		bind:this={fileInputEl}
		type="file"
		accept=".json"
		class="sr-only"
		on:change={handleFileInputChange}
	/>

	{#if strategy}
		<div class="strategy-workbench">
			<!-- Left Pane: Searchable & Workspace-Filterable Preset Stack -->
			<aside class="strategy-pane strategy-pane-left" data-testid="strategy-pane-left">
				<div class="pane-header">
					<div class="pane-head-row">
						<h2>{tr('strategy_pane_presets')}</h2>
						<button class="secondary compact-btn" on:click={handleNewStrategy} title={tr('new_strategy')}>
							<Plus size={13} strokeWidth={2} />
							<span>{tr('new_strategy')}</span>
						</button>
					</div>

					<input
						type="search"
						class="preset-search-input"
						bind:value={librarySearch}
						placeholder={tr('strategy_search_placeholder')}
					/>

					<div class="workspace-filter-pills">
						<button
							type="button"
							class="filter-pill"
							class:is-active={workspaceFilter === 'all'}
							on:click={() => (workspaceFilter = 'all')}
						>
							{tr('strategy_filter_all')}
						</button>
						<button
							type="button"
							class="filter-pill"
							class:is-active={workspaceFilter === 'achat'}
							on:click={() => (workspaceFilter = 'achat')}
						>
							{tr('strategy_filter_achat')}
						</button>
						<button
							type="button"
							class="filter-pill"
							class:is-active={workspaceFilter === 'short'}
							on:click={() => (workspaceFilter = 'short')}
						>
							{tr('strategy_filter_short')}
						</button>
						<button
							type="button"
							class="filter-pill"
							class:is-active={workspaceFilter === 'swing'}
							on:click={() => (workspaceFilter = 'swing')}
						>
							{tr('strategy_filter_swing')}
						</button>
						<button
							type="button"
							class="filter-pill"
							class:is-active={workspaceFilter === 'pinned'}
							on:click={() => (workspaceFilter = 'pinned')}
							title={tr('cockpit_pinned_badge')}
						>
							{tr('strategy_filter_pinned')}
						</button>
					</div>
				</div>

				<div class="pane-body preset-stack-body">
					{#if filteredLibrary.length === 0}
						<p class="muted preset-empty">{tr('strategy_no_presets_match')}</p>
					{:else}
						{#each filteredLibrary as preset (preset.id)}
							{@const isLinked = currentPresetId === preset.id}
							<div
								class="preset-row"
								class:is-linked={isLinked}
								class:is-pinned={Boolean(preset.pinned)}
							>
								<button
									type="button"
									class="preset-select-btn"
									on:click={() => handleLoadPreset(preset.id)}
								>
									<span class="preset-name">{preset.name}</span>
									<span class="preset-meta font-mono">
										{tr('cockpit_rules_count', {
											enabled: preset.enabled_rule_count,
											total: preset.rule_count
										})}
									</span>
								</button>

								<div class="preset-actions">
									<button
										type="button"
										class="secondary icon-btn"
										class:is-pinned={Boolean(preset.pinned)}
										on:click={() => handleTogglePin(preset.id)}
										title={preset.pinned ? tr('cockpit_unpin_strategy') : tr('cockpit_pin_strategy')}
									>
										<Pin size={12} strokeWidth={1.8} />
									</button>
									<button
										type="button"
										class="secondary icon-btn"
										disabled={!canMoveInGroup(preset, 'up')}
										on:click={() => handleMoveOrder(preset.id, 'up')}
										title={tr('cockpit_move_up')}
									>
										<ArrowUp size={12} strokeWidth={1.8} />
									</button>
									<button
										type="button"
										class="secondary icon-btn"
										disabled={!canMoveInGroup(preset, 'down')}
										on:click={() => handleMoveOrder(preset.id, 'down')}
										title={tr('cockpit_move_down')}
									>
										<ArrowDown size={12} strokeWidth={1.8} />
									</button>
									<button
										type="button"
										class="secondary icon-btn is-activate"
										on:click={() => handleActivatePreset(preset.id)}
										title={tr('activate_preset')}
									>
										<Play size={12} strokeWidth={1.8} />
									</button>
									<button
										type="button"
										class="secondary icon-btn is-danger"
										on:click={() => handleDeletePreset(preset.id)}
										title={tr('delete_preset')}
									>
										<Trash2 size={12} strokeWidth={1.8} />
									</button>
								</div>
							</div>
						{/each}
					{/if}
				</div>
			</aside>

			<!-- Center Pane: Strategy Header, Risk Strip & 3-Stage Visual Pipeline -->
			<section class="strategy-pane strategy-pane-center" data-testid="strategy-pane-center">
				<div class="pane-header center-header">
					<div class="strategy-header-row">
						<input
							class="strategy-name-input"
							bind:value={strategy.name}
							placeholder={tr('strategy_name_placeholder')}
							on:input={refreshJson}
						/>
						<div class="strategy-header-actions">
							<button type="button" class="secondary compact-btn" on:click={handleValidate}>
								<CheckCircle2 size={13} strokeWidth={1.8} />
								<span>{tr('validate')}</span>
							</button>
							<button type="button" class="compact-btn" on:click={handleSave}>
								<Save size={13} strokeWidth={1.8} />
								<span>{tr('strategy_save_changes')}</span>
							</button>
							<button
								type="button"
								class="secondary compact-btn"
								on:click={handleSavePreset}
								title={tr('save_as_preset')}
							>
								<Plus size={13} strokeWidth={1.8} />
								<span>{tr('save_as_preset')}</span>
							</button>
							<button
								type="button"
								class="secondary compact-btn"
								on:click={handleNativeImportFile}
								title={tr('import_strategy')}
							>
								<Upload size={13} strokeWidth={1.8} />
								<span>{tr('import_file')}</span>
							</button>
							<button
								type="button"
								class="secondary compact-btn"
								on:click={handleExport}
								title={tr('export')}
							>
								<Download size={13} strokeWidth={1.8} />
								<span>{tr('export')}</span>
							</button>
							<button
								type="button"
								class="secondary compact-btn"
								class:is-json-open={showJson}
								on:click={() => {
									showJson = !showJson;
									if (showJson) refreshJson();
								}}
							>
								<Code2 size={13} strokeWidth={1.8} />
								<span>{tr('strategy_json_toggle')}</span>
							</button>
						</div>
					</div>

					{#if validationStatus !== 'idle' || errors.length > 0}
						<div class="validation-strip" class:is-valid={validationStatus === 'valid'} class:is-invalid={validationStatus === 'invalid'}>
							<span class="validation-badge">
								{validationStatus === 'valid'
									? tr('strategy_status_valid')
									: tr('strategy_status_invalid')}
							</span>
							{#if errors.length > 0}
								<span class="validation-errors">{errors.join(' · ')}</span>
							{/if}
						</div>
					{/if}

					<div class="risk-strip">
						<label class="risk-field">
							<span>{tr('strategy_risk_strip_capital')}</span>
							<input
								type="number"
								class="font-mono"
								bind:value={strategy.initial_capital}
								on:change={refreshJson}
							/>
						</label>
						<label class="risk-field">
							<span>{tr('strategy_risk_strip_max_pos')}</span>
							<input
								type="number"
								class="font-mono"
								min="1"
								bind:value={strategy.max_positions}
								on:change={refreshJson}
							/>
						</label>
						<label class="risk-field">
							<span>{tr('strategy_risk_strip_sizing')}</span>
							<select bind:value={strategy.position_size_mode} on:change={refreshJson}>
								<option value="equal">{tr('strategy_sizing_equal')}</option>
								<option value="shares">{tr('strategy_sizing_shares')}</option>
								<option value="dollar">{tr('strategy_sizing_dollar')}</option>
								<option value="percent">{tr('strategy_sizing_percent')}</option>
							</select>
						</label>
						<label class="risk-field">
							<span>{tr('value')}</span>
							<input
								type="number"
								step="0.01"
								class="font-mono"
								bind:value={strategy.position_size_value}
								on:change={refreshJson}
							/>
						</label>
						<label class="risk-field risk-field-tickers">
							<span>{tr('strategy_risk_strip_tickers')}</span>
							<input
								type="text"
								class="font-mono"
								bind:value={tickersInput}
								placeholder={tr('strategy_tickers_placeholder')}
								on:change={syncTickersFromInput}
							/>
						</label>
					</div>
				</div>

				<div class="pane-body pipeline-body">
					{#each pipelineStages as stage}
						{@const stageRules = strategy.rules.filter((r) => r.action === stage.action)}
						<div class="pipeline-stage" data-stage={stage.action}>
							<div class="stage-header">
								<div class="stage-title-group">
									<span class={`stage-pill stage-${stage.action}`}>{stage.title}</span>
									<span class="stage-count font-mono">{stageRules.length}</span>
								</div>
								<button
									type="button"
									class="secondary compact-btn"
									on:click={() => startNewRuleInStage(stage.action)}
								>
									{tr('strategy_add_to_stage')}
								</button>
							</div>

							{#if stageRules.length === 0}
								<p class="stage-empty">{tr('strategy_stage_empty')}</p>
							{:else}
								<div class="stage-cards">
									{#each stageRules as rule (rule.id)}
										<div
											class="rule-card"
											class:is-selected={editingRuleId === rule.id}
											class:is-disabled={!rule.enabled}
										>
											<button
												type="button"
												class="rule-card-main"
												on:click={() => editRule(rule)}
											>
												<div class="rule-card-top">
													<strong>{rule.name}</strong>
													<span class="scope-chip">
														{rule.scope === 'global'
															? tr('scope_badge_global')
															: tr('scope_badge_per_ticker')}
													</span>
												</div>
												<span class="rule-card-preview font-mono">{formatRulePreview(rule)}</span>
											</button>

											<div class="rule-card-actions">
												<label class="toggle rule-toggle">
													<input
														type="checkbox"
														checked={rule.enabled}
														on:change={() => toggleRule(rule.id)}
													/>
													<span class="toggle-slider"></span>
												</label>
												<button
													type="button"
													class="secondary icon-btn is-danger"
													on:click={() => removeRule(rule.id)}
													title={tr('delete_rule')}
												>
													<Trash2 size={12} strokeWidth={1.8} />
												</button>
											</div>
										</div>
									{/each}
								</div>
							{/if}
						</div>
					{/each}
				</div>
			</section>

			<!-- Right Pane: Compact 2-Column Rule Inspector -->
			<section class="strategy-pane strategy-pane-right" data-testid="strategy-pane-right">
				<div class="pane-header">
					<div class="pane-head-row">
						<h2>
							<Sliders size={14} strokeWidth={1.8} />
							<span>
								{editingRuleId ? tr('strategy_inspector_editing') : tr('strategy_inspector_new')}
							</span>
						</h2>
						{#if editingRuleId}
							<button
								type="button"
								class="secondary compact-btn"
								on:click={() => resetForm(actionType)}
							>
								{tr('cancel_edit')}
							</button>
						{/if}
					</div>
				</div>

				<div class="pane-body inspector-body">
					<div class="inspector-grid-2col">
						<label class="span-2">
							<span>{tr('rule_name')}</span>
							<input bind:value={ruleName} placeholder={tr('rule_name_placeholder')} />
						</label>

						<label>
							<span>{tr('action')}</span>
							<select bind:value={actionType}>
								{#each actionOptions as opt}
									<option value={opt.value}>{opt.label}</option>
								{/each}
							</select>
						</label>

						<label>
							<span>{tr('scope')}</span>
							<select bind:value={ruleScope}>
								<option value="per_ticker">{tr('scope_per_ticker')}</option>
								<option value="global">{tr('scope_global')}</option>
							</select>
						</label>

						<label>
							<span>{tr('priority')}</span>
							<input type="number" class="font-mono" bind:value={priority} min="0" max="100" />
						</label>

						<label>
							<span>{tr('condition_type')}</span>
							<select bind:value={conditionType}>
								{#each conditionOptions as opt}
									<option value={opt.value}>{opt.label}</option>
								{/each}
							</select>
						</label>
					</div>

					<div class="inspector-section">
						<span class="inspector-section-title">{tr('indicator_a')}</span>
						<div class="inspector-grid-2col">
							<label>
								<span>{tr('indicator_type')}</span>
								<select
									bind:value={indicatorA.type}
									on:change={() => handleIndicatorTypeChange('A')}
								>
									{#each indicatorTypes as opt}
										<option value={opt.value}>{opt.label}</option>
									{/each}
								</select>
							</label>

							<label>
								<span>{tr('timeframe')}</span>
								<select bind:value={indicatorA.timeframe}>
									{#each timeframeOptions as tf}
										<option value={tf}>{tf}</option>
									{/each}
								</select>
							</label>

							<label>
								<span>{tr('source')}</span>
								<select bind:value={indicatorA.source}>
									{#each sourceOptions as src}
										<option value={src}>{src}</option>
									{/each}
								</select>
							</label>

							<label>
								<span>{tr('symbol')}</span>
								<input
									type="text"
									class="font-mono"
									bind:value={indicatorA.symbol}
									placeholder={tr('strategy_symbol_default_placeholder')}
								/>
							</label>

							{#if ['ema', 'sma', 'rsi', 'bollinger', 'rvol', 'atr', 'relative_strength', 'relative_performance'].includes(indicatorA.type)}
								<label>
									<span>{tr('length')}</span>
									<input
										type="number"
										class="font-mono"
										min="1"
										max="500"
										bind:value={indicatorA.length}
									/>
								</label>
							{/if}

							{#if indicatorA.type === 'high_52w'}
								<div class="span-2 inspector-note">{tr('strategy_high_52w_note')}</div>
							{/if}

							{#if indicatorA.type === 'macd'}
								<label>
									<span>{tr('fast_length')}</span>
									<input type="number" class="font-mono" bind:value={indicatorA.params.fast} />
								</label>
								<label>
									<span>{tr('slow_length')}</span>
									<input type="number" class="font-mono" bind:value={indicatorA.params.slow} />
								</label>
								<label>
									<span>{tr('signal_length')}</span>
									<input type="number" class="font-mono" bind:value={indicatorA.params.signal} />
								</label>
								<label>
									<span>{tr('component_label')}</span>
									<select bind:value={indicatorA.component}>
										<option value="macd">{tr('comp_macd_line')}</option>
										<option value="signal">{tr('comp_macd_signal')}</option>
										<option value="histogram">{tr('comp_macd_histogram')}</option>
									</select>
								</label>
							{/if}

							{#if indicatorA.type === 'bollinger'}
								<label>
									<span>{tr('std_dev')}</span>
									<input
										type="number"
										class="font-mono"
										step="0.1"
										bind:value={indicatorA.params.std_dev}
									/>
								</label>
								<label>
									<span>{tr('band')}</span>
									<select bind:value={indicatorA.component}>
										<option value="basis">{tr('comp_bb_basis')}</option>
										<option value="upper">{tr('comp_bb_upper')}</option>
										<option value="lower">{tr('comp_bb_lower')}</option>
										<option value="bandwidth">{tr('comp_bb_bandwidth')}</option>
										<option value="percent_b">{tr('comp_bb_percent_b')}</option>
									</select>
								</label>
							{/if}

							{#if indicatorA.type === 'stochastic'}
								<label>
									<span>{tr('k_length')}</span>
									<input
										type="number"
										class="font-mono"
										bind:value={indicatorA.params.k_length}
									/>
								</label>
								<label>
									<span>{tr('k_smoothing')}</span>
									<input
										type="number"
										class="font-mono"
										bind:value={indicatorA.params.k_smooth}
									/>
								</label>
								<label>
									<span>{tr('d_smoothing')}</span>
									<input
										type="number"
										class="font-mono"
										bind:value={indicatorA.params.d_smooth}
									/>
								</label>
								<label>
									<span>{tr('line')}</span>
									<select bind:value={indicatorA.component}>
										<option value="k">K</option>
										<option value="d">D</option>
									</select>
								</label>
							{/if}

							{#if indicatorA.type === 'heikin_ashi'}
								<label>
									<span>{tr('component_label')}</span>
									<select bind:value={indicatorA.component}>
										<option value="close">{tr('comp_ha_close')}</option>
										<option value="open">{tr('comp_ha_open')}</option>
										<option value="high">{tr('comp_ha_high')}</option>
										<option value="low">{tr('comp_ha_low')}</option>
									</select>
								</label>
							{/if}

							{#if indicatorA.type === 'guppy'}
								<label>
									<span>{tr('component_label')}</span>
									<select bind:value={indicatorA.component}>
										<option value="fast_max">{tr('comp_guppy_fast_max')}</option>
										<option value="fast_min">{tr('comp_guppy_fast_min')}</option>
										<option value="slow_min">{tr('comp_guppy_slow_min')}</option>
										<option value="slow_max">{tr('comp_guppy_slow_max')}</option>
										<option value="anchor">{tr('comp_guppy_anchor')}</option>
									</select>
								</label>
							{/if}

							{#if indicatorA.type === 'atr'}
								<label>
									<span>{tr('component_label')}</span>
									<select bind:value={indicatorA.component}>
										<option value="atr">{tr('comp_atr_value')}</option>
										<option value="atr_pct">{tr('comp_atr_pct')}</option>
									</select>
								</label>
							{/if}

							{#if indicatorA.type === 'alligator'}
								<label>
									<span>{tr('line')}</span>
									<select bind:value={indicatorA.component}>
										<option value="jaw">{tr('comp_alligator_jaw')}</option>
										<option value="teeth">{tr('comp_alligator_teeth')}</option>
										<option value="lips">{tr('comp_alligator_lips')}</option>
									</select>
								</label>
							{/if}

							{#if indicatorA.type === 'ml_signal'}
								<label>
									<span>{tr('model_path')}</span>
									<input type="text" bind:value={indicatorA.params.model_path} />
								</label>
								<label>
									<span>{tr('signal_column')}</span>
									<input type="text" bind:value={indicatorA.params.column} />
								</label>
							{/if}
						</div>
					</div>

					<div class="inspector-section">
						<span class="inspector-section-title">
							<Filter size={12} strokeWidth={1.8} />
							<span>{tr('strategy_params_heading')}</span>
						</span>
						<div class="inspector-grid-2col">
							<label>
								<span>{tr('lookback_periods')}</span>
								<input
									type="number"
									class="font-mono"
									min="1"
									max="252"
									bind:value={lookbackPeriods}
								/>
							</label>

							{#if needsThreshold && needsIndicatorB}
								<label>
									<span>{tr('compare_to')}</span>
									<select bind:value={compareMode}>
										<option value="indicator">{tr('compare_to_indicator')}</option>
										<option value="threshold">{tr('compare_to_threshold')}</option>
									</select>
								</label>
							{/if}

							{#if needsThreshold && (!needsIndicatorB || compareMode === 'threshold')}
								<label>
									<span>{tr('threshold_value')}</span>
									<input
										type="number"
										step="0.01"
										class="font-mono"
										bind:value={thresholdValue}
									/>
								</label>
							{/if}

							{#if needsRange}
								<label>
									<span>{tr('start_time')}</span>
									<input type="text" class="font-mono" bind:value={rangeStart} />
								</label>
								<label>
									<span>{tr('end_time')}</span>
									<input type="text" class="font-mono" bind:value={rangeEnd} />
								</label>
							{/if}
						</div>
					</div>

					{#if needsIndicatorB && (!needsThreshold || compareMode === 'indicator')}
						<div class="inspector-section">
							<span class="inspector-section-title">{tr('indicator_b')}</span>
							<div class="inspector-grid-2col">
								<label>
									<span>{tr('indicator_type')}</span>
									<select
										bind:value={indicatorB.type}
										on:change={() => handleIndicatorTypeChange('B')}
									>
										{#each indicatorTypes as opt}
											<option value={opt.value}>{opt.label}</option>
										{/each}
									</select>
								</label>

								<label>
									<span>{tr('timeframe')}</span>
									<select bind:value={indicatorB.timeframe}>
										{#each timeframeOptions as tf}
											<option value={tf}>{tf}</option>
										{/each}
									</select>
								</label>

								<label>
									<span>{tr('source')}</span>
									<select bind:value={indicatorB.source}>
										{#each sourceOptions as src}
											<option value={src}>{src}</option>
										{/each}
									</select>
								</label>

								<label>
									<span>{tr('symbol')}</span>
									<input
										type="text"
										class="font-mono"
										bind:value={indicatorB.symbol}
										placeholder={tr('strategy_symbol_default_placeholder')}
									/>
								</label>

								{#if ['ema', 'sma', 'rsi', 'bollinger', 'rvol', 'atr', 'relative_strength', 'relative_performance'].includes(indicatorB.type)}
									<label>
										<span>{tr('length')}</span>
										<input
											type="number"
											class="font-mono"
											min="1"
											max="500"
											bind:value={indicatorB.length}
										/>
									</label>
								{/if}

								{#if indicatorB.type === 'macd'}
									<label>
										<span>{tr('fast_length')}</span>
										<input type="number" class="font-mono" bind:value={indicatorB.params.fast} />
									</label>
									<label>
										<span>{tr('slow_length')}</span>
										<input type="number" class="font-mono" bind:value={indicatorB.params.slow} />
									</label>
									<label>
										<span>{tr('signal_length')}</span>
										<input type="number" class="font-mono" bind:value={indicatorB.params.signal} />
									</label>
									<label>
										<span>{tr('component_label')}</span>
										<select bind:value={indicatorB.component}>
											<option value="macd">{tr('comp_macd_line')}</option>
											<option value="signal">{tr('comp_macd_signal')}</option>
											<option value="histogram">{tr('comp_macd_histogram')}</option>
										</select>
									</label>
								{/if}

								{#if indicatorB.type === 'bollinger'}
									<label>
										<span>{tr('std_dev')}</span>
										<input
											type="number"
											class="font-mono"
											step="0.1"
											bind:value={indicatorB.params.std_dev}
										/>
									</label>
									<label>
										<span>{tr('band')}</span>
										<select bind:value={indicatorB.component}>
											<option value="basis">{tr('comp_bb_basis')}</option>
											<option value="upper">{tr('comp_bb_upper')}</option>
											<option value="lower">{tr('comp_bb_lower')}</option>
											<option value="bandwidth">{tr('comp_bb_bandwidth')}</option>
											<option value="percent_b">{tr('comp_bb_percent_b')}</option>
										</select>
									</label>
								{/if}

								{#if indicatorB.type === 'stochastic'}
									<label>
										<span>{tr('k_length')}</span>
										<input
											type="number"
											class="font-mono"
											bind:value={indicatorB.params.k_length}
										/>
									</label>
									<label>
										<span>{tr('k_smoothing')}</span>
										<input
											type="number"
											class="font-mono"
											bind:value={indicatorB.params.k_smooth}
										/>
									</label>
									<label>
										<span>{tr('d_smoothing')}</span>
										<input
											type="number"
											class="font-mono"
											bind:value={indicatorB.params.d_smooth}
										/>
									</label>
									<label>
										<span>{tr('line')}</span>
										<select bind:value={indicatorB.component}>
											<option value="k">K</option>
											<option value="d">D</option>
										</select>
									</label>
								{/if}

								{#if indicatorB.type === 'heikin_ashi'}
									<label>
										<span>{tr('component_label')}</span>
										<select bind:value={indicatorB.component}>
											<option value="close">{tr('comp_ha_close')}</option>
											<option value="open">{tr('comp_ha_open')}</option>
											<option value="high">{tr('comp_ha_high')}</option>
											<option value="low">{tr('comp_ha_low')}</option>
										</select>
									</label>
								{/if}

								{#if indicatorB.type === 'guppy'}
									<label>
										<span>{tr('component_label')}</span>
										<select bind:value={indicatorB.component}>
											<option value="fast_max">{tr('comp_guppy_fast_max')}</option>
											<option value="fast_min">{tr('comp_guppy_fast_min')}</option>
											<option value="slow_min">{tr('comp_guppy_slow_min')}</option>
											<option value="slow_max">{tr('comp_guppy_slow_max')}</option>
											<option value="anchor">{tr('comp_guppy_anchor')}</option>
										</select>
									</label>
								{/if}

								{#if indicatorB.type === 'atr'}
									<label>
										<span>{tr('component_label')}</span>
										<select bind:value={indicatorB.component}>
											<option value="atr">{tr('comp_atr_value')}</option>
											<option value="atr_pct">{tr('comp_atr_pct')}</option>
										</select>
									</label>
								{/if}

								{#if indicatorB.type === 'alligator'}
									<label>
										<span>{tr('line')}</span>
										<select bind:value={indicatorB.component}>
											<option value="jaw">{tr('comp_alligator_jaw')}</option>
											<option value="teeth">{tr('comp_alligator_teeth')}</option>
											<option value="lips">{tr('comp_alligator_lips')}</option>
										</select>
									</label>
								{/if}
							</div>
						</div>
					{/if}

					<div class="inspector-footer">
						<button type="button" on:click={saveRule}>
							{editingRuleId ? tr('update_rule') : tr('add_rule')}
						</button>
					</div>
				</div>
			</section>
		</div>

		{#if showJson}
			<aside class="json-drawer" aria-label={tr('strategy_json_drawer_title')}>
				<div class="json-drawer-head">
					<h2>{tr('strategy_json_drawer_title')}</h2>
					<button type="button" class="secondary compact-btn" on:click={() => (showJson = false)}>
						<X size={14} strokeWidth={1.8} />
						<span>{tr('strategy_close_drawer')}</span>
					</button>
				</div>

				<div class="json-drawer-actions">
					<label class="secondary file-import-label">
						<Upload size={13} strokeWidth={1.8} />
						<span>{tr('import_from_file')}</span>
						<input
							type="file"
							accept=".json"
							class="sr-only"
							on:change={handleFileInputChange}
						/>
					</label>
					<button type="button" class="secondary compact-btn" on:click={handleExport}>
						<Download size={13} strokeWidth={1.8} />
						<span>{tr('export')}</span>
					</button>
					<button type="button" class="compact-btn" on:click={handleImportJsonText}>
						{tr('strategy_import_json_text')}
					</button>
				</div>

				{#if message}
					<div class="json-drawer-alert" role="status">{message}</div>
				{/if}

				<textarea class="json-drawer-textarea font-mono" bind:value={json}></textarea>
			</aside>
		{/if}
	{:else}
		<div class="card">
			<p class="muted">{tr('loading_strategy')}</p>
		</div>
	{/if}
</div>

<style>
	.strategy-page {
		height: calc(100dvh - 46px);
		max-height: calc(100dvh - 46px);
		min-height: 0;
		overflow: hidden;
		display: grid;
		grid-template-rows: auto minmax(0, 1fr);
		gap: 8px;
	}

	.strategy-topbar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		min-height: 28px;
	}

	.strategy-topbar h1 {
		margin: 0;
		font-size: 18px;
		line-height: 1.2;
	}

	.strategy-status-toast {
		font-size: 12px;
		color: var(--cobalt-cyan);
		padding: 3px 10px;
		border-radius: 999px;
		border: 1px solid rgba(56, 189, 248, 0.35);
		background: rgba(29, 99, 237, 0.14);
	}

	.strategy-workbench {
		display: grid;
		grid-template-columns: minmax(230px, 260px) minmax(0, 1.05fr) minmax(320px, 0.95fr);
		gap: 10px;
		height: 100%;
		min-height: 0;
		overflow: hidden;
	}

	.strategy-pane {
		display: flex;
		flex-direction: column;
		min-height: 0;
		min-width: 0;
		overflow: hidden;
		background: var(--bg-card);
		border: 1px solid var(--border-subtle);
		border-radius: 12px;
	}

	.pane-header {
		flex-shrink: 0;
		padding: 10px 12px;
		border-bottom: 1px solid var(--border-subtle);
		display: grid;
		gap: 8px;
		background: var(--bg-surface);
	}

	.pane-head-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
	}

	.pane-head-row h2 {
		margin: 0;
		font-size: 13px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: var(--text-secondary);
		display: inline-flex;
		align-items: center;
		gap: 6px;
	}

	.pane-body {
		flex: 1 1 auto;
		min-height: 0;
		overflow-y: auto;
		overscroll-behavior: contain;
		padding: 10px 12px;
	}

	.compact-btn {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		padding: 5px 9px;
		font-size: 12px;
		line-height: 1.2;
		white-space: nowrap;
	}

	.compact-btn.is-json-open {
		border-color: var(--cobalt-cyan);
		color: var(--cobalt-cyan);
	}

	.preset-search-input {
		width: 100%;
		padding: 6px 9px;
		font-size: 13px;
	}

	.workspace-filter-pills {
		display: flex;
		flex-wrap: wrap;
		gap: 4px;
	}

	.filter-pill {
		padding: 4px 8px;
		font-size: 11px;
		font-weight: 700;
		border-radius: 6px;
		background: var(--bg-void);
		border: 1px solid var(--border-subtle);
		color: var(--text-secondary);
	}

	.filter-pill.is-active {
		background: var(--cobalt-electric);
		border-color: var(--cobalt-cyan);
		color: #ffffff;
	}

	.preset-stack-body {
		display: grid;
		gap: 6px;
		align-content: start;
	}

	.preset-empty {
		margin: 8px 0;
		font-size: 12px;
	}

	.preset-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 6px;
		padding: 7px 8px;
		border: 1px solid var(--border-subtle);
		border-radius: 8px;
		background: var(--bg-surface);
	}

	.preset-row.is-linked {
		border-color: var(--cobalt-cyan);
		background: rgba(29, 99, 237, 0.14);
	}

	.preset-row.is-pinned:not(.is-linked) {
		border-color: rgba(250, 160, 49, 0.45);
	}

	.preset-select-btn {
		flex: 1 1 auto;
		min-width: 0;
		padding: 0;
		background: transparent;
		border: none;
		color: inherit;
		text-align: left;
		display: grid;
		gap: 2px;
	}

	.preset-name {
		font-size: 12px;
		font-weight: 700;
		color: var(--text-primary);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.preset-meta {
		font-size: 11px;
		color: var(--text-secondary);
	}

	.preset-actions {
		display: inline-flex;
		align-items: center;
		gap: 3px;
		flex-shrink: 0;
	}

	.icon-btn {
		padding: 4px;
		min-width: 24px;
		height: 24px;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		border-radius: 5px;
	}

	.icon-btn.is-pinned {
		border-color: var(--cobalt-orange);
		color: var(--cobalt-orange-light);
		background: rgba(250, 160, 49, 0.16);
	}

	.icon-btn.is-activate {
		color: var(--sem-bull-text);
	}

	.icon-btn.is-danger {
		color: var(--sem-bear-text);
	}

	.strategy-header-row {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
	}

	.strategy-name-input {
		flex: 1 1 180px;
		min-width: 140px;
		padding: 6px 9px;
		font-size: 13px;
		font-weight: 700;
	}

	.strategy-header-actions {
		display: flex;
		align-items: center;
		gap: 5px;
		flex-wrap: wrap;
	}

	.validation-strip {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
		font-size: 12px;
	}

	.validation-badge {
		padding: 2px 8px;
		border-radius: 999px;
		font-weight: 700;
		font-size: 11px;
	}

	.validation-strip.is-valid .validation-badge {
		background: rgba(16, 185, 129, 0.16);
		color: var(--sem-bull-text);
		border: 1px solid rgba(16, 185, 129, 0.45);
	}

	.validation-strip.is-invalid .validation-badge {
		background: rgba(239, 68, 68, 0.16);
		color: var(--sem-bear-text);
		border: 1px solid rgba(239, 68, 68, 0.45);
	}

	.validation-errors {
		color: var(--sem-bear-text);
	}

	.risk-strip {
		display: grid;
		grid-template-columns: repeat(4, minmax(0, 1fr)) minmax(110px, 1.4fr);
		gap: 6px;
		align-items: end;
	}

	.risk-field {
		display: grid;
		gap: 2px;
		font-size: 11px;
		color: var(--text-secondary);
		min-width: 0;
	}

	.risk-field input,
	.risk-field select {
		width: 100%;
		min-width: 0;
		font-size: 12px;
		padding-top: 5px;
		padding-bottom: 5px;
	}

	.pipeline-body {
		display: grid;
		gap: 10px;
		align-content: start;
	}

	.pipeline-stage {
		display: grid;
		gap: 6px;
		padding: 8px 10px;
		border: 1px solid var(--border-subtle);
		border-radius: 10px;
		background: var(--bg-surface);
	}

	.stage-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
	}

	.stage-title-group {
		display: inline-flex;
		align-items: center;
		gap: 6px;
	}

	.stage-pill {
		font-size: 11px;
		font-weight: 700;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		padding: 3px 8px;
		border-radius: 999px;
	}

	.stage-filter {
		background: rgba(56, 189, 248, 0.14);
		color: var(--cobalt-cyan);
		border: 1px solid rgba(56, 189, 248, 0.35);
	}

	.stage-buy {
		background: rgba(16, 185, 129, 0.14);
		color: var(--sem-bull-text);
		border: 1px solid rgba(16, 185, 129, 0.35);
	}

	.stage-sell {
		background: rgba(250, 160, 49, 0.14);
		color: var(--sem-warn-text);
		border: 1px solid rgba(250, 160, 49, 0.35);
	}

	.stage-count {
		font-size: 11px;
		color: var(--text-secondary);
	}

	.stage-empty {
		margin: 0;
		font-size: 12px;
		color: var(--text-muted);
	}

	.stage-cards {
		display: grid;
		gap: 6px;
	}

	.rule-card {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		padding: 8px 10px;
		border: 1px solid var(--border-subtle);
		border-radius: 8px;
		background: var(--bg-card);
	}

	.rule-card.is-selected {
		border-color: var(--cobalt-cyan);
		background: rgba(29, 99, 237, 0.14);
	}

	.rule-card.is-disabled {
		opacity: 0.55;
	}

	.rule-card-main {
		flex: 1 1 auto;
		min-width: 0;
		background: transparent;
		border: none;
		padding: 0;
		color: inherit;
		text-align: left;
		display: grid;
		gap: 3px;
	}

	.rule-card-top {
		display: flex;
		align-items: center;
		gap: 6px;
		flex-wrap: wrap;
	}

	.rule-card-top strong {
		font-size: 13px;
		color: var(--text-primary);
	}

	.scope-chip {
		font-size: 10px;
		font-weight: 700;
		padding: 1px 6px;
		border-radius: 999px;
		border: 1px solid var(--border-highlight);
		color: var(--text-secondary);
	}

	.rule-card-preview {
		font-size: 11px;
		color: var(--text-secondary);
		word-break: break-word;
	}

	.rule-card-actions {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		flex-shrink: 0;
	}

	.rule-toggle {
		gap: 0;
	}

	.inspector-body {
		display: grid;
		gap: 10px;
		align-content: start;
	}

	.inspector-grid-2col {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 8px;
	}

	.inspector-grid-2col label {
		display: grid;
		gap: 3px;
		font-size: 11px;
		color: var(--text-secondary);
		min-width: 0;
	}

	.inspector-grid-2col label.span-2,
	.span-2 {
		grid-column: 1 / -1;
	}

	.inspector-grid-2col input,
	.inspector-grid-2col select {
		width: 100%;
		min-width: 0;
		font-size: 12px;
		padding-top: 6px;
		padding-bottom: 6px;
	}

	.inspector-section {
		display: grid;
		gap: 6px;
		padding: 8px 10px;
		border: 1px solid var(--border-subtle);
		border-radius: 10px;
		background: var(--bg-surface);
	}

	.inspector-section-title {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: 11px;
		font-weight: 700;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		color: var(--cobalt-cyan);
	}

	.inspector-note {
		font-size: 11px;
		color: var(--text-secondary);
		padding: 4px 8px;
		border-radius: 6px;
		background: var(--bg-card);
		border: 1px solid var(--border-subtle);
	}

	.inspector-footer {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
		padding-top: 4px;
	}

	/* Slide-over JSON Drawer */
	.json-drawer {
		position: fixed;
		inset: 0 0 0 auto;
		width: min(540px, 46vw);
		z-index: 60;
		background: var(--bg-surface);
		border-left: 1px solid var(--border-highlight);
		padding: 16px;
		display: flex;
		flex-direction: column;
		gap: 10px;
		box-shadow: -16px 0 40px rgba(0, 0, 0, 0.55);
	}

	.json-drawer-alert {
		font-size: 12px;
		color: var(--cobalt-cyan);
		padding: 6px 10px;
		border-radius: 8px;
		border: 1px solid rgba(56, 189, 248, 0.4);
		background: rgba(29, 99, 237, 0.16);
		flex-shrink: 0;
	}

	.json-drawer-head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
	}

	.json-drawer-head h2 {
		margin: 0;
		font-size: 16px;
	}

	.json-drawer-actions {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
	}

	.file-import-label {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		padding: 5px 9px;
		border-radius: 8px;
		font-size: 12px;
		font-weight: 600;
		background: #1f2937;
		color: var(--text-primary);
		border: 1px solid var(--border-highlight);
		cursor: pointer;
	}

	.json-drawer-textarea {
		width: 100%;
		flex: 1 1 0%;
		min-height: 0;
		resize: none;
		font-size: 12px;
		line-height: 1.45;
	}
</style>
