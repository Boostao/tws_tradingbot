import { writable } from 'svelte/store';
import { getState } from '$lib/api';

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
	recent_logs?: string[];
	error_message?: string;
	runner_active?: boolean;
};

const defaultState: BotState = {
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

export const botState = writable<BotState>(defaultState);

let pollingTimer: ReturnType<typeof setInterval> | null = null;

export async function refreshBotState(): Promise<void> {
	try {
		const data = await getState();
		botState.set({ ...defaultState, ...(data as Partial<BotState>) });
	} catch {
		// swallow errors to avoid breaking UI
	}
}

export function startBotStatePolling(intervalMs = 5000): void {
	if (pollingTimer) return;
	void refreshBotState();
	pollingTimer = setInterval(() => {
		void refreshBotState();
	}, intervalMs);
}

export function stopBotStatePolling(): void {
	if (pollingTimer) {
		clearInterval(pollingTimer);
		pollingTimer = null;
	}
}
