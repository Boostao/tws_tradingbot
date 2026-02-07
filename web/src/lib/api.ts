export const API_BASE = import.meta.env.VITE_API_URL ?? 'http://localhost:8000';

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
	config: number;
	watchlist: number;
	symbols: number;
	backtestStatus: number;
	backtestResults: number;
};

const cachePolicy: CachePolicy = {
	strategy: 5000,
	config: 10000,
	watchlist: 10000,
	symbols: 60000,
	backtestStatus: 1000,
	backtestResults: 60000
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

export function clearCacheByPrefix(prefix: string): void {
	for (const key of cache.keys()) {
		if (key.startsWith(prefix)) {
			cache.delete(key);
		}
	}
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

export type BacktestRunRequest = {
	tickers: string[];
	start_date: string;
	end_date: string;
	timeframe: string;
	initial_capital: number;
	use_tws_data: boolean;
	use_nautilus: boolean;
};

export type BacktestRunResponse = { job_id: string };

export type BacktestStatusResponse = {
	job_id: string;
	status: string;
	error?: string | null;
	started_at?: string | null;
	finished_at?: string | null;
};

export type BacktestResultResponse = {
	job_id: string;
	status: string;
	result?: Record<string, unknown> | null;
	error?: string | null;
};

export type SymbolRecord = {
	symbol: string;
	name?: string;
	exchange?: string;
	type?: string;
};

export type ConfigResponse = {
	ib?: {
		host?: string;
		port?: number;
		client_id?: number;
	};
};

export async function getConfig(force = false): Promise<ConfigResponse> {
	const cacheKey = 'config';
	const cached = !force ? getCached<ConfigResponse>(cacheKey) : null;
	if (cached) return cached;
	const response = await timedFetch('config.get', `${API_BASE}/api/v1/config`);
	if (!response.ok) {
		throw await buildApiError(response, 'Config');
	}
	const data = (await response.json()) as ConfigResponse;
	setCached(cacheKey, data, cachePolicy.config);
	return data;
}

export async function runBacktest(payload: BacktestRunRequest): Promise<BacktestRunResponse> {
	const response = await timedFetch('backtest.run', `${API_BASE}/api/v1/backtest/run`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(payload)
	});
	if (!response.ok) {
		throw await buildApiError(response, 'Backtest run');
	}
	return response.json();
}

export async function getBacktestStatus(
	jobId: string,
	force = false
): Promise<BacktestStatusResponse> {
	const cacheKey = `backtest_status:${jobId}`;
	const cached = !force ? getCached<BacktestStatusResponse>(cacheKey) : null;
	if (cached) return cached;
	const response = await timedFetch('backtest.status', `${API_BASE}/api/v1/backtest/${jobId}`);
	if (!response.ok) {
		throw await buildApiError(response, 'Backtest status');
	}
	const data = await response.json();
	setCached(cacheKey, data, cachePolicy.backtestStatus);
	return data;
}

export async function getBacktestResults(
	jobId: string,
	force = false
): Promise<BacktestResultResponse> {
	const cacheKey = `backtest_results:${jobId}`;
	const cached = !force ? getCached<BacktestResultResponse>(cacheKey) : null;
	if (cached) return cached;
	const response = await timedFetch('backtest.results', `${API_BASE}/api/v1/backtest/${jobId}/results`);
	if (!response.ok) {
		throw await buildApiError(response, 'Backtest results');
	}
	const data = await response.json();
	if (data?.status === 'completed') {
		setCached(cacheKey, data, cachePolicy.backtestResults);
	}
	return data;
}

export type Strategy = Record<string, unknown>;

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

export async function exportStrategy(): Promise<Strategy> {
	const response = await timedFetch('strategy.export', `${API_BASE}/api/v1/strategy/export`);
	if (!response.ok) {
		throw await buildApiError(response, 'Strategy export');
	}
	const data = await response.json();
	setCached('strategy', data, cachePolicy.strategy);
	return data;
}

export async function getWatchlist(force = false): Promise<{ symbols: string[] }> {
	const cached = !force ? getCached<{ symbols: string[] }>('watchlist') : null;
	if (cached) return cached;
	const response = await timedFetch('watchlist.get', `${API_BASE}/api/v1/watchlist`);
	if (!response.ok) {
		throw await buildApiError(response, 'Watchlist fetch');
	}
	const data = await response.json();
	setCached('watchlist', data, cachePolicy.watchlist);
	return data;
}

export async function replaceWatchlist(symbols: string[]): Promise<{ symbols: string[] }> {
	const response = await timedFetch('watchlist.replace', `${API_BASE}/api/v1/watchlist`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ symbols })
	});
	if (!response.ok) {
		throw await buildApiError(response, 'Watchlist update');
	}
	const data = await response.json();
	setCached('watchlist', data, cachePolicy.watchlist);
	return data;
}

