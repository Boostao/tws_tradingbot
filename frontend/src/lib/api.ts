import {
	ApplyStrategyPreset,
	ConnectTws,
	DeleteStrategyPreset,
	DisconnectTws,
	DryRunBot,
	ExportStrategyFile,
	ExportWatchlistFile,
	GetCockpitState,
	GetConfig,
	GetDiagnostics,
	GetRecentTrades,
	GetRuntimeState,
	GetStrategy,
	GetStrategyLibrary,
	GetStrategyPreset,
	GetSymbols,
	GetWatchlist,
	GetWatchlistById,
	GetWatchlists,
	ImportStrategyFile,
	ImportTradingViewWatchlist,
	ImportWatchlistFile,
	RefreshWatchlistFeed,
	ReplaceWatchlist,
	ResetCircuitBreaker,
	SaveStrategyPreset,
	SaveWatchlist,
	StartBot,
	StopBot,
	UpdateCockpitState,
	UpdateConfig,
	UpdateStrategy,
	ValidateStrategy
} from '../../bindings/tws_traderbot/app';

export const API_BASE = '';

type RestMetric = {
	lastMs: number;
	lastAt: number;
	count: number;
	errorCount: number;
};

export class ApiError extends Error {
	status: number;
	statusText: string;
	details?: string;
	url?: string;

	constructor(message: string, status: number, statusText: string, details?: string, url?: string) {
		super(message);
		this.name = 'ApiError';
		this.status = status;
		this.statusText = statusText;
		this.details = details;
		this.url = url;
	}
}

type CacheEntry<T> = { value: T; expiresAt: number };
const cache = new Map<string, CacheEntry<unknown>>();
const restMetrics = new Map<string, RestMetric>();
let lastApiError: ApiError | null = null;

type CachePolicy = {
	strategy: number;
	cockpit: number;
	config: number;
	diagnostics: number;
	state: number;
	watchlist: number;
	symbols: number;
};

const cachePolicy: CachePolicy = {
	strategy: 5000,
	cockpit: 5000,
	config: 10000,
	diagnostics: 5000,
	state: 2000,
	watchlist: 10000,
	symbols: 60000
};

export function getCachePolicy(): CachePolicy {
	return { ...cachePolicy };
}

export function setCacheTtl(key: keyof CachePolicy, ttlMs: number): void {
	cachePolicy[key] = Math.max(0, ttlMs);
}

export function clearCache(key?: string): void {
	if (key) {
		cache.delete(key);
		return;
	}
	cache.clear();
}

export function getLastApiError(): ApiError | null {
	return lastApiError;
}

export function formatApiError(err: unknown): string {
	if (err instanceof ApiError) {
		return err.message;
	}
	if (err instanceof Error) {
		return err.message;
	}
	return 'Unknown API error';
}

function recordRestMetric(key: string, durationMs: number, ok: boolean): void {
	const existing = restMetrics.get(key) ?? { lastMs: 0, lastAt: 0, count: 0, errorCount: 0 };
	existing.lastMs = durationMs;
	existing.lastAt = Date.now();
	existing.count += 1;
	if (!ok) existing.errorCount += 1;
	restMetrics.set(key, existing);
}

export function getRestMetric(key: string): RestMetric | null {
	return restMetrics.get(key) ?? null;
}

function getCached<T>(key: string): T | null {
	const entry = cache.get(key) as CacheEntry<T> | undefined;
	if (!entry) return null;
	if (Date.now() > entry.expiresAt) {
		cache.delete(key);
		return null;
	}
	return entry.value;
}

function setCached<T>(key: string, value: T, ttlMs: number): void {
	cache.set(key, { value, expiresAt: Date.now() + ttlMs });
}

async function timedCall<T>(key: string, fn: () => Promise<T>): Promise<T> {
	const start = typeof performance !== 'undefined' ? performance.now() : Date.now();
	let ok = false;
	try {
		const res = await fn();
		ok = true;
		return res;
	} catch (err: unknown) {
		const message = formatApiError(err);
		lastApiError = new ApiError(message, 500, 'IPC Error', message);
		throw lastApiError;
	} finally {
		const end = typeof performance !== 'undefined' ? performance.now() : Date.now();
		recordRestMetric(key, Math.max(0, end - start), ok);
	}
}

