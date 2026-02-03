<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { get, put } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import type { EntityDef, FieldDef } from '$lib/types/admin';
	import type { LayoutDataV2, LayoutSectionV2, LayoutV2Response } from '$lib/types/layout';
	import { createDefaultSection, createDefaultVisibility, convertV1ToV2, getAllFieldNames } from '$lib/types/layout';
	import SectionEditor from '$lib/components/SectionEditor.svelte';

	let entityName = $derived($page.params.entity);
	let layoutType = $derived($page.params.type);
	let isListLayout = $derived(layoutType === 'list');

	let entity = $state<EntityDef | null>(null);
	let fields = $state<FieldDef[]>([]);
	let loading = $state(true);
	let saving = $state(false);
	let error = $state<string | null>(null);

	// V2 Layout data (for detail layouts)
	let layout = $state<LayoutDataV2 | null>(null);

	// V1 Layout data (for list layouts - simple array of field names)
	let listColumns = $state<string[]>([]);

	// UI state
	let collapsedSections = $state<Set<string>>(new Set());

	// List layout drag state
	let draggedColumnIndex = $state<number | null>(null);
	let dragOverColumnIndex = $state<number | null>(null);

	// Track which fields are used across all sections
	let usedFieldNames = $derived(() => {
		if (!layout) return new Set<string>();
		const used = new Set<string>();
		for (const section of layout.sections) {
			for (const field of section.fields) {
				used.add(field.name);
			}
		}
		return used;
	});

	// Drag state for section reordering
	let draggedSectionIndex = $state<number | null>(null);
	let dragOverSectionIndex = $state<number | null>(null);

	const layoutTypeInfo: Record<string, { name: string; description: string }> = {
		list: { name: 'List', description: 'Configure which columns appear in the list/table view' },
		detail: { name: 'Detail', description: 'Configure field sections for the record detail page' },
		detailSmall: { name: 'Quick View', description: 'Configure the compact view shown in modals and popovers' },
		filters: { name: 'Filters', description: 'Configure which fields are available in the search/filter panel' },
		massUpdate: { name: 'Mass Update', description: 'Configure which fields are available for bulk editing' }
	};

	function generateSectionId(): string {
		return 'section_' + Math.random().toString(36).substr(2, 9);
	}

	async function loadData() {
		try {
			loading = true;
			error = null;

			const [entityData, fieldsData] = await Promise.all([
				get<EntityDef>(`/admin/entities/${entityName}`),
				get<FieldDef[]>(`/admin/entities/${entityName}/fields`)
			]);

			entity = entityData;
			fields = fieldsData;

			if (isListLayout) {
				// List layouts use V1 format (simple array)
				try {
					const v1Response = await get<{ layoutData: string }>(
						`/admin/entities/${entityName}/layouts/${layoutType}`
					);
					const parsed = JSON.parse(v1Response.layoutData || '[]');
					// Handle both V1 (array) and V2 (object with sections) formats
					if (Array.isArray(parsed)) {
						listColumns = parsed;
					} else if (parsed && typeof parsed === 'object' && 'sections' in parsed) {
						// Convert V2 to V1 by extracting field names
						listColumns = getAllFieldNames(parsed);
					} else {
						listColumns = [];
					}
				} catch {
					// Default to first 5 non-system fields
					listColumns = fields
						.filter(f => f.name !== 'id' && !f.name.startsWith('created_') && !f.name.startsWith('modified_'))
						.slice(0, 5)
						.map(f => f.name);
				}
			} else {
				// Detail layouts use V2 format (sections)
				try {
					const layoutResponse = await get<LayoutV2Response>(
						`/admin/entities/${entityName}/layouts/${layoutType}/v2`
					);
					layout = layoutResponse.layout;
				} catch {
					// If v2 endpoint fails, try v1 and convert
					try {
						const v1Response = await get<{ layoutData: string }>(
							`/admin/entities/${entityName}/layouts/${layoutType}`
						);
						const v1Fields = JSON.parse(v1Response.layoutData || '[]');
						layout = convertV1ToV2(Array.isArray(v1Fields) ? v1Fields : []);
					} catch {
						// Default empty layout
						layout = {
							version: 2,
							sections: [createDefaultSection(generateSectionId(), 'General Information', 1)]
						};
					}
				}
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load data';
		} finally {
			loading = false;
		}
	}

	async function saveLayout() {
		try {
			saving = true;

			if (isListLayout) {
				// List layouts save as V1 format (simple array)
				await put(`/admin/entities/${entityName}/layouts/${layoutType}`, {
					layoutData: JSON.stringify(listColumns)
				});
			} else {
				// Detail layouts save as V2 format (sections)
				if (!layout) return;
				await put(`/admin/entities/${entityName}/layouts/${layoutType}/v2`, layout);
			}

			toast.success('Layout saved successfully');
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to save layout';
			toast.error(message);
		} finally {
			saving = false;
		}
	}

	function addSection() {
		if (!layout) {
			console.error('Cannot add section: layout is null');
			return;
		}

		const maxOrder = Math.max(0, ...layout.sections.map((s) => s.order));
		const newSection = createDefaultSection(
			generateSectionId(),
			'New Section',
			maxOrder + 1
		);

		// Create new layout object to trigger reactivity
		const newSections = [...layout.sections, newSection];
		layout = {
			version: layout.version,
			sections: newSections
		};

		console.log('Added section, total sections:', layout.sections.length);
	}

	function updateSection(index: number, section: LayoutSectionV2) {
		if (!layout) return;

		const sections = [...layout.sections];
		sections[index] = section;
		layout = { ...layout, sections };
	}

	function deleteSection(index: number) {
		if (!layout || layout.sections.length <= 1) {
			toast.error('Cannot delete the last section');
			return;
		}

		if (!confirm('Delete this section and all its fields?')) return;

		const sections = layout.sections.filter((_, i) => i !== index);
		layout = { ...layout, sections };
	}

	function toggleSectionCollapse(sectionId: string) {
		const newCollapsed = new Set(collapsedSections);
		if (newCollapsed.has(sectionId)) {
			newCollapsed.delete(sectionId);
		} else {
			newCollapsed.add(sectionId);
		}
		collapsedSections = newCollapsed;
	}

	// Section drag handlers
	function handleSectionDragStart(index: number) {
		draggedSectionIndex = index;
	}

	function handleSectionDragOver(e: DragEvent, index: number) {
		e.preventDefault();
		dragOverSectionIndex = index;
	}

	function handleSectionDragLeave() {
		dragOverSectionIndex = null;
	}

	function handleSectionDrop(e: DragEvent, dropIndex: number) {
		e.preventDefault();
		if (!layout || draggedSectionIndex === null || draggedSectionIndex === dropIndex) {
			draggedSectionIndex = null;
			dragOverSectionIndex = null;
			return;
		}

		const sections = [...layout.sections];
		const [draggedSection] = sections.splice(draggedSectionIndex, 1);
		sections.splice(dropIndex, 0, draggedSection);

		// Update order numbers
		sections.forEach((s, i) => {
			s.order = i + 1;
		});

		layout = { ...layout, sections };

		draggedSectionIndex = null;
		dragOverSectionIndex = null;
	}

	function handleSectionDragEnd() {
		draggedSectionIndex = null;
		dragOverSectionIndex = null;
	}

	// === List Layout Functions ===

	// Fields available to add to list columns
	let availableListFields = $derived(
		fields.filter(f => !listColumns.includes(f.name))
	);

	function addListColumn(fieldName: string) {
		listColumns = [...listColumns, fieldName];
	}

	function removeListColumn(index: number) {
		listColumns = listColumns.filter((_, i) => i !== index);
	}

	function handleColumnDragStart(index: number) {
		draggedColumnIndex = index;
	}

	function handleColumnDragOver(e: DragEvent, index: number) {
		e.preventDefault();
		dragOverColumnIndex = index;
	}

	function handleColumnDragLeave() {
		dragOverColumnIndex = null;
	}

	function handleColumnDrop(e: DragEvent, dropIndex: number) {
		e.preventDefault();
		if (draggedColumnIndex === null || draggedColumnIndex === dropIndex) {
			draggedColumnIndex = null;
			dragOverColumnIndex = null;
			return;
		}

		const newColumns = [...listColumns];
		const [draggedColumn] = newColumns.splice(draggedColumnIndex, 1);
		newColumns.splice(dropIndex, 0, draggedColumn);
		listColumns = newColumns;

		draggedColumnIndex = null;
		dragOverColumnIndex = null;
	}

	function handleColumnDragEnd() {
		draggedColumnIndex = null;
		dragOverColumnIndex = null;
	}

	// Get field label by name
	function getFieldLabel(fieldName: string): string {
		return fields.find(f => f.name === fieldName)?.label || fieldName;
	}

	// Get all fields not yet assigned to any section (for V2 layouts)
	let unassignedFields = $derived(
		fields.filter((f) => !usedFieldNames().has(f.name))
	);

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
			<a href="/admin/entity-manager/{entityName}" class="hover:text-gray-700"
				>{entity?.label || entityName}</a
			>
			<span class="mx-2">/</span>
			<a href="/admin/entity-manager/{entityName}/layouts" class="hover:text-gray-700">Layouts</a>
			<span class="mx-2">/</span>
			<span class="text-gray-900">{layoutTypeInfo[layoutType]?.name || layoutType}</span>
		</nav>
		<div class="flex items-center justify-between">
			<div>
				<h1 class="text-2xl font-bold text-gray-900">
					{entity?.label || entityName} - {layoutTypeInfo[layoutType]?.name || layoutType} Layout
				</h1>
				<p class="text-sm text-gray-500 mt-1">
					{layoutTypeInfo[layoutType]?.description || ''}
				</p>
			</div>
			<div class="flex gap-2">
				{#if !isListLayout}
					<button
						type="button"
						onclick={addSection}
						disabled={!layout}
						class="px-4 py-2 bg-gray-100 text-gray-700 rounded-md hover:bg-gray-200 disabled:opacity-50 disabled:cursor-not-allowed"
					>
						Add Section
					</button>
				{/if}
				<button
					type="button"
					onclick={saveLayout}
					disabled={saving || (isListLayout ? false : !layout)}
					class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-600/90 disabled:opacity-50"
				>
					{saving ? 'Saving...' : 'Save Layout'}
				</button>
			</div>
		</div>
	</div>

	{#if loading}
		<div class="text-center py-12 text-gray-500">Loading...</div>
	{:else if error}
		<div class="text-center py-12 text-red-500">{error}</div>
	{:else if isListLayout}
		<!-- List Layout Editor (V1 - Simple Column Picker) -->
		<div class="bg-blue-50 border border-blue-200 rounded-lg p-4 text-sm text-blue-800">
			<p>
				<strong>List Columns:</strong> Select which fields appear as columns in the list view. Drag to reorder.
			</p>
		</div>

		<div class="grid grid-cols-1 md:grid-cols-2 gap-6">
			<!-- Selected Columns -->
			<div class="bg-white shadow rounded-lg p-4">
				<h3 class="text-sm font-medium text-gray-900 mb-3">Selected Columns ({listColumns.length})</h3>
				{#if listColumns.length === 0}
					<p class="text-sm text-gray-500 italic py-4 text-center">No columns selected. Add fields from the right.</p>
				{:else}
					<div class="space-y-2">
						{#each listColumns as columnName, index (columnName)}
							<div
								draggable="true"
								ondragstart={() => handleColumnDragStart(index)}
								ondragover={(e) => handleColumnDragOver(e, index)}
								ondragleave={handleColumnDragLeave}
								ondrop={(e) => handleColumnDrop(e, index)}
								ondragend={handleColumnDragEnd}
								class="flex items-center justify-between px-3 py-2 bg-gray-50 rounded border border-gray-200 cursor-move transition-all
									{draggedColumnIndex === index ? 'opacity-50' : ''}
									{dragOverColumnIndex === index && draggedColumnIndex !== index ? 'border-t-2 border-blue-500' : ''}"
							>
								<div class="flex items-center gap-2">
									<svg class="w-4 h-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8h16M4 16h16" />
									</svg>
									<span class="text-sm text-gray-700">{getFieldLabel(columnName)}</span>
									<span class="text-xs text-gray-400">({columnName})</span>
								</div>
								<button
									type="button"
									onclick={() => removeListColumn(index)}
									class="text-gray-400 hover:text-red-500"
								>
									<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
									</svg>
								</button>
							</div>
						{/each}
					</div>
				{/if}
			</div>

			<!-- Available Fields -->
			<div class="bg-white shadow rounded-lg p-4">
				<h3 class="text-sm font-medium text-gray-900 mb-3">Available Fields ({availableListFields.length})</h3>
				{#if availableListFields.length === 0}
					<p class="text-sm text-gray-500 italic py-4 text-center">All fields are in use.</p>
				{:else}
					<div class="space-y-2">
						{#each availableListFields as field (field.id)}
							<div
								class="flex items-center justify-between px-3 py-2 bg-gray-50 rounded border border-gray-200 hover:bg-gray-100"
							>
								<div>
									<span class="text-sm text-gray-700">{field.label}</span>
									<span class="text-xs text-gray-400 ml-1">({field.name})</span>
								</div>
								<button
									type="button"
									onclick={() => addListColumn(field.name)}
									class="text-blue-600 hover:text-blue-800 text-sm font-medium"
								>
									+ Add
								</button>
							</div>
						{/each}
					</div>
				{/if}
			</div>
		</div>

		<!-- Preview -->
		<div class="bg-white shadow rounded-lg p-6">
			<h2 class="text-lg font-medium text-gray-900 mb-4">Preview</h2>
			<div class="overflow-x-auto">
				<table class="min-w-full divide-y divide-gray-200">
					<thead class="bg-gray-50">
						<tr>
							{#each listColumns as columnName (columnName)}
								<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
									{getFieldLabel(columnName)}
								</th>
							{/each}
							<th class="px-4 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
								Actions
							</th>
						</tr>
					</thead>
					<tbody class="bg-white divide-y divide-gray-200">
						<tr>
							{#each listColumns as columnName, i (columnName)}
								<td class="px-4 py-3 text-sm {i === 0 ? 'text-blue-600 font-medium' : 'text-gray-500'}">
									sample data
								</td>
							{/each}
							<td class="px-4 py-3 text-right text-sm text-gray-400">
								Edit | Delete
							</td>
						</tr>
					</tbody>
				</table>
			</div>
		</div>
	{:else if layout}
		<!-- Detail Layout Editor (V2 - Sections) -->
		<div class="bg-blue-50 border border-blue-200 rounded-lg p-4 text-sm text-blue-800">
			<p>
				<strong>Layout Editor:</strong> Organize fields into sections. Drag sections to reorder them.
				Click the settings icon on a section to configure columns and conditional visibility.
			</p>
		</div>

		<!-- Sections -->
		<div class="space-y-4">
			{#each [...layout.sections].sort((a, b) => a.order - b.order) as section, index (section.id)}
				<div
					draggable="true"
					ondragstart={() => handleSectionDragStart(index)}
					ondragover={(e) => handleSectionDragOver(e, index)}
					ondragleave={handleSectionDragLeave}
					ondrop={(e) => handleSectionDrop(e, index)}
					ondragend={handleSectionDragEnd}
					class="transition-all duration-150 {draggedSectionIndex === index
						? 'opacity-50'
						: ''} {dragOverSectionIndex === index && draggedSectionIndex !== index
						? 'border-t-4 border-blue-500 pt-2'
						: ''}"
				>
					<SectionEditor
						{section}
						allFields={fields}
						usedFieldNames={usedFieldNames()}
						collapsed={collapsedSections.has(section.id)}
						onupdate={(s) => updateSection(index, s)}
						ondelete={() => deleteSection(index)}
						ontoggle={() => toggleSectionCollapse(section.id)}
					/>
				</div>
			{/each}
		</div>

		<!-- Unassigned fields warning -->
		{#if unassignedFields.length > 0}
			<div class="bg-yellow-50 border border-yellow-200 rounded-lg p-4">
				<h3 class="text-sm font-medium text-yellow-800 mb-2">
					{unassignedFields.length} field{unassignedFields.length !== 1 ? 's' : ''} not in layout:
				</h3>
				<div class="flex flex-wrap gap-2">
					{#each unassignedFields as field (field.id)}
						<span class="px-2 py-1 text-xs bg-yellow-100 text-yellow-800 rounded">
							{field.label}
						</span>
					{/each}
				</div>
				<p class="mt-2 text-xs text-yellow-700">
					Add these fields to a section above, or they won't appear on the detail page.
				</p>
			</div>
		{/if}

		<!-- Preview -->
		<div class="bg-white shadow rounded-lg p-6">
			<h2 class="text-lg font-medium text-gray-900 mb-4">Preview</h2>
			<div class="space-y-6">
				{#each [...layout.sections].sort((a, b) => a.order - b.order) as section (section.id)}
					{#if section.fields.length > 0}
						<div class="border border-gray-200 rounded-lg overflow-hidden">
							<div class="px-4 py-3 bg-gray-100 border-b border-gray-200">
								<h3 class="font-medium text-gray-900">{section.label}</h3>
								{#if section.visibility.type === 'conditional'}
									<span class="text-xs text-blue-600">(conditionally visible)</span>
								{/if}
							</div>
							<div
								class="p-4 grid gap-4"
								style="grid-template-columns: repeat({section.columns}, minmax(0, 1fr))"
							>
								{#each section.fields as field (field.name)}
									{@const fieldDef = fields.find((f) => f.name === field.name)}
									<div class="grid grid-cols-3 gap-2">
										<dt class="text-sm font-medium text-gray-500">{fieldDef?.label || field.name}</dt>
										<dd class="col-span-2 text-sm text-gray-400 italic">
											sample data
											{#if field.visibility.type === 'conditional'}
												<span class="text-blue-600">(conditional)</span>
											{/if}
										</dd>
									</div>
								{/each}
							</div>
						</div>
					{/if}
				{/each}
			</div>
		</div>
	{/if}
</div>
