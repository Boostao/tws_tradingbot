const DEFAULT_API_BASE = import.meta.env.DEV ? 'http://localhost:8000' : '';

export const API_BASE =
	import.meta.env.VITE_API_URL ?? DEFAULT_API_BASE;

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

async function timedFetch(
	key: string,
	input: RequestInfo | URL,
	init?: RequestInit
): Promise<Response> {
	const start = typeof performance !== 'undefined' ? performance.now() : Date.now();
	let ok = false;
	try {
		const response = await fetch(input, init);
		ok = response.ok;
		return response;
	} finally {
		const end = typeof performance !== 'undefined' ? performance.now() : Date.now();
		recordRestMetric(key, Math.max(0, end - start), ok);
	}
}

async function buildApiError(response: Response, label: string): Promise<ApiError> {
	let detail = '';
	try {
		const text = await response.text();
		if (text) {
			try {
				const parsed = JSON.parse(text) as Record<string, unknown>;
				const candidate =
					(typeof parsed.detail === 'string' && parsed.detail) ||
					(typeof parsed.message === 'string' && parsed.message) ||
					(typeof parsed.error === 'string' && parsed.error) ||
					'';
				detail = candidate || text;
			} catch {
				detail = text;
			}
		}
	} catch {
		detail = '';
	}

	const trimmed = detail.trim();
	const detailText = trimmed ? ` - ${trimmed.slice(0, 300)}` : '';
	const statusText = response.statusText ? ` ${response.statusText}` : '';
	const message = `${label} failed: ${response.status}${statusText}${detailText}`;
	const error = new ApiError(message, response.status, response.statusText, trimmed, response.url);
	lastApiError = error;
	return error;
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

async function fetchJson<T>(key: string, url: string, label: string, init?: RequestInit): Promise<T> {
	const response = await timedFetch(key, url, init);
	if (!response.ok) {
		throw await buildApiError(response, label);
	}
	return (await response.json()) as T;
}

async function fetchCachedJson<T>(
	cacheKey: string,
	ttlMs: number,
	requestKey: string,
	url: string,
	label: string,
	force = false
): Promise<T> {
	const cached = !force ? getCached<T>(cacheKey) : null;
	if (cached) return cached;
	const data = await fetchJson<T>(requestKey, url, label);
	setCached(cacheKey, data, ttlMs);
	return data;
}

export async function getHealth(): Promise<{ status: string }> {
	return fetchJson<{ status: string }>('health', `${API_BASE}/health`, 'Health check');
}

export type SymbolRecord = {
	symbol: string;
	name?: string;
	exchange?: string;
	type?: string;
};

export type WatchlistItem = {
	symbol: string;
	exchange?: string;
	name?: string;
	enabled: boolean;
};

export type WatchlistGroup = {
	id: string;
	name: string;
	source?: string;
	items: WatchlistItem[];
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
export type PineScriptResponse = {
	script: string;
	warnings: string[];
};

export async function getStrategy(force = false): Promise<Strategy> {
	return fetchCachedJson<Strategy>('strategy', cachePolicy.strategy, 'strategy.get', `${API_BASE}/api/v1/strategy`, 'Strategy fetch', force);
}

export async function getConfig(force = false): Promise<ConfigResponse> {
	return fetchCachedJson<ConfigResponse>('config', cachePolicy.config, 'config.get', `${API_BASE}/api/v1/config`, 'Config fetch', force);
}

export async function updateConfig(updates: Record<string, Record<string, unknown>>): Promise<ConfigResponse> {
	const response = await timedFetch('config.update', `${API_BASE}/api/v1/config`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ updates })
	});
	if (!response.ok) {
		throw await buildApiError(response, 'Config update');
	}
	const data = (await response.json()) as ConfigResponse;
	setCached('config', data, cachePolicy.config);
	return data;
}

export async function getState(force = false): Promise<BotState> {
	return fetchCachedJson<BotState>('state', cachePolicy.state, 'state.get', `${API_BASE}/api/v1/state`, 'State fetch', force);
}

export async function getDiagnostics(force = false): Promise<DiagnosticsResponse> {
	return fetchCachedJson<DiagnosticsResponse>(
		'diagnostics',
		cachePolicy.diagnostics,
		'diagnostics.get',
		`${API_BASE}/api/v1/diagnostics`,
		'Diagnostics fetch',
		force
	);
}

export async function connectTws(payload: { host?: string; port?: number; client_id?: number }): Promise<{ status: string }> {
	const response = await timedFetch('tws.connect', `${API_BASE}/api/v1/tws/connect`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(payload)
	});
	if (!response.ok) {
		throw await buildApiError(response, 'TWS connect');
	}
	clearCache('state');
	clearCache('config');
	clearCache('diagnostics');
	return response.json();
}

