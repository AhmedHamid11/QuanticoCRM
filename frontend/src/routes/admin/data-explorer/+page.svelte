<script lang="ts">
	import { post } from '$lib/utils/api';
	import { addToast } from '$lib/stores/toast.svelte';

	interface QueryResponse {
		columns: string[];
		rows: (string | number | boolean | null)[][];
		rowCount: number;
		warning?: string;
	}

	let sqlQuery = $state('SELECT * FROM contacts LIMIT 10');
	let loading = $state(false);
	let result = $state<QueryResponse | null>(null);
	let error = $state<string | null>(null);
	let warning = $state<string | null>(null);

	async function executeQuery() {
		if (!sqlQuery.trim()) {
			error = 'Please enter a SQL query';
			return;
		}

		loading = true;
		error = null;
		warning = null;
		result = null;

		try {
			const response = await post<QueryResponse>('/data-explorer/query', { sql: sqlQuery });
			result = response;
			warning = response.warning || null;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to execute query';
		} finally {
			loading = false;
		}
	}

	function clearQuery() {
		sqlQuery = '';
		result = null;
		error = null;
	}

	function exportCSV() {
		if (!result || result.rows.length === 0) {
			addToast('No data to export', 'info');
			return;
		}

		// Build CSV content
		const headers = result.columns.join(',');
		const rows = result.rows.map(row =>
			row.map(cell => {
				if (cell === null) return '';
				const str = String(cell);
				// Escape quotes and wrap in quotes if contains comma, quote, or newline
				if (str.includes(',') || str.includes('"') || str.includes('\n')) {
					return `"${str.replace(/"/g, '""')}"`;
				}
				return str;
			}).join(',')
		);
		const csv = [headers, ...rows].join('\n');

		// Download file
		const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
		const url = URL.createObjectURL(blob);
		const link = document.createElement('a');
		link.href = url;
		link.download = `query-export-${new Date().toISOString().slice(0, 10)}.csv`;
		document.body.appendChild(link);
		link.click();
		document.body.removeChild(link);
		URL.revokeObjectURL(url);

		addToast('CSV exported successfully', 'success');
	}

	function handleKeydown(e: KeyboardEvent) {
		// Ctrl/Cmd + Enter to execute
		if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
			e.preventDefault();
			executeQuery();
		}
	}
</script>

<div class="space-y-6">
	<!-- Breadcrumb -->
	<nav class="text-sm text-gray-500">
		<a href="/admin" class="hover:text-gray-700">Administration</a>
		<span class="mx-2">/</span>
		<span class="text-gray-900">Data Explorer</span>
	</nav>

	<!-- Header -->
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold text-gray-900">Data Explorer</h1>
			<p class="mt-1 text-sm text-gray-500">Execute SQL queries and explore your data</p>
		</div>
	</div>

	<!-- Query Input -->
	<div class="bg-white shadow rounded-lg p-6">
		<label for="sql-query" class="block text-sm font-medium text-gray-700 mb-2">
			SQL Query
		</label>
		<textarea
			id="sql-query"
			bind:value={sqlQuery}
			onkeydown={handleKeydown}
			rows="6"
			class="w-full font-mono text-sm border border-gray-300 rounded-lg p-3 focus:ring-2 focus:ring-teal-500 focus:border-teal-500"
			placeholder="SELECT * FROM table_name LIMIT 10"
		></textarea>
		<p class="mt-1 text-xs text-gray-400">Press Ctrl+Enter to execute</p>

		<!-- Actions -->
		<div class="mt-4 flex items-center justify-between">
			<div class="flex gap-3">
				<button
					onclick={executeQuery}
					disabled={loading}
					class="px-4 py-2 bg-teal-600 text-white rounded-lg hover:bg-teal-700 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
				>
					{#if loading}
						<svg class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
						</svg>
						Executing...
					{:else}
						Execute
					{/if}
				</button>
				<button
					onclick={clearQuery}
					class="px-4 py-2 border border-gray-300 text-gray-700 rounded-lg hover:bg-gray-50"
				>
					Clear
				</button>
				{#if result && result.rows.length > 0}
					<button
						onclick={exportCSV}
						class="px-4 py-2 border border-gray-300 text-gray-700 rounded-lg hover:bg-gray-50 flex items-center gap-2"
					>
						<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
						</svg>
						Export CSV
					</button>
				{/if}
			</div>
			{#if result}
				<span class="text-sm text-gray-500">
					{result.rowCount} row{result.rowCount !== 1 ? 's' : ''} returned
					{#if result.rowCount >= 1000}
						<span class="text-amber-600">(limited to 1000)</span>
					{/if}
				</span>
			{/if}
		</div>
	</div>

	<!-- Error Display -->
	{#if error}
		<div class="bg-red-50 border border-red-200 rounded-lg p-4">
			<div class="flex items-start">
				<svg class="h-5 w-5 text-red-500 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
				</svg>
				<div class="ml-3">
					<h3 class="text-sm font-medium text-red-800">Query Error</h3>
					<p class="mt-1 text-sm text-red-700 font-mono">{error}</p>
				</div>
			</div>
		</div>
	{/if}

	<!-- Warning Display -->
	{#if warning}
		<div class="bg-amber-50 border border-amber-200 rounded-lg p-4">
			<div class="flex items-start">
				<svg class="h-5 w-5 text-amber-500 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
				</svg>
				<div class="ml-3">
					<h3 class="text-sm font-medium text-amber-800">Note</h3>
					<p class="mt-1 text-sm text-amber-700">{warning}</p>
				</div>
			</div>
		</div>
	{/if}

	<!-- Results Table -->
	{#if result && result.rows && result.rows.length > 0}
		<div class="crm-card overflow-hidden">
			<div class="overflow-x-auto">
				<table class="min-w-full divide-y divide-gray-200">
					<thead class="bg-gray-50">
						<tr>
							{#each result.columns as column}
								<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider whitespace-nowrap">
									{column}
								</th>
							{/each}
						</tr>
					</thead>
					<tbody class="divide-y divide-gray-200">
						{#each result.rows as row, rowIndex}
							<tr class="hover:bg-gray-50" class:bg-gray-25={rowIndex % 2 === 1}>
								{#each row as cell}
									<td class="px-4 py-2 text-sm text-gray-900 whitespace-nowrap font-mono max-w-xs truncate" title={String(cell ?? '')}>
										{#if cell === null}
											<span class="text-gray-400 italic">null</span>
										{:else}
											{cell}
										{/if}
									</td>
								{/each}
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</div>
	{:else if result && (!result.rows || result.rows.length === 0)}
		<div class="bg-white shadow rounded-lg p-8 text-center">
			<svg class="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
			</svg>
			<h3 class="mt-2 text-sm font-medium text-gray-900">No results</h3>
			<p class="mt-1 text-sm text-gray-500">The query executed successfully but returned no rows.</p>
		</div>
	{/if}
</div>
