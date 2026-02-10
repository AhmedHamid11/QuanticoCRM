<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { get, put, ApiError } from '$lib/utils/api';
	import { getEntityNameFromPath } from '$lib/stores/navigation.svelte';
	import { fieldNameToKey, getRecordValue } from '$lib/utils/fieldMapping';
	import type { EntityDef, FieldDef } from '$lib/types/admin';
	import type { FieldValidationError } from '$lib/types/validation';
	import type { LayoutDataV2 } from '$lib/types/layout';
	import { parseLayoutData, getVisibleSections } from '$lib/types/layout';
	import LookupField from '$lib/components/LookupField.svelte';
	import MultiLookupField from '$lib/components/MultiLookupField.svelte';
	import StreamField from '$lib/components/StreamField.svelte';
	import ValidationErrors from '$lib/components/ValidationErrors.svelte';
	import EditSectionRenderer from '$lib/components/EditSectionRenderer.svelte';

	interface LookupRecord {
		id: string;
		name: string;
	}

	let entitySlug = $derived($page.params.entity!);
	let entityName = $derived(getEntityNameFromPath(entitySlug) || toPascalCase(entitySlug));
	let recordId = $derived($page.params.id!);

	function toPascalCase(slug: string): string {
		let singular = slug;
		if (slug.endsWith('s') && slug.length > 1) {
			singular = slug.slice(0, -1);
		}
		return singular.charAt(0).toUpperCase() + singular.slice(1);
	}

	let entityDef = $state<EntityDef | null>(null);
	let fields = $state<FieldDef[]>([]);
	let layout = $state<LayoutDataV2 | null>(null);
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

	// Get visible sections based on form data
	let visibleSections = $derived(() => layout ? getVisibleSections(layout, formData) : []);


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

			// Load layout (may be v1, v2, or legacy section format)
			try {
				const layoutResponse = await get<{ layoutData: string }>(`/entities/${entityName}/layouts/detail`);
				layout = parseLayoutData(layoutResponse.layoutData, fields.map(f => f.name));
			} catch {
				// Default to all fields
				layout = parseLayoutData('[]', fields.map(f => f.name));
			}
		} catch {
			fields = [];
			layout = null;
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


	// Reload data when entity or record changes (handles navigation between edit pages)
	$effect(() => {
		// Track these reactive values to trigger reload on navigation
		const _entity = entityName;
		const _recordId = recordId;

		// Reset state
		entityDef = null;
		fields = [];
		layout = null;
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
		<div class="space-y-6">
			{#if fieldErrors.length > 0}
				<ValidationErrors errors={fieldErrors} />
			{:else if error}
				<div class="mb-4 p-3 bg-red-50 border border-red-200 rounded text-red-700 text-sm">
					{error}
				</div>
			{/if}

			<form onsubmit={(e) => { e.preventDefault(); handleSubmit(); }} class="space-y-6">
				{#each visibleSections() as section (section.id)}
					<EditSectionRenderer
						{section}
						{fields}
						bind:formData
						{lookupNames}
						{multiLookupValues}
						{getFieldError}
						onLookupChange={(fieldName, id, name) => {
							formData[`${fieldName}Id`] = id;
							formData[`${fieldName}Name`] = name;
							lookupNames[fieldName] = name;
						}}
						onMultiLookupChange={(fieldName, values) => {
							multiLookupValues[fieldName] = values;
							formData[`${fieldName}Ids`] = JSON.stringify(values.map(v => v.id));
							formData[`${fieldName}Names`] = JSON.stringify(values.map(v => v.name));
						}}
					/>
				{/each}

				<div class="bg-white shadow rounded-lg p-6">
					<div class="flex justify-end gap-3">
						<a
							href="/{entitySlug}/{recordId}"
							class="px-4 py-2 text-gray-700 bg-gray-100 rounded-lg hover:bg-gray-200 transition-colors"
						>
							Cancel
						</a>
						<button
							type="submit"
							disabled={saving}
							class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-600/90 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
						>
							{saving ? 'Saving...' : 'Save'}
						</button>
					</div>
				</div>
			</form>
		</div>
	{/if}
</div>
