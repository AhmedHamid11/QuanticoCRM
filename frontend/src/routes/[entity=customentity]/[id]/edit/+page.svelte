<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { get, put, ApiError } from '$lib/utils/api';
	import { getEntityNameFromPath } from '$lib/stores/navigation.svelte';
	import { fieldNameToKey, getRecordValue } from '$lib/utils/fieldMapping';
	import type { EntityDef, FieldDef } from '$lib/types/admin';
	import type { FieldValidationError } from '$lib/types/validation';
	import LookupField from '$lib/components/LookupField.svelte';
	import MultiLookupField from '$lib/components/MultiLookupField.svelte';
	import StreamField from '$lib/components/StreamField.svelte';
	import ValidationErrors from '$lib/components/ValidationErrors.svelte';

	interface LookupRecord {
		id: string;
		name: string;
	}

	let entitySlug = $derived($page.params.entity);
	let entityName = $derived(getEntityNameFromPath(entitySlug) || toPascalCase(entitySlug));
	let recordId = $derived($page.params.id);

	function toPascalCase(slug: string): string {
		let singular = slug;
		if (slug.endsWith('s') && slug.length > 1) {
			singular = slug.slice(0, -1);
		}
		return singular.charAt(0).toUpperCase() + singular.slice(1);
	}

	let entityDef = $state<EntityDef | null>(null);
	let fields = $state<FieldDef[]>([]);
	let formData = $state<Record<string, unknown>>({});
	let lookupNames = $state<Record<string, string>>({});
	let multiLookupValues = $state<Record<string, LookupRecord[]>>({});
	let loading = $state(true);
	let saving = $state(false);
	let error = $state<string | null>(null);
	let fieldErrors = $state<FieldValidationError[]>([]);

	// Get field error for a specific field
	function getFieldError(fieldName: string): FieldValidationError | undefined {
		return fieldErrors.find((e) => e.field === fieldName);
	}

	// Editable fields (exclude id, system fields)
	let editableFields = $derived(
		fields.filter(f => f.name !== 'id' && !f.isReadOnly)
	);

	// Parse enum options (handles both JSON array and comma-separated formats)
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

	async function loadEntityDef() {
		try {
			// Use public endpoint (doesn't require admin role)
			entityDef = await get<EntityDef>(`/entities/${entityName}/def`);
		} catch {
			entityDef = null;
		}
	}

	async function loadFields() {
		try {
			// Use public endpoint (doesn't require admin role)
			fields = await get<FieldDef[]>(`/entities/${entityName}/fields`);
		} catch {
			fields = [];
		}
	}

	async function loadRecord() {
		try {
			const record = await get<Record<string, unknown>>(`/entities/${entityName}/records/${recordId}`);
			formData = { ...record };

			// Map camelCase record keys to snake_case field names
			// Backend returns camelCase keys, but field.name uses snake_case
			// Uses centralized fieldNameToKey utility from $lib/utils/fieldMapping
			for (const field of fields) {
				const camelKey = fieldNameToKey(field.name);
				if (camelKey !== field.name && camelKey in formData && !(field.name in formData)) {
					formData[field.name] = formData[camelKey];
				}
			}

			// Load lookup display names for link fields from the record data
			// Backend returns {fieldName}Id, {fieldName}Name, {fieldName}Link (in camelCase)
			for (const field of fields) {
				if (field.type === 'link' && field.linkEntity) {
					// Try camelCase key first, then snake_case
					const camelKey = fieldNameToKey(field.name);
					const nameVal = getRecordValue(record, `${camelKey}Name`) || getRecordValue(record, `${field.name}Name`);
					if (nameVal) {
						lookupNames[field.name] = String(nameVal);
					}
				}
				// Load multi-lookup values
				// Backend returns {fieldName}Ids, {fieldName}Names, {fieldName}Links as JSON arrays (in camelCase)
				if (field.type === 'linkMultiple' && field.linkEntity) {
					const camelKey = fieldNameToKey(field.name);
					const idsVal = getRecordValue(record, `${camelKey}Ids`) || getRecordValue(record, `${field.name}Ids`);
					const namesVal = getRecordValue(record, `${camelKey}Names`) || getRecordValue(record, `${field.name}Names`);

					if (idsVal && namesVal && idsVal !== '[]') {
						try {
							const ids = typeof idsVal === 'string' ? JSON.parse(idsVal) : idsVal;
							const names = typeof namesVal === 'string' ? JSON.parse(namesVal) : namesVal;

							if (Array.isArray(ids) && Array.isArray(names)) {
								multiLookupValues[field.name] = ids.map((id: string, i: number) => ({
									id,
									name: names[i] || ''
								}));
							}
						} catch {
							// Not valid JSON, ignore
						}
					}
				}
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load record';
		}
	}

	async function handleSubmit() {
		saving = true;
		error = null;
		fieldErrors = [];

		try {
			await put<Record<string, unknown>>(`/entities/${entityName}/records/${recordId}`, formData);
			goto(`/${entitySlug}/${recordId}`);
		} catch (e) {
			if (e instanceof ApiError && e.fieldErrors) {
				fieldErrors = e.fieldErrors;
				error = e.message;
			} else {
				error = e instanceof Error ? e.message : 'Failed to update record';
			}
		} finally {
			saving = false;
		}
	}

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

	// Reload data when entity or record changes (handles navigation between edit pages)
	$effect(() => {
		// Track these reactive values to trigger reload on navigation
		const _entity = entityName;
		const _recordId = recordId;

		// Reset state
		entityDef = null;
		fields = [];
		formData = {};
		lookupNames = {};
		multiLookupValues = {};
		loading = true;
		error = null;
		fieldErrors = [];

		// Load data
		(async () => {
			await Promise.all([loadEntityDef(), loadFields()]);
			await loadRecord();
			loading = false;
		})();
	});
</script>

<div class="space-y-6">
	<!-- Breadcrumb -->
	<div>
		<nav class="text-sm text-gray-500 mb-2">
			<a href="/{entitySlug}" class="hover:text-gray-700">{entityDef?.labelPlural || entityName + 's'}</a>
			<span class="mx-2">/</span>
			<a href="/{entitySlug}/{recordId}" class="hover:text-gray-700">{formData.name || recordId}</a>
			<span class="mx-2">/</span>
			<span class="text-gray-900">Edit</span>
		</nav>
		<h1 class="text-2xl font-bold text-gray-900">
			Edit {entityDef?.label || entityName}
		</h1>
	</div>

	{#if loading}
		<div class="text-center py-12 text-gray-500">Loading...</div>
	{:else}
		<div class="bg-white shadow rounded-lg p-6 max-w-2xl">
			{#if fieldErrors.length > 0}
				<ValidationErrors errors={fieldErrors} />
			{:else if error}
				<div class="mb-4 p-3 bg-red-50 border border-red-200 rounded text-red-700 text-sm">
					{error}
				</div>
			{/if}

			<form onsubmit={(e) => { e.preventDefault(); handleSubmit(); }}>
				<div class="space-y-4">
					{#each editableFields as field}
						<div>
							{#if field.type === 'link' && field.linkEntity}
								<LookupField
									entity={field.linkEntity}
									value={formData[`${field.name}Id`] as string | null}
									valueName={lookupNames[field.name] || ''}
									label={field.label}
									required={field.isRequired}
									onchange={(id, name) => {
										// Store both ID and Name for lookup fields per standard
										formData[`${field.name}Id`] = id;
										formData[`${field.name}Name`] = name;
										lookupNames[field.name] = name;
									}}
								/>
							{:else if field.type === 'linkMultiple' && field.linkEntity}
								<MultiLookupField
									entity={field.linkEntity}
									values={multiLookupValues[field.name] || []}
									label={field.label}
									required={field.isRequired}
									onchange={(values) => {
										multiLookupValues[field.name] = values;
										// Store as JSON arrays for the API
										formData[`${field.name}Ids`] = JSON.stringify(values.map(v => v.id));
										formData[`${field.name}Names`] = JSON.stringify(values.map(v => v.name));
									}}
								/>
							{:else if field.type === 'stream'}
								<StreamField
									label={field.label}
									entry={String(formData[field.name] || '')}
									log={String(formData[`${field.name}Log`] || '')}
									required={field.isRequired}
									onchange={(value) => {
										formData[field.name] = value;
									}}
								/>
							{:else}
							{@const fieldError = getFieldError(field.name)}
							<label for={field.name} class="block text-sm font-medium mb-1" class:text-gray-700={!fieldError} class:text-red-700={fieldError}>
								{field.label}
								{#if field.isRequired}
									<span class="text-red-500">*</span>
								{/if}
							</label>

							{#if field.type === 'text'}
								<textarea
									id={field.name}
									name={field.name}
									data-field={field.name}
									bind:value={formData[field.name]}
									required={field.isRequired}
									rows="3"
									class="w-full px-3 py-2 border rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
									class:border-gray-300={!fieldError}
									class:border-red-500={fieldError}
								></textarea>
							{:else if field.type === 'bool'}
								<input
									type="checkbox"
									id={field.name}
									name={field.name}
									data-field={field.name}
									bind:checked={formData[field.name]}
									class="w-4 h-4 rounded text-blue-600 focus:ring-blue-500"
									class:border-gray-300={!fieldError}
									class:border-red-500={fieldError}
								/>
							{:else if field.type === 'enum' && field.options}
								<select
									id={field.name}
									name={field.name}
									data-field={field.name}
									bind:value={formData[field.name]}
									required={field.isRequired}
									class="w-full px-3 py-2 border rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
									class:border-gray-300={!fieldError}
									class:border-red-500={fieldError}
								>
									<option value="">Select...</option>
									{#each getEnumOptions(field) as option}
										<option value={option}>{option}</option>
									{/each}
								</select>
							{:else if field.type === 'multiEnum' && field.options}
								{@const options = getEnumOptions(field)}
								{@const selectedValues = (() => {
									const val = formData[field.name];
									if (!val) return [];
									if (typeof val === 'string' && val.startsWith('[')) {
										try { return JSON.parse(val); } catch { return []; }
									}
									return Array.isArray(val) ? val : [];
								})()}
								<div class="space-y-2">
									{#each options as option}
										<label class="flex items-center gap-2">
											<input
												type="checkbox"
												checked={selectedValues.includes(option)}
												onchange={(e) => {
													const checked = (e.target as HTMLInputElement).checked;
													let current = [...selectedValues];
													if (checked && !current.includes(option)) {
														current.push(option);
													} else if (!checked) {
														current = current.filter(v => v !== option);
													}
													formData[field.name] = JSON.stringify(current);
												}}
												class="w-4 h-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
											/>
											<span class="text-sm text-gray-700">{option}</span>
										</label>
									{/each}
								</div>
							{:else}
								<input
									type={getInputType(field)}
									id={field.name}
									name={field.name}
									data-field={field.name}
									bind:value={formData[field.name]}
									required={field.isRequired}
									maxlength={field.maxLength || undefined}
									min={field.minValue || undefined}
									max={field.maxValue || undefined}
									step={field.type === 'float' || field.type === 'currency' ? '0.01' : undefined}
									class="w-full px-3 py-2 border rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
									class:border-gray-300={!fieldError}
									class:border-red-500={fieldError}
								/>
							{/if}

							{#if fieldError}
								<p class="mt-1 text-xs text-red-600">{fieldError.message}</p>
							{:else if field.tooltip}
								<p class="mt-1 text-xs text-gray-500">{field.tooltip}</p>
							{/if}
							{/if}
						</div>
					{/each}
				</div>

				<div class="mt-6 flex justify-end gap-3">
					<a
						href="/{entitySlug}/{recordId}"
						class="px-4 py-2 text-gray-700 bg-gray-100 rounded-lg hover:bg-gray-200 transition-colors"
					>
						Cancel
					</a>
					<button
						type="submit"
						disabled={saving}
						class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
					>
						{saving ? 'Saving...' : 'Save'}
					</button>
				</div>
			</form>
		</div>
	{/if}
</div>