export async function getHealth(): Promise<{ status: string }> {
	return { status: 'healthy' };
}

export type SymbolRecord = {
	symbol: string;
	name?: string;
	exchange?: string;
	type?: string;
};

export type WatchlistItem = {
	id?: number;
	watchlist_id?: string;
	group_id?: string;
	group_name?: string;
	symbol: string;
	exchange?: string;
	venue?: string;
	instrument_id?: string;
	name?: string;
	enabled: boolean;
	tier?: string;
	is_orderable?: boolean;
	is_regime_reference?: boolean;
	is_holding?: boolean;
	size_mode?: string;
	size_value?: number;
	created_at?: string;
	updated_at?: string;
};

export type WatchlistGroup = {
	id: string;
	name: string;
	source?: string;
	items: WatchlistItem[];
};

export type WatchlistSummary = {
	id: string;
	name: string;
	description?: string;
	is_default?: boolean;
	source?: string;
	feed_url?: string;
	item_count?: number;
	updated_at?: string;
};

export type WatchlistFeed = {
	provider: string;
	url: string;
	title?: string | null;
	external_id?: string | null;
	last_refreshed_at?: string | null;
};

export type WatchlistResponse = {
	symbols: string[];
	groups: WatchlistGroup[];
	feed?: WatchlistFeed | null;
	updated_at?: string | null;
};

export type CockpitStrategySummary = {
	id: string;
	name: string;
	rule_count: number;
	enabled_rule_count: number;
	source: string;
};

export type CockpitStrategySlot = {
	id: string;
	label: string;
	strategy_id?: string | null;
	enabled: boolean;
};

export type CockpitWorkspace = {
	id: string;
	name: string;
	kind: string;
	enabled: boolean;
	strategy_slots: CockpitStrategySlot[];
};

export type CockpitState = {
	global_enabled: boolean;
	active_workspace_id?: string | null;
	workspaces: CockpitWorkspace[];
	strategy_library: CockpitStrategySummary[];
	feed?: WatchlistFeed | null;
	updated_at?: string | null;
};

export type StrategyLibraryEntry = {
	id: string;
	name: string;
	rule_count: number;
	enabled_rule_count: number;
	updated_at?: string | null;
};

export type BotState = {
	status: string;
	tws_connected: boolean;
	equity: number;
	daily_pnl: number;
	daily_pnl_percent: number;
	total_pnl: number;
	daily_loss_limit?: number;
	circuit_breaker_tripped?: boolean;
	active_strategy: string;
	open_positions_count: number;
	pending_orders_count: number;
	trades_today: number;
	win_rate_today: number;
	last_update?: string | null;
	last_heartbeat?: string | null;
	recent_logs?: string[];
	recent_orders?: Array<Record<string, unknown>>;
	recent_trades?: Array<Record<string, unknown>>;
	last_dry_run?: Record<string, unknown> | null;
	last_runtime_reload_at?: string | null;
	last_runtime_reload_reason?: string | null;
	last_disconnect_at?: string | null;
	last_disconnect_reason?: string | null;
	error_message?: string;
	runner_active?: boolean;
};

export type DiagnosticsStartup = {
	environment: string;
	trading_mode: string;
	ib_host: string;
	ib_port: number;
	client_id: number;
	account: string;
	log_level: string;
	log_file: string;
	watchlist_path: string;
	strategy_path: string;
	symbol_cache_path: string;
};

export type DiagnosticsRuntime = {
	runner_active: boolean;
	last_runtime_reload_at?: string | null;
	last_runtime_reload_reason?: string | null;
	last_disconnect_at?: string | null;
	last_disconnect_reason?: string | null;
};

export type DiagnosticsSymbols = {
	source?: string | null;
	last_checked_at?: string | null;
	last_warning?: string | null;
};

export type DiagnosticsResponse = {
	startup: DiagnosticsStartup;
	runtime: DiagnosticsRuntime;
	symbols: DiagnosticsSymbols;
};

export type BotDryRun = {
	strategy: string;
	workspace_kind: string;
	subscriptions: Array<{ symbol: string; timeframe: string }>;
	positions: Record<string, unknown>;
	open_orders: Array<Record<string, unknown>>;
	planned_orders: Array<Record<string, unknown>>;
	state: BotState;
	generated_at: string;
};

