<script lang="ts">
	import { get } from '$lib/utils/api';

	interface LookupRecord {
		id: string;
		name: string;
	}

	interface LookupSearchResult {
		records: LookupRecord[];
		total: number;
	}

	interface Props {
		entity: string;
		value: string | null;
		valueName: string;
		label: string;
		required?: boolean;
		disabled?: boolean;
		onchange: (id: string | null, name: string) => void;
	}

	let { entity, value, valueName, label, required = false, disabled = false, onchange }: Props = $props();

	let searchTerm = $state('');
	let results = $state<LookupRecord[]>([]);
	let showDropdown = $state(false);
	let loading = $state(false);
	let inputElement: HTMLInputElement;
	let debounceTimer: ReturnType<typeof setTimeout>;

	// Display name when we have a value
	let displayValue = $derived(value ? valueName : '');

	async function search(term: string) {
		if (!term || term.length < 1) {
			results = [];
			return;
		}

		loading = true;
		try {
			const data = await get<LookupSearchResult>(`/lookup/${entity}?search=${encodeURIComponent(term)}&limit=10`);
			results = data.records || [];
		} catch (e) {
			console.error('Lookup search failed:', e);
			results = [];
		} finally {
			loading = false;
		}
	}

	function handleInput(e: Event) {
		const target = e.target as HTMLInputElement;
		searchTerm = target.value;
		showDropdown = true;

		// Clear selection if user is typing
		if (value) {
			onchange(null, '');
		}

		// Debounce search
		clearTimeout(debounceTimer);
		debounceTimer = setTimeout(() => {
			search(searchTerm);
		}, 200);
	}

	function handleFocus() {
		showDropdown = true;
		if (!value && searchTerm) {
			search(searchTerm);
		}
	}

	function handleBlur() {
		// Delay hiding to allow click on dropdown item
		setTimeout(() => {
			showDropdown = false;
			// Reset to display value if no selection made
			if (!value) {
				searchTerm = '';
			}
		}, 200);
	}

	function selectRecord(record: LookupRecord) {
		onchange(record.id, record.name);
		searchTerm = '';
		showDropdown = false;
		results = [];
	}

	function clearSelection() {
		onchange(null, '');
		searchTerm = '';
		results = [];
		inputElement?.focus();
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			showDropdown = false;
		}
	}
</script>

<div class="relative">
	<label class="block text-sm font-medium text-gray-700 mb-1">
		{label}
		{#if required}
			<span class="text-red-500">*</span>
		{/if}
	</label>

	<div class="relative">
		{#if value}
			<!-- Show selected value with clear button -->
			<div class="flex items-center gap-2 w-full px-3 py-2 border border-gray-300 rounded-md bg-gray-50">
				<a
					href="/{entity.toLowerCase()}s/{value}"
					class="text-primary hover:text-blue-800 hover:underline flex-1 truncate"
				>
					{valueName}
				</a>
				{#if !disabled}
					<button
						type="button"
						onclick={clearSelection}
						class="text-gray-400 hover:text-gray-600 flex-shrink-0"
						aria-label="Clear selection"
					>
						<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
						</svg>
					</button>
				{/if}
			</div>
		{:else}
			<!-- Search input -->
			<input
				bind:this={inputElement}
				type="text"
				value={searchTerm}
				oninput={handleInput}
				onfocus={handleFocus}
				onblur={handleBlur}
				onkeydown={handleKeydown}
				placeholder="Search {entity}..."
				class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary focus:border-primary"
				{disabled}
				required={required && !value}
			/>

			{#if loading}
				<div class="absolute right-3 top-1/2 transform -translate-y-1/2">
					<svg class="w-4 h-4 animate-spin text-gray-400" fill="none" viewBox="0 0 24 24">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
						<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
					</svg>
				</div>
			{/if}
		{/if}

		<!-- Dropdown -->
		{#if showDropdown && !value && (results.length > 0 || (searchTerm && !loading))}
			<div class="absolute z-50 mt-1 w-full bg-white border border-gray-300 rounded-md shadow-lg max-h-60 overflow-auto">
				{#if results.length > 0}
					{#each results as record (record.id)}
						<button
							type="button"
							onclick={() => selectRecord(record)}
							class="w-full px-3 py-2 text-left hover:bg-blue-50 focus:bg-blue-50 focus:outline-none"
						>
							{record.name}
						</button>
					{/each}
				{:else if searchTerm && !loading}
					<div class="px-3 py-2 text-gray-500 text-sm">No results found</div>
				{/if}
			</div>
		{/if}
	</div>
</div>
