<script lang="ts">
	import { api } from '$lib/utils/api';
	import { addToast } from '$lib/stores/toast.svelte';

	interface Props {
		entityName: string;
		onComplete: () => void;
		onCancel: () => void;
	}

	let { entityName, onComplete, onCancel }: Props = $props();

	interface PreviewResponse {
		headers: string[];
		mappedHeaders: string[];
		sampleRows: Record<string, any>[];
		totalRows: number;
		unmappedColumns: string[];
		fields: FieldMapping[];
	}

	interface FieldMapping {
		csvHeader: string;
		fieldName: string;
		fieldLabel: string;
		fieldType: string;
		mapped: boolean;
	}

	interface ValidationIssue {
		row: number;
		column: string;
		fieldName: string;
		value: string;
		issueType: string;
		message: string;
		expected?: string;
	}

	interface AnalyzeResult {
		valid: boolean;
		totalRows: number;
		validRows: number;
		invalidRows: number;
		issues: ValidationIssue[];
		mappedFields: string[];
		missingRequired: string[];
	}

	interface ImportResponse {
		created: number;
		updated: number;
		failed: number;
		totalRows: number;
		errors?: Array<{ index: number; error: string }>;
		ids?: string[];
	}

	let step = $state(1); // 1=Upload, 2=Validate, 3=Import
	let file: File | null = $state(null);
	let previewData: PreviewResponse | null = $state(null);
	let columnMapping: Record<string, string> = $state({});
	let analyzeResult: AnalyzeResult | null = $state(null);
	let importResult: ImportResponse | null = $state(null);
	let loading = $state(false);
	let error = $state('');
	let availableFields: string[] = $state([]);

	// Handle file selection
	async function handleFileSelect(event: Event) {
		const input = event.target as HTMLInputElement;
		if (!input.files || input.files.length === 0) return;

		file = input.files[0];
		error = '';
		loading = true;

		try {
			// Upload file for preview
			const formData = new FormData();
			formData.append('file', file);

			const response = await fetch(`/api/v1/entities/${entityName}/import/csv/preview`, {
				method: 'POST',
				body: formData,
				credentials: 'include',
				headers: {
					'Authorization': `Bearer ${localStorage.getItem('accessToken') || ''}`,
					'X-CSRF-Token': getCsrfToken()
				}
			});

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || 'Failed to preview CSV');
			}

			previewData = await response.json();

			// Initialize column mapping with auto-mapped fields
			const mapping: Record<string, string> = {};
			previewData.headers.forEach((header, idx) => {
				if (previewData!.mappedHeaders[idx]) {
					mapping[header] = previewData!.mappedHeaders[idx];
				}
			});
			columnMapping = mapping;

			// Get available fields for dropdowns
			availableFields = previewData.fields.map(f => f.fieldName).filter(Boolean);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load file';
			file = null;
		} finally {
			loading = false;
		}
	}

	// Proceed to validation
	async function analyzeData() {
		if (!file) return;

		loading = true;
		error = '';

		try {
			const formData = new FormData();
			formData.append('file', file);
			formData.append('options', JSON.stringify({ columnMapping }));

			const response = await fetch(`/api/v1/entities/${entityName}/import/csv/analyze`, {
				method: 'POST',
				body: formData,
				credentials: 'include',
				headers: {
					'Authorization': `Bearer ${localStorage.getItem('accessToken') || ''}`,
					'X-CSRF-Token': getCsrfToken()
				}
			});

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || 'Validation failed');
			}

			analyzeResult = await response.json();
			step = 2;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Validation failed';
		} finally {
			loading = false;
		}
	}

	// Execute import
	async function executeImport() {
		if (!file) return;

		loading = true;
		error = '';

		try {
			const formData = new FormData();
			formData.append('file', file);
			formData.append('options', JSON.stringify({
				columnMapping,
				mode: 'create',
				skipErrors: false
			}));

			const response = await fetch(`/api/v1/entities/${entityName}/import/csv`, {
				method: 'POST',
				body: formData,
				credentials: 'include',
				headers: {
					'Authorization': `Bearer ${localStorage.getItem('accessToken') || ''}`,
					'X-CSRF-Token': getCsrfToken()
				}
			});

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || 'Import failed');
			}

			importResult = await response.json();
			step = 3;
			addToast('success', `Successfully imported ${importResult.created} records`);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Import failed';
			addToast('error', 'Import failed');
		} finally {
			loading = false;
		}
	}

	function getCsrfToken(): string {
		if (typeof document === 'undefined') return '';
		const match = document.cookie.match(/(?:^|; )csrf_token=([^;]*)/);
		return match ? decodeURIComponent(match[1]) : '';
	}

	function goBack() {
		if (step === 2) {
			step = 1;
			analyzeResult = null;
		}
	}

	function updateMapping(csvHeader: string, fieldName: string) {
		columnMapping[csvHeader] = fieldName;
	}
