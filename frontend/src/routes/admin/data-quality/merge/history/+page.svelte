<script lang="ts">
	import { onMount } from 'svelte';
	import { mergeHistory, mergeUndo, type MergeHistoryEntry } from '$lib/api/data-quality';
	import { toast } from '$lib/stores/toast.svelte';

	// State
	let entries = $state<MergeHistoryEntry[]>([]);
	let loading = $state(true);
	let page = $state(1);
	let total = $state(0);
	let pageSize = $state(20);
	let entityFilter = $state('');
	let undoingSnapshot = $state<string | null>(null);

	// Available entity types for filter
	const entityTypes = ['', 'Contact', 'Account', 'Lead', 'Opportunity', 'Task', 'Meeting', 'Call', 'Email'];

	onMount(() => {
		loadHistory();
	});

	async function loadHistory() {
		loading = true;
		try {
			const params: any = {
				page,
				pageSize
			};

			if (entityFilter) {
				params.entityType = entityFilter;
			}

			const response = await mergeHistory(params);
			entries = response.data;
			total = response.total;
		} catch (err: any) {
			toast.error(`Failed to load merge history: ${err.message}`);
		} finally {
			loading = false;
		}
	}

	function canUndo(entry: MergeHistoryEntry): boolean {
		// Backend provides canUndo field
		return entry.canUndo;
	}

	function getUndoTooltip(entry: MergeHistoryEntry): string {
		if (!entry.canUndo) {
			const expiresAt = new Date(entry.expiresAt);
			if (expiresAt <= new Date()) return 'Expired (30-day window passed)';
			return 'Already undone';
		}
		return 'Undo this merge';
	}

	function getStatusText(entry: MergeHistoryEntry): string {
		if (!entry.canUndo) {
			const expiresAt = new Date(entry.expiresAt);
			if (expiresAt <= new Date()) return 'Permanent';
			return 'Undone';
		}
		return 'Active';
	}

	function getStatusClass(entry: MergeHistoryEntry): string {
		if (!entry.canUndo) {
			const expiresAt = new Date(entry.expiresAt);
			if (expiresAt <= new Date()) return 'bg-gray-100 text-gray-600';
			return 'bg-gray-100 text-gray-800';
		}
		return 'bg-green-100 text-green-800';
	}

	async function handleUndo(entry: MergeHistoryEntry) {
		if (!canUndo(entry)) {
			toast.error('Cannot undo this merge');
			return;
		}

		// Confirm dialog
		const confirmed = confirm(
			'This will restore the merged records to their pre-merge state. Continue?'
		);

		if (!confirmed) return;

		undoingSnapshot = entry.snapshotId;

		try {
			await mergeUndo(entry.snapshotId);
			toast.success('Merge undone successfully');

			// Reload history to reflect the change
			await loadHistory();
		} catch (err: any) {
			toast.error(`Failed to undo merge: ${err.message}`);
		} finally {
			undoingSnapshot = null;
		}
	}

	function formatDate(dateStr: string): string {
		try {
			const date = new Date(dateStr);
			return date.toLocaleDateString('en-US', {
				year: 'numeric',
				month: 'short',
				day: 'numeric',
				hour: '2-digit',
				minute: '2-digit'
			});
		} catch {
			return dateStr;
		}
	}

	function handleEntityFilterChange() {
		page = 1; // Reset to first page
		loadHistory();
	}

	function handlePageChange(newPage: number) {
		page = newPage;
		loadHistory();
	}

	// Calculate total pages
	const totalPages = $derived(Math.ceil(total / pageSize));
</script>

