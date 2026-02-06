<script lang="ts">
	import { getBannerClass, type PendingAlert } from '$lib/api/dedup';

	interface Props {
		alert: PendingAlert;
		onViewMatches: () => void;
		onDismiss: () => void;
	}

	let { alert, onViewMatches, onDismiss }: Props = $props();

	function getConfidenceLabel(tier: 'high' | 'medium' | 'low'): string {
		switch (tier) {
			case 'high': return 'High confidence';
			case 'medium': return 'Medium confidence';
			case 'low': return 'Low confidence';
			default: return '';
		}
	}
</script>

<div class="rounded-lg border p-4 mb-4 {getBannerClass(alert.highestConfidence)}">
	<div class="flex items-center justify-between flex-wrap gap-2">
		<div class="flex items-center gap-3">
			<!-- Warning icon -->
			<svg class="w-5 h-5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
				      d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
			</svg>
			<div>
				<span class="font-medium">
					{alert.totalMatchCount} potential duplicate{alert.totalMatchCount !== 1 ? 's' : ''} found
				</span>
				<span class="text-sm opacity-75 ml-2">
					({getConfidenceLabel(alert.highestConfidence)})
				</span>
				{#if alert.isBlockMode}
					<span class="text-xs ml-2 px-2 py-0.5 bg-red-200 text-red-800 rounded">
						Block Mode
					</span>
				{/if}
			</div>
		</div>
		<div class="flex items-center gap-2">
			<button
				onclick={onViewMatches}
				class="text-sm font-medium underline hover:no-underline focus:outline-none focus:ring-2 focus:ring-offset-2 rounded px-1"
			>
				View Matches
			</button>
			<button
				onclick={onDismiss}
				class="p-1 rounded hover:bg-black/10 focus:outline-none focus:ring-2 focus:ring-offset-2"
				aria-label="Dismiss alert"
				title="Dismiss"
			>
				<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
				</svg>
			</button>
		</div>
	</div>
</div>
