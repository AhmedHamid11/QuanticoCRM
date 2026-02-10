<script lang="ts">
	import { navigating } from '$app/stores';

	let progress = $state(0);
	let visible = $state(false);
	let timeout: ReturnType<typeof setTimeout> | null = null;
	let interval: ReturnType<typeof setInterval> | null = null;

	function startProgress() {
		// Clear any existing timers
		if (timeout) clearTimeout(timeout);
		if (interval) clearInterval(interval);

		progress = 0;
		visible = true;

		// Simulate progress - fast at first, then slow down
		interval = setInterval(() => {
			if (progress < 90) {
				// Slow down as we approach 90%
				const increment = (90 - progress) / 10;
				progress = Math.min(90, progress + increment);
			}
		}, 100);
	}

	function completeProgress() {
		if (interval) {
			clearInterval(interval);
			interval = null;
		}

		progress = 100;

		// Hide after animation completes
		timeout = setTimeout(() => {
			visible = false;
			progress = 0;
		}, 200);
	}

	// React to navigation state
	$effect(() => {
		if ($navigating) {
			startProgress();
		} else if (visible) {
			completeProgress();
		}
	});

	// Cleanup on unmount
	$effect(() => {
		return () => {
			if (timeout) clearTimeout(timeout);
			if (interval) clearInterval(interval);
		};
	});
</script>

{#if visible}
	<div class="fixed top-0 left-0 right-0 z-50 h-1 bg-gray-200/50">
		<div
			class="h-full bg-blue-600 transition-all duration-200 ease-out"
			style="width: {progress}%"
		></div>
	</div>
{/if}
