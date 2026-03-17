<script lang="ts">
	import Skeleton from './Skeleton.svelte';

	interface Props {
		rows?: number;
		columns?: number;
		showHeader?: boolean;
		class?: string;
	}

	let { rows = 5, columns = 5, showHeader = true, class: className = '' }: Props = $props();
</script>

<div class="bg-white shadow rounded-lg overflow-hidden {className}">
	<table class="min-w-full divide-y divide-gray-200">
		{#if showHeader}
			<thead class="bg-gray-50">
				<tr>
					{#each Array(columns) as _, i (i)}
						<th class="px-6 py-3 text-left">
							<Skeleton variant="text" width={i === 0 ? '60%' : '50%'} height="0.75rem" />
						</th>
					{/each}
				</tr>
			</thead>
		{/if}
		<tbody class="divide-y divide-gray-200">
			{#each Array(rows) as _, rowIndex (rowIndex)}
				<tr>
					{#each Array(columns) as _, colIndex (colIndex)}
						<td class="px-6 py-4 whitespace-nowrap">
							{#if colIndex === 0}
								<!-- First column often has link-styled text -->
								<Skeleton variant="text" width="80%" />
							{:else if colIndex === columns - 1}
								<!-- Last column often has actions -->
								<div class="flex justify-end gap-2">
									<Skeleton variant="text" width="3rem" />
									<Skeleton variant="text" width="3rem" />
								</div>
							{:else}
								<Skeleton variant="text" width={`${50 + Math.random() * 30}%`} />
							{/if}
						</td>
					{/each}
				</tr>
			{/each}
		</tbody>
	</table>
</div>
