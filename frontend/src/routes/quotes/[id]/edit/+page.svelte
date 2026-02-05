<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { get, put } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import { FormSkeleton, ErrorDisplay } from '$lib/components/ui';
	import { fieldNameToKey } from '$lib/utils/fieldMapping';
	import QuoteLineItemEditor from '$lib/components/QuoteLineItemEditor.svelte';
	import EditSectionRenderer from '$lib/components/EditSectionRenderer.svelte';
	import type { Quote, QuoteLineItemInput } from '$lib/types/quote';
	import type { FieldDef } from '$lib/types/admin';
	import type { LayoutDataV2 } from '$lib/types/layout';
	import { parseLayoutData, getVisibleSections } from '$lib/types/layout';

	interface LookupRecord {
		id: string;
		name: string;
	}

	// System fields that exist as columns in the quotes table (in camelCase)
	const SYSTEM_FIELDS = new Set([
		'name', 'status', 'accountId', 'accountName', 'contactId', 'contactName',
		'validUntil', 'currency', 'discountPercent', 'taxPercent', 'shippingAmount',
		'subtotal', 'totalDiscount', 'totalTax', 'grandTotal',
		'description', 'terms', 'notes', 'assignedUserId',
		'createdAt', 'modifiedAt', 'createdById', 'modifiedById', 'deleted'
	]);

	// Fields that are handled specially (line items, pricing totals)
	const SPECIAL_FIELDS = new Set([
		'lineItems', 'subtotal', 'totalDiscount', 'totalTax', 'grandTotal',
		'discountPercent', 'taxPercent', 'shippingAmount'
	]);

	let quoteId = $derived($page.params.id);
	let loading = $state(true);
	let saving = $state(false);
	let error = $state<string | null>(null);
	let fields = $state<FieldDef[]>([]);
	let layout = $state<LayoutDataV2 | null>(null);
	let lineItemFields = $state<FieldDef[]>([]);

	let formData = $state<Record<string, unknown>>({});
	let lookupNames = $state<Record<string, string>>({});
	let multiLookupValues = $state<Record<string, LookupRecord[]>>({});

	// Pricing fields (special handling)
	let currency = $state('USD');
	let discountPercent = $state(0);
	let taxPercent = $state(0);
	let shippingAmount = $state(0);
	let lineItems = $state<QuoteLineItemInput[]>([]);

	// Get visible sections based on form data
	let visibleSections = $derived(() => layout ? getVisibleSections(layout, formData) : []);

	function isSystemField(fieldName: string): boolean {
		const key = fieldNameToKey(fieldName);
		return SYSTEM_FIELDS.has(key);
	}

	// Filter out special fields from sections
	function filterSection(section: any) {
		return {
			...section,
			fields: section.fields.filter((f: any) => !SPECIAL_FIELDS.has(f.name))
		};
	}

	async function loadData() {
		try {
			loading = true;
			error = null;

			const [quoteData, fieldsData, lineItemFieldsData] = await Promise.all([
				get<Quote>(`/quotes/${quoteId}`),
				get<FieldDef[]>('/entities/Quote/fields').catch(() => [] as FieldDef[]),
				get<FieldDef[]>('/entities/QuoteLineItem/fields').catch(() => [] as FieldDef[])
			]);

			fields = fieldsData;
			lineItemFields = lineItemFieldsData;

			// Load layout
			try {
				const layoutResponse = await get<{ layoutData: string }>('/entities/Quote/layouts/detail');
				layout = parseLayoutData(layoutResponse.layoutData, fieldsData.map(f => f.name));
			} catch {
				// Default to all fields
				layout = parseLayoutData('[]', fieldsData.map(f => f.name));
			}

			// Initialize form data
			const data: Record<string, unknown> = {};
			for (const field of fieldsData) {
				if (SPECIAL_FIELDS.has(field.name)) continue;

				const key = fieldNameToKey(field.name);
				if (isSystemField(field.name)) {
					data[field.name] = (quoteData as Record<string, unknown>)[key] ?? '';
				} else {
					data[field.name] = (quoteData as any).customFields?.[field.name] ?? '';
				}

				// For link fields, load display name
				if (field.type === 'link') {
					const nameKey = `${key}Name`;
					const nameVal = isSystemField(field.name)
						? (quoteData as Record<string, unknown>)[nameKey]
						: (quoteData as any).customFields?.[`${field.name}Name`];
					if (nameVal) {
						lookupNames[field.name] = String(nameVal);
					}
				}

				// For linkMultiple fields, load values
				if (field.type === 'linkMultiple') {
					const idsVal = (quoteData as any).customFields?.[`${field.name}Ids`];
					const namesVal = (quoteData as any).customFields?.[`${field.name}Names`];

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

			// Initialize special pricing fields
			currency = quoteData.currency || 'USD';
			discountPercent = quoteData.discountPercent || 0;
			taxPercent = quoteData.taxPercent || 0;
			shippingAmount = quoteData.shippingAmount || 0;

			// Initialize line items
			lineItems = (quoteData.lineItems || []).map(li => ({
				id: li.id,
				name: li.name,
				description: li.description,
				sku: li.sku,
				quantity: li.quantity,
				unitPrice: li.unitPrice,
				discountPercent: li.discountPercent,
				discountAmount: li.discountAmount,
				taxPercent: li.taxPercent,
				sortOrder: li.sortOrder
			}));
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load quote';
		} finally {
			loading = false;
		}
	}

	async function handleSubmit(e: Event) {
		e.preventDefault();

		const name = formData.name;
		if (!name || String(name).trim() === '') {
			toast.error('Name is required');
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

			// Add special pricing fields
			payload.currency = currency;
			payload.discountPercent = discountPercent;
			payload.taxPercent = taxPercent;
			payload.shippingAmount = shippingAmount;
			payload.lineItems = lineItems;

			// Add custom fields
			if (Object.keys(customFields).length > 0) {
				payload.customFields = customFields;
			}

			await put<Quote>(`/quotes/${quoteId}`, payload);
			toast.success('Quote updated');
			goto(`/quotes/${quoteId}`);
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to update quote');
		} finally {
			saving = false;
		}
	}

	onMount(() => loadData());
</script>

<div class="max-w-4xl mx-auto">
	<div class="flex items-center justify-between mb-6">
		<h1 class="text-2xl font-bold text-gray-900">Edit Quote</h1>
		<a href="/quotes/{quoteId}" class="text-gray-600 hover:text-gray-900 text-sm">
			← Back to Quote
		</a>
	</div>

	{#if loading}
		<FormSkeleton fields={6} />
	{:else if error}
		<ErrorDisplay message={error} onRetry={loadData} />
	{:else}
		<form onsubmit={handleSubmit} class="space-y-6">
			<!-- Render layout sections -->
			{#each visibleSections() as section (section.id)}
				{@const filteredSection = filterSection(section)}
				{#if filteredSection.fields.length > 0}
					<EditSectionRenderer
						section={filteredSection}
						{fields}
						bind:formData
						{lookupNames}
						{multiLookupValues}
						getFieldError={() => undefined}
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
				{/if}
			{/each}

			<!-- Line Items (Special Section) -->
			<div class="bg-white shadow rounded-lg overflow-hidden">
				<div class="px-6 py-4 bg-gray-50 border-b border-gray-200">
					<h2 class="text-lg font-medium text-gray-900">Line Items</h2>
				</div>
				<div class="p-6">
					<QuoteLineItemEditor bind:items={lineItems} fields={lineItemFields} {currency} />
				</div>
			</div>

			<!-- Pricing (Special Section) -->
			<div class="bg-white shadow rounded-lg overflow-hidden">
				<div class="px-6 py-4 bg-gray-50 border-b border-gray-200">
					<h2 class="text-lg font-medium text-gray-900">Pricing</h2>
				</div>
				<div class="p-6">
					<div class="grid grid-cols-1 md:grid-cols-3 gap-4">
						<div>
							<label for="discountPercent" class="block text-sm font-medium text-gray-700 mb-1">Discount %</label>
							<input id="discountPercent" type="number" bind:value={discountPercent} min="0" max="100" step="0.1"
								class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500" />
						</div>
						<div>
							<label for="taxPercent" class="block text-sm font-medium text-gray-700 mb-1">Tax %</label>
							<input id="taxPercent" type="number" bind:value={taxPercent} min="0" step="0.1"
								class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500" />
						</div>
						<div>
							<label for="shippingAmount" class="block text-sm font-medium text-gray-700 mb-1">Shipping</label>
							<input id="shippingAmount" type="number" bind:value={shippingAmount} min="0" step="0.01"
								class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500" />
						</div>
					</div>
				</div>
			</div>

			<!-- Actions -->
			<div class="bg-white shadow rounded-lg p-6">
				<div class="flex justify-end gap-3">
					<a href="/quotes/{quoteId}" class="px-4 py-2 text-gray-700 bg-gray-100 rounded-md hover:bg-gray-200">
						Cancel
					</a>
					<button type="submit" disabled={saving}
						class="px-4 py-2 text-white bg-blue-600 rounded-md hover:bg-blue-600/90 disabled:opacity-50">
						{saving ? 'Saving...' : 'Save Changes'}
					</button>
				</div>
			</div>
		</form>
	{/if}
</div>