export async function disconnectTws(): Promise<{ status: string }> {
	const response = await timedFetch('tws.disconnect', `${API_BASE}/api/v1/tws/disconnect`, {
		method: 'POST'
	});
	if (!response.ok) {
		throw await buildApiError(response, 'TWS disconnect');
	}
	clearCache('state');
	return response.json();
}

export async function startBot(): Promise<{ status: string }> {
	const response = await timedFetch('bot.start', `${API_BASE}/api/v1/bot/start`, {
		method: 'POST'
	});
	if (!response.ok) {
		throw await buildApiError(response, 'Bot start');
	}
	clearCache('state');
	return response.json();
}

export async function stopBot(): Promise<{ status: string }> {
	const response = await timedFetch('bot.stop', `${API_BASE}/api/v1/bot/stop`, {
		method: 'POST'
	});
	if (!response.ok) {
		throw await buildApiError(response, 'Bot stop');
	}
	clearCache('state');
	return response.json();
}

export async function dryRunBot(): Promise<BotDryRun> {
	const response = await timedFetch('bot.dry_run', `${API_BASE}/api/v1/bot/dry_run`, {
		method: 'POST'
	});
	if (!response.ok) {
		throw await buildApiError(response, 'Bot dry run');
	}
	clearCache('state');
	return response.json();
}

export async function saveStrategy(strategy: Strategy): Promise<Strategy> {
	const response = await timedFetch('strategy.save', `${API_BASE}/api/v1/strategy`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(strategy)
	});
	if (!response.ok) {
		throw await buildApiError(response, 'Strategy save');
	}
	const data = await response.json();
	setCached('strategy', data, cachePolicy.strategy);
	return data;
}

export async function validateStrategy(strategy: Strategy): Promise<{ valid: boolean; errors: string[] }> {
	const response = await timedFetch('strategy.validate', `${API_BASE}/api/v1/strategy/validate`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(strategy)
	});
	if (!response.ok) {
		throw await buildApiError(response, 'Strategy validation');
	}
	return response.json();
}

export async function importStrategy(strategy: Strategy): Promise<Strategy> {
	const response = await timedFetch('strategy.import', `${API_BASE}/api/v1/strategy/import`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(strategy)
	});
	if (!response.ok) {
		throw await buildApiError(response, 'Strategy import');
	}
	const data = await response.json();
	setCached('strategy', data, cachePolicy.strategy);
	return data;
}

export async function importStrategyFile(file: File): Promise<Strategy> {
	const formData = new FormData();
	formData.append('file', file);
	const response = await timedFetch('strategy.import_file', `${API_BASE}/api/v1/strategy/import/file`, {
		method: 'POST',
		body: formData
	});
	if (!response.ok) {
		throw await buildApiError(response, 'Strategy file import');
	}
	const data = await response.json();
	setCached('strategy', data, cachePolicy.strategy);
	return data;
}

export async function getStrategyPineScript(): Promise<PineScriptResponse> {
	const response = await timedFetch('strategy.pine', `${API_BASE}/api/v1/strategy/pine-script`);
	if (!response.ok) {
		throw await buildApiError(response, 'PineScript generation');
	}
	return response.json();
}

export async function getStrategyLibrary(): Promise<StrategyLibraryEntry[]> {
	const response = await timedFetch('strategy.library', `${API_BASE}/api/v1/strategy/library`);
	if (!response.ok) {
		throw await buildApiError(response, 'Strategy library fetch');
	}
	return (await response.json()) as StrategyLibraryEntry[];
}

export async function saveStrategyPreset(strategy: Strategy, name?: string): Promise<StrategyLibraryEntry> {
	const response = await timedFetch('strategy.library.save', `${API_BASE}/api/v1/strategy/library/save`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ strategy, name })
	});
	if (!response.ok) {
		throw await buildApiError(response, 'Strategy preset save');
	}
	return (await response.json()) as StrategyLibraryEntry;
}

export async function updateStrategyPreset(strategyId: string, strategy: Strategy, name?: string): Promise<StrategyLibraryEntry> {
	const response = await timedFetch('strategy.library.update', `${API_BASE}/api/v1/strategy/library/${strategyId}`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ strategy, name })
	});
	if (!response.ok) {
		throw await buildApiError(response, 'Strategy preset update');
	}
	return (await response.json()) as StrategyLibraryEntry;
}

