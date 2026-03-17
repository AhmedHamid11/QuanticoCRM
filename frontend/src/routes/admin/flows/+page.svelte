<script lang="ts">
	import { onMount } from 'svelte';
	import { get, del } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import type { FlowListItem, FlowListResponse } from '$lib/types/flow';
	import type { EntityDef } from '$lib/types/admin';

	let flows = $state<FlowListItem[]>([]);
	let entities = $state<EntityDef[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let search = $state('');
	let entityFilter = $state('');
	let page = $state(1);
	let pageSize = $state(20);
	let total = $state(0);
	let totalPages = $state(0);
	let sortBy = $state('modified_at');
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

	async function loadFlows() {
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
			const result = await get<FlowListResponse>(`/flows?${params}`);
			flows = result.data || [];
			total = result.total;
			totalPages = result.totalPages;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load flows';
			toast.error(error);
		} finally {
			loading = false;
		}
	}

	async function toggleFlow(flow: FlowListItem) {
		const backup = flow.isActive;
		flow.isActive = !flow.isActive;
		flows = [...flows];

		try {
			await import('$lib/utils/api').then(m =>
				m.put(`/flows/${flow.id}`, { isActive: flow.isActive })
			);
			toast.success(flow.isActive ? 'Flow activated' : 'Flow deactivated');
		} catch (e) {
			flow.isActive = backup;
			flows = [...flows];
			const message = e instanceof Error ? e.message : 'Failed to toggle flow';
			toast.error(message);
		}
	}

	async function deleteFlow(id: string) {
		if (!confirm('Are you sure you want to delete this flow? This cannot be undone.')) return;

		const backup = [...flows];
		flows = flows.filter((f) => f.id !== id);
		total = total - 1;

		try {
			await del(`/flows/${id}`);
			toast.success('Flow deleted');
		} catch (e) {
			flows = backup;
			total = total + 1;
			const message = e instanceof Error ? e.message : 'Failed to delete flow';
			toast.error(message);
		}
	}

	function handleSearchInput() {
		if (searchTimeout) {
			clearTimeout(searchTimeout);
		}
		searchTimeout = setTimeout(() => {
			page = 1;
			loadFlows();
		}, 300);
	}

	function handleFilterChange() {
		page = 1;
		loadFlows();
	}

	function handleSort(column: string) {
		if (sortBy === column) {
			sortDir = sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			sortBy = column;
			sortDir = 'asc';
		}
		loadFlows();
	}

	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleDateString();
	}

	onMount(() => {
		loadEntities();
		loadFlows();
	});
</script>

<div class="space-y-4">
	<!-- Header -->
	<div class="flex justify-between items-center">
		<div>
			<h1 class="text-2xl font-bold text-gray-900">Screen Flows</h1>
			<p class="text-sm text-gray-500 mt-1">Interactive step-by-step wizards for user-guided processes</p>
		</div>
		<a
			href="/admin/flows/new"
			class="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-blue-600 hover:bg-blue-600/90"
		>
			+ New Flow
		</a>
	</div>

	<!-- Search and Filters -->
	<div class="flex gap-4 flex-wrap">
		<div class="flex-1 min-w-64 relative">
			<input
				type="text"
				bind:value={search}
				oninput={handleSearchInput}
				placeholder="Search flows..."
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
	{:else if flows.length === 0}
		<div class="text-center py-12 text-gray-500">
			No flows found. <a href="/admin/flows/new" class="text-blue-600 hover:underline">Create one</a>
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
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Description
						</th>
						<th
							class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
							onclick={() => handleSort('version')}
						>
							Version
							{#if sortBy === 'version'}
								<span class="ml-1">{sortDir === 'asc' ? '↑' : '↓'}</span>
							{/if}
						</th>
						<th
							class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
							onclick={() => handleSort('modified_at')}
						>
							Modified
							{#if sortBy === 'modified_at'}
								<span class="ml-1">{sortDir === 'asc' ? '↑' : '↓'}</span>
							{/if}
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Status
						</th>
						<th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
							Actions
						</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-200">
					{#each flows as flow (flow.id)}
						<tr class="hover:bg-gray-50">
							<td class="px-6 py-4 whitespace-nowrap">
								<a href="/admin/flows/{flow.id}" class="text-blue-600 hover:underline font-medium">
									{flow.name}
								</a>
							</td>
							<td class="px-6 py-4 text-sm text-gray-500 max-w-xs truncate">
								{flow.description || '-'}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								v{flow.version}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{formatDate(flow.modifiedAt)}
							</td>
							<td class="px-6 py-4 whitespace-nowrap">
								<button
									onclick={() => toggleFlow(flow)}
									class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 {flow.isActive ? 'bg-blue-600' : 'bg-gray-200'}"
								>
									<span
										class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out {flow.isActive ? 'translate-x-5' : 'translate-x-0'}"
									></span>
								</button>
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-right text-sm">
								<a href="/admin/flows/{flow.id}" class="text-blue-600 hover:underline mr-4">
									Edit
								</a>
								<button onclick={() => deleteFlow(flow.id)} class="text-red-600 hover:underline">
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
						onclick={() => { page = page - 1; loadFlows(); }}
						disabled={page === 1}
						class="px-3 py-1 border border-gray-300 rounded-md text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-50"
					>
						Previous
					</button>
					<span class="px-3 py-1 text-sm text-gray-700">
						Page {page} of {totalPages}
					</span>
					<button
						onclick={() => { page = page + 1; loadFlows(); }}
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
