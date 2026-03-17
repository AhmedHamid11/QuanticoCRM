<script lang="ts">
	import { onMount } from 'svelte';
	import { get, post, del } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import type { Tripwire, TripwireListResponse } from '$lib/types/tripwire';
	import type { EntityDef } from '$lib/types/admin';

	let tripwires = $state<Tripwire[]>([]);
	let entities = $state<EntityDef[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let search = $state('');
	let entityFilter = $state('');
	let page = $state(1);
	let pageSize = $state(20);
	let total = $state(0);
	let totalPages = $state(0);
	let sortBy = $state('created_at');
	let sortDir = $state<'asc' | 'desc'>('desc');
	let searchTimeout: ReturnType<typeof setTimeout> | null = null;

	async function loadEntities() {
		try {
			const result = await get<EntityDef[]>('/admin/entities');
			entities = result;
		} catch (e) {
			console.error('Failed to load entities:', e);
		}
	}

	async function loadTripwires() {
		try {
			loading = true;
			error = null;
			const params = new URLSearchParams({
				page: page.toString(),
				pageSize: pageSize.toString(),
				sortBy,
				sortDir
			});
			if (search) {
				params.set('search', search);
			}
			if (entityFilter) {
				params.set('entityType', entityFilter);
			}
			const result = await get<TripwireListResponse>(`/tripwires?${params}`);
			tripwires = result.data;
			total = result.total;
			totalPages = result.totalPages;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load tripwires';
			toast.error(error);
		} finally {
			loading = false;
		}
	}

	async function toggleTripwire(tw: Tripwire) {
		const backup = tw.enabled;
		tw.enabled = !tw.enabled;
		tripwires = [...tripwires];

		try {
			await post(`/tripwires/${tw.id}/toggle`, {});
			toast.success(tw.enabled ? 'Tripwire enabled' : 'Tripwire disabled');
		} catch (e) {
			tw.enabled = backup;
			tripwires = [...tripwires];
			const message = e instanceof Error ? e.message : 'Failed to toggle tripwire';
			toast.error(message);
		}
	}

	async function deleteTripwire(id: string) {
		if (!confirm('Are you sure you want to delete this tripwire?')) return;

		const backup = [...tripwires];
		tripwires = tripwires.filter((t) => t.id !== id);
		total = total - 1;

		try {
			await del(`/tripwires/${id}`);
			toast.success('Tripwire deleted');
		} catch (e) {
			tripwires = backup;
			total = total + 1;
			const message = e instanceof Error ? e.message : 'Failed to delete tripwire';
			toast.error(message);
		}
	}

	function handleSearchInput() {
		if (searchTimeout) {
			clearTimeout(searchTimeout);
		}
		searchTimeout = setTimeout(() => {
			page = 1;
			loadTripwires();
		}, 300);
	}

	function handleFilterChange() {
		page = 1;
		loadTripwires();
	}

	function handleSort(column: string) {
		if (sortBy === column) {
			sortDir = sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			sortBy = column;
			sortDir = 'asc';
		}
		loadTripwires();
	}

	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleDateString();
	}

	function truncateUrl(url: string, maxLen: number = 40): string {
		if (url.length <= maxLen) return url;
		return url.substring(0, maxLen) + '...';
	}

	function getConditionsSummary(tw: Tripwire): string {
		if (!tw.conditions || tw.conditions.length === 0) return 'No conditions';
		const count = tw.conditions.length;
		return `${count} condition${count > 1 ? 's' : ''} (${tw.conditionLogic})`;
	}

	onMount(() => {
		loadEntities();
		loadTripwires();
	});
</script>

