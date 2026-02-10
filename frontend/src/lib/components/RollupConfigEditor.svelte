<script lang="ts">
	interface Props {
		rollupResultType: string | undefined;
		rollupQuery: string | undefined;
		rollupDecimalPlaces: number | undefined;
		onResultTypeChange: (value: string) => void;
		onQueryChange: (value: string) => void;
		onDecimalPlacesChange: (value: number) => void;
	}

	let {
		rollupResultType = $bindable(),
		rollupQuery = $bindable(),
		rollupDecimalPlaces = $bindable(),
		onResultTypeChange,
		onQueryChange,
		onDecimalPlacesChange
	}: Props = $props();
</script>

<div class="space-y-4">
	<div>
		<label for="rollupResultType" class="block text-sm font-medium text-gray-700 mb-1">
			Result Type <span class="text-red-500">*</span>
		</label>
		<select
			id="rollupResultType"
			value={rollupResultType || ''}
			onchange={(e) => onResultTypeChange(e.currentTarget.value)}
			class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
		>
			<option value="">Select result type...</option>
			<option value="numeric">Numeric (SUM, COUNT, AVG, MIN, MAX)</option>
			<option value="text">Text (GROUP_CONCAT)</option>
		</select>
	</div>

	{#if rollupResultType === 'numeric'}
		<div>
			<label for="rollupDecimalPlaces" class="block text-sm font-medium text-gray-700 mb-1">
				Decimal Places
			</label>
			<input
				id="rollupDecimalPlaces"
				type="number"
				value={rollupDecimalPlaces ?? ''}
				oninput={(e) => onDecimalPlacesChange(parseInt(e.currentTarget.value) || 0)}
				min="0"
				max="10"
				class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
				placeholder="2"
			/>
		</div>
	{/if}

	<div>
		<label for="rollupQuery" class="block text-sm font-medium text-gray-700 mb-1">
			SQL Query <span class="text-red-500">*</span>
		</label>
		<textarea
			id="rollupQuery"
			value={rollupQuery || ''}
			oninput={(e) => onQueryChange(e.currentTarget.value)}
			rows="4"
			class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500 font-mono text-sm"
			placeholder={"SELECT COUNT(*) FROM contacts WHERE account_id = '{{id}}'"}
		></textarea>
		<p class="mt-1 text-xs text-gray-500">
			Use <code class="bg-gray-100 px-1 rounded">{'{{id}}'}</code> to reference the current record's ID (optional).
			Query must return a single value. Omit {'{{id}}'} for global aggregates.
		</p>
		<div class="mt-2 p-3 bg-amber-50 border border-amber-200 rounded-md">
			<p class="text-xs text-amber-800">
				<strong>Security:</strong> Only SELECT queries are allowed. The query runs with a 2-second timeout.
			</p>
		</div>
	</div>
</div>
