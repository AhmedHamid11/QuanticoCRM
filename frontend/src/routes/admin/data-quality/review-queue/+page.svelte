<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import {
		listPendingAlerts,
		mergePreview,
		mergeExecute,
		mergeUndo,
		getBannerClass,
		getConfidenceBadgeClass,
		formatConfidence,
		type PendingAlert,
		type MergePreview,
		type PaginatedResponse
	} from '$lib/api/data-quality';
	import { resolveAlert } from '$lib/api/dedup';
	import { toast } from '$lib/stores/toast.svelte';

	// State management with Svelte 5 runes
	let alerts = $state<PendingAlert[]>([]);
	let loading = $state(true);
	let entityFilter = $state('');
	let currentPage = $state(1);
	let total = $state(0);
	let pageSize = $state(20);
	let selectedIds = $state<Set<string>>(new Set());
	let showBulkBar = $derived(selectedIds.size > 0);
	let bulkProcessing = $state(false);
	let bulkProgress = $state({ current: 0, total: 0 });

	// Load alerts on mount
	onMount(() => {
		loadAlerts();
	});

	// Reload when filter or page changes
	let prevFilter = $state('');
	let prevPage = $state(1);
	$effect(() => {
		const filter = entityFilter;
		const pg = currentPage;
		if (filter !== prevFilter || pg !== prevPage) {
			prevFilter = filter;
			prevPage = pg;
			loadAlerts();
		}
	});

	async function loadAlerts() {
		loading = true;
		try {
			const response: PaginatedResponse<PendingAlert> = await listPendingAlerts({
				entityType: entityFilter || undefined,
				page: currentPage,
				pageSize
			});
			alerts = response.data;
			total = response.total;
		} catch (error: any) {
			toast.error(`Failed to load alerts: ${error.message || 'Unknown error'}`);
		} finally {
			loading = false;
		}
	}

	function handleFilterChange(event: Event) {
		const target = event.target as HTMLSelectElement;
		entityFilter = target.value;
		currentPage = 1;
		selectedIds.clear();
	}

	function handlePageChange(newPage: number) {
		currentPage = newPage;
		selectedIds.clear();
		window.scrollTo({ top: 0, behavior: 'smooth' });
	}

	function toggleSelection(alertId: string) {
		if (selectedIds.has(alertId)) {
			selectedIds.delete(alertId);
		} else {
			selectedIds.add(alertId);
		}
		selectedIds = new Set(selectedIds); // Trigger reactivity
	}

	function toggleSelectAll() {
		if (selectedIds.size === alerts.length) {
			selectedIds.clear();
		} else {
			selectedIds = new Set(alerts.map((a) => a.id));
		}
	}

	async function dismissAlert(alert: PendingAlert) {
		const backup = [...alerts];
		// Optimistic UI: remove immediately
		alerts = alerts.filter((a) => a.id !== alert.id);
		selectedIds.delete(alert.id);
		selectedIds = new Set(selectedIds);

		try {
			await resolveAlert(alert.entityType, alert.recordId, 'dismissed');
			toast.success('Alert dismissed');
			total--;
		} catch (error: any) {
			// Restore on error
			alerts = backup;
			toast.error(`Failed to dismiss: ${error.message || 'Unknown error'}`);
		}
	}

	async function quickMerge(alert: PendingAlert) {
		try {
			// Step 1: Get merge preview to determine survivor
			const recordIds = [alert.recordId, ...alert.matches.map((m) => m.recordId)];
			const preview: MergePreview = await mergePreview({
				recordIds,
				entityType: alert.entityType
			});

			// Step 2: Auto-select fields from survivor (most complete record)
			const survivorId = preview.suggestedSurvivorId;
			const duplicateIds = recordIds.filter((id) => id !== survivorId);

			// Step 3: Execute merge with auto-selected survivor
			const result = await mergeExecute({
				entityType: alert.entityType,
				survivorId,
				duplicateIds,
				mergedFields: {}
			});

			// Optimistic UI: remove card
			alerts = alerts.filter((a) => a.id !== alert.id);
			selectedIds.delete(alert.id);
			selectedIds = new Set(selectedIds);
			total--;

			toast.success('Records merged successfully.');

			// Mark alert as merged
			await resolveAlert(alert.entityType, alert.recordId, 'merged');

		} catch (error: any) {
			toast.error(`Merge failed: ${error.message || 'Unknown error'}`);
		}
	}

	function navigateToMergeWizard(alertId: string) {
		goto(`/admin/data-quality/merge/${alertId}`);
	}

	async function bulkDismiss() {
		if (selectedIds.size === 0) return;

		bulkProcessing = true;
		bulkProgress = { current: 0, total: selectedIds.size };

		const selectedAlerts = alerts.filter((a) => selectedIds.has(a.id));
		let successCount = 0;
		let failCount = 0;

		for (const alert of selectedAlerts) {
			try {
				await resolveAlert(alert.entityType, alert.recordId, 'dismissed');
				successCount++;
			} catch (error) {
				failCount++;
			}
			bulkProgress.current++;
		}

		// Remove dismissed alerts from list
		alerts = alerts.filter((a) => !selectedIds.has(a.id));
		total -= successCount;
		selectedIds.clear();
		bulkProcessing = false;

		if (failCount > 0) {
			toast.error(`Dismissed ${successCount} alerts, ${failCount} failed`);
		} else {
			toast.success(`Dismissed ${successCount} alerts`);
		}
	}

	async function bulkMerge() {
		if (selectedIds.size === 0) return;

		bulkProcessing = true;
		bulkProgress = { current: 0, total: selectedIds.size };

		const selectedAlerts = alerts.filter((a) => selectedIds.has(a.id));
		let successCount = 0;
		let failCount = 0;

		for (const alert of selectedAlerts) {
			try {
				await quickMerge(alert);
				successCount++;
			} catch (error) {
				failCount++;
			}
			bulkProgress.current++;
		}

		selectedIds.clear();
		bulkProcessing = false;

		if (failCount > 0) {
			toast.error(`Merged ${successCount} groups, ${failCount} failed`);
		} else {
			toast.success(`Merged ${successCount} groups`);
		}
	}

	// Compute pagination
	const totalPages = $derived(Math.ceil(total / pageSize));
	const hasNextPage = $derived(currentPage < totalPages);
	const hasPrevPage = $derived(currentPage > 1);
