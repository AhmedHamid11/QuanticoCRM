<script lang="ts">
	import { onMount } from 'svelte';
	import { getConfidenceBadgeClass, formatConfidence, type PendingAlert, type DuplicateMatch } from '$lib/api/dedup';
	import { goto } from '$app/navigation';
	import { get, put, del } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';

	interface Props {
		alert: PendingAlert;
		entityType: string;
		currentRecordId: string;
		isBlockMode?: boolean;
		onClose: () => void;
		onDismiss: () => void;
		onMergeComplete?: () => void;
	}

	let {
		alert,
		entityType,
		currentRecordId,
		isBlockMode = false,
		onClose,
		onDismiss,
		onMergeComplete
	}: Props = $props();

	let overrideText = $state('');

	// Current record details
	let currentRecord = $state<Record<string, unknown> | null>(null);
	// Available matches (filtered to only those that still exist)
	let availableMatches = $state<DuplicateMatch[]>([]);
	// Selected duplicate for comparison (set after loading)
	let selectedMatchId = $state<string>('');
	// Duplicate record details
	let matchDetails = $state<Record<string, Record<string, unknown>>>({});
	let loadingDetails = $state(true);

	// Which record is selected as primary (the one to keep)
	let primaryRecordId = $state<string>(currentRecordId);

	// Field selections: which record's value to use for each field
	let fieldSelections = $state<Record<string, string>>({});

	// Merge mode vs view mode
	let mergeMode = $state(false);
	let merging = $state(false);

	// Default fields to compare (used if no merge display fields configured in matching rule)
	const defaultCompareFields = [
		'firstName', 'lastName', 'emailAddress', 'phoneNumber',
		'accountName', 'description', 'addressStreet', 'addressCity',
		'addressState', 'addressPostalCode', 'addressCountry'
	];

	// Use merge display fields from alert if available, else use defaults
	let compareFields = $derived(
		alert.mergeDisplayFields && alert.mergeDisplayFields.length > 0
			? alert.mergeDisplayFields
			: defaultCompareFields
	);

	// Fetch all record details when modal opens
	onMount(async () => {
		loadingDetails = true;
		const entityPlural = entityType.toLowerCase() + 's';

		try {
			// Fetch current record
			const current = await get<Record<string, unknown>>(`/${entityPlural}/${currentRecordId}`);
			currentRecord = current;

			// Fetch all match records in parallel
			const fetchPromises = alert.matches.map(async (match) => {
				try {
					const record = await get<Record<string, unknown>>(`/${entityPlural}/${match.recordId}`);
					return { id: match.recordId, record, match };
				} catch (error) {
					console.error(`Failed to fetch record ${match.recordId}:`, error);
					return { id: match.recordId, record: null, match };
				}
			});

			const results = await Promise.all(fetchPromises);
			const details: Record<string, Record<string, unknown>> = {};
			const validMatches: DuplicateMatch[] = [];

			for (const result of results) {
				if (result.record) {
					details[result.id] = result.record;
					validMatches.push(result.match);
				}
			}
			matchDetails = details;
			availableMatches = validMatches;

			// Set selected match to first available one
			if (validMatches.length > 0) {
				selectedMatchId = validMatches[0].recordId;
			}

			// Initialize field selections - prefer non-empty values
			initializeFieldSelections();
		} catch (error) {
			console.error('Failed to load record details:', error);
		} finally {
			loadingDetails = false;
		}
	});

	function initializeFieldSelections() {
		const selections: Record<string, string> = {};

		// If no match selected, default all fields to current record
		if (!selectedMatchId || !matchDetails[selectedMatchId]) {
			for (const field of compareFields) {
				selections[field] = currentRecordId;
			}
			fieldSelections = selections;
			return;
		}

		const otherRecord = matchDetails[selectedMatchId];

		for (const field of compareFields) {
			const currentVal = currentRecord?.[field];
			const otherVal = otherRecord?.[field];

			// Prefer non-empty values, default to current record
			if (currentVal && !otherVal) {
				selections[field] = currentRecordId;
			} else if (!currentVal && otherVal) {
				selections[field] = selectedMatchId;
			} else {
				// Both have values or both empty - default to primary
				selections[field] = primaryRecordId;
			}
		}
		fieldSelections = selections;
	}

	// Update field selections when primary changes
	$effect(() => {
		if (primaryRecordId && !mergeMode) {
			// Reset all to primary when switching primary record
			const selections: Record<string, string> = {};
			for (const field of compareFields) {
				selections[field] = primaryRecordId;
			}
			fieldSelections = selections;
		}
	});

	function getRecordName(record: Record<string, unknown> | null): string {
		if (!record) return 'Unknown';
		if (record.firstName || record.lastName) {
			return [record.firstName, record.lastName].filter(Boolean).join(' ');
		}
		if (record.name) return String(record.name);
		return String(record.id || 'Unknown').slice(0, 12) + '...';
	}

	function getFieldValue(record: Record<string, unknown> | null, field: string): string {
		if (!record) return '-';
		const value = record[field];
		if (value === null || value === undefined || value === '') return '-';
		return String(value);
	}

	function formatFieldLabel(field: string): string {
		return field.replace(/([A-Z])/g, ' $1').trim();
	}

	function handleViewRecord(recordId: string) {
		const entityPlural = entityType.toLowerCase() + 's';
		goto(`/${entityPlural}/${recordId}`);
		onClose();
	}

	async function handleMerge() {
		if (merging) return;
		merging = true;

		try {
			const entityPlural = entityType.toLowerCase() + 's';
			const secondaryId = primaryRecordId === currentRecordId ? selectedMatchId : currentRecordId;
			const primaryRecord = primaryRecordId === currentRecordId ? currentRecord : matchDetails[selectedMatchId];
			const secondaryRecord = primaryRecordId === currentRecordId ? matchDetails[selectedMatchId] : currentRecord;

			// Build merged data from field selections
			const mergedData: Record<string, unknown> = {};
			for (const field of compareFields) {
				const sourceId = fieldSelections[field];
				const sourceRecord = sourceId === currentRecordId ? currentRecord : matchDetails[selectedMatchId];
				if (sourceRecord && sourceRecord[field] !== undefined && sourceRecord[field] !== null && sourceRecord[field] !== '') {
					mergedData[field] = sourceRecord[field];
				}
			}

			// Update the primary record with merged data
			// Note: This is a simplified merge - full merge would handle related records, etc.
			await put(`/${entityPlural}/${primaryRecordId}`, mergedData);

			// Delete the secondary record
			await del(`/${entityPlural}/${secondaryId}`);

			toast.success(`Records merged. ${getRecordName(secondaryRecord)} was deleted.`);

			// Navigate to the primary record
			goto(`/${entityPlural}/${primaryRecordId}`);
			onClose();
			if (onMergeComplete) onMergeComplete();
		} catch (error) {
			console.error('Merge failed:', error);
			toast.error('Failed to merge records. Please try again.');
		} finally {
			merging = false;
		}
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			onClose();
		}
	}

	let canProceed = $derived(!isBlockMode || overrideText.toUpperCase() === 'DUPLICATE');
	let selectedMatch = $derived(availableMatches.find(m => m.recordId === selectedMatchId));
	let otherRecord = $derived(matchDetails[selectedMatchId]);
	let hasAvailableMatches = $derived(availableMatches.length > 0);
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
		class="bg-white rounded-lg shadow-xl max-w-4xl w-full max-h-[90vh] overflow-hidden flex flex-col"
		onclick={(e) => e.stopPropagation()}
		role="document"
	>
		<!-- Header -->
		<div class="px-6 py-4 border-b border-gray-200 bg-yellow-50 flex-shrink-0">
			<div class="flex items-center justify-between">
				<div>
					<h2 id="modal-title" class="text-lg font-medium text-yellow-800">
						{mergeMode ? 'Merge Records' : 'Potential Duplicate Found'}
					</h2>
					{#if selectedMatch}
						<p class="text-sm text-yellow-700 mt-1">
							<span class="font-medium">{selectedMatch.matchResult.confidenceTier.toUpperCase()}</span> confidence match
							({formatConfidence(selectedMatch.matchResult.score)})
						</p>
					{/if}
				</div>
				{#if availableMatches.length > 1}
					<select
						bind:value={selectedMatchId}
						onchange={() => initializeFieldSelections()}
						class="text-sm border border-gray-300 rounded px-2 py-1"
					>
						{#each availableMatches as match}
							<option value={match.recordId}>
								{matchDetails[match.recordId] ? getRecordName(matchDetails[match.recordId]) : match.recordId.slice(0, 12)}
							</option>
						{/each}
					</select>
				{/if}
			</div>
		</div>

		<!-- Body -->
		<div class="px-6 py-4 overflow-y-auto flex-1">
			{#if loadingDetails}
				<div class="text-center py-8 text-gray-500">Loading record details...</div>
			{:else if !hasAvailableMatches}
				<div class="text-center py-8">
					<p class="text-gray-600 mb-2">The matched records are no longer available.</p>
					<p class="text-sm text-gray-500">They may have been deleted or merged previously.</p>
				</div>
			{:else}
				<!-- Primary Selection -->
				{#if mergeMode}
					<div class="mb-4 p-3 bg-blue-50 rounded-lg">
						<p class="text-sm font-medium text-blue-800 mb-2">Select which record to keep as primary:</p>
						<div class="flex gap-4">
							<label class="flex items-center gap-2 cursor-pointer">
								<input
									type="radio"
									name="primary"
									value={currentRecordId}
									bind:group={primaryRecordId}
									class="text-blue-600"
								/>
								<span class="text-sm">{getRecordName(currentRecord)} (Current)</span>
							</label>
							<label class="flex items-center gap-2 cursor-pointer">
								<input
									type="radio"
									name="primary"
									value={selectedMatchId}
									bind:group={primaryRecordId}
									class="text-blue-600"
								/>
								<span class="text-sm">{getRecordName(otherRecord)}</span>
							</label>
						</div>
					</div>
				{/if}

				<!-- Side-by-side comparison -->
				<div class="border rounded-lg overflow-hidden">
					<!-- Header row -->
					<div class="grid grid-cols-3 bg-gray-100 text-sm font-medium">
						<div class="px-4 py-2 border-r border-gray-200">Field</div>
						<div class="px-4 py-2 border-r border-gray-200 {primaryRecordId === currentRecordId ? 'bg-green-100' : ''}">
							{getRecordName(currentRecord)}
							{#if primaryRecordId === currentRecordId}
								<span class="text-xs text-green-600 ml-1">(Primary)</span>
							{/if}
						</div>
						<div class="px-4 py-2 {primaryRecordId === selectedMatchId ? 'bg-green-100' : ''}">
							{getRecordName(otherRecord)}
							{#if primaryRecordId === selectedMatchId}
								<span class="text-xs text-green-600 ml-1">(Primary)</span>
							{/if}
						</div>
					</div>

					<!-- Field rows -->
					{#each compareFields as field}
						{@const currentVal = getFieldValue(currentRecord, field)}
						{@const otherVal = getFieldValue(otherRecord, field)}
						{@const hasConflict = currentVal !== '-' && otherVal !== '-' && currentVal !== otherVal}
						<div class="grid grid-cols-3 text-sm border-t border-gray-200 {hasConflict ? 'bg-yellow-50' : ''}">
							<div class="px-4 py-2 border-r border-gray-200 text-gray-600 capitalize">
								{formatFieldLabel(field)}
								{#if hasConflict}
									<span class="text-yellow-600 ml-1">*</span>
								{/if}
							</div>
							<div class="px-4 py-2 border-r border-gray-200 {fieldSelections[field] === currentRecordId && mergeMode ? 'bg-green-50 font-medium' : ''}">
								{#if mergeMode}
									<label class="flex items-center gap-2 cursor-pointer">
										<input
											type="radio"
											name={field}
											value={currentRecordId}
											bind:group={fieldSelections[field]}
											class="text-green-600"
										/>
										<span class="truncate" title={currentVal}>{currentVal}</span>
									</label>
								{:else}
									<span class="truncate block" title={currentVal}>{currentVal}</span>
								{/if}
							</div>
							<div class="px-4 py-2 {fieldSelections[field] === selectedMatchId && mergeMode ? 'bg-green-50 font-medium' : ''}">
								{#if mergeMode}
									<label class="flex items-center gap-2 cursor-pointer">
										<input
											type="radio"
											name={field}
											value={selectedMatchId}
											bind:group={fieldSelections[field]}
											class="text-green-600"
										/>
										<span class="truncate" title={otherVal}>{otherVal}</span>
									</label>
								{:else}
									<span class="truncate block" title={otherVal}>{otherVal}</span>
								{/if}
							</div>
						</div>
					{/each}
				</div>

				{#if mergeMode}
					<p class="text-xs text-gray-500 mt-2">
						<span class="text-yellow-600">*</span> Conflicting values highlighted — select the value you want to keep for each field
					</p>
				{/if}
			{/if}
		</div>

		<!-- Footer -->
		<div class="px-6 py-4 border-t border-gray-200 bg-gray-50 flex-shrink-0">
			{#if isBlockMode && !mergeMode}
				<p class="text-sm text-gray-600 mb-3">
					Block mode is enabled. Type "DUPLICATE" to keep both records.
				</p>
				<input
					type="text"
					bind:value={overrideText}
					placeholder='Type "DUPLICATE" to override'
					class="w-full px-3 py-2 border border-gray-300 rounded-md mb-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
				/>
			{/if}

			<div class="flex flex-wrap justify-between gap-3">
				<div class="flex gap-2">
					<button
						onclick={() => handleViewRecord(currentRecordId)}
						class="text-sm text-blue-600 hover:text-blue-800 hover:underline"
					>
						View Current
					</button>
					{#if hasAvailableMatches}
						<button
							onclick={() => handleViewRecord(selectedMatchId)}
							class="text-sm text-blue-600 hover:text-blue-800 hover:underline"
						>
							View Duplicate
						</button>
					{/if}
				</div>

				<div class="flex gap-3">
					<button
						onclick={onClose}
						class="px-4 py-2 text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50"
					>
						Cancel
					</button>

					{#if !hasAvailableMatches}
						<!-- All matches were deleted, just allow dismissing -->
						<button
							onclick={onDismiss}
							class="px-4 py-2 text-white bg-blue-600 rounded-md hover:bg-blue-700"
						>
							Dismiss Alert
						</button>
					{:else if mergeMode}
						<button
							onclick={() => mergeMode = false}
							class="px-4 py-2 text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50"
						>
							Back
						</button>
						<button
							onclick={handleMerge}
							disabled={merging}
							class="px-4 py-2 text-white bg-green-600 rounded-md hover:bg-green-700 disabled:opacity-50"
						>
							{merging ? 'Merging...' : 'Merge Records'}
						</button>
					{:else}
						<button
							onclick={onDismiss}
							class="px-4 py-2 text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50"
						>
							Not Duplicates
						</button>
						<button
							onclick={() => mergeMode = true}
							class="px-4 py-2 text-white bg-green-600 rounded-md hover:bg-green-700"
						>
							Merge Records
						</button>
						<button
							onclick={onDismiss}
							disabled={!canProceed}
							class="px-4 py-2 text-white bg-blue-600 rounded-md hover:bg-blue-700 disabled:opacity-50"
						>
							Keep Both
						</button>
					{/if}
				</div>
			</div>
		</div>
	</div>
</div>