<div class="space-y-6">
	<!-- Header -->
	<div>
		<h1 class="text-2xl font-bold text-gray-900">Merge History</h1>
		<p class="mt-1 text-sm text-gray-500">
			View recent merge operations. You can undo merges within 30 days.
		</p>
	</div>

	<!-- Filters -->
	<div class="bg-white rounded-lg shadow p-4">
		<div class="flex items-center gap-4">
			<label class="flex items-center gap-2">
				<span class="text-sm font-medium text-gray-700">Entity Type:</span>
				<select
					bind:value={entityFilter}
					onchange={handleEntityFilterChange}
					class="border border-gray-300 rounded px-3 py-2 text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
				>
					<option value="">All</option>
					{#each entityTypes.slice(1) as type}
						<option value={type}>{type}</option>
					{/each}
				</select>
			</label>
		</div>
	</div>

	<!-- History Table -->
	<div class="bg-white rounded-lg shadow overflow-hidden">
		{#if loading}
			<div class="p-12 text-center">
				<div class="animate-spin h-8 w-8 border-4 border-blue-500 border-t-transparent rounded-full mx-auto"></div>
				<p class="mt-4 text-gray-600">Loading merge history...</p>
			</div>
		{:else if entries.length === 0}
			<div class="p-12 text-center">
				<p class="text-gray-500 text-lg">No merge operations recorded yet.</p>
				{#if entityFilter}
					<button
						onclick={() => {
							entityFilter = '';
							handleEntityFilterChange();
						}}
						class="mt-4 text-blue-600 hover:text-blue-800 text-sm"
					>
						Clear filter
					</button>
				{/if}
			</div>
		{:else}
			<div class="overflow-x-auto">
				<table class="min-w-full divide-y divide-gray-200">
					<thead class="bg-gray-50">
						<tr>
							<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
								Date
							</th>
							<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
								Entity Type
							</th>
							<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
								Survivor ID
							</th>
							<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
								Duplicates Merged
							</th>
							<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
								Merged By
							</th>
							<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
								Status
							</th>
							<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
								Actions
							</th>
						</tr>
					</thead>
					<tbody class="bg-white divide-y divide-gray-200">
						{#each entries as entry}
							<tr class="hover:bg-gray-50">
								<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
									{formatDate(entry.createdAt)}
								</td>
								<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
									{entry.entityType}
								</td>
								<td class="px-6 py-4 whitespace-nowrap text-sm">
									<a
										href="/{entry.entityType.toLowerCase()}s/{entry.survivorId}"
										class="text-blue-600 hover:text-blue-800 font-mono"
									>
										{entry.survivorId.substring(0, 8)}...
									</a>
								</td>
								<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
									{entry.duplicateIds.length}
								</td>
								<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900 font-mono">
									{entry.mergedById.substring(0, 8)}...
								</td>
								<td class="px-6 py-4 whitespace-nowrap">
									<span class="px-2 py-1 text-xs font-medium rounded {getStatusClass(entry)}">
										{getStatusText(entry)}
									</span>
								</td>
								<td class="px-6 py-4 whitespace-nowrap text-sm">
									{#if canUndo(entry)}
										<button
											onclick={() => handleUndo(entry)}
											disabled={undoingSnapshot === entry.snapshotId}
											title={getUndoTooltip(entry)}
											class="px-3 py-1 bg-yellow-600 text-white rounded hover:bg-yellow-700 disabled:bg-gray-400 disabled:cursor-not-allowed transition"
										>
											{undoingSnapshot === entry.snapshotId ? 'Undoing...' : 'Undo'}
										</button>
									{:else}
										<span
											title={getUndoTooltip(entry)}
											class="text-gray-400 cursor-not-allowed"
										>
											Undo
										</span>
									{/if}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>

			<!-- Pagination -->
			{#if totalPages > 1}
				<div class="bg-gray-50 px-6 py-4 border-t border-gray-200">
					<div class="flex items-center justify-between">
						<div class="text-sm text-gray-700">
							Showing <strong>{(page - 1) * pageSize + 1}</strong> to
							<strong>{Math.min(page * pageSize, total)}</strong> of
							<strong>{total}</strong> results
						</div>
						<div class="flex gap-2">
							<button
								onclick={() => handlePageChange(page - 1)}
								disabled={page === 1}
								class="px-3 py-1 border border-gray-300 rounded text-sm font-medium text-gray-700 hover:bg-gray-100 disabled:opacity-50 disabled:cursor-not-allowed"
							>
								Previous
							</button>

							{#if page > 2}
								<button
									onclick={() => handlePageChange(1)}
									class="px-3 py-1 border border-gray-300 rounded text-sm font-medium text-gray-700 hover:bg-gray-100"
								>
									1
								</button>
							{/if}

							{#if page > 3}
								<span class="px-3 py-1 text-gray-500">...</span>
							{/if}

							{#if page > 1}
								<button
									onclick={() => handlePageChange(page - 1)}
									class="px-3 py-1 border border-gray-300 rounded text-sm font-medium text-gray-700 hover:bg-gray-100"
								>
									{page - 1}
								</button>
							{/if}

							<button
								class="px-3 py-1 border border-blue-500 bg-blue-50 rounded text-sm font-medium text-blue-600"
							>
								{page}
							</button>

							{#if page < totalPages}
								<button
									onclick={() => handlePageChange(page + 1)}
									class="px-3 py-1 border border-gray-300 rounded text-sm font-medium text-gray-700 hover:bg-gray-100"
								>
									{page + 1}
								</button>
							{/if}

							{#if page < totalPages - 2}
								<span class="px-3 py-1 text-gray-500">...</span>
							{/if}

							{#if page < totalPages - 1}
								<button
									onclick={() => handlePageChange(totalPages)}
									class="px-3 py-1 border border-gray-300 rounded text-sm font-medium text-gray-700 hover:bg-gray-100"
								>
									{totalPages}
								</button>
							{/if}

							<button
								onclick={() => handlePageChange(page + 1)}
								disabled={page === totalPages}
								class="px-3 py-1 border border-gray-300 rounded text-sm font-medium text-gray-700 hover:bg-gray-100 disabled:opacity-50 disabled:cursor-not-allowed"
							>
								Next
							</button>
						</div>
					</div>
				</div>
			{/if}
		{/if}
	</div>
</div>
