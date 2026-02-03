<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { get, del } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import { TableSkeleton, ErrorDisplay } from '$lib/components/ui';
	import type { Quote, QuoteListResponse } from '$lib/types/quote';
	import { getStatusColor, formatCurrency } from '$lib/types/quote';

	let quotes = $state<Quote[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let search = $state('');
	let page = $state(1);
	let pageSize = $state(20);
	let total = $state(0);
	let totalPages = $state(0);
	let sortBy = $state('created_at');
	let sortDir = $state<'asc' | 'desc'>('desc');
	let searchTimeout: ReturnType<typeof setTimeout> | null = null;
	let knownTotal = $state<number | null>(null);

	async function loadQuotes() {
		try {
			loading = true;
			error = null;
			const params = new URLSearchParams({
				page: page.toString(),
				pageSize: pageSize.toString(),
				sortBy,
				sortDir
			});
			if (search) params.set('search', search);
			if (knownTotal !== null && page > 1) params.set('knownTotal', knownTotal.toString());

			const result = await get<QuoteListResponse>(`/quotes?${params}`);
			quotes = result.data;
			total = result.total;
			totalPages = result.totalPages;
			knownTotal = result.total;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load quotes';
			toast.error(error);
		} finally {
			loading = false;
		}
	}

	async function deleteQuote(id: string) {
		const backup = [...quotes];
		quotes = quotes.filter(q => q.id !== id);
		total -= 1;

		try {
			await del(`/quotes/${id}`);
			toast.success('Quote deleted');
		} catch (e) {
			quotes = backup;
			total += 1;
			toast.error(e instanceof Error ? e.message : 'Failed to delete quote');
		}
	}

	function handleSearchInput() {
		if (searchTimeout) clearTimeout(searchTimeout);
		searchTimeout = setTimeout(() => {
			page = 1;
			knownTotal = null;
			loadQuotes();
		}, 300);
	}

	function handleSort(column: string) {
		if (sortBy === column) {
			sortDir = sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			sortBy = column;
			sortDir = 'asc';
		}
		knownTotal = null;
		loadQuotes();
	}

	function formatDate(dateStr: string): string {
		if (!dateStr) return '-';
		return new Date(dateStr).toLocaleDateString();
	}

	onMount(() => {
		loadQuotes();
	});
</script>

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<h1 class="text-2xl font-bold text-gray-900">Quotes</h1>
		<a
			href="/quotes/new"
			class="inline-flex items-center px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-600/90"
		>
			<svg class="w-4 h-4 mr-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
			</svg>
			New Quote
		</a>
	</div>

	<!-- Search -->
	<div class="flex gap-3">
		<div class="flex-1">
			<input
				type="text"
				bind:value={search}
				oninput={handleSearchInput}
				placeholder="Search quotes..."
				class="w-full px-4 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
			/>
		</div>
	</div>

	{#if loading}
		<TableSkeleton />
	{:else if error}
		<ErrorDisplay message={error} onRetry={loadQuotes} />
	{:else if quotes.length === 0}
		<div class="text-center py-12 bg-white rounded-lg shadow">
			<svg class="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
			</svg>
			<h3 class="mt-2 text-sm font-medium text-gray-900">No quotes</h3>
			<p class="mt-1 text-sm text-gray-500">Get started by creating a new quote.</p>
			<div class="mt-4">
				<a href="/quotes/new" class="inline-flex items-center px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-600/90">
					New Quote
				</a>
			</div>
		</div>
	{:else}
		<div class="bg-white shadow rounded-lg overflow-hidden">
			<table class="min-w-full divide-y divide-gray-200">
				<thead class="bg-gray-50">
					<tr>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
							onclick={() => handleSort('quote_number')}>
							Number
							{#if sortBy === 'quote_number'}<span>{sortDir === 'asc' ? ' ↑' : ' ↓'}</span>{/if}
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
							onclick={() => handleSort('name')}>
							Name
							{#if sortBy === 'name'}<span>{sortDir === 'asc' ? ' ↑' : ' ↓'}</span>{/if}
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
							onclick={() => handleSort('status')}>
							Status
							{#if sortBy === 'status'}<span>{sortDir === 'asc' ? ' ↑' : ' ↓'}</span>{/if}
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Account
						</th>
						<th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
							onclick={() => handleSort('grand_total')}>
							Total
							{#if sortBy === 'grand_total'}<span>{sortDir === 'asc' ? ' ↑' : ' ↓'}</span>{/if}
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Valid Until
						</th>
						<th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
							Actions
						</th>
					</tr>
				</thead>
				<tbody class="bg-white divide-y divide-gray-200">
					{#each quotes as quote (quote.id)}
						<tr class="hover:bg-gray-50 cursor-pointer" onclick={() => goto(`/quotes/${quote.id}`)}>
							<td class="px-6 py-4 whitespace-nowrap text-sm font-medium text-blue-600">
								{quote.quoteNumber}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
								{quote.name}
							</td>
							<td class="px-6 py-4 whitespace-nowrap">
								<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium {getStatusColor(quote.status)}">
									{quote.status}
								</span>
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{quote.accountName || '-'}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-right font-medium text-gray-900">
								{formatCurrency(quote.grandTotal, quote.currency)}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{formatDate(quote.validUntil)}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-right text-sm">
								<!-- svelte-ignore a11y_click_events_have_key_events -->
								<button
									onclick={(e) => { e.stopPropagation(); deleteQuote(quote.id); }}
									class="text-red-600 hover:text-red-900"
								>
									Delete
								</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<!-- Pagination -->
		{#if totalPages > 1}
			<div class="flex items-center justify-between">
				<p class="text-sm text-gray-700">
					Showing {(page - 1) * pageSize + 1} to {Math.min(page * pageSize, total)} of {total}
				</p>
				<div class="flex gap-2">
					<button
						onclick={() => { page--; loadQuotes(); }}
						disabled={page <= 1}
						class="px-3 py-1 text-sm border rounded-md disabled:opacity-50 hover:bg-gray-50"
					>
						Previous
					</button>
					<span class="px-3 py-1 text-sm text-gray-700">Page {page} of {totalPages}</span>
					<button
						onclick={() => { page++; loadQuotes(); }}
						disabled={page >= totalPages}
						class="px-3 py-1 text-sm border rounded-md disabled:opacity-50 hover:bg-gray-50"
					>
						Next
					</button>
				</div>
			</div>
		{/if}
	{/if}
</div>
