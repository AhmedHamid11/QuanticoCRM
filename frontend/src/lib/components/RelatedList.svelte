<script lang="ts">
	import { goto } from '$app/navigation';
	import { get, post } from '$lib/utils/api';
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

	// Inline editing state
	let isAddingInline = $state(false);
	let newRecord = $state<Record<string, unknown>>({});
	let saving = $state(false);
	let inlineError = $state<string | null>(null);

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
			// The handler expects {fieldName}Id format for lookup fields
			// If lookupField already ends with "Id", use it as-is, otherwise append "Id"
			if (config.relatedEntity === 'Task') {
				newRecord = { parentType: parentEntity, parentId: parentId };
			} else {
				// For lookup fields, the API expects {fieldName}Id format
				// e.g., for field "accountId", send "accountIdId" (weird but that's how it works)
				const idKey = config.lookupField.endsWith('Id')
					? config.lookupField + 'Id'  // accountId -> accountIdId
					: config.lookupField + 'Id'; // account -> accountId
				newRecord = { [idKey]: parentId };
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

<div class="bg-white shadow rounded-lg overflow-hidden">
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
			class="px-3 py-1.5 text-sm bg-primary text-black rounded-md hover:bg-primary/90"
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
										<span class="text-primary">
											{sortDir === 'asc' ? '↑' : '↓'}
										</span>
									{/if}
								</div>
							</th>
						{/each}
						{#if isAddingInline}
							<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider w-24">
								Actions
							</th>
						{/if}
					</tr>
				</thead>
				<tbody class="bg-white divide-y divide-gray-200">
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
											class="w-full px-2 py-1 text-sm border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary"
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
											class="w-4 h-4 rounded text-primary focus:ring-primary border-gray-300"
											checked={!!newRecord[fieldConfig.field]}
											onchange={(e) => updateNewRecordField(fieldConfig.field, (e.target as HTMLInputElement).checked)}
											disabled={saving}
										/>
									{:else if fieldDef?.type === 'text'}
										<!-- Text area field (but keep compact for inline) -->
										<input
											type="text"
											class="w-full px-2 py-1 text-sm border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary"
											placeholder={fieldConfig.label || fieldConfig.field}
											value={(newRecord[fieldConfig.field] as string) || ''}
											oninput={(e) => updateNewRecordField(fieldConfig.field, (e.target as HTMLInputElement).value)}
											disabled={saving}
										/>
									{:else}
										<!-- Default: use appropriate input type -->
										<input
											type={fieldDef ? getInputType(fieldDef) : 'text'}
											class="w-full px-2 py-1 text-sm border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary"
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
					{#each data.records as record}
						<tr
							class="hover:bg-gray-50 cursor-pointer"
							onclick={() => navigateToRecord(record.id as string)}
						>
							{#each config.displayFields as fieldConfig}
								<td class="px-4 py-3 text-sm text-gray-900">
									{getDisplayValue(record, fieldConfig)}
								</td>
							{/each}
							{#if isAddingInline}
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
						class="px-3 py-1 text-sm border rounded {currentPage === pageNum ? 'bg-primary text-black border-primary' : 'hover:bg-gray-50'}"
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
