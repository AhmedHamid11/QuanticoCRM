<script lang="ts">
	import { page } from '$app/stores';
	import { get } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import type { Tripwire, TripwireLog, TripwireLogListResponse } from '$lib/types/tripwire';

	let tripwire = $state<Tripwire | null>(null);
	let logs = $state<TripwireLog[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let currentPage = $state(1);
	let pageSize = $state(20);
	let total = $state(0);
	let totalPages = $state(0);
	let statusFilter = $state('');
	let eventFilter = $state('');

	const tripwireId = $derived($page.params.id);

	async function loadTripwire() {
		try {
			tripwire = await get<Tripwire>(`/tripwires/${tripwireId}`);
		} catch (e) {
			toast.error('Failed to load tripwire');
		}
	}

	async function loadLogs() {
		try {
			loading = true;
			error = null;
			const params = new URLSearchParams({
				page: currentPage.toString(),
				pageSize: pageSize.toString(),
				sortBy: 'executed_at',
				sortDir: 'desc'
			});
			if (statusFilter) {
				params.set('status', statusFilter);
			}
			if (eventFilter) {
				params.set('eventType', eventFilter);
			}
			const result = await get<TripwireLogListResponse>(`/tripwires/${tripwireId}/logs?${params}`);
			logs = result.data;
			total = result.total;
			totalPages = result.totalPages;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load logs';
			toast.error(error);
		} finally {
			loading = false;
		}
	}

	function handleFilterChange() {
		currentPage = 1;
		loadLogs();
	}

	function formatDateTime(dateStr: string): string {
		return new Date(dateStr).toLocaleString();
	}

	function getStatusColor(status: string): string {
		switch (status) {
			case 'success': return 'bg-green-100 text-green-800';
			case 'failed': return 'bg-red-100 text-red-800';
			case 'timeout': return 'bg-yellow-100 text-yellow-800';
			default: return 'bg-gray-100 text-gray-800';
		}
	}

	function getEventColor(event: string): string {
		switch (event) {
			case 'CREATE': return 'bg-blue-100 text-blue-800';
			case 'UPDATE': return 'bg-purple-100 text-purple-800';
			case 'DELETE': return 'bg-red-100 text-red-800';
			default: return 'bg-gray-100 text-gray-800';
		}
	}

	// Reload data when tripwire ID changes (handles navigation between tripwire log pages)
	$effect(() => {
		// Track tripwireId to trigger reload on navigation
		const _id = tripwireId;

		// Reset state
		tripwire = null;
		logs = [];
		loading = true;
		error = null;
		currentPage = 1;
		total = 0;
		totalPages = 0;
		statusFilter = '';
		eventFilter = '';

		// Load data
		(async () => {
			await loadTripwire();
			await loadLogs();
		})();
	});
</script>

<div class="space-y-4">
	<!-- Header -->
	<div class="flex justify-between items-center">
		<div>
			<h1 class="text-2xl font-bold text-gray-900">Execution Logs</h1>
			{#if tripwire}
				<p class="text-sm text-gray-500 mt-1">Tripwire: {tripwire.name}</p>
			{/if}
		</div>
		<a
			href="/admin/tripwires"
			class="text-sm text-gray-600 hover:text-gray-900"
		>
			&larr; Back to Tripwires
		</a>
	</div>

	<!-- Filters -->
	<div class="flex gap-4 flex-wrap">
		<select
			bind:value={statusFilter}
			onchange={handleFilterChange}
			class="px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
		>
			<option value="">All Statuses</option>
			<option value="success">Success</option>
			<option value="failed">Failed</option>
			<option value="timeout">Timeout</option>
		</select>
		<select
			bind:value={eventFilter}
			onchange={handleFilterChange}
			class="px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
		>
			<option value="">All Events</option>
			<option value="CREATE">CREATE</option>
			<option value="UPDATE">UPDATE</option>
			<option value="DELETE">DELETE</option>
		</select>
	</div>

	<!-- Table -->
	{#if loading}
		<div class="text-center py-12 text-gray-500">Loading...</div>
	{:else if error}
		<div class="text-center py-12 text-red-500">{error}</div>
	{:else if logs.length === 0}
		<div class="text-center py-12 text-gray-500">
			No execution logs found.
		</div>
	{:else}
		<div class="bg-white shadow rounded-lg overflow-hidden">
			<table class="min-w-full divide-y divide-gray-200">
				<thead class="bg-gray-50">
					<tr>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Executed At
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Event
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Entity / Record
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Status
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Response
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Duration
						</th>
					</tr>
				</thead>
				<tbody class="bg-white divide-y divide-gray-200">
					{#each logs as log (log.id)}
						<tr class="hover:bg-gray-50">
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{formatDateTime(log.executedAt)}
							</td>
							<td class="px-6 py-4 whitespace-nowrap">
								<span class="px-2 py-1 text-xs font-medium rounded-full {getEventColor(log.eventType)}">
									{log.eventType}
								</span>
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{log.entityType} / {log.recordId}
							</td>
							<td class="px-6 py-4 whitespace-nowrap">
								<span class="px-2 py-1 text-xs font-medium rounded-full {getStatusColor(log.status)}">
									{log.status}
								</span>
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{#if log.responseCode}
									HTTP {log.responseCode}
								{:else if log.errorMessage}
									<span class="text-red-600" title={log.errorMessage}>
										{log.errorMessage.length > 30 ? log.errorMessage.substring(0, 30) + '...' : log.errorMessage}
									</span>
								{:else}
									-
								{/if}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{#if log.durationMs !== undefined}
									{log.durationMs}ms
								{:else}
									-
								{/if}
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
						onclick={() => { currentPage = currentPage - 1; loadLogs(); }}
						disabled={currentPage === 1}
						class="px-3 py-1 border border-gray-300 rounded-md text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-50"
					>
						Previous
					</button>
					<span class="px-3 py-1 text-sm text-gray-700">
						Page {currentPage} of {totalPages}
					</span>
					<button
						onclick={() => { currentPage = currentPage + 1; loadLogs(); }}
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
