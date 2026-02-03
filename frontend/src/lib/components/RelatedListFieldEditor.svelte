<script lang="ts">
	import { get } from '$lib/utils/api';
	import type { FieldDef } from '$lib/types/admin';
	import type { FieldConfig, RelatedListConfigCreateInput } from '$lib/types/related-list';

	interface Props {
		config: RelatedListConfigCreateInput;
		onSave: (config: RelatedListConfigCreateInput) => void;
		onClose: () => void;
	}

	let { config, onSave, onClose }: Props = $props();

	let availableFields = $state<FieldDef[]>([]);
	let selectedFields = $state<FieldConfig[]>([...config.displayFields]);
	let loading = $state(true);
	let defaultSort = $state(config.defaultSort || 'createdAt');
	let defaultSortDir = $state<'asc' | 'desc'>(config.defaultSortDir || 'desc');
	let pageSize = $state(config.pageSize || 5);

	let unselectedFields = $derived(
		availableFields.filter((f) => !selectedFields.some((sf) => sf.field === f.name))
	);

	async function loadFields() {
		try {
			loading = true;
			availableFields = await get<FieldDef[]>(`/admin/entities/${config.relatedEntity}/fields`);
		} catch (e) {
			console.error('Failed to load fields:', e);
		} finally {
			loading = false;
		}
	}

	function addField(field: FieldDef) {
		selectedFields = [
			...selectedFields,
			{
				field: field.name,
				label: field.label,
				position: selectedFields.length
			}
		];
	}

	function removeField(index: number) {
		selectedFields = selectedFields.filter((_, i) => i !== index);
		// Update positions
		selectedFields = selectedFields.map((f, i) => ({ ...f, position: i }));
	}

	function moveUp(index: number) {
		if (index <= 0) return;
		const newFields = [...selectedFields];
		[newFields[index - 1], newFields[index]] = [newFields[index], newFields[index - 1]];
		selectedFields = newFields.map((f, i) => ({ ...f, position: i }));
	}

	function moveDown(index: number) {
		if (index >= selectedFields.length - 1) return;
		const newFields = [...selectedFields];
		[newFields[index], newFields[index + 1]] = [newFields[index + 1], newFields[index]];
		selectedFields = newFields.map((f, i) => ({ ...f, position: i }));
	}

	function handleSave() {
		const updatedConfig: RelatedListConfigCreateInput = {
			...config,
			displayFields: selectedFields,
			defaultSort,
			defaultSortDir,
			pageSize
		};
		onSave(updatedConfig);
	}

	function getFieldLabel(fieldName: string): string {
		const field = availableFields.find((f) => f.name === fieldName);
		return field?.label || fieldName;
	}

	// Load fields on mount
	$effect(() => {
		loadFields();
	});
</script>

