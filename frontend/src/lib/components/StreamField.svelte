<script lang="ts">
	interface Props {
		label: string;
		entry: string;
		log: string;
		required?: boolean;
		onchange: (entry: string) => void;
		readonly?: boolean;
	}

	let { label, entry, log, required = false, onchange, readonly = false }: Props = $props();

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
		onchange(target.value);
	}
</script>

<div class="space-y-3">
	<label class="block text-sm font-medium text-gray-700">
		{label}
		{#if required}
			<span class="text-red-500">*</span>
		{/if}
	</label>

	{#if !readonly}
		<!-- Entry input -->
		<div>
			<textarea
				value={entry}
				oninput={handleInput}
				placeholder="Add a new entry..."
				rows="2"
				class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500 text-sm"
			></textarea>
			<p class="mt-1 text-xs text-gray-500">
				Press Save to add this entry to the log with a timestamp
			</p>
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
					<div class="px-3 py-2 text-sm {i > 0 ? 'border-t border-gray-100' : ''}">
						{#if parsed.timestamp}
							<span class="text-gray-400 text-xs">{parsed.timestamp}</span>
							<p class="text-gray-900 mt-0.5 whitespace-pre-wrap">{parsed.content}</p>
						{:else}
							<p class="text-gray-900 whitespace-pre-wrap">{parsed.content}</p>
						{/if}
					</div>
				{/each}
			</div>
		</div>
	{:else if readonly}
		<p class="text-sm text-gray-500 italic">No entries yet</p>
	{/if}
</div>
