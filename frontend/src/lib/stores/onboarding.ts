import { goto } from '$app/navigation';
import { get, writable } from 'svelte/store';

export type TourTrackId = 1 | 2 | 3;
export type TourMode = 'welcome' | 'step';

export interface TourState {
	open: boolean;
	mode: TourMode;
	track: TourTrackId;
	step: number;
}

export interface TourStepDef {
	selector: string;
	fallbackSelector: string;
	titleKey: string;
	bodyKey: string;
	tipKey: string;
}

export interface TourTrackDef {
	id: TourTrackId;
	route: string;
	labelKey: string;
	titleKey: string;
	summaryKey: string;
	durationKey: string;
	steps: TourStepDef[];
}

export const STORAGE_KEY = 'cobalt_onboarding_state';

export const TOUR_TRACKS: Record<TourTrackId, TourTrackDef> = {
	1: {
		id: 1,
		route: '/cockpit',
		labelKey: 'tour_track1_badge',
		titleKey: 'tour_track1_title',
		summaryKey: 'tour_track1_desc',
		durationKey: 'tour_track1_duration',
		steps: [
			{
				selector: '.cockpit-hud-strip',
				fallbackSelector: '.cockpit-top-actions',
				titleKey: 'tour_t1_s1_title',
				bodyKey: 'tour_t1_s1_body',
				tipKey: 'tour_t1_s1_tip'
			},
			{
				selector: '.cockpit-tabs',
				fallbackSelector: '.cockpit-header',
				titleKey: 'tour_t1_s2_title',
				bodyKey: 'tour_t1_s2_body',
				tipKey: 'tour_t1_s2_tip'
			},
			{
				selector: '.cockpit-slot-list',
				fallbackSelector: '.cockpit-grid > .cockpit-column:first-child',
				titleKey: 'tour_t1_s3_title',
				bodyKey: 'tour_t1_s3_body',
				tipKey: 'tour_t1_s3_tip'
			},
			{
				selector: '.cockpit-watchlist-pillbar',
				fallbackSelector: '.cockpit-meta-strip',
				titleKey: 'tour_t1_s4_title',
				bodyKey: 'tour_t1_s4_body',
				tipKey: 'tour_t1_s4_tip'
			},
			{
				selector: '.cockpit-ticker-row',
				fallbackSelector: '.cockpit-group-list',
				titleKey: 'tour_t1_s5_title',
				bodyKey: 'tour_t1_s5_body',
				tipKey: 'tour_t1_s5_tip'
			}
		]
	},
	2: {
		id: 2,
		route: '/watchlist',
		labelKey: 'tour_track2_badge',
		titleKey: 'tour_track2_title',
		summaryKey: 'tour_track2_desc',
		durationKey: 'tour_track2_duration',
		steps: [
			{
				selector: '.watchlist-toolbar',
				fallbackSelector: 'select.watchlist-selector',
				titleKey: 'tour_t2_s1_title',
				bodyKey: 'tour_t2_s1_body',
				tipKey: 'tour_t2_s1_tip'
			},
			{
				selector: '.watchlist-summary',
				fallbackSelector: '.watchlist-toolbar',
				titleKey: 'tour_t2_s2_title',
				bodyKey: 'tour_t2_s2_body',
				tipKey: 'tour_t2_s2_tip'
			},
			{
				selector: '.watchlist-item-row.is-holding, .watchlist-item-row',
				fallbackSelector: '.watchlist-groups',
				titleKey: 'tour_t2_s3_title',
				bodyKey: 'tour_t2_s3_body',
				tipKey: 'tour_t2_s3_tip'
			},
			{
				selector: '.watchlist-item-actions',
				fallbackSelector: '.watchlist-groups',
				titleKey: 'tour_t2_s4_title',
				bodyKey: 'tour_t2_s4_body',
				tipKey: 'tour_t2_s4_tip'
			}
		]
	},
	3: {
		id: 3,
		route: '/strategy',
		labelKey: 'tour_track3_badge',
		titleKey: 'tour_track3_title',
		summaryKey: 'tour_track3_desc',
		durationKey: 'tour_track3_duration',
		steps: [
			{
				selector: '[data-testid="strategy-pane-left"]',
				fallbackSelector: '.strategy-pane-left',
				titleKey: 'tour_t3_s1_title',
				bodyKey: 'tour_t3_s1_body',
				tipKey: 'tour_t3_s1_tip'
			},
			{
				selector: '.center-header',
				fallbackSelector: '[data-testid="strategy-pane-center"]',
				titleKey: 'tour_t3_s2_title',
				bodyKey: 'tour_t3_s2_body',
				tipKey: 'tour_t3_s2_tip'
			},
			{
				selector: '.pipeline-body',
				fallbackSelector: '[data-testid="strategy-pane-center"]',
				titleKey: 'tour_t3_s3_title',
				bodyKey: 'tour_t3_s3_body',
				tipKey: 'tour_t3_s3_tip'
			},
			{
				selector: '[data-testid="strategy-pane-right"]',
				fallbackSelector: '.strategy-pane-right',
				titleKey: 'tour_t3_s4_title',
				bodyKey: 'tour_t3_s4_body',
				tipKey: 'tour_t3_s4_tip'
			}
		]
	}
};

