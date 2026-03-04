<script lang="ts">
	import { onMount } from 'svelte';
	import type { FieldDef } from '$lib/types/admin';
	import { getInputType, getEnumOptions, parseMultiEnumValue } from '$lib/utils/fieldFormatters';

	interface Props {
		field: FieldDef;
		value: unknown;
		oncommit: (newValue: unknown) => void;
		oncancel: () => void;
	}

	let { field, value, oncommit, oncancel }: Props = $props();

	// ---------------------------------------------------------------------------
	// Shared state
	// ---------------------------------------------------------------------------

	let editingValue = $state<unknown>(initValue());
	let inputEl = $state<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement | null>(null);

	// For multiEnum: track selected items in an array
	let multiSelected = $state<string[]>(parseMultiEnumValue(value));
	let multiContainerEl = $state<HTMLDivElement | null>(null);

	// ---------------------------------------------------------------------------
	// Initialise value depending on type
	// ---------------------------------------------------------------------------

	function initValue(): unknown {
		if (field.type === 'date' && value) {
			// Convert to YYYY-MM-DD
			const d = new Date(String(value));
			if (!isNaN(d.getTime())) {
				return d.toISOString().slice(0, 10);
			}
		}
		if (field.type === 'datetime' && value) {
			// Convert to YYYY-MM-DDTHH:mm
			const d = new Date(String(value));
			if (!isNaN(d.getTime())) {
				return d.toISOString().slice(0, 16);
			}
		}
		return value ?? '';
	}

	// ---------------------------------------------------------------------------
	// Event handlers shared by text-like inputs
	// ---------------------------------------------------------------------------

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			e.preventDefault();
			oncommit(editingValue);
		} else if (e.key === 'Escape') {
			e.preventDefault();
			oncancel();
		}
	}

	function handleBlur() {
		oncommit(editingValue);
	}

	// For textarea: Enter adds newline; only blur/Escape and Done button commit
	function handleTextareaKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			e.preventDefault();
			oncancel();
		}
		// Enter in textarea = newline (default behavior — do not prevent)
	}

	// For select: commit on change (blur also commits, but change gives immediate feedback)
	function handleSelectChange() {
		// Let blur handle commit for consistency
	}

	// ---------------------------------------------------------------------------
	// multiEnum container focus-out handling
	// ---------------------------------------------------------------------------

	function handleMultiContainerFocusOut(e: FocusEvent) {
		// Check if focus moved outside the container
		const related = e.relatedTarget as Node | null;
		if (multiContainerEl && related && multiContainerEl.contains(related)) {
			// Focus still inside — don't commit yet
			return;
		}
		// Focus left the container — commit
		setTimeout(() => {
			oncommit(JSON.stringify(multiSelected));
		}, 0);
	}

	function toggleMultiOption(option: string) {
		if (multiSelected.includes(option)) {
			multiSelected = multiSelected.filter((o) => o !== option);
		} else {
			multiSelected = [...multiSelected, option];
		}
	}

	// ---------------------------------------------------------------------------
	// onMount: autofocus the input
	// ---------------------------------------------------------------------------

	onMount(() => {
		if (inputEl) {
			inputEl.focus();
			if (inputEl instanceof HTMLInputElement && inputEl.type === 'text') {
				inputEl.select();
			}
		} else if (multiContainerEl) {
			// Focus first checkbox inside multi container
			const first = multiContainerEl.querySelector<HTMLElement>('input[type="checkbox"]');
			first?.focus();
		}
	});

	// ---------------------------------------------------------------------------
	// Shared input CSS class
	// ---------------------------------------------------------------------------

	const inputClass =
		'w-full px-2 py-1 text-sm border border-blue-300 rounded focus:ring-1 focus:ring-blue-500 focus:border-blue-500 outline-none';
</script>

