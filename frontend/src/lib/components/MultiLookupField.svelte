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
		values: LookupRecord[];
		label: string;
		required?: boolean;
		disabled?: boolean;
		onchange: (values: LookupRecord[]) => void;
	}

	let { entity, values = [], label, required = false, disabled = false, onchange }: Props = $props();

	let searchTerm = $state('');
	let results = $state<LookupRecord[]>([]);
	let showDropdown = $state(false);
	let loading = $state(false);
	let inputElement: HTMLInputElement;
	let debounceTimer: ReturnType<typeof setTimeout>;

	// Filter out already selected records from search results
	let filteredResults = $derived(
		results.filter(r => !values.some(v => v.id === r.id))
	);

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

		// Debounce search
		clearTimeout(debounceTimer);
		debounceTimer = setTimeout(() => {
			search(searchTerm);
		}, 200);
	}

	function handleFocus() {
		showDropdown = true;
		if (searchTerm) {
			search(searchTerm);
		}
	}

	function handleBlur() {
		// Delay hiding to allow click on dropdown item
		setTimeout(() => {
			showDropdown = false;
		}, 200);
	}

	function selectRecord(record: LookupRecord) {
		const newValues = [...values, record];
		onchange(newValues);
		searchTerm = '';
		results = [];
		inputElement?.focus();
	}

	function removeRecord(recordId: string) {
		const newValues = values.filter(v => v.id !== recordId);
		onchange(newValues);
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			showDropdown = false;
		}
		// Remove last item on backspace if search is empty
		if (e.key === 'Backspace' && !searchTerm && values.length > 0) {
			removeRecord(values[values.length - 1].id);
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

	<div class="w-full min-h-[42px] px-2 py-1 border border-gray-300 rounded-md focus-within:ring-2 focus-within:ring-blue-500 focus-within:border-blue-500 bg-white">
		<!-- Selected items as chips -->
		<div class="flex flex-wrap gap-1 items-center">
			{#each values as record (record.id)}
				<span class="inline-flex items-center gap-1 px-2 py-1 bg-blue-100 text-blue-800 text-sm rounded">
					<a
						href="/{entity.toLowerCase()}s/{record.id}"
						class="hover:underline"
					>
						{record.name}
					</a>
					{#if !disabled}
						<button
							type="button"
							onclick={() => removeRecord(record.id)}
							class="text-blue-600 hover:text-blue-800"
							aria-label="Remove {record.name}"
						>
							<svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
							</svg>
						</button>
					{/if}
				</span>
			{/each}

			<!-- Search input inline with chips -->
			{#if !disabled}
				<input
					bind:this={inputElement}
					type="text"
					value={searchTerm}
					oninput={handleInput}
					onfocus={handleFocus}
					onblur={handleBlur}
					onkeydown={handleKeydown}
					placeholder={values.length === 0 ? `Search ${entity}...` : ''}
					class="flex-1 min-w-[120px] border-none outline-none focus:ring-0 py-1 px-1 text-sm"
				/>
			{/if}
		</div>

		{#if loading}
			<div class="absolute right-3 top-1/2 transform -translate-y-1/2">
				<svg class="w-4 h-4 animate-spin text-gray-400" fill="none" viewBox="0 0 24 24">
					<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
					<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
				</svg>
			</div>
		{/if}
	</div>

	<!-- Dropdown -->
	{#if showDropdown && (filteredResults.length > 0 || (searchTerm && !loading))}
		<div class="absolute z-50 mt-1 w-full bg-white border border-gray-300 rounded-md shadow-lg max-h-60 overflow-auto">
			{#if filteredResults.length > 0}
				{#each filteredResults as record (record.id)}
					<button
						type="button"
						onclick={() => selectRecord(record)}
						class="w-full px-3 py-2 text-left hover:bg-blue-50 focus:bg-blue-50 focus:outline-none text-sm"
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
