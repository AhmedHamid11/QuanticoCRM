<script lang="ts">
	import { onMount } from 'svelte';
	import { get, del, post, put } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import { TableSkeleton, ErrorDisplay } from '$lib/components/ui';
	import type { Contact, ContactListResponse } from '$lib/types/contact';

	// List view types
	interface ListView {
		id: string;
		orgId: string;
		entityName: string;
		name: string;
		filterQuery: string;
		columns: string;
		sortBy: string;
		sortDir: string;
		isDefault: boolean;
		isSystem: boolean;
		createdById?: string;
		createdAt: string;
		modifiedAt: string;
	}

	let contacts = $state<Contact[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let search = $state('');
	let page = $state(1);
	let pageSize = $state(20);
	let total = $state(0);
	let totalPages = $state(0);
	let sortBy = $state('created_at');
	let sortDir = $state<'asc' | 'desc'>('desc');
	let searchTimeout: ReturnType<typeof setTimeout> | null = null;
	let knownTotal = $state<number | null>(null);

	// Filter and List View state
	let filterQuery = $state('');
	let filterError = $state<string | null>(null);
	let showFilterInput = $state(false);
	let listViews = $state<ListView[]>([]);
	let selectedListView = $state<ListView | null>(null);
	let showSaveModal = $state(false);
	let showManageModal = $state(false);
	let newViewName = $state('');
	let saveAsDefault = $state(false);
	let savingView = $state(false);
	let saveMode = $state<'new' | 'update'>('new');
	let viewToUpdate = $state<ListView | null>(null);

	async function loadListViews() {
		try {
			listViews = await get<ListView[]>(`/list-views/Contact`);
			const defaultView = listViews.find(v => v.isDefault);
			if (defaultView && !selectedListView) {
				selectListView(defaultView);
			}
		} catch (e) {
			listViews = [];
		}
	}

	function selectListView(view: ListView | null) {
		selectedListView = view;
		if (view) {
			filterQuery = view.filterQuery || '';
			sortBy = view.sortBy || 'created_at';
			sortDir = (view.sortDir as 'asc' | 'desc') || 'desc';
		} else {
			filterQuery = '';
			sortBy = 'created_at';
			sortDir = 'desc';
		}
		page = 1;
		knownTotal = null;
		loadContacts();
	}

	async function loadContacts() {
		try {
			loading = true;
			error = null;
			filterError = null;
			const params = new URLSearchParams({
				page: page.toString(),
				pageSize: pageSize.toString(),
				sortBy,
				sortDir
			});
			if (search) {
				params.set('search', search);
			}
			if (filterQuery.trim()) {
				params.set('filter', filterQuery.trim());
			}
			if (knownTotal !== null && page > 1) {
				params.set('knownTotal', knownTotal.toString());
			}
			const result = await get<ContactListResponse>(`/contacts?${params}`);
			contacts = result.data;
			total = result.total;
			totalPages = result.totalPages;
			// Cache total for subsequent page navigations
			knownTotal = result.total;
		} catch (e) {
			const errorMsg = e instanceof Error ? e.message : 'Failed to load contacts';
			if (errorMsg.toLowerCase().includes('filter') || errorMsg.toLowerCase().includes('invalid')) {
				filterError = errorMsg;
				error = null;
			} else {
				error = errorMsg;
				toast.error(errorMsg);
			}
		} finally {
			loading = false;
		}
	}

	async function deleteContact(id: string) {
		const backup = [...contacts];
		contacts = contacts.filter((c) => c.id !== id);
		total = total - 1;

		try {
			await del(`/contacts/${id}`);
			toast.success('Contact deleted');
		} catch (e) {
			contacts = backup;
			total = total + 1;
			const message = e instanceof Error ? e.message : 'Failed to delete contact';
			toast.error(message);
		}
	}

	function handleSearch() {
		page = 1;
		knownTotal = null;
		loadContacts();
	}

	function handleSearchInput() {
		if (searchTimeout) {
			clearTimeout(searchTimeout);
		}
		searchTimeout = setTimeout(() => {
			page = 1;
			knownTotal = null;
			loadContacts();
		}, 300);
	}

	function handleFilterInput() {
		if (searchTimeout) {
			clearTimeout(searchTimeout);
		}
		searchTimeout = setTimeout(() => {
			page = 1;
			knownTotal = null;
			if (selectedListView && filterQuery !== selectedListView.filterQuery) {
				selectedListView = null;
			}
			loadContacts();
		}, 500);
	}

	function handleSort(column: string) {
		if (sortBy === column) {
			sortDir = sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			sortBy = column;
			sortDir = 'asc';
		}
		if (selectedListView) {
			selectedListView = null;
		}
		knownTotal = null;
		loadContacts();
	}

	function clearFilter() {
		filterQuery = '';
		filterError = null;
		selectedListView = null;
		page = 1;
		knownTotal = null;
		loadContacts();
	}

	async function saveListView() {
		savingView = true;
		try {
			if (saveMode === 'update' && viewToUpdate) {
				// Update existing view
				const updatedView = await put<ListView>(`/list-views/Contact/${viewToUpdate.id}`, {
					name: viewToUpdate.name,
					filterQuery: filterQuery.trim(),
					columns: '[]',
					sortBy,
					sortDir,
					isDefault: viewToUpdate.isDefault
				});
				listViews = listViews.map(v => v.id === updatedView.id ? updatedView : v);
				selectedListView = updatedView;
				toast.success('List view updated');
			} else {
				// Create new view
				if (!newViewName.trim()) return;
				const newView = await post<ListView>(`/list-views/Contact`, {
					name: newViewName.trim(),
					filterQuery: filterQuery.trim(),
					columns: '[]',
					sortBy,
					sortDir,
					isDefault: saveAsDefault
				});
				listViews = [...listViews, newView];
				selectedListView = newView;

				if (saveAsDefault) {
					await loadListViews();
				}
				toast.success('List view saved');
			}
			showSaveModal = false;
			newViewName = '';
			saveAsDefault = false;
			saveMode = 'new';
			viewToUpdate = null;
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to save list view');
		} finally {
			savingView = false;
		}
	}

	async function deleteListView(view: ListView) {
		if (view.isSystem) {
			toast.error('Cannot delete system views');
			return;
		}

		const backup = [...listViews];
		listViews = listViews.filter(v => v.id !== view.id);
		if (selectedListView?.id === view.id) {
			selectedListView = null;
		}

		try {
			await del(`/list-views/Contact/${view.id}`);
			toast.success('List view deleted');
		} catch (e) {
			listViews = backup;
			toast.error(e instanceof Error ? e.message : 'Failed to delete list view');
		}
	}

	async function setDefaultView(view: ListView) {
		try {
			await put<ListView>(`/list-views/Contact/${view.id}`, {
				name: view.name,
				filterQuery: view.filterQuery,
				columns: view.columns,
				sortBy: view.sortBy,
				sortDir: view.sortDir,
				isDefault: true
			});
			await loadListViews();
			toast.success('Default view updated');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to set default view');
		}
	}

	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleDateString();
	}

	function getFullName(contact: Contact): string {
		const parts = [contact.salutationName, contact.firstName, contact.lastName].filter(Boolean);
		return parts.join(' ');
	}

	onMount(async () => {
		await loadListViews();
		loadContacts();
	});
