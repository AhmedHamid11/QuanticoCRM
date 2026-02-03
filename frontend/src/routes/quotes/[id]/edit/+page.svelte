<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { get, put } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import { DetailSkeleton, ErrorDisplay } from '$lib/components/ui';
	import QuoteLineItemEditor from '$lib/components/QuoteLineItemEditor.svelte';
	import LookupField from '$lib/components/LookupField.svelte';
	import type { Quote, QuoteLineItemInput } from '$lib/types/quote';
	import type { FieldDef } from '$lib/types/admin';
	import { QUOTE_STATUSES } from '$lib/types/quote';

	let quoteId = $derived($page.params.id);
	let loading = $state(true);
	let saving = $state(false);
	let error = $state<string | null>(null);
	let lineItemFields = $state<FieldDef[]>([]);

	let name = $state('');
	let status = $state('Draft');
	let accountId = $state<string | null>(null);
	let accountName = $state('');
	let contactId = $state<string | null>(null);
	let contactName = $state('');
	let validUntil = $state('');
	let currency = $state('USD');
	let discountPercent = $state(0);
	let taxPercent = $state(0);
	let shippingAmount = $state(0);
	let description = $state('');
	let terms = $state('');
	let notes = $state('');
	let lineItems = $state<QuoteLineItemInput[]>([]);

	async function loadQuote() {
		try {
			loading = true;
			error = null;

			// Load quote and field definitions in parallel
			const [quote, fields] = await Promise.all([
				get<Quote>(`/quotes/${quoteId}`),
				get<FieldDef[]>('/entities/QuoteLineItem/fields').catch(() => [] as FieldDef[])
			]);
			lineItemFields = fields;

			name = quote.name;
			status = quote.status;
			accountId = quote.accountId;
			accountName = quote.accountName;
			contactId = quote.contactId;
			contactName = quote.contactName;
			validUntil = quote.validUntil;
			currency = quote.currency;
			discountPercent = quote.discountPercent;
			taxPercent = quote.taxPercent;
			shippingAmount = quote.shippingAmount;
			description = quote.description;
			terms = quote.terms;
			notes = quote.notes;

			lineItems = (quote.lineItems || []).map(li => ({
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

		if (!name.trim()) {
			toast.error('Name is required');
			return;
		}

		saving = true;
		try {
			await put<Quote>(`/quotes/${quoteId}`, {
				name: name.trim(),
				status,
				accountId,
				accountName,
				contactId,
				contactName,
				validUntil,
				currency,
				discountPercent,
				taxPercent,
				shippingAmount,
				description,
				terms,
				notes,
				lineItems
			});
			toast.success('Quote updated');
			goto(`/quotes/${quoteId}`);
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to update quote');
		} finally {
			saving = false;
		}
	}

	onMount(() => loadQuote());
</script>

<div class="max-w-4xl mx-auto">
	<div class="flex items-center justify-between mb-6">
		<h1 class="text-2xl font-bold text-gray-900">Edit Quote</h1>
		<a href="/quotes/{quoteId}" class="text-gray-600 hover:text-gray-900 text-sm">
			← Back to Quote
		</a>
	</div>

	{#if loading}
		<DetailSkeleton />
	{:else if error}
		<ErrorDisplay message={error} onRetry={loadQuote} />
	{:else}
		<form onsubmit={handleSubmit} class="space-y-6">
			<!-- Basic Info -->
			<div class="bg-white shadow rounded-lg p-6 space-y-4">
				<h2 class="text-lg font-medium text-gray-900">Quote Details</h2>

				<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
					<div>
						<label for="name" class="block text-sm font-medium text-gray-700 mb-1">
							Name <span class="text-red-500">*</span>
						</label>
						<input id="name" type="text" bind:value={name} required
							class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500" />
					</div>

					<div>
						<label for="status" class="block text-sm font-medium text-gray-700 mb-1">Status</label>
						<select id="status" bind:value={status}
							class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500">
							{#each QUOTE_STATUSES as s}
								<option value={s}>{s}</option>
							{/each}
						</select>
					</div>

					<div>
						<LookupField
							entity="Account"
							value={accountId}
							valueName={accountName}
							label="Account"
							onchange={(id, n) => { accountId = id; accountName = n; }}
						/>
					</div>

					<div>
						<LookupField
							entity="Contact"
							value={contactId}
							valueName={contactName}
							label="Contact"
							onchange={(id, n) => { contactId = id; contactName = n; }}
						/>
					</div>

					<div>
						<label for="validUntil" class="block text-sm font-medium text-gray-700 mb-1">Valid Until</label>
						<input id="validUntil" type="date" bind:value={validUntil}
							class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500" />
					</div>

					<div>
						<label for="currency" class="block text-sm font-medium text-gray-700 mb-1">Currency</label>
						<select id="currency" bind:value={currency}
							class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500">
							<option value="USD">USD</option>
							<option value="EUR">EUR</option>
							<option value="GBP">GBP</option>
						</select>
					</div>
				</div>
			</div>

			<!-- Line Items -->
			<div class="bg-white shadow rounded-lg p-6">
				<QuoteLineItemEditor bind:items={lineItems} fields={lineItemFields} {currency} />
			</div>

			<!-- Pricing -->
			<div class="bg-white shadow rounded-lg p-6 space-y-4">
				<h2 class="text-lg font-medium text-gray-900">Pricing</h2>
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

			<!-- Additional Info -->
			<div class="bg-white shadow rounded-lg p-6 space-y-4">
				<h2 class="text-lg font-medium text-gray-900">Additional Information</h2>
				<div>
					<label for="description" class="block text-sm font-medium text-gray-700 mb-1">Description</label>
					<textarea id="description" bind:value={description} rows="3"
						class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"></textarea>
				</div>
				<div>
					<label for="terms" class="block text-sm font-medium text-gray-700 mb-1">Terms & Conditions</label>
					<textarea id="terms" bind:value={terms} rows="3"
						class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"></textarea>
				</div>
				<div>
					<label for="notes" class="block text-sm font-medium text-gray-700 mb-1">Notes</label>
					<textarea id="notes" bind:value={notes} rows="2"
						class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"></textarea>
				</div>
			</div>

			<!-- Actions -->
			<div class="flex justify-end gap-3">
				<a href="/quotes/{quoteId}" class="px-4 py-2 text-gray-700 bg-gray-100 rounded-md hover:bg-gray-200">
					Cancel
				</a>
				<button type="submit" disabled={saving}
					class="px-4 py-2 text-white bg-blue-600 rounded-md hover:bg-blue-600/90 disabled:opacity-50">
					{saving ? 'Saving...' : 'Save Changes'}
				</button>
			</div>
		</form>
	{/if}
</div>
