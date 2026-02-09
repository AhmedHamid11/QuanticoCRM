<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { mergePreview, mergeExecute, mergeUndo, type MergePreview, type FieldDef } from '$lib/api/data-quality';
	import { toast } from '$lib/stores/toast.svelte';

	// Get params from URL
	const groupId = $page.params.groupId;
	const entityTypeParam = $page.url.searchParams.get('entityType') || '';
	const recordIdsParam = $page.url.searchParams.get('recordIds') || '';

	// Parse record IDs
	const recordIds = recordIdsParam.split(',').filter(Boolean);

	// State
	let preview = $state<MergePreview | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let survivorId = $state<string>('');
	let fieldSelections = $state<Record<string, string>>({}); // fieldName -> recordId
	let merging = $state(false);

	// System fields to skip
	const SKIP_FIELDS = ['id', 'org_id', 'orgId', 'created_at', 'createdAt', 'updated_at', 'updatedAt', 'archived_at', 'archivedAt', 'archived_by', 'archivedBy', 'merge_survivor_id', 'mergeSurvivorId'];

	onMount(async () => {
		if (!entityTypeParam || recordIds.length < 2) {
			error = 'Invalid merge parameters. Please navigate from the review queue.';
			loading = false;
			return;
		}

		try {
			// Call merge preview API
			const previewData = await mergePreview({
				recordIds,
				entityType: entityTypeParam
			});

			preview = previewData;
			survivorId = previewData.suggestedSurvivorId;

			// Auto-select field values
			initializeFieldSelections();
			loading = false;
		} catch (err: any) {
			error = err.message || 'Failed to load merge preview';
			loading = false;
		}
	});

	function initializeFieldSelections() {
		if (!preview) return;

		const selections: Record<string, string> = {};

		// Get the survivor record
		const survivorRecord = preview.records.find((r) => r.id === survivorId);
		if (!survivorRecord) return;

		// For each field in the preview
		preview.fields.forEach((field) => {
			// Skip system fields
			if (SKIP_FIELDS.includes(field.name)) return;

			// Check if survivor has this field filled
			const survivorValue = survivorRecord[field.name];

			if (survivorValue !== null && survivorValue !== undefined && survivorValue !== '') {
				// Survivor has value, use it
				selections[field.name] = survivorId;
			} else {
				// Survivor field is empty, find first record with non-empty value
				const recordWithValue = preview.records.find((r) => {
					const val = r[field.name];
					return val !== null && val !== undefined && val !== '';
				});

				if (recordWithValue) {
					selections[field.name] = recordWithValue.id;
				} else {
					// All empty, default to survivor
					selections[field.name] = survivorId;
				}
			}
		});

		fieldSelections = selections;
	}

	function getRecordName(record: Record<string, any>): string {
		// Try firstName + lastName (contacts, leads)
		const first = record.firstName || record.first_name || '';
		const last = record.lastName || record.last_name || '';
		const fullName = `${first} ${last}`.trim();
		if (fullName) return fullName;
		// Try name field (accounts, opportunities, tasks)
		if (record.name) return record.name;
		if (record.fullName) return record.fullName;
		if (record.title) return record.title;
		if (record.email) return record.email;
		return record.id?.substring(0, 8) || 'Unknown';
	}

	function formatFieldValue(value: any): string {
		if (value === null || value === undefined || value === '') {
			return '(empty)';
		}
		if (typeof value === 'boolean') {
			return value ? 'Yes' : 'No';
		}
		if (typeof value === 'object') {
			return JSON.stringify(value);
		}
		return String(value);
	}

	// For lookup fields (e.g. accountId), display the related name (accountName) if available
	function getDisplayValue(record: Record<string, any>, field: FieldDef): string {
		if (field.type === 'lookup' && field.name.endsWith('Id')) {
			const nameField = field.name.slice(0, -2) + 'Name';
			if (record[nameField]) return String(record[nameField]);
		}
		const value = record[field.name];
		// Format datetime fields
		if (field.type === 'datetime' || field.name.endsWith('At') || field.name.endsWith('_at')) {
			if (typeof value === 'string' && value.includes('T')) {
				try {
					return new Date(value).toLocaleDateString('en-US', {
						year: 'numeric', month: 'short', day: 'numeric',
						hour: '2-digit', minute: '2-digit'
					});
				} catch { /* fall through */ }
			}
		}
		return formatFieldValue(value);
	}

	function valuesAreDifferent(fieldName: string): boolean {
		if (!preview) return false;

		const values = preview.records.map((r) => r[fieldName]);
		const firstValue = values[0];

		return values.some((v) => {
			// Handle null/undefined/empty string as equivalent
			const firstEmpty = firstValue === null || firstValue === undefined || firstValue === '';
			const vEmpty = v === null || v === undefined || v === '';

			if (firstEmpty && vEmpty) return false;
			if (firstEmpty !== vEmpty) return true;

			return v !== firstValue;
		});
	}

	async function handleMerge() {
		if (!preview || !survivorId) {
			toast.error('Invalid merge state');
			return;
		}

		merging = true;

		try {
			// Build merged fields from selections
			const mergedFields: Record<string, any> = {};

			for (const [fieldName, selectedRecordId] of Object.entries(fieldSelections)) {
				const record = preview.records.find((r) => r.id === selectedRecordId);
				if (record) {
					mergedFields[fieldName] = record[fieldName];
				}
			}

			// Execute merge
			const result = await mergeExecute({
				survivorId,
				duplicateIds: preview.records.filter((r) => r.id !== survivorId).map((r) => r.id),
				mergedFields: mergedFields,
				entityType: entityTypeParam
			});

			// Show success toast with undo functionality
			// Note: Toast system doesn't support actions, so we'll just show success
			// User can undo from merge history page
			toast.success(`Records merged successfully (Snapshot: ${result.snapshotId.substring(0, 8)}). Visit Merge History to undo within 30 days.`);

			// Return to review queue
			goto('/admin/data-quality/review-queue');
		} catch (err: any) {
			toast.error(`Failed to merge records: ${err.message}`);
			merging = false;
		}
	}

	function handleCancel() {
		goto('/admin/data-quality/review-queue');
	}

	function getCompletenessPercentage(recordId: string): number {
		if (!preview?.completenessScores) return 0;
		return Math.round((preview.completenessScores[recordId] || 0) * 100);
	}

	function getRelatedRecordCount(recordId: string): number {
		if (!preview?.relatedRecordCounts) return 0;
		return preview.relatedRecordCounts
			.filter((r) => r.recordId === recordId)
			.reduce((sum, r) => sum + r.count, 0);
	}

	// Group related records by entity type
	function getRelatedRecordsByEntity(): Array<{
		entityType: string;
		entityLabel: string;
		items: Array<{ recordId: string; count: number; records: any[] }>;
	}> {
		if (!preview?.relatedRecordCounts) return [];

		const grouped = new Map<string, {
			entityType: string;
			entityLabel: string;
			items: Array<{ recordId: string; count: number; records: any[] }>;
		}>();

		preview.relatedRecordCounts.forEach((rel) => {
			if (!grouped.has(rel.entityType)) {
				grouped.set(rel.entityType, {
					entityType: rel.entityType,
					entityLabel: rel.entityLabel,
					items: []
				});
			}

			grouped.get(rel.entityType)!.items.push({
				recordId: rel.recordId,
				count: rel.count,
				records: rel.records || []
			});
		});

		return Array.from(grouped.values());
	}
