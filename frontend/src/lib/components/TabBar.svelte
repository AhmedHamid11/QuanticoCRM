<script lang="ts">
	import type { LayoutTabV3 } from '$lib/types/layout';

	interface Props {
		tabs: LayoutTabV3[];
		activeTabId: string;
		onTabChange: (tabId: string) => void;
	}

	let { tabs, activeTabId, onTabChange }: Props = $props();
</script>

{#if tabs.length > 1}
	<div class="border-b border-gray-200">
		<nav class="-mb-px flex space-x-6 overflow-x-auto">
			{#each tabs as tab (tab.id)}
				<button
					type="button"
					class="flex-shrink-0 whitespace-nowrap border-b-2 px-1 py-4 text-sm font-medium {activeTabId === tab.id
						? 'border-blue-500 text-blue-600'
						: 'border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700'}"
					onclick={() => onTabChange(tab.id)}
				>
					{tab.label}
				</button>
			{/each}
		</nav>
	</div>
{/if}