export type ConfigResponse = {
	ib?: {
		host?: string;
		port?: number;
		client_id?: number;
		account?: string;
		timeout?: number;
		trading_mode?: string;
	};
	runtime?: {
		fixed_notional?: number;
		bracket_enabled?: boolean;
		stop_loss_pct?: number;
		take_profit_pct?: number;
	};
};

export type Strategy = Record<string, unknown>;

export async function getStrategy(force = false): Promise<Strategy> {
	const cached = !force ? getCached<Strategy>('strategy') : null;
	if (cached) return cached;
	try {
		const data = (await timedCall('strategy.get', () => GetStrategy())) as unknown as Strategy;
		if (data) {
			setCached('strategy', data, cachePolicy.strategy);
			return data;
		}
	} catch (e) {
		// return fallback empty strategy
	}
	return {} as Strategy;
}

export async function getConfig(force = false): Promise<ConfigResponse> {
	const cached = !force ? getCached<ConfigResponse>('config') : null;
	if (cached) return cached;
	try {
		const data = (await timedCall('config.get', () => GetConfig())) as unknown as ConfigResponse;
		if (data) {
			setCached('config', data, cachePolicy.config);
			return data;
		}
	} catch (e) {
		// return fallback empty config
	}
	return {} as ConfigResponse;
}

export async function updateConfig(updates: Record<string, Record<string, unknown>>): Promise<ConfigResponse> {
	try {
		const data = (await timedCall('config.update', () => UpdateConfig(updates as any))) as unknown as ConfigResponse;
		setCached('config', data, cachePolicy.config);
		return data;
	} catch (e) {
		const err = new ApiError(`Config update failed: ${e}`, 500, 'Server Error', String(e));
		lastApiError = err;
		throw err;
	}
}

export async function getState(force = false): Promise<BotState> {
	const cached = !force ? getCached<BotState>('state') : null;
	if (cached) return cached;
	try {
		const data = await timedCall('state.get', () => GetRuntimeState());
		if (data) {
			const res = data as unknown as BotState;
			setCached('state', res, cachePolicy.state);
			return res;
		}
	} catch (e) {
		// fallback
	}
	return {
		status: 'STOPPED',
		tws_connected: false,
		equity: 0,
		daily_pnl: 0,
		daily_pnl_percent: 0,
		total_pnl: 0,
		active_strategy: '',
		open_positions_count: 0,
		pending_orders_count: 0,
		trades_today: 0,
		win_rate_today: 0,
		runner_active: false
	};
}

export async function getDiagnostics(force = false): Promise<DiagnosticsResponse> {
	const cached = !force ? getCached<DiagnosticsResponse>('diagnostics') : null;
	if (cached) return cached;
	try {
		const data = await timedCall('diagnostics.get', () => GetDiagnostics());
		if (!data) {
			throw new Error('GetDiagnostics returned empty response');
		}
		const res = data as unknown as DiagnosticsResponse;
		setCached('diagnostics', res, cachePolicy.diagnostics);
		return res;
	} catch (e) {
		const err = new ApiError(`Diagnostics fetch failed: ${e}`, 500, 'Server Error', String(e));
		lastApiError = err;
		throw err;
	}
}

export async function connectTws(payload: { host?: string; port?: number; client_id?: number }): Promise<{ status: string }> {
	try {
		const res = await timedCall('tws.connect', () =>
			ConnectTws(payload.host || '127.0.0.1', payload.port || 7497, payload.client_id || 1)
		);
		return { status: ((res as Record<string, any>)?.status as string) || 'ok' };
	} catch (e) {
		return { status: 'error' };
	}
}

export async function disconnectTws(): Promise<{ status: string }> {
	try {
		const res = await timedCall('tws.disconnect', () => DisconnectTws());
		return { status: ((res as Record<string, any>)?.status as string) || 'ok' };
	} catch (e) {
		return { status: 'error' };
	}
}

export async function startBot(): Promise<{ status: string }> {
	try {
		const res = await timedCall('bot.start', () => StartBot());
		return { status: ((res as Record<string, any>)?.status as string) || 'ok' };
	} catch (e) {
		return { status: 'error' };
	}
}