<!-- Modal backdrop -->
<div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
	<div class="bg-white rounded-lg shadow-xl w-full max-w-3xl max-h-[90vh] overflow-hidden">
		<!-- Header -->
		<div class="px-6 py-4 border-b border-gray-200 flex items-center justify-between">
			<h3 class="text-lg font-medium text-gray-900">
				Configure "{config.label}" Columns
			</h3>
			<button onclick={onClose} class="text-gray-400 hover:text-gray-600">
				<svg class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
				</svg>
			</button>
		</div>

		<!-- Content -->
		<div class="p-6 overflow-y-auto max-h-[60vh]">
			{#if loading}
				<div class="text-center py-8 text-gray-500">Loading fields...</div>
			{:else}
				<div class="grid grid-cols-2 gap-6">
					<!-- Available Fields -->
					<div>
						<h4 class="text-sm font-medium text-gray-700 mb-3">Available Fields</h4>
						{#if unselectedFields.length === 0}
							<p class="text-sm text-gray-500 py-4 text-center bg-gray-50 rounded">
								All fields are selected
							</p>
						{:else}
							<div class="space-y-2 max-h-64 overflow-y-auto">
								{#each unselectedFields as field (field.id)}
									<button
										onclick={() => addField(field)}
										class="w-full flex items-center justify-between p-2 text-left bg-gray-50 rounded border border-gray-200 hover:bg-gray-100"
									>
										<span class="text-sm text-gray-900">{field.label}</span>
										<svg class="w-5 h-5 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
											<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
										</svg>
									</button>
								{/each}
							</div>
						{/if}
					</div>

					<!-- Selected Fields -->
					<div>
						<h4 class="text-sm font-medium text-gray-700 mb-3">
							Display Columns
							<span class="text-gray-400 font-normal">(drag to reorder)</span>
						</h4>
						{#if selectedFields.length === 0}
							<p class="text-sm text-gray-500 py-4 text-center bg-gray-50 rounded">
								Add fields from the available list
							</p>
						{:else}
							<div class="space-y-2 max-h-64 overflow-y-auto">
								{#each selectedFields as fieldConfig, index (fieldConfig.field)}
									<div class="flex items-center gap-2 p-2 bg-blue-50 rounded border border-blue-200">
										<div class="flex flex-col gap-0.5">
											<button
												onclick={() => moveUp(index)}
												disabled={index === 0}
												class="p-0.5 text-gray-400 hover:text-gray-600 disabled:opacity-30"
											>
												<svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
													<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 15l7-7 7 7" />
												</svg>
											</button>
											<button
												onclick={() => moveDown(index)}
												disabled={index === selectedFields.length - 1}
												class="p-0.5 text-gray-400 hover:text-gray-600 disabled:opacity-30"
											>
												<svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
													<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
												</svg>
											</button>
										</div>
										<span class="flex-1 text-sm text-gray-900">
											{fieldConfig.label || getFieldLabel(fieldConfig.field)}
										</span>
										<input
											type="number"
											bind:value={fieldConfig.width}
											placeholder="Auto"
											class="w-16 px-2 py-1 text-xs border rounded"
											title="Column width %"
										/>
										<button
											onclick={() => removeField(index)}
											class="p-1 text-red-500 hover:text-red-700"
										>
											<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
												<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
											</svg>
										</button>
									</div>
								{/each}
							</div>
						{/if}
					</div>
				</div>

				<!-- Settings -->
				<div class="mt-6 pt-6 border-t border-gray-200">
					<h4 class="text-sm font-medium text-gray-700 mb-3">Settings</h4>
					<div class="grid grid-cols-3 gap-4">
						<div>
							<label class="block text-xs text-gray-500 mb-1">Default Sort</label>
							<select
								bind:value={defaultSort}
								class="w-full px-3 py-2 border rounded-md text-sm"
							>
								{#each selectedFields as field}
									<option value={field.field}>{field.label || getFieldLabel(field.field)}</option>
								{/each}
								<option value="createdAt">Created Date</option>
								<option value="modifiedAt">Modified Date</option>
							</select>
						</div>
						<div>
							<label class="block text-xs text-gray-500 mb-1">Sort Direction</label>
							<select
								bind:value={defaultSortDir}
								class="w-full px-3 py-2 border rounded-md text-sm"
							>
								<option value="asc">Ascending</option>
								<option value="desc">Descending</option>
							</select>
						</div>
						<div>
							<label class="block text-xs text-gray-500 mb-1">Page Size</label>
							<select
								bind:value={pageSize}
								class="w-full px-3 py-2 border rounded-md text-sm"
							>
								<option value={5}>5 records</option>
								<option value={10}>10 records</option>
								<option value={15}>15 records</option>
								<option value={20}>20 records</option>
							</select>
						</div>
					</div>
				</div>
			{/if}
		</div>

		<!-- Footer -->
		<div class="px-6 py-4 border-t border-gray-200 flex justify-end gap-3">
			<button
				onclick={onClose}
				class="px-4 py-2 text-sm text-gray-700 border rounded-md hover:bg-gray-50"
			>
				Cancel
			</button>
			<button
				onclick={handleSave}
				class="px-4 py-2 text-sm bg-blue-600 text-white rounded-md hover:bg-blue-600/90"
			>
				Save
			</button>
		</div>
	</div>
</div>
