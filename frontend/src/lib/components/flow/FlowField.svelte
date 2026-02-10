<script lang="ts">
	import type { ScreenField } from '$lib/types/flow';
	import FormField from '$lib/components/ui/FormField.svelte';

	interface Props {
		field: ScreenField;
		value: unknown;
		error?: string;
		onchange: (value: unknown) => void;
	}

	let { field, value, error, onchange }: Props = $props();

	// Convert value for input binding
	let inputValue = $derived(value ?? '');

	function handleInput(e: Event) {
		const target = e.target as HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement;
		let newValue: unknown = target.value;

		// Convert to appropriate type
		if (field.type === 'number' || field.type === 'currency' || field.type === 'percent') {
			const parsed = parseFloat(target.value);
			newValue = isNaN(parsed) ? '' : parsed;
		} else if (field.type === 'checkbox') {
			newValue = (target as HTMLInputElement).checked;
		}

		onchange(newValue);
	}

	function handleSelectChange(e: Event) {
		const target = e.target as HTMLSelectElement;
		onchange(target.value);
	}

	const inputClasses = 'block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm';
	const errorClasses = 'border-red-300 focus:border-red-500 focus:ring-red-500';
</script>

<FormField label={field.label} htmlFor={field.name} required={field.required} {error}>
	{#if field.type === 'text' || field.type === 'email' || field.type === 'phone' || field.type === 'url'}
		<input
			type={field.type === 'email' ? 'email' : field.type === 'phone' ? 'tel' : field.type === 'url' ? 'url' : 'text'}
			id={field.name}
			name={field.name}
			value={inputValue}
			placeholder={field.placeholder}
			oninput={handleInput}
			class="{inputClasses} {error ? errorClasses : ''}"
		/>
	{:else if field.type === 'textarea'}
		<textarea
			id={field.name}
			name={field.name}
			rows={field.rows || 3}
			placeholder={field.placeholder}
			oninput={handleInput}
			class="{inputClasses} {error ? errorClasses : ''}"
		>{inputValue}</textarea>
	{:else if field.type === 'number' || field.type === 'currency' || field.type === 'percent'}
		<div class="relative">
			{#if field.type === 'currency'}
				<div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
					<span class="text-gray-500 sm:text-sm">$</span>
				</div>
			{/if}
			<input
				type="number"
				id={field.name}
				name={field.name}
				value={inputValue}
				placeholder={field.placeholder}
				min={field.minValue}
				max={field.maxValue}
				step={field.type === 'currency' ? '0.01' : field.type === 'percent' ? '0.1' : '1'}
				oninput={handleInput}
				class="{inputClasses} {error ? errorClasses : ''} {field.type === 'currency' ? 'pl-7' : ''} {field.type === 'percent' ? 'pr-8' : ''}"
			/>
			{#if field.type === 'percent'}
				<div class="absolute inset-y-0 right-0 pr-3 flex items-center pointer-events-none">
					<span class="text-gray-500 sm:text-sm">%</span>
				</div>
			{/if}
		</div>
	{:else if field.type === 'date'}
		<input
			type="date"
			id={field.name}
			name={field.name}
			value={inputValue}
			oninput={handleInput}
			class="{inputClasses} {error ? errorClasses : ''}"
		/>
	{:else if field.type === 'datetime'}
		<input
			type="datetime-local"
			id={field.name}
			name={field.name}
			value={inputValue}
			oninput={handleInput}
			class="{inputClasses} {error ? errorClasses : ''}"
		/>
	{:else if field.type === 'select' && field.options}
		<select
			id={field.name}
			name={field.name}
			value={inputValue}
			onchange={handleSelectChange}
			class="{inputClasses} {error ? errorClasses : ''}"
		>
			<option value="">Select an option...</option>
			{#each field.options as option}
				<option value={option.value}>{option.label}</option>
			{/each}
		</select>
	{:else if field.type === 'radio' && field.options}
		<div class="space-y-2 mt-1">
			{#each field.options as option}
				<label class="flex items-center">
					<input
						type="radio"
						name={field.name}
						value={option.value}
						checked={inputValue === option.value}
						onchange={handleInput}
						class="h-4 w-4 text-blue-600 border-gray-300 focus:ring-blue-500"
					/>
					<span class="ml-2 text-sm text-gray-700">{option.label}</span>
				</label>
			{/each}
		</div>
	{:else if field.type === 'checkbox'}
		<div class="flex items-center mt-1">
			<input
				type="checkbox"
				id={field.name}
				name={field.name}
				checked={Boolean(value)}
				onchange={handleInput}
				class="h-4 w-4 text-blue-600 border-gray-300 rounded focus:ring-blue-500"
			/>
			{#if field.helpText}
				<span class="ml-2 text-sm text-gray-500">{field.helpText}</span>
			{/if}
		</div>
	{:else if field.type === 'lookup'}
		<!-- TODO: Implement lookup field with search/autocomplete -->
		<input
			type="text"
			id={field.name}
			name={field.name}
			value={inputValue}
			placeholder={field.placeholder || `Search ${field.entity}...`}
			oninput={handleInput}
			class="{inputClasses} {error ? errorClasses : ''}"
		/>
		<p class="mt-1 text-xs text-gray-500">Lookup for {field.entity}</p>
	{:else if field.type === 'display'}
		<!-- Read-only display field for showing variable values, with optional variant styling -->
		{@const variant = (field as unknown as { variant?: string }).variant}
		{#if variant && variant !== 'default'}
			{@const variantClasses = {
				info: 'bg-blue-50 border-blue-200 text-blue-800',
				warning: 'bg-amber-50 border-amber-200 text-amber-800',
				error: 'bg-red-50 border-red-200 text-red-800',
				success: 'bg-green-50 border-green-200 text-green-800'
			}[variant] || 'bg-gray-50 border-gray-200 text-gray-900'}
			{@const iconPath = {
				info: 'M11.25 11.25l.041-.02a.75.75 0 011.063.852l-.708 2.836a.75.75 0 001.063.853l.041-.021M21 12a9 9 0 11-18 0 9 9 0 0118 0zm-9-3.75h.008v.008H12V8.25z',
				warning: 'M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z',
				error: 'M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z',
				success: 'M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z'
			}[variant] || ''}
			<div class="rounded-md border p-3 {variantClasses}">
				<div class="flex">
					{#if iconPath}
						<div class="flex-shrink-0">
							<svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
								<path stroke-linecap="round" stroke-linejoin="round" d={iconPath} />
							</svg>
						</div>
					{/if}
					<div class="{iconPath ? 'ml-3' : ''} text-sm font-medium">
						{inputValue || '—'}
					</div>
				</div>
			</div>
		{:else}
			<div class="py-2 px-3 bg-gray-50 border border-gray-200 rounded-md text-sm text-gray-900 font-medium">
				{inputValue || '—'}
			</div>
		{/if}
	{:else}
		<!-- Fallback to text input -->
		<input
			type="text"
			id={field.name}
			name={field.name}
			value={inputValue}
			placeholder={field.placeholder}
			oninput={handleInput}
			class="{inputClasses} {error ? errorClasses : ''}"
		/>
	{/if}

	{#if field.helpText && field.type !== 'checkbox'}
		<p class="mt-1 text-xs text-gray-500">{field.helpText}</p>
	{/if}
</FormField>
