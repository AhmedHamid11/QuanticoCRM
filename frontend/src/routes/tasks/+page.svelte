<script lang="ts">
	import { onMount } from 'svelte';
	import { get, del, post, put } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import { TableSkeleton, ErrorDisplay } from '$lib/components/ui';
	import type { Task, TaskListResponse } from '$lib/types/task';

	interface ListView {
		id: string;
		orgId: string;
		entityName: string;
		name: string;
		filterQuery: string;
		columns: string[];
		sortBy: string;
		sortDir: string;
		isDefault: boolean;
		isSystem: boolean;
	}

	let tasks = $state<Task[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let search = $state('');
	let page = $state(1);
	let pageSize = $state(20);
	let total = $state(0);
	let totalPages = $state(0);
	let sortBy = $state('created_at');
	let sortDir = $state<'asc' | 'desc'>('desc');
	let statusFilter = $state('');
	let typeFilter = $state('');
	let searchTimeout: ReturnType<typeof setTimeout> | null = null;
	let knownTotal = $state<number | null>(null);

	// Filter and list view state
	let filterQuery = $state('');
	let showFilterPanel = $state(false);
	let listViews = $state<ListView[]>([]);
	let selectedListView = $state<ListView | null>(null);
	let showSaveViewModal = $state(false);
	let showManageViewsModal = $state(false);
	let newViewName = $state('');
	let saveAsNew = $state(true);
	let saveMode = $state<'new' | 'update'>('new');
	let viewToUpdate = $state<ListView | null>(null);

	async function loadTasks() {
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
			if (statusFilter) {
				params.set('status', statusFilter);
			}
			if (typeFilter) {
				params.set('type', typeFilter);
			}
			if (filterQuery.trim()) {
				params.set('filter', filterQuery.trim());
			}
			if (knownTotal !== null && page > 1) {
				params.set('knownTotal', knownTotal.toString());
			}
			const result = await get<TaskListResponse>(`/tasks?${params}`);
			tasks = result.data;
			total = result.total;
			totalPages = result.totalPages;
			knownTotal = result.total;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load tasks';
			toast.error(error);
		} finally {
			loading = false;
		}
	}

	async function loadListViews() {
		try {
			const result = await get<ListView[]>('/list-views/Task');
			listViews = result || [];
			const defaultView = listViews.find((v) => v.isDefault);
			if (defaultView && !selectedListView) {
				selectListView(defaultView);
			}
		} catch (e) {
			console.error('Failed to load list views:', e);
		}
	}

	function selectListView(view: ListView | null) {
		selectedListView = view;
		if (view) {
			filterQuery = view.filterQuery || '';
			sortBy = view.sortBy || 'created_at';
			sortDir = (view.sortDir as 'asc' | 'desc') || 'desc';
			showFilterPanel = !!view.filterQuery;
		} else {
			filterQuery = '';
			sortBy = 'created_at';
			sortDir = 'desc';
		}
		page = 1;
		knownTotal = null;
		loadTasks();
	}

	async function saveListView() {
		try {
			if (saveMode === 'update' && viewToUpdate) {
				// Update existing view
				const viewData = {
					name: viewToUpdate.name,
					entityName: 'Task',
					filterQuery: filterQuery.trim(),
					columns: [],
					sortBy,
					sortDir,
					isDefault: viewToUpdate.isDefault
				};
				const result = await put<ListView>(`/list-views/Task/${viewToUpdate.id}`, viewData);
				listViews = listViews.map((v) => (v.id === result.id ? result : v));
				selectedListView = result;
				toast.success('View updated');
			} else {
				// Create new view
				if (!newViewName.trim()) {
					toast.error('Please enter a view name');
					return;
				}
				const viewData = {
					name: newViewName.trim(),
					entityName: 'Task',
					filterQuery: filterQuery.trim(),
					columns: [],
					sortBy,
					sortDir
				};
				const result = await post<ListView>('/list-views/Task', viewData);
				listViews = [...listViews, result];
				selectedListView = result;
				toast.success('View saved');
			}
			showSaveViewModal = false;
			newViewName = '';
			saveMode = 'new';
			viewToUpdate = null;
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to save view';
			toast.error(message);
		}
	}

	async function deleteListView(view: ListView) {
		if (view.isSystem) {
			toast.error('Cannot delete system views');
			return;
		}

		try {
			await del(`/list-views/Task/${view.id}`);
			listViews = listViews.filter((v) => v.id !== view.id);
			if (selectedListView?.id === view.id) {
				selectedListView = null;
				filterQuery = '';
				loadTasks();
			}
			toast.success('View deleted');
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to delete view';
			toast.error(message);
		}
	}

	async function setDefaultView(view: ListView) {
		try {
			await put(`/list-views/Task/${view.id}`, { ...view, isDefault: true });
			listViews = listViews.map((v) => ({
				...v,
				isDefault: v.id === view.id
			}));
			toast.success('Default view updated');
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to set default view';
			toast.error(message);
		}
	}

	async function deleteTask(id: string) {
		const backup = [...tasks];
		tasks = tasks.filter((t) => t.id !== id);
		total = total - 1;

		try {
			await del(`/tasks/${id}`);
			toast.success('Task deleted');
		} catch (e) {
			tasks = backup;
			total = total + 1;
			const message = e instanceof Error ? e.message : 'Failed to delete task';
			toast.error(message);
		}
	}

	function handleSearchInput() {
		if (searchTimeout) {
			clearTimeout(searchTimeout);
		}
		searchTimeout = setTimeout(() => {
			page = 1;
			knownTotal = null;
			loadTasks();
		}, 300);
	}

	function handleFilterChange() {
		page = 1;
		knownTotal = null;
		loadTasks();
	}

	function applyFilter() {
		page = 1;
		knownTotal = null;
		loadTasks();
	}

	function clearFilter() {
		filterQuery = '';
		selectedListView = null;
		page = 1;
		knownTotal = null;
		loadTasks();
	}

	function handleSort(column: string) {
		if (sortBy === column) {
			sortDir = sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			sortBy = column;
			sortDir = 'asc';
		}
		knownTotal = null;
		loadTasks();
	}

	function formatDate(dateStr: string | null): string {
		if (!dateStr) return '-';
		return new Date(dateStr).toLocaleDateString();
	}

	function getStatusColor(status: string): string {
		switch (status) {
			case 'Open': return 'bg-blue-100 text-blue-800';
			case 'In Progress': return 'bg-yellow-100 text-yellow-800';
			case 'Completed': return 'bg-green-100 text-green-800';
			case 'Deferred': return 'bg-gray-100 text-gray-800';
			case 'Cancelled': return 'bg-red-100 text-red-800';
			default: return 'bg-gray-100 text-gray-800';
		}
	}

	function getPriorityColor(priority: string): string {
		switch (priority) {
			case 'Urgent': return 'text-red-600 font-semibold';
			case 'High': return 'text-orange-600';
			case 'Normal': return 'text-gray-600';
			case 'Low': return 'text-gray-400';
			default: return 'text-gray-600';
		}
	}

	function openSaveViewModal() {
		newViewName = '';
		saveAsNew = !selectedListView || selectedListView.isSystem;
		showSaveViewModal = true;
	}

	onMount(() => {
		loadListViews();
		loadTasks();
	});