export async function stopBot(): Promise<{ status: string }> {
	try {
		const res = await timedCall('bot.stop', () => StopBot());
		return { status: ((res as Record<string, any>)?.status as string) || 'ok' };
	} catch (e) {
		return { status: 'error' };
	}
}

export async function dryRunBot(): Promise<BotDryRun> {
	try {
		const data = await timedCall('bot.dry_run', () => DryRunBot());
		if (!data) {
			throw new Error('DryRunBot returned empty response');
		}
		clearCache('state');
		return data as unknown as BotDryRun;
	} catch (e) {
		const err = new ApiError(`DryRunBot failed: ${e}`, 500, 'Server Error', String(e));
		lastApiError = err;
		throw err;
	}
}

export async function saveStrategy(strategy: Strategy): Promise<Strategy> {
	try {
		const data = await timedCall('strategy.save', () => UpdateStrategy(strategy as any));
		const res = (data as unknown as Strategy) ?? strategy;
		setCached('strategy', res, cachePolicy.strategy);
		return res;
	} catch (e) {
		const err = new ApiError(`Strategy save failed: ${e}`, 500, 'Server Error', String(e));
		lastApiError = err;
		throw err;
	}
}

export async function validateStrategy(strategy: Strategy): Promise<{ valid: boolean; errors: string[] }> {
	try {
		const res = await timedCall('strategy.validate', () => ValidateStrategy(strategy as any));
		if (!res) {
			return { valid: true, errors: [] };
		}
		return { valid: res.valid, errors: res.errors ?? [] };
	} catch (e) {
		const err = new ApiError(`Strategy validation failed: ${e}`, 500, 'Server Error', String(e));
		lastApiError = err;
		throw err;
	}
}

export async function importStrategy(strategy: Strategy): Promise<Strategy> {
	return saveStrategy(strategy);
}

export async function importStrategyFile(file: File): Promise<Strategy> {
	try {
		const text = await file.text();
		const parsed = JSON.parse(text) as Strategy;
		return await importStrategy(parsed);
	} catch (e) {
		const err = new ApiError(`Strategy file import failed: ${e}`, 500, 'Server Error', String(e));
		lastApiError = err;
		throw err;
	}
}

export async function getStrategyLibrary(): Promise<StrategyLibraryEntry[]> {
	try {
		const data = await timedCall('strategy.library', () => GetStrategyLibrary());
		return (data ?? []) as unknown as StrategyLibraryEntry[];
	} catch (e) {
		const err = new ApiError(`Strategy library fetch failed: ${e}`, 500, 'Server Error', String(e));
		lastApiError = err;
		throw err;
	}
}

export async function saveStrategyPreset(strategy: Strategy, name?: string): Promise<StrategyLibraryEntry> {
	try {
		const toSave: Record<string, unknown> = { ...strategy };
		if (name) toSave.name = name;
		const data = await timedCall('strategy.library.save', () => SaveStrategyPreset(toSave as any));
		if (!data) {
			throw new Error('SaveStrategyPreset returned empty response');
		}
		return data as unknown as StrategyLibraryEntry;
	} catch (e) {
		const err = new ApiError(`Strategy preset save failed: ${e}`, 500, 'Server Error', String(e));
		lastApiError = err;
		throw err;
	}
}

export async function updateStrategyPreset(strategyId: string, strategy: Strategy, name?: string): Promise<StrategyLibraryEntry> {
	try {
		const toSave: Record<string, unknown> = { ...strategy, id: strategyId };
		if (name) toSave.name = name;
		const data = await timedCall('strategy.library.update', () => SaveStrategyPreset(toSave as any));
		if (!data) {
			throw new Error('UpdateStrategyPreset returned empty response');
		}
		return data as unknown as StrategyLibraryEntry;
	} catch (e) {
		const err = new ApiError(`Strategy preset update failed: ${e}`, 500, 'Server Error', String(e));
		lastApiError = err;
		throw err;
	}
}

export async function getStrategyPreset(strategyId: string): Promise<Strategy> {
	try {
		const data = await timedCall('strategy.library.item', () => GetStrategyPreset(strategyId));
		if (!data) {
			throw new Error(`Strategy preset ${strategyId} not found`);
		}
		return data as unknown as Strategy;
	} catch (e) {
		const err = new ApiError(`Strategy preset fetch failed: ${e}`, 500, 'Server Error', String(e));
		lastApiError = err;
		throw err;
	}
}

