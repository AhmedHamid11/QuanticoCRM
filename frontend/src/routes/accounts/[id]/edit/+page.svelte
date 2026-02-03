<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { get, put, ApiError } from '$lib/utils/api';
	import { addToast } from '$lib/stores/toast.svelte';
	import { fieldNameToKey } from '$lib/utils/fieldMapping';
	import LookupField from '$lib/components/LookupField.svelte';
	import MultiLookupField from '$lib/components/MultiLookupField.svelte';
	import ValidationErrors from '$lib/components/ValidationErrors.svelte';
	import type { Account } from '$lib/types/account';
	import type { FieldDef } from '$lib/types/admin';
	import type { FieldValidationError } from '$lib/types/validation';
	import type { LayoutV2Response } from '$lib/types/layout';
	import { parseLayoutData, getAllFieldNames } from '$lib/types/layout';

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
	let layoutFields = $state<string[]>([]);
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

	// Only show fields that are in the layout, in layout order
	let editableFields = $derived(
		layoutFields
			.map(fieldName => fields.find(f => f.name === fieldName))
			.filter((f): f is FieldDef => f !== undefined && f.name !== 'id' && !f.isReadOnly)
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

	async function loadFields() {
		try {
			fields = await get<FieldDef[]>(`/entities/Account/fields`);

			// Load layout (may be v1, v2, or legacy section format)
			try {
				const layoutResponse = await get<{ layoutData: string }>('/entities/Account/layouts/detail');
				const layout = parseLayoutData(layoutResponse.layoutData, fields.map(f => f.name));
				layoutFields = getAllFieldNames(layout);
			} catch {
				// Default to all fields
				const layout = parseLayoutData('[]', fields.map(f => f.name));
				layoutFields = getAllFieldNames(layout);
			}
		} catch {
			fields = [];
			layoutFields = [];
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
					data[field.name] = (accountData as Record<string, unknown>)[key] ?? '';
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
						? (accountData as Record<string, unknown>)[nameKey]
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

	// Reload data when account ID changes (handles navigation between edit pages)
	$effect(() => {
		// Track accountId to trigger reload on navigation
		const _id = accountId;

		// Reset state
		account = null;
		fields = [];
		layoutFields = [];
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

			<div class="bg-white shadow rounded-lg overflow-hidden">
				<div class="px-6 py-4 border-b border-gray-200">
					<h2 class="text-lg font-medium text-gray-900">Account Information</h2>
				</div>
				<div class="px-6 py-4">
					<div class="grid grid-cols-1 md:grid-cols-2 gap-6">
						{#each editableFields as field}
							<div class={field.type === 'text' ? 'md:col-span-2' : ''}>
								{#if field.type === 'link' && field.linkEntity}
									<LookupField
										entity={field.linkEntity}
										value={formData[`${field.name}Id`] as string | null}
										valueName={lookupNames[field.name] || ''}
										label={field.label}
										required={field.isRequired}
										onchange={(id, name) => {
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
											formData[`${field.name}Ids`] = JSON.stringify(values.map(v => v.id));
											formData[`${field.name}Names`] = JSON.stringify(values.map(v => v.name));
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
											bind:value={formData[field.name]}
											required={field.isRequired}
											rows="3"
											class="w-full px-3 py-2 border rounded-md shadow-sm focus:ring-primary focus:border-primary"
											class:border-gray-300={!fieldError}
											class:border-red-500={fieldError}
										></textarea>
									{:else if field.type === 'bool'}
										<input
											type="checkbox"
											id={field.name}
											bind:checked={formData[field.name]}
											class="w-4 h-4 rounded text-primary focus:ring-primary"
											class:border-gray-300={!fieldError}
											class:border-red-500={fieldError}
										/>
									{:else if field.type === 'enum' && field.options}
										<select
											id={field.name}
											bind:value={formData[field.name]}
											required={field.isRequired}
											class="w-full px-3 py-2 border rounded-md shadow-sm focus:ring-primary focus:border-primary"
											class:border-gray-300={!fieldError}
											class:border-red-500={fieldError}
										>
											<option value="">-- Select --</option>
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
														class="w-4 h-4 rounded border-gray-300 text-primary focus:ring-primary"
													/>
													<span class="text-sm text-gray-700">{option}</span>
												</label>
											{/each}
										</div>
									{:else}
										<input
											type={getInputType(field)}
											id={field.name}
											bind:value={formData[field.name]}
											required={field.isRequired}
											maxlength={field.maxLength || undefined}
											min={field.minValue || undefined}
											max={field.maxValue || undefined}
											step={field.type === 'float' || field.type === 'currency' ? '0.01' : undefined}
											class="w-full px-3 py-2 border rounded-md shadow-sm focus:ring-primary focus:border-primary"
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
				</div>
			</div>

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
					class="px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-black bg-primary hover:bg-primary/90 disabled:opacity-50"
				>
					{saving ? 'Saving...' : 'Save'}
				</button>
			</div>
		</form>
	</div>
{/if}