<div class="space-y-4">
	<!-- Header -->
	<div class="flex justify-between items-center">
		<div>
			<h1 class="text-2xl font-bold text-gray-900">Tripwires</h1>
			<p class="text-sm text-gray-500 mt-1">Webhook triggers that fire when entity records are created, updated, or deleted</p>
		</div>
		<div class="flex gap-2">
			<a
				href="/admin/settings/webhooks"
				class="inline-flex items-center px-4 py-2 border border-gray-300 text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50"
			>
				Webhook Settings
			</a>
			<a
				href="/admin/tripwires/new"
				class="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-blue-600 hover:bg-blue-600/90"
			>
				+ New Tripwire
			</a>
		</div>
	</div>

	<!-- Search and Filters -->
	<div class="flex gap-4 flex-wrap">
		<div class="flex-1 min-w-64 relative">
			<input
				type="text"
				bind:value={search}
				oninput={handleSearchInput}
				placeholder="Search tripwires..."
				class="w-full pl-10 pr-4 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
			/>
			<svg
				class="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-400"
				xmlns="http://www.w3.org/2000/svg"
				viewBox="0 0 20 20"
				fill="currentColor"
			>
				<path
					fill-rule="evenodd"
					d="M9 3.5a5.5 5.5 0 100 11 5.5 5.5 0 000-11zM2 9a7 7 0 1112.452 4.391l3.328 3.329a.75.75 0 11-1.06 1.06l-3.329-3.328A7 7 0 012 9z"
					clip-rule="evenodd"
				/>
			</svg>
		</div>
		<select
			bind:value={entityFilter}
			onchange={handleFilterChange}
			class="px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
		>
			<option value="">All Entities</option>
			{#each entities as entity}
				<option value={entity.name}>{entity.label}</option>
			{/each}
		</select>
	</div>

	<!-- Table -->
	{#if loading}
		<div class="text-center py-12 text-gray-500">Loading...</div>
	{:else if error}
		<div class="text-center py-12 text-red-500">{error}</div>
	{:else if tripwires.length === 0}
		<div class="text-center py-12 text-gray-500">
			No tripwires found. <a href="/admin/tripwires/new" class="text-blue-600 hover:underline">Create one</a>
		</div>
	{:else}
		<div class="crm-card overflow-hidden">
			<table class="min-w-full divide-y divide-gray-200">
				<thead class="bg-gray-50">
					<tr>
						<th
							class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
							onclick={() => handleSort('name')}
						>
							Name
							{#if sortBy === 'name'}
								<span class="ml-1">{sortDir === 'asc' ? '↑' : '↓'}</span>
							{/if}
						</th>
						<th
							class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
							onclick={() => handleSort('entity_type')}
						>
							Entity
							{#if sortBy === 'entity_type'}
								<span class="ml-1">{sortDir === 'asc' ? '↑' : '↓'}</span>
							{/if}
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Endpoint
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Conditions
						</th>
						<th
							class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
							onclick={() => handleSort('enabled')}
						>
							Status
							{#if sortBy === 'enabled'}
								<span class="ml-1">{sortDir === 'asc' ? '↑' : '↓'}</span>
							{/if}
						</th>
						<th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
							Actions
						</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-200">
					{#each tripwires as tw (tw.id)}
						<tr class="hover:bg-gray-50">
							<td class="px-6 py-4 whitespace-nowrap">
								<a href="/admin/tripwires/{tw.id}" class="text-blue-600 hover:underline font-medium">
									{tw.name}
								</a>
								{#if tw.description}
									<p class="text-xs text-gray-500 mt-0.5">{tw.description}</p>
								{/if}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{tw.entityType}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500" title={tw.endpointUrl}>
								{truncateUrl(tw.endpointUrl)}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{getConditionsSummary(tw)}
							</td>
							<td class="px-6 py-4 whitespace-nowrap">
								<button
									onclick={() => toggleTripwire(tw)}
									class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 {tw.enabled ? 'bg-blue-600' : 'bg-gray-200'}"
								>
									<span
										class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out {tw.enabled ? 'translate-x-5' : 'translate-x-0'}"
									></span>
								</button>
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-right text-sm">
								<a href="/admin/tripwires/{tw.id}/logs" class="text-gray-600 hover:underline mr-4">
									Logs
								</a>
								<a href="/admin/tripwires/{tw.id}" class="text-blue-600 hover:underline mr-4">
									Edit
								</a>
								<button onclick={() => deleteTripwire(tw.id)} class="text-red-600 hover:underline">
									Delete
								</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<!-- Pagination -->
		{#if totalPages > 1}
			<div class="flex justify-between items-center">
				<p class="text-sm text-gray-700">
					Showing {(page - 1) * pageSize + 1} to {Math.min(page * pageSize, total)} of {total} results
				</p>
				<div class="flex gap-2">
					<button
						onclick={() => { page = page - 1; loadTripwires(); }}
						disabled={page === 1}
						class="px-3 py-1 border border-gray-300 rounded-md text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-50"
					>
						Previous
					</button>
					<span class="px-3 py-1 text-sm text-gray-700">
						Page {page} of {totalPages}
					</span>
					<button
						onclick={() => { page = page + 1; loadTripwires(); }}
						disabled={page === totalPages}
						class="px-3 py-1 border border-gray-300 rounded-md text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-50"
					>
						Next
					</button>
				</div>
			</div>
		{/if}
	{/if}
</div>
