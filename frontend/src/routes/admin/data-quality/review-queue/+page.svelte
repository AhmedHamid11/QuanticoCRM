<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import {
		listPendingAlerts,
		mergePreview,
		mergeExecute,
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
	let expandedIds = $state<Set<string>>(new Set());

	function toggleExpand(id: string) {
		if (expandedIds.has(id)) {
			expandedIds.delete(id);
		} else {
			expandedIds.add(id);
		}
		expandedIds = new Set(expandedIds);
	}

	function toggleExpandAll() {
		if (expandedIds.size === alerts.length) {
			expandedIds = new Set();
		} else {
			expandedIds = new Set(alerts.map((a) => a.id));
		}
	}

	const allExpanded = $derived(expandedIds.size === alerts.length && alerts.length > 0);

	// Load alerts on mount
	onMount(() => {
		loadAlerts();
	});

	async function loadAlerts() {
		loading = true;
		try {
			const response: PaginatedResponse<PendingAlert> = await listPendingAlerts({
				entityType: entityFilter || undefined,
				page: currentPage,
				pageSize
			});
			alerts = response.data || [];
			total = response.total || 0;
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
		selectedIds = new Set();
		loadAlerts();
	}

	function handlePageChange(newPage: number) {
		currentPage = newPage;
		selectedIds = new Set();
		window.scrollTo({ top: 0, behavior: 'smooth' });
		loadAlerts();
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
			selectedIds = new Set();
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

	function navigateToMergeWizard(alert: PendingAlert) {
		const recordIds = [alert.recordId, ...alert.matches.map((m) => m.recordId)].join(',');
		goto(`/admin/data-quality/merge/${alert.id}?entityType=${alert.entityType}&recordIds=${recordIds}`);
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
		selectedIds = new Set();
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

		selectedIds = new Set();
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
			<label for="entity-filter" class="sr-only">Filter by entity type</label>
			<select
				id="entity-filter"
				name="entity-filter"
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
			<!-- Toolbar: Select All + Expand All -->
			{#if alerts.length > 0}
				<div class="flex items-center gap-4 rounded-lg bg-gray-50 px-4 py-2">
					<div class="flex items-center gap-2">
						<input
							id="select-all"
							name="select-all"
							type="checkbox"
							checked={selectedIds.size === alerts.length && alerts.length > 0}
							onchange={toggleSelectAll}
							class="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
						/>
						<label for="select-all" class="text-sm text-gray-700">
							{selectedIds.size > 0 ? `${selectedIds.size} selected` : 'Select all'}
						</label>
					</div>
					<div class="h-4 w-px bg-gray-300"></div>
					<button
						onclick={toggleExpandAll}
						class="flex items-center gap-1 text-sm text-gray-600 hover:text-gray-900"
					>
						<svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							{#if allExpanded}
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 15l7-7 7 7" />
							{:else}
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
							{/if}
						</svg>
						{allExpanded ? 'Collapse All' : 'Expand All'}
					</button>
				</div>
			{/if}

			<!-- Compact Alert Rows -->
			<div class="overflow-hidden rounded-lg border border-gray-200 bg-white">
				{#each alerts as alert, i (alert.id)}
					{@const badgeClass = getConfidenceBadgeClass(alert.highestConfidence)}
					{@const confidenceScore = alert.matches[0]?.matchResult?.score || 0}
					{@const isExpanded = expandedIds.has(alert.id)}

					<!-- Compact Row -->
					<div class="{i > 0 ? 'border-t border-gray-200' : ''}">
						<div
							class="flex items-center gap-3 px-4 py-3 hover:bg-gray-50 cursor-pointer"
							role="button"
							tabindex="0"
							onclick={(e) => {
								if ((e.target as HTMLElement).closest('input, button')) return;
								toggleExpand(alert.id);
							}}
							onkeydown={(e) => {
								if (e.key === 'Enter' || e.key === ' ') {
									e.preventDefault();
									toggleExpand(alert.id);
								}
							}}
						>
							<!-- Checkbox -->
							<input
								type="checkbox"
								id="alert-{alert.id}"
								name="alert-{alert.id}"
								checked={selectedIds.has(alert.id)}
								onchange={() => toggleSelection(alert.id)}
								aria-label="Select {alert.entityType} {alert.recordId}"
								class="h-4 w-4 shrink-0 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
							/>

							<!-- Expand Chevron -->
							<svg
								class="h-4 w-4 shrink-0 text-gray-400 transition-transform duration-200 {isExpanded ? 'rotate-90' : ''}"
								fill="none" stroke="currentColor" viewBox="0 0 24 24"
							>
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
							</svg>

							<!-- Record Name -->
							<span class="min-w-0 truncate font-medium text-gray-900">
								{alert.recordName || alert.recordId}
							</span>

							<!-- Entity Badge -->
							<span class="shrink-0 rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-600">
								{alert.entityType}
							</span>

							<!-- Confidence Badge -->
							<span class="shrink-0 rounded-full {badgeClass} px-2 py-0.5 text-xs font-medium">
								{formatConfidence(confidenceScore)} {alert.highestConfidence.toUpperCase()}
							</span>

							<!-- Match Count -->
							<span class="shrink-0 text-xs text-gray-500">
								{alert.totalMatchCount} {alert.totalMatchCount === 1 ? 'match' : 'matches'}
							</span>

							<!-- Spacer -->
							<div class="flex-1"></div>

							<!-- Inline Action Buttons -->
							<div class="flex shrink-0 items-center gap-1">
								<button
									onclick={(e) => { e.stopPropagation(); dismissAlert(alert); }}
									class="rounded px-2.5 py-1 text-xs font-medium text-gray-600 hover:bg-gray-200"
								>
									Dismiss
								</button>
								<button
									onclick={(e) => { e.stopPropagation(); quickMerge(alert); }}
									class="rounded bg-blue-600 px-2.5 py-1 text-xs font-medium text-white hover:bg-blue-700"
								>
									Quick Merge
								</button>
								<button
									onclick={(e) => { e.stopPropagation(); navigateToMergeWizard(alert); }}
									class="rounded border border-blue-600 px-2.5 py-1 text-xs font-medium text-blue-600 hover:bg-blue-50"
								>
									Merge
								</button>
							</div>
						</div>

						<!-- Expanded Match Details -->
						{#if isExpanded}
							<div class="border-t border-gray-100 bg-gray-50 px-4 py-3 pl-14">
								<h4 class="mb-2 text-xs font-medium uppercase tracking-wide text-gray-500">Matched Records</h4>
								<div class="space-y-2">
									{#each alert.matches as match}
										<div class="rounded-lg border border-gray-200 bg-white p-3">
											<div class="flex items-center gap-2">
												<span class="font-medium text-gray-900">
													{match.recordName || match.recordId}
												</span>
												<span class="text-xs text-gray-500">
													Match: {formatConfidence(match.matchResult?.score || 0)}
												</span>
											</div>
											{#if match.matchResult?.matchingFields?.length > 0}
												<div class="mt-1 flex flex-wrap gap-1">
													{#each match.matchResult?.matchingFields || [] as field}
														<span class="rounded bg-blue-100 px-2 py-0.5 text-xs text-blue-700">
															{field}
														</span>
													{/each}
												</div>
											{/if}
										</div>
									{/each}
								</div>
							</div>
						{/if}
					</div>
				{/each}
			</div>
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