export async function getStrategyPreset(strategyId: string): Promise<Strategy> {
	const response = await timedFetch('strategy.library.item', `${API_BASE}/api/v1/strategy/library/${strategyId}`);
	if (!response.ok) {
		throw await buildApiError(response, 'Strategy preset fetch');
	}
	return (await response.json()) as Strategy;
}

export async function applyStrategyPreset(strategyId: string): Promise<Strategy> {
	const response = await timedFetch('strategy.library.apply', `${API_BASE}/api/v1/strategy/library/${strategyId}/apply`, {
		method: 'POST'
	});
	if (!response.ok) {
		throw await buildApiError(response, 'Strategy preset apply');
	}
	const data = (await response.json()) as Strategy;
	setCached('strategy', data, cachePolicy.strategy);
	return data;
}

export async function deleteStrategyPreset(strategyId: string): Promise<void> {
	const response = await timedFetch('strategy.library.delete', `${API_BASE}/api/v1/strategy/library/${strategyId}`, {
		method: 'DELETE'
	});
	if (!response.ok) {
		throw await buildApiError(response, 'Strategy preset delete');
	}
}

export async function getWatchlist(force = false): Promise<WatchlistResponse> {
	return fetchCachedJson<WatchlistResponse>('watchlist', cachePolicy.watchlist, 'watchlist.get', `${API_BASE}/api/v1/watchlist`, 'Watchlist fetch', force);
}

export async function getCockpit(force = false): Promise<CockpitState> {
	return fetchCachedJson<CockpitState>('cockpit', cachePolicy.cockpit, 'cockpit.get', `${API_BASE}/api/v1/cockpit`, 'Cockpit fetch', force);
}

export async function saveCockpit(payload: {
	global_enabled: boolean;
	active_workspace_id?: string | null;
	workspaces: CockpitWorkspace[];
}): Promise<CockpitState> {
	const response = await timedFetch('cockpit.save', `${API_BASE}/api/v1/cockpit`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(payload)
	});
	if (!response.ok) {
		throw await buildApiError(response, 'Cockpit update');
	}
	const data = (await response.json()) as CockpitState;
	setCached('cockpit', data, cachePolicy.cockpit);
	return data;
}

export async function replaceWatchlist(
	payload: string[] | { groups: WatchlistGroup[]; feed?: WatchlistFeed | null }
): Promise<WatchlistResponse> {
	const response = await timedFetch('watchlist.replace', `${API_BASE}/api/v1/watchlist`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: Array.isArray(payload)
			? JSON.stringify({ symbols: payload })
			: JSON.stringify({ groups: payload.groups, feed: payload.feed ?? null })
	});
	if (!response.ok) {
		throw await buildApiError(response, 'Watchlist update');
	}
	const data = (await response.json()) as WatchlistResponse;
	setCached('watchlist', data, cachePolicy.watchlist);
	return data;
}

export async function importTradingViewWatchlist(url: string): Promise<WatchlistResponse> {
	const response = await timedFetch(
		'watchlist.import_tradingview',
		`${API_BASE}/api/v1/watchlist/import/tradingview`,
		{
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ url })
		}
	);
	if (!response.ok) {
		throw await buildApiError(response, 'TradingView watchlist import');
	}
	const data = (await response.json()) as WatchlistResponse;
	setCached('watchlist', data, cachePolicy.watchlist);
	return data;
}

export async function refreshWatchlistFeed(): Promise<WatchlistResponse> {
	const response = await timedFetch('watchlist.feed_refresh', `${API_BASE}/api/v1/watchlist/feed/refresh`, {
		method: 'POST'
	});
	if (!response.ok) {
		throw await buildApiError(response, 'Watchlist feed refresh');
	}
	const data = (await response.json()) as WatchlistResponse;
	setCached('watchlist', data, cachePolicy.watchlist);
	return data;
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
	const cached = !force
		? getCached<SymbolsResponse>(cacheKey)
		: null;
	if (cached) return cached;
	const params = new URLSearchParams();
	if (query.q) params.set('q', query.q);
	if (query.type) params.set('type', query.type);
	if (query.exchange) params.set('exchange', query.exchange);
	if (typeof query.limit === 'number') params.set('limit', String(query.limit));
	if (query.refresh) params.set('refresh', 'true');
	const url = params.toString() ? `${API_BASE}/api/v1/symbols?${params}` : `${API_BASE}/api/v1/symbols`;
	const response = await timedFetch('symbols.get', url);
	if (!response.ok) {
		throw await buildApiError(response, 'Symbols fetch');
	}
	const data = await response.json();
	setCached(cacheKey, data, cachePolicy.symbols);
	return data;
}
