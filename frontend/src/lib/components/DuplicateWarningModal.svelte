<script lang="ts">
	import { onMount } from 'svelte';
	import { getConfidenceBadgeClass, formatConfidence, type PendingAlert, type DuplicateMatch } from '$lib/api/dedup';
	import { goto } from '$app/navigation';
	import { get } from '$lib/utils/api';

	interface Props {
		alert: PendingAlert;
		entityType: string;
		isBlockMode?: boolean;
		userCanMerge?: boolean;
		onClose: () => void;
		onDismiss: () => void;
		onCreateAnyway: (overrideText?: string) => void;
		onMerge?: (targetRecordId: string) => void;
	}

	let {
		alert,
		entityType,
		isBlockMode = false,
		userCanMerge = false,
		onClose,
		onDismiss,
		onCreateAnyway,
		onMerge
	}: Props = $props();

	let overrideText = $state('');
	let showAllMatches = $state(false);

	// Store fetched record details for each match
	let matchDetails = $state<Record<string, Record<string, unknown>>>({});
	let loadingDetails = $state(true);

	// Fetch details for all matches when modal opens
	onMount(async () => {
		loadingDetails = true;
		const entityPlural = entityType.toLowerCase() + 's';

		// Fetch all match records in parallel
		const fetchPromises = alert.matches.map(async (match) => {
			try {
				const record = await get<Record<string, unknown>>(`/${entityPlural}/${match.recordId}`);
				return { id: match.recordId, record };
			} catch (error) {
				console.error(`Failed to fetch record ${match.recordId}:`, error);
				return { id: match.recordId, record: null };
			}
		});

		const results = await Promise.all(fetchPromises);
		const details: Record<string, Record<string, unknown>> = {};
		for (const result of results) {
			if (result.record) {
				details[result.id] = result.record;
			}
		}
		matchDetails = details;
		loadingDetails = false;
	});

	// Helper to get display name for a record
	function getRecordDisplayName(match: DuplicateMatch): string {
		const details = matchDetails[match.recordId];
		if (details) {
			// Try common name patterns
			if (details.firstName || details.lastName) {
				return [details.firstName, details.lastName].filter(Boolean).join(' ');
			}
			if (details.name) {
				return String(details.name);
			}
		}
		return match.recordName || `Record: ${match.recordId.slice(0, 12)}...`;
	}

	// Helper to get key field values for display
	function getKeyFieldValues(match: DuplicateMatch): Array<{label: string, value: string}> {
		const details = matchDetails[match.recordId];
		if (!details) return [];

		const keyFields: Array<{label: string, value: string}> = [];

		// Show matching fields with their actual values
		for (const field of match.matchResult.matchingFields) {
			const value = details[field];
			if (value !== null && value !== undefined && value !== '') {
				keyFields.push({
					label: field.replace(/([A-Z])/g, ' $1').trim(),
					value: String(value)
				});
			}
		}

		// Also show some standard fields if not already included
		const standardFields = ['firstName', 'lastName', 'emailAddress', 'phoneNumber', 'accountName'];
		for (const field of standardFields) {
			if (!match.matchResult.matchingFields.includes(field)) {
				const value = details[field];
				if (value !== null && value !== undefined && value !== '') {
					keyFields.push({
						label: field.replace(/([A-Z])/g, ' $1').trim(),
						value: String(value)
					});
				}
			}
		}

		return keyFields.slice(0, 6); // Limit to 6 fields
	}

	// Matches to display (top 3 or all)
	let displayedMatches = $derived(
		showAllMatches ? alert.matches : alert.matches.slice(0, 3)
	);

	let canProceed = $derived(
		!isBlockMode || overrideText.toUpperCase() === 'DUPLICATE'
	);

	function handleViewRecord(recordId: string) {
		// Navigate to the record
		const entityPlural = entityType.toLowerCase() + 's';
		goto(`/${entityPlural}/${recordId}`);
		onClose();
	}

	function handleMerge(targetRecordId: string) {
		if (onMerge) {
			onMerge(targetRecordId);
		} else {
			// Default: navigate to merge UI (Phase 13 will implement this)
			const entityPlural = entityType.toLowerCase() + 's';
			goto(`/${entityPlural}/${alert.recordId}/merge?target=${targetRecordId}`);
			onClose();
		}
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			onClose();
		}
	}
</script>

<svelte:window on:keydown={handleKeydown} />

<!-- Backdrop -->
<div
	class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4"
	onclick={onClose}
	onkeydown={(e) => e.key === 'Enter' && onClose()}
	role="dialog"
	aria-modal="true"
	aria-labelledby="modal-title"
	tabindex="-1"
