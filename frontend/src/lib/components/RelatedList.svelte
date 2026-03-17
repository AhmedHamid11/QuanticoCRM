<script lang="ts">
	import { goto } from '$app/navigation';
	import { get, post, patch } from '$lib/utils/api';
	import { getNavigationTabs } from '$lib/stores/navigation.svelte';
	import { TableSkeleton, ErrorDisplay } from '$lib/components/ui';
	import { toast } from '$lib/stores/toast.svelte';
	import type { RelatedListConfig, RelatedRecordsResponse } from '$lib/types/related-list';
	import type { FieldDef } from '$lib/types/admin';
	import LookupField from '$lib/components/LookupField.svelte';

	interface Props {
		config: RelatedListConfig;
		parentEntity: string;
		parentId: string;
		onCreateNew?: () => void;
	}

	let { config, parentEntity, parentId, onCreateNew }: Props = $props();

	let data = $state<RelatedRecordsResponse | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let currentPage = $state(1);
	let sortField = $state(config.defaultSort || 'createdAt');
	let sortDir = $state<'asc' | 'desc'>(config.defaultSortDir || 'desc');

	// Inline creation state
	let isAddingInline = $state(false);
	let newRecord = $state<Record<string, unknown>>({});
	let saving = $state(false);
	let inlineError = $state<string | null>(null);

	// Inline row editing state
	let editingRowId = $state<string | null>(null);
	let editingRowData = $state<Record<string, unknown>>({});
	let originalRowData = $state<Record<string, unknown>>({});
	let savingRow = $state(false);
	let rowSaveSuccess = $state<string | null>(null);

	// Field definitions for inline editing
	let fieldDefs = $state<FieldDef[]>([]);
	let fieldDefsLoaded = $state(false);
	let lookupNames = $state<Record<string, string>>({});

	// Get the correct URL path for an entity from navigation tabs
	function getEntityPath(entityName: string): string {
		const tabs = getNavigationTabs();
		const tab = tabs.find(t => t.entityName === entityName);
		if (tab) {
			return tab.href;
		}
		// Fallback: lowercase + 's' with encoding
		return '/' + encodeURIComponent(entityName.toLowerCase() + 's');
	}

	async function loadData() {
		try {
			loading = true;
			error = null;
			const params = new URLSearchParams({
				page: String(currentPage),
				pageSize: String(config.pageSize || 5),
				sort: sortField,
				dir: sortDir
			});
			data = await get<RelatedRecordsResponse>(
				`/${parentEntity.toLowerCase()}s/${parentId}/related/${config.relatedEntity}?${params}`
			);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load related records';
		} finally {
			loading = false;
		}
	}

	function toggleSort(field: string) {
		if (sortField === field) {
			sortDir = sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			sortField = field;
			sortDir = 'desc';
		}
		currentPage = 1;
		loadData();
	}

	function goToPage(page: number) {
		currentPage = page;
		loadData();
	}

	function navigateToRecord(recordId: string) {
		const entityPath = getEntityPath(config.relatedEntity);
		goto(`${entityPath}/${recordId}`);
	}

	async function handleCreateNew() {
		if (onCreateNew) {
			onCreateNew();
		} else if (config.editInList) {
			// Inline editing mode: show editable row
			// Load field definitions if not already loaded
			await loadFieldDefs();
			isAddingInline = true;
			inlineError = null;
			lookupNames = {};
			// Pre-fill the lookup field
			// The backend generic entity handler expects the exact field name from metadata
			// config.lookupField is already the correct key (e.g., "accountId", "parentId")
			if (config.relatedEntity === 'Task') {
				newRecord = { parentType: parentEntity, parentId: parentId };
			} else {
				newRecord = { [config.lookupField]: parentId };
			}
		} else {
			// Default: navigate to create page with pre-filled lookup
			const entityPath = getEntityPath(config.relatedEntity);

			// Special handling for Task entity (polymorphic relationship)
			if (config.relatedEntity === 'Task') {
				goto(`${entityPath}/new?parentType=${parentEntity}&parentId=${parentId}`);
			} else {
				goto(`${entityPath}/new?${config.lookupField}=${parentId}`);
			}
		}
	}

	async function saveInlineRecord() {
		try {
			saving = true;
			inlineError = null;

			// Auto-fill name if not provided and not in display fields
			const recordToSave = { ...newRecord };
			const nameInDisplayFields = config.displayFields.some(f => f.field === 'name');
			if (!nameInDisplayFields && !recordToSave.name) {
				// Try to build name from firstName/lastName if available
				const firstName = recordToSave.firstName || recordToSave.first_name || '';
				const lastName = recordToSave.lastName || recordToSave.last_name || '';
				if (firstName || lastName) {
					recordToSave.name = `${firstName} ${lastName}`.trim();
				} else {
					// Default to timestamp-based name
					recordToSave.name = `${config.relatedEntity} ${Date.now()}`;
				}
			}

			// POST to create new record
			await post(`/entities/${config.relatedEntity}/records`, recordToSave);

			// Success: reset state and refresh list
			isAddingInline = false;
			newRecord = {};
			toast.success(`${config.relatedEntity} created successfully`);
			await loadData();
		} catch (e) {
			inlineError = e instanceof Error ? e.message : 'Failed to create record';
			toast.error(inlineError);
		} finally {
			saving = false;
		}
	}

	function cancelInline() {
		isAddingInline = false;
		newRecord = {};
		lookupNames = {};
		inlineError = null;
	}

	function updateNewRecordField(field: string, value: unknown) {
		newRecord = { ...newRecord, [field]: value };
	}

	function updateEditingField(field: string, value: unknown) {
		editingRowData = { ...editingRowData, [field]: value };
	}

	async function startEditing(record: Record<string, unknown>) {
		// If another row is being edited, cancel it first
		if (editingRowId !== null) {
			cancelEditingRow();
		}

		// Load field definitions if not already loaded
		await loadFieldDefs();

		// Build editingRowData from current record values
		const rowData: Record<string, unknown> = {};
		for (const fieldConfig of config.displayFields) {
			const key = findRecordKey(record, fieldConfig.field);
			if (key !== null) {
				rowData[fieldConfig.field] = record[key];
			} else {
				rowData[fieldConfig.field] = '';
			}
		}

		// Pre-populate lookupNames from existing *Name fields in the record
		const newLookupNames: Record<string, string> = {};
		for (const fieldConfig of config.displayFields) {
			const fieldDef = getFieldDef(fieldConfig.field);
			if (fieldDef?.type === 'link') {
				const nameKey = findRecordKey(record, `${fieldConfig.field}Name`);
				if (nameKey !== null && record[nameKey]) {
					newLookupNames[fieldConfig.field] = String(record[nameKey]);
				}
			}
		}
		lookupNames = { ...lookupNames, ...newLookupNames };

		editingRowData = rowData;
		originalRowData = { ...rowData };
		editingRowId = record.id as string;
	}

	async function saveEditedRow() {
		if (!editingRowId || !data) return;

		savingRow = true;

		// Optimistically update the record in data.records
		const recordIndex = data.records.findIndex(r => (r.id as string) === editingRowId);
		if (recordIndex !== -1) {
			const updatedRecords = [...data.records];
			const existingRecord = updatedRecords[recordIndex] as Record<string, unknown>;
			const updatedRecord = { ...existingRecord };

			// Apply editingRowData values to the record
			for (const [field, value] of Object.entries(editingRowData)) {
				const key = findRecordKey(existingRecord, field);
				if (key !== null) {
					updatedRecord[key] = value;
				} else {
					updatedRecord[field] = value;
				}
			}
			updatedRecords[recordIndex] = updatedRecord;
			data = { ...data, records: updatedRecords };
		}

		const savedRowId = editingRowId;

		try {
			await patch(`/entities/${config.relatedEntity}/records/${editingRowId}`, editingRowData);

			// Success: clear edit state and show green flash
			editingRowId = null;
			editingRowData = {};
			originalRowData = {};
			rowSaveSuccess = savedRowId;
			toast.success('Record updated successfully');

			// Clear green flash after 1.5s
			setTimeout(() => {
				rowSaveSuccess = null;
			}, 1500);
		} catch (e) {
			// Failure: revert to original values
			if (data && recordIndex !== -1) {
				const revertedRecords = [...data.records];
				const existingRecord = revertedRecords[recordIndex] as Record<string, unknown>;
				const revertedRecord = { ...existingRecord };

				for (const [field, value] of Object.entries(originalRowData)) {
					const key = findRecordKey(existingRecord, field);
					if (key !== null) {
						revertedRecord[key] = value;
					} else {
						revertedRecord[field] = value;
					}
				}
				revertedRecords[recordIndex] = revertedRecord;
				data = { ...data, records: revertedRecords };
			}

			const errorMsg = e instanceof Error ? e.message : 'Failed to update record';
			toast.error(errorMsg);
			// Keep row in edit mode so user can retry or cancel
		} finally {
			savingRow = false;
		}
	}

	function cancelEditingRow() {
		editingRowId = null;
		editingRowData = {};
		originalRowData = {};
	}

	// Load field definitions for the related entity (for inline editing)
	async function loadFieldDefs() {
		if (fieldDefsLoaded) return;
		try {
			fieldDefs = await get<FieldDef[]>(`/entities/${config.relatedEntity}/fields`);
			fieldDefsLoaded = true;
		} catch {
			fieldDefs = [];
		}
	}

	// Get field definition by name
	function getFieldDef(fieldName: string): FieldDef | undefined {
		// Try exact match first
		let def = fieldDefs.find(f => f.name === fieldName);
		if (def) return def;

		// Try without Id suffix for lookup fields
		if (fieldName.endsWith('Id')) {
			const baseName = fieldName.slice(0, -2);
			def = fieldDefs.find(f => f.name === baseName || f.name === baseName + '_id');
			if (def) return def;
		}

		// Try snake_case to camelCase conversion
		const camelName = snakeToCamel(fieldName);
		return fieldDefs.find(f => f.name === camelName);
	}

	// Get the HTML input type for a field
	function getInputType(field: FieldDef): string {
		switch (field.type) {
			case 'email': return 'email';
			case 'url': return 'url';
			case 'phone': return 'tel';
			case 'int': case 'float': case 'currency': return 'number';
			case 'date': return 'date';
			case 'datetime': return 'datetime-local';
			default: return 'text';
		}
	}

	// Parse enum options from field definition
	function getEnumOptions(field: FieldDef): string[] {
		if (!field.options) return [];
		const opts = field.options.trim();
		if (opts.startsWith('[')) {
			try {
				return JSON.parse(opts);
			} catch {
				return [];
			}
		}
		return opts.split(',').map(o => o.trim());
	}

	function formatFieldValue(value: unknown, fieldName: string): string {
		if (value === null || value === undefined) return '-';

		// Handle dates - only fields ending with "At" or "_at" (e.g., createdAt, modified_at)
		const isDateField = /(?:At|_at|Date)$/i.test(fieldName);
		if (isDateField && typeof value === 'string') {
			try {
				const date = new Date(value);
				if (!isNaN(date.getTime())) {
					return date.toLocaleDateString();
				}
			} catch {
				// Fall through to return string
			}
		}

		// Handle booleans
		if (typeof value === 'boolean') {
			return value ? 'Yes' : 'No';
		}

		return String(value);
	}

	// Convert snake_case to camelCase
	function snakeToCamel(s: string): string {
		return s.replace(/_([a-z])/g, (_, letter) => letter.toUpperCase());
	}

	// Find a key in the record, trying exact match first, then snake_case->camelCase, then case-insensitive
	function findRecordKey(record: Record<string, unknown>, fieldName: string): string | null {
		// Exact match first
		if (fieldName in record) {
			return fieldName;
		}
		// Try converting snake_case to camelCase (handles displayFields saved as snake_case)
		if (fieldName.includes('_')) {
			const camelName = snakeToCamel(fieldName);
			if (camelName in record) {
				return camelName;
			}
		}
		// Case-insensitive fallback
		const lowerField = fieldName.toLowerCase().replace(/_/g, '');
		for (const key of Object.keys(record)) {
			if (key.toLowerCase() === lowerField) {
				return key;
			}
		}
		return null;
	}

	function getDisplayValue(record: Record<string, unknown>, fieldConfig: { field: string }): string {
		const fieldName = fieldConfig.field;

		// For lookup fields (ending in Id), check for the *Name variant first
		if (fieldName.endsWith('Id')) {
			const nameField = fieldName + 'Name';
			const foundNameKey = findRecordKey(record, nameField);
			if (foundNameKey && record[foundNameKey] !== undefined && record[foundNameKey] !== null) {
				return String(record[foundNameKey]);
			}
		}

		// Find the actual key in the record (with case-insensitive fallback)
		const actualKey = findRecordKey(record, fieldName);
		if (actualKey) {
			const value = record[actualKey];
			return formatFieldValue(value, fieldName);
		}

		// For link fields configured without "Id" suffix, try {fieldName}Name first (display name)
		// This handles cases where displayField is "form433D" but record has "form433DId" and "form433DName"
		const nameVariant = fieldName + 'Name';
		const foundNameKey = findRecordKey(record, nameVariant);
		if (foundNameKey && record[foundNameKey] !== undefined && record[foundNameKey] !== null) {
			return String(record[foundNameKey]);
		}

		// Also try {fieldName}Id if name variant not found
		const idVariant = fieldName + 'Id';
		const foundIdKey = findRecordKey(record, idVariant);
		if (foundIdKey && record[foundIdKey] !== undefined && record[foundIdKey] !== null) {
			return String(record[foundIdKey]);
		}

		// Check customFields object for custom entity fields
		const customFields = record['customFields'] as Record<string, unknown> | undefined;
		if (customFields && typeof customFields === 'object') {
			const customKey = findRecordKey(customFields, fieldName);
			if (customKey) {
				const value = customFields[customKey];
				return formatFieldValue(value, fieldName);
			}
		}

		// Debug logging when field is not found even with case-insensitive matching
		console.warn(`[RelatedList] Field "${fieldName}" not found in record. Available keys:`, Object.keys(record),
			customFields ? `customFields keys: ${Object.keys(customFields)}` : '(no customFields)');
		return '-';
	}

	// Load data on mount
	$effect(() => {
		loadData();
	});