{#if field.type === 'text'}
	<!-- Textarea: Enter = newline, Escape = cancel, blur/Done button = commit -->
	<div class="relative">
		<textarea
			bind:this={inputEl as HTMLTextAreaElement}
			bind:value={editingValue as string}
			rows="3"
			class="{inputClass} resize-none"
			onkeydown={handleTextareaKeydown}
			onblur={handleBlur}
		></textarea>
		<div class="flex justify-end mt-1 gap-1">
			<button
				type="button"
				class="text-xs px-2 py-0.5 rounded bg-gray-100 hover:bg-gray-200 text-gray-600"
				onmousedown={(e) => { e.preventDefault(); oncancel(); }}
			>Cancel</button>
			<button
				type="button"
				class="text-xs px-2 py-0.5 rounded bg-blue-600 hover:bg-blue-700 text-white"
				onmousedown={(e) => { e.preventDefault(); oncommit(editingValue); }}
			>Done</button>
		</div>
	</div>

{:else if field.type === 'enum'}
	<select
		bind:this={inputEl as HTMLSelectElement}
		bind:value={editingValue as string}
		class="{inputClass}"
		onchange={handleSelectChange}
		onblur={handleBlur}
		onkeydown={(e) => {
			if (e.key === 'Escape') { e.preventDefault(); oncancel(); }
		}}
	>
		<option value="">-- Select --</option>
		{#each getEnumOptions(field) as option}
			<option value={option}>{option}</option>
		{/each}
	</select>

{:else if field.type === 'multiEnum'}
	<!-- Compact checkbox list; onfocusout on container commits when focus leaves -->
	<div
		bind:this={multiContainerEl}
		class="border border-blue-300 rounded bg-white shadow-sm max-h-48 overflow-y-auto p-1"
		onfocusout={handleMultiContainerFocusOut}
		onkeydown={(e) => {
			if (e.key === 'Escape') { e.preventDefault(); oncancel(); }
			if (e.key === 'Enter') { e.preventDefault(); oncommit(JSON.stringify(multiSelected)); }
		}}
	>
		{#each getEnumOptions(field) as option}
			<label class="flex items-center gap-2 px-2 py-1 text-sm hover:bg-gray-50 rounded cursor-pointer">
				<input
					type="checkbox"
					checked={multiSelected.includes(option)}
					onchange={() => toggleMultiOption(option)}
					class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
				/>
				{option}
			</label>
		{/each}
		{#if getEnumOptions(field).length === 0}
			<p class="px-2 py-1 text-sm text-gray-400 italic">No options defined</p>
		{/if}
	</div>

{:else if field.type === 'bool'}
	<!-- bool should toggle directly in FieldDisplay; this is a safeguard -->
	<input
		bind:this={inputEl as HTMLInputElement}
		type="checkbox"
		checked={editingValue === true || editingValue === 'true' || editingValue === 1}
		class="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
		onchange={(e) => {
			editingValue = (e.currentTarget as HTMLInputElement).checked;
			oncommit(editingValue);
		}}
		onkeydown={(e) => {
			if (e.key === 'Escape') { e.preventDefault(); oncancel(); }
		}}
	/>

{:else if field.type === 'int'}
	<input
		bind:this={inputEl as HTMLInputElement}
		type="number"
		step="1"
		bind:value={editingValue as string}
		class="{inputClass}"
		onkeydown={handleKeydown}
		onblur={handleBlur}
	/>

{:else if field.type === 'float'}
	<input
		bind:this={inputEl as HTMLInputElement}
		type="number"
		step="0.01"
		bind:value={editingValue as string}
		class="{inputClass}"
		onkeydown={handleKeydown}
		onblur={handleBlur}
	/>

{:else if field.type === 'currency'}
	<input
		bind:this={inputEl as HTMLInputElement}
		type="number"
		step="0.01"
		bind:value={editingValue as string}
		class="{inputClass}"
		onkeydown={handleKeydown}
		onblur={handleBlur}
	/>

{:else if field.type === 'date'}
	<input
		bind:this={inputEl as HTMLInputElement}
		type="date"
		bind:value={editingValue as string}
		class="{inputClass}"
		onkeydown={handleKeydown}
		onblur={handleBlur}
	/>

{:else if field.type === 'datetime'}
	<input
		bind:this={inputEl as HTMLInputElement}
		type="datetime-local"
		bind:value={editingValue as string}
		class="{inputClass}"
		onkeydown={handleKeydown}
		onblur={handleBlur}
	/>

{:else}
	<!-- varchar, email, phone, url — and any unknown type -->
	<input
		bind:this={inputEl as HTMLInputElement}
		type={getInputType(field.type)}
		bind:value={editingValue as string}
		class="{inputClass}"
		onkeydown={handleKeydown}
		onblur={handleBlur}
	/>
{/if}
