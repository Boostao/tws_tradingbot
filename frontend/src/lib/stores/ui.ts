import { writable } from 'svelte/store';

export const watchlistDraftUrl = writable<string>('');
export const strategyDraftJson = writable<string>('');
