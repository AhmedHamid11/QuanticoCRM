<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { get, put } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import RelatedListFieldEditor from '$lib/components/RelatedListFieldEditor.svelte';
	import type { EntityDef, FieldDef } from '$lib/types/admin';
	import type {
		RelatedListConfig,
		PossibleRelatedList,
		RelatedListConfigCreateInput,
		RelatedListConfigBulkInput
	} from '$lib/types/related-list';

	let entityName = $derived($page.params.entity);

	let entity = $state<EntityDef | null>(null);
	let availableOptions = $state<PossibleRelatedList[]>([]);
	let configuredLists = $state<RelatedListConfigCreateInput[]>([]);
	let loading = $state(true);
	let saving = $state(false);
	let error = $state<string | null>(null);

	// Modal state
	let editingConfig = $state<RelatedListConfigCreateInput | null>(null);

	// Derived: enabled and disabled lists
	let enabledLists = $derived(configuredLists.filter((c) => c.enabled));
	let availableButDisabled = $derived(
		availableOptions.filter(
			(opt) =>
				!configuredLists.some(
					(c) => c.relatedEntity === opt.relatedEntity && c.lookupField === opt.lookupField && c.enabled
				)
		)
	);

	async function loadData() {
		try {
			loading = true;
			error = null;

			const [entityData, optionsData, configsData] = await Promise.all([
				get<EntityDef>(`/admin/entities/${entityName}`),
				get<PossibleRelatedList[]>(`/entities/${entityName}/related-list-options`),
				get<RelatedListConfig[]>(`/entities/${entityName}/related-list-configs`)
			]);

			entity = entityData;
			availableOptions = optionsData;

			// Convert configs to input format
			configuredLists = configsData.map((c) => ({
				relatedEntity: c.relatedEntity,
				lookupField: c.lookupField,
				label: c.label,
				enabled: c.enabled,
				editInList: c.editInList || false,
				displayFields: c.displayFields || [],
				sortOrder: c.sortOrder,
				defaultSort: c.defaultSort,
				defaultSortDir: c.defaultSortDir,
				pageSize: c.pageSize
			}));
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load data';
		} finally {
			loading = false;
		}
	}

	async function saveConfigs() {
		try {
			saving = true;
			const input: RelatedListConfigBulkInput = { configs: configuredLists };
			await put(`/entities/${entityName}/related-list-configs`, input);
			toast.success('Related lists saved successfully');
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to save';
			toast.error(message);
		} finally {
			saving = false;
		}
	}

	function toggleRelatedList(option: PossibleRelatedList) {
		const existingIndex = configuredLists.findIndex(
			(c) => c.relatedEntity === option.relatedEntity && c.lookupField === option.lookupField
		);

		if (existingIndex >= 0) {
			// Toggle enabled state
			configuredLists[existingIndex].enabled = !configuredLists[existingIndex].enabled;
			configuredLists = [...configuredLists];
		} else {
			// Add new config
			configuredLists = [
				...configuredLists,
				{
					relatedEntity: option.relatedEntity,
					lookupField: option.lookupField,
					label: option.suggestedLabel,
					enabled: true,
					displayFields: [],
					sortOrder: configuredLists.length,
					defaultSort: 'createdAt',
					defaultSortDir: 'desc',
					pageSize: 5
				}
			];
		}
	}

	function removeRelatedList(config: RelatedListConfigCreateInput) {
		config.enabled = false;
		configuredLists = [...configuredLists];
	}

	function moveUp(index: number) {
		if (index <= 0) return;
		const enabled = configuredLists.filter((c) => c.enabled);
		const config = enabled[index];
		const prevConfig = enabled[index - 1];

		// Swap sort orders
		const tempOrder = config.sortOrder;
		config.sortOrder = prevConfig.sortOrder;
		prevConfig.sortOrder = tempOrder;

		configuredLists = [...configuredLists].sort((a, b) => (a.sortOrder || 0) - (b.sortOrder || 0));
	}

	function moveDown(index: number) {
		const enabled = configuredLists.filter((c) => c.enabled);
		if (index >= enabled.length - 1) return;

		const config = enabled[index];
		const nextConfig = enabled[index + 1];

		// Swap sort orders
		const tempOrder = config.sortOrder;
		config.sortOrder = nextConfig.sortOrder;
		nextConfig.sortOrder = tempOrder;

		configuredLists = [...configuredLists].sort((a, b) => (a.sortOrder || 0) - (b.sortOrder || 0));
	}

	function openFieldEditor(config: RelatedListConfigCreateInput) {
		editingConfig = config;
	}

	function toggleEditInList(config: RelatedListConfigCreateInput) {
		config.editInList = !config.editInList;
		configuredLists = [...configuredLists];
	}

	function handleFieldEditorSave(updatedConfig: RelatedListConfigCreateInput) {
		const index = configuredLists.findIndex(
			(c) => c.relatedEntity === updatedConfig.relatedEntity && c.lookupField === updatedConfig.lookupField
		);
		if (index >= 0) {
			configuredLists[index] = updatedConfig;
			configuredLists = [...configuredLists];
		}
		editingConfig = null;
	}

	function handleFieldEditorClose() {
		editingConfig = null;
	}

	onMount(() => {
		loadData();
	});
