<script lang="ts">
	import type { FieldDef } from '$lib/types/admin';
	import type { LayoutSectionV2, LayoutFieldV2, VisibilityRule } from '$lib/types/layout';
	import { createDefaultField, createDefaultVisibility } from '$lib/types/layout';
	import VisibilityRuleEditor from './VisibilityRuleEditor.svelte';

	interface Props {
		section: LayoutSectionV2;
		allFields: FieldDef[];
		usedFieldNames: Set<string>;
		collapsed: boolean;
		onupdate: (section: LayoutSectionV2) => void;
		ondelete: () => void;
		ontoggle: () => void;
	}

	let {
		section,
		allFields,
		usedFieldNames,
		collapsed,
		onupdate,
		ondelete,
		ontoggle
	}: Props = $props();

	// Fields available to add to this section
	let availableFields = $derived(
		allFields.filter(
			(f) =>
				!section.fields.some((sf) => sf.name === f.name) &&
				!usedFieldNames.has(f.name)
		)
	);

	// Drag state
	let draggedIndex = $state<number | null>(null);
	let dragOverIndex = $state<number | null>(null);

	// Edit modal state
	let showFieldVisibility = $state<string | null>(null);
	let showSectionSettings = $state(false);

	function updateSection(updates: Partial<LayoutSectionV2>) {
		onupdate({ ...section, ...updates });
	}

	function addField(fieldName: string) {
		const fields = [...section.fields, createDefaultField(fieldName)];
		updateSection({ fields });
	}

	function removeField(fieldName: string) {
		const fields = section.fields.filter((f) => f.name !== fieldName);
		updateSection({ fields });
	}

	function updateFieldVisibility(fieldName: string, visibility: VisibilityRule) {
		const fields = section.fields.map((f) =>
			f.name === fieldName ? { ...f, visibility } : f
		);
		updateSection({ fields });
	}

	function handleDragStart(index: number) {
		draggedIndex = index;
	}

	function handleDragOver(e: DragEvent, index: number) {
		e.preventDefault();
		dragOverIndex = index;
	}

	function handleDragLeave() {
		dragOverIndex = null;
	}

	function handleDrop(e: DragEvent, dropIndex: number) {
		e.preventDefault();
		if (draggedIndex === null || draggedIndex === dropIndex) {
			draggedIndex = null;
			dragOverIndex = null;
			return;
		}

		const fields = [...section.fields];
		const [draggedItem] = fields.splice(draggedIndex, 1);
		fields.splice(dropIndex, 0, draggedItem);
		updateSection({ fields });

		draggedIndex = null;
		dragOverIndex = null;
	}

	function handleDragEnd() {
		draggedIndex = null;
		dragOverIndex = null;
	}

	function getFieldLabel(fieldName: string): string {
		const field = allFields.find((f) => f.name === fieldName);
		return field?.label || fieldName;
	}

	function getFieldType(fieldName: string): string {
		const field = allFields.find((f) => f.name === fieldName);
		return field?.type || 'unknown';
	}

	function hasConditionalVisibility(field: LayoutFieldV2): boolean {
		return field.visibility.type === 'conditional';
	}
</script>

