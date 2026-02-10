<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { get, put } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';

	interface MirrorSourceField {
		id?: string;
		mirrorId?: string;
		fieldName: string;
		fieldType: 'text' | 'number' | 'date' | 'boolean' | 'email' | 'phone';
		isRequired: boolean;
		description: string;
		mapField: string | null;
		sortOrder?: number;
	}

	interface Mirror {
		id: string;
		orgId: string;
		name: string;
		targetEntity: string;
		uniqueKeyField: string;
		unmappedFieldMode: 'strict' | 'flexible';
		rateLimit: number;
		isActive: boolean;
		sourceFields: MirrorSourceField[];
		createdAt: string;
		updatedAt: string;
	}

	interface Entity {
		name: string;
		displayName?: string;
	}

	interface FieldDef {
		name: string;
		label: string;
		type: string;
		isRequired?: boolean;
	}

	interface RecordError {
		index: number;
		uniqueKey: string;
		field: string;
		message: string;
		code: string;
	}

	interface IngestJob {
		id: string;
		orgId: string;
		mirrorId: string;
		keyId: string;
		status: 'accepted' | 'processing' | 'complete' | 'partial' | 'failed';
		recordsReceived: number;
		recordsProcessed: number;
		recordsPromoted: number;
		recordsSkipped: number;
		recordsFailed: number;
		errors: RecordError[];
		warnings: string[];
		startedAt?: string;
		completedAt?: string;
		createdAt: string;
		updatedAt: string;
	}

	// Mirror ID from URL params
	const mirrorId = $derived($page.params.id);

	// State
	let mirror = $state<Mirror | null>(null);
	let entities = $state<Entity[]>([]);
	let targetFields = $state<FieldDef[]>([]);
	let loading = $state(true);
	let savingSettings = $state(false);
	let savingSourceFields = $state(false);
	let savingMappings = $state(false);

	// Job history state
	let jobs = $state<IngestJob[]>([]);
	let loadingJobs = $state(false);
	let expandedJobs = $state<Set<string>>(new Set());
	let refreshInterval: number | null = null;

	// Local editing state for settings
	let editedSettings = $state({
		name: '',
		targetEntity: '',
		uniqueKeyField: '',
		unmappedFieldMode: 'flexible' as 'strict' | 'flexible',
		rateLimit: 500
	});

	// Local editing state for source fields
	let editedSourceFields = $state<MirrorSourceField[]>([]);
	let showAddFieldForm = $state(false);
	let editingFieldIndex = $state<number | null>(null);
	let newField = $state<MirrorSourceField>({
		fieldName: '',
		fieldType: 'text',
		isRequired: false,
		description: '',
		mapField: null
	});

	// Computed values
	const mappedFieldsCount = $derived(
		editedSourceFields.filter((f) => f.mapField !== null && f.mapField !== '').length
	);
	const totalFieldsCount = $derived(editedSourceFields.length);

	async function loadMirror() {
		try {
			loading = true;
			const response = await get<Mirror>(`/admin/mirrors/${mirrorId}`);
			mirror = response;

			// Initialize edited state
			editedSettings = {
				name: response.name,
				targetEntity: response.targetEntity,
				uniqueKeyField: response.uniqueKeyField,
				unmappedFieldMode: response.unmappedFieldMode,
				rateLimit: response.rateLimit
			};
			editedSourceFields = [...(response.sourceFields || [])];
		} catch (err) {
			toast.error('Failed to load mirror');
		} finally {
			loading = false;
		}
	}

	async function loadEntities() {
		try {
			const response = await get<Entity[]>('/admin/entities');
			entities = response || [];
		} catch (err) {
			toast.error('Failed to load entities');
		}
	}

	async function loadTargetFields(entityName: string) {
		if (!entityName) {
			targetFields = [];
			return;
		}

		try {
			const response = await get<FieldDef[]>(`/admin/entities/${entityName}/fields`);
			targetFields = response || [];
		} catch (err) {
			toast.error('Failed to load target fields');
			targetFields = [];
		}
	}

	async function saveSettings() {
		if (!editedSettings.name.trim()) {
			toast.error('Mirror name is required');
			return;
		}
		if (!editedSettings.targetEntity) {
			toast.error('Target entity is required');
			return;
		}
		if (!editedSettings.uniqueKeyField.trim()) {
			toast.error('Unique key field is required');
			return;
		}

		try {
			savingSettings = true;
			await put(`/admin/mirrors/${mirrorId}`, {
				name: editedSettings.name.trim(),
				targetEntity: editedSettings.targetEntity,
				uniqueKeyField: editedSettings.uniqueKeyField.trim(),
				unmappedFieldMode: editedSettings.unmappedFieldMode,
				rateLimit: editedSettings.rateLimit
			});
			toast.success('Settings saved');
			await loadMirror();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to save settings');
		} finally {
			savingSettings = false;
		}
	}

	async function saveSourceFields() {
		// Validate all fields have names
		for (const field of editedSourceFields) {
			if (!field.fieldName.trim()) {
				toast.error('All source fields must have a name');
				return;
			}
		}

		try {
			savingSourceFields = true;
			// Send all source fields (backend will replace them)
			await put(`/admin/mirrors/${mirrorId}`, {
				sourceFields: editedSourceFields.map((f) => ({
					fieldName: f.fieldName.trim(),
					fieldType: f.fieldType,
					isRequired: f.isRequired,
					description: f.description.trim(),
					mapField: f.mapField
				}))
			});
			toast.success('Source fields saved');
			await loadMirror();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to save source fields');
		} finally {
			savingSourceFields = false;
		}
	}

	async function saveMappings() {
		try {
			savingMappings = true;
			// Send all source fields with updated mapField values
			await put(`/admin/mirrors/${mirrorId}`, {
				sourceFields: editedSourceFields.map((f) => ({
					fieldName: f.fieldName,
					fieldType: f.fieldType,
					isRequired: f.isRequired,
					description: f.description,
					mapField: f.mapField
				}))
			});
			toast.success('Field mappings saved');
			await loadMirror();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to save mappings');
		} finally {
			savingMappings = false;
		}
	}

	function addFieldRow() {
		showAddFieldForm = true;
		newField = {
			fieldName: '',
			fieldType: 'text',
			isRequired: false,
			description: '',
			mapField: null
		};
	}

	function saveNewField() {
		if (!newField.fieldName.trim()) {
			toast.error('Field name is required');
			return;
		}

		if (editingFieldIndex !== null) {
			// Update existing field
			editedSourceFields[editingFieldIndex] = { ...newField };
			editingFieldIndex = null;
		} else {
			// Add new field
			editedSourceFields = [...editedSourceFields, { ...newField }];
		}

		showAddFieldForm = false;
		newField = {
			fieldName: '',
			fieldType: 'text',
			isRequired: false,
			description: '',
			mapField: null
		};
	}

	function cancelFieldEdit() {
		showAddFieldForm = false;
		editingFieldIndex = null;
		newField = {
			fieldName: '',
			fieldType: 'text',
			isRequired: false,
			description: '',
			mapField: null
		};
	}

	function editField(index: number) {
		editingFieldIndex = index;
		newField = { ...editedSourceFields[index] };
		showAddFieldForm = true;
	}

	function removeField(index: number) {
		editedSourceFields = editedSourceFields.filter((_, i) => i !== index);
	}

	function updateMapping(index: number, targetFieldName: string) {
		editedSourceFields[index] = {
			...editedSourceFields[index],
			mapField: targetFieldName === '' ? null : targetFieldName
		};
	}

	function getEntityDisplayName(entityName: string): string {
		const entity = entities.find((e) => e.name === entityName);
		return entity?.displayName || entityName;
	}

	function getFieldTypeBadgeColor(fieldType: string): string {
		switch (fieldType) {
			case 'text':
				return 'bg-blue-100 text-blue-800';
			case 'number':
				return 'bg-green-100 text-green-800';
			case 'date':
				return 'bg-purple-100 text-purple-800';
			case 'boolean':
				return 'bg-pink-100 text-pink-800';
			case 'email':
				return 'bg-indigo-100 text-indigo-800';
			case 'phone':
				return 'bg-teal-100 text-teal-800';
			default:
				return 'bg-gray-100 text-gray-800';
		}
	}

	async function loadJobs() {
		try {
			loadingJobs = true;
			const response = await get<{ jobs: IngestJob[]; total: number }>(
				`/admin/mirrors/${mirrorId}/jobs?limit=50`
			);
			jobs = response.jobs || [];
		} catch (err) {
			toast.error('Failed to load job history');
		} finally {
			loadingJobs = false;
		}
	}

	function toggleJobExpanded(jobId: string) {
		const newExpanded = new Set(expandedJobs);
		if (newExpanded.has(jobId)) {
			newExpanded.delete(jobId);
		} else {
			newExpanded.add(jobId);
		}
		expandedJobs = newExpanded;
	}

	function getStatusBadgeColor(status: string): string {
		switch (status) {
			case 'complete':
				return 'bg-green-100 text-green-800';
			case 'partial':
				return 'bg-yellow-100 text-yellow-800';
			case 'failed':
				return 'bg-red-100 text-red-800';
			case 'processing':
				return 'bg-blue-100 text-blue-800 animate-pulse';
			case 'accepted':
				return 'bg-gray-100 text-gray-800';
			default:
				return 'bg-gray-100 text-gray-800';
		}
	}

	function formatDuration(startedAt?: string, completedAt?: string): string {
		if (!startedAt || !completedAt) return '-';
		const start = new Date(startedAt).getTime();
		const end = new Date(completedAt).getTime();
		const diffMs = end - start;
		const diffSec = Math.floor(diffMs / 1000);

		if (diffSec < 60) {
			return `${diffSec}s`;
		}
		const mins = Math.floor(diffSec / 60);
		const secs = diffSec % 60;
		return `${mins}m ${secs}s`;
	}

	function formatTimestamp(timestamp: string): string {
		const date = new Date(timestamp);
		const now = new Date();
		const diffMs = now.getTime() - date.getTime();
		const diffMin = Math.floor(diffMs / 60000);

		if (diffMin < 60) {
			return `${diffMin} min ago`;
		}
		if (diffMin < 1440) {
			const hours = Math.floor(diffMin / 60);
			return `${hours} hour${hours > 1 ? 's' : ''} ago`;
		}

		// Format as "Feb 10, 2:30 PM"
		return date.toLocaleString('en-US', {
			month: 'short',
			day: 'numeric',
			hour: 'numeric',
			minute: '2-digit',
			hour12: true
		});
	}

	function setupAutoRefresh() {
		// Clear existing interval if any
		if (refreshInterval !== null) {
			clearInterval(refreshInterval);
			refreshInterval = null;
		}

		// Check if any jobs are in progress
		const hasActiveJobs = jobs.some((job) => job.status === 'accepted' || job.status === 'processing');

		if (hasActiveJobs) {
			// Auto-refresh every 30 seconds
			refreshInterval = setInterval(() => {
				loadJobs();
			}, 30000) as unknown as number;
		}
	}

	// Re-fetch target fields when target entity changes
	$effect(() => {
		if (editedSettings.targetEntity) {
			loadTargetFields(editedSettings.targetEntity);
		}
	});

	// Setup auto-refresh when jobs change
	$effect(() => {
		if (jobs.length > 0) {
			setupAutoRefresh();
		}
	});

	onMount(async () => {
		await loadMirror();
		await loadEntities();
		await loadJobs();
	});

	onDestroy(() => {
		if (refreshInterval !== null) {
			clearInterval(refreshInterval);
		}
	});
