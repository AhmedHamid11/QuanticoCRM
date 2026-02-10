<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { get, put } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import { FormSkeleton, ErrorDisplay, FieldError } from '$lib/components/ui';
	import { useFormErrors } from '$lib/hooks/useFormErrors.svelte';
	import { fieldNameToKey } from '$lib/utils/fieldMapping';
	import type { Contact } from '$lib/types/contact';
	import type { FieldDef } from '$lib/types/admin';
	import type { LayoutV2Response, LayoutDataV2 } from '$lib/types/layout';
	import { parseLayoutData, getVisibleSections } from '$lib/types/layout';
	import LookupField from '$lib/components/LookupField.svelte';
	import MultiLookupField from '$lib/components/MultiLookupField.svelte';
	import EditSectionRenderer from '$lib/components/EditSectionRenderer.svelte';

	interface LookupRecord {
		id: string;
		name: string;
	}

	const formErrors = useFormErrors();

	// System fields that exist as columns in the contacts table (in camelCase)
	const SYSTEM_FIELDS = new Set([
		'salutationName', 'firstName', 'lastName', 'emailAddress',
		'phoneNumber', 'phoneNumberType', 'doNotCall', 'description',
		'addressStreet', 'addressCity', 'addressState', 'addressCountry',
		'addressPostalCode', 'accountId', 'accountName', 'assignedUserId'
	]);

	let contactId = $derived($page.params.id);
	let loading = $state(true);
	let saving = $state(false);
	let error = $state<string | null>(null);
	let fields = $state<FieldDef[]>([]);
	let layout = $state<LayoutDataV2 | null>(null);
	let lookupNames = $state<Record<string, string>>({});
	let multiLookupValues = $state<Record<string, LookupRecord[]>>({});

	// Dynamic form data - keyed by field name
	let formData = $state<Record<string, unknown>>({});

	// Get visible sections based on form data
	let visibleSections = $derived(() => layout ? getVisibleSections(layout, formData) : []);

	function isSystemField(fieldName: string): boolean {
		const key = fieldNameToKey(fieldName);
		return SYSTEM_FIELDS.has(key);
	}

	async function loadData() {
		try {
			loading = true;
			error = null;

			const [contactData, fieldsData] = await Promise.all([
				get<Contact & { customFields?: Record<string, unknown> }>(`/contacts/${contactId}`),
				get<FieldDef[]>('/entities/Contact/fields')
			]);

			fields = fieldsData;

			// Load layout (may be v1, v2, or legacy section format)
			try {
				const layoutResponse = await get<{ layoutData: string }>('/entities/Contact/layouts/detail');
				layout = parseLayoutData(layoutResponse.layoutData, fieldsData.map(f => f.name));
			} catch {
				// Default to all fields
				layout = parseLayoutData('[]', fieldsData.map(f => f.name));
			}

			// Initialize form data from contact
			const data: Record<string, unknown> = {};
			for (const field of fieldsData) {
				const key = fieldNameToKey(field.name);
				if (isSystemField(field.name)) {
					// System field - get from contact directly
					data[field.name] = (contactData as unknown as Record<string, unknown>)[key] ?? '';
				} else {
					// Custom field - get from customFields
					data[field.name] = contactData.customFields?.[field.name] ?? '';
				}

				// For link fields, also load the display name
				if (field.type === 'link') {
					const nameFieldName = field.name.replace(/Id$/, 'Name');
					const nameKey = fieldNameToKey(nameFieldName);
					if (isSystemField(nameFieldName)) {
						data[nameFieldName] = (contactData as unknown as Record<string, unknown>)[nameKey] ?? '';
					} else {
						data[nameFieldName] = contactData.customFields?.[nameFieldName] ?? '';
					}
					// Store lookup name for EditSectionRenderer
					lookupNames[field.name] = String(data[nameFieldName] || '');
				}

				// For linkMultiple fields, load the values
				if (field.type === 'linkMultiple') {
					const idsFieldName = `${field.name}Ids`;
					const namesFieldName = `${field.name}Names`;
					const idsKey = fieldNameToKey(idsFieldName);
					const namesKey = fieldNameToKey(namesFieldName);

					const idsVal = isSystemField(idsFieldName)
						? (contactData as unknown as Record<string, unknown>)[idsKey]
						: contactData.customFields?.[idsFieldName];
					const namesVal = isSystemField(namesFieldName)
						? (contactData as unknown as Record<string, unknown>)[namesKey]
						: contactData.customFields?.[namesFieldName];

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
			formData = data;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load contact';
		} finally {
			loading = false;
		}
	}

	async function handleSubmit(e: Event) {
		e.preventDefault();
		formErrors.clearAll();

		saving = true;

		try {
			// Separate system fields and custom fields
			const payload: Record<string, unknown> = {};
			const customFields: Record<string, unknown> = {};

			for (const [fieldName, value] of Object.entries(formData)) {
				const key = fieldNameToKey(fieldName);
				if (isSystemField(fieldName)) {
					payload[key] = value;
				} else {
					customFields[fieldName] = value;
				}
			}

			// Add custom fields to payload
			payload.customFields = customFields;

			await put<Contact>(`/contacts/${contactId}`, payload);
			toast.success('Contact updated successfully');
			goto(`/contacts/${contactId}`);
		} catch (err) {
			formErrors.setFromApiError(err);
			if (formErrors.generalError) {
				toast.error(formErrors.generalError);
			}
		} finally {
			saving = false;
		}
	}

	onMount(() => {
		loadData();
	});
</script>

<div class="space-y-6">
	<!-- Header -->
	<div class="flex items-center justify-between">
		<h1 class="text-2xl font-bold text-gray-900">Edit Contact</h1>
		<a href="/contacts/{contactId}" class="text-gray-600 hover:text-gray-900 text-sm">
			← Back to Contact
		</a>
	</div>

	{#if loading}
		<FormSkeleton fields={6} />
	{:else if error}
		<ErrorDisplay message={error} onRetry={loadData} />
	{:else}
		<form onsubmit={handleSubmit} class="space-y-6">
			{#each visibleSections() as section (section.id)}
				<EditSectionRenderer
					{section}
					{fields}
					bind:formData
					{lookupNames}
					{multiLookupValues}
					getFieldError={(fieldName) => formErrors.getFieldError(fieldName) ? { field: fieldName, message: formErrors.getFieldError(fieldName) || '', ruleId: '' } : undefined}
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

			<!-- Actions -->
			<div class="bg-white shadow rounded-lg p-6">
				<div class="flex justify-end gap-3">
					<a
						href="/contacts/{contactId}"
						class="px-4 py-2 text-gray-700 bg-gray-100 rounded-md hover:bg-gray-200"
					>
						Cancel
					</a>
					<button
						type="submit"
						disabled={saving}
						class="px-4 py-2 text-white bg-blue-600 rounded-md hover:bg-blue-600/90 disabled:opacity-50"
					>
						{saving ? 'Saving...' : 'Save Changes'}
					</button>
				</div>
			</div>
		</form>
	{/if}
</div>