>
	<!-- Modal -->
	<div
		class="bg-white rounded-lg shadow-xl max-w-2xl w-full max-h-[90vh] overflow-hidden flex flex-col"
		onclick={(e) => e.stopPropagation()}
		role="document"
	>
		<!-- Header -->
		<div class="px-6 py-4 border-b border-gray-200 bg-yellow-50 flex-shrink-0">
			<h2 id="modal-title" class="text-lg font-medium text-yellow-800">
				Potential Duplicates Found
			</h2>
			<p class="text-sm text-yellow-700 mt-1">
				{alert.totalMatchCount} potential match{alert.totalMatchCount !== 1 ? 'es' : ''} found for this record
			</p>
		</div>

		<!-- Body -->
		<div class="px-6 py-4 space-y-4 overflow-y-auto flex-1">
			{#if loadingDetails}
				<div class="text-center py-4 text-gray-500">Loading match details...</div>
			{:else}
				{#each displayedMatches as match (match.recordId)}
					<div class="border rounded-lg p-4 hover:bg-gray-50 transition-colors">
						<!-- Match header with confidence badge -->
						<div class="flex items-center justify-between mb-3">
							<span class="text-base font-medium text-gray-900">
								{getRecordDisplayName(match)}
							</span>
							<span class="px-2 py-1 text-xs font-medium rounded border {getConfidenceBadgeClass(match.matchResult.confidenceTier)}">
								{match.matchResult.confidenceTier.toUpperCase()} - {formatConfidence(match.matchResult.score)}
							</span>
						</div>

						<!-- Record details -->
						{#if getKeyFieldValues(match).length > 0}
							<div class="grid grid-cols-2 gap-x-4 gap-y-1 text-sm mb-3 bg-gray-50 rounded p-3">
								{#each getKeyFieldValues(match) as field}
									<div class="flex flex-col">
										<span class="text-xs text-gray-500 capitalize">{field.label}</span>
										<span class="text-gray-900 truncate" title={field.value}>{field.value}</span>
									</div>
								{/each}
							</div>
						{/if}

						<!-- Matching fields with scores -->
						<div class="text-xs text-gray-500 mb-2">Match scores:</div>
						<div class="grid grid-cols-2 gap-2 text-sm mb-3">
							{#each Object.entries(match.matchResult.fieldScores) as [field, score]}
								<div class="flex justify-between px-2 py-1 rounded {score >= 0.85 ? 'bg-yellow-50' : ''}">
									<span class="text-gray-600 capitalize">{field.replace(/([A-Z])/g, ' $1').trim()}:</span>
									<span class="font-medium {score >= 0.95 ? 'text-red-600' : score >= 0.85 ? 'text-yellow-600' : 'text-gray-600'}">
										{formatConfidence(score)}
									</span>
								</div>
							{/each}
						</div>

						<!-- Match actions -->
						<div class="flex gap-3 pt-2 border-t border-gray-100">
							<button
								onclick={() => handleViewRecord(match.recordId)}
								class="text-sm text-blue-600 hover:text-blue-800 hover:underline"
							>
								View Record
							</button>
							{#if userCanMerge}
								<button
									onclick={() => handleMerge(match.recordId)}
									class="text-sm text-green-600 hover:text-green-800 hover:underline"
								>
									Merge with this
								</button>
							{/if}
						</div>
					</div>
				{/each}
			{/if}

			<!-- Show more indicator -->
			{#if alert.totalMatchCount > 3 && !showAllMatches}
				<button
					onclick={() => showAllMatches = true}
					class="w-full text-center py-2 text-sm text-blue-600 hover:text-blue-800 hover:underline"
				>
					Show {alert.totalMatchCount - 3} more match{alert.totalMatchCount - 3 !== 1 ? 'es' : ''}...
				</button>
			{/if}
		</div>

		<!-- Footer -->
		<div class="px-6 py-4 border-t border-gray-200 bg-gray-50 flex-shrink-0">
			<!-- Block mode override input -->
			{#if isBlockMode}
				<p class="text-sm text-gray-600 mb-3">
					Block mode is enabled. Type "DUPLICATE" to proceed anyway.
				</p>
				<input
					type="text"
					bind:value={overrideText}
					placeholder='Type "DUPLICATE" to override'
					class="w-full px-3 py-2 border border-gray-300 rounded-md mb-3 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
				/>
			{/if}

			<!-- Action buttons -->
			<div class="flex flex-wrap justify-end gap-3">
				<button
					onclick={onClose}
					class="px-4 py-2 text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-gray-500"
				>
					Cancel
				</button>

				<button
					onclick={onDismiss}
					class="px-4 py-2 text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-gray-500"
				>
					Dismiss Alert
				</button>

				<button
					onclick={() => onCreateAnyway(isBlockMode ? overrideText : undefined)}
					disabled={!canProceed}
					class="px-4 py-2 text-white bg-blue-600 rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					Keep This Record
				</button>
			</div>
		</div>
	</div>
</div>
