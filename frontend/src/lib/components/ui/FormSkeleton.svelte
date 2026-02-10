<script lang="ts">
	import Skeleton from './Skeleton.svelte';

	interface Props {
		fields?: number;
		showHeader?: boolean;
		class?: string;
	}

	let { fields = 6, showHeader = true, class: className = '' }: Props = $props();
</script>

<div class="max-w-2xl mx-auto {className}">
	{#if showHeader}
		<div class="flex items-center justify-between mb-6">
			<Skeleton variant="heading" width="10rem" height="1.75rem" />
			<Skeleton variant="text" width="8rem" height="0.875rem" />
		</div>
	{/if}

	<div class="bg-white shadow rounded-lg p-6 space-y-4">
		{#each Array(fields) as _, i (i)}
			<div>
				<!-- Label -->
				<Skeleton variant="text" width="30%" height="0.875rem" class="mb-1.5" />
				<!-- Input -->
				<Skeleton variant="input" />
			</div>
		{/each}

		<!-- Action buttons -->
		<div class="flex justify-end gap-3 pt-4 border-t border-gray-200">
			<Skeleton variant="button" width="5rem" />
			<Skeleton variant="button" width="7rem" />
		</div>
	</div>
</div>