<div class="bg-white border border-gray-200 rounded-lg shadow-sm">
	<!-- Section Header -->
	<div class="flex items-center gap-3 p-4 border-b border-gray-200 bg-gray-50 rounded-t-lg">
		<button
			onclick={ontoggle}
			class="p-1 text-gray-400 hover:text-gray-600"
			title={collapsed ? 'Expand section' : 'Collapse section'}
		>
			<svg
				class="w-5 h-5 transition-transform {collapsed ? '' : 'rotate-90'}"
				fill="none"
				viewBox="0 0 24 24"
				stroke="currentColor"
			>
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
			</svg>
		</button>

		<input
			type="text"
			value={section.label}
			onchange={(e) => updateSection({ label: e.currentTarget.value })}
			class="flex-1 text-lg font-medium bg-transparent border-none focus:ring-0 p-0"
			placeholder="Section name"
		/>

		<div class="flex items-center gap-2">
			<button
				onclick={() => (showSectionSettings = !showSectionSettings)}
				class="p-2 text-gray-400 hover:text-gray-600 hover:bg-gray-100 rounded"
				title="Section settings"
			>
				<svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						stroke-width="2"
						d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"
					/>
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						stroke-width="2"
						d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
					/>
				</svg>
			</button>

			<button
				onclick={ondelete}
				class="p-2 text-red-400 hover:text-red-600 hover:bg-red-50 rounded"
				title="Delete section"
			>
				<svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						stroke-width="2"
						d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
					/>
				</svg>
			</button>
		</div>
	</div>

	<!-- Section Settings (expandable) -->
	{#if showSectionSettings}
		<div class="p-4 border-b border-gray-200 bg-blue-50 space-y-4">
			<div class="grid grid-cols-1 md:grid-cols-3 gap-4">
				<div>
					<label class="block text-sm font-medium text-gray-700 mb-1">Columns</label>
					<select
						value={section.columns}
						onchange={(e) => updateSection({ columns: parseInt(e.currentTarget.value) as 1 | 2 | 3 })}
						class="w-full rounded-md border-gray-300 shadow-sm text-sm focus:border-blue-500 focus:ring-blue-500"
					>
						<option value={1}>1 Column</option>
						<option value={2}>2 Columns</option>
						<option value={3}>3 Columns</option>
					</select>
				</div>
				<div class="flex items-center gap-2">
					<input
						type="checkbox"
						id="collapsible-{section.id}"
						checked={section.collapsible}
						onchange={(e) => updateSection({ collapsible: e.currentTarget.checked })}
						class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
					/>
					<label for="collapsible-{section.id}" class="text-sm text-gray-700">Collapsible</label>
				</div>
				{#if section.collapsible}
					<div class="flex items-center gap-2">
						<input
							type="checkbox"
							id="collapsed-{section.id}"
							checked={section.collapsed}
							onchange={(e) => updateSection({ collapsed: e.currentTarget.checked })}
							class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
						/>
						<label for="collapsed-{section.id}" class="text-sm text-gray-700">Default Collapsed</label>
					</div>
				{/if}
			</div>

			<div>
				<h4 class="text-sm font-medium text-gray-700 mb-2">Section Visibility</h4>
				<VisibilityRuleEditor
					rule={section.visibility}
					fields={allFields}
					onchange={(rule) => updateSection({ visibility: rule })}
				/>
			</div>
		</div>
	{/if}

	<!-- Section Content (collapsible) -->
	{#if !collapsed}
		<div class="p-4 space-y-4">
			<!-- Fields in this section -->
			{#if section.fields.length === 0}
				<p class="text-gray-500 text-sm py-4 text-center">
					No fields in this section. Add fields from the available list below.
				</p>
			{:else}
				<div class="space-y-2">
					{#each section.fields as field, index (field.name)}
						<div
							draggable="true"
							ondragstart={() => handleDragStart(index)}
							ondragover={(e) => handleDragOver(e, index)}
							ondragleave={handleDragLeave}
							ondrop={(e) => handleDrop(e, index)}
							ondragend={handleDragEnd}
							class="flex items-center gap-2 p-3 bg-gray-50 rounded-lg border-2 transition-all duration-150
								{draggedIndex === index ? 'opacity-50 border-blue-300 bg-blue-50' : 'border-gray-200'}
								{dragOverIndex === index && draggedIndex !== index ? 'border-blue-500 bg-blue-50' : ''}
								{draggedIndex !== null ? 'cursor-grabbing' : 'cursor-grab'}"
						>
							<div class="text-gray-400 hover:text-gray-600">
								<svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8h16M4 16h16" />
								</svg>
							</div>
							<div class="flex-1">
								<div class="font-medium text-gray-900">{getFieldLabel(field.name)}</div>
								<div class="text-xs text-gray-500">
									{field.name} ({getFieldType(field.name)})
									{#if hasConditionalVisibility(field)}
										<span class="ml-2 text-blue-600">(conditional)</span>
									{/if}
								</div>
							</div>

							<button
								onclick={() => (showFieldVisibility = showFieldVisibility === field.name ? null : field.name)}
								class="p-2 text-gray-400 hover:text-blue-600 hover:bg-blue-50 rounded {showFieldVisibility === field.name ? 'text-blue-600 bg-blue-50' : ''}"
								title="Field visibility"
							>
								<svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
									<path
										stroke-linecap="round"
										stroke-linejoin="round"
										stroke-width="2"
										d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
									/>
									<path
										stroke-linecap="round"
										stroke-linejoin="round"
										stroke-width="2"
										d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
									/>
								</svg>
							</button>

							<button
								onclick={() => removeField(field.name)}
								class="p-2 text-red-500 hover:text-red-700 hover:bg-red-50 rounded"
								title="Remove from section"
							>
								<svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
								</svg>
							</button>
						</div>

						<!-- Field visibility editor (inline) -->
						{#if showFieldVisibility === field.name}
							<div class="ml-8 p-3 bg-blue-50 rounded-lg border border-blue-200">
								<h5 class="text-sm font-medium text-gray-700 mb-2">
									Visibility for "{getFieldLabel(field.name)}"
								</h5>
								<VisibilityRuleEditor
									rule={field.visibility}
									fields={allFields}
									onchange={(rule) => updateFieldVisibility(field.name, rule)}
								/>
							</div>
						{/if}
					{/each}
				</div>
			{/if}

			<!-- Available fields to add -->
			{#if availableFields.length > 0}
				<div class="border-t border-gray-200 pt-4">
					<h4 class="text-sm font-medium text-gray-700 mb-2">Add Fields</h4>
					<div class="flex flex-wrap gap-2">
						{#each availableFields as field (field.id)}
							<button
								onclick={() => addField(field.name)}
								class="inline-flex items-center gap-1 px-3 py-1.5 text-sm bg-gray-100 text-gray-700 rounded-full hover:bg-green-100 hover:text-green-700 transition-colors"
							>
								<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
								</svg>
								{field.label}
							</button>
						{/each}
					</div>
				</div>
			{/if}
		</div>
	{/if}
</div>