export async function getState(): Promise<Record<string, unknown>> {
	const response = await timedFetch('state.get', `${API_BASE}/api/v1/state`);
	if (!response.ok) {
		throw await buildApiError(response, 'State fetch');
	}
	return response.json();
}

export async function getLogs(): Promise<{ logs: string[] }> {
	const response = await timedFetch('logs.get', `${API_BASE}/api/v1/logs`);
	if (!response.ok) {
		throw await buildApiError(response, 'Logs fetch');
	}
	return response.json();
}

export async function startBot(): Promise<{ status: string }> {
	const response = await timedFetch('bot.start', `${API_BASE}/api/v1/bot/start`, { method: 'POST' });
	if (!response.ok) {
		throw await buildApiError(response, 'Bot start');
	}
	return response.json();
}

export async function stopBot(): Promise<{ status: string }> {
	const response = await timedFetch('bot.stop', `${API_BASE}/api/v1/bot/stop`, { method: 'POST' });
	if (!response.ok) {
		throw await buildApiError(response, 'Bot stop');
	}
	return response.json();
}

export async function emergencyStop(): Promise<{ status: string }> {
	const response = await timedFetch('bot.emergency_stop', `${API_BASE}/api/v1/bot/emergency_stop`, {
		method: 'POST'
	});
	if (!response.ok) {
		throw await buildApiError(response, 'Emergency stop');
	}
	return response.json();
}

export async function connectTws(payload: { host?: string; port?: number; client_id?: number }): Promise<{ connected: boolean }> {
	const response = await timedFetch('tws.connect', `${API_BASE}/api/v1/tws/connect`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(payload)
	});
	if (!response.ok) {
		throw await buildApiError(response, 'TWS connect');
	}
	return response.json();
}

export async function disconnectTws(): Promise<{ connected: boolean }> {
	const response = await timedFetch('tws.disconnect', `${API_BASE}/api/v1/tws/disconnect`, {
		method: 'POST'
	});
	if (!response.ok) {
		throw await buildApiError(response, 'TWS disconnect');
	}
	return response.json();
}

export async function updateConfig(
	updates: Record<string, Record<string, unknown>>
): Promise<Record<string, unknown>> {
	const response = await timedFetch('config.update', `${API_BASE}/api/v1/config`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ updates })
	});
	if (!response.ok) {
		throw await buildApiError(response, 'Config update');
	}
	const data = await response.json();
	setCached('config', data, cachePolicy.config);
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
	const cached = !force ? getCached<{ symbols: SymbolRecord[]; source?: string; updated_at?: string | null }>(cacheKey) : null;
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

export async function testNotification(
	message: string,
	channel?: 'telegram' | 'discord'
): Promise<{ status: string }> {
	const response = await timedFetch('notifications.test', `${API_BASE}/api/v1/notifications/test`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ message, channel })
	});
	if (!response.ok) {
		throw await buildApiError(response, 'Notification test');
	}
	return response.json();
}

export async function getNotifications(
	page = 1,
	pageSize = 25
): Promise<{ events: Array<Record<string, unknown>>; total: number; page: number; page_size: number }> {
	const params = new URLSearchParams({
		page: String(page),
		page_size: String(pageSize)
	});
	const response = await timedFetch(
		'notifications.list',
		`${API_BASE}/api/v1/notifications?${params.toString()}`
	);
	if (!response.ok) {
		throw await buildApiError(response, 'Notifications fetch');
	}
	return response.json();
}
