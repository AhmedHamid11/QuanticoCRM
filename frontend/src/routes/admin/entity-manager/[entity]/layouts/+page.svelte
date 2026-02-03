<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { get } from '$lib/utils/api';
	import type { EntityDef, FieldDef } from '$lib/types/admin';

	let entityName = $derived($page.params.entity);
	let entity = $state<EntityDef | null>(null);
	let fields = $state<FieldDef[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	const layoutTypes = [
		{ id: 'list', name: 'List', description: 'Columns shown in the list/table view', icon: 'M4 6h16M4 10h16M4 14h16M4 18h16' },
		{ id: 'detail', name: 'Detail', description: 'Fields shown on the record detail page', icon: 'M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z' },
		{ id: 'detailSmall', name: 'Quick View', description: 'Compact view shown in modals and popovers', icon: 'M15 12a3 3 0 11-6 0 3 3 0 016 0z M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z' },
		{ id: 'filters', name: 'Filters', description: 'Fields available in the search/filter panel', icon: 'M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z' },
		{ id: 'massUpdate', name: 'Mass Update', description: 'Fields available for bulk editing', icon: 'M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15' },
	];

	async function loadData() {
		try {
			loading = true;
			const [entityData, fieldsData] = await Promise.all([
				get<EntityDef>(`/admin/entities/${entityName}`),
				get<FieldDef[]>(`/admin/entities/${entityName}/fields`)
			]);
			entity = entityData;
			fields = fieldsData;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load data';
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		loadData();
	});
</script>

<div class="space-y-6">
	<div>
		<nav class="text-sm text-gray-500 mb-2">
			<a href="/admin" class="hover:text-gray-700">Administration</a>
			<span class="mx-2">/</span>
			<a href="/admin/entity-manager" class="hover:text-gray-700">Entity Manager</a>
			<span class="mx-2">/</span>
			<a href="/admin/entity-manager/{entityName}" class="hover:text-gray-700">{entity?.label || entityName}</a>
			<span class="mx-2">/</span>
			<span class="text-gray-900">Layouts</span>
		</nav>
		<h1 class="text-2xl font-bold text-gray-900">{entity?.label || entityName} - Layouts</h1>
	</div>

	{#if loading}
		<div class="text-center py-12 text-gray-500">Loading...</div>
	{:else if error}
		<div class="text-center py-12 text-red-500">{error}</div>
	{:else}
		<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
			{#each layoutTypes as layout}
				<a
					href="/admin/entity-manager/{entityName}/layouts/{layout.id}"
					class="bg-white shadow rounded-lg p-6 hover:shadow-md transition-shadow"
				>
					<div class="flex items-start">
						<div class="flex-shrink-0">
							<svg class="h-8 w-8 text-purple-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={layout.icon} />
							</svg>
						</div>
						<div class="ml-4">
							<h3 class="text-lg font-medium text-gray-900">{layout.name}</h3>
							<p class="mt-1 text-sm text-gray-500">{layout.description}</p>
						</div>
					</div>
				</a>
			{/each}

			<!-- Related Lists - special layout type -->
			<a
				href="/admin/entity-manager/{entityName}/related-lists"
				class="bg-white shadow rounded-lg p-6 hover:shadow-md transition-shadow border-l-4 border-orange-500"
			>
				<div class="flex items-start">
					<div class="flex-shrink-0">
						<svg class="h-8 w-8 text-orange-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
						</svg>
					</div>
					<div class="ml-4">
						<h3 class="text-lg font-medium text-gray-900">Related Lists</h3>
						<p class="mt-1 text-sm text-gray-500">Configure which related records appear on the detail page</p>
					</div>
				</div>
			</a>
		</div>

		<!-- Available Fields Reference -->
		<div class="bg-white shadow rounded-lg p-6">
			<h2 class="text-lg font-medium text-gray-900 mb-4">Available Fields</h2>
			<p class="text-sm text-gray-500 mb-4">
				These fields can be added to layouts. Go to <a href="/admin/entity-manager/{entityName}/fields" class="text-primary hover:underline">Field Manager</a> to add more fields.
			</p>
			<div class="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-3">
				{#each fields as field (field.id)}
					<div class="px-3 py-2 bg-gray-50 rounded text-sm border border-gray-200">
						<div class="font-medium text-gray-900">{field.label}</div>
						<div class="text-xs text-gray-500">{field.type}</div>
					</div>
				{/each}
			</div>
		</div>
	{/if}
</div>