export const helpDrawerOpen = writable<boolean>(false);

export const tourStore = writable<TourState>({
	open: false,
	mode: 'welcome',
	track: 1,
	step: 0
});

function hasStorage(): boolean {
	return typeof window !== 'undefined' && typeof localStorage !== 'undefined';
}

export function openHelpDrawer(): void {
	helpDrawerOpen.set(true);
}

export function closeHelpDrawer(): void {
	helpDrawerOpen.set(false);
}

export function toggleHelpDrawer(): void {
	helpDrawerOpen.update((open) => !open);
}

export function initOnboardingOnCockpit(): void {
	if (!hasStorage()) return;
	if (get(tourStore).open) return;

	try {
		const raw = localStorage.getItem(STORAGE_KEY);
		if (raw) {
			const parsed = JSON.parse(raw) as { completed?: boolean; dismissed?: boolean };
			if (parsed?.completed || parsed?.dismissed) {
				return;
			}
		}
	} catch {
		// Malformed storage entry: treat as not completed and offer welcome prompt
	}

	tourStore.set({
		open: true,
		mode: 'welcome',
		track: 1,
		step: 0
	});
}

export function launchTour(track: TourTrackId): void {
	helpDrawerOpen.set(false);
	tourStore.set({
		open: true,
		mode: 'step',
		track,
		step: 0
	});

	const targetRoute = TOUR_TRACKS[track].route;
	if (typeof window !== 'undefined' && window.location.pathname !== targetRoute) {
		void goto(targetRoute);
	}
}

export function nextTourStep(): void {
	const current = get(tourStore);
	if (!current.open) return;

	if (current.mode === 'welcome') {
		launchTour(current.track);
		return;
	}

	const total = TOUR_TRACKS[current.track].steps.length;
	if (current.step < total - 1) {
		tourStore.set({
			...current,
			step: current.step + 1
		});
	} else {
		completeOrDismissTour(true);
	}
}

export function prevTourStep(): void {
	const current = get(tourStore);
	if (!current.open || current.mode !== 'step') return;

	if (current.step > 0) {
		tourStore.set({
			...current,
			step: current.step - 1
		});
	}
}

export function completeOrDismissTour(completed: boolean): void {
	const current = get(tourStore);
	if (hasStorage()) {
		try {
			localStorage.setItem(
				STORAGE_KEY,
				JSON.stringify({
					completed,
					dismissed: true,
					track: current.track,
					step: completed ? TOUR_TRACKS[current.track].steps.length : current.step + 1
				})
			);
		} catch {
			// Ignore storage quota errors
		}
	}
	tourStore.set({
		...current,
		open: false
	});
}
