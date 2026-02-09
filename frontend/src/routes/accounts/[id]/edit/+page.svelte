<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { get, put, ApiError } from '$lib/utils/api';
	import { addToast } from '$lib/stores/toast.svelte';
	import { fieldNameToKey } from '$lib/utils/fieldMapping';
	import LookupField from '$lib/components/LookupField.svelte';
	import MultiLookupField from '$lib/components/MultiLookupField.svelte';
	import ValidationErrors from '$lib/components/ValidationErrors.svelte';
	import EditSectionRenderer from '$lib/components/EditSectionRenderer.svelte';
	import type { Account } from '$lib/types/account';
	import type { FieldDef } from '$lib/types/admin';
	import type { FieldValidationError } from '$lib/types/validation';
	import type { LayoutV2Response, LayoutDataV2 } from '$lib/types/layout';
	import { parseLayoutData, getVisibleSections } from '$lib/types/layout';

	interface LookupRecord {
		id: string;
		name: string;
	}

	// System fields that exist as columns in the accounts table (in camelCase)
	const SYSTEM_FIELDS = new Set([
		'name', 'website', 'emailAddress', 'phoneNumber', 'type', 'industry',
		'sicCode', 'billingAddressStreet', 'billingAddressCity', 'billingAddressState',
		'billingAddressCountry', 'billingAddressPostalCode', 'shippingAddressStreet',
		'shippingAddressCity', 'shippingAddressState', 'shippingAddressCountry',
		'shippingAddressPostalCode', 'description', 'stage', 'assignedUserId',
		'createdAt', 'modifiedAt', 'createdById', 'modifiedById', 'deleted'
	]);

	function isSystemField(fieldName: string): boolean {
		const key = fieldNameToKey(fieldName);
		return SYSTEM_FIELDS.has(key);
	}

	let account = $state<Account | null>(null);
	let fields = $state<FieldDef[]>([]);
	let layout = $state<LayoutDataV2 | null>(null);
	let formData = $state<Record<string, unknown>>({});
	let lookupNames = $state<Record<string, string>>({});
	let multiLookupValues = $state<Record<string, LookupRecord[]>>({});
	let loading = $state(true);
	let saving = $state(false);
	let loadError = $state<string | null>(null);
	let saveError = $state<string | null>(null);
	let fieldErrors = $state<FieldValidationError[]>([]);

	// Get field error for a specific field
	function getFieldError(fieldName: string): FieldValidationError | undefined {
		return fieldErrors.find((e) => e.field === fieldName);
	}

	let accountId = $derived($page.params.id);

	// Get visible sections based on form data
	let visibleSections = $derived(() => layout ? getVisibleSections(layout, formData) : []);

	async function loadFields() {
		try {
			fields = await get<FieldDef[]>(`/entities/Account/fields`);

			// Load layout (may be v1, v2, or legacy section format)
			try {
				const layoutResponse = await get<{ layoutData: string }>('/entities/Account/layouts/detail');
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

	async function loadAccount() {
		try {
			loading = true;
			loadError = null;
			const accountData = await get<Account & { customFields?: Record<string, unknown> }>(`/accounts/${accountId}`);
			account = accountData;

			// Initialize form data from account
			const data: Record<string, unknown> = {};
			for (const field of fields) {
				const key = fieldNameToKey(field.name);
				if (isSystemField(field.name)) {
					// System field - get from account directly using camelCase key
					data[field.name] = (accountData as unknown as Record<string, unknown>)[key] ?? '';
				} else {
					// Custom field - get from customFields (try camelCase key first, then original)
					data[field.name] = accountData.customFields?.[key] ?? accountData.customFields?.[field.name] ?? '';
				}
			}
			formData = data;

			// Load lookup display names for link fields from the record data
			for (const field of fields) {
				if (field.type === 'link' && field.linkEntity) {
					const key = fieldNameToKey(field.name);
					const nameKey = `${key}Name`;
					const nameVal = isSystemField(field.name)
						? (accountData as unknown as Record<string, unknown>)[nameKey]
						: accountData.customFields?.[nameKey] ?? accountData.customFields?.[`${field.name}Name`];
					if (nameVal) {
						lookupNames[field.name] = String(nameVal);
					}
				}
				// Load multi-lookup values
				if (field.type === 'linkMultiple' && field.linkEntity) {
					const key = fieldNameToKey(field.name);
					const idsKey = `${key}Ids`;
					const namesKey = `${key}Names`;
					const idsVal = accountData.customFields?.[idsKey] ?? accountData.customFields?.[`${field.name}Ids`];
					const namesVal = accountData.customFields?.[namesKey] ?? accountData.customFields?.[`${field.name}Names`];

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
			loadError = e instanceof Error ? e.message : 'Failed to load account';
			addToast(loadError, 'error');
		} finally {
			loading = false;
		}
	}

	async function handleSubmit(e: Event) {
		e.preventDefault();
		saveError = null;
		fieldErrors = [];

		const nameValue = formData.name;
		if (!nameValue || String(nameValue).trim() === '') {
			addToast('Account name is required', 'error');
			return;
		}

		try {
			saving = true;

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

			await put(`/accounts/${accountId}`, payload);
			addToast('Account updated', 'success');
			goto(`/accounts/${accountId}`);
		} catch (e) {
			if (e instanceof ApiError && e.fieldErrors) {
				fieldErrors = e.fieldErrors;
				saveError = e.message;
			} else {
				const message = e instanceof Error ? e.message : 'Failed to update account';
				saveError = message;
				addToast(message, 'error');
			}
		} finally {
			saving = false;
		}
	}

	// Reload data when account ID changes (handles navigation between edit pages)
	$effect(() => {
		// Track accountId to trigger reload on navigation
		const _id = accountId;

		// Reset state
		account = null;
		fields = [];
		layout = null;
		formData = {};
		lookupNames = {};
		multiLookupValues = {};
		loading = true;
		loadError = null;
		saveError = null;
		fieldErrors = [];

		// Load data
		(async () => {
			await loadFields();
			await loadAccount();
		})();
	});
</script>

{#if loading}
	<div class="text-center py-12 text-gray-500">Loading...</div>
{:else if loadError}
	<div class="text-center py-12 text-red-500">{loadError}</div>
{:else if account}
	<div class="space-y-6">
		<!-- Header -->
		<div class="flex justify-between items-center">
			<div>
				<div class="flex items-center space-x-2 text-sm text-gray-500 mb-2">
					<a href="/accounts" class="hover:text-gray-700">Accounts</a>
					<span>/</span>
					<a href="/accounts/{account.id}" class="hover:text-gray-700">{account.name}</a>
					<span>/</span>
					<span>Edit</span>
				</div>
				<h1 class="text-2xl font-bold text-gray-900">Edit Account</h1>
			</div>
		</div>

		<!-- Form -->
		<form onsubmit={handleSubmit} class="space-y-6">
			{#if fieldErrors.length > 0}
				<ValidationErrors errors={fieldErrors} />
			{:else if saveError}
				<div class="p-4 bg-red-50 border border-red-200 rounded-lg text-red-700">
					{saveError}
				</div>
			{/if}

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

			<!-- Actions -->
			<div class="flex justify-end space-x-3">
				<a
					href="/accounts/{account.id}"
					class="px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
				>
					Cancel
				</a>
				<button
					type="submit"
					disabled={saving}
					class="px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-600/90 disabled:opacity-50"
				>
					{saving ? 'Saving...' : 'Save'}
				</button>
			</div>
		</form>
	</div>
{/if}
