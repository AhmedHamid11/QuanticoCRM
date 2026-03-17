<script lang="ts">
	import type { QuoteLineItemInput } from '$lib/types/quote';
	import type { FieldDef } from '$lib/types/admin';
	import { formatCurrency } from '$lib/types/quote';

	interface Props {
		items: QuoteLineItemInput[];
		fields?: FieldDef[];
		currency?: string;
		disabled?: boolean;
		onchange?: (items: QuoteLineItemInput[]) => void;
	}

	let { items = $bindable([]), fields = [], currency = 'USD', disabled = false, onchange }: Props = $props();

	// Default fields to show if no field definitions provided
	const defaultVisibleFields = ['name', 'description', 'sku', 'quantity', 'unitPrice', 'discountPercent', 'total'];

	// Fields that should be visible in the table (sorted by sortOrder)
	let visibleFields = $derived(
		fields.length > 0
			? fields
					.filter(f => !['quoteId', 'sortOrder', 'createdAt', 'modifiedAt', 'id'].includes(f.name))
					.sort((a, b) => a.sortOrder - b.sortOrder)
			: defaultVisibleFields.map((name, i) => ({
					id: name,
					entityName: 'QuoteLineItem',
					name,
					label: getDefaultLabel(name),
					type: getDefaultType(name),
					isRequired: ['name', 'quantity', 'unitPrice'].includes(name),
					isReadOnly: name === 'total',
					isAudited: false,
					isCustom: false,
					sortOrder: i,
					createdAt: '',
					modifiedAt: ''
				} as FieldDef))
	);

	function getDefaultLabel(name: string): string {
		const labels: Record<string, string> = {
			name: 'Item',
			description: 'Description',
			sku: 'SKU',
			quantity: 'Qty',
			unitPrice: 'Price',
			discountPercent: 'Disc %',
			discountAmount: 'Disc Amt',
			taxPercent: 'Tax %',
			total: 'Total'
		};
		return labels[name] || name;
	}

	function getDefaultType(name: string): string {
		const types: Record<string, string> = {
			name: 'varchar',
			description: 'text',
			sku: 'varchar',
			quantity: 'float',
			unitPrice: 'currency',
			discountPercent: 'float',
			discountAmount: 'currency',
			taxPercent: 'float',
			total: 'currency'
		};
		return types[name] || 'varchar';
	}

	function addItem() {
		const newItem: QuoteLineItemInput = {
			name: '',
			description: '',
			sku: '',
			quantity: 1,
			unitPrice: 0,
			discountPercent: 0,
			discountAmount: 0,
			taxPercent: 0,
			sortOrder: items.length
		};
		items = [...items, newItem];
		notifyChange();
	}

	function removeItem(index: number) {
		items = items.filter((_, i) => i !== index);
		notifyChange();
	}

	function moveItem(from: number, to: number) {
		if (to < 0 || to >= items.length) return;
		const arr = [...items];
		const [item] = arr.splice(from, 1);
		arr.splice(to, 0, item);
		items = arr;
		notifyChange();
	}

	function calcLineTotal(item: QuoteLineItemInput): number {
		let total = item.quantity * item.unitPrice;
		if (item.discountPercent > 0) {
			total -= (total * item.discountPercent) / 100;
		} else if (item.discountAmount > 0) {
			total -= item.discountAmount;
		}
		return Math.round(total * 100) / 100;
	}

	function notifyChange() {
		onchange?.(items);
	}

	function getFieldValue(item: QuoteLineItemInput, fieldName: string): unknown {
		if (fieldName === 'total') {
			return calcLineTotal(item);
		}
		return (item as unknown as Record<string, unknown>)[fieldName];
	}

	function setFieldValue(item: QuoteLineItemInput, fieldName: string, value: unknown) {
		(item as unknown as Record<string, unknown>)[fieldName] = value;
		items = [...items]; // Trigger reactivity
		notifyChange();
	}

	function getColumnWidth(fieldName: string): string {
		const widths: Record<string, string> = {
			name: '',
			description: '',
			sku: 'w-20',
			quantity: 'w-16',
			unitPrice: 'w-24',
			discountPercent: 'w-20',
			discountAmount: 'w-24',
			taxPercent: 'w-20',
			total: 'w-24'
		};
		return widths[fieldName] || 'w-24';
	}

	function getColumnAlign(field: FieldDef): string {
		if (['float', 'int', 'currency'].includes(field.type)) {
			return 'text-right';
		}
		return 'text-left';
	}

	let subtotal = $derived(
		items.reduce((sum, item) => sum + calcLineTotal(item), 0)
	);

	// Count columns for colspan calculation
	let columnCount = $derived(visibleFields.length + 1 + (disabled ? 0 : 1)); // +1 for row number, +1 for actions
</script>

