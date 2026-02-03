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
	import { parseLayoutData, getAllFieldNames } from '$lib/types/layout';
	import LookupField from '$lib/components/LookupField.svelte';

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
	let layoutFields = $state<string[]>([]);

	// Dynamic form data - keyed by field name
	let formData = $state<Record<string, unknown>>({});

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
				const layout = parseLayoutData(layoutResponse.layoutData, fieldsData.map(f => f.name));
				layoutFields = getAllFieldNames(layout);
			} catch {
				// Default to all fields
				const layout = parseLayoutData('[]', fieldsData.map(f => f.name));
				layoutFields = getAllFieldNames(layout);
			}

			// Initialize form data from contact
			const data: Record<string, unknown> = {};
			for (const field of fieldsData) {
				const key = fieldNameToKey(field.name);
				if (isSystemField(field.name)) {
					// System field - get from contact directly
					data[field.name] = (contactData as Record<string, unknown>)[key] ?? '';
				} else {
					// Custom field - get from customFields
					data[field.name] = contactData.customFields?.[field.name] ?? '';
				}

				// For link fields, also load the display name
				if (field.type === 'link') {
					const nameFieldName = field.name.replace(/Id$/, 'Name');
					const nameKey = fieldNameToKey(nameFieldName);
					if (isSystemField(nameFieldName)) {
						data[nameFieldName] = (contactData as Record<string, unknown>)[nameKey] ?? '';
					} else {
						data[nameFieldName] = contactData.customFields?.[nameFieldName] ?? '';
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

		// Validate required fields
		for (const fieldName of layoutFields) {
			const field = fields.find(f => f.name === fieldName);
			if (field?.isRequired && !formData[fieldName]) {
				formErrors.setFieldError(fieldName, `${field.label} is required`);
			}
		}

		if (formErrors.hasErrors()) {
			return;
		}

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

	function getFieldDef(fieldName: string): FieldDef | undefined {
		return fields.find(f => f.name === fieldName);
	}

	function getInputType(field: FieldDef): string {
		switch (field.type) {
			case 'email': return 'email';
			case 'phone': return 'tel';
			case 'url': return 'url';
			case 'int':
			case 'float':
			case 'currency': return 'number';
			case 'date': return 'date';
			case 'datetime': return 'datetime-local';
			default: return 'text';
		}
	}

	function parseOptions(optionsStr: string | null | undefined): string[] {
		if (!optionsStr) return [];
		try {
			const parsed = JSON.parse(optionsStr);
			return Array.isArray(parsed) ? parsed : [];
		} catch {
			return optionsStr.split(',').map(s => s.trim()).filter(Boolean);
		}
	}

	function handleLookupChange(field: FieldDef, id: string | null, name: string) {
		// For lookup fields, store both the ID and Name
		formData[field.name] = id || '';
		const nameFieldName = field.name.replace(/Id$/, 'Name');
		formData[nameFieldName] = name;
	}

	onMount(() => {
		loadData();
	});
</script>

<div class="max-w-2xl mx-auto">
	<div class="flex items-center justify-between mb-6">
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
		<form onsubmit={handleSubmit} class="bg-white shadow rounded-lg p-6 space-y-4">
			{#each layoutFields as fieldName (fieldName)}
				{@const field = getFieldDef(fieldName)}
				{#if field}
					<div>
						{#if field.type === 'link' && field.linkEntity}
							<!-- Lookup field for link type (renders its own label) -->
							{@const nameFieldName = field.name.replace(/Id$/, 'Name')}
							<LookupField
								entity={field.linkEntity}
								value={formData[field.name] as string | null}
								valueName={formData[nameFieldName] as string || ''}
								label={field.label}
								required={field.isRequired}
								disabled={field.isReadOnly}
								onchange={(id, name) => handleLookupChange(field, id, name)}
							/>

						{:else}
						<label for={fieldName} class="block text-sm font-medium text-gray-700 mb-1">
							{field.label}
							{#if field.isRequired}
								<span class="text-red-500">*</span>
							{/if}
						</label>

						{#if field.type === 'text'}
							<!-- Textarea for text type -->
							<textarea
								id={fieldName}
								bind:value={formData[fieldName]}
								rows="3"
								class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary focus:border-primary"
								required={field.isRequired}
								readonly={field.isReadOnly}
							></textarea>

						{:else if field.type === 'bool'}
							<!-- Checkbox for boolean -->
							<label class="flex items-center">
								<input
									type="checkbox"
									id={fieldName}
									bind:checked={formData[fieldName]}
									class="rounded border-gray-300 text-primary focus:ring-primary"
									disabled={field.isReadOnly}
								/>
								<span class="ml-2 text-sm text-gray-700">Yes</span>
							</label>

						{:else if field.type === 'enum'}
							<!-- Select for enum -->
							{@const options = parseOptions(field.options)}
							<select
								id={fieldName}
								bind:value={formData[fieldName]}
								class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary focus:border-primary"
								required={field.isRequired}
								disabled={field.isReadOnly}
							>
								<option value="">--</option>
								{#each options as option}
									<option value={option}>{option}</option>
								{/each}
							</select>

						{:else}
							<!-- Standard input for other types -->
							<input
								id={fieldName}
								type={getInputType(field)}
								bind:value={formData[fieldName]}
								class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary focus:border-primary"
								required={field.isRequired}
								readonly={field.isReadOnly}
								maxlength={field.maxLength || undefined}
								min={field.minValue ?? undefined}
								max={field.maxValue ?? undefined}
								pattern={field.pattern || undefined}
							/>
						{/if}
						{/if}

						{#if field.tooltip && !formErrors.getFieldError(fieldName)}
							<p class="mt-1 text-xs text-gray-500">{field.tooltip}</p>
						{/if}
						<FieldError message={formErrors.getFieldError(fieldName)} />
					</div>
				{/if}
			{/each}

			<!-- Actions -->
			<div class="flex justify-end gap-3 pt-4 border-t border-gray-200">
				<a
					href="/contacts/{contactId}"
					class="px-4 py-2 text-gray-700 bg-gray-100 rounded-md hover:bg-gray-200"
				>
					Cancel
				</a>
				<button
					type="submit"
					disabled={saving}
					class="px-4 py-2 text-black bg-primary rounded-md hover:bg-primary/90 disabled:opacity-50"
				>
					{saving ? 'Saving...' : 'Save Changes'}
				</button>
			</div>
		</form>
	{/if}
</div>
