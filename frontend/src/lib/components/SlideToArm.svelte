<script lang="ts">
	import { createEventDispatcher, onDestroy } from 'svelte';

	export let label = 'Slide to arm';
	export let disabled = false;

	const ARM_THRESHOLD = 0.78;
	const MIN_DRAG_DISTANCE_PX = 16;

	const dispatch = createEventDispatcher<{ arm: void }>();

	let trackEl: HTMLDivElement | null = null;
	let dragging = false;
	let progress = 0;
	let startClientX = 0;
	let maxDragDistance = 0;

	function computeProgress(clientX: number): number {
		if (!trackEl) return 0;
		const rect = trackEl.getBoundingClientRect();
		if (rect.width <= 0) return 0;
		return Math.min(1, Math.max(0, (clientX - rect.left) / rect.width));
	}

	function detachListeners(): void {
		if (typeof window === 'undefined') return;
		window.removeEventListener('pointermove', handlePointerMove);
		window.removeEventListener('pointerup', handlePointerUp);
		window.removeEventListener('pointercancel', handlePointerCancel);
		window.removeEventListener('blur', handlePointerCancel);
	}

	function handlePointerDown(event: PointerEvent): void {
		if (disabled) return;
		if (event.pointerType === 'mouse' && event.button !== 0) return;
		event.preventDefault();
		dragging = true;
		startClientX = event.clientX;
		maxDragDistance = 0;
		progress = Math.min(0.25, computeProgress(event.clientX));
		if (typeof window !== 'undefined') {
			window.addEventListener('pointermove', handlePointerMove);
			window.addEventListener('pointerup', handlePointerUp);
			window.addEventListener('pointercancel', handlePointerCancel);
			window.addEventListener('blur', handlePointerCancel);
		}
	}

	function handlePointerMove(event: PointerEvent): void {
		if (!dragging) return;
		maxDragDistance = Math.max(maxDragDistance, Math.abs(event.clientX - startClientX));
		progress = computeProgress(event.clientX);
	}

	function handlePointerCancel(): void {
		if (!dragging) return;
		dragging = false;
		progress = 0;
		maxDragDistance = 0;
		detachListeners();
	}

	function handlePointerUp(event: PointerEvent): void {
		if (!dragging) return;
		maxDragDistance = Math.max(maxDragDistance, Math.abs(event.clientX - startClientX));
		const finalProgress = computeProgress(event.clientX);
		dragging = false;
		detachListeners();

		if (!disabled && finalProgress >= ARM_THRESHOLD && maxDragDistance >= MIN_DRAG_DISTANCE_PX) {
			progress = 1;
			dispatch('arm');
			progress = 0;
		} else {
			progress = 0;
		}
	}

	onDestroy(() => {
		detachListeners();
	});
</script>

<div
	bind:this={trackEl}
	class="slide-to-arm-track"
	class:is-dragging={dragging}
	class:is-ready={progress >= ARM_THRESHOLD}
	class:is-disabled={disabled}
	data-testid="slide-to-arm"
	role="slider"
	tabindex={disabled ? -1 : 0}
	aria-label={label}
	aria-valuemin={0}
	aria-valuemax={100}
	aria-valuenow={Math.round(progress * 100)}
	aria-disabled={disabled}
	on:pointerdown={handlePointerDown}
>
	<div
		class="slide-to-arm-fill"
		class:is-animating={!dragging}
		style="width: {Math.round(progress * 100)}%;"
	></div>
	<span class="slide-to-arm-label">{label}</span>
	<span
		class="slide-to-arm-thumb"
		class:is-animating={!dragging}
		style="left: calc(3px + {progress} * (100% - 28px));"
		aria-hidden="true"
	>
		»
	</span>
</div>