</script>

<div class="space-y-6">
	<!-- Breadcrumb -->
	<div>
		<nav class="text-sm text-gray-500 mb-2">
			<a href="/admin" class="hover:text-gray-700">Administration</a>
			<span class="mx-2">/</span>
			<a href="/admin/entity-manager" class="hover:text-gray-700">Entity Manager</a>
			<span class="mx-2">/</span>
			<a href="/admin/entity-manager/{entityName}" class="hover:text-gray-700">{entity?.label || entityName}</a>
			<span class="mx-2">/</span>
			<span class="text-gray-900">Related Lists</span>
		</nav>
		<div class="flex items-center justify-between">
			<div>
				<h1 class="text-2xl font-bold text-gray-900">
					{entity?.label || entityName} - Related Lists
				</h1>
				<p class="text-sm text-gray-500 mt-1">
					Configure which related lists appear on the detail page
				</p>
			</div>
			<button
				onclick={saveConfigs}
				disabled={saving}
				class="px-4 py-2 bg-primary text-black rounded-md hover:bg-primary/90 disabled:opacity-50"
			>
				{saving ? 'Saving...' : 'Save Changes'}
			</button>
		</div>
	</div>

	{#if loading}
		<div class="text-center py-12 text-gray-500">Loading...</div>
	{:else if error}
		<div class="text-center py-12 text-red-500">{error}</div>
	{:else}
		<!-- Enabled Related Lists -->
		<div class="bg-white shadow rounded-lg p-6">
			<h2 class="text-lg font-medium text-gray-900 mb-4">
				Enabled Related Lists
				<span class="text-sm font-normal text-gray-500">({enabledLists.length})</span>
			</h2>
			<p class="text-sm text-gray-500 mb-4">
				These related lists will appear on the {entity?.label || entityName} detail page. Drag to reorder.
			</p>

			{#if enabledLists.length === 0}
				<p class="text-gray-500 text-sm py-8 text-center bg-gray-50 rounded">
					No related lists enabled. Add from the available list below.
				</p>
			{:else}
				<div class="space-y-2">
					{#each enabledLists as config, index (config.relatedEntity + config.lookupField)}
						<div class="flex items-center gap-3 p-4 bg-gray-50 rounded-lg border border-gray-200">
							<!-- Reorder buttons -->
							<div class="flex flex-col gap-1">
								<button
									onclick={() => moveUp(index)}
									disabled={index === 0}
									class="p-1 text-gray-400 hover:text-gray-600 disabled:opacity-30 disabled:cursor-not-allowed"
									title="Move up"
								>
									<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 15l7-7 7 7" />
									</svg>
								</button>
								<button
									onclick={() => moveDown(index)}
									disabled={index === enabledLists.length - 1}
									class="p-1 text-gray-400 hover:text-gray-600 disabled:opacity-30 disabled:cursor-not-allowed"
									title="Move down"
								>
									<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
									</svg>
								</button>
							</div>

							<!-- Label input -->
							<div class="flex-1">
								<input
									type="text"
									bind:value={config.label}
									class="px-3 py-2 border rounded-md w-full max-w-xs font-medium"
									placeholder="Label"
								/>
								<div class="text-xs text-gray-500 mt-1">
									{config.relatedEntity} via {config.lookupField}
								</div>
							</div>

							<!-- Edit in List toggle -->
							<label
								class="flex items-center gap-2 cursor-pointer"
								title="When enabled, clicking 'New' creates an inline editable row instead of navigating to a new page"
							>
								<button
									type="button"
									onclick={() => toggleEditInList(config)}
									class="relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary focus:ring-offset-2 {config.editInList ? 'bg-primary' : 'bg-gray-200'}"
									role="switch"
									aria-checked={config.editInList}
								>
									<span
										class="pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out {config.editInList ? 'translate-x-4' : 'translate-x-0'}"
									></span>
								</button>
								<span class="text-sm text-gray-600">Inline</span>
							</label>

							<!-- Fields button -->
							<button
								onclick={() => openFieldEditor(config)}
								class="px-3 py-2 text-sm border rounded-md hover:bg-gray-100"
							>
								{config.displayFields?.length || 0} fields
							</button>

							<!-- Remove button -->
							<button
								onclick={() => removeRelatedList(config)}
								class="p-2 text-red-500 hover:text-red-700 hover:bg-red-50 rounded"
								title="Remove"
							>
								<svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
								</svg>
							</button>
						</div>
					{/each}
				</div>
			{/if}
		</div>

		<!-- Available Related Lists -->
		{#if availableButDisabled.length > 0}
			<div class="bg-white shadow rounded-lg p-6">
				<h2 class="text-lg font-medium text-gray-900 mb-4">
					Available Related Lists
					<span class="text-sm font-normal text-gray-500">({availableButDisabled.length})</span>
				</h2>
				<p class="text-sm text-gray-500 mb-4">
					These entities have lookup fields pointing to {entity?.label || entityName}. Click to add.
				</p>

				<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
					{#each availableButDisabled as option (option.relatedEntity + option.lookupField)}
						<button
							onclick={() => toggleRelatedList(option)}
							class="flex items-center gap-3 p-4 text-left bg-gray-50 rounded-lg border border-gray-200 hover:bg-gray-100 hover:border-gray-300"
						>
							<svg class="w-5 h-5 text-green-500 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
							</svg>
							<div>
								<div class="font-medium text-gray-900">{option.suggestedLabel}</div>
								<div class="text-xs text-gray-500">via {option.fieldLabel} ({option.lookupField})</div>
							</div>
						</button>
					{/each}
				</div>
			</div>
		{/if}

		<!-- No relationships found -->
		{#if availableOptions.length === 0}
			<div class="bg-white shadow rounded-lg p-6">
				<div class="text-center py-8">
					<svg class="w-12 h-12 mx-auto text-gray-400 mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
					</svg>
					<h3 class="text-lg font-medium text-gray-900 mb-2">No Related Lists Available</h3>
					<p class="text-sm text-gray-500">
						No other entities have lookup fields pointing to {entity?.label || entityName}.<br />
						Create a lookup field on another entity to enable related lists.
					</p>
				</div>
			</div>
		{/if}
	{/if}
</div>

<!-- Field Editor Modal -->
{#if editingConfig}
	<RelatedListFieldEditor
		config={editingConfig}
		onSave={handleFieldEditorSave}
		onClose={handleFieldEditorClose}
	/>
{/if}
