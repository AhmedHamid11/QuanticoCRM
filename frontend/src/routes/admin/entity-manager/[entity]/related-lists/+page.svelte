<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';

	let entityName = $derived($page.params.entity);

	onMount(() => {
		goto(`/admin/entity-manager/${entityName}/layouts/detail-v3`, { replaceState: true });
	});
</script>

<div class="flex items-center justify-center py-12 text-gray-500">
	Redirecting to Layout Editor...
</div>
