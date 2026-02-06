<script lang="ts">
	import { PUBLIC_API_URL } from '$env/static/public';
	import { addToast } from '$lib/stores/toast.svelte';
	import { auth } from '$lib/stores/auth.svelte';

	const API_BASE = PUBLIC_API_URL || '/api/v1';

	interface Props {
		entityName: string;
		onComplete: () => void;
		onCancel: () => void;
	}

	let { entityName, onComplete, onCancel }: Props = $props();

	interface AvailableField {
		name: string;
		label: string;
		type: string;
		relatedEntity?: string; // For link fields
	}

	interface LookupResolution {
		matchField: string;
		createIfNotFound?: boolean;
		newRecordData?: Record<string, Record<string, any>>; // matchValue -> field values
	}

	interface MissingLookup {
		fieldName: string;
		relatedEntity: string;
		matchValue: string;
		matchField: string;
		requiredFields: AvailableField[];
		rowIndices: number[];
	}

	interface AnalyzeLookupResponse {
		missingLookups: MissingLookup[];
	}

	interface PreviewResponse {
		headers: string[];
		mappedHeaders: string[];
		sampleRows: Record<string, any>[];
		totalRows: number;
		unmappedColumns: string[];
		fields: FieldMapping[];
		availableFields: AvailableField[];
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

	let step = $state(1); // 1=Upload, 2=Validate, 2.5=CreateLookups, 3=Import
	let file: File | null = $state(null);
	let previewData: PreviewResponse | null = $state(null);
	let columnMapping: Record<string, string> = $state({});
	let lookupResolution: Record<string, LookupResolution> = $state({}); // fieldName -> resolution config
	let missingLookups: MissingLookup[] = $state([]); // Lookups that need to be created
	let newRecordData: Record<string, Record<string, Record<string, any>>> = $state({}); // fieldName -> matchValue -> field values
	let analyzeResult: AnalyzeResult | null = $state(null);
	let importResult: ImportResponse | null = $state(null);
	let loading = $state(false);
	let error = $state('');
	let availableFields: AvailableField[] = $state([]);

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

			const response = await fetch(`${API_BASE}/entities/${entityName}/import/csv/preview`, {
				method: 'POST',
				body: formData,
				credentials: 'include',
				headers: {
					'Authorization': `Bearer ${auth.accessToken || ''}`,
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

			// Get available fields for dropdowns (use the full list from backend)
			availableFields = previewData.availableFields || [];
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
			// Check if any lookups have createIfNotFound enabled
			const hasCreateIfNotFound = Object.values(lookupResolution).some(r => r.createIfNotFound);

			// If createIfNotFound is enabled, first check for missing lookups
			if (hasCreateIfNotFound) {
				const lookupFormData = new FormData();
				lookupFormData.append('file', file);
				lookupFormData.append('options', JSON.stringify({ columnMapping, lookupResolution }));

				const lookupResponse = await fetch(`${API_BASE}/entities/${entityName}/import/csv/analyze-lookups`, {
					method: 'POST',
					body: lookupFormData,
					credentials: 'include',
					headers: {
						'Authorization': `Bearer ${auth.accessToken || ''}`,
						'X-CSRF-Token': getCsrfToken()
					}
				});

				if (lookupResponse.ok) {
					const lookupResult: AnalyzeLookupResponse = await lookupResponse.json();
					if (lookupResult.missingLookups && lookupResult.missingLookups.length > 0) {
						missingLookups = lookupResult.missingLookups;
						// Initialize newRecordData for each missing lookup
						for (const lookup of missingLookups) {
							if (!newRecordData[lookup.fieldName]) {
								newRecordData[lookup.fieldName] = {};
							}
							if (!newRecordData[lookup.fieldName][lookup.matchValue]) {
								newRecordData[lookup.fieldName][lookup.matchValue] = {};
							}
						}
						step = 2.5; // Show create lookups step
						loading = false;
						return;
					}
				}
			}

			// Standard validation
			const formData = new FormData();
			formData.append('file', file);
			formData.append('options', JSON.stringify({ columnMapping }));

			const response = await fetch(`${API_BASE}/entities/${entityName}/import/csv/analyze`, {
				method: 'POST',
				body: formData,
				credentials: 'include',
				headers: {
					'Authorization': `Bearer ${auth.accessToken || ''}`,
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
			// Build final lookup resolution with newRecordData
			const finalLookupResolution: Record<string, LookupResolution> = {};
			for (const [fieldName, resolution] of Object.entries(lookupResolution)) {
				finalLookupResolution[fieldName] = {
					...resolution,
					newRecordData: newRecordData[fieldName] || undefined
				};
			}

			const formData = new FormData();
			formData.append('file', file);
			formData.append('options', JSON.stringify({
				columnMapping,
				lookupResolution: Object.keys(finalLookupResolution).length > 0 ? finalLookupResolution : undefined,
				mode: 'create',
				skipErrors: false
			}));

			const response = await fetch(`${API_BASE}/entities/${entityName}/import/csv`, {
				method: 'POST',
				body: formData,
				credentials: 'include',
				headers: {
					'Authorization': `Bearer ${auth.accessToken || ''}`,
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
		} else if (step === 2.5) {
			step = 1;
			missingLookups = [];
		}
	}

	// Proceed from create lookups step to validation
	async function proceedFromLookups() {
		loading = true;
		error = '';

		try {
			const formData = new FormData();
			formData.append('file', file!);
			formData.append('options', JSON.stringify({ columnMapping }));

			const response = await fetch(`${API_BASE}/entities/${entityName}/import/csv/analyze`, {
				method: 'POST',
				body: formData,
				credentials: 'include',
				headers: {
					'Authorization': `Bearer ${auth.accessToken || ''}`,
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

	function updateMapping(csvHeader: string, fieldName: string) {
		columnMapping[csvHeader] = fieldName;
		// Clear lookup resolution if field changed
		const baseFieldName = fieldName.replace(/Id$/, '');
		if (!fieldName || !isLinkField(baseFieldName)) {
			delete lookupResolution[baseFieldName];
		}
	}

	function isLinkField(fieldName: string): boolean {
		const field = availableFields.find(f => f.name === fieldName);
		return field?.type === 'link';
	}

	function getFieldForMapping(mappedFieldName: string): AvailableField | undefined {
		// mappedFieldName might be "accountId" or "account"
		const baseFieldName = mappedFieldName.replace(/Id$/, '');
		return availableFields.find(f => f.name === baseFieldName || f.name === mappedFieldName);
	}

	function toggleLookupByName(fieldName: string, enabled: boolean) {
		if (enabled) {
			lookupResolution[fieldName] = { matchField: 'name', createIfNotFound: false };
		} else {
			delete lookupResolution[fieldName];
		}
		// Trigger reactivity
		lookupResolution = { ...lookupResolution };
	}

	function toggleCreateIfNotFound(fieldName: string, enabled: boolean) {
		if (lookupResolution[fieldName]) {
			lookupResolution[fieldName] = {
				...lookupResolution[fieldName],
				createIfNotFound: enabled
			};
			lookupResolution = { ...lookupResolution };
		}
	}

	function updateNewRecordField(fieldName: string, matchValue: string, field: string, value: string) {
		if (!newRecordData[fieldName]) {
			newRecordData[fieldName] = {};
		}
		if (!newRecordData[fieldName][matchValue]) {
			newRecordData[fieldName][matchValue] = {};
		}
		newRecordData[fieldName][matchValue][field] = value;
		newRecordData = { ...newRecordData };
	}

	// Check if all required fields are filled for missing lookups
	function canProceedFromLookups(): boolean {
		for (const lookup of missingLookups) {
			for (const reqField of lookup.requiredFields) {
				const value = newRecordData[lookup.fieldName]?.[lookup.matchValue]?.[reqField.name];
				if (!value || value.trim() === '') {
					return false;
				}
			}
		}
		return true;
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
									<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Options</th>
								</tr>
							</thead>
							<tbody class="bg-white divide-y divide-gray-200">
								{#each previewData.headers as header, idx}
									{@const mappedField = getFieldForMapping(columnMapping[header] || '')}
									{@const isLink = mappedField?.type === 'link'}
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
													<option value={field.name}>{field.label || field.name}</option>
												{/each}
											</select>
										</td>
										<td class="px-4 py-3">
											{#if isLink && mappedField}
												<div class="space-y-2">
													<label class="flex items-center text-sm text-gray-600 cursor-pointer">
														<input
															type="checkbox"
															checked={!!lookupResolution[mappedField.name]}
															onchange={(e) => toggleLookupByName(mappedField.name, e.currentTarget.checked)}
															class="mr-2 h-4 w-4 text-blue-600 border-gray-300 rounded focus:ring-blue-500"
														/>
														<span>Look up by name</span>
													</label>
													{#if lookupResolution[mappedField.name]}
														<label class="flex items-center text-sm text-gray-600 cursor-pointer ml-6">
															<input
																type="checkbox"
																checked={!!lookupResolution[mappedField.name]?.createIfNotFound}
																onchange={(e) => toggleCreateIfNotFound(mappedField.name, e.currentTarget.checked)}
																class="mr-2 h-4 w-4 text-green-600 border-gray-300 rounded focus:ring-green-500"
															/>
															<span>Create if not found</span>
														</label>
														<p class="text-xs text-gray-500 ml-6">
															{lookupResolution[mappedField.name]?.createIfNotFound
																? `Will create new ${mappedField.relatedEntity || 'record'} if not found`
																: `Will match ${mappedField.relatedEntity || 'record'} by name`}
														</p>
													{/if}
												</div>
											{:else}
												<span class="text-gray-400 text-sm">—</span>
											{/if}
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

	<!-- Step 2.5: Create Missing Lookups -->
	{#if step === 2.5 && missingLookups.length > 0}
		<div class="bg-white shadow rounded-lg p-6">
			<h2 class="text-xl font-semibold mb-4">Create New Records</h2>
			<p class="text-gray-600 mb-4">
				The following records will be created during import. Please fill in the required fields.
			</p>

			{#each missingLookups as lookup}
				<div class="border rounded-lg p-4 mb-4">
					<h3 class="font-medium text-gray-900 mb-2">
						New {lookup.relatedEntity}: <span class="text-blue-600">{lookup.matchValue}</span>
					</h3>
					<p class="text-sm text-gray-500 mb-3">
						Referenced by {lookup.rowIndices.length} row{lookup.rowIndices.length > 1 ? 's' : ''} in your CSV
					</p>

					{#if lookup.requiredFields.length > 0}
						<div class="grid grid-cols-2 gap-4">
							{#each lookup.requiredFields as reqField}
								<div>
									<label class="block text-sm font-medium text-gray-700 mb-1">
										{reqField.label || reqField.name} <span class="text-red-500">*</span>
									</label>
									<input
										type="text"
										value={newRecordData[lookup.fieldName]?.[lookup.matchValue]?.[reqField.name] || ''}
										oninput={(e) => updateNewRecordField(lookup.fieldName, lookup.matchValue, reqField.name, e.currentTarget.value)}
										class="block w-full border-gray-300 rounded-md shadow-sm focus:ring-blue-500 focus:border-blue-500 sm:text-sm"
										placeholder={`Enter ${reqField.label || reqField.name}`}
									/>
								</div>
							{/each}
						</div>
					{:else}
						<p class="text-sm text-green-600">No additional required fields needed.</p>
					{/if}
				</div>
			{/each}

			<div class="flex justify-between mt-6">
				<button
					onclick={goBack}
					class="px-4 py-2 border border-gray-300 rounded-md text-gray-700 hover:bg-gray-50"
				>
					Back
				</button>
				<button
					onclick={proceedFromLookups}
					disabled={loading || !canProceedFromLookups()}
					class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50"
					title={!canProceedFromLookups() ? 'Please fill in all required fields' : ''}
				>
					{loading ? 'Validating...' : 'Continue'}
				</button>
			</div>
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