</script>

<div class="p-6">
	<!-- Header -->
	<div class="mb-6 flex items-center justify-between">
		<div class="flex items-center gap-4">
			<h1 class="text-2xl font-semibold text-gray-900">Review Queue</h1>
			<span class="rounded-full bg-blue-100 px-3 py-1 text-sm font-medium text-blue-800">
				{total} pending
			</span>
		</div>

		<div class="flex items-center gap-4">
			<!-- Entity Filter -->
			<select
				onchange={handleFilterChange}
				value={entityFilter}
				class="rounded-lg border border-gray-300 px-4 py-2 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
			>
				<option value="">All Entity Types</option>
				<option value="Contact">Contact</option>
				<option value="Account">Account</option>
				<option value="Lead">Lead</option>
				<option value="Opportunity">Opportunity</option>
			</select>

			<!-- Pagination Controls -->
			{#if totalPages > 1}
				<div class="flex items-center gap-2">
					<button
						onclick={() => handlePageChange(currentPage - 1)}
						disabled={!hasPrevPage}
						class="rounded-lg border border-gray-300 px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50"
					>
						Previous
					</button>
					<span class="text-sm text-gray-600">
						Page {currentPage} of {totalPages}
					</span>
					<button
						onclick={() => handlePageChange(currentPage + 1)}
						disabled={!hasNextPage}
						class="rounded-lg border border-gray-300 px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50"
					>
						Next
					</button>
				</div>
			{/if}
		</div>
	</div>

	<!-- Loading State -->
	{#if loading}
		<div class="space-y-4">
			{#each [1, 2, 3] as i}
				<div class="animate-pulse rounded-lg border border-gray-200 bg-white p-6">
					<div class="h-4 w-1/3 rounded bg-gray-200"></div>
					<div class="mt-4 h-20 rounded bg-gray-200"></div>
					<div class="mt-4 flex gap-2">
						<div class="h-8 w-20 rounded bg-gray-200"></div>
						<div class="h-8 w-24 rounded bg-gray-200"></div>
						<div class="h-8 w-16 rounded bg-gray-200"></div>
					</div>
				</div>
			{/each}
		</div>
	{:else if alerts.length === 0}
		<!-- Empty State -->
		<div class="flex flex-col items-center justify-center rounded-lg border-2 border-dashed border-gray-300 bg-white py-16">
			<svg
				class="mb-4 h-16 w-16 text-gray-400"
				fill="none"
				stroke="currentColor"
				viewBox="0 0 24 24"
			>
				<path
					stroke-linecap="round"
					stroke-linejoin="round"
					stroke-width="2"
					d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
				/>
			</svg>
			<h3 class="mb-2 text-lg font-medium text-gray-900">No duplicates found</h3>
			<p class="mb-6 text-sm text-gray-500">
				All clear! No duplicate records detected in your data.
			</p>
			<button
				onclick={() => goto('/admin/data-quality/scan-jobs')}
				class="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
			>
				Run a scan
			</button>
		</div>
	{:else}
		<!-- Alert Cards -->
		<div class="space-y-4">
			{#if alerts.length > 0}
				<!-- Select All Checkbox -->
				<div class="flex items-center gap-2 rounded-lg bg-gray-50 px-4 py-2">
					<input
						type="checkbox"
						checked={selectedIds.size === alerts.length && alerts.length > 0}
						onchange={toggleSelectAll}
						class="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
					/>
					<label class="text-sm text-gray-700">
						{selectedIds.size > 0 ? `${selectedIds.size} selected` : 'Select all'}
					</label>
				</div>
			{/if}

			{#each alerts as alert (alert.id)}
				{@const borderClass = getBannerClass(alert.highestConfidence)}
				{@const badgeClass = getConfidenceBadgeClass(alert.highestConfidence)}
				{@const confidenceScore = alert.matches[0]?.matchResult?.score || 0}

				<div class="rounded-lg border-l-4 {borderClass} border-r border-t border-b border-gray-200 bg-white p-6 shadow-sm">
					<div class="flex items-start gap-4">
						<!-- Checkbox -->
						<input
							type="checkbox"
							checked={selectedIds.has(alert.id)}
							onchange={() => toggleSelection(alert.id)}
							class="mt-1 h-5 w-5 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
						/>

						<!-- Card Content -->
						<div class="flex-1">
							<!-- Header -->
							<div class="mb-4 flex items-start justify-between">
								<div>
									<div class="flex items-center gap-2">
										<h3 class="text-lg font-medium text-gray-900">
											{alert.entityType}
										</h3>
										<span class="rounded-full {badgeClass} px-2 py-1 text-xs font-medium">
											{formatConfidence(confidenceScore)} {alert.highestConfidence.toUpperCase()}
										</span>
									</div>
									<p class="mt-1 text-sm text-gray-500">
										Record ID: {alert.recordId}
									</p>
								</div>
							</div>

							<!-- Matched Records -->
							<div class="mb-4 space-y-3">
								<h4 class="text-sm font-medium text-gray-700">Matched Records ({alert.totalMatchCount}):</h4>
								{#each alert.matches as match, idx}
									<div class="rounded-lg border border-gray-200 bg-gray-50 p-3">
										<div class="flex items-start justify-between">
											<div class="flex-1">
												<div class="flex items-center gap-2">
													<span class="font-medium text-gray-900">
														{match.recordName || match.recordId}
													</span>
													<span class="text-xs text-gray-500">
														Match: {formatConfidence(match.matchResult.score)}
													</span>
												</div>
												{#if match.matchResult.matchingFields.length > 0}
													<div class="mt-1 flex flex-wrap gap-1">
														{#each match.matchResult.matchingFields as field}
															<span class="rounded bg-blue-100 px-2 py-0.5 text-xs text-blue-700">
																{field}
															</span>
														{/each}
													</div>
												{/if}
											</div>
										</div>
									</div>
								{/each}
							</div>

							<!-- Action Buttons -->
							<div class="flex gap-2">
								<button
									onclick={() => dismissAlert(alert)}
									class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
								>
									Dismiss
								</button>
								<button
									onclick={() => quickMerge(alert)}
									class="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
								>
									Quick Merge
								</button>
								<button
									onclick={() => navigateToMergeWizard(alert.id)}
									class="rounded-lg border border-blue-600 bg-white px-4 py-2 text-sm font-medium text-blue-600 hover:bg-blue-50"
								>
									Merge
								</button>
							</div>
						</div>
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>

<!-- Floating Bulk Action Bar -->
{#if showBulkBar}
	<div class="fixed bottom-0 left-0 right-0 border-t border-gray-200 bg-white px-6 py-4 shadow-lg">
		<div class="mx-auto flex max-w-7xl items-center justify-between">
			<div class="flex items-center gap-4">
				<span class="text-sm font-medium text-gray-900">
					{selectedIds.size} selected
				</span>
				{#if bulkProcessing}
					<span class="text-sm text-gray-600">
						Processing {bulkProgress.current} / {bulkProgress.total}...
					</span>
				{/if}
			</div>
			<div class="flex gap-2">
				<button
					onclick={bulkDismiss}
					disabled={bulkProcessing}
					class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50"
				>
					Dismiss All
				</button>
				<button
					onclick={bulkMerge}
					disabled={bulkProcessing}
					class="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50"
				>
					Merge All
				</button>
			</div>
		</div>
	</div>
{/if}
