<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { get, post } from '$lib/utils/api';
	import { addToast } from '$lib/stores/toast.svelte';
	import LookupField from '$lib/components/LookupField.svelte';
	import MultiLookupField from '$lib/components/MultiLookupField.svelte';
	import type { Account } from '$lib/types/account';
	import type { FieldDef } from '$lib/types/admin';

	interface LookupRecord {
		id: string;
		name: string;
	}

	// System fields that exist as columns in the accounts table
	const SYSTEM_FIELDS = new Set([
		'name', 'website', 'emailAddress', 'phoneNumber', 'type', 'industry',
		'sicCode', 'billingAddressStreet', 'billingAddressCity', 'billingAddressState',
		'billingAddressCountry', 'billingAddressPostalCode', 'shippingAddressStreet',
		'shippingAddressCity', 'shippingAddressState', 'shippingAddressCountry',
		'shippingAddressPostalCode', 'description', 'assignedUserId',
		'createdAt', 'modifiedAt', 'createdById', 'modifiedById', 'deleted'
	]);

	function isSystemField(fieldName: string): boolean {
		return SYSTEM_FIELDS.has(fieldName);
	}

	let fields = $state<FieldDef[]>([]);
	let formData = $state<Record<string, unknown>>({});
	let lookupNames = $state<Record<string, string>>({});
	let multiLookupValues = $state<Record<string, LookupRecord[]>>({});
	let loading = $state(true);
	let saving = $state(false);

	// Editable fields (exclude id, system fields that are read-only)
	let editableFields = $derived(
		fields.filter(f => f.name !== 'id' && !f.isReadOnly && f.type !== 'rollup')
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
			loading = true;
			fields = await get<FieldDef[]>(`/entities/Account/fields`);

			// Initialize form data with default values
			const data: Record<string, unknown> = {};
			for (const field of fields) {
				if (field.defaultValue) {
					data[field.name] = field.defaultValue;
				} else {
					data[field.name] = '';
				}
			}
			formData = data;
		} catch {
			fields = [];
		} finally {
			loading = false;
		}
	}

	async function handleSubmit(e: Event) {
		e.preventDefault();

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
				// Skip empty values
				if (value === '' || value === null || value === undefined) continue;

				if (isSystemField(fieldName) || fieldName.endsWith('Id') || fieldName.endsWith('Name')) {
					payload[fieldName] = value;
				} else {
					customFields[fieldName] = value;
				}
			}

			// Add custom fields to payload if any
			if (Object.keys(customFields).length > 0) {
				payload.customFields = customFields;
			}

			const account = await post<Account>('/accounts', payload);
			addToast('Account created', 'success');
			goto(`/accounts/${account.id}`);
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to create account';
			addToast(message, 'error');
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

	onMount(() => {
		loadFields();
	});
</script>

<div class="space-y-6">
	<!-- Header -->
	<div class="flex justify-between items-center">
		<div>
			<div class="flex items-center space-x-2 text-sm text-gray-500 mb-2">
				<a href="/accounts" class="hover:text-gray-700">Accounts</a>
				<span>/</span>
				<span>New</span>
			</div>
			<h1 class="text-2xl font-bold text-gray-900">New Account</h1>
		</div>
	</div>

	{#if loading}
		<div class="text-center py-12 text-gray-500">Loading...</div>
	{:else}
		<!-- Form -->
		<form onsubmit={handleSubmit} class="space-y-6">
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
									<label for={field.name} class="block text-sm font-medium text-gray-700 mb-1">
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
											class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-blue-500 focus:border-blue-500"
										></textarea>
									{:else if field.type === 'bool'}
										<input
											type="checkbox"
											id={field.name}
											checked={!!formData[field.name]}
											onchange={(e) => { formData[field.name] = e.currentTarget.checked; }}
											class="w-4 h-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
										/>
									{:else if field.type === 'enum' && field.options}
										<select
											id={field.name}
											bind:value={formData[field.name]}
											required={field.isRequired}
											class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-blue-500 focus:border-blue-500"
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
											bind:value={formData[field.name]}
											required={field.isRequired}
											maxlength={field.maxLength || undefined}
											min={field.minValue || undefined}
											max={field.maxValue || undefined}
											step={field.type === 'float' || field.type === 'currency' ? '0.01' : undefined}
											class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-blue-500 focus:border-blue-500"
										/>
									{/if}

									{#if field.tooltip}
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
					href="/accounts"
					class="px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
				>
					Cancel
				</a>
				<button
					type="submit"
					disabled={saving}
					class="px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-600/90 disabled:opacity-50"
				>
					{saving ? 'Creating...' : 'Create Account'}
				</button>
			</div>
		</form>
	{/if}
</div>