</script>

<div class="crm-card overflow-hidden">
	<!-- Header -->
	<div class="px-6 py-4 border-b border-gray-200 flex items-center justify-between">
		<h3 class="text-lg font-medium text-gray-900">
			{config.label}
			{#if data}
				<span class="text-sm font-normal text-gray-500">({data.total})</span>
			{/if}
		</h3>
		<button
			onclick={handleCreateNew}
			class="px-3 py-1.5 text-sm bg-blue-600 text-white rounded-md hover:bg-blue-600/90"
		>
			+ New
		</button>
	</div>

	<!-- Content -->
	<div class="overflow-x-auto">
		{#if loading}
			<TableSkeleton rows={config.pageSize || 5} columns={config.displayFields.length || 3} showHeader={true} />
		{:else if error}
			<ErrorDisplay message={error} onRetry={loadData} />
		{:else if !data?.records?.length && !isAddingInline}
			<div class="px-6 py-8 text-center text-gray-500">No records found</div>
		{:else}
			<table class="min-w-full divide-y divide-gray-200">
				<thead class="bg-gray-50">
					<tr>
						{#each config.displayFields as fieldConfig}
							<th
								class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
								style={fieldConfig.width ? `width: ${fieldConfig.width}%` : ''}
								onclick={() => toggleSort(fieldConfig.field)}
							>
								<div class="flex items-center gap-1">
									{fieldConfig.label || fieldConfig.field}
									{#if sortField === fieldConfig.field}
										<span class="text-blue-600">
											{sortDir === 'asc' ? '↑' : '↓'}
										</span>
									{/if}
								</div>
							</th>
						{/each}
						{#if isAddingInline || editingRowId !== null}
							<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider w-24">
								Actions
							</th>
						{/if}
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-200">
					{#if isAddingInline}
						<tr class="bg-blue-50 border-blue-200">
							{#each config.displayFields as fieldConfig}
								{@const fieldDef = getFieldDef(fieldConfig.field)}
								<td class="px-4 py-2">
									{#if fieldDef?.type === 'link' && fieldDef.linkEntity}
										<!-- Lookup field -->
										<LookupField
											entity={fieldDef.linkEntity}
											value={newRecord[`${fieldConfig.field}Id`] as string | null || newRecord[fieldConfig.field] as string | null}
											valueName={lookupNames[fieldConfig.field] || ''}
											label=""
											required={fieldDef.isRequired}
											onchange={(id, name) => {
												newRecord = {
													...newRecord,
													[`${fieldConfig.field}Id`]: id,
													[`${fieldConfig.field}Name`]: name,
													[fieldConfig.field]: id
												};
												lookupNames = { ...lookupNames, [fieldConfig.field]: name };
											}}
										/>
									{:else if fieldDef?.type === 'enum' && fieldDef.options}
										<!-- Picklist/Enum field -->
										<select
											class="w-full px-2 py-1 text-sm border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
											value={(newRecord[fieldConfig.field] as string) || ''}
											onchange={(e) => updateNewRecordField(fieldConfig.field, (e.target as HTMLSelectElement).value)}
											disabled={saving}
										>
											<option value="">Select...</option>
											{#each getEnumOptions(fieldDef) as option}
												<option value={option}>{option}</option>
											{/each}
										</select>
									{:else if fieldDef?.type === 'bool'}
										<!-- Boolean/Checkbox field -->
										<input
											type="checkbox"
											class="w-4 h-4 rounded text-blue-600 focus:ring-blue-500 border-gray-300"
											checked={!!newRecord[fieldConfig.field]}
											onchange={(e) => updateNewRecordField(fieldConfig.field, (e.target as HTMLInputElement).checked)}
											disabled={saving}
										/>
									{:else if fieldDef?.type === 'text'}
										<!-- Text area field (but keep compact for inline) -->
										<input
											type="text"
											class="w-full px-2 py-1 text-sm border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
											placeholder={fieldConfig.label || fieldConfig.field}
											value={(newRecord[fieldConfig.field] as string) || ''}
											oninput={(e) => updateNewRecordField(fieldConfig.field, (e.target as HTMLInputElement).value)}
											disabled={saving}
										/>
									{:else}
										<!-- Default: use appropriate input type -->
										<input
											type={fieldDef ? getInputType(fieldDef) : 'text'}
											class="w-full px-2 py-1 text-sm border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
											placeholder={fieldConfig.label || fieldConfig.field}
											value={(newRecord[fieldConfig.field] as string) || ''}
											oninput={(e) => updateNewRecordField(fieldConfig.field, (e.target as HTMLInputElement).value)}
											disabled={saving}
											step={fieldDef?.type === 'float' || fieldDef?.type === 'currency' ? '0.01' : undefined}
										/>
									{/if}
								</td>
							{/each}
							<td class="px-4 py-2 whitespace-nowrap">
								<div class="flex gap-2">
									<button
										onclick={saveInlineRecord}
										disabled={saving}
										class="px-2 py-1 text-xs bg-green-600 text-white rounded hover:bg-green-700 disabled:opacity-50 disabled:cursor-not-allowed"
									>
										{saving ? 'Saving...' : 'Save'}
									</button>
									<button
										onclick={cancelInline}
										disabled={saving}
										class="px-2 py-1 text-xs bg-gray-300 text-gray-700 rounded hover:bg-gray-400 disabled:opacity-50"
									>
										Cancel
									</button>
								</div>
							</td>
						</tr>
					{/if}
					{#each data!.records as record}
						<tr
							class="hover:bg-gray-50 {editingRowId === (record.id as string) ? 'bg-yellow-50 ring-1 ring-inset ring-yellow-200' : ''} {rowSaveSuccess === (record.id as string) ? 'bg-green-50 transition-colors duration-1000' : ''} {editingRowId !== (record.id as string) ? 'cursor-pointer' : ''}"
							onclick={() => { if (editingRowId !== (record.id as string)) navigateToRecord(record.id as string); }}
							onkeydown={(e) => {
								if (editingRowId === (record.id as string)) {
									if (e.key === 'Enter') { e.preventDefault(); saveEditedRow(); }
									if (e.key === 'Escape') cancelEditingRow();
								}
							}}
						>
							{#each config.displayFields as fieldConfig}
								{#if editingRowId === (record.id as string)}
									<!-- EDIT MODE: render field-type-aware input -->
									{@const fieldDef = getFieldDef(fieldConfig.field)}
									<td class="px-4 py-2" onclick={(e) => e.stopPropagation()}>
										{#if fieldDef?.type === 'link' && fieldDef.linkEntity}
											<!-- Lookup field -->
											<LookupField
												entity={fieldDef.linkEntity}
												value={editingRowData[`${fieldConfig.field}Id`] as string | null || editingRowData[fieldConfig.field] as string | null}
												valueName={lookupNames[fieldConfig.field] || ''}
												label=""
												required={fieldDef.isRequired}
												onchange={(id, name) => {
													editingRowData = {
														...editingRowData,
														[`${fieldConfig.field}Id`]: id,
														[`${fieldConfig.field}Name`]: name,
														[fieldConfig.field]: id
													};
													lookupNames = { ...lookupNames, [fieldConfig.field]: name };
												}}
											/>
										{:else if fieldDef?.type === 'enum' && fieldDef.options}
											<!-- Picklist/Enum field -->
											<select
												class="w-full px-2 py-1 text-sm border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
												value={(editingRowData[fieldConfig.field] as string) || ''}
												onchange={(e) => updateEditingField(fieldConfig.field, (e.target as HTMLSelectElement).value)}
												disabled={savingRow}
											>
												<option value="">Select...</option>
												{#each getEnumOptions(fieldDef) as option}
													<option value={option}>{option}</option>
												{/each}
											</select>
										{:else if fieldDef?.type === 'bool'}
											<!-- Boolean/Checkbox field -->
											<input
												type="checkbox"
												class="w-4 h-4 rounded text-blue-600 focus:ring-blue-500 border-gray-300"
												checked={!!editingRowData[fieldConfig.field]}
												onchange={(e) => updateEditingField(fieldConfig.field, (e.target as HTMLInputElement).checked)}
												disabled={savingRow}
											/>
										{:else if fieldDef?.type === 'text'}
											<!-- Text area field (keep compact for inline) -->
											<input
												type="text"
												class="w-full px-2 py-1 text-sm border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
												placeholder={fieldConfig.label || fieldConfig.field}
												value={(editingRowData[fieldConfig.field] as string) || ''}
												oninput={(e) => updateEditingField(fieldConfig.field, (e.target as HTMLInputElement).value)}
												disabled={savingRow}
											/>
										{:else}
											<!-- Default: use appropriate input type -->
											<input
												type={fieldDef ? getInputType(fieldDef) : 'text'}
												class="w-full px-2 py-1 text-sm border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
												placeholder={fieldConfig.label || fieldConfig.field}
												value={(editingRowData[fieldConfig.field] as string) || ''}
												oninput={(e) => updateEditingField(fieldConfig.field, (e.target as HTMLInputElement).value)}
												disabled={savingRow}
												step={fieldDef?.type === 'float' || fieldDef?.type === 'currency' ? '0.01' : undefined}
											/>
										{/if}
									</td>
								{:else}
									<!-- VIEW MODE: clickable cell -->
									<td
										class="px-4 py-3 text-sm text-gray-900 {config.editInList ? 'hover:bg-blue-50 hover:cursor-cell' : ''}"
										onclick={(e) => { if (config.editInList) { e.stopPropagation(); startEditing(record as Record<string, unknown>); } }}
									>
										{getDisplayValue(record, fieldConfig)}
									</td>
								{/if}
							{/each}
							<!-- Actions column: save/cancel when editing, empty cell when add-inline active -->
							{#if editingRowId === (record.id as string)}
								<td class="px-4 py-2 whitespace-nowrap" onclick={(e) => e.stopPropagation()}>
									<div class="flex items-center gap-2">
										<button
											onclick={saveEditedRow}
											disabled={savingRow}
											class="px-2 py-1 text-xs bg-green-600 text-white rounded hover:bg-green-700 disabled:opacity-50"
										>
											{savingRow ? 'Saving...' : 'Save'}
										</button>
										<button
											onclick={cancelEditingRow}
											disabled={savingRow}
											class="px-2 py-1 text-xs bg-gray-300 text-gray-700 rounded hover:bg-gray-400"
										>
											Cancel
										</button>
									<button
										onclick={() => navigateToRecord(record.id as string)}
										title="Open record"
										class="p-1 text-gray-400 hover:text-blue-600 rounded hover:bg-blue-50"
									>
										<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
											<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
										</svg>
									</button>
								</td>
							{:else if editingRowId !== null || isAddingInline}
								<td></td>
							{/if}
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}
	</div>

	<!-- Pagination -->
	{#if data && data.totalPages > 1}
		<div class="px-6 py-3 border-t border-gray-200 flex items-center justify-between">
			<div class="text-sm text-gray-500">
				Showing {(currentPage - 1) * config.pageSize + 1} - {Math.min(currentPage * config.pageSize, data.total)} of {data.total}
			</div>
			<div class="flex gap-1">
				<button
					onclick={() => goToPage(currentPage - 1)}
					disabled={currentPage === 1}
					class="px-3 py-1 text-sm border rounded hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					Previous
				</button>
				{#each Array(Math.min(data.totalPages, 5)) as _, i}
					{@const pageNum = i + 1}
					<button
						onclick={() => goToPage(pageNum)}
						class="px-3 py-1 text-sm border rounded {currentPage === pageNum ? 'bg-blue-600 text-white border-blue-500' : 'hover:bg-gray-50'}"
					>
						{pageNum}
					</button>
				{/each}
				<button
					onclick={() => goToPage(currentPage + 1)}
					disabled={currentPage === data.totalPages}
					class="px-3 py-1 text-sm border rounded hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					Next
				</button>
			</div>
		</div>
	{/if}
</div>
