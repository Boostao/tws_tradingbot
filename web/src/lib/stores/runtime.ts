import { writable } from 'svelte/store';

import { getState, type BotState } from '$lib/api';

const DEFAULT_RUNTIME_POLL_MS = 5000;

const runtimeStateStore = writable<BotState | null>(null);

let runtimePollTimer: ReturnType<typeof setInterval> | null = null;
let runtimePollConsumers = 0;
let runtimeRefreshPromise: Promise<BotState | null> | null = null;

export const runtimeState = {
	subscribe: runtimeStateStore.subscribe
};

export async function refreshRuntimeState(force = false): Promise<BotState | null> {
	if (runtimeRefreshPromise) {
		return runtimeRefreshPromise;
	}

	runtimeRefreshPromise = (async () => {
		try {
			const nextState = await getState(force);
			runtimeStateStore.set(nextState);
			return nextState;
		} catch {
			return null;
		} finally {
			runtimeRefreshPromise = null;
		}
	})();

	return runtimeRefreshPromise;
}

export function setRuntimeState(nextState: BotState | null): void {
	runtimeStateStore.set(nextState);
}

export function startRuntimePolling(intervalMs = DEFAULT_RUNTIME_POLL_MS): () => void {
	runtimePollConsumers += 1;
	void refreshRuntimeState(true);

	if (runtimePollTimer === null) {
		runtimePollTimer = setInterval(() => {
			void refreshRuntimeState(true);
		}, intervalMs);
	}

	return () => {
		runtimePollConsumers = Math.max(0, runtimePollConsumers - 1);
		if (runtimePollConsumers === 0 && runtimePollTimer !== null) {
			clearInterval(runtimePollTimer);
			runtimePollTimer = null;
		}
	};
}