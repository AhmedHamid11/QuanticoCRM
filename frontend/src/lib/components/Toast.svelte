<script lang="ts">
	import { getToasts } from '$lib/stores/toast.svelte';

	let toasts = $derived(getToasts());
</script>

{#if toasts.length > 0}
	<div class="fixed bottom-4 right-4 z-50 flex flex-col gap-2">
		{#each toasts as toast (toast.id)}
			<div
				class="rounded-lg px-4 py-3 shadow-lg transition-all duration-300"
				class:bg-green-500={toast.type === 'success'}
				class:bg-red-500={toast.type === 'error'}
				class:bg-blue-600={toast.type === 'info'}
			>
				<p class="text-white text-sm font-medium">{toast.message}</p>
			</div>
		{/each}
	</div>
{/if}