export async function applyStrategyPreset(strategyId: string, workspaceId = '', slotId = ''): Promise<Strategy> {
	try {
		await timedCall('strategy.library.apply', () => ApplyStrategyPreset(workspaceId, slotId, strategyId));
		const preset = await GetStrategyPreset(strategyId);
		const data = (preset ?? {}) as unknown as Strategy;
		setCached('strategy', data, cachePolicy.strategy);
		return data;
	} catch (e) {
		const err = new ApiError(`Strategy preset apply failed: ${e}`, 500, 'Server Error', String(e));
		lastApiError = err;
		throw err;
	}
}

export async function deleteStrategyPreset(strategyId: string): Promise<void> {
	try {
		await timedCall('strategy.library.delete', () => DeleteStrategyPreset(strategyId));
	} catch (e) {
		const err = new ApiError(`Strategy preset delete failed: ${e}`, 500, 'Server Error', String(e));
		lastApiError = err;
		throw err;
	}
}

export async function getWatchlist(force = false): Promise<WatchlistResponse> {
	const cached = !force ? getCached<WatchlistResponse>('watchlist') : null;
	if (cached) return cached;
	try {
		const data = await timedCall('watchlist.get', () => GetWatchlist());
		const res: WatchlistResponse = (data as unknown as WatchlistResponse) ?? {
			symbols: [],
			groups: [],
			feed: null,
			updated_at: null
		};
		setCached('watchlist', res, cachePolicy.watchlist);
		return res;
	} catch (e) {
		return {
			symbols: [],
			groups: [],
			feed: null,
			updated_at: null
		};
	}
}

export async function getWatchlistById(id: string): Promise<WatchlistResponse> {
	try {
		const data = await timedCall('watchlist.get_by_id', () => GetWatchlistById(id));
		if (!data) {
			throw new Error(`Watchlist ${id} not found`);
		}
		return data as unknown as WatchlistResponse;
	} catch (e) {
		const err = new ApiError(`Watchlist fetch failed: ${e}`, 500, 'Server Error', String(e));
		lastApiError = err;
		throw err;
	}
}

export async function getWatchlists(): Promise<WatchlistSummary[]> {
	try {
		const data = await timedCall('watchlist.list', () => GetWatchlists());
		return (data ?? []) as unknown as WatchlistSummary[];
	} catch (e) {
		return [];
	}
}

export async function saveWatchlist(wl: WatchlistResponse): Promise<void> {
	try {
		await timedCall('watchlist.save', () => SaveWatchlist(wl as any));
		setCached('watchlist', wl, cachePolicy.watchlist);
	} catch (e) {
		const err = new ApiError(`Watchlist save failed: ${e}`, 500, 'Server Error', String(e));
		lastApiError = err;
		throw err;
	}
}

export async function getCockpit(force = false): Promise<CockpitState> {
	const cached = !force ? getCached<CockpitState>('cockpit') : null;
	if (cached) return cached;
	try {
		const data = await timedCall('cockpit.get', () => GetCockpitState());
		const res: CockpitState = (data as unknown as CockpitState) ?? {
			global_enabled: false,
			active_workspace_id: 'default',
			workspaces: [],
			strategy_library: [],
			feed: null,
			updated_at: null
		};
		setCached('cockpit', res, cachePolicy.cockpit);
		return res;
	} catch (e) {
		return {
			global_enabled: false,
			active_workspace_id: 'default',
			workspaces: [],
			strategy_library: [],
			feed: null,
			updated_at: null
		};
	}
}

export async function saveCockpit(payload: {
	global_enabled: boolean;
	active_workspace_id?: string | null;
	workspaces: CockpitWorkspace[];
}): Promise<CockpitState> {
	try {
		const data = await timedCall('cockpit.save', () => UpdateCockpitState(payload as any));
		if (!data) {
			throw new Error('Save cockpit returned empty response');
		}
		const res = data as unknown as CockpitState;
		setCached('cockpit', res, cachePolicy.cockpit);
		return res;
	} catch (e) {
		const err = new ApiError(`Cockpit save failed: ${e}`, 500, 'Server Error', String(e));
		lastApiError = err;
		throw err;
	}
}