</script>

<div class="space-y-6">
	<!-- Header -->
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold text-gray-900">Merge Records</h1>
			<p class="mt-1 text-sm text-gray-500">{entityTypeParam}</p>
		</div>
		<a
			href="/admin/data-quality/review-queue"
			class="text-sm text-blue-600 hover:text-blue-800"
		>
			← Back to Review Queue
		</a>
	</div>

	{#if loading}
		<div class="bg-white rounded-lg shadow p-12 text-center">
			<div class="animate-spin h-8 w-8 border-4 border-blue-500 border-t-transparent rounded-full mx-auto"></div>
			<p class="mt-4 text-gray-600">Loading merge preview...</p>
		</div>
	{:else if error}
		<div class="bg-red-50 border border-red-200 rounded-lg p-6">
			<h3 class="text-red-800 font-medium">Error</h3>
			<p class="text-red-700 mt-2">{error}</p>
			<button
				onclick={handleCancel}
				class="mt-4 px-4 py-2 bg-red-600 text-white rounded hover:bg-red-700"
			>
				Return to Queue
			</button>
		</div>
	{:else if preview}
		<!-- Section 1: Survivor Selection -->
		<div class="bg-white rounded-lg shadow p-6">
			<h2 class="text-lg font-semibold text-gray-900 mb-4">1. Select Surviving Record</h2>
			<p class="text-sm text-gray-600 mb-4">
				Choose which record will remain after the merge. All data from other records will be transferred to this record.
			</p>

			<div class="space-y-3">
				{#each preview.records as record}
					<label class="flex items-start gap-4 p-4 border rounded-lg cursor-pointer hover:bg-gray-50 transition {survivorId === record.id ? 'border-blue-500 bg-blue-50' : 'border-gray-200'}">
						<input
							type="radio"
							name="survivor"
							value={record.id}
							bind:group={survivorId}
							class="mt-1"
						/>
						<div class="flex-1">
							<div class="font-medium text-gray-900">{getRecordName(record)}</div>
							<div class="mt-2 flex items-center gap-6 text-sm">
								<div>
									<span class="text-gray-600">Completeness:</span>
									<div class="mt-1 w-32 bg-gray-200 rounded-full h-2">
										<div
											class="bg-blue-500 h-2 rounded-full"
											style="width: {getCompletenessPercentage(record.id)}%"
										></div>
									</div>
									<span class="text-xs text-gray-500">{getCompletenessPercentage(record.id)}%</span>
								</div>
								<div>
									<span class="text-gray-600">Related Records:</span>
									<span class="ml-2 font-medium">{getRelatedRecordCount(record.id)}</span>
								</div>
							</div>
						</div>
					</label>
				{/each}
			</div>
		</div>

		<!-- Section 2: Field Comparison -->
		<div class="bg-white rounded-lg shadow p-6">
			<h2 class="text-lg font-semibold text-gray-900 mb-4">2. Select Field Values</h2>
			<p class="text-sm text-gray-600 mb-4">
				Choose which value to keep for each field. Differences are highlighted in yellow.
			</p>

			<div class="overflow-x-auto">
				<table class="min-w-full divide-y divide-gray-200">
					<thead>
						<tr>
							<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider bg-gray-50">
								Field
							</th>
							{#each preview.records as record}
								<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider bg-gray-50">
									{getRecordName(record)}
									{#if record.id === survivorId}
										<span class="ml-1 text-blue-600">(Survivor)</span>
									{/if}
								</th>
							{/each}
						</tr>
					</thead>
					<tbody class="bg-white divide-y divide-gray-200">
						{#each preview.fields as field}
							{#if !SKIP_FIELDS.includes(field.name)}
								<tr class="{valuesAreDifferent(field.name) ? 'bg-yellow-50' : ''}">
									<td class="px-4 py-3 text-sm font-medium text-gray-900">
										{field.label}
									</td>
									{#each preview.records as record}
										<td class="px-4 py-3">
											<label class="flex items-start gap-2 cursor-pointer">
												<input
													type="radio"
													name="field-{field.name}"
													value={record.id}
													bind:group={fieldSelections[field.name]}
													class="mt-1"
												/>
												<span class="text-sm text-gray-700">
													{getDisplayValue(record, field)}
												</span>
											</label>
										</td>
									{/each}
								</tr>
							{/if}
						{/each}
					</tbody>
				</table>
			</div>
		</div>

		<!-- Section 3: Related Records -->
		<div class="bg-white rounded-lg shadow p-6">
			<h2 class="text-lg font-semibold text-gray-900 mb-4">3. Related Records</h2>
			<p class="text-sm text-gray-600 mb-4">
				These records will be transferred to the surviving record.
			</p>

			{#if getRelatedRecordsByEntity().length === 0}
				<p class="text-gray-500 italic">No related records found.</p>
			{:else}
				<div class="space-y-4">
					{#each getRelatedRecordsByEntity() as relGroup}
						<div class="border rounded-lg p-4">
							<h3 class="font-medium text-gray-900 mb-2">
								{relGroup.entityLabel}
								<span class="text-gray-500">
									({relGroup.items.reduce((sum, item) => sum + item.count, 0)} records)
								</span>
							</h3>
							<div class="space-y-2">
								{#each relGroup.items as item}
									{@const sourceRecord = preview.records.find(r => r.id === item.recordId)}
									<div class="text-sm text-gray-600">
										<span class="font-medium">{item.count} records</span>
										from {getRecordName(sourceRecord || {})}
									</div>
								{/each}
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</div>

		<!-- Section 4: Confirmation -->
		<div class="bg-white rounded-lg shadow p-6">
			<h2 class="text-lg font-semibold text-gray-900 mb-4">4. Confirm Merge</h2>

			<div class="bg-blue-50 border border-blue-200 rounded-lg p-4 mb-4">
				<p class="text-sm text-gray-700">
					Merging <strong>{preview.records.length} records</strong> into
					<strong>{getRecordName(preview.records.find(r => r.id === survivorId) || {})}</strong>.
					{#if getRelatedRecordsByEntity().length > 0}
						<strong>{getRelatedRecordsByEntity().reduce((sum, g) => sum + g.items.reduce((s, i) => s + i.count, 0), 0)} related records</strong> will be transferred.
					{/if}
				</p>
			</div>

			<!-- Warning for data loss -->
			{#if preview.records.some(r => r.id !== survivorId)}
				<div class="bg-yellow-50 border border-yellow-200 rounded-lg p-4 mb-4">
					<p class="text-sm text-yellow-800">
						<strong>Warning:</strong> Non-selected field values from duplicate records will be archived.
						You can undo this merge within 30 days.
					</p>
				</div>
			{/if}

			<div class="flex gap-4">
				<button
					onclick={handleMerge}
					disabled={merging}
					class="flex-1 px-6 py-3 bg-blue-600 text-white font-medium rounded-lg hover:bg-blue-700 disabled:bg-gray-400 disabled:cursor-not-allowed transition"
				>
					{merging ? 'Merging...' : 'Merge Records'}
				</button>
				<button
					onclick={handleCancel}
					disabled={merging}
					class="px-6 py-3 bg-white border border-gray-300 text-gray-700 font-medium rounded-lg hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed transition"
				>
					Cancel
				</button>
			</div>
		</div>
	{/if}
</div>
