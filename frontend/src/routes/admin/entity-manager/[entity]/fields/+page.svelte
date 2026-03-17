<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { get, post, put, del } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import FieldFormModal from '$lib/components/FieldFormModal.svelte';
	import type { EntityDef, FieldDef, FieldTypeInfo, FieldDefCreateInput } from '$lib/types/admin';

	let entityName = $derived($page.params.entity);
	let entity = $state<EntityDef | null>(null);
	let fields = $state<FieldDef[]>([]);
	let fieldTypes = $state<FieldTypeInfo[]>([]);
	let allEntities = $state<EntityDef[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// Search state
	let searchQuery = $state('');
	let filteredFields = $derived.by(() => {
		if (!searchQuery.trim()) return fields;
		const query = searchQuery.toLowerCase();
		return fields.filter(f =>
			f.label.toLowerCase().includes(query) ||
			f.name.toLowerCase().includes(query) ||
			f.type.toLowerCase().includes(query)
		);
	});

	// Modal state
	let showAddModal = $state(false);
	let showEditModal = $state(false);
	let editingField = $state<FieldDef | null>(null);
	let saving = $state(false);

	// Form state
	let newField = $state<FieldDefCreateInput>({
		name: '',
		label: '',
		type: 'varchar',
		isRequired: false,
		isReadOnly: false,
		isAudited: false
	});

	let enumOptions = $state<string[]>(['']);
	let apiNameManuallyEdited = $state(false);

	async function loadData() {
		try {
			loading = true;
			const [entityData, fieldsData, typesData, entitiesData] = await Promise.all([
				get<EntityDef>(`/admin/entities/${entityName}`),
				get<FieldDef[]>(`/admin/entities/${entityName}/fields`),
				get<FieldTypeInfo[]>('/admin/field-types'),
				get<EntityDef[]>('/admin/entities')
			]);
			entity = entityData;
			fields = fieldsData;
			fieldTypes = typesData;
			allEntities = entitiesData;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load data';
		} finally {
			loading = false;
		}
	}

	function openAddModal() {
		newField = {
			name: '',
			label: '',
			type: 'varchar',
			isRequired: false,
			isReadOnly: false,
			isAudited: false
		};
		enumOptions = [''];
		apiNameManuallyEdited = false;
		showAddModal = true;
	}

	function openEditModal(field: FieldDef) {
		editingField = { ...field };
		if (field.options) {
			try {
				enumOptions = JSON.parse(field.options);
			} catch {
				enumOptions = [''];
			}
		} else {
			enumOptions = [''];
		}
		showEditModal = true;
	}

	function closeModals() {
		showAddModal = false;
		showEditModal = false;
		editingField = null;
	}

	function generateFieldName(label: string): string {
		return label
			.toLowerCase()
			.replace(/[^a-z0-9]+/g, '_')
			.replace(/^_|_$/g, '')
			.replace(/_+/g, '_');
	}

	function handleLabelChange() {
		if (!apiNameManuallyEdited) {
			newField.name = generateFieldName(newField.label);
		}
	}

	function handleApiNameChange() {
		apiNameManuallyEdited = true;
	}

	async function createField() {
		if (!newField.name || !newField.label) {
			toast.error('Name and label are required');
			return;
		}

		saving = true;
		try {
			const input: FieldDefCreateInput = { ...newField };

			// Handle enum options
			if (newField.type === 'enum' || newField.type === 'multiEnum') {
				const validOptions = enumOptions.filter(o => o.trim());
				if (validOptions.length === 0) {
					toast.error('At least one option is required for dropdown fields');
					saving = false;
					return;
				}
				input.options = JSON.stringify(validOptions);
			}

			// Handle lookup fields
			if (newField.type === 'link' || newField.type === 'linkMultiple') {
				if (!newField.linkEntity) {
					toast.error('Related entity is required for lookup fields');
					saving = false;
					return;
				}
			}

			// Handle rollup fields
			if (newField.type === 'rollup') {
				if (!newField.rollupQuery) {
					toast.error('SQL query is required for rollup fields');
					saving = false;
					return;
				}
				if (!newField.rollupResultType) {
					toast.error('Result type is required for rollup fields');
					saving = false;
					return;
				}
			}

			await post(`/admin/entities/${entityName}/fields`, input);
			toast.success('Field created successfully');
			closeModals();
			await loadData();
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to create field');
		} finally {
			saving = false;
		}
	}

	async function updateField() {
		if (!editingField) return;

		saving = true;
		try {
			const input: Record<string, unknown> = {
				label: editingField.label,
				isRequired: editingField.isRequired,
				isReadOnly: editingField.isReadOnly,
				isAudited: editingField.isAudited,
				maxLength: editingField.maxLength,
				tooltip: editingField.tooltip
			};

			if (editingField.type === 'enum' || editingField.type === 'multiEnum') {
				const validOptions = enumOptions.filter(o => o.trim());
				input.options = JSON.stringify(validOptions);
				input.defaultValue = editingField.defaultValue || '';
			}

			if (editingField.type === 'rollup') {
				input.rollupResultType = editingField.rollupResultType;
				input.rollupQuery = editingField.rollupQuery;
				input.rollupDecimalPlaces = editingField.rollupDecimalPlaces;
			}

			// Handle date/datetime fields
			if (editingField.type === 'date' || editingField.type === 'datetime') {
				input.defaultToToday = editingField.defaultToToday;
			}

			await put(`/admin/entities/${entityName}/fields/${encodeURIComponent(editingField.name)}`, input);
			toast.success('Field updated successfully');
			closeModals();
			await loadData();
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to update field');
		} finally {
			saving = false;
		}
	}

	async function deleteField(field: FieldDef) {
		if (!confirm(`Are you sure you want to delete the field "${field.label}"?`)) {
			return;
		}

		try {
			await del(`/admin/entities/${entityName}/fields/${encodeURIComponent(field.name)}`);
			toast.success('Field deleted successfully');
			await loadData();
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to delete field');
		}
	}

	function getFieldTypeLabel(type: string): string {
		const typeInfo = fieldTypes.find(t => t.name === type);
		return typeInfo?.label || type;
	}

	onMount(() => {
		loadData();
	});
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<nav class="text-sm text-gray-500 mb-2">
				<a href="/admin" class="hover:text-gray-700">Administration</a>
				<span class="mx-2">/</span>
				<a href="/admin/entity-manager" class="hover:text-gray-700">Entity Manager</a>
				<span class="mx-2">/</span>
				<a href="/admin/entity-manager/{entityName}" class="hover:text-gray-700">{entity?.label || entityName}</a>
				<span class="mx-2">/</span>
				<span class="text-gray-900">Fields</span>
			</nav>
			<h1 class="text-2xl font-bold text-gray-900">{entity?.label || entityName} - Fields</h1>
		</div>
		<button
			onclick={openAddModal}
			class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-600/90"
		>
			+ Add Field
		</button>
	</div>

	{#if loading}
		<div class="text-center py-12 text-gray-500">Loading fields...</div>
	{:else if error}
		<div class="text-center py-12 text-red-500">{error}</div>
	{:else}
		<!-- Search Filter -->
		<div class="mb-4">
			<div class="relative">
				<input
					type="text"
					bind:value={searchQuery}
					placeholder="Search fields by name, label, or type..."
					class="w-full pl-10 pr-4 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
				/>
				<svg
					class="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-400"
					xmlns="http://www.w3.org/2000/svg"
					viewBox="0 0 20 20"
					fill="currentColor"
				>
					<path
						fill-rule="evenodd"
						d="M9 3.5a5.5 5.5 0 100 11 5.5 5.5 0 000-11zM2 9a7 7 0 1112.452 4.391l3.328 3.329a.75.75 0 11-1.06 1.06l-3.329-3.328A7 7 0 012 9z"
						clip-rule="evenodd"
					/>
				</svg>
				{#if searchQuery}
					<button
						onclick={() => searchQuery = ''}
						class="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600"
					>
						<svg class="h-5 w-5" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
							<path d="M6.28 5.22a.75.75 0 00-1.06 1.06L8.94 10l-3.72 3.72a.75.75 0 101.06 1.06L10 11.06l3.72 3.72a.75.75 0 101.06-1.06L11.06 10l3.72-3.72a.75.75 0 00-1.06-1.06L10 8.94 6.28 5.22z" />
						</svg>
					</button>
				{/if}
			</div>
			{#if searchQuery && filteredFields.length !== fields.length}
				<p class="mt-2 text-sm text-gray-500">
					Showing {filteredFields.length} of {fields.length} fields
				</p>
			{/if}
		</div>

		<div class="crm-card overflow-hidden">
			<table class="min-w-full divide-y divide-gray-200">
				<thead class="bg-gray-50">
					<tr>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Field
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Type
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Required
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Custom
						</th>
						<th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
							Actions
						</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-200">
					{#each filteredFields as field (field.id)}
						<tr class="hover:bg-gray-50">
							<td class="px-6 py-4 whitespace-nowrap">
								<div class="text-sm font-medium text-gray-900">{field.label}</div>
								<div class="text-sm text-gray-500">{field.name}</div>
							</td>
							<td class="px-6 py-4 whitespace-nowrap">
								<span class="px-2 py-1 text-xs bg-gray-100 text-gray-800 rounded">
									{getFieldTypeLabel(field.type)}
								</span>
							</td>
							<td class="px-6 py-4 whitespace-nowrap">
								{#if field.isRequired}
									<span class="text-green-600">Yes</span>
								{:else}
									<span class="text-gray-400">No</span>
								{/if}
							</td>
							<td class="px-6 py-4 whitespace-nowrap">
								{#if field.isCustom}
									<span class="px-2 py-1 text-xs bg-green-100 text-green-800 rounded-full">Custom</span>
								{:else}
									<span class="px-2 py-1 text-xs bg-blue-100 text-blue-800 rounded-full">System</span>
								{/if}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-right text-sm">
								{#if field.isCustom}
									<button
										onclick={() => openEditModal(field)}
										class="text-blue-600 hover:text-blue-800 mr-4"
									>
										Edit
									</button>
									<button
										onclick={() => deleteField(field)}
										class="text-red-600 hover:text-red-800"
									>
										Delete
									</button>
								{:else}
									<span class="text-gray-400 text-xs">System field</span>
								{/if}
							</td>
						</tr>
					{:else}
						<tr>
							<td colspan="5" class="px-6 py-8 text-center text-gray-500">
								{#if searchQuery}
									No fields match "{searchQuery}"
								{:else}
									No fields defined
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>

<!-- Add Field Modal -->
{#if showAddModal}
	<FieldFormModal
		mode="add"
		field={newField}
		{fieldTypes}
		{allEntities}
		{enumOptions}
		{saving}
		onSave={createField}
		onClose={closeModals}
		onFieldChange={(f) => newField = f as FieldDefCreateInput}
		onEnumOptionsChange={(opts) => enumOptions = opts}
		onLabelChange={handleLabelChange}
		onApiNameChange={handleApiNameChange}
	/>
{/if}

<!-- Edit Field Modal -->
{#if showEditModal && editingField}
	<FieldFormModal
		mode="edit"
		field={editingField}
		{fieldTypes}
		{allEntities}
		{enumOptions}
		{saving}
		onSave={updateField}
		onClose={closeModals}
		onFieldChange={(f) => editingField = f as FieldDef}
		onEnumOptionsChange={(opts) => enumOptions = opts}
	/>
{/if}