</script>

<div class="max-w-6xl mx-auto">
	<!-- Step Indicator -->
	<div class="flex items-center justify-center mb-8">
		<div class="flex items-center">
			<div class="flex items-center justify-center w-10 h-10 rounded-full"
				class:bg-blue-600={step >= 1}
				class:bg-gray-300={step < 1}
				class:text-white={step >= 1}
			>
				1
			</div>
			<div class="w-24 h-1 mx-2" class:bg-blue-600={step >= 2} class:bg-gray-300={step < 2}></div>
			<div class="flex items-center justify-center w-10 h-10 rounded-full"
				class:bg-blue-600={step >= 2}
				class:bg-gray-300={step < 2}
				class:text-white={step >= 2}
			>
				2
			</div>
			<div class="w-24 h-1 mx-2" class:bg-blue-600={step >= 3} class:bg-gray-300={step < 3}></div>
			<div class="flex items-center justify-center w-10 h-10 rounded-full"
				class:bg-blue-600={step >= 3}
				class:bg-gray-300={step < 3}
				class:text-white={step >= 3}
			>
				3
			</div>
		</div>
	</div>

	<!-- Error Display -->
	{#if error}
		<div class="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded mb-4">
			{error}
		</div>
	{/if}

	<!-- Step 1: Upload & Map -->
	{#if step === 1}
		<div class="bg-white shadow rounded-lg p-6">
			<h2 class="text-xl font-semibold mb-4">Step 1: Upload CSV & Map Columns</h2>

			{#if !previewData}
				<div class="mb-4">
					<label class="block text-sm font-medium text-gray-700 mb-2">
						Select CSV File
					</label>
					<input
						type="file"
						accept=".csv"
						onchange={handleFileSelect}
						class="block w-full text-sm text-gray-900 border border-gray-300 rounded-lg cursor-pointer bg-gray-50 focus:outline-none"
					/>
				</div>
			{:else}
				<div class="mb-4">
					<p class="text-sm text-gray-600 mb-4">
						File: <span class="font-medium">{file?.name}</span> ({previewData.totalRows} rows)
					</p>

					<!-- Column Mapping Table -->
					<div class="overflow-x-auto">
						<table class="min-w-full divide-y divide-gray-200">
							<thead class="bg-gray-50">
								<tr>
									<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">CSV Header</th>
									<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Sample Data</th>
									<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Maps To</th>
								</tr>
							</thead>
							<tbody class="bg-white divide-y divide-gray-200">
								{#each previewData.headers as header, idx}
									<tr>
										<td class="px-4 py-3 text-sm font-medium text-gray-900">{header}</td>
										<td class="px-4 py-3 text-sm text-gray-600">
											{previewData.sampleRows[0]?.[header] || '-'}
										</td>
										<td class="px-4 py-3">
											<select
												bind:value={columnMapping[header]}
												onchange={() => updateMapping(header, columnMapping[header])}
												class="block w-full border-gray-300 rounded-md shadow-sm focus:ring-blue-500 focus:border-blue-500 sm:text-sm"
											>
												<option value="">-- Skip this column --</option>
												{#each availableFields as field}
													<option value={field}>{field}</option>
												{/each}
											</select>
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				</div>

				<div class="flex justify-between mt-6">
					<button
						onclick={onCancel}
						class="px-4 py-2 border border-gray-300 rounded-md text-gray-700 hover:bg-gray-50"
					>
						Cancel
					</button>
					<button
						onclick={analyzeData}
						disabled={loading}
						class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50"
					>
						{loading ? 'Analyzing...' : 'Analyze Data'}
					</button>
				</div>
			{/if}
		</div>
	{/if}

	<!-- Step 2: Validate -->
	{#if step === 2 && analyzeResult}
		<div class="bg-white shadow rounded-lg p-6">
			<h2 class="text-xl font-semibold mb-4">Step 2: Validation Results</h2>

			<!-- Summary -->
			<div class="grid grid-cols-3 gap-4 mb-6">
				<div class="bg-blue-50 p-4 rounded-lg">
					<p class="text-sm text-gray-600">Total Rows</p>
					<p class="text-2xl font-bold text-blue-600">{analyzeResult.totalRows}</p>
				</div>
				<div class="bg-green-50 p-4 rounded-lg">
					<p class="text-sm text-gray-600">Valid Rows</p>
					<p class="text-2xl font-bold text-green-600">{analyzeResult.validRows}</p>
				</div>
				<div class="bg-red-50 p-4 rounded-lg">
					<p class="text-sm text-gray-600">Invalid Rows</p>
					<p class="text-2xl font-bold text-red-600">{analyzeResult.invalidRows}</p>
				</div>
			</div>

			<!-- Missing Required Fields -->
			{#if analyzeResult.missingRequired.length > 0}
				<div class="bg-yellow-50 border border-yellow-200 rounded-lg p-4 mb-4">
					<h3 class="font-semibold text-yellow-800 mb-2">Missing Required Fields</h3>
					<ul class="list-disc list-inside text-yellow-700">
						{#each analyzeResult.missingRequired as field}
							<li>{field}</li>
						{/each}
					</ul>
				</div>
			{/if}

			<!-- Validation Issues -->
			{#if analyzeResult.issues.length > 0}
				<div class="mb-4">
					<h3 class="font-semibold mb-2">Validation Issues</h3>
					<div class="overflow-x-auto max-h-96 overflow-y-auto">
						<table class="min-w-full divide-y divide-gray-200">
							<thead class="bg-gray-50 sticky top-0">
								<tr>
									<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">Row</th>
									<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">Column</th>
									<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">Value</th>
									<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">Issue</th>
									<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">Expected</th>
								</tr>
							</thead>
							<tbody class="bg-white divide-y divide-gray-200">
								{#each analyzeResult.issues as issue}
									<tr>
										<td class="px-4 py-2 text-sm">{issue.row}</td>
										<td class="px-4 py-2 text-sm font-medium">{issue.column}</td>
										<td class="px-4 py-2 text-sm text-gray-600">{issue.value || '(empty)'}</td>
										<td class="px-4 py-2 text-sm text-red-600">{issue.message}</td>
										<td class="px-4 py-2 text-sm text-gray-500">{issue.expected || '-'}</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				</div>
			{/if}

			<!-- Actions -->
			<div class="flex justify-between mt-6">
				<button
					onclick={goBack}
					class="px-4 py-2 border border-gray-300 rounded-md text-gray-700 hover:bg-gray-50"
				>
					Back
				</button>
				<button
					onclick={executeImport}
					disabled={loading || !analyzeResult.valid}
					class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50"
					title={!analyzeResult.valid ? 'Fix validation issues before importing' : ''}
				>
					{loading ? 'Importing...' : 'Import Data'}
				</button>
			</div>
		</div>
	{/if}

	<!-- Step 3: Import Complete -->
	{#if step === 3 && importResult}
		<div class="bg-white shadow rounded-lg p-6">
			<h2 class="text-xl font-semibold mb-4">Step 3: Import Complete</h2>

			<div class="bg-green-50 border border-green-200 rounded-lg p-4 mb-4">
				<h3 class="font-semibold text-green-800 mb-2">Import Successful</h3>
				<p class="text-green-700">
					Created {importResult.created} records out of {importResult.totalRows} rows
				</p>
			</div>

			{#if importResult.failed > 0}
				<div class="bg-red-50 border border-red-200 rounded-lg p-4 mb-4">
					<h3 class="font-semibold text-red-800 mb-2">Failed Records: {importResult.failed}</h3>
					{#if importResult.errors}
						<ul class="list-disc list-inside text-red-700">
							{#each importResult.errors as err}
								<li>Row {err.index}: {err.error}</li>
							{/each}
						</ul>
					{/if}
				</div>
			{/if}

			<div class="flex justify-end mt-6">
				<button
					onclick={onComplete}
					class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700"
				>
					Done
				</button>
			</div>
		</div>
	{/if}

	<!-- Loading Overlay -->
	{#if loading}
		<div class="fixed inset-0 bg-black bg-opacity-25 flex items-center justify-center z-50">
			<div class="bg-white rounded-lg p-6 shadow-xl">
				<div class="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mx-auto"></div>
				<p class="mt-4 text-gray-600">Processing...</p>
			</div>
		</div>
	{/if}
</div>
