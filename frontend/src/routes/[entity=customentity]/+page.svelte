<script lang="ts">
	import { page } from '$app/stores';
	import { get, del, post, put, isAbortError } from '$lib/utils/api';
	import { getEntityNameFromPath } from '$lib/stores/navigation.svelte';
	import { TableSkeleton, ErrorDisplay } from '$lib/components/ui';
	import type { EntityDef, FieldDef } from '$lib/types/admin';
	import type { LayoutV2Response } from '$lib/types/layout';
	import { getAllFieldNames } from '$lib/types/layout';

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

	// Get entity name from URL via navigation lookup or fallback to PascalCase conversion
	let entitySlug = $derived($page.params.entity);
	let entityName = $derived(getEntityNameFromPath(entitySlug) || toPascalCase(entitySlug));

	// Convert slug to PascalCase as fallback (remove trailing 's' for plural)
	function toPascalCase(slug: string): string {
		let singular = slug;
		if (slug.endsWith('s') && slug.length > 1) {
			singular = slug.slice(0, -1);
		}
		return singular.charAt(0).toUpperCase() + singular.slice(1);
	}

	let entityDef = $state<EntityDef | null>(null);
	let fields = $state<FieldDef[]>([]);
	let layoutFields = $state<string[]>([]);
	let layoutExists = $state(false);
	let records = $state<Record<string, unknown>[]>([]);
	let loading = $state(true);
	let metadataLoading = $state(true); // Track metadata (fields, layout) loading separately
	let error = $state<string | null>(null);
	let search = $state('');
	let currentPage = $state(1);
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

	// AbortController to cancel in-flight requests on navigation
	let abortController: AbortController | null = null;

	// List columns to display - use layout if configured, otherwise fallback to auto-generated
	let displayFields = $derived.by(() => {
		// If layout exists (has been explicitly saved), use those fields
		if (layoutExists) {
			return layoutFields
				.map(name => fields.find(f => f.name === name))
				.filter((f): f is FieldDef => f !== undefined);
		}
		// Fallback (only when no layout has been configured): first 5 fields excluding id and timestamps
		return fields
			.filter(f => f.name !== 'id' && !f.name.startsWith('created_') && !f.name.startsWith('modified_'))
			.slice(0, 5);
	});

	async function loadEntityDef(signal?: AbortSignal) {
		try {
			// Use public endpoint (doesn't require admin role)
			entityDef = await get<EntityDef>(`/entities/${entityName}/def`, signal);
		} catch (e) {
			// Ignore abort errors; entity might not exist in metadata yet, that's ok
			if (!isAbortError(e)) {
				entityDef = null;
			}
		}
	}

	async function loadFields(signal?: AbortSignal) {
		try {
			// Use public endpoint (doesn't require admin role)
			fields = await get<FieldDef[]>(`/entities/${entityName}/fields`, signal);
		} catch (e) {
			if (!isAbortError(e)) {
				fields = [];
			}
		}
	}

	async function loadLayout(signal?: AbortSignal) {
		// Use public endpoint (doesn't require admin role)
		try {
			const response = await get<{ layoutData: string; exists: boolean }>(`/entities/${entityName}/layouts/list`, signal);
			const parsed = JSON.parse(response.layoutData || '[]');

			// Handle both V1 (simple array) and V2 (object with sections) formats
			if (Array.isArray(parsed)) {
				// V1 format: simple array of field names
				layoutFields = parsed;
			} else if (parsed && typeof parsed === 'object' && 'sections' in parsed) {
				// V2 format: extract field names from sections
				layoutFields = getAllFieldNames(parsed);
			} else {
				layoutFields = [];
			}
			layoutExists = response.exists ?? false;
		} catch (e) {
			if (!isAbortError(e)) {
				layoutFields = [];
				layoutExists = false;
			}
		}
	}

	async function loadListViews(signal?: AbortSignal) {
		try {
			listViews = await get<ListView[]>(`/list-views/${entityName}`, signal);
			// Auto-select default view if one exists
			const defaultView = listViews.find(v => v.isDefault);
			if (defaultView && !selectedListView) {
				selectListView(defaultView);
			}
		} catch (e) {
			if (!isAbortError(e)) {
				listViews = [];
			}
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
		currentPage = 1;
		knownTotal = null;
		// Use existing abort controller signal if available
		loadRecords(abortController?.signal);
	}

	async function loadRecords(signal?: AbortSignal) {
		try {
			loading = true;
			error = null;
			filterError = null;
			const params = new URLSearchParams({
				page: currentPage.toString(),
				pageSize: pageSize.toString(),
				sortBy,
				sortDir,
				includeRollups: 'false'
			});
			if (search) {
				params.set('search', search);
			}
			if (filterQuery.trim()) {
				params.set('filter', filterQuery.trim());
			}
			if (knownTotal !== null && currentPage > 1) {
				params.set('knownTotal', knownTotal.toString());
			}

			const result = await get<{
				data: Record<string, unknown>[];
				total: number;
				totalPages: number;
			}>(`/entities/${entityName}/records?${params}`, signal);

			records = result.data;
			total = result.total;
			totalPages = result.totalPages;
			knownTotal = result.total;
		} catch (e) {
			// Ignore abort errors (user navigated away)
			if (isAbortError(e)) {
				return;
			}
			const errorMsg = e instanceof Error ? e.message : 'Failed to load records';
			// Check if this is a filter error
			if (errorMsg.toLowerCase().includes('filter') || errorMsg.toLowerCase().includes('invalid')) {
				filterError = errorMsg;
				error = null;
			} else {
				error = errorMsg;
			}
		} finally {
			loading = false;
		}
	}

	async function deleteRecord(id: string) {
		const backup = [...records];
		records = records.filter(r => r.id !== id);
		total = total - 1;

		try {
			await del(`/entities/${entityName}/records/${id}`);
		} catch (e) {
			records = backup;
			total = total + 1;
			error = e instanceof Error ? e.message : 'Failed to delete record';
		}
	}

	function handleSearchInput() {
		if (searchTimeout) {
			clearTimeout(searchTimeout);
		}
		searchTimeout = setTimeout(() => {
			currentPage = 1;
			knownTotal = null;
			loadRecords();
		}, 300);
	}

	function handleFilterInput() {
		if (searchTimeout) {
			clearTimeout(searchTimeout);
		}
		searchTimeout = setTimeout(() => {
			currentPage = 1;
			knownTotal = null;
			// Deselect list view when filter is manually changed
			if (selectedListView && filterQuery !== selectedListView.filterQuery) {
				selectedListView = null;
			}
			loadRecords();
		}, 500);
	}

	function handleSort(column: string) {
		if (sortBy === column) {
			sortDir = sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			sortBy = column;
			sortDir = 'asc';
		}
		// Deselect list view when sort is manually changed
		if (selectedListView) {
			selectedListView = null;
		}
		knownTotal = null;
		loadRecords();
	}

	// Get the display value for a field from a record
	function getFieldValue(record: Record<string, unknown>, field: FieldDef): unknown {
		// For link fields, the display name is stored in {fieldName}Name
		if (field.type === 'link') {
			return record[`${field.name}Name`] ?? record[field.name];
		}
		// For multi-link fields, names are in {fieldName}Names
		if (field.type === 'linkMultiple') {
			const names = record[`${field.name}Names`];
			if (names) {
				try {
					const parsed = typeof names === 'string' ? JSON.parse(names) : names;
					if (Array.isArray(parsed)) {
						return parsed.join(', ');
					}
				} catch {
					// Not valid JSON
				}
			}
			return record[field.name];
		}
		return record[field.name];
	}

	function formatValue(value: unknown, field: FieldDef): string {
		if (value === null || value === undefined || value === '') return '-';

		if (field.type === 'date' || field.type === 'datetime') {
			return new Date(String(value)).toLocaleDateString();
		}
		if (field.type === 'bool') {
			return value ? 'Yes' : 'No';
		}
		if (field.type === 'currency') {
			return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(Number(value));
		}
		if (field.type === 'enum' || field.type === 'multiEnum') {
			const strValue = String(value);
			// Check if it's JSON array format
			if (strValue.startsWith('[')) {
				try {
					const parsed = JSON.parse(strValue);
					if (Array.isArray(parsed)) {
						return parsed.join(', ');
					}
				} catch {
					// Not valid JSON, return as-is
				}
			}
			return strValue;
		}

		return String(value);
	}

	function clearSearch() {
		search = '';
		currentPage = 1;
		knownTotal = null;
		loadRecords();
	}

	function clearFilter() {
		filterQuery = '';
		filterError = null;
		selectedListView = null;
		currentPage = 1;
		knownTotal = null;
		loadRecords();
	}

	async function saveListView() {
		if (!newViewName.trim()) return;

		savingView = true;
		try {
			const newView = await post<ListView>(`/list-views/${entityName}`, {
				name: newViewName.trim(),
				filterQuery: filterQuery.trim(),
				columns: JSON.stringify(layoutFields),
				sortBy,
				sortDir,
				isDefault: saveAsDefault
			});
			listViews = [...listViews, newView];
			selectedListView = newView;
			showSaveModal = false;
			newViewName = '';
			saveAsDefault = false;

			// If set as default, reload to update the list
			if (saveAsDefault) {
				await loadListViews();
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save list view';
		} finally {
			savingView = false;
		}
	}

	async function updateListView(view: ListView) {
		try {
			await put<ListView>(`/list-views/${entityName}/${view.id}`, {
				name: view.name,
				filterQuery: filterQuery.trim(),
				columns: view.columns,
				sortBy,
				sortDir,
				isDefault: view.isDefault
			});

			// Update local state
			listViews = listViews.map(v => v.id === view.id ? { ...v, filterQuery: filterQuery.trim(), sortBy, sortDir } : v);
			selectedListView = { ...view, filterQuery: filterQuery.trim(), sortBy, sortDir };
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to update list view';
		}
	}

	async function deleteListView(view: ListView) {
		if (view.isSystem) {
			error = 'Cannot delete system views';
			return;
		}

		const backup = [...listViews];
		listViews = listViews.filter(v => v.id !== view.id);
		if (selectedListView?.id === view.id) {
			selectedListView = null;
		}

		try {
			await del(`/list-views/${entityName}/${view.id}`);
		} catch (e) {
			listViews = backup;
			error = e instanceof Error ? e.message : 'Failed to delete list view';
		}
	}

	async function setDefaultView(view: ListView) {
		try {
			await put<ListView>(`/list-views/${entityName}/${view.id}`, {
				name: view.name,
				filterQuery: view.filterQuery,
				columns: view.columns,
				sortBy: view.sortBy,
				sortDir: view.sortDir,
				isDefault: true
			});
			await loadListViews();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to set default view';
		}
	}

	// Reload data when entity changes (handles navigation between list pages)
	$effect(() => {
		// Track entityName to trigger reload on navigation
		const _entity = entityName;

		// Abort any in-flight requests from the previous entity
		if (abortController) {
			abortController.abort();
		}
		abortController = new AbortController();
		const signal = abortController.signal;

		// Reset state
		entityDef = null;
		fields = [];
		layoutFields = [];
		layoutExists = false;
		records = [];
		loading = true;
		metadataLoading = true;
		error = null;
		search = '';
		filterQuery = '';
		filterError = null;
		currentPage = 1;
		knownTotal = null;
		sortBy = 'created_at';
		sortDir = 'desc';
		listViews = [];
		selectedListView = null;
		showFilterInput = false;

		// Load data
		(async () => {
			await Promise.all([
				loadEntityDef(signal),
				loadFields(signal),
				loadLayout(signal),
				loadListViews(signal)
			]);
			metadataLoading = false;
			await loadRecords(signal);
		})();

		// Cleanup function - abort requests when effect re-runs or component unmounts
		return () => {
			if (abortController) {
				abortController.abort();
			}
		};
	});
</script>

<div class="space-y-4">
	<!-- Header -->
	<div class="flex justify-between items-center">
		<div class="flex items-center gap-3">
			{#if entityDef}
				<div
					class="w-10 h-10 rounded flex items-center justify-center text-white text-lg font-semibold"
					style="background-color: {entityDef.color}"
				>
					{entityDef.label.charAt(0)}
				</div>
			{/if}
			<h1 class="text-2xl font-bold text-gray-900">
				{entityDef?.labelPlural || entityName + 's'}
			</h1>
		</div>
		<a
			href="/{entitySlug}/new"
			class="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-black bg-primary hover:bg-primary/90"
		>
			+ New {entityDef?.label || entityName}
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
				placeholder="Search..."
				class="w-full pl-10 pr-4 py-2 border border-gray-300 rounded-md focus:ring-primary focus:border-primary"
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
					onclick={clearSearch}
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
				class="pl-3 pr-8 py-2 border border-gray-300 rounded-md focus:ring-primary focus:border-primary bg-white text-sm"
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
			class="px-3 py-2 border rounded-md text-sm flex items-center gap-2 {showFilterInput || filterQuery ? 'border-primary bg-blue-50 text-blue-700' : 'border-gray-300 text-gray-700 hover:bg-gray-50'}"
		>
			<svg class="h-4 w-4" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
				<path fill-rule="evenodd" d="M2.628 1.601C5.028 1.206 7.49 1 10 1s4.973.206 7.372.601a.75.75 0 01.628.74v2.288a2.25 2.25 0 01-.659 1.59l-4.682 4.683a2.25 2.25 0 00-.659 1.59v3.037c0 .684-.31 1.33-.844 1.757l-1.937 1.55A.75.75 0 018 18.25v-5.757a2.25 2.25 0 00-.659-1.591L2.659 6.22A2.25 2.25 0 012 4.629V2.34a.75.75 0 01.628-.74z" clip-rule="evenodd" />
			</svg>
			Filter
			{#if filterQuery}
				<span class="bg-primary text-black text-xs px-1.5 py-0.5 rounded-full">1</span>
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
				<a
					href="https://en.wikipedia.org/wiki/SQL#Syntax"
					target="_blank"
					class="text-xs text-primary hover:underline"
				>
					SQL-style syntax help
				</a>
			</div>
			<div class="relative">
				<input
					type="text"
					bind:value={filterQuery}
					oninput={handleFilterInput}
					placeholder="e.g., status = 'Active' AND amount > 1000"
					class="w-full px-3 py-2 border rounded-md font-mono text-sm {filterError ? 'border-red-500 focus:ring-red-500 focus:border-red-500' : 'border-gray-300 focus:ring-primary focus:border-primary'}"
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
				<p><strong>Logic:</strong> AND, OR, parentheses for grouping</p>
				<p><strong>Examples:</strong></p>
				<ul class="list-disc list-inside pl-2 space-y-0.5">
					<li><code class="bg-gray-200 px-1 rounded">name LIKE '%Smith%'</code></li>
					<li><code class="bg-gray-200 px-1 rounded">status IN ('Active', 'Pending')</code></li>
					<li><code class="bg-gray-200 px-1 rounded">amount &gt; 1000 AND status = 'Active'</code></li>
				</ul>
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
	{#if loading || metadataLoading}
		<TableSkeleton rows={pageSize} columns={displayFields.length > 0 ? displayFields.length + 2 : 7} />
	{:else if error}
		<ErrorDisplay message={error} onRetry={loadRecords} />
	{:else if records.length === 0}
		<div class="text-center py-12 text-gray-500">
			{#if filterQuery}
				No records match your filter.
				<button onclick={clearFilter} class="text-primary hover:underline">
					Clear filter
				</button>
			{:else}
				No records found.
				<a href="/{entitySlug}/new" class="text-primary hover:underline">
					Create one
				</a>
			{/if}
		</div>
	{:else}
		<div class="bg-white shadow rounded-lg overflow-x-auto">
			<table class="min-w-full divide-y divide-gray-200">
				<thead class="bg-gray-50">
					<tr>
						{#each displayFields as field}
							<th
								class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
								onclick={() => handleSort(field.name)}
							>
								{field.label}
								{#if sortBy === field.name}
									<span class="ml-1">{sortDir === 'asc' ? '↑' : '↓'}</span>
								{/if}
							</th>
						{/each}
						<th
							class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
							onclick={() => handleSort('created_at')}
						>
							Created
							{#if sortBy === 'created_at'}
								<span class="ml-1">{sortDir === 'asc' ? '↑' : '↓'}</span>
							{/if}
						</th>
						<th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
							Actions
						</th>
					</tr>
				</thead>
				<tbody class="bg-white divide-y divide-gray-200">
					{#each records as record (record.id)}
						<tr class="hover:bg-gray-50">
							{#each displayFields as field, i}
								<td class="px-6 py-4 whitespace-nowrap text-sm">
									{#if i === 0}
										<a href="/{entitySlug}/{record.id}" class="text-primary hover:underline font-medium">
											{formatValue(getFieldValue(record, field), field)}
										</a>
									{:else}
										<span class="text-gray-500">
											{formatValue(getFieldValue(record, field), field)}
										</span>
									{/if}
								</td>
							{/each}
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{record.created_at ? new Date(String(record.created_at)).toLocaleDateString() : '-'}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-right text-sm">
								<a
									href="/{entitySlug}/{record.id}/edit"
									class="text-primary hover:underline mr-4"
								>
									Edit
								</a>
								<button
									onclick={() => deleteRecord(String(record.id))}
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
					Showing {(currentPage - 1) * pageSize + 1} to {Math.min(currentPage * pageSize, total)} of {total} results
				</p>
				<div class="flex gap-2">
					<button
						onclick={() => { currentPage = currentPage - 1; loadRecords(); }}
						disabled={currentPage === 1}
						class="px-3 py-1 border border-gray-300 rounded-md text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-50"
					>
						Previous
					</button>
					<span class="px-3 py-1 text-sm text-gray-700">
						Page {currentPage} of {totalPages}
					</span>
					<button
						onclick={() => { currentPage = currentPage + 1; loadRecords(); }}
						disabled={currentPage === totalPages}
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
				<div>
					<label class="block text-sm font-medium text-gray-700 mb-1">View Name</label>
					<input
						type="text"
						bind:value={newViewName}
						placeholder="e.g., Active Contacts, High Value Deals"
						class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary focus:border-primary"
					/>
				</div>
				<div>
					<label class="block text-sm font-medium text-gray-700 mb-1">Current Filter</label>
					<code class="block bg-gray-100 px-3 py-2 rounded text-sm font-mono text-gray-700">
						{filterQuery || '(no filter)'}
					</code>
				</div>
				<div>
					<label class="block text-sm font-medium text-gray-700 mb-1">Current Sort</label>
					<code class="block bg-gray-100 px-3 py-2 rounded text-sm font-mono text-gray-700">
						{sortBy} {sortDir.toUpperCase()}
					</code>
				</div>
				<div class="flex items-center gap-2">
					<input
						type="checkbox"
						id="saveAsDefault"
						bind:checked={saveAsDefault}
						class="h-4 w-4 text-primary focus:ring-primary border-gray-300 rounded"
					/>
					<label for="saveAsDefault" class="text-sm text-gray-700">Set as default view</label>
				</div>
			</div>
			<div class="flex justify-end gap-3 mt-6">
				<button
					onclick={() => showSaveModal = false}
					class="px-4 py-2 border border-gray-300 rounded-md text-sm text-gray-700 hover:bg-gray-50"
				>
					Cancel
				</button>
				<button
					onclick={saveListView}
					disabled={!newViewName.trim() || savingView}
					class="px-4 py-2 bg-primary text-black rounded-md text-sm hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					{savingView ? 'Saving...' : 'Save View'}
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
								{#if view.isSystem}
									<span class="ml-2 text-xs bg-gray-100 text-gray-600 px-2 py-0.5 rounded">System</span>
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
									class="text-xs text-primary hover:underline"
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
