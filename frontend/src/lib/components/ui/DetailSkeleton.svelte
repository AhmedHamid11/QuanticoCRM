<script lang="ts">
	import Skeleton from './Skeleton.svelte';

	interface Props {
		sections?: number;
		fieldsPerSection?: number;
		showAvatar?: boolean;
		class?: string;
	}

	let {
		sections = 2,
		fieldsPerSection = 4,
		showAvatar = true,
		class: className = ''
	}: Props = $props();
</script>

<div class="space-y-6 {className}">
	<!-- Header skeleton -->
	<div class="flex items-center justify-between">
		<div class="flex items-center gap-4">
			{#if showAvatar}
				<Skeleton variant="avatar" width="4rem" height="4rem" />
			{/if}
			<div class="space-y-2">
				<Skeleton variant="heading" width="12rem" />
				<Skeleton variant="text" width="8rem" height="0.875rem" />
			</div>
		</div>
		<div class="flex gap-2">
			<Skeleton variant="button" width="4rem" />
			<Skeleton variant="button" width="4rem" />
		</div>
	</div>

	<!-- Tabs skeleton -->
	<div class="border-b border-gray-200 pb-2">
		<div class="flex gap-6">
			<Skeleton variant="text" width="4rem" height="1rem" />
			<Skeleton variant="text" width="5rem" height="1rem" />
		</div>
	</div>

	<!-- Sections skeleton -->
	{#each Array(sections) as _, sectionIndex (sectionIndex)}
		<div class="bg-white shadow rounded-lg p-6">
			<Skeleton variant="heading" width="30%" class="mb-4" />
			<div class="grid grid-cols-2 gap-4">
				{#each Array(fieldsPerSection) as _, fieldIndex (fieldIndex)}
					<div>
						<Skeleton variant="text" width="40%" height="0.75rem" class="mb-1" />
						<Skeleton variant="text" width="70%" height="1rem" />
					</div>
				{/each}
			</div>
		</div>
	{/each}

	<!-- Record info skeleton -->
	<div class="bg-white shadow rounded-lg p-6">
		<Skeleton variant="heading" width="40%" class="mb-4" />
		<div class="grid grid-cols-2 gap-4">
			<div>
				<Skeleton variant="text" width="30%" height="0.75rem" class="mb-1" />
				<Skeleton variant="text" width="60%" />
			</div>
			<div>
				<Skeleton variant="text" width="30%" height="0.75rem" class="mb-1" />
				<Skeleton variant="text" width="60%" />
			</div>
		</div>
	</div>
</div>