export async function replaceWatchlist(
	payload: string[] | { groups: WatchlistGroup[]; feed?: WatchlistFeed | null }
): Promise<WatchlistResponse> {
	try {
		const req = Array.isArray(payload)
			? { symbols: payload }
			: { groups: payload.groups, feed: payload.feed ?? null };
		const data = await timedCall('watchlist.replace', () => ReplaceWatchlist(req as any));
		if (!data) {
			throw new Error('Replace watchlist returned empty response');
		}
		const res = data as unknown as WatchlistResponse;
		setCached('watchlist', res, cachePolicy.watchlist);
		return res;
	} catch (e) {
		const err = new ApiError(`Watchlist replace failed: ${e}`, 500, 'Server Error', String(e));
		lastApiError = err;
		throw err;
	}
}

export async function importTradingViewWatchlist(url: string): Promise<WatchlistResponse> {
	try {
		const data = await timedCall('watchlist.import_tv', () => ImportTradingViewWatchlist(url));
		if (!data) {
			throw new Error('Watchlist import returned empty response');
		}
		setCached('watchlist', data, cachePolicy.watchlist);
		return data as unknown as WatchlistResponse;
	} catch (e) {
		const err = new ApiError(`TradingView watchlist import failed: ${e}`, 500, 'Server Error', String(e));
		lastApiError = err;
		throw err;
	}
}

export async function exportWatchlistFile(): Promise<void> {
	try {
		await timedCall('watchlist.export_file', () => ExportWatchlistFile());
	} catch (e) {
		const err = new ApiError(`Watchlist export failed: ${e}`, 500, 'Server Error', String(e));
		lastApiError = err;
		throw err;
	}
}

export async function importWatchlistFile(): Promise<WatchlistResponse> {
	try {
		const data = await timedCall('watchlist.import_file', () => ImportWatchlistFile());
		if (!data) {
			throw new Error('Watchlist import returned empty response');
		}
		setCached('watchlist', data, cachePolicy.watchlist);
		return data as unknown as WatchlistResponse;
	} catch (e) {
		const err = new ApiError(`Watchlist import failed: ${e}`, 500, 'Server Error', String(e));
		lastApiError = err;
		throw err;
	}
}

export async function refreshWatchlistFeed(watchlistId = ''): Promise<WatchlistResponse> {
	try {
		const data = await timedCall('watchlist.feed_refresh', () => RefreshWatchlistFeed(watchlistId));
		if (!data) {
			throw new Error('Watchlist feed refresh returned empty response');
		}
		const res = data as unknown as WatchlistResponse;
		setCached('watchlist', res, cachePolicy.watchlist);
		return res;
	} catch (e) {
		const err = new ApiError(`Watchlist feed refresh failed: ${e}`, 500, 'Server Error', String(e));
		lastApiError = err;
		throw err;
	}
}

export type SymbolQuery = {
	q?: string;
	type?: string;
	exchange?: string;
	limit?: number;
	refresh?: boolean;
};

export type SymbolsResponse = {
	symbols: SymbolRecord[];
	source?: string;
	updated_at?: string | null;
	warning?: string | null;
};

export async function getSymbols(
	query: SymbolQuery = {},
	force = false
): Promise<SymbolsResponse> {
	const cacheKey = `symbols:${JSON.stringify(query)}`;
	const cached = !force ? getCached<SymbolsResponse>(cacheKey) : null;
	if (cached) return cached;
	try {
		const data = await timedCall('symbols.get', () => GetSymbols(query as any));
		const res: SymbolsResponse = (data as unknown as SymbolsResponse) ?? {
			symbols: [],
			source: 'local_sqlite'
		};
		setCached(cacheKey, res, cachePolicy.symbols);
		return res;
	} catch (e) {
		return {
			symbols: [],
			source: 'local_sqlite'
		};
	}
}

export async function resetCircuitBreaker(): Promise<void> {
	try {
		await timedCall('breaker.reset', () => ResetCircuitBreaker());
	} catch (e) {
		const err = new ApiError(`Circuit breaker reset failed: ${e}`, 500, 'Server Error', String(e));
		lastApiError = err;
		throw err;
	}
}