</script>

<div class="space-y-4">
	<!-- Header -->
	<div class="flex justify-between items-center">
		<h1 class="text-2xl font-bold text-gray-900">Contacts</h1>
		<a
			href="/contacts/new"
			class="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-blue-600 hover:bg-blue-600/90"
		>
			+ New Contact
		</a>
	</div>

	<!-- Search and Filter Bar -->
	<div class="flex flex-wrap gap-3 items-start">
		<!-- Search -->
		<div class="flex-1 min-w-[200px] relative">
			<input
				type="text"
				bind:value={search}
				oninput={handleSearchInput}
				placeholder="Search contacts..."
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
			{#if search}
				<button
					onclick={() => { search = ''; handleSearch(); }}
					class="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600"
				>
					<svg class="h-5 w-5" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
						<path d="M6.28 5.22a.75.75 0 00-1.06 1.06L8.94 10l-3.72 3.72a.75.75 0 101.06 1.06L10 11.06l3.72 3.72a.75.75 0 101.06-1.06L11.06 10l3.72-3.72a.75.75 0 00-1.06-1.06L10 8.94 6.28 5.22z" />
					</svg>
				</button>
			{/if}
		</div>

		<!-- List View Selector -->
		<div class="relative">
			<select
				class="pl-3 pr-8 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500 bg-white text-sm"
				onchange={(e) => {
					const target = e.target as HTMLSelectElement;
					const viewId = target.value;
					if (viewId === '') {
						selectListView(null);
					} else {
						const view = listViews.find(v => v.id === viewId);
						if (view) selectListView(view);
					}
				}}
			>
				<option value="" selected={!selectedListView}>All Records</option>
				{#each listViews as view}
					<option value={view.id} selected={selectedListView?.id === view.id}>
						{view.name} {view.isDefault ? '(Default)' : ''}
					</option>
				{/each}
			</select>
		</div>

		<!-- Filter Toggle Button -->
		<button
			onclick={() => showFilterInput = !showFilterInput}
			class="px-3 py-2 border rounded-md text-sm flex items-center gap-2 {showFilterInput || filterQuery ? 'border-blue-500 bg-blue-50 text-blue-700' : 'border-gray-300 text-gray-700 hover:bg-gray-50'}"
		>
			<svg class="h-4 w-4" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
				<path fill-rule="evenodd" d="M2.628 1.601C5.028 1.206 7.49 1 10 1s4.973.206 7.372.601a.75.75 0 01.628.74v2.288a2.25 2.25 0 01-.659 1.59l-4.682 4.683a2.25 2.25 0 00-.659 1.59v3.037c0 .684-.31 1.33-.844 1.757l-1.937 1.55A.75.75 0 018 18.25v-5.757a2.25 2.25 0 00-.659-1.591L2.659 6.22A2.25 2.25 0 012 4.629V2.34a.75.75 0 01.628-.74z" clip-rule="evenodd" />
			</svg>
			Filter
			{#if filterQuery}
				<span class="bg-blue-600 text-white text-xs px-1.5 py-0.5 rounded-full">1</span>
			{/if}
		</button>

		<!-- Save View Button -->
		<button
			onclick={() => showSaveModal = true}
			class="px-3 py-2 border border-gray-300 rounded-md text-sm text-gray-700 hover:bg-gray-50 flex items-center gap-2"
		>
			<svg class="h-4 w-4" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
				<path d="M10.75 4.75a.75.75 0 00-1.5 0v4.5h-4.5a.75.75 0 000 1.5h4.5v4.5a.75.75 0 001.5 0v-4.5h4.5a.75.75 0 000-1.5h-4.5v-4.5z" />
			</svg>
			Save View
		</button>

		<!-- Manage Views Button -->
		{#if listViews.length > 0}
			<button
				onclick={() => showManageModal = true}
				class="px-3 py-2 border border-gray-300 rounded-md text-sm text-gray-700 hover:bg-gray-50"
			>
				Manage Views
			</button>
		{/if}
	</div>

	<!-- Filter Input (Expandable) -->
	{#if showFilterInput}
		<div class="bg-gray-50 border border-gray-200 rounded-lg p-4 space-y-3">
			<div class="flex items-center justify-between">
				<label class="text-sm font-medium text-gray-700">Filter Query</label>
				<span class="text-xs text-gray-500">SQL-style syntax</span>
			</div>
			<div class="relative">
				<input
					type="text"
					bind:value={filterQuery}
					oninput={handleFilterInput}
					placeholder="e.g., lastName = 'Smith' AND emailAddress LIKE '%@company.com'"
					class="w-full px-3 py-2 border rounded-md font-mono text-sm {filterError ? 'border-red-500 focus:ring-red-500 focus:border-red-500' : 'border-gray-300 focus:ring-blue-500 focus:border-blue-500'}"
				/>
				{#if filterQuery}
					<button
						onclick={clearFilter}
						class="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600"
					>
						<svg class="h-5 w-5" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
							<path d="M6.28 5.22a.75.75 0 00-1.06 1.06L8.94 10l-3.72 3.72a.75.75 0 101.06 1.06L10 11.06l3.72 3.72a.75.75 0 101.06-1.06L11.06 10l3.72-3.72a.75.75 0 00-1.06-1.06L10 8.94 6.28 5.22z" />
						</svg>
					</button>
				{/if}
			</div>
			{#if filterError}
				<p class="text-sm text-red-600">{filterError}</p>
			{/if}
			<div class="text-xs text-gray-500 space-y-1">
				<p><strong>Operators:</strong> =, !=, &lt;, &gt;, &lt;=, &gt;=, LIKE, IN, NOT IN, IS NULL, IS NOT NULL</p>
				<p><strong>Examples:</strong> <code class="bg-gray-200 px-1 rounded">lastName LIKE '%son'</code> | <code class="bg-gray-200 px-1 rounded">emailAddress IS NOT NULL</code></p>
			</div>
		</div>
	{/if}

	<!-- Active Filter Display -->
	{#if filterQuery && !showFilterInput}
		<div class="flex items-center gap-2 text-sm">
			<span class="text-gray-500">Filter:</span>
			<code class="bg-gray-100 px-2 py-1 rounded text-gray-700 font-mono text-xs">{filterQuery}</code>
			<button onclick={clearFilter} class="text-gray-400 hover:text-red-500">
				<svg class="h-4 w-4" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
					<path d="M6.28 5.22a.75.75 0 00-1.06 1.06L8.94 10l-3.72 3.72a.75.75 0 101.06 1.06L10 11.06l3.72 3.72a.75.75 0 101.06-1.06L11.06 10l3.72-3.72a.75.75 0 00-1.06-1.06L10 8.94 6.28 5.22z" />
				</svg>
			</button>
		</div>
	{/if}

	<!-- Table -->
	{#if loading}
		<TableSkeleton rows={pageSize} columns={5} />
	{:else if error}
		<ErrorDisplay message={error} onRetry={loadContacts} />
	{:else if contacts.length === 0}
		<div class="text-center py-12 text-gray-500">
			{#if filterQuery}
				No contacts match your filter.
				<button onclick={clearFilter} class="text-blue-600 hover:underline">Clear filter</button>
			{:else}
				No contacts found. <a href="/contacts/new" class="text-blue-600 hover:underline">Create one</a>
			{/if}
		</div>
	{:else}
		<div class="bg-white shadow rounded-lg overflow-hidden">
			<table class="min-w-full divide-y divide-gray-200">
				<thead class="bg-gray-50">
					<tr>
						<th
							class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
							onclick={() => handleSort('first_name')}
						>
							Name
							{#if sortBy === 'first_name'}
								<span class="ml-1">{sortDir === 'asc' ? '↑' : '↓'}</span>
							{/if}
						</th>
						<th
							class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
							onclick={() => handleSort('email_address')}
						>
							Email
							{#if sortBy === 'email_address'}
								<span class="ml-1">{sortDir === 'asc' ? '↑' : '↓'}</span>
							{/if}
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Phone
						</th>
						<th
							class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
							onclick={() => handleSort('modified_at')}
						>
							Last Modified
							{#if sortBy === 'modified_at'}
								<span class="ml-1">{sortDir === 'asc' ? '↑' : '↓'}</span>
							{/if}
						</th>
						<th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
							Actions
						</th>
					</tr>
				</thead>
				<tbody class="bg-white divide-y divide-gray-200">
					{#each contacts as contact (contact.id)}
						<tr class="hover:bg-gray-50">
							<td class="px-6 py-4 whitespace-nowrap">
								<a href="/contacts/{contact.id}" class="text-blue-600 hover:underline font-medium">
									{getFullName(contact)}
								</a>
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{contact.emailAddress || '-'}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{contact.phoneNumber || '-'}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								<div>{formatDate(contact.modifiedAt)}</div>
								{#if contact.modifiedByName}
									<div class="text-xs text-gray-400">{contact.modifiedByName}</div>
								{/if}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-right text-sm">
								<a
									href="/contacts/{contact.id}/edit"
									class="text-blue-600 hover:underline mr-4"
								>
									Edit
								</a>
								<button
									onclick={() => deleteContact(contact.id)}
									class="text-red-600 hover:underline"
								>
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
						onclick={() => { page = page - 1; loadContacts(); }}
						disabled={page === 1}
						class="px-3 py-1 border border-gray-300 rounded-md text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-50"
					>
						Previous
					</button>
					<span class="px-3 py-1 text-sm text-gray-700">
						Page {page} of {totalPages}
					</span>
					<button
						onclick={() => { page = page + 1; loadContacts(); }}
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

<!-- Save View Modal -->
{#if showSaveModal}
	<div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onclick={() => showSaveModal = false}>
		<div class="bg-white rounded-lg shadow-xl p-6 w-full max-w-md" onclick={(e) => e.stopPropagation()}>
			<h2 class="text-lg font-semibold mb-4">Save List View</h2>
			<div class="space-y-4">
				<!-- Save Mode Selection -->
				{#if listViews.filter(v => !v.isSystem).length > 0}
					<div class="flex gap-4 border-b pb-3">
						<label class="flex items-center gap-2 cursor-pointer">
							<input
								type="radio"
								name="saveMode"
								value="new"
								bind:group={saveMode}
								class="h-4 w-4 text-blue-600"
							/>
							<span class="text-sm text-gray-700">Create new view</span>
						</label>
						<label class="flex items-center gap-2 cursor-pointer">
							<input
								type="radio"
								name="saveMode"
								value="update"
								bind:group={saveMode}
								class="h-4 w-4 text-blue-600"
							/>
							<span class="text-sm text-gray-700">Update existing</span>
						</label>
					</div>
				{/if}

				{#if saveMode === 'new'}
					<div>
						<label class="block text-sm font-medium text-gray-700 mb-1">View Name</label>
						<input
							type="text"
							bind:value={newViewName}
							placeholder="e.g., VIP Contacts, Recent Leads"
							class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
						/>
					</div>
					<div class="flex items-center gap-2">
						<input
							type="checkbox"
							id="saveAsDefaultContact"
							bind:checked={saveAsDefault}
							class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded"
						/>
						<label for="saveAsDefaultContact" class="text-sm text-gray-700">Set as default view</label>
					</div>
				{:else}
					<div>
						<label class="block text-sm font-medium text-gray-700 mb-1">Select View to Update</label>
						<select
							bind:value={viewToUpdate}
							class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
						>
							<option value={null}>-- Select a view --</option>
							{#each listViews.filter(v => !v.isSystem) as view}
								<option value={view}>{view.name}</option>
							{/each}
						</select>
					</div>
				{/if}

				<div>
					<label class="block text-sm font-medium text-gray-700 mb-1">Current Filter</label>
					<code class="block bg-gray-100 px-3 py-2 rounded text-sm font-mono text-gray-700">
						{filterQuery || '(no filter)'}
					</code>
				</div>
				<div class="text-xs text-gray-500">
					Sort: {sortBy} ({sortDir})
				</div>
			</div>
			<div class="flex justify-end gap-3 mt-6">
				<button
					onclick={() => { showSaveModal = false; saveMode = 'new'; viewToUpdate = null; }}
					class="px-4 py-2 border border-gray-300 rounded-md text-sm text-gray-700 hover:bg-gray-50"
				>
					Cancel
				</button>
				<button
					onclick={saveListView}
					disabled={(saveMode === 'new' && !newViewName.trim()) || (saveMode === 'update' && !viewToUpdate) || savingView}
					class="px-4 py-2 bg-blue-600 text-white rounded-md text-sm hover:bg-blue-600/90 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					{savingView ? 'Saving...' : (saveMode === 'update' ? 'Update View' : 'Save View')}
				</button>
			</div>
		</div>
	</div>
{/if}

<!-- Manage Views Modal -->
{#if showManageModal}
	<div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onclick={() => showManageModal = false}>
		<div class="bg-white rounded-lg shadow-xl p-6 w-full max-w-lg" onclick={(e) => e.stopPropagation()}>
			<h2 class="text-lg font-semibold mb-4">Manage List Views</h2>
			<div class="space-y-2 max-h-96 overflow-y-auto">
				{#each listViews as view}
					<div class="flex items-center justify-between p-3 bg-gray-50 rounded-lg">
						<div>
							<div class="font-medium text-gray-900">
								{view.name}
								{#if view.isDefault}
									<span class="ml-2 text-xs bg-blue-100 text-blue-800 px-2 py-0.5 rounded">Default</span>
								{/if}
							</div>
							{#if view.filterQuery}
								<code class="text-xs text-gray-500 font-mono">{view.filterQuery}</code>
							{:else}
								<span class="text-xs text-gray-400">No filter</span>
							{/if}
						</div>
						<div class="flex items-center gap-2">
							{#if !view.isDefault && !view.isSystem}
								<button
									onclick={() => setDefaultView(view)}
									class="text-xs text-blue-600 hover:underline"
								>
									Set Default
								</button>
							{/if}
							{#if !view.isSystem}
								<button
									onclick={() => deleteListView(view)}
									class="text-xs text-red-600 hover:underline"
								>
									Delete
								</button>
							{/if}
						</div>
					</div>
				{/each}
			</div>
			<div class="flex justify-end mt-6">
				<button
					onclick={() => showManageModal = false}
					class="px-4 py-2 bg-gray-100 text-gray-700 rounded-md text-sm hover:bg-gray-200"
				>
					Close
				</button>
			</div>
		</div>
	</div>
{/if}
