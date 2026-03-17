<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { PUBLIC_API_URL } from '$env/static/public';
	import { get, put } from '$lib/utils/api';
	import { auth } from '$lib/stores/auth.svelte';
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

	// CSV Import state
	let csvFile = $state<File | null>(null);
	let csvHeaders = $state<string[]>([]);
	let csvRowCount = $state(0);
	let csvDragOver = $state(false);
	let csvUploading = $state(false);
	let csvImportJob = $state<IngestJob | null>(null);
	let csvPollInterval: number | null = null;
	let fileInputEl: HTMLInputElement;

	// CSV column mapping preview: maps CSV header -> { sourceField, targetField }
	const csvColumnMapping = $derived(
		csvHeaders.map((header) => {
			const normalizedHeader = header.trim().toLowerCase();
			// Try to find a matching source field
			const sourceField = editedSourceFields.find(
				(sf) => sf.fieldName.toLowerCase() === normalizedHeader
			);
			const targetFieldName = sourceField?.mapField || null;
			const targetField = targetFieldName
				? targetFields.find((tf) => tf.name === targetFieldName)
				: null;
			return {
				csvColumn: header.trim(),
				sourceFieldName: sourceField?.fieldName || null,
				targetFieldName: targetField?.name || null,
				targetFieldLabel: targetField?.label || null,
				matched: !!sourceField
			};
		})
	);

	const csvCanImport = $derived(csvFile !== null && !csvUploading);

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

	// --- CSV Import functions ---

	function parseCSVHeaders(text: string): { headers: string[]; rowCount: number } {
		const lines = text.split('\n').filter((line) => line.trim() !== '');
		if (lines.length === 0) return { headers: [], rowCount: 0 };
		// Split first line by comma (simple parse for preview only)
		const headers = lines[0].split(',').map((h) => h.trim().replace(/^"|"$/g, ''));
		// Row count excludes header
		return { headers, rowCount: Math.max(0, lines.length - 1) };
	}

	function handleCSVFileSelect(file: File) {
		if (!file.name.toLowerCase().endsWith('.csv')) {
			toast.error('Please select a CSV file');
			return;
		}
		csvFile = file;
		csvImportJob = null;

		// Parse headers from the file
		const reader = new FileReader();
		reader.onload = (e) => {
			const text = e.target?.result as string;
			const { headers, rowCount } = parseCSVHeaders(text);
			csvHeaders = headers;
			csvRowCount = rowCount;
		};
		reader.readAsText(file);
	}

	function handleCSVDrop(e: DragEvent) {
		e.preventDefault();
		csvDragOver = false;
		const file = e.dataTransfer?.files?.[0];
		if (file) handleCSVFileSelect(file);
	}

	function handleCSVDragOver(e: DragEvent) {
		e.preventDefault();
		csvDragOver = true;
	}

	function handleCSVDragLeave() {
		csvDragOver = false;
	}

	function handleCSVInputChange(e: Event) {
		const input = e.target as HTMLInputElement;
		const file = input.files?.[0];
		if (file) handleCSVFileSelect(file);
	}

	function clearCSVFile() {
		csvFile = null;
		csvHeaders = [];
		csvRowCount = 0;
		csvImportJob = null;
		if (fileInputEl) fileInputEl.value = '';
		stopCSVPoll();
	}

	function formatFileSize(bytes: number): string {
		if (bytes < 1024) return `${bytes} B`;
		if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
		return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
	}

	async function uploadCSV() {
		if (!csvFile || !mirrorId) return;

		const apiBase = PUBLIC_API_URL || '/api/v1';

		try {
			csvUploading = true;
			const formData = new FormData();
			formData.append('file', csvFile);

			const headers: Record<string, string> = {};
			const token = auth.accessToken;
			if (token) {
				headers['Authorization'] = `Bearer ${token}`;
			}
			// CSRF token for state-changing request
			const csrfMatch = document.cookie.match(/(?:^|; )csrf_token=([^;]*)/);
			if (csrfMatch) {
				headers['X-CSRF-Token'] = decodeURIComponent(csrfMatch[1]);
			}

			const response = await fetch(`${apiBase}/admin/mirrors/${mirrorId}/import-csv`, {
				method: 'POST',
				headers,
				body: formData,
				credentials: 'include'
			});

			if (!response.ok) {
				const errBody = await response.json().catch(() => ({ error: 'Upload failed' }));
				throw new Error(errBody.error || `HTTP ${response.status}`);
			}

			const result = await response.json();
			csvImportJob = result.job || result;
			toast.success('CSV import started');

			// Start polling for job status
			startCSVPoll();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to upload CSV');
		} finally {
			csvUploading = false;
		}
	}

	async function pollCSVJobStatus() {
		if (!csvImportJob) return;

		try {
			const response = await get<{ jobs: IngestJob[]; total: number }>(
				`/admin/mirrors/${mirrorId}/jobs?limit=10`
			);
			const updatedJobs = response.jobs || [];
			// Find our job
			const updated = updatedJobs.find((j) => j.id === csvImportJob!.id);
			if (updated) {
				csvImportJob = updated;
				if (
					updated.status === 'complete' ||
					updated.status === 'partial' ||
					updated.status === 'failed'
				) {
					stopCSVPoll();
					// Refresh the main job history list
					await loadJobs();
				}
			}
		} catch {
			// Silently ignore poll errors
		}
	}

	function startCSVPoll() {
		stopCSVPoll();
		csvPollInterval = setInterval(pollCSVJobStatus, 2000) as unknown as number;
	}

	function stopCSVPoll() {
		if (csvPollInterval !== null) {
			clearInterval(csvPollInterval);
			csvPollInterval = null;
		}
	}

	function resetCSVImport() {
		clearCSVFile();
	}

	function getCSVStatusLabel(status: string): string {
		switch (status) {
			case 'accepted':
				return 'Queued';
			case 'processing':
				return 'Processing...';
			case 'complete':
				return 'Import Complete';
			case 'partial':
				return 'Partial Import';
			case 'failed':
				return 'Import Failed';
			default:
				return status;
		}
	}

	function getCSVStatusColor(status: string): string {
		switch (status) {
			case 'complete':
				return 'text-green-700 bg-green-50 border-green-200';
			case 'partial':
				return 'text-yellow-700 bg-yellow-50 border-yellow-200';
			case 'failed':
				return 'text-red-700 bg-red-50 border-red-200';
			case 'processing':
			case 'accepted':
				return 'text-blue-700 bg-blue-50 border-blue-200';
			default:
				return 'text-gray-700 bg-gray-50 border-gray-200';
		}
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
		stopCSVPoll();
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
						<tbody class="divide-y divide-gray-200">
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

		<!-- Section 4: CSV Import -->
		<div class="bg-white shadow rounded-lg p-6">
			<h2 class="text-lg font-medium text-gray-900 mb-4">CSV Import</h2>

			{#if csvImportJob && (csvImportJob.status === 'complete' || csvImportJob.status === 'partial' || csvImportJob.status === 'failed')}
				<!-- Import Result -->
				<div class="border rounded-lg p-4 {getCSVStatusColor(csvImportJob.status)}">
					<div class="flex items-center gap-2 mb-3">
						{#if csvImportJob.status === 'complete'}
							<svg class="w-5 h-5 text-green-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
							</svg>
						{:else if csvImportJob.status === 'partial'}
							<svg class="w-5 h-5 text-yellow-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
							</svg>
						{:else}
							<svg class="w-5 h-5 text-red-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
							</svg>
						{/if}
						<span class="font-medium">{getCSVStatusLabel(csvImportJob.status)}</span>
					</div>

					<div class="text-sm space-y-1">
						<p>
							{csvImportJob.recordsReceived} received
							· {csvImportJob.recordsPromoted} promoted
							{#if csvImportJob.recordsSkipped > 0}
								· {csvImportJob.recordsSkipped} skipped
							{/if}
							{#if csvImportJob.recordsFailed > 0}
								· {csvImportJob.recordsFailed} failed
							{/if}
						</p>
					</div>

					{#if csvImportJob.errors && csvImportJob.errors.length > 0}
						<details class="mt-3">
							<summary class="cursor-pointer text-sm font-medium">
								Errors ({csvImportJob.errors.length})
							</summary>
							<div class="mt-2 max-h-48 overflow-y-auto">
								<table class="min-w-full text-xs">
									<thead>
										<tr class="border-b">
											<th class="text-left py-1 pr-2">#</th>
											<th class="text-left py-1 pr-2">Key</th>
											<th class="text-left py-1 pr-2">Field</th>
											<th class="text-left py-1">Message</th>
										</tr>
									</thead>
									<tbody>
										{#each csvImportJob.errors as error}
											<tr class="border-b border-opacity-50">
												<td class="py-1 pr-2">{error.index}</td>
												<td class="py-1 pr-2">{error.uniqueKey}</td>
												<td class="py-1 pr-2">{error.field || '-'}</td>
												<td class="py-1">{error.message}</td>
											</tr>
										{/each}
									</tbody>
								</table>
							</div>
						</details>
					{/if}

					{#if csvImportJob.warnings && csvImportJob.warnings.length > 0}
						<details class="mt-2">
							<summary class="cursor-pointer text-sm font-medium">
								Warnings ({csvImportJob.warnings.length})
							</summary>
							<ul class="mt-1 list-disc list-inside text-xs space-y-0.5">
								{#each csvImportJob.warnings as warning}
									<li>{warning}</li>
								{/each}
							</ul>
						</details>
					{/if}

					<div class="mt-4">
						<button
							onclick={resetCSVImport}
							class="px-4 py-2 text-sm font-medium text-gray-700 border border-gray-300 rounded-md hover:bg-gray-50 bg-white"
						>
							Import Another
						</button>
					</div>
				</div>
			{:else if csvImportJob && (csvImportJob.status === 'accepted' || csvImportJob.status === 'processing')}
				<!-- Upload in progress / processing -->
				<div class="border rounded-lg p-4 {getCSVStatusColor(csvImportJob.status)}">
					<div class="flex items-center gap-3">
						<svg class="animate-spin h-5 w-5 text-blue-600" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
						</svg>
						<span class="font-medium">{getCSVStatusLabel(csvImportJob.status)}</span>
					</div>
					{#if csvImportJob.recordsReceived > 0}
						<p class="mt-2 text-sm">
							{csvImportJob.recordsProcessed} of {csvImportJob.recordsReceived} records processed
						</p>
					{/if}
				</div>
			{:else}
				<!-- File upload area -->
				{#if !csvFile}
					<!-- Dropzone -->
					<div
						role="button"
						tabindex="0"
						class="border-2 border-dashed rounded-lg p-8 text-center cursor-pointer transition-colors
							{csvDragOver ? 'border-blue-500 bg-blue-50' : 'border-gray-300 hover:border-gray-400 hover:bg-gray-50'}"
						ondrop={handleCSVDrop}
						ondragover={handleCSVDragOver}
						ondragleave={handleCSVDragLeave}
						onclick={() => fileInputEl?.click()}
						onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') fileInputEl?.click(); }}
					>
						<svg class="mx-auto h-10 w-10 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
						</svg>
						<p class="mt-2 text-sm text-gray-600">
							Drag & drop a CSV file here, or <span class="text-blue-600 font-medium">click to browse</span>
						</p>
						<p class="mt-1 text-xs text-gray-400">Only .csv files are accepted</p>
						<input
							bind:this={fileInputEl}
							type="file"
							accept=".csv"
							class="hidden"
							onchange={handleCSVInputChange}
						/>
					</div>
				{:else}
					<!-- File selected -->
					<div class="space-y-4">
						<!-- File info bar -->
						<div class="flex items-center justify-between bg-gray-50 rounded-lg px-4 py-3">
							<div class="flex items-center gap-3">
								<svg class="h-5 w-5 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
								</svg>
								<div>
									<p class="text-sm font-medium text-gray-900">{csvFile.name}</p>
									<p class="text-xs text-gray-500">
										{formatFileSize(csvFile.size)} · {csvRowCount} row{csvRowCount !== 1 ? 's' : ''}
									</p>
								</div>
							</div>
							<button
								onclick={clearCSVFile}
								class="text-gray-400 hover:text-gray-600"
								title="Remove file"
							>
								<svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
								</svg>
							</button>
						</div>

						<!-- Column Mapping Preview -->
						{#if csvHeaders.length > 0}
							<div>
								<h3 class="text-sm font-medium text-gray-700 mb-2">Column Mapping Preview</h3>
								<div class="overflow-x-auto border rounded-lg">
									<table class="min-w-full divide-y divide-gray-200">
										<thead class="bg-gray-50">
											<tr>
												<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">CSV Column</th>
												<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">Source Field</th>
												<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">Target Field</th>
												<th class="px-4 py-2 text-center text-xs font-medium text-gray-500 uppercase w-16">Status</th>
											</tr>
										</thead>
										<tbody class="divide-y divide-gray-200">
											{#each csvColumnMapping as col}
												<tr>
													<td class="px-4 py-2 text-sm font-medium text-gray-900">{col.csvColumn}</td>
													<td class="px-4 py-2 text-sm text-gray-600">{col.sourceFieldName || '--'}</td>
													<td class="px-4 py-2 text-sm text-gray-600">
														{#if col.targetFieldLabel}
															{col.targetFieldLabel} ({col.targetFieldName})
														{:else if col.sourceFieldName}
															<span class="text-gray-400">Not mapped</span>
														{:else}
															--
														{/if}
													</td>
													<td class="px-4 py-2 text-center">
														{#if col.matched && col.targetFieldName}
															<span class="inline-flex items-center justify-center w-6 h-6 rounded-full bg-green-100 text-green-600" title="Matched">
																<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
																	<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
																</svg>
															</span>
														{:else}
															<span class="inline-flex items-center justify-center w-6 h-6 rounded-full bg-yellow-100 text-yellow-600" title="Unmatched">
																<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
																	<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01" />
																</svg>
															</span>
														{/if}
													</td>
												</tr>
											{/each}
										</tbody>
									</table>
								</div>
							</div>
						{/if}

						<!-- Import button -->
						<div class="flex justify-end">
							<button
								onclick={uploadCSV}
								disabled={!csvCanImport}
								class="inline-flex items-center px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-600/90 disabled:opacity-50 disabled:cursor-not-allowed"
							>
								{#if csvUploading}
									<svg class="animate-spin -ml-1 mr-2 h-4 w-4 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
										<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
										<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
									</svg>
									Uploading...
								{:else}
									Import CSV
								{/if}
							</button>
						</div>
					</div>
				{/if}
			{/if}
		</div>

		<!-- Section 5: Job History -->
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
						<tbody class="divide-y divide-gray-200">
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
																<tbody class="divide-y divide-gray-200">
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
