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
	watchlist: number;
	symbols: number;
};

const cachePolicy: CachePolicy = {
	strategy: 5000,
	cockpit: 5000,
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

export async function getHealth(): Promise<{ status: string }> {
	const response = await timedFetch('health', `${API_BASE}/health`);
	if (!response.ok) {
		throw await buildApiError(response, 'Health check');
	}
	return response.json();
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

export type Strategy = Record<string, unknown>;
export type PineScriptResponse = {
	script: string;
	warnings: string[];
};

export async function getStrategy(force = false): Promise<Strategy> {
	const cached = !force ? getCached<Strategy>('strategy') : null;
	if (cached) return cached;
	const response = await timedFetch('strategy.get', `${API_BASE}/api/v1/strategy`);
	if (!response.ok) {
		throw await buildApiError(response, 'Strategy fetch');
	}
	const data = await response.json();
	setCached('strategy', data, cachePolicy.strategy);
	return data;
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
	const cached = !force ? getCached<WatchlistResponse>('watchlist') : null;
	if (cached) return cached;
	const response = await timedFetch('watchlist.get', `${API_BASE}/api/v1/watchlist`);
	if (!response.ok) {
		throw await buildApiError(response, 'Watchlist fetch');
	}
	const data = (await response.json()) as WatchlistResponse;
	setCached('watchlist', data, cachePolicy.watchlist);
	return data;
}

export async function getCockpit(force = false): Promise<CockpitState> {
	const cached = !force ? getCached<CockpitState>('cockpit') : null;
	if (cached) return cached;
	const response = await timedFetch('cockpit.get', `${API_BASE}/api/v1/cockpit`);
	if (!response.ok) {
		throw await buildApiError(response, 'Cockpit fetch');
	}
	const data = (await response.json()) as CockpitState;
	setCached('cockpit', data, cachePolicy.cockpit);
	return data;
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

export async function getSymbols(
	query: SymbolQuery = {},
	force = false
): Promise<{ symbols: SymbolRecord[]; source?: string; updated_at?: string | null }> {
	const cacheKey = `symbols:${JSON.stringify(query)}`;
	const cached = !force
		? getCached<{ symbols: SymbolRecord[]; source?: string; updated_at?: string | null }>(cacheKey)
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