</script>

<div class="space-y-6">
	<!-- Header -->
	<div class="flex items-center justify-between">
		<div class="flex items-center gap-4">
			<div>
				<div class="flex items-center gap-3">
					<h1 class="text-2xl font-bold text-gray-900">{mirror?.name || 'Loading...'}</h1>
					{#if mirror}
						{#if mirror.isActive}
							<span
								class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800"
							>
								Active
							</span>
						{:else}
							<span
								class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800"
							>
								Inactive
							</span>
						{/if}
					{/if}
				</div>
				<p class="mt-1 text-sm text-gray-500">
					{#if mirror}
						Target: {getEntityDisplayName(mirror.targetEntity)}
					{/if}
				</p>
			</div>
		</div>
		<a
			href="/admin/mirrors"
			class="inline-flex items-center px-4 py-2 border border-gray-300 text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50"
		>
			Back to Mirrors
		</a>
	</div>

	{#if loading}
		<div class="bg-white shadow rounded-lg p-6">
			<div class="animate-pulse space-y-4">
				<div class="h-4 bg-gray-200 rounded w-1/4"></div>
				<div class="h-10 bg-gray-200 rounded w-1/2"></div>
			</div>
		</div>
	{:else if !mirror}
		<div class="bg-white shadow rounded-lg p-6">
			<p class="text-gray-500">Mirror not found</p>
		</div>
	{:else}
		<!-- Section 1: Mirror Settings -->
		<div class="bg-white shadow rounded-lg p-6">
			<h2 class="text-lg font-medium text-gray-900 mb-4">Mirror Settings</h2>

			<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
				<div>
					<label for="name" class="block text-sm font-medium text-gray-700 mb-1">
						Mirror Name <span class="text-red-500">*</span>
					</label>
					<input
						type="text"
						id="name"
						bind:value={editedSettings.name}
						class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
					/>
				</div>

				<div>
					<label for="targetEntity" class="block text-sm font-medium text-gray-700 mb-1">
						Target Entity <span class="text-red-500">*</span>
					</label>
					<select
						id="targetEntity"
						bind:value={editedSettings.targetEntity}
						class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
					>
						<option value="">Select entity...</option>
						{#each entities as entity}
							<option value={entity.name}>{entity.displayName || entity.name}</option>
						{/each}
					</select>
				</div>

				<div>
					<label for="uniqueKeyField" class="block text-sm font-medium text-gray-700 mb-1">
						Unique Key Field <span class="text-red-500">*</span>
					</label>
					<input
						type="text"
						id="uniqueKeyField"
						bind:value={editedSettings.uniqueKeyField}
						class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
						placeholder="e.g., sf_id, external_id"
					/>
				</div>

				<div>
					<label for="unmappedFieldMode" class="block text-sm font-medium text-gray-700 mb-1">
						Unmapped Field Mode
					</label>
					<select
						id="unmappedFieldMode"
						bind:value={editedSettings.unmappedFieldMode}
						class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
					>
						<option value="flexible">Flexible - accept unknown fields with warning</option>
						<option value="strict">Strict - reject unknown fields</option>
					</select>
				</div>

				<div>
					<label for="rateLimit" class="block text-sm font-medium text-gray-700 mb-1">
						Rate Limit
					</label>
					<div class="flex rounded-md shadow-sm">
						<input
							type="number"
							id="rateLimit"
							bind:value={editedSettings.rateLimit}
							min="1"
							max="10000"
							class="block w-full rounded-l-md border-gray-300 focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
						/>
						<span
							class="inline-flex items-center px-3 rounded-r-md border border-l-0 border-gray-300 bg-gray-50 text-gray-500 text-sm"
						>
							/min
						</span>
					</div>
				</div>
			</div>

			<div class="flex justify-end mt-4">
				<button
					onclick={saveSettings}
					disabled={savingSettings}
					class="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-600/90 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					{savingSettings ? 'Saving...' : 'Save Settings'}
				</button>
			</div>
		</div>

		<!-- Section 2: Source Fields -->
		<div class="bg-white shadow rounded-lg p-6">
			<div class="flex items-center justify-between mb-4">
				<h2 class="text-lg font-medium text-gray-900">Source Fields</h2>
				<button
					onclick={addFieldRow}
					disabled={showAddFieldForm}
					class="inline-flex items-center px-3 py-1.5 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-600/90 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					<svg class="w-4 h-4 mr-1.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
					</svg>
					Add Field
				</button>
			</div>

			{#if editedSourceFields.length === 0 && !showAddFieldForm}
				<p class="text-sm text-gray-500 text-center py-6">
					No source fields defined. Add fields to start mapping.
				</p>
			{:else}
				<div class="overflow-x-auto">
					<table class="min-w-full divide-y divide-gray-200">
						<thead class="bg-gray-50">
							<tr>
								<th
									scope="col"
									class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
									>Field Name</th
								>
								<th
									scope="col"
									class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
									>Type</th
								>
								<th
									scope="col"
									class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
									>Required</th
								>
								<th
									scope="col"
									class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
									>Description</th
								>
								<th scope="col" class="relative px-4 py-3">
									<span class="sr-only">Actions</span>
								</th>
							</tr>
						</thead>
						<tbody class="bg-white divide-y divide-gray-200">
							{#each editedSourceFields as field, index (index)}
								<tr class="hover:bg-gray-50">
									<td class="px-4 py-3 whitespace-nowrap text-sm font-medium text-gray-900">
										{field.fieldName}
									</td>
									<td class="px-4 py-3 whitespace-nowrap">
										<span
											class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium {getFieldTypeBadgeColor(
												field.fieldType
											)}"
										>
											{field.fieldType}
										</span>
									</td>
									<td class="px-4 py-3 whitespace-nowrap text-sm text-gray-500">
										{#if field.isRequired}
											<span class="text-red-600 font-medium">Yes</span>
										{:else}
											<span class="text-gray-400">No</span>
										{/if}
									</td>
									<td class="px-4 py-3 text-sm text-gray-500">
										{field.description || '-'}
									</td>
									<td class="px-4 py-3 whitespace-nowrap text-right text-sm font-medium">
										<button
											onclick={() => editField(index)}
											class="text-blue-600 hover:text-blue-900 mr-3"
										>
											Edit
										</button>
										<button
											onclick={() => removeField(index)}
											class="text-red-600 hover:text-red-900"
										>
											Remove
										</button>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}

			<!-- Add/Edit Field Form -->
			{#if showAddFieldForm}
				<div class="mt-4 border-t border-gray-200 pt-4 bg-gray-50 rounded-lg p-4">
					<h3 class="text-sm font-medium text-gray-900 mb-3">
						{editingFieldIndex !== null ? 'Edit Field' : 'Add New Field'}
					</h3>
					<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
						<div>
							<label for="newFieldName" class="block text-sm font-medium text-gray-700 mb-1">
								Field Name <span class="text-red-500">*</span>
							</label>
							<input
								type="text"
								id="newFieldName"
								bind:value={newField.fieldName}
								class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
								placeholder="e.g., FirstName, Email"
							/>
						</div>

						<div>
							<label for="newFieldType" class="block text-sm font-medium text-gray-700 mb-1">
								Type
							</label>
							<select
								id="newFieldType"
								bind:value={newField.fieldType}
								class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
							>
								<option value="text">Text</option>
								<option value="number">Number</option>
								<option value="date">Date</option>
								<option value="boolean">Boolean</option>
								<option value="email">Email</option>
								<option value="phone">Phone</option>
							</select>
						</div>

						<div class="md:col-span-2">
							<label for="newFieldDescription" class="block text-sm font-medium text-gray-700 mb-1">
								Description
							</label>
							<input
								type="text"
								id="newFieldDescription"
								bind:value={newField.description}
								class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
								placeholder="Optional description"
							/>
						</div>

						<div class="md:col-span-2">
							<label class="flex items-center">
								<input
									type="checkbox"
									bind:checked={newField.isRequired}
									class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
								/>
								<span class="ml-2 text-sm text-gray-700">Required field</span>
							</label>
						</div>
					</div>

					<div class="flex justify-end gap-3 mt-4">
						<button
							onclick={cancelFieldEdit}
							class="px-4 py-2 text-sm font-medium text-gray-700 border border-gray-300 rounded-md hover:bg-gray-50"
						>
							Cancel
						</button>
						<button
							onclick={saveNewField}
							class="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-600/90"
						>
							{editingFieldIndex !== null ? 'Update Field' : 'Add Field'}
						</button>
					</div>
				</div>
			{/if}

			<div class="flex justify-end mt-4">
				<button
					onclick={saveSourceFields}
					disabled={savingSourceFields || editedSourceFields.length === 0}
					class="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-600/90 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					{savingSourceFields ? 'Saving...' : 'Save Source Fields'}
				</button>
			</div>
		</div>

		<!-- Section 3: Field Mapping Grid -->
		<div class="bg-white shadow rounded-lg p-6">
			<div class="flex items-center justify-between mb-4">
				<h2 class="text-lg font-medium text-gray-900">Field Mapping</h2>
				<span class="text-sm text-gray-600">
					{mappedFieldsCount} of {totalFieldsCount} fields mapped
				</span>
			</div>

			{#if editedSourceFields.length === 0}
				<p class="text-sm text-gray-500 text-center py-6">
					Add source fields above to configure field mappings.
				</p>
			{:else}
				<div class="space-y-3">
					<div class="grid grid-cols-12 gap-4 text-sm font-medium text-gray-700 pb-2 border-b">
						<div class="col-span-5">Source Fields</div>
						<div class="col-span-1"></div>
						<div class="col-span-6">Quantico Fields</div>
					</div>

					{#each editedSourceFields as field, index (index)}
						{@const isUnmapped = !field.mapField || field.mapField === ''}
						<div
							class="grid grid-cols-12 gap-4 items-center py-3 rounded-lg {isUnmapped
								? 'border-l-4 border-amber-400 bg-amber-50 pl-3'
								: 'border-l-4 border-transparent pl-3'}"
						>
							<!-- Source Field -->
							<div class="col-span-5 flex items-center gap-2">
								{#if isUnmapped}
									<svg class="w-4 h-4 text-amber-500 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path
											stroke-linecap="round"
											stroke-linejoin="round"
											stroke-width="2"
											d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
										/>
									</svg>
								{/if}
								<div class="flex-1">
									<div class="flex items-center gap-2">
										<span class="font-medium text-gray-900">{field.fieldName}</span>
										{#if field.isRequired}
											<span class="text-red-500 font-bold">*</span>
										{/if}
									</div>
									<div class="flex items-center gap-2 mt-1">
										<span
											class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium {getFieldTypeBadgeColor(
												field.fieldType
											)}"
										>
											{field.fieldType}
										</span>
										{#if isUnmapped}
											<span class="text-xs text-amber-600 font-medium">Unmapped</span>
										{/if}
									</div>
								</div>
							</div>

							<!-- Arrow -->
							<div class="col-span-1 flex items-center justify-center">
								<svg class="w-5 h-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14 5l7 7m0 0l-7 7m7-7H3" />
								</svg>
							</div>

							<!-- Target Field Dropdown -->
							<div class="col-span-6">
								<select
									value={field.mapField || ''}
									onchange={(e) => updateMapping(index, e.currentTarget.value)}
									class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
								>
									<option value="">-- Not Mapped --</option>
									{#each targetFields as targetField}
										<option value={targetField.name}>
											{targetField.label} ({targetField.name})
										</option>
									{/each}
								</select>
							</div>
						</div>
					{/each}
				</div>

				<div class="flex justify-end mt-4">
					<button
						onclick={saveMappings}
						disabled={savingMappings || editedSourceFields.length === 0}
						class="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-600/90 disabled:opacity-50 disabled:cursor-not-allowed"
					>
						{savingMappings ? 'Saving...' : 'Save Mappings'}
					</button>
				</div>
			{/if}
		</div>

		<!-- Section 4: Job History -->
		<div class="bg-white shadow rounded-lg p-6">
			<div class="flex items-center justify-between mb-4">
				<div class="flex items-center gap-3">
					<h2 class="text-lg font-medium text-gray-900">Job History</h2>
					<span class="text-sm text-gray-500">({jobs.length} jobs)</span>
				</div>
				<button
					onclick={loadJobs}
					disabled={loadingJobs}
					class="inline-flex items-center px-3 py-1.5 text-sm font-medium text-gray-700 border border-gray-300 rounded-md hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					<svg
						class="w-4 h-4 mr-1.5 {loadingJobs ? 'animate-spin' : ''}"
						fill="none"
						stroke="currentColor"
						viewBox="0 0 24 24"
					>
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="2"
							d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
						/>
					</svg>
					Refresh
				</button>
			</div>

			{#if loadingJobs && jobs.length === 0}
				<!-- Loading skeleton -->
				<div class="space-y-3">
					{#each Array(3) as _}
						<div class="animate-pulse flex space-x-4 py-3 border-b">
							<div class="flex-1 space-y-2">
								<div class="h-4 bg-gray-200 rounded w-1/4"></div>
								<div class="h-3 bg-gray-200 rounded w-1/2"></div>
							</div>
						</div>
					{/each}
				</div>
			{:else if jobs.length === 0}
				<!-- Empty state -->
				<div class="text-center py-12">
					<svg
						class="mx-auto h-12 w-12 text-gray-400"
						fill="none"
						stroke="currentColor"
						viewBox="0 0 24 24"
					>
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="2"
							d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
						/>
					</svg>
					<h3 class="mt-2 text-sm font-medium text-gray-900">No ingest jobs yet</h3>
					<p class="mt-1 text-sm text-gray-500">
						Jobs appear here when external systems push data through this mirror.
					</p>
				</div>
			{:else}
				<!-- Job table -->
				<div class="overflow-x-auto">
					<table class="min-w-full divide-y divide-gray-200">
						<thead class="bg-gray-50">
							<tr>
								<th
									scope="col"
									class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
									>Status</th
								>
								<th
									scope="col"
									class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
									>Records</th
								>
								<th
									scope="col"
									class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
									>Started</th
								>
								<th
									scope="col"
									class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
									>Completed</th
								>
								<th
									scope="col"
									class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
									>Duration</th
								>
								<th scope="col" class="relative px-4 py-3">
									<span class="sr-only">Expand</span>
								</th>
							</tr>
						</thead>
						<tbody class="bg-white divide-y divide-gray-200">
							{#each jobs as job (job.id)}
								<tr class="hover:bg-gray-50">
									<td class="px-4 py-3 whitespace-nowrap">
										<span
											class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium {getStatusBadgeColor(
												job.status
											)}"
										>
											{job.status}
										</span>
									</td>
									<td class="px-4 py-3 whitespace-nowrap text-sm text-gray-900">
										<div class="space-y-0.5">
											<div>
												{job.recordsReceived} recv / {job.recordsProcessed} ok
											</div>
											<div class="text-xs text-gray-500">
												{job.recordsSkipped} skip / {job.recordsFailed} err
											</div>
										</div>
									</td>
									<td class="px-4 py-3 whitespace-nowrap text-sm text-gray-500">
										{job.startedAt ? formatTimestamp(job.startedAt) : '-'}
									</td>
									<td class="px-4 py-3 whitespace-nowrap text-sm text-gray-500">
										{job.completedAt ? formatTimestamp(job.completedAt) : '-'}
									</td>
									<td class="px-4 py-3 whitespace-nowrap text-sm text-gray-500">
										{formatDuration(job.startedAt, job.completedAt)}
									</td>
									<td class="px-4 py-3 whitespace-nowrap text-right text-sm font-medium">
										<button
											onclick={() => toggleJobExpanded(job.id)}
											class="text-blue-600 hover:text-blue-900"
										>
											{#if expandedJobs.has(job.id)}
												<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
													<path
														stroke-linecap="round"
														stroke-linejoin="round"
														stroke-width="2"
														d="M5 15l7-7 7 7"
													/>
												</svg>
											{:else}
												<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
													<path
														stroke-linecap="round"
														stroke-linejoin="round"
														stroke-width="2"
														d="M19 9l-7 7-7-7"
													/>
												</svg>
											{/if}
										</button>
									</td>
								</tr>

								{#if expandedJobs.has(job.id)}
									<tr>
										<td colspan="6" class="px-4 py-4 bg-gray-50">
											<div class="space-y-4">
												<!-- Errors -->
												{#if job.errors && job.errors.length > 0}
													<div>
														<h4 class="text-sm font-medium text-gray-900 mb-2">
															Errors ({job.errors.length})
														</h4>
														<div class="overflow-x-auto">
															<table class="min-w-full divide-y divide-gray-200">
																<thead class="bg-white">
																	<tr>
																		<th
																			scope="col"
																			class="px-3 py-2 text-left text-xs font-medium text-gray-500 uppercase"
																			>Record #</th
																		>
																		<th
																			scope="col"
																			class="px-3 py-2 text-left text-xs font-medium text-gray-500 uppercase"
																			>Unique Key</th
																		>
																		<th
																			scope="col"
																			class="px-3 py-2 text-left text-xs font-medium text-gray-500 uppercase"
																			>Field</th
																		>
																		<th
																			scope="col"
																			class="px-3 py-2 text-left text-xs font-medium text-gray-500 uppercase"
																			>Code</th
																		>
																		<th
																			scope="col"
																			class="px-3 py-2 text-left text-xs font-medium text-gray-500 uppercase"
																			>Message</th
																		>
																	</tr>
																</thead>
																<tbody class="bg-white divide-y divide-gray-200">
																	{#each job.errors as error}
																		<tr>
																			<td class="px-3 py-2 whitespace-nowrap text-sm text-gray-900">
																				{error.index}
																			</td>
																			<td class="px-3 py-2 whitespace-nowrap text-sm text-gray-900">
																				{error.uniqueKey}
																			</td>
																			<td class="px-3 py-2 whitespace-nowrap text-sm text-gray-900">
																				{error.field || '-'}
																			</td>
																			<td class="px-3 py-2 whitespace-nowrap text-sm font-mono text-gray-600">
																				{error.code}
																			</td>
																			<td class="px-3 py-2 text-sm text-gray-700">
																				{error.message}
																			</td>
																		</tr>
																	{/each}
																</tbody>
															</table>
														</div>
													</div>
												{/if}

												<!-- Warnings -->
												{#if job.warnings && job.warnings.length > 0}
													<div>
														<h4 class="text-sm font-medium text-gray-900 mb-2">
															Warnings ({job.warnings.length})
														</h4>
														<ul class="list-disc list-inside space-y-1">
															{#each job.warnings as warning}
																<li class="text-sm text-gray-700">{warning}</li>
															{/each}
														</ul>
													</div>
												{/if}

												<!-- No errors or warnings -->
												{#if (!job.errors || job.errors.length === 0) && (!job.warnings || job.warnings.length === 0)}
													<p class="text-sm text-gray-500">No errors or warnings</p>
												{/if}
											</div>
										</td>
									</tr>
								{/if}
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</div>
	{/if}
</div>
