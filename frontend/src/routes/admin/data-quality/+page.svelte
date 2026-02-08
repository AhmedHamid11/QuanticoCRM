<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	onMount(() => {
		goto('/admin/data-quality/duplicate-rules', { replaceState: true });
	});
</script>

<div class="flex items-center justify-center py-12">
	<div class="text-gray-400">Redirecting...</div>
</div>
