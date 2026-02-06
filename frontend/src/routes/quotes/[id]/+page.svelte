<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { get, del } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { DetailSkeleton, ErrorDisplay } from '$lib/components/ui';
	import Bearing from '$lib/components/Bearing.svelte';
	import SectionRenderer from '$lib/components/SectionRenderer.svelte';
	import ActivitiesStream from '$lib/components/ActivitiesStream.svelte';
	import type { Quote } from '$lib/types/quote';
	import type { PdfTemplate } from '$lib/types/pdf-template';
	import type { BearingWithStages } from '$lib/types/bearing';
	import type { EntityDef, FieldDef } from '$lib/types/admin';
	import type { LayoutDataV2 } from '$lib/types/layout';
	import { parseLayoutData, getVisibleSections } from '$lib/types/layout';
	import { getStatusColor, formatCurrency } from '$lib/types/quote';
	import { PUBLIC_API_URL } from '$env/static/public';

	const API_BASE = PUBLIC_API_URL || '/api/v1';

	// System fields that exist as columns in the quotes table
	const SYSTEM_FIELDS = new Set([
		'name', 'quoteNumber', 'status', 'accountId', 'contactId', 'validUntil',
		'subtotal', 'discountPercent', 'discountAmount', 'taxPercent', 'taxAmount',
		'shippingAmount', 'grandTotal', 'currency', 'billingAddressStreet',
		'billingAddressCity', 'billingAddressState', 'billingAddressCountry',
		'billingAddressPostalCode', 'shippingAddressStreet', 'shippingAddressCity',
		'shippingAddressState', 'shippingAddressCountry', 'shippingAddressPostalCode',
		'description', 'terms', 'notes', 'assignedUserId', 'createdAt', 'modifiedAt',
		'createdById', 'modifiedById', 'deleted'
	]);

	function isSystemField(fieldName: string): boolean {
		return SYSTEM_FIELDS.has(fieldName);
	}

	type TabId = 'details' | 'activities';
	let activeTab = $state<TabId>('details');

	let quoteId = $derived($page.params.id);
	let quote = $state<(Quote & { customFields?: Record<string, unknown> }) | null>(null);
	let templates = $state<PdfTemplate[]>([]);
	let bearings = $state<BearingWithStages[]>([]);
	let fields = $state<FieldDef[]>([]);
	let lineItemFields = $state<FieldDef[]>([]);
	let layout = $state<LayoutDataV2 | null>(null);
	let entityDef = $state<EntityDef | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let deleting = $state(false);
	let generatingPdf = $state(false);
	let showTemplateDropdown = $state(false);

	// Line item fields that should be displayed in the table (exclude system fields)
	let visibleLineItemFields = $derived(
		lineItemFields.length > 0
			? lineItemFields
					.filter(f => !['quoteId', 'sortOrder', 'createdAt', 'modifiedAt', 'id'].includes(f.name))
					.sort((a, b) => a.sortOrder - b.sortOrder)
			: []
	);

	// Get record data as a flat object for visibility evaluation
	let recordData = $derived(() => {
		if (!quote) return {};
		const data: Record<string, unknown> = {};

		// Add system fields
		for (const fieldName of SYSTEM_FIELDS) {
			if (fieldName in quote) {
				data[fieldName] = (quote as Record<string, unknown>)[fieldName];
			}
		}

		// Add derived name fields
		data['accountName'] = quote.accountName;
		data['contactName'] = quote.contactName;
		data['createdByName'] = quote.createdByName;
		data['modifiedByName'] = quote.modifiedByName;

		// Add custom fields
		if (quote.customFields) {
			for (const [key, value] of Object.entries(quote.customFields)) {
				data[key] = value;
			}
		}

		// Add other dynamic fields from the quote object
		for (const field of fields) {
			if (!SYSTEM_FIELDS.has(field.name) && field.name in quote && !(field.name in data)) {
				data[field.name] = (quote as Record<string, unknown>)[field.name];
			}
		}

		return data;
	});

	// Get visible sections based on record data
	let visibleSections = $derived(() => {
		if (!layout) return [];
		return getVisibleSections(layout, recordData());
	});

	async function loadData() {
		try {
			loading = true;
			error = null;

			const [quoteData, templatesData, bearingsData, fieldsData, lineItemFieldsData, entityDefData] = await Promise.all([
				get<Quote>(`/quotes/${quoteId}`),
				get<PdfTemplate[]>('/pdf-templates?entityType=Quote').catch(() => []),
				get<BearingWithStages[]>('/entities/Quote/bearings').catch(() => []),
				get<FieldDef[]>('/entities/Quote/fields').catch(() => []),
				get<FieldDef[]>('/entities/QuoteLineItem/fields').catch(() => []),
				get<EntityDef>('/entities/Quote/def').catch(() => null)
			]);

			quote = quoteData;
			templates = templatesData;
			bearings = bearingsData;
			fields = fieldsData;
			lineItemFields = lineItemFieldsData;
			entityDef = entityDefData;

			// Debug: log bearings data
			console.log('Bearings loaded:', bearingsData);

			// Load layout (may be v1, v2, or legacy section array format)
			try {
				const layoutResponse = await get<{ layoutData: string }>(
					'/entities/Quote/layouts/detail'
				);
				layout = parseLayoutData(layoutResponse.layoutData, fieldsData.map(f => f.name));
			} catch {
				// Default to field-based layout if no layout exists
				layout = parseLayoutData('[]', fieldsData.map(f => f.name));
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load quote';
		} finally {
			loading = false;
		}
	}

	function handleBearingUpdate(fieldName: string, newValue: string) {
		if (quote) {
			if (isSystemField(fieldName)) {
				(quote as Record<string, unknown>)[fieldName] = newValue;
			} else {
				if (!quote.customFields) {
					quote.customFields = {};
				}
				quote.customFields[fieldName] = newValue;
			}
			quote = { ...quote }; // Trigger reactivity
		}
	}

	async function deleteQuote() {
		if (!confirm('Are you sure you want to delete this quote?')) return;
		deleting = true;
		try {
			await del(`/quotes/${quoteId}`);
			toast.success('Quote deleted');
			goto('/quotes');
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to delete quote');
		} finally {
			deleting = false;
		}
	}

	async function generatePdf(templateId?: string, download = true) {
		generatingPdf = true;
		showTemplateDropdown = false;
		try {
			const params = new URLSearchParams();
			if (templateId) params.set('template', templateId);
			params.set('download', download.toString());

			// Get auth token from memory store (not localStorage)
			const authToken = auth.accessToken;
			if (!authToken) {
				throw new Error('Not authenticated. Please refresh the page and try again.');
			}

			const response = await fetch(
				`${API_BASE}/quotes/${quoteId}/pdf?${params}`,
				{
					headers: {
						'Authorization': `Bearer ${authToken}`
					},
					credentials: 'include' // Send cookies for session
				}
			);

			if (!response.ok) {
				const err = await response.json().catch(() => ({ error: 'Failed to generate PDF' }));
				throw new Error(err.error);
			}

			const blob = await response.blob();
			const url = URL.createObjectURL(blob);

			if (download) {
				const a = document.createElement('a');
				a.href = url;
				a.download = quote?.quoteNumber ? `${quote.quoteNumber}.pdf` : 'quote.pdf';
				a.click();
				URL.revokeObjectURL(url);
			} else {
				window.open(url, '_blank');
			}
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to generate PDF');
		} finally {
			generatingPdf = false;
		}
	}

	function formatDate(dateStr: string): string {
		if (!dateStr) return '-';
		try {
			return new Date(dateStr).toLocaleDateString('en-US', { year: 'numeric', month: 'long', day: 'numeric' });
		} catch {
			return dateStr;
		}
	}

	function getFieldDef(fieldName: string): FieldDef | undefined {
		return fields.find(f => f.name === fieldName);
	}

	function formatFieldValue(fieldName: string, value: unknown): string {
		if (value === null || value === undefined || value === '') return '-';

		const field = getFieldDef(fieldName);
		if (!field) return String(value);

		switch (field.type) {
			case 'bool':
				return value ? 'Yes' : 'No';
			case 'date':
				return new Date(String(value)).toLocaleDateString();
			case 'datetime':
				return new Date(String(value)).toLocaleString();
			case 'currency':
			case 'float':
			case 'int':
				if (typeof value === 'number') {
					if (field.type === 'currency') {
						return formatCurrency(value, quote?.currency || 'USD');
					}
					return value.toLocaleString();
				}
				return String(value);
			default:
				return String(value);
		}
	}

	function getLinkInfo(fieldName: string, value: unknown): { href: string; text: string } | null {
		const field = getFieldDef(fieldName);
		if (!field || !value) return null;

		switch (field.type) {
			case 'email':
				return { href: `mailto:${value}`, text: String(value) };
			case 'phone':
				return { href: `tel:${value}`, text: String(value) };
			case 'url':
				return { href: String(value), text: String(value) };
			case 'link':
				if (field.linkEntity && quote) {
					const id = (quote as Record<string, unknown>)[fieldName];
					if (!id) return null;
					const entityPath = field.linkEntity.toLowerCase() + 's';
					// Get the display name from the record
					const nameField = fieldName.replace('Id', 'Name');
					const displayName = (quote as Record<string, unknown>)[nameField] || value;
					return { href: `/${entityPath}/${id}`, text: String(displayName) };
				}
				return null;
			default:
				return null;
		}
	}

	onMount(() => loadData());
</script>

<div class="space-y-6">
	{#if loading}
		<DetailSkeleton />
	{:else if error}
		<ErrorDisplay message={error} onRetry={loadData} />
	{:else if quote}
		<!-- Header -->
		<div class="flex items-start justify-between">
			<div>
				<div class="flex items-center space-x-2 text-sm text-gray-500 mb-2">
					<a href="/quotes" class="hover:text-gray-700">Quotes</a>
					<span>/</span>
					<span>{quote.name}</span>
				</div>
				<div class="flex items-center gap-3">
					<h1 class="text-2xl font-bold text-gray-900">{quote.name}</h1>
					<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium {getStatusColor(quote.status)}">
						{quote.status}
					</span>
				</div>
				<p class="mt-1 text-sm text-gray-500">{quote.quoteNumber}</p>
			</div>
			<div class="flex items-center gap-2">
				<!-- PDF Button -->
				<div class="relative">
					<div class="inline-flex rounded-md shadow-sm">
						<button
							onclick={() => generatePdf()}
							disabled={generatingPdf}
							class="inline-flex items-center px-4 py-2 text-sm font-medium text-white bg-green-600 rounded-l-md hover:bg-green-700 disabled:opacity-50"
						>
							{#if generatingPdf}
								<svg class="animate-spin w-4 h-4 mr-2" fill="none" viewBox="0 0 24 24">
									<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
									<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
								</svg>
								Generating...
							{:else}
								<svg class="w-4 h-4 mr-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
								</svg>
								Download PDF
							{/if}
						</button>
						<button
							onclick={() => showTemplateDropdown = !showTemplateDropdown}
							disabled={generatingPdf}
							class="inline-flex items-center px-2 py-2 text-white bg-green-600 rounded-r-md border-l border-green-700 hover:bg-green-700 disabled:opacity-50"
						>
							<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
							</svg>
						</button>
					</div>

					{#if showTemplateDropdown}
						<div class="absolute right-0 mt-2 w-56 bg-white border border-gray-200 rounded-lg shadow-lg z-10">
							<div class="p-2">
								<button onclick={() => generatePdf(undefined, false)}
									class="w-full text-left px-3 py-2 text-sm text-gray-700 hover:bg-gray-50 rounded">
									Preview in browser
								</button>
								<hr class="my-1">
								{#if templates.length > 0}
									<p class="px-3 py-1 text-xs font-medium text-gray-500 uppercase">Templates</p>
									{#each templates as tpl}
										<button onclick={() => generatePdf(tpl.id)}
											class="w-full text-left px-3 py-2 text-sm text-gray-700 hover:bg-gray-50 rounded flex items-center justify-between">
											{tpl.name}
											{#if tpl.isDefault}
												<span class="text-xs text-blue-600">(Default)</span>
											{/if}
										</button>
									{/each}
								{:else}
									<p class="px-3 py-2 text-xs text-gray-400">No templates configured</p>
								{/if}
							</div>
						</div>
					{/if}
				</div>

				<a href="/quotes/{quote.id}/edit"
					class="inline-flex items-center px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50">
					Edit
				</a>
				<button onclick={deleteQuote} disabled={deleting}
					class="inline-flex items-center px-4 py-2 text-sm font-medium text-red-700 bg-white border border-red-300 rounded-md hover:bg-red-50 disabled:opacity-50">
					{deleting ? 'Deleting...' : 'Delete'}
				</button>
			</div>
		</div>

		<!-- Tabs (only show if entity has activities enabled) -->
		{#if entityDef?.hasActivities}
			<div class="border-b border-gray-200">
				<nav class="-mb-px flex space-x-8">
					<button
						onclick={() => activeTab = 'details'}
						class="whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm {activeTab === 'details' ? 'border-blue-500 text-blue-600' : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'}"
					>
						Details
					</button>
					<button
						onclick={() => activeTab = 'activities'}
						class="whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm {activeTab === 'activities' ? 'border-blue-500 text-blue-600' : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'}"
					>
						Activities
					</button>
				</nav>
			</div>
		{/if}

		{#if activeTab === 'details'}
		<!-- Bearings (Stage Progress Indicators) -->
		{#if bearings.length > 0}
			<div class="space-y-4">
				{#each bearings.toSorted((a, b) => a.displayOrder - b.displayOrder) as bearing (bearing.id)}
					{@const fieldName = bearing.sourcePicklist}
					{@const currentVal = isSystemField(fieldName)
						? (quote as Record<string, unknown>)[fieldName]
						: quote.customFields?.[fieldName]}
					<Bearing
						{bearing}
						currentValue={currentVal as string | null}
						recordId={quote.id}
						entityType="Quote"
						fieldName={fieldName}
						onUpdate={(newValue) => handleBearingUpdate(fieldName, newValue)}
					/>
				{/each}
			</div>
		{/if}

		<!-- Dynamic Sections from Layout -->
		{#each visibleSections() as section (section.id)}
			<SectionRenderer
				{section}
				{fields}
				record={recordData()}
				formatValue={formatFieldValue}
				renderLink={getLinkInfo}
			/>
		{/each}

		<!-- Totals Summary (Quote-specific, always shown) -->
		<div class="bg-white shadow rounded-lg p-6">
			<h3 class="text-sm font-medium text-gray-500 uppercase tracking-wider mb-3">Totals</h3>
			<dl class="space-y-2">
				<div class="flex justify-between"><dt class="text-sm text-gray-500">Subtotal</dt><dd class="text-sm">{formatCurrency(quote.subtotal, quote.currency)}</dd></div>
				{#if quote.discountAmount > 0 || quote.discountPercent > 0}
					<div class="flex justify-between"><dt class="text-sm text-gray-500">Discount{#if quote.discountPercent > 0} ({quote.discountPercent}%){/if}</dt><dd class="text-sm text-red-600">-{formatCurrency(quote.discountAmount, quote.currency)}</dd></div>
				{/if}
				{#if quote.taxAmount > 0}
					<div class="flex justify-between"><dt class="text-sm text-gray-500">Tax{#if quote.taxPercent > 0} ({quote.taxPercent}%){/if}</dt><dd class="text-sm">{formatCurrency(quote.taxAmount, quote.currency)}</dd></div>
				{/if}
				{#if quote.shippingAmount > 0}
					<div class="flex justify-between"><dt class="text-sm text-gray-500">Shipping</dt><dd class="text-sm">{formatCurrency(quote.shippingAmount, quote.currency)}</dd></div>
				{/if}
				<div class="flex justify-between border-t pt-2"><dt class="text-sm font-bold text-gray-900">Grand Total</dt><dd class="text-lg font-bold text-gray-900">{formatCurrency(quote.grandTotal, quote.currency)}</dd></div>
			</dl>
		</div>

		<!-- Line Items (Quote-specific) -->
		{#if quote.lineItems && quote.lineItems.length > 0}
			<div class="bg-white shadow rounded-lg overflow-hidden">
				<div class="px-6 py-4 border-b border-gray-200">
					<h3 class="text-lg font-medium text-gray-900">Line Items</h3>
				</div>
				<table class="min-w-full divide-y divide-gray-200">
					<thead class="bg-gray-50">
						<tr>
							<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">#</th>
							{#each visibleLineItemFields as field (field.name)}
								<th class="px-6 py-3 text-xs font-medium text-gray-500 uppercase {['float', 'int', 'currency'].includes(field.type) ? 'text-right' : 'text-left'}">
									{field.label}
								</th>
							{/each}
							{#if visibleLineItemFields.length === 0}
								<!-- Fallback columns if no field definitions loaded -->
								<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Item</th>
								<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">SKU</th>
								<th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase">Qty</th>
								<th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase">Unit Price</th>
								<th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase">Discount</th>
								<th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase">Total</th>
							{/if}
						</tr>
					</thead>
					<tbody class="bg-white divide-y divide-gray-200">
						{#each quote.lineItems as item, i}
							<tr>
								<td class="px-6 py-4 text-sm text-gray-500">{i + 1}</td>
								{#each visibleLineItemFields as field (field.name)}
									{@const value = (item as unknown as Record<string, unknown>)[field.name]}
									<td class="px-6 py-4 text-sm {['float', 'int', 'currency'].includes(field.type) ? 'text-right' : 'text-left'}">
										{#if field.name === 'name'}
											<div class="font-medium text-gray-900">{item.name}</div>
											{#if item.description && visibleLineItemFields.some(f => f.name === 'description')}
												<!-- Description shown with name if both are visible -->
											{/if}
										{:else if field.name === 'description'}
											<span class="text-gray-500 text-xs">{item.description || '-'}</span>
										{:else if field.type === 'currency'}
											<span class="text-gray-900">{formatCurrency(value as number || 0, quote.currency)}</span>
										{:else if field.name === 'discountPercent'}
											{#if (value as number) > 0}
												<span class="text-gray-500">{value}%</span>
											{:else}
												<span class="text-gray-400">-</span>
											{/if}
										{:else if field.type === 'float' || field.type === 'int'}
											<span class="text-gray-900">{value ?? '-'}</span>
										{:else}
											<span class="text-gray-500">{value || '-'}</span>
										{/if}
									</td>
								{/each}
								{#if visibleLineItemFields.length === 0}
									<!-- Fallback row if no field definitions loaded -->
									<td class="px-6 py-4">
										<div class="text-sm font-medium text-gray-900">{item.name}</div>
										{#if item.description}
											<div class="text-xs text-gray-500">{item.description}</div>
										{/if}
									</td>
									<td class="px-6 py-4 text-sm text-gray-500">{item.sku || '-'}</td>
									<td class="px-6 py-4 text-sm text-right text-gray-900">{item.quantity}</td>
									<td class="px-6 py-4 text-sm text-right text-gray-900">{formatCurrency(item.unitPrice, quote.currency)}</td>
									<td class="px-6 py-4 text-sm text-right text-gray-500">
										{#if item.discountPercent > 0}
											{item.discountPercent}%
										{:else}
											-
										{/if}
									</td>
									<td class="px-6 py-4 text-sm text-right font-medium text-gray-900">{formatCurrency(item.total, quote.currency)}</td>
								{/if}
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}

		<!-- System Info -->
		<div class="bg-white shadow rounded-lg overflow-hidden">
			<div class="px-6 py-4 border-b border-gray-200">
				<h2 class="text-lg font-medium text-gray-900">System Information</h2>
			</div>
			<div class="px-6 py-4">
				<dl class="grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-4">
					<div>
						<dt class="text-sm font-medium text-gray-500">Created</dt>
						<dd class="mt-1 text-sm text-gray-900">
							{formatDate(quote.createdAt)}
							{#if quote.createdByName}
								<span class="text-gray-500"> by {quote.createdByName}</span>
							{/if}
						</dd>
					</div>
					<div>
						<dt class="text-sm font-medium text-gray-500">Last Modified</dt>
						<dd class="mt-1 text-sm text-gray-900">
							{formatDate(quote.modifiedAt)}
							{#if quote.modifiedByName}
								<span class="text-gray-500"> by {quote.modifiedByName}</span>
							{/if}
						</dd>
					</div>
					<div>
						<dt class="text-sm font-medium text-gray-500">ID</dt>
						<dd class="mt-1 text-sm text-gray-500 font-mono">{quote.id}</dd>
					</div>
				</dl>
			</div>
		</div>
		{:else if activeTab === 'activities'}
		<!-- Activities Tab -->
		<ActivitiesStream
			parentEntity="Quote"
			parentId={quote.id}
			parentName={quote.name}
		/>
		{/if}
	{/if}
</div>