</script>

<div class="space-y-4">
	<!-- Header -->
	<div class="flex justify-between items-center">
		<h1 class="text-2xl font-bold text-gray-900">Tasks</h1>
		<a
			href="/tasks/new"
			class="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-black bg-primary hover:bg-primary/90"
		>
			+ New Task
		</a>
	</div>

	<!-- Search, Filters, and List Views -->
	<div class="flex gap-4 flex-wrap items-center">
		<div class="flex-1 min-w-64 relative">
			<input
				type="text"
				bind:value={search}
				oninput={handleSearchInput}
				placeholder="Search tasks..."
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
		</div>
		<select
			bind:value={statusFilter}
			onchange={handleFilterChange}
			class="px-3 py-2 border border-gray-300 rounded-md focus:ring-primary focus:border-primary"
		>
			<option value="">All Statuses</option>
			<option value="Open">Open</option>
			<option value="In Progress">In Progress</option>
			<option value="Completed">Completed</option>
			<option value="Deferred">Deferred</option>
			<option value="Cancelled">Cancelled</option>
		</select>
		<select
			bind:value={typeFilter}
			onchange={handleFilterChange}
			class="px-3 py-2 border border-gray-300 rounded-md focus:ring-primary focus:border-primary"
		>
			<option value="">All Types</option>
			<option value="Call">Call</option>
			<option value="Email">Email</option>
			<option value="Meeting">Meeting</option>
			<option value="Todo">Todo</option>
		</select>

		<!-- Filter Toggle -->
		<button
			onclick={() => (showFilterPanel = !showFilterPanel)}
			class="px-3 py-2 border rounded-md text-sm font-medium {showFilterPanel || filterQuery
				? 'border-primary bg-blue-50 text-blue-700'
				: 'border-gray-300 text-gray-700 hover:bg-gray-50'}"
			aria-label="Toggle filter panel"
		>
			<svg class="w-5 h-5 inline-block mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path
					stroke-linecap="round"
					stroke-linejoin="round"
					stroke-width="2"
					d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z"
				/>
			</svg>
			Filter
			{#if filterQuery}
				<span class="ml-1 text-xs bg-primary text-black px-1.5 py-0.5 rounded-full">1</span>
			{/if}
		</button>

		<!-- List View Selector -->
		{#if listViews.length > 0}
			<div class="relative">
				<select
					bind:value={selectedListView}
					onchange={() => selectListView(selectedListView)}
					class="px-3 py-2 border border-gray-300 rounded-md focus:ring-primary focus:border-primary pr-8"
				>
					<option value={null}>All Tasks</option>
					{#each listViews as view (view.id)}
						<option value={view}>{view.name} {view.isDefault ? '(Default)' : ''}</option>
					{/each}
				</select>
			</div>
		{/if}

		<!-- View Actions -->
		{#if listViews.length > 0}
			<button
				onclick={() => (showManageViewsModal = true)}
				class="px-3 py-2 text-sm text-gray-600 hover:text-gray-900"
				aria-label="Manage saved views"
			>
				Manage Views
			</button>
		{/if}
	</div>

	<!-- Filter Panel -->
	{#if showFilterPanel}
		<div class="bg-gray-50 border border-gray-200 rounded-lg p-4 space-y-3">
			<div class="flex items-start gap-4">
				<div class="flex-1">
					<label for="filter-query" class="block text-sm font-medium text-gray-700 mb-1">
						Filter Query (SQL-style WHERE clause)
					</label>
					<textarea
						id="filter-query"
						bind:value={filterQuery}
						placeholder="e.g., status = 'Open' AND priority = 'High'"
						rows="2"
						class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary focus:border-primary font-mono text-sm"
					></textarea>
				</div>
				<div class="flex flex-col gap-2 pt-6">
					<button
						onclick={openSaveViewModal}
						class="px-4 py-2 bg-green-600 text-white text-sm font-medium rounded-md hover:bg-green-700"
					>
						Save View
					</button>
					<button
						onclick={applyFilter}
						class="px-4 py-2 bg-primary text-black text-sm font-medium rounded-md hover:bg-primary/90"
					>
						Apply
					</button>
					<button
						onclick={clearFilter}
						class="px-4 py-2 border border-gray-300 text-sm font-medium rounded-md hover:bg-gray-50"
					>
						Clear
					</button>
				</div>
			</div>
			<details class="text-sm text-gray-600">
				<summary class="cursor-pointer hover:text-gray-900">Filter Syntax Help</summary>
				<div class="mt-2 pl-4 space-y-1">
					<p><code class="bg-gray-200 px-1 rounded">status = 'Open'</code> - Exact match</p>
					<p><code class="bg-gray-200 px-1 rounded">subject LIKE '%report%'</code> - Contains</p>
					<p><code class="bg-gray-200 px-1 rounded">priority IN ('High', 'Urgent')</code> - Multiple values</p>
					<p><code class="bg-gray-200 px-1 rounded">due_date {'>'} TODAY</code> - Future tasks</p>
					<p><code class="bg-gray-200 px-1 rounded">due_date {'<'} TODAY</code> - Overdue tasks</p>
					<p><code class="bg-gray-200 px-1 rounded">due_date {'<='} TODAY + 7</code> - Due within a week</p>
					<p><code class="bg-gray-200 px-1 rounded">due_date {'>'} TODAY - 1m</code> - Last month (d=days, w=weeks, m=months)</p>
					<p><code class="bg-gray-200 px-1 rounded">status = 'Open' AND priority = 'High'</code> - Combined</p>
				</div>
			</details>
		</div>
	{/if}

	<!-- Table -->
	{#if loading}
		<TableSkeleton rows={pageSize} columns={7} />
	{:else if error}
		<ErrorDisplay message={error} onRetry={loadTasks} />
	{:else if tasks.length === 0}
		<div class="text-center py-12 text-gray-500">
			No tasks found. <a href="/tasks/new" class="text-primary hover:underline">Create one</a>
		</div>
	{:else}
		<div class="bg-white shadow rounded-lg overflow-hidden">
			<table class="min-w-full divide-y divide-gray-200">
				<thead class="bg-gray-50">
					<tr>
						<th
							class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
							onclick={() => handleSort('subject')}
						>
							Subject
							{#if sortBy === 'subject'}
								<span class="ml-1">{sortDir === 'asc' ? '↑' : '↓'}</span>
							{/if}
						</th>
						<th
							class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
							onclick={() => handleSort('status')}
						>
							Status
							{#if sortBy === 'status'}
								<span class="ml-1">{sortDir === 'asc' ? '↑' : '↓'}</span>
							{/if}
						</th>
						<th
							class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
							onclick={() => handleSort('type')}
						>
							Type
							{#if sortBy === 'type'}
								<span class="ml-1">{sortDir === 'asc' ? '↑' : '↓'}</span>
							{/if}
						</th>
						<th
							class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
							onclick={() => handleSort('priority')}
						>
							Priority
							{#if sortBy === 'priority'}
								<span class="ml-1">{sortDir === 'asc' ? '↑' : '↓'}</span>
							{/if}
						</th>
						<th
							class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
							onclick={() => handleSort('due_date')}
						>
							Due Date
							{#if sortBy === 'due_date'}
								<span class="ml-1">{sortDir === 'asc' ? '↑' : '↓'}</span>
							{/if}
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Related To
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
					{#each tasks as task (task.id)}
						<tr class="hover:bg-gray-50">
							<td class="px-6 py-4 whitespace-nowrap">
								<a href="/tasks/{task.id}" class="text-primary hover:underline font-medium">
									{task.subject}
								</a>
							</td>
							<td class="px-6 py-4 whitespace-nowrap">
								<span class="px-2 py-1 text-xs font-medium rounded-full {getStatusColor(task.status)}">
									{task.status}
								</span>
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{task.type}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm {getPriorityColor(task.priority)}">
								{task.priority}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{formatDate(task.dueDate)}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{#if task.parentType && task.parentId}
									<a
										href="/{task.parentType.toLowerCase()}s/{task.parentId}"
										class="text-primary hover:underline"
									>
										{task.parentName || task.parentType}
									</a>
								{:else}
									-
								{/if}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								<div>{formatDate(task.modifiedAt)}</div>
								{#if task.modifiedByName}
									<div class="text-xs text-gray-400">{task.modifiedByName}</div>
								{/if}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-right text-sm">
								<a href="/tasks/{task.id}/edit" class="text-primary hover:underline mr-4">
									Edit
								</a>
								<button onclick={() => deleteTask(task.id)} class="text-red-600 hover:underline">
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
						onclick={() => { page = page - 1; loadTasks(); }}
						disabled={page === 1}
						class="px-3 py-1 border border-gray-300 rounded-md text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-50"
					>
						Previous
					</button>
					<span class="px-3 py-1 text-sm text-gray-700">
						Page {page} of {totalPages}
					</span>
					<button
						onclick={() => { page = page + 1; loadTasks(); }}
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
{#if showSaveViewModal}
	<div
		class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50"
		role="dialog"
		aria-modal="true"
		aria-labelledby="save-view-title"
		onclick={() => { showSaveViewModal = false; saveMode = 'new'; viewToUpdate = null; }}
		onkeydown={(e) => e.key === 'Escape' && (showSaveViewModal = false)}
	>
		<div
			class="bg-white rounded-lg shadow-xl p-6 w-full max-w-md"
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => e.stopPropagation()}
			role="document"
		>
			<h3 id="save-view-title" class="text-lg font-semibold mb-4">Save View</h3>

			<!-- Save Mode Selection -->
			{#if listViews.filter(v => !v.isSystem).length > 0}
				<div class="flex gap-4 border-b pb-3 mb-4">
					<label class="flex items-center gap-2 cursor-pointer">
						<input
							type="radio"
							name="taskSaveMode"
							value="new"
							bind:group={saveMode}
							class="h-4 w-4 text-primary"
						/>
						<span class="text-sm text-gray-700">Create new view</span>
					</label>
					<label class="flex items-center gap-2 cursor-pointer">
						<input
							type="radio"
							name="taskSaveMode"
							value="update"
							bind:group={saveMode}
							class="h-4 w-4 text-primary"
						/>
						<span class="text-sm text-gray-700">Update existing</span>
					</label>
				</div>
			{/if}

			{#if saveMode === 'new'}
				<div class="mb-4">
					<label for="view-name" class="block text-sm font-medium text-gray-700 mb-1">
						View Name
					</label>
					<input
						id="view-name"
						type="text"
						bind:value={newViewName}
						placeholder="e.g., My High Priority Tasks"
						class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary focus:border-primary"
					/>
				</div>
			{:else}
				<div class="mb-4">
					<label class="block text-sm font-medium text-gray-700 mb-1">Select View to Update</label>
					<select
						bind:value={viewToUpdate}
						class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary focus:border-primary"
					>
						<option value={null}>-- Select a view --</option>
						{#each listViews.filter(v => !v.isSystem) as view}
							<option value={view}>{view.name}</option>
						{/each}
					</select>
				</div>
			{/if}

			<div class="mb-4 text-sm text-gray-600">
				<p><strong>Current settings:</strong></p>
				<p>Filter: {filterQuery || '(none)'}</p>
				<p>Sort: {sortBy} ({sortDir})</p>
			</div>
			<div class="flex justify-end gap-3">
				<button
					onclick={() => { showSaveViewModal = false; saveMode = 'new'; viewToUpdate = null; }}
					class="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium hover:bg-gray-50"
				>
					Cancel
				</button>
				<button
					onclick={saveListView}
					disabled={(saveMode === 'new' && !newViewName.trim()) || (saveMode === 'update' && !viewToUpdate)}
					class="px-4 py-2 bg-primary text-black rounded-md text-sm font-medium hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					{saveMode === 'update' ? 'Update View' : 'Save View'}
				</button>
			</div>
		</div>
	</div>
{/if}

<!-- Manage Views Modal -->
{#if showManageViewsModal}
	<div
		class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50"
		role="dialog"
		aria-modal="true"
		aria-labelledby="manage-views-title"
		onclick={() => (showManageViewsModal = false)}
		onkeydown={(e) => e.key === 'Escape' && (showManageViewsModal = false)}
	>
		<div
			class="bg-white rounded-lg shadow-xl p-6 w-full max-w-lg"
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => e.stopPropagation()}
			role="document"
		>
			<h3 id="manage-views-title" class="text-lg font-semibold mb-4">Manage Views</h3>
			<div class="space-y-2 max-h-96 overflow-y-auto">
				{#each listViews as view (view.id)}
					<div class="flex items-center justify-between p-3 border rounded-md hover:bg-gray-50">
						<div>
							<span class="font-medium">{view.name}</span>
							{#if view.isDefault}
								<span class="ml-2 text-xs bg-blue-100 text-blue-800 px-2 py-0.5 rounded">Default</span>
							{/if}
							{#if view.isSystem}
								<span class="ml-2 text-xs bg-gray-100 text-gray-600 px-2 py-0.5 rounded">System</span>
							{/if}
							{#if view.filterQuery}
								<p class="text-xs text-gray-500 mt-1 font-mono truncate max-w-xs">
									{view.filterQuery}
								</p>
							{/if}
						</div>
						<div class="flex items-center gap-2">
							{#if !view.isDefault}
								<button
									onclick={() => setDefaultView(view)}
									class="text-sm text-primary hover:underline"
								>
									Set Default
								</button>
							{/if}
							{#if !view.isSystem}
								<button
									onclick={() => deleteListView(view)}
									class="text-sm text-red-600 hover:underline"
								>
									Delete
								</button>
							{/if}
						</div>
					</div>
				{/each}
				{#if listViews.length === 0}
					<p class="text-gray-500 text-center py-4">No saved views yet.</p>
				{/if}
			</div>
			<div class="flex justify-end mt-4">
				<button
					onclick={() => (showManageViewsModal = false)}
					class="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium hover:bg-gray-50"
				>
					Close
				</button>
			</div>
		</div>
	</div>
{/if}
