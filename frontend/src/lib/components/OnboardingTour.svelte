<script lang="ts">
	import { onDestroy, tick } from 'svelte';
	import { goto } from '$app/navigation';
	import { Compass, Sparkles, X } from 'lucide-svelte';
	import { language, t } from '$lib/i18n';
	import {
		completeOrDismissTour,
		launchTour,
		nextTourStep,
		prevTourStep,
		TOUR_TRACKS,
		tourStore,
		type TourTrackId
	} from '$lib/stores/onboarding';

	interface RectBox {
		top: number;
		left: number;
		width: number;
		height: number;
	}

	$: lang = $language;
	$: state = $tourStore;
	$: activeTrack = TOUR_TRACKS[state.track];
	$: activeSteps = activeTrack.steps;
	$: activeStep = activeSteps[state.step] ?? activeSteps[0];
	$: isLastStep = state.step >= activeSteps.length - 1;

	const RING_PAD = 6;

	let tourRootEl: HTMLDivElement | null = null;
	let targetRect: RectBox | null = null;
	let popoverEl: HTMLDivElement | null = null;
	let popoverLeft = 24;
	let popoverTop = 24;
	let popoverCentered = true;

	let cleanupTracker: (() => void) | null = null;
	let lastScrolledEl: Element | null = null;

	function computeVisibleTargetRect(el: Element): RectBox | null {
		if (typeof window === 'undefined') return null;
		const raw = el.getBoundingClientRect();
		let left = raw.left;
		let top = raw.top;
		let right = raw.right;
		let bottom = raw.bottom;

		let ancestor = el.parentElement;
		while (ancestor && ancestor !== document.body && ancestor !== document.documentElement) {
			const cs = window.getComputedStyle(ancestor);
			const clipsX =
				cs.overflowX === 'hidden' ||
				cs.overflowX === 'clip' ||
				cs.overflowX === 'auto' ||
				cs.overflowX === 'scroll';
			const clipsY =
				cs.overflowY === 'hidden' ||
				cs.overflowY === 'clip' ||
				cs.overflowY === 'auto' ||
				cs.overflowY === 'scroll';
			if (clipsX || clipsY) {
				const ar = ancestor.getBoundingClientRect();
				if (clipsX) {
					left = Math.max(left, ar.left);
					right = Math.min(right, ar.right);
				}
				if (clipsY) {
					top = Math.max(top, ar.top);
					bottom = Math.min(bottom, ar.bottom);
				}
			}
			ancestor = ancestor.parentElement;
		}

		const vw = window.innerWidth;
		const vh = window.innerHeight;
		left = Math.max(RING_PAD, left);
		top = Math.max(RING_PAD, top);
		right = Math.min(vw - RING_PAD, right);
		bottom = Math.min(vh - RING_PAD, bottom);

		const width = right - left;
		const height = bottom - top;
		if (width < 12 || height < 12) {
			return null;
		}
		return { top, left, width, height };
	}

	function findTargetElement(selector: string, fallbackSelector: string): Element | null {
		if (typeof document === 'undefined') return null;
		const primary = document.querySelector(selector);
		if (primary) {
			const r = primary.getBoundingClientRect();
			if (r.width > 0 && r.height > 0) return primary;
		}
		if (fallbackSelector) {
			const fallback = document.querySelector(fallbackSelector);
			if (fallback) {
				const r = fallback.getBoundingClientRect();
				if (r.width > 0 && r.height > 0) return fallback;
			}
		}
		return null;
	}

	async function updatePopoverPosition() {
		if (typeof window === 'undefined') return;
		await tick();
		const vw = window.innerWidth;
		const vh = window.innerHeight;
		const margin = 16;
		const measuredW = popoverEl?.offsetWidth || Math.min(390, vw - margin * 2);
		const measuredH = popoverEl?.offsetHeight || 220;

		if (!targetRect) {
			popoverCentered = true;
			popoverLeft = Math.max(margin, Math.round((vw - measuredW) / 2));
			popoverTop = Math.max(margin, Math.round((vh - measuredH) / 2));
			return;
		}

		popoverCentered = false;
		const gap = 14;
		const pad = RING_PAD;
		const spotTop = targetRect.top - pad;
		const spotBottom = targetRect.top + targetRect.height + pad;
		const spotLeft = targetRect.left - pad;

		const spaceBelow = vh - spotBottom - margin;
		const spaceAbove = spotTop - margin;

		let top: number;
		if (spaceBelow >= measuredH + gap) {
			top = spotBottom + gap;
		} else if (spaceAbove >= measuredH + gap) {
			top = spotTop - measuredH - gap;
		} else if (spaceBelow >= spaceAbove) {
			top = Math.min(vh - measuredH - margin, spotBottom + gap);
		} else {
			top = Math.max(margin, spotTop - measuredH - gap);
		}

		let left = spotLeft;
		if (left + measuredW > vw - margin) {
			left = vw - measuredW - margin;
		}
		if (left < margin) {
			left = margin;
		}
		top = Math.max(margin, Math.min(top, vh - measuredH - margin));

		popoverLeft = Math.round(left);
		popoverTop = Math.round(top);
	}

	function stopTracking() {
		if (cleanupTracker) {
			cleanupTracker();
			cleanupTracker = null;
		}
	}

	function startTracking(trackId: TourTrackId, stepIdx: number) {
		stopTracking();
		if (typeof window === 'undefined' || typeof document === 'undefined') return;

		const trackDef = TOUR_TRACKS[trackId];
		const stepDef = trackDef.steps[stepIdx] ?? trackDef.steps[0];
		if (!stepDef) return;

		if (window.location.pathname !== trackDef.route) {
			void goto(trackDef.route);
		}

		let disposed = false;
		let rafId = 0;
		let retryCount = 0;
		const maxRetries = 90; // ~1500ms at 60fps
		lastScrolledEl = null;

		const measure = () => {
			if (disposed) return;
			const el = findTargetElement(stepDef.selector, stepDef.fallbackSelector);
			if (el) {
				if (el !== lastScrolledEl) {
					lastScrolledEl = el;
					const r0 = el.getBoundingClientRect();
					if (r0.top < 0 || r0.bottom > window.innerHeight || r0.left < 0 || r0.right > window.innerWidth) {
						el.scrollIntoView({ block: 'nearest', inline: 'nearest' });
					}
				}
				targetRect = computeVisibleTargetRect(el);
			} else {
				targetRect = null;
			}
			void updatePopoverPosition();
		};

		const tickResolve = () => {
			if (disposed) return;
			measure();
			if (!targetRect && retryCount < maxRetries) {
				retryCount += 1;
				rafId = window.requestAnimationFrame(tickResolve);
			}
		};

		const observer = new MutationObserver((mutations) => {
			if (disposed) return;
			const hasExternalMutation = mutations.some(
				(m) => !tourRootEl || !tourRootEl.contains(m.target)
			);
			if (hasExternalMutation) {
				measure();
			}
		});
		observer.observe(document.body, { childList: true, subtree: true, attributes: true });

		const handleViewportChange = () => measure();
		window.addEventListener('resize', handleViewportChange);
		window.addEventListener('scroll', handleViewportChange, true);

		rafId = window.requestAnimationFrame(tickResolve);

		cleanupTracker = () => {
			disposed = true;
			if (rafId) window.cancelAnimationFrame(rafId);
			observer.disconnect();
			window.removeEventListener('resize', handleViewportChange);
			window.removeEventListener('scroll', handleViewportChange, true);
		};
	}

	$: if (state.open && state.mode === 'step') {
		startTracking(state.track, state.step);
	} else {
		stopTracking();
		targetRect = null;
	}

	onDestroy(() => {
		stopTracking();
	});
