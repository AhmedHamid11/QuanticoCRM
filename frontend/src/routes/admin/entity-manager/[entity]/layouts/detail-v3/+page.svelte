<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { beforeNavigate } from '$app/navigation';
	import { get, put } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import type { EntityDef, FieldDef } from '$lib/types/admin';
	import type { LayoutDataV3, LayoutSectionV2, LayoutFieldV2, LayoutV3Response } from '$lib/types/layout';
	import { createDefaultVisibility, createDefaultField } from '$lib/types/layout';

	let entityName = $derived($page.params.entity);

	// Data
	let entity = $state<EntityDef | null>(null);
	let fields = $state<FieldDef[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// Editor state — isolated deep clone, never touches live layout until Save
	let editorLayout = $state<LayoutDataV3 | null>(null);
	let isDirty = $state(false);
	let saving = $state(false);

	// Section drag state
	let draggedSectionIndex = $state<number | null>(null);
	let dragOverSectionIndex = $state<number | null>(null);

	// Per-section field drag state (keyed by section index)
	let draggedFieldInfo = $state<{ sectionIndex: number; fieldIndex: number } | null>(null);
	let dragOverFieldInfo = $state<{ sectionIndex: number; fieldIndex: number } | null>(null);

	// UI state — which sections have their field list expanded
	let expandedSections = $state<Set<string>>(new Set());

	// Track all used field names across all sections
	let usedFieldNames = $derived(() => {
		if (!editorLayout) return new Set<string>();
		const used = new Set<string>();
		for (const section of editorLayout.sections) {
			for (const field of section.fields) {
				used.add(field.name);
			}
		}
		return used;
	});

	// Fields not assigned to any section
	let unassignedFields = $derived(fields.filter((f) => !usedFieldNames().has(f.name)));

	// Navigation guard — must be called at component initialization, not inside onMount
	beforeNavigate(({ cancel }) => {
		if (isDirty && !confirm('You have unsaved changes. Leave anyway?')) cancel();
	});

	// Browser unload guard for hard navigation
	function handleBeforeUnload(e: BeforeUnloadEvent) {
		if (isDirty) {
			e.preventDefault();
		}
	}

	onMount(() => {
		window.addEventListener('beforeunload', handleBeforeUnload);
		loadData();
	});

	onDestroy(() => {
		window.removeEventListener('beforeunload', handleBeforeUnload);
	});

	// ---- Data loading ----

	async function loadData() {
		try {
			loading = true;
			error = null;

			const [entityData, fieldsData, layoutResponse] = await Promise.all([
				get<EntityDef>(`/admin/entities/${entityName}`),
				get<FieldDef[]>(`/admin/entities/${entityName}/fields`),
				get<LayoutV3Response>(`/admin/entities/${entityName}/layouts/detail/v3`)
			]);

			entity = entityData;
			fields = fieldsData;
			editorLayout = structuredClone(layoutResponse.layout);
			isDirty = false;

			// Start with all sections expanded
			expandedSections = new Set(editorLayout.sections.map((s) => s.id));
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load layout';
		} finally {
			loading = false;
		}
	}

	// ---- Mutation helper ----

	function mutate(updater: (l: LayoutDataV3) => void) {
		if (!editorLayout) return;
		updater(editorLayout);
		isDirty = true;
	}

	// ---- Save / Discard ----

	async function saveLayout() {
		if (!editorLayout) return;
		try {
			saving = true;
			await put(`/admin/entities/${entityName}/layouts/detail/v3`, editorLayout);
			isDirty = false;
			toast.success('Layout saved successfully');
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to save layout';
			toast.error(message);
		} finally {
			saving = false;
		}
	}

	async function discardChanges() {
		if (!isDirty || !confirm('Discard all unsaved changes?')) return;
		await loadData();
	}

	// ---- Section operations ----

	function generateId(prefix: string): string {
		return prefix + '_' + Math.random().toString(36).substr(2, 9);
	}

	function addSection() {
		if (!editorLayout) return;
		const newSection: LayoutSectionV2 = {
			id: generateId('section'),
			label: 'New Section',
			order: editorLayout.sections.length + 1,
			collapsible: true,
			collapsed: false,
			columns: 2,
			visibility: createDefaultVisibility(),
			fields: []
		};
		mutate((l) => {
			l.sections = [...l.sections, newSection];
		});
		expandedSections = new Set([...expandedSections, newSection.id]);
	}

	function deleteSection(index: number) {
		if (!editorLayout) return;
		if (editorLayout.sections.length <= 1) {
			toast.error('Cannot delete the last section');
			return;
		}
		if (!confirm('Delete this section and all its fields?')) return;

		const sectionId = editorLayout.sections[index].id;
		mutate((l) => {
			l.sections = l.sections.filter((_, i) => i !== index);
			// Renumber order
			l.sections.forEach((s, i) => { s.order = i + 1; });
			// Remove from all tabs
			for (const tab of l.tabs) {
				tab.sectionIds = tab.sectionIds.filter((id) => id !== sectionId);
			}
		});
	}

	function updateSectionLabel(index: number, label: string) {
		mutate((l) => {
			l.sections[index].label = label;
		});
	}

	function toggleColumns(index: number, columns: 1 | 2) {
		mutate((l) => {
			l.sections[index].columns = columns;
		});
	}

	function toggleSectionExpand(sectionId: string) {
		const next = new Set(expandedSections);
		if (next.has(sectionId)) {
			next.delete(sectionId);
		} else {
			next.add(sectionId);
		}
		expandedSections = next;
	}

	// ---- Section drag-and-drop ----

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
		if (draggedSectionIndex === null || draggedSectionIndex === dropIndex) {
			draggedSectionIndex = null;
			dragOverSectionIndex = null;
			return;
		}

		const sections = editorLayout ? [...editorLayout.sections] : [];
		const [dragged] = sections.splice(draggedSectionIndex, 1);
		sections.splice(dropIndex, 0, dragged);
		// Critical: renumber order after reorder
		sections.forEach((s, i) => { s.order = i + 1; });

		mutate((l) => { l.sections = sections; });

		draggedSectionIndex = null;
		dragOverSectionIndex = null;
	}

	function handleSectionDragEnd() {
		draggedSectionIndex = null;
		dragOverSectionIndex = null;
	}

	// ---- Field operations ----

	function addFieldToSection(sectionIndex: number, fieldName: string) {
		mutate((l) => {
			l.sections[sectionIndex].fields = [
				...l.sections[sectionIndex].fields,
				createDefaultField(fieldName)
			];
		});
	}

	function removeFieldFromSection(sectionIndex: number, fieldIndex: number) {
		mutate((l) => {
			l.sections[sectionIndex].fields = l.sections[sectionIndex].fields.filter(
				(_, i) => i !== fieldIndex
			);
		});
	}

	// ---- Field drag-and-drop ----

	function handleFieldDragStart(sectionIndex: number, fieldIndex: number) {
		draggedFieldInfo = { sectionIndex, fieldIndex };
	}

	function handleFieldDragOver(e: DragEvent, sectionIndex: number, fieldIndex: number) {
		e.preventDefault();
		dragOverFieldInfo = { sectionIndex, fieldIndex };
	}

	function handleFieldDragLeave() {
		dragOverFieldInfo = null;
	}

	function handleFieldDrop(e: DragEvent, dropSectionIndex: number, dropFieldIndex: number) {
		e.preventDefault();
		if (!draggedFieldInfo || !editorLayout) {
			draggedFieldInfo = null;
			dragOverFieldInfo = null;
			return;
		}

		const { sectionIndex: srcSection, fieldIndex: srcField } = draggedFieldInfo;

		// Only support reorder within same section in this plan
		if (srcSection !== dropSectionIndex || srcField === dropFieldIndex) {
			draggedFieldInfo = null;
			dragOverFieldInfo = null;
			return;
		}

		const sectionFields = [...editorLayout.sections[srcSection].fields];
		const [dragged] = sectionFields.splice(srcField, 1);
		sectionFields.splice(dropFieldIndex, 0, dragged);

		mutate((l) => {
			l.sections[srcSection].fields = sectionFields;
		});

		draggedFieldInfo = null;
		dragOverFieldInfo = null;
	}

	function handleFieldDragEnd() {
		draggedFieldInfo = null;
		dragOverFieldInfo = null;
	}

	// ---- Helpers ----

	function getFieldLabel(fieldName: string): string {
		return fields.find((f) => f.name === fieldName)?.label || fieldName;
	}

	function getFieldType(fieldName: string): string {
		return fields.find((f) => f.name === fieldName)?.type || '';
	}

	// Available fields for a specific section (not yet used anywhere)
	function availableFieldsForSection(sectionIndex: number): FieldDef[] {
		return fields.filter((f) => !usedFieldNames().has(f.name));
	}
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
			<a href="/admin/entity-manager/{entityName}/layouts" class="hover:text-gray-700">Layouts</a>
			<span class="mx-2">/</span>
			<span class="text-gray-900">V3 Detail Editor</span>
		</nav>

		<!-- Top bar -->
		<div class="flex items-center justify-between">
			<div>
				<h1 class="text-2xl font-bold text-gray-900">
					{entity?.label || entityName} — V3 Detail Layout Editor
				</h1>
				<p class="text-sm text-gray-500 mt-1">
					Drag sections to reorder. Toggle column count per section. Save when done.
				</p>
			</div>
			<div class="flex items-center gap-3">
				{#if isDirty}
					<span class="text-xs text-amber-600 font-medium">Unsaved changes</span>
				{/if}
				<button
					type="button"
					onclick={discardChanges}
					disabled={!isDirty || saving}
					class="px-4 py-2 bg-white border border-gray-300 text-gray-700 rounded-md hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					Discard
				</button>
				<button
					type="button"
					onclick={saveLayout}
					disabled={!isDirty || saving}
					class="px-4 py-2 bg-green-600 text-white rounded-md hover:bg-green-700 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					{saving ? 'Saving...' : 'Save Layout'}
				</button>
			</div>
		</div>
	</div>

	{#if loading}
		<div class="text-center py-12 text-gray-500">Loading layout...</div>
	{:else if error}
		<div class="text-center py-12 text-red-500">{error}</div>
	{:else if editorLayout}
		<!-- Info banner -->
		<div class="bg-blue-50 border border-blue-200 rounded-lg p-4 text-sm text-blue-800">
			<strong>V3 Layout Editor:</strong> Drag the grip handle to reorder sections. Toggle 1 or 2 columns per section. Expand a section to reorder its fields or add/remove fields.
		</div>

		<!-- Sections list -->
		<div class="space-y-3">
			{#each [...editorLayout.sections].sort((a, b) => a.order - b.order) as section, index (section.id)}
				{@const isExpanded = expandedSections.has(section.id)}
				{@const isDraggingThis = draggedSectionIndex === index}
				{@const isDragTarget = dragOverSectionIndex === index && draggedSectionIndex !== index}
				{@const sectionAvailableFields = availableFieldsForSection(index)}

				<div
					draggable="true"
					ondragstart={() => handleSectionDragStart(index)}
					ondragover={(e) => handleSectionDragOver(e, index)}
					ondragleave={handleSectionDragLeave}
					ondrop={(e) => handleSectionDrop(e, index)}
					ondragend={handleSectionDragEnd}
					class="bg-white shadow rounded-lg border transition-all duration-150
						{isDraggingThis ? 'opacity-50 border-blue-300' : 'border-gray-200'}
						{isDragTarget ? 'border-blue-500 bg-blue-50' : ''}"
				>
					<!-- Section header -->
					<div class="flex items-center gap-3 px-4 py-3 border-b border-gray-100">
						<!-- Drag handle -->
						<div class="flex-shrink-0 cursor-move text-gray-400 hover:text-gray-600">
							<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8h16M4 16h16" />
							</svg>
						</div>

						<!-- Section label input -->
						<input
							type="text"
							value={section.label}
							oninput={(e) => updateSectionLabel(index, (e.target as HTMLInputElement).value)}
							class="flex-1 text-sm font-medium text-gray-900 bg-transparent border-0 focus:outline-none focus:ring-1 focus:ring-blue-500 rounded px-1 py-0.5"
							placeholder="Section name"
						/>

						<!-- Column toggle -->
						<div class="flex-shrink-0 flex items-center gap-1 border border-gray-200 rounded-md overflow-hidden">
							<button
								type="button"
								onclick={() => toggleColumns(index, 1)}
								class="px-2.5 py-1 text-xs font-medium transition-colors
									{section.columns === 1
										? 'bg-blue-600 text-white'
										: 'text-gray-600 hover:bg-gray-100'}"
								title="Single column"
							>
								1 col
							</button>
							<button
								type="button"
								onclick={() => toggleColumns(index, 2)}
								class="px-2.5 py-1 text-xs font-medium transition-colors border-l border-gray-200
									{section.columns === 2 || section.columns === 3
										? 'bg-blue-600 text-white'
										: 'text-gray-600 hover:bg-gray-100'}"
								title="Two columns"
							>
								2 col
							</button>
						</div>

						<!-- Field count badge -->
						<span class="flex-shrink-0 text-xs text-gray-400 tabular-nums">
							{section.fields.length} field{section.fields.length !== 1 ? 's' : ''}
						</span>

						<!-- Expand/collapse toggle -->
						<button
							type="button"
							onclick={() => toggleSectionExpand(section.id)}
							class="flex-shrink-0 text-gray-400 hover:text-gray-600"
							title={isExpanded ? 'Collapse section' : 'Expand section'}
						>
							<svg
								class="w-5 h-5 transition-transform {isExpanded ? 'rotate-180' : ''}"
								fill="none"
								stroke="currentColor"
								viewBox="0 0 24 24"
							>
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
							</svg>
						</button>

						<!-- Delete section -->
						<button
							type="button"
							onclick={() => deleteSection(index)}
							class="flex-shrink-0 text-gray-400 hover:text-red-500"
							title="Delete section"
						>
							<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
							</svg>
						</button>
					</div>

					<!-- Expanded: field list + add field -->
					{#if isExpanded}
						<div class="p-4 space-y-2">
							{#if section.fields.length === 0}
								<p class="text-sm text-gray-400 italic text-center py-3">
									No fields in this section. Add fields below.
								</p>
							{:else}
								<!-- Field rows with drag-and-drop -->
								{#each section.fields as field, fieldIndex (field.name)}
									{@const isFieldDragging = draggedFieldInfo?.sectionIndex === index && draggedFieldInfo?.fieldIndex === fieldIndex}
									{@const isFieldDropTarget = dragOverFieldInfo?.sectionIndex === index && dragOverFieldInfo?.fieldIndex === fieldIndex && draggedFieldInfo?.sectionIndex === index && draggedFieldInfo?.fieldIndex !== fieldIndex}

									<div
										draggable="true"
										ondragstart={() => handleFieldDragStart(index, fieldIndex)}
										ondragover={(e) => handleFieldDragOver(e, index, fieldIndex)}
										ondragleave={handleFieldDragLeave}
										ondrop={(e) => handleFieldDrop(e, index, fieldIndex)}
										ondragend={handleFieldDragEnd}
										class="flex items-center gap-2 px-3 py-2 bg-gray-50 rounded border transition-all
											{isFieldDragging ? 'opacity-50 border-blue-300' : 'border-gray-200'}
											{isFieldDropTarget ? 'border-blue-500 bg-blue-50' : ''}"
									>
										<!-- Field drag handle -->
										<div class="flex-shrink-0 cursor-move text-gray-400">
											<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
												<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8h16M4 16h16" />
											</svg>
										</div>

										<!-- Field label -->
										<span class="flex-1 text-sm text-gray-800">{getFieldLabel(field.name)}</span>

										<!-- Field name (api name) -->
										<span class="text-xs text-gray-400 font-mono">{field.name}</span>

										<!-- Field type badge -->
										{#if getFieldType(field.name)}
											<span class="flex-shrink-0 text-xs px-1.5 py-0.5 bg-gray-200 text-gray-600 rounded">
												{getFieldType(field.name)}
											</span>
										{/if}

										<!-- Remove field -->
										<button
											type="button"
											onclick={() => removeFieldFromSection(index, fieldIndex)}
											class="flex-shrink-0 text-gray-400 hover:text-red-500"
											title="Remove field from section"
										>
											<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
												<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
											</svg>
										</button>
									</div>
								{/each}
							{/if}

							<!-- Add field pills -->
							{#if sectionAvailableFields.length > 0}
								<div class="pt-2 border-t border-gray-100">
									<p class="text-xs text-gray-500 mb-2">Add field:</p>
									<div class="flex flex-wrap gap-1.5">
										{#each sectionAvailableFields as availField (availField.id)}
											<button
												type="button"
												onclick={() => addFieldToSection(index, availField.name)}
												class="text-xs px-2 py-1 bg-blue-50 text-blue-700 border border-blue-200 rounded hover:bg-blue-100 transition-colors"
											>
												+ {availField.label}
											</button>
										{/each}
									</div>
								</div>
							{/if}
						</div>
					{/if}
				</div>
			{/each}
		</div>

		<!-- Add section button -->
		<div>
			<button
				type="button"
				onclick={addSection}
				class="w-full px-4 py-3 border-2 border-dashed border-gray-300 text-gray-500 rounded-lg hover:border-blue-400 hover:text-blue-600 transition-colors text-sm font-medium"
			>
				+ Add Section
			</button>
		</div>

		<!-- Unassigned fields warning -->
		{#if unassignedFields.length > 0}
			<div class="bg-yellow-50 border border-yellow-200 rounded-lg p-4">
				<h3 class="text-sm font-medium text-yellow-800 mb-2">
					{unassignedFields.length} field{unassignedFields.length !== 1 ? 's' : ''} not in any section:
				</h3>
				<div class="flex flex-wrap gap-2">
					{#each unassignedFields as field (field.id)}
						<span class="px-2 py-1 text-xs bg-yellow-100 text-yellow-800 rounded">
							{field.label}
						</span>
					{/each}
				</div>
				<p class="mt-2 text-xs text-yellow-700">
					These fields won't appear on the detail page until added to a section.
				</p>
			</div>
		{/if}
	{/if}
</div>