<div class="space-y-3">
	<div class="flex items-center justify-between">
		<h3 class="text-sm font-medium text-gray-700">Line Items</h3>
		{#if !disabled}
			<button
				type="button"
				onclick={addItem}
				class="inline-flex items-center px-3 py-1.5 text-xs font-medium text-blue-700 bg-blue-50 rounded-md hover:bg-blue-100"
			>
				<svg class="w-3.5 h-3.5 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
				</svg>
				Add Item
			</button>
		{/if}
	</div>

	{#if items.length === 0}
		<div class="text-center py-8 text-gray-400 border-2 border-dashed border-gray-200 rounded-lg">
			<p class="text-sm">No line items yet</p>
			{#if !disabled}
				<button type="button" onclick={addItem} class="mt-2 text-sm text-blue-600 hover:text-blue-700">
					Add your first item
				</button>
			{/if}
		</div>
	{:else}
		<div class="overflow-x-auto border border-gray-200 rounded-lg">
			<table class="min-w-full divide-y divide-gray-200">
				<thead class="bg-gray-50">
					<tr>
						<th class="px-3 py-2 text-left text-xs font-medium text-gray-500 uppercase w-8">#</th>
						{#each visibleFields as field (field.name)}
							<th class="px-3 py-2 {getColumnAlign(field)} text-xs font-medium text-gray-500 uppercase {getColumnWidth(field.name)}">
								{field.label}
							</th>
						{/each}
						{#if !disabled}
							<th class="px-3 py-2 w-20"></th>
						{/if}
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-200">
					{#each items as item, i (i)}
						<tr>
							<td class="px-3 py-2 text-sm text-gray-500">{i + 1}</td>
							{#each visibleFields as field (field.name)}
								<td class="px-3 py-2 {getColumnAlign(field)}">
									{#if field.isReadOnly || field.name === 'total'}
										<!-- Read-only field (e.g., total) -->
										<span class="text-sm font-medium">
											{#if field.type === 'currency'}
												{formatCurrency(getFieldValue(item, field.name) as number, currency)}
											{:else}
												{getFieldValue(item, field.name) ?? '-'}
											{/if}
										</span>
									{:else if field.name === 'name'}
										<!-- Special handling for name + description in same cell -->
										<input
											type="text"
											value={item.name}
											oninput={(e) => setFieldValue(item, 'name', (e.target as HTMLInputElement).value)}
											placeholder="Item name"
											{disabled}
											class="w-full text-sm border-0 border-b border-transparent focus:border-blue-500 focus:ring-0 p-0 bg-transparent"
										/>
										{#if visibleFields.some(f => f.name === 'description')}
											<!-- Description is shown inline with name -->
										{/if}
									{:else if field.name === 'description'}
										<input
											type="text"
											value={item.description ?? ''}
											oninput={(e) => setFieldValue(item, 'description', (e.target as HTMLInputElement).value)}
											placeholder="-"
											{disabled}
											class="w-full text-xs text-gray-400 border-0 focus:ring-0 p-0 bg-transparent"
										/>
									{:else if field.type === 'float' || field.type === 'int'}
										<input
											type="number"
											value={getFieldValue(item, field.name) as number ?? 0}
											oninput={(e) => setFieldValue(item, field.name, parseFloat((e.target as HTMLInputElement).value) || 0)}
											min={field.minValue ?? 0}
											max={field.maxValue ?? undefined}
											step={field.type === 'int' ? '1' : '0.01'}
											{disabled}
											class="w-full text-sm text-right border-0 border-b border-transparent focus:border-blue-500 focus:ring-0 p-0 bg-transparent"
										/>
									{:else if field.type === 'currency'}
										<input
											type="number"
											value={getFieldValue(item, field.name) as number ?? 0}
											oninput={(e) => setFieldValue(item, field.name, parseFloat((e.target as HTMLInputElement).value) || 0)}
											min="0"
											step="0.01"
											{disabled}
											class="w-full text-sm text-right border-0 border-b border-transparent focus:border-blue-500 focus:ring-0 p-0 bg-transparent"
										/>
									{:else}
										<!-- Default text input for varchar, text, etc. -->
										<input
											type="text"
											value={getFieldValue(item, field.name) as string ?? ''}
											oninput={(e) => setFieldValue(item, field.name, (e.target as HTMLInputElement).value)}
											placeholder="-"
											{disabled}
											class="w-full text-sm border-0 border-b border-transparent focus:border-blue-500 focus:ring-0 p-0 bg-transparent"
										/>
									{/if}
								</td>
							{/each}
							{#if !disabled}
								<td class="px-3 py-2">
									<div class="flex items-center gap-1">
										<button
											type="button"
											onclick={() => moveItem(i, i - 1)}
											disabled={i === 0}
											class="p-1 text-gray-400 hover:text-gray-600 disabled:opacity-30"
											title="Move up"
										>
											<svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
												<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 15l7-7 7 7" />
											</svg>
										</button>
										<button
											type="button"
											onclick={() => moveItem(i, i + 1)}
											disabled={i === items.length - 1}
											class="p-1 text-gray-400 hover:text-gray-600 disabled:opacity-30"
											title="Move down"
										>
											<svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
												<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
											</svg>
										</button>
										<button
											type="button"
											onclick={() => removeItem(i)}
											class="p-1 text-red-400 hover:text-red-600"
											title="Remove"
										>
											<svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
												<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
											</svg>
										</button>
									</div>
								</td>
							{/if}
						</tr>
					{/each}
				</tbody>
				<tfoot>
					<tr class="bg-gray-50">
						<td colspan={columnCount - 1} class="px-3 py-2 text-right text-sm font-medium text-gray-700">
							Subtotal
						</td>
						<td class="px-3 py-2 text-right text-sm font-bold">
							{formatCurrency(subtotal, currency)}
						</td>
					</tr>
				</tfoot>
			</table>
		</div>
	{/if}
</div>