</script>

{#if state.open}
	<div
		bind:this={tourRootEl}
		class="onboarding-tour"
		data-testid="onboarding-tour"
		role="dialog"
		aria-modal="true"
		aria-label={lang && t('tour_dialog_label')}
	>
		{#if state.mode === 'welcome'}
			<div class="tour-backdrop-dim"></div>
			<div class="tour-welcome-card">
				<div class="tour-welcome-header">
					<span class="tour-kicker font-mono">COBALT V2 · WORKSTATION</span>
					<button
						type="button"
						class="secondary tour-icon-close"
						aria-label={lang && t('tour_skip_btn')}
						on:click={() => completeOrDismissTour(false)}
					>
						<X size={15} strokeWidth={2} />
					</button>
				</div>
				<div class="tour-welcome-title-row">
					<span class="tour-brand-badge"><Sparkles size={18} strokeWidth={1.8} /></span>
					<h2>{lang && t('tour_welcome_title')}</h2>
				</div>
				<p class="tour-welcome-lead">{lang && t('tour_welcome_subtitle')}</p>

				<div class="tour-track-list">
					<button type="button" class="tour-track-cta primary-track" on:click={() => launchTour(1)}>
						<div class="tour-track-meta">
							<span class="tour-track-pill font-mono">{lang && t('tour_track1_duration')}</span>
							<strong>{lang && t('tour_welcome_start_t1')}</strong>
							<span class="tour-track-sub">{lang && t('tour_track1_desc')}</span>
						</div>
					</button>
					<button type="button" class="secondary tour-track-cta" on:click={() => launchTour(2)}>
						<div class="tour-track-meta">
							<span class="tour-track-pill font-mono">{lang && t('tour_track2_duration')}</span>
							<strong>{lang && t('tour_welcome_start_t2')}</strong>
							<span class="tour-track-sub">{lang && t('tour_track2_desc')}</span>
						</div>
					</button>
					<button type="button" class="secondary tour-track-cta" on:click={() => launchTour(3)}>
						<div class="tour-track-meta">
							<span class="tour-track-pill font-mono">{lang && t('tour_track3_duration')}</span>
							<strong>{lang && t('tour_welcome_start_t3')}</strong>
							<span class="tour-track-sub">{lang && t('tour_track3_desc')}</span>
						</div>
					</button>
				</div>

				<div class="tour-welcome-footer">
					<span class="tour-esc-hint font-mono">{lang && t('tour_esc_hint')}</span>
					<button type="button" class="secondary" on:click={() => completeOrDismissTour(false)}>
						{lang && t('tour_skip_later')}
					</button>
				</div>
			</div>
		{:else}
			{#if targetRect}
				<div
					class="tour-spotlight-ring"
					style="top: {targetRect.top - RING_PAD}px; left: {targetRect.left - RING_PAD}px; width: {targetRect.width + RING_PAD * 2}px; height: {targetRect.height + RING_PAD * 2}px;"
					aria-hidden="true"
				></div>
			{:else}
				<div class="tour-backdrop-dim" aria-hidden="true"></div>
			{/if}

			<div
				bind:this={popoverEl}
				class="tour-popover"
				class:is-centered={popoverCentered}
				style="left: {popoverLeft}px; top: {popoverTop}px;"
			>
				<div class="tour-popover-header">
					<div class="tour-track-indicator">
						<Compass size={14} strokeWidth={1.8} />
						<span class="tour-kicker font-mono">{lang && t(activeTrack.labelKey)}</span>
						<span class="tour-step-counter font-mono">
							{lang && t('tour_step_counter', { current: state.step + 1, total: activeSteps.length })}
						</span>
					</div>
					<button
						type="button"
						class="secondary tour-icon-close"
						aria-label={lang && t('tour_skip_btn')}
						on:click={() => completeOrDismissTour(false)}
					>
						<X size={14} strokeWidth={2} />
					</button>
				</div>

				<div class="tour-track-switcher">
					<button
						type="button"
						class="secondary tour-mini-pill"
						class:is-active={state.track === 1}
						on:click={() => launchTour(1)}
					>
						{lang && t('tour_pill_t1')}
					</button>
					<button
						type="button"
						class="secondary tour-mini-pill"
						class:is-active={state.track === 2}
						on:click={() => launchTour(2)}
					>
						{lang && t('tour_pill_t2')}
					</button>
					<button
						type="button"
						class="secondary tour-mini-pill"
						class:is-active={state.track === 3}
						on:click={() => launchTour(3)}
					>
						{lang && t('tour_pill_t3')}
					</button>
				</div>

				<h3 class="tour-step-title">{lang && t(activeStep.titleKey)}</h3>
				<p class="tour-step-body">{lang && t(activeStep.bodyKey)}</p>
				<div class="tour-step-tip">{lang && t(activeStep.tipKey)}</div>

				<div class="tour-popover-footer">
					{#if !isLastStep}
						<button type="button" class="secondary" on:click={() => completeOrDismissTour(false)}>
							{lang && t('tour_skip_btn')}
						</button>
					{:else}
						<span></span>
					{/if}
					<div class="tour-nav-group">
						<button
							type="button"
							class="secondary"
							disabled={state.step === 0}
							on:click={prevTourStep}
						>
							{lang && t('tour_back_btn')}
						</button>
						{#if isLastStep}
							<button type="button" on:click={() => completeOrDismissTour(true)}>
								{lang && t('tour_finish_btn')}
							</button>
						{:else}
							<button type="button" on:click={nextTourStep}>
								{lang && t('tour_next_btn')}
							</button>
						{/if}
					</div>
				</div>
			</div>
		{/if}
	</div>
{/if}

<style>
	.onboarding-tour {
		position: fixed;
		inset: 0;
		z-index: 300;
		pointer-events: none;
		font-family: var(--font-sans);
		color: var(--text-primary, #e6edf3);
	}

	.tour-backdrop-dim {
		position: fixed;
		inset: 0;
		background: rgba(5, 7, 11, 0.78);
		backdrop-filter: blur(2px);
		pointer-events: auto;
	}

	.tour-spotlight-ring {
		position: fixed;
		box-sizing: border-box;
		border-radius: 10px;
		border: 2px solid var(--cobalt-cyan, #38bdf8);
		box-shadow:
			0 0 0 9999px rgba(5, 7, 11, 0.78),
			0 0 24px rgba(56, 189, 248, 0.35);
		pointer-events: none;
		transition: top 0.16s ease, left 0.16s ease, width 0.16s ease, height 0.16s ease;
	}

	.tour-welcome-card {
		position: fixed;
		top: 50%;
		left: 50%;
		transform: translate(-50%, -50%);
		width: min(500px, calc(100vw - 32px));
		max-height: calc(100vh - 32px);
		overflow-y: auto;
		pointer-events: auto;
		background: var(--bg-card, #0f1726);
		border: 1px solid var(--cobalt-brand, #263c8a);
		border-radius: 14px;
		padding: 18px 20px;
		box-shadow: 0 24px 64px rgba(0, 0, 0, 0.65);
		display: grid;
		gap: 12px;
	}

	.tour-welcome-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
	}

	.tour-kicker {
		font-size: 11px;
		font-weight: 700;
		letter-spacing: 0.09em;
		text-transform: uppercase;
		color: var(--cobalt-orange, #faa031);
	}

	.tour-welcome-title-row {
		display: flex;
		align-items: center;
		gap: 10px;
	}

	.tour-brand-badge {
		width: 32px;
		height: 32px;
		border-radius: 8px;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		background: linear-gradient(135deg, var(--cobalt-brand, #263c8a), var(--cobalt-electric, #1d63ed));
		color: #ffffff;
		flex-shrink: 0;
	}

	.tour-welcome-title-row h2 {
		margin: 0;
		font-size: 19px;
		font-weight: 700;
		letter-spacing: -0.01em;
	}

	.tour-welcome-lead {
		margin: 0;
		font-size: 13px;
		line-height: 1.5;
		color: var(--text-secondary, #94a3b8);
	}

	.tour-track-list {
		display: grid;
		gap: 8px;
	}

	.tour-track-cta {
		width: 100%;
		text-align: left;
		padding: 11px 13px;
		border-radius: 10px;
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.tour-track-cta.primary-track {
		background: linear-gradient(135deg, var(--cobalt-brand, #263c8a), var(--cobalt-electric, #1d63ed));
		border: 1px solid var(--cobalt-cyan, #38bdf8);
		color: #ffffff;
	}

	.tour-track-meta {
		display: grid;
		gap: 3px;
	}

	.tour-track-pill {
		font-size: 10px;
		font-weight: 700;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		color: var(--cobalt-orange-light, #fdcd92);
	}

	.tour-track-meta strong {
		font-size: 14px;
		font-weight: 700;
	}

	.tour-track-sub {
		font-size: 12px;
		font-weight: 400;
		line-height: 1.4;
		color: rgba(230, 237, 243, 0.85);
	}

	.tour-welcome-footer {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
		padding-top: 6px;
		border-top: 1px solid var(--border-subtle, #1f2937);
	}

	.tour-esc-hint {
		font-size: 11px;
		color: var(--text-muted, #64748b);
	}

	.tour-popover {
		position: fixed;
		width: min(390px, calc(100vw - 32px));
		pointer-events: auto;
		background: var(--bg-card, #0f1726);
		border: 1px solid var(--cobalt-cyan, #38bdf8);
		border-radius: 12px;
		padding: 14px 16px;
		box-shadow: 0 20px 50px rgba(0, 0, 0, 0.7);
		display: grid;
		gap: 10px;
		z-index: 301;
	}

	.tour-popover-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
	}

	.tour-track-indicator {
		display: inline-flex;
		align-items: center;
		gap: 7px;
		color: var(--cobalt-cyan, #38bdf8);
	}

	.tour-step-counter {
		font-size: 11px;
		font-weight: 700;
		padding: 2px 7px;
		border-radius: 999px;
		background: var(--bg-void, #05070b);
		border: 1px solid var(--border-highlight, #2f3b52);
		color: var(--text-primary, #e6edf3);
	}

	.tour-icon-close {
		padding: 4px;
		display: inline-flex;
		align-items: center;
		justify-content: center;
	}

	.tour-track-switcher {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		gap: 5px;
	}

	.tour-mini-pill {
		padding: 4px 6px;
		font-size: 11px;
		border-radius: 6px;
		text-align: center;
	}

	.tour-mini-pill.is-active {
		background: var(--cobalt-brand, #263c8a);
		border-color: var(--cobalt-cyan, #38bdf8);
		color: #ffffff;
	}

	.tour-step-title {
		margin: 0;
		font-size: 16px;
		font-weight: 700;
		color: var(--text-primary, #e6edf3);
	}

	.tour-step-body {
		margin: 0;
		font-size: 13px;
		line-height: 1.5;
		color: var(--text-secondary, #94a3b8);
	}

	.tour-step-tip {
		padding: 7px 9px;
		border-radius: 8px;
		background: rgba(56, 189, 248, 0.08);
		border: 1px solid rgba(56, 189, 248, 0.25);
		font-size: 12px;
		line-height: 1.4;
		color: var(--cobalt-link, #8ab4f8);
	}

	.tour-popover-footer {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		padding-top: 6px;
		border-top: 1px solid var(--border-subtle, #1f2937);
	}

	.tour-nav-group {
		display: inline-flex;
		align-items: center;
		gap: 6px;
	}
</style>
