import { writable } from 'svelte/store';
import { Events } from '@wailsio/runtime';
import { getState, type BotState } from '$lib/api';

const runtimeStateStore = writable<BotState | null>(null);

export const runtimeState = {
	subscribe: runtimeStateStore.subscribe
};

export async function refreshRuntimeState(force = false): Promise<BotState | null> {
    const nextState = await getState();
    runtimeStateStore.set(nextState);
    return nextState;
}

export function setRuntimeState(nextState: BotState | null): void {
	runtimeStateStore.set(nextState);
}

export function startRuntimePolling(): () => void {
    // Initial fetch
    refreshRuntimeState();

    // Listen to Wails events instead of polling
    Events.On('state_update', (event) => {
        runtimeStateStore.set(event.data as BotState);
    });

	return () => {
        // cleanup if needed
	};
}
