<script lang="ts">
	interface Props {
		label: string;
		entry: string;
		log: string;
		required?: boolean;
		onchange?: (entry: string) => void;
		onsubmit?: (entry: string) => Promise<void>;
		ondelete?: (entryIndex: number) => Promise<void>;
		readonly?: boolean;
	}

	let { label, entry, log, required = false, onchange, onsubmit, ondelete, readonly = false }: Props = $props();

	// Local entry state for inline editing mode (when onsubmit is provided)
	let localEntry = $state('');
	let isSubmitting = $state(false);
	let submitError = $state<string | null>(null);
	let deletingIndex = $state<number | null>(null);

	// Parse log entries (newest first - entries separated by newlines)
	let logEntries = $derived(() => {
		if (!log) return [];
		return log.split('\n').filter(line => line.trim());
	});

	// Parse a log entry into timestamp and content
	function parseEntry(entry: string): { timestamp: string | null; content: string } {
		const match = entry.match(/^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}) - (.*)$/s);
		if (match) {
			return { timestamp: match[1], content: match[2] };
		}
		return { timestamp: null, content: entry };
	}

	function handleInput(e: Event) {
		const target = e.target as HTMLTextAreaElement;
		if (onchange) {
			onchange(target.value);
		} else {
			localEntry = target.value;
		}
	}

	async function handleSubmit() {
		if (!onsubmit) return;

		const entryValue = onchange ? entry : localEntry;
		if (!entryValue.trim()) return;

		isSubmitting = true;
		submitError = null;

		try {
			await onsubmit(entryValue);
			// Clear the local entry on success
			localEntry = '';
		} catch (e) {
			submitError = e instanceof Error ? e.message : 'Failed to add entry';
		} finally {
			isSubmitting = false;
		}
	}

	async function handleDelete(index: number) {
		if (!ondelete) return;
		if (!confirm('Delete this entry? This cannot be undone.')) return;

		deletingIndex = index;
		try {
			await ondelete(index);
		} catch (e) {
			// Error is handled by parent, just reset state
		} finally {
			deletingIndex = null;
		}
	}

	function handleKeydown(e: KeyboardEvent) {
		// Submit on Ctrl/Cmd + Enter
		if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
			e.preventDefault();
			handleSubmit();
		}
	}

	// Determine which entry value to use
	let displayEntry = $derived(onchange ? entry : localEntry);

	// Show input if not readonly, or if onsubmit is provided (inline mode)
	let showInput = $derived(!readonly || !!onsubmit);
</script>

<div class="space-y-3">
	<label class="block text-sm font-medium text-gray-700">
		{label}
		{#if required}
			<span class="text-red-500">*</span>
		{/if}
	</label>

	{#if showInput}
		<!-- Entry input -->
		<div>
			<div class="relative">
				<textarea
					value={displayEntry}
					oninput={handleInput}
					onkeydown={handleKeydown}
					placeholder="Add a new entry..."
					rows="2"
					disabled={isSubmitting}
					class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary focus:border-primary text-sm disabled:bg-gray-50 disabled:text-gray-500"
				></textarea>
			</div>
			{#if submitError}
				<p class="mt-1 text-xs text-red-600">{submitError}</p>
			{/if}
			{#if onsubmit}
				<!-- Inline submission mode -->
				<div class="mt-2 flex items-center justify-between">
					<p class="text-xs text-gray-500">
						Press Ctrl+Enter to submit
					</p>
					<button
						type="button"
						onclick={handleSubmit}
						disabled={isSubmitting || !displayEntry.trim()}
						class="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium text-black bg-primary rounded-md hover:bg-primary/90 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
					>
						{#if isSubmitting}
							<svg class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
								<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
								<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
							</svg>
							Adding...
						{:else}
							<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
							</svg>
							Add Entry
						{/if}
					</button>
				</div>
			{:else}
				<p class="mt-1 text-xs text-gray-500">
					Press Save to add this entry to the log with a timestamp
				</p>
			{/if}
		</div>
	{/if}

	<!-- Log display -->
	{#if logEntries().length > 0}
		<div class="border border-gray-200 rounded-lg overflow-hidden">
			<div class="bg-gray-50 px-3 py-2 border-b border-gray-200">
				<span class="text-xs font-medium text-gray-500 uppercase">History ({logEntries().length} entries)</span>
			</div>
			<div class="max-h-64 overflow-y-auto">
				{#each logEntries() as logEntry, i}
					{@const parsed = parseEntry(logEntry)}
					{@const isDeleting = deletingIndex === i}
					<div class="group relative px-3 py-2 text-sm {i > 0 ? 'border-t border-gray-100' : ''} {isDeleting ? 'opacity-50' : ''}">
						{#if parsed.timestamp}
							<span class="text-gray-400 text-xs">{parsed.timestamp}</span>
							<p class="text-gray-900 mt-0.5 whitespace-pre-wrap pr-8">{parsed.content}</p>
						{:else}
							<p class="text-gray-900 whitespace-pre-wrap pr-8">{parsed.content}</p>
						{/if}

						{#if ondelete}
							<button
								type="button"
								onclick={() => handleDelete(i)}
								disabled={isDeleting || deletingIndex !== null}
								class="absolute top-2 right-2 p-1 text-gray-400 hover:text-red-600 opacity-0 group-hover:opacity-100 focus:opacity-100 transition-opacity disabled:opacity-50"
								title="Delete entry"
							>
								{#if isDeleting}
									<svg class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
										<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
										<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
									</svg>
								{:else}
									<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
									</svg>
								{/if}
							</button>
						{/if}
					</div>
				{/each}
			</div>
		</div>
	{:else if readonly && !onsubmit}
		<p class="text-sm text-gray-500 italic">No entries yet</p>
	{/if}
</div>
