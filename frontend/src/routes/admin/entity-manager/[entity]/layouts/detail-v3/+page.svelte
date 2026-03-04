<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { beforeNavigate } from '$app/navigation';
	import { get, put } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import type { EntityDef, FieldDef } from '$lib/types/admin';
	import type { LayoutDataV3, LayoutSectionV2, LayoutFieldV2, LayoutTabV3, LayoutSidebarCardV3, LayoutV3Response, SectionCardType, RelatedListCardConfig, CustomPageCardConfig } from '$lib/types/layout';
	import { createDefaultVisibility, createDefaultField } from '$lib/types/layout';
	import type { RelatedListConfig } from '$lib/types/related-list';

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

	// Tab drag state
	let draggedTabIndex = $state<number | null>(null);
	let dragOverTabIndex = $state<number | null>(null);

	// Sidebar card drag state
	let draggedCardIndex = $state<number | null>(null);
	let dragOverCardIndex = $state<number | null>(null);

	// Header field drag state
	let draggedHeaderFieldIndex = $state<number | null>(null);
	let dragOverHeaderFieldIndex = $state<number | null>(null);

	// UI state — which sections have their field list expanded
	let expandedSections = $state<Set<string>>(new Set());

	// Panel collapse state (all expanded by default)
	let tabsPanelCollapsed = $state(false);
	let headerPanelCollapsed = $state(false);
	let sidebarPanelCollapsed = $state(false);

	// Sidebar card expanded state
	let expandedCards = $state<Set<string>>(new Set());

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

	// Sections not assigned to any tab
	let unassignedSections = $derived(() => {
		if (!editorLayout) return [];
		return editorLayout.sections.filter(
			(s) => !editorLayout!.tabs.some((t) => t.sectionIds.includes(s.id))
		);
	});

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
			// Start with all sidebar cards collapsed
			expandedCards = new Set();

			// Fetch related list configs for RelatedListCard dropdown
			try {
				relatedListConfigs = await get<RelatedListConfig[]>(`/admin/entities/${entityName}/related-lists`);
			} catch {
				relatedListConfigs = [];
			}
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

	function openAddSection() {
		cardTypeSelectorTarget = 'new';
		showCardTypeSelector = true;
	}

	function selectCardType(cardType: SectionCardType) {
		showCardTypeSelector = false;
		if (cardTypeSelectorTarget === 'new') {
			addSectionWithType(cardType);
		} else {
			changeSectionCardType(cardTypeSelectorTarget, cardType);
		}
	}

	function addSectionWithType(cardType: SectionCardType) {
		if (!editorLayout) return;
		const newSection: LayoutSectionV2 = {
			id: generateId('section'),
			label: cardType === 'activity' ? 'Activities' : cardType === 'relatedList' ? 'Related Records' : cardType === 'customPage' ? 'Custom Content' : 'New Section',
			order: editorLayout.sections.length + 1,
			collapsible: true,
			collapsed: false,
			columns: 2,
			visibility: createDefaultVisibility(),
			fields: [],
			cardType,
			cardConfig: cardType === 'activity' ? {} : cardType === 'relatedList' ? { relatedListConfigId: '' } : cardType === 'customPage' ? { mode: 'iframe' as const, url: '', height: 400 } : null
		};
		mutate((l) => {
			l.sections = [...l.sections, newSection];
		});
		expandedSections = new Set([...expandedSections, newSection.id]);
	}

	function changeSectionCardType(sectionIndex: number, newType: SectionCardType) {
		mutate((l) => {
			const section = l.sections[sectionIndex];
			section.cardType = newType;
			// Clear previous config when switching types
			if (newType === 'field') {
				section.cardConfig = undefined;
			} else if (newType === 'activity') {
				section.cardConfig = {};
				section.fields = [];
			} else if (newType === 'relatedList') {
				section.cardConfig = { relatedListConfigId: '' };
				section.fields = [];
			} else if (newType === 'customPage') {
				section.cardConfig = { mode: 'iframe' as const, url: '', height: 400 };
				section.fields = [];
			}
		});
	}

	function openChangeCardType(sectionIndex: number) {
		cardTypeSelectorTarget = sectionIndex;
		showCardTypeSelector = true;
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

	function toggleColumns(index: number, columns: 1 | 2 | 3) {
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

	// ---- Tab operations ----

	function addTab() {
		if (!editorLayout) return;
		const maxOrder = editorLayout.tabs.length > 0
			? Math.max(...editorLayout.tabs.map((t) => t.order))
			: 0;
		const newTab: LayoutTabV3 = {
			id: 'tab_' + Math.random().toString(36).substr(2, 9),
			label: 'New Tab',
			order: maxOrder + 1,
			sectionIds: []
		};
		mutate((l) => {
			l.tabs = [...l.tabs, newTab];
		});
	}

	function deleteTab(tabId: string) {
		if (!editorLayout) return;
		const tab = editorLayout.tabs.find((t) => t.id === tabId);
		if (!tab) return;

		const sectionCount = tab.sectionIds.length;
		const confirmMsg = sectionCount > 0
			? `Delete tab "${tab.label}"? Its ${sectionCount} section${sectionCount !== 1 ? 's' : ''} will become unassigned.`
			: `Delete tab "${tab.label}"?`;

		if (!confirm(confirmMsg)) return;

		mutate((l) => {
			l.tabs = l.tabs.filter((t) => t.id !== tabId);
			// Renumber order
			l.tabs.sort((a, b) => a.order - b.order).forEach((t, i) => { t.order = i + 1; });
		});
	}

	function updateTabLabel(tabId: string, label: string) {
		mutate((l) => {
			const tab = l.tabs.find((t) => t.id === tabId);
			if (tab) tab.label = label;
		});
	}

	function moveSectionToTab(sectionId: string, targetTabId: string) {
		mutate((l) => {
			// Remove sectionId from ALL tabs first (critical invariant)
			for (const tab of l.tabs) {
				tab.sectionIds = tab.sectionIds.filter((id) => id !== sectionId);
			}
			// Add to target tab if one is specified
			if (targetTabId) {
				const target = l.tabs.find((t) => t.id === targetTabId);
				if (target) {
					target.sectionIds = [...target.sectionIds, sectionId];
				}
			}
		});
	}

	function getSectionTabId(sectionId: string): string {
		if (!editorLayout) return '';
		const tab = editorLayout.tabs.find((t) => t.sectionIds.includes(sectionId));
		return tab ? tab.id : '';
	}

	// ---- Tab drag-and-drop ----

	function handleTabDragStart(index: number) {
		draggedTabIndex = index;
	}

	function handleTabDragOver(e: DragEvent, index: number) {
		e.preventDefault();
		dragOverTabIndex = index;
	}

	function handleTabDragLeave() {
		dragOverTabIndex = null;
	}

	function handleTabDrop(e: DragEvent, dropIndex: number) {
		e.preventDefault();
		if (draggedTabIndex === null || draggedTabIndex === dropIndex || !editorLayout) {
			draggedTabIndex = null;
			dragOverTabIndex = null;
			return;
		}

		const tabs = [...editorLayout.tabs].sort((a, b) => a.order - b.order);
		const [dragged] = tabs.splice(draggedTabIndex, 1);
		tabs.splice(dropIndex, 0, dragged);
		// Renumber order after reorder
		tabs.forEach((t, i) => { t.order = i + 1; });

		mutate((l) => { l.tabs = tabs; });

		draggedTabIndex = null;
		dragOverTabIndex = null;
	}

	function handleTabDragEnd() {
		draggedTabIndex = null;
		dragOverTabIndex = null;
	}

	// ---- Header field operations ----

	function addHeaderField(fieldName: string) {
		mutate((l) => {
			if (!l.header.fields.includes(fieldName)) {
				l.header.fields = [...l.header.fields, fieldName];
			}
		});
	}

	function removeHeaderField(fieldName: string) {
		mutate((l) => {
			l.header.fields = l.header.fields.filter((f) => f !== fieldName);
		});
	}

	// ---- Header field drag-and-drop ----

	function handleHeaderFieldDragStart(index: number) {
		draggedHeaderFieldIndex = index;
	}

	function handleHeaderFieldDragOver(e: DragEvent, index: number) {
		e.preventDefault();
		dragOverHeaderFieldIndex = index;
	}

	function handleHeaderFieldDragLeave() {
		dragOverHeaderFieldIndex = null;
	}

	function handleHeaderFieldDrop(e: DragEvent, dropIndex: number) {
		e.preventDefault();
		if (draggedHeaderFieldIndex === null || draggedHeaderFieldIndex === dropIndex || !editorLayout) {
			draggedHeaderFieldIndex = null;
			dragOverHeaderFieldIndex = null;
			return;
		}

		const headerFields = [...editorLayout.header.fields];
		const [dragged] = headerFields.splice(draggedHeaderFieldIndex, 1);
		headerFields.splice(dropIndex, 0, dragged);

		mutate((l) => { l.header.fields = headerFields; });

		draggedHeaderFieldIndex = null;
		dragOverHeaderFieldIndex = null;
	}

	function handleHeaderFieldDragEnd() {
		draggedHeaderFieldIndex = null;
		dragOverHeaderFieldIndex = null;
	}

	// ---- Sidebar card operations ----

	function addSidebarCard() {
		if (!editorLayout) return;
		const maxOrder = editorLayout.sidebar.cards.length > 0
			? Math.max(...editorLayout.sidebar.cards.map((c) => c.order))
			: 0;
		const newCard: LayoutSidebarCardV3 = {
			id: 'card_' + Math.random().toString(36).substr(2, 9),
			label: 'New Card',
			order: maxOrder + 1,
			fields: []
		};
		mutate((l) => {
			l.sidebar.cards = [...l.sidebar.cards, newCard];
		});
		expandedCards = new Set([...expandedCards, newCard.id]);
	}

	function deleteSidebarCard(cardId: string) {
		if (!editorLayout) return;
		const card = editorLayout.sidebar.cards.find((c) => c.id === cardId);
		if (!card) return;

		if (!confirm(`Delete card "${card.label}"?`)) return;

		mutate((l) => {
			l.sidebar.cards = l.sidebar.cards.filter((c) => c.id !== cardId);
			// Renumber order
			l.sidebar.cards.sort((a, b) => a.order - b.order).forEach((c, i) => { c.order = i + 1; });
		});
		const next = new Set(expandedCards);
		next.delete(cardId);
		expandedCards = next;
	}

	function updateCardLabel(cardId: string, label: string) {
		mutate((l) => {
			const card = l.sidebar.cards.find((c) => c.id === cardId);
			if (card) card.label = label;
		});
	}

	function addFieldToCard(cardId: string, fieldName: string) {
		mutate((l) => {
			const card = l.sidebar.cards.find((c) => c.id === cardId);
			if (card && !card.fields.includes(fieldName)) {
				card.fields = [...card.fields, fieldName];
			}
		});
	}

	function removeFieldFromCard(cardId: string, fieldName: string) {
		mutate((l) => {
			const card = l.sidebar.cards.find((c) => c.id === cardId);
			if (card) {
				card.fields = card.fields.filter((f) => f !== fieldName);
			}
		});
	}

	function toggleCardExpand(cardId: string) {
		const next = new Set(expandedCards);
		if (next.has(cardId)) {
			next.delete(cardId);
		} else {
			next.add(cardId);
		}
		expandedCards = next;
	}

	// ---- Sidebar card drag-and-drop ----

	function handleCardDragStart(index: number) {
		draggedCardIndex = index;
	}

	function handleCardDragOver(e: DragEvent, index: number) {
		e.preventDefault();
		dragOverCardIndex = index;
	}

	function handleCardDragLeave() {
		dragOverCardIndex = null;
	}

	function handleCardDrop(e: DragEvent, dropIndex: number) {
		e.preventDefault();
		if (draggedCardIndex === null || draggedCardIndex === dropIndex || !editorLayout) {
			draggedCardIndex = null;
			dragOverCardIndex = null;
			return;
		}

		const cards = [...editorLayout.sidebar.cards].sort((a, b) => a.order - b.order);
		const [dragged] = cards.splice(draggedCardIndex, 1);
		cards.splice(dropIndex, 0, dragged);
		// Renumber order
		cards.forEach((c, i) => { c.order = i + 1; });

		mutate((l) => { l.sidebar.cards = cards; });

		draggedCardIndex = null;
		dragOverCardIndex = null;
	}

	function handleCardDragEnd() {
		draggedCardIndex = null;
		dragOverCardIndex = null;
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

	// Fields available to add to header (all fields — header can show any field)
	function availableHeaderFields(): FieldDef[] {
		if (!editorLayout) return [];
		return fields.filter((f) => !editorLayout!.header.fields.includes(f.name));
	}

	// Card type selector state
	let relatedListConfigs = $state<RelatedListConfig[]>([]);
	let showCardTypeSelector = $state(false);
	let cardTypeSelectorTarget = $state<'new' | number>('new');

	// Fields available to add to a sidebar card (all fields — sidebar can show any field)
	function availableCardFields(cardId: string): FieldDef[] {
		if (!editorLayout) return [];
		const card = editorLayout.sidebar.cards.find((c) => c.id === cardId);
		if (!card) return [];
		return fields.filter((f) => !card.fields.includes(f.name));
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
					Configure tabs, header fields, sidebar cards, and sections. Save when done.
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

		<!-- =========================================================== -->
		<!-- TABS PANEL                                                   -->
		<!-- =========================================================== -->
		<div class="bg-white shadow rounded-lg border border-gray-200">
			<!-- Panel header -->
			<div class="flex items-center justify-between px-4 py-3 border-b border-gray-100">
				<div class="flex items-center gap-2">
					<button
						type="button"
						onclick={() => (tabsPanelCollapsed = !tabsPanelCollapsed)}
						class="text-gray-400 hover:text-gray-600"
						title={tabsPanelCollapsed ? 'Expand' : 'Collapse'}
					>
						<svg
							class="w-5 h-5 transition-transform {tabsPanelCollapsed ? '-rotate-90' : ''}"
							fill="none"
							stroke="currentColor"
							viewBox="0 0 24 24"
						>
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
						</svg>
					</button>
					<h2 class="text-sm font-semibold text-gray-900">Tabs</h2>
					<span class="text-xs px-2 py-0.5 bg-gray-100 text-gray-600 rounded-full">
						{editorLayout.tabs.length} tab{editorLayout.tabs.length !== 1 ? 's' : ''}
					</span>
				</div>
				<button
					type="button"
					onclick={addTab}
					class="text-xs px-3 py-1.5 bg-blue-600 text-white rounded hover:bg-blue-700 transition-colors"
				>
					+ Add Tab
				</button>
			</div>

			{#if !tabsPanelCollapsed}
				<div class="p-4 space-y-2">
					{#if editorLayout.tabs.length === 0}
						<p class="text-sm text-gray-400 italic text-center py-3">
							No tabs defined. Add a tab to organize your sections.
						</p>
					{:else}
						{#each [...editorLayout.tabs].sort((a, b) => a.order - b.order) as tab, tabIndex (tab.id)}
							{@const isTabDragging = draggedTabIndex === tabIndex}
							{@const isTabDropTarget = dragOverTabIndex === tabIndex && draggedTabIndex !== tabIndex}

							<div
								draggable="true"
								ondragstart={() => handleTabDragStart(tabIndex)}
								ondragover={(e) => handleTabDragOver(e, tabIndex)}
								ondragleave={handleTabDragLeave}
								ondrop={(e) => handleTabDrop(e, tabIndex)}
								ondragend={handleTabDragEnd}
								class="flex items-center gap-3 px-3 py-2 bg-gray-50 rounded border transition-all
									{isTabDragging ? 'opacity-50 border-blue-300' : 'border-gray-200'}
									{isTabDropTarget ? 'border-blue-500 bg-blue-50' : ''}"
							>
								<!-- Drag handle -->
								<div class="flex-shrink-0 cursor-move text-gray-400 hover:text-gray-600">
									<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8h16M4 16h16" />
									</svg>
								</div>

								<!-- Tab label input -->
								<input
									type="text"
									value={tab.label}
									oninput={(e) => updateTabLabel(tab.id, (e.target as HTMLInputElement).value)}
									class="flex-1 text-sm font-medium text-gray-900 bg-transparent border-0 focus:outline-none focus:ring-1 focus:ring-blue-500 rounded px-1 py-0.5"
									placeholder="Tab name"
								/>

								<!-- Section count badge -->
								<span class="flex-shrink-0 text-xs px-2 py-0.5 bg-blue-100 text-blue-700 rounded-full">
									{tab.sectionIds.length} section{tab.sectionIds.length !== 1 ? 's' : ''}
								</span>

								<!-- Delete tab -->
								<button
									type="button"
									onclick={() => deleteTab(tab.id)}
									class="flex-shrink-0 text-gray-400 hover:text-red-500"
									title="Delete tab"
								>
									<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
									</svg>
								</button>
							</div>
						{/each}
					{/if}
				</div>
			{/if}
		</div>

		<!-- Unassigned sections warning -->
		{#if unassignedSections().length > 0}
			<div class="bg-yellow-50 border border-yellow-200 rounded-lg p-4">
				<h3 class="text-sm font-medium text-yellow-800 mb-2">
					{unassignedSections().length} section{unassignedSections().length !== 1 ? 's' : ''} not assigned to any tab:
				</h3>
				<div class="flex flex-wrap gap-2">
					{#each unassignedSections() as section (section.id)}
						<span class="px-2 py-1 text-xs bg-yellow-100 text-yellow-800 rounded">
							{section.label}
						</span>
					{/each}
				</div>
				<p class="mt-2 text-xs text-yellow-700">
					Unassigned sections won't appear on any tab. Use the tab dropdown on each section to assign it.
				</p>
			</div>
		{/if}

		<!-- =========================================================== -->
		<!-- HEADER FIELDS PANEL                                          -->
		<!-- =========================================================== -->
		<div class="bg-white shadow rounded-lg border border-gray-200">
			<!-- Panel header -->
			<div class="flex items-center gap-2 px-4 py-3 border-b border-gray-100">
				<button
					type="button"
					onclick={() => (headerPanelCollapsed = !headerPanelCollapsed)}
					class="text-gray-400 hover:text-gray-600"
					title={headerPanelCollapsed ? 'Expand' : 'Collapse'}
				>
					<svg
						class="w-5 h-5 transition-transform {headerPanelCollapsed ? '-rotate-90' : ''}"
						fill="none"
						stroke="currentColor"
						viewBox="0 0 24 24"
					>
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
					</svg>
				</button>
				<div>
					<h2 class="text-sm font-semibold text-gray-900">Header Fields</h2>
					<p class="text-xs text-gray-500">Key fields displayed above tabs on the record page</p>
				</div>
			</div>

			{#if !headerPanelCollapsed}
				<div class="p-4">
					<div class="grid grid-cols-2 gap-4">
						<!-- Available fields -->
						<div>
							<h3 class="text-xs font-medium text-gray-600 uppercase tracking-wide mb-2">Available Fields</h3>
							<div class="space-y-1 max-h-60 overflow-y-auto">
								{#if availableHeaderFields().length === 0}
									<p class="text-xs text-gray-400 italic py-2">All fields added to header</p>
								{:else}
									{#each availableHeaderFields() as field (field.name)}
										<div class="flex items-center gap-2 px-2 py-1.5 rounded hover:bg-gray-50">
											<span class="flex-1 text-sm text-gray-800">{field.label}</span>
											{#if field.type}
												<span class="text-xs px-1.5 py-0.5 bg-gray-100 text-gray-500 rounded">{field.type}</span>
											{/if}
											<button
												type="button"
												onclick={() => addHeaderField(field.name)}
												class="text-xs px-2 py-0.5 bg-blue-600 text-white rounded hover:bg-blue-700 transition-colors"
											>
												Add &gt;
											</button>
										</div>
									{/each}
								{/if}
							</div>
						</div>

						<!-- Selected fields -->
						<div>
							<h3 class="text-xs font-medium text-gray-600 uppercase tracking-wide mb-2">
								Selected Fields ({editorLayout.header.fields.length})
							</h3>
							<div class="space-y-1 max-h-60 overflow-y-auto">
								{#if editorLayout.header.fields.length === 0}
									<p class="text-xs text-gray-400 italic py-2">No header fields selected</p>
								{:else}
									{#each editorLayout.header.fields as fieldName, hfIndex (fieldName)}
										{@const isHFDragging = draggedHeaderFieldIndex === hfIndex}
										{@const isHFDropTarget = dragOverHeaderFieldIndex === hfIndex && draggedHeaderFieldIndex !== hfIndex}

										<div
											draggable="true"
											ondragstart={() => handleHeaderFieldDragStart(hfIndex)}
											ondragover={(e) => handleHeaderFieldDragOver(e, hfIndex)}
											ondragleave={handleHeaderFieldDragLeave}
											ondrop={(e) => handleHeaderFieldDrop(e, hfIndex)}
											ondragend={handleHeaderFieldDragEnd}
											class="flex items-center gap-2 px-2 py-1.5 rounded border transition-all
												{isHFDragging ? 'opacity-50 border-blue-300 bg-blue-50' : 'border-transparent hover:border-gray-200 hover:bg-gray-50'}
												{isHFDropTarget ? 'border-blue-500 bg-blue-50' : ''}"
										>
											<!-- Drag handle -->
											<div class="flex-shrink-0 cursor-move text-gray-300 hover:text-gray-500">
												<svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
													<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8h16M4 16h16" />
												</svg>
											</div>
											<span class="flex-1 text-sm text-gray-800">{getFieldLabel(fieldName)}</span>
											{#if getFieldType(fieldName)}
												<span class="text-xs px-1.5 py-0.5 bg-gray-100 text-gray-500 rounded">{getFieldType(fieldName)}</span>
											{/if}
											<button
												type="button"
												onclick={() => removeHeaderField(fieldName)}
												class="text-xs px-2 py-0.5 bg-gray-100 text-gray-600 rounded hover:bg-red-100 hover:text-red-700 transition-colors"
											>
												&lt; Remove
											</button>
										</div>
									{/each}
								{/if}
							</div>
						</div>
					</div>
				</div>
			{/if}
		</div>

		<!-- =========================================================== -->
		<!-- SIDEBAR CARDS PANEL                                          -->
		<!-- =========================================================== -->
		<div class="bg-white shadow rounded-lg border border-gray-200">
			<!-- Panel header -->
			<div class="flex items-center justify-between px-4 py-3 border-b border-gray-100">
				<div class="flex items-center gap-2">
					<button
						type="button"
						onclick={() => (sidebarPanelCollapsed = !sidebarPanelCollapsed)}
						class="text-gray-400 hover:text-gray-600"
						title={sidebarPanelCollapsed ? 'Expand' : 'Collapse'}
					>
						<svg
							class="w-5 h-5 transition-transform {sidebarPanelCollapsed ? '-rotate-90' : ''}"
							fill="none"
							stroke="currentColor"
							viewBox="0 0 24 24"
						>
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
						</svg>
					</button>
					<div>
						<h2 class="text-sm font-semibold text-gray-900">Sidebar Cards</h2>
						<p class="text-xs text-gray-500">Info cards shown in the right sidebar on the record page</p>
					</div>
				</div>
				<button
					type="button"
					onclick={addSidebarCard}
					class="text-xs px-3 py-1.5 bg-blue-600 text-white rounded hover:bg-blue-700 transition-colors"
				>
					+ Add Card
				</button>
			</div>

			{#if !sidebarPanelCollapsed}
				<div class="p-4 space-y-2">
					{#if editorLayout.sidebar.cards.length === 0}
						<p class="text-sm text-gray-400 italic text-center py-3">
							No sidebar cards. Add a card to display fields in the sidebar.
						</p>
					{:else}
						{#each [...editorLayout.sidebar.cards].sort((a, b) => a.order - b.order) as card, cardIndex (card.id)}
							{@const isCardDragging = draggedCardIndex === cardIndex}
							{@const isCardDropTarget = dragOverCardIndex === cardIndex && draggedCardIndex !== cardIndex}
							{@const isCardExpanded = expandedCards.has(card.id)}

							<div
								class="border rounded-lg transition-all
									{isCardDragging ? 'opacity-50 border-blue-300' : 'border-gray-200'}
									{isCardDropTarget ? 'border-blue-500' : ''}"
							>
								<!-- Card header row -->
								<div
									draggable="true"
									ondragstart={() => handleCardDragStart(cardIndex)}
									ondragover={(e) => handleCardDragOver(e, cardIndex)}
									ondragleave={handleCardDragLeave}
									ondrop={(e) => handleCardDrop(e, cardIndex)}
									ondragend={handleCardDragEnd}
									class="flex items-center gap-3 px-3 py-2 {isCardExpanded ? 'border-b border-gray-100' : ''}"
								>
									<!-- Drag handle -->
									<div class="flex-shrink-0 cursor-move text-gray-400 hover:text-gray-600">
										<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
											<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8h16M4 16h16" />
										</svg>
									</div>

									<!-- Card label input -->
									<input
										type="text"
										value={card.label}
										oninput={(e) => updateCardLabel(card.id, (e.target as HTMLInputElement).value)}
										class="flex-1 text-sm font-medium text-gray-900 bg-transparent border-0 focus:outline-none focus:ring-1 focus:ring-blue-500 rounded px-1 py-0.5"
										placeholder="Card name"
									/>

									<!-- Field count badge -->
									<span class="flex-shrink-0 text-xs px-2 py-0.5 bg-purple-100 text-purple-700 rounded-full">
										{card.fields.length} field{card.fields.length !== 1 ? 's' : ''}
									</span>

									<!-- Expand/collapse toggle -->
									<button
										type="button"
										onclick={() => toggleCardExpand(card.id)}
										class="flex-shrink-0 text-gray-400 hover:text-gray-600"
										title={isCardExpanded ? 'Collapse card' : 'Expand card'}
									>
										<svg
											class="w-4 h-4 transition-transform {isCardExpanded ? 'rotate-180' : ''}"
											fill="none"
											stroke="currentColor"
											viewBox="0 0 24 24"
										>
											<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
										</svg>
									</button>

									<!-- Delete card -->
									<button
										type="button"
										onclick={() => deleteSidebarCard(card.id)}
										class="flex-shrink-0 text-gray-400 hover:text-red-500"
										title="Delete card"
									>
										<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
											<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
										</svg>
									</button>
								</div>

								<!-- Expanded: field picker for this card -->
								{#if isCardExpanded}
									<div class="p-3">
										<div class="grid grid-cols-2 gap-4">
											<!-- Available fields -->
											<div>
												<h4 class="text-xs font-medium text-gray-600 uppercase tracking-wide mb-2">Available Fields</h4>
												<div class="space-y-1 max-h-48 overflow-y-auto">
													{#if availableCardFields(card.id).length === 0}
														<p class="text-xs text-gray-400 italic py-2">All fields added to card</p>
													{:else}
														{#each availableCardFields(card.id) as field (field.name)}
															<div class="flex items-center gap-2 px-2 py-1 rounded hover:bg-gray-50">
																<span class="flex-1 text-xs text-gray-800">{field.label}</span>
																{#if field.type}
																	<span class="text-xs px-1 py-0.5 bg-gray-100 text-gray-500 rounded">{field.type}</span>
																{/if}
																<button
																	type="button"
																	onclick={() => addFieldToCard(card.id, field.name)}
																	class="text-xs px-1.5 py-0.5 bg-blue-600 text-white rounded hover:bg-blue-700 transition-colors"
																>
																	Add &gt;
																</button>
															</div>
														{/each}
													{/if}
												</div>
											</div>

											<!-- Selected fields -->
											<div>
												<h4 class="text-xs font-medium text-gray-600 uppercase tracking-wide mb-2">
													Selected ({card.fields.length})
												</h4>
												<div class="space-y-1 max-h-48 overflow-y-auto">
													{#if card.fields.length === 0}
														<p class="text-xs text-gray-400 italic py-2">No fields in this card</p>
													{:else}
														{#each card.fields as fieldName (fieldName)}
															<div class="flex items-center gap-2 px-2 py-1 rounded hover:bg-gray-50">
																<span class="flex-1 text-xs text-gray-800">{getFieldLabel(fieldName)}</span>
																{#if getFieldType(fieldName)}
																	<span class="text-xs px-1 py-0.5 bg-gray-100 text-gray-500 rounded">{getFieldType(fieldName)}</span>
																{/if}
																<button
																	type="button"
																	onclick={() => removeFieldFromCard(card.id, fieldName)}
																	class="text-xs px-1.5 py-0.5 bg-gray-100 text-gray-600 rounded hover:bg-red-100 hover:text-red-700 transition-colors"
																>
																	Remove
																</button>
															</div>
														{/each}
													{/if}
												</div>
											</div>
										</div>
									</div>
								{/if}
							</div>
						{/each}
					{/if}
				</div>
			{/if}
		</div>

		<!-- =========================================================== -->
		<!-- SECTIONS LIST                                                -->
		<!-- =========================================================== -->
		<div>
			<div class="flex items-center justify-between mb-3">
				<h2 class="text-sm font-semibold text-gray-900">Sections</h2>
				<p class="text-xs text-gray-500">Drag to reorder. Use dropdown to assign to a tab.</p>
			</div>

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

							<!-- Tab assignment dropdown -->
							{#if editorLayout.tabs.length > 0}
								<select
									value={getSectionTabId(section.id)}
									onchange={(e) => moveSectionToTab(section.id, (e.target as HTMLSelectElement).value)}
									class="flex-shrink-0 text-xs border border-gray-200 rounded px-2 py-1 text-gray-700 focus:outline-none focus:ring-1 focus:ring-blue-500"
									title="Assign to tab"
								>
									<option value="">Unassigned</option>
									{#each [...editorLayout.tabs].sort((a, b) => a.order - b.order) as tab (tab.id)}
										<option value={tab.id}>{tab.label}</option>
									{/each}
								</select>
							{/if}

							<!-- Card type badge (for non-field cards) -->
							{#if section.cardType && section.cardType !== 'field'}
								<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium {
									section.cardType === 'activity' ? 'bg-green-100 text-green-700' :
									section.cardType === 'relatedList' ? 'bg-purple-100 text-purple-700' :
									'bg-amber-100 text-amber-700'
								}">
									{section.cardType === 'activity' ? 'Activity' : section.cardType === 'relatedList' ? 'Related List' : 'Custom Page'}
								</span>
							{/if}

							<!-- Column toggle (field cards only) -->
							{#if !section.cardType || section.cardType === 'field'}
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
											{section.columns === 2
												? 'bg-blue-600 text-white'
												: 'text-gray-600 hover:bg-gray-100'}"
										title="Two columns"
									>
										2 col
									</button>
									<button
										type="button"
										onclick={() => toggleColumns(index, 3)}
										class="px-2.5 py-1 text-xs font-medium transition-colors border-l border-gray-200
											{section.columns === 3
												? 'bg-blue-600 text-white'
												: 'text-gray-600 hover:bg-gray-100'}"
										title="Three columns"
									>
										3 col
									</button>
								</div>
							{/if}

							<!-- Field count badge (field cards only) -->
							{#if !section.cardType || section.cardType === 'field'}
								<span class="flex-shrink-0 text-xs text-gray-400 tabular-nums">
									{section.fields.length} field{section.fields.length !== 1 ? 's' : ''}
								</span>
							{/if}

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

							<!-- Change card type -->
							<button
								type="button"
								onclick={() => openChangeCardType(index)}
								class="flex-shrink-0 text-xs text-gray-400 hover:text-gray-600 px-1.5 py-0.5 rounded hover:bg-gray-100"
								title="Change card type"
							>
								Change Type
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

						<!-- Type-specific config panels (non-field cards) -->
						{#if isExpanded && section.cardType === 'relatedList'}
							{@const rlConfig = (section.cardConfig ?? { relatedListConfigId: '' }) as RelatedListCardConfig}
							<div class="px-4 py-3 bg-purple-50 border-b border-purple-100">
								<label class="block text-xs font-medium text-gray-600 mb-1">Related Entity</label>
								<select
									class="w-full text-sm border border-gray-300 rounded px-2 py-1.5"
									value={rlConfig.relatedListConfigId}
									onchange={(e) => {
										mutate((l) => {
											const cfg = l.sections[index].cardConfig as RelatedListCardConfig;
											if (cfg) cfg.relatedListConfigId = (e.target as HTMLSelectElement).value;
											else l.sections[index].cardConfig = { relatedListConfigId: (e.target as HTMLSelectElement).value };
										});
									}}
								>
									<option value="">-- Select related entity --</option>
									{#each relatedListConfigs.filter(c => c.enabled) as rlc (rlc.id)}
										<option value={rlc.id}>{rlc.label} ({rlc.relatedEntity})</option>
									{/each}
								</select>
							</div>
						{/if}

						{#if isExpanded && section.cardType === 'customPage'}
							{@const cpConfig = (section.cardConfig ?? { mode: 'iframe', url: '', height: 400 }) as CustomPageCardConfig}
							<div class="px-4 py-3 bg-amber-50 border-b border-amber-100 space-y-2">
								<div class="flex gap-2">
									<button
										type="button"
										onclick={() => mutate(l => { (l.sections[index].cardConfig as CustomPageCardConfig).mode = 'iframe'; })}
										class="px-3 py-1 text-xs rounded {cpConfig.mode === 'iframe' ? 'bg-amber-200 text-amber-800' : 'bg-white text-gray-600 border border-gray-300'}"
									>
										Iframe
									</button>
									<button
										type="button"
										onclick={() => mutate(l => { (l.sections[index].cardConfig as CustomPageCardConfig).mode = 'html'; })}
										class="px-3 py-1 text-xs rounded {cpConfig.mode === 'html' ? 'bg-amber-200 text-amber-800' : 'bg-white text-gray-600 border border-gray-300'}"
									>
										HTML
									</button>
								</div>
								{#if cpConfig.mode === 'iframe'}
									<div>
										<label class="block text-xs font-medium text-gray-600 mb-1">URL</label>
										<input type="text" class="w-full text-sm border border-gray-300 rounded px-2 py-1.5" placeholder={'https://example.com/embed/{recordId}'}
											value={cpConfig.url ?? ''}
											oninput={(e) => mutate(l => { (l.sections[index].cardConfig as CustomPageCardConfig).url = (e.target as HTMLInputElement).value; })}
										/>
									</div>
									<div>
										<label class="block text-xs font-medium text-gray-600 mb-1">Height (px)</label>
										<input type="number" class="w-32 text-sm border border-gray-300 rounded px-2 py-1.5" placeholder="400"
											value={cpConfig.height ?? 400}
											oninput={(e) => mutate(l => { (l.sections[index].cardConfig as CustomPageCardConfig).height = parseInt((e.target as HTMLInputElement).value) || 400; })}
										/>
									</div>
								{:else}
									<div>
										<label class="block text-xs font-medium text-gray-600 mb-1">HTML Content <span class="text-gray-400">(admin-trusted)</span></label>
										<textarea class="w-full text-sm border border-gray-300 rounded px-2 py-1.5 font-mono" rows="6" placeholder="<div>Your custom HTML here</div>"
											value={cpConfig.content ?? ''}
											oninput={(e) => mutate(l => { (l.sections[index].cardConfig as CustomPageCardConfig).content = (e.target as HTMLTextAreaElement).value; })}
										></textarea>
									</div>
								{/if}
							</div>
						{/if}

						{#if isExpanded && section.cardType === 'activity'}
							<div class="px-4 py-3 bg-green-50 border-b border-green-100">
								<p class="text-sm text-green-700">Activity Card — displays tasks, calls, and meetings linked to this record. No additional configuration required.</p>
							</div>
						{/if}

						<!-- Expanded: field list + add field (field cards only) -->
						{#if isExpanded && (!section.cardType || section.cardType === 'field')}
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
			<div class="mt-3">
				<button
					type="button"
					onclick={openAddSection}
					class="w-full px-4 py-3 border-2 border-dashed border-gray-300 text-gray-500 rounded-lg hover:border-blue-400 hover:text-blue-600 transition-colors text-sm font-medium"
				>
					+ Add Section
				</button>
			</div>
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

	<!-- Card Type Selector Modal -->
	{#if showCardTypeSelector}
		<div
			class="fixed inset-0 bg-black/40 z-50 flex items-center justify-center"
			onclick={() => (showCardTypeSelector = false)}
			onkeydown={(e) => e.key === 'Escape' && (showCardTypeSelector = false)}
			role="dialog"
			tabindex="-1"
		>
			<div class="bg-white rounded-xl shadow-xl p-6 w-full max-w-lg" onclick={(e) => e.stopPropagation()}>
				<h3 class="text-lg font-semibold text-gray-900 mb-4">
					{cardTypeSelectorTarget === 'new' ? 'Add Section' : 'Change Card Type'}
				</h3>
				<div class="grid grid-cols-2 gap-3">
					<!-- Field Card -->
					<button onclick={() => selectCardType('field')} class="flex flex-col items-center gap-2 p-4 rounded-lg border-2 border-gray-200 hover:border-blue-500 hover:bg-blue-50 transition-colors text-center">
						<div class="w-10 h-10 rounded-lg bg-blue-100 flex items-center justify-center">
							<svg class="w-5 h-5 text-blue-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
								<path stroke-linecap="round" stroke-linejoin="round" d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25H12" />
							</svg>
						</div>
						<span class="text-sm font-medium text-gray-900">Field Card</span>
						<span class="text-xs text-gray-500">Display record fields</span>
					</button>
					<!-- Activity Card -->
					<button onclick={() => selectCardType('activity')} class="flex flex-col items-center gap-2 p-4 rounded-lg border-2 border-gray-200 hover:border-green-500 hover:bg-green-50 transition-colors text-center">
						<div class="w-10 h-10 rounded-lg bg-green-100 flex items-center justify-center">
							<svg class="w-5 h-5 text-green-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
								<path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z" />
							</svg>
						</div>
						<span class="text-sm font-medium text-gray-900">Activity Card</span>
						<span class="text-xs text-gray-500">Tasks, calls &amp; meetings</span>
					</button>
					<!-- Related List Card -->
					<button onclick={() => selectCardType('relatedList')} class="flex flex-col items-center gap-2 p-4 rounded-lg border-2 border-gray-200 hover:border-purple-500 hover:bg-purple-50 transition-colors text-center">
						<div class="w-10 h-10 rounded-lg bg-purple-100 flex items-center justify-center">
							<svg class="w-5 h-5 text-purple-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
								<path stroke-linecap="round" stroke-linejoin="round" d="M3.75 12h16.5m-16.5 3.75h16.5M3.75 19.5h16.5M5.625 4.5h12.75a1.875 1.875 0 010 3.75H5.625a1.875 1.875 0 010-3.75z" />
							</svg>
						</div>
						<span class="text-sm font-medium text-gray-900">Related List</span>
						<span class="text-xs text-gray-500">Inline related records</span>
					</button>
					<!-- Custom Page Card -->
					<button onclick={() => selectCardType('customPage')} class="flex flex-col items-center gap-2 p-4 rounded-lg border-2 border-gray-200 hover:border-amber-500 hover:bg-amber-50 transition-colors text-center">
						<div class="w-10 h-10 rounded-lg bg-amber-100 flex items-center justify-center">
							<svg class="w-5 h-5 text-amber-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
								<path stroke-linecap="round" stroke-linejoin="round" d="M17.25 6.75L22.5 12l-5.25 5.25m-10.5 0L1.5 12l5.25-5.25m7.5-3l-4.5 16.5" />
							</svg>
						</div>
						<span class="text-sm font-medium text-gray-900">Custom Page</span>
						<span class="text-xs text-gray-500">Iframe or HTML embed</span>
					</button>
				</div>
				<button onclick={() => (showCardTypeSelector = false)} class="mt-4 w-full py-2 text-sm text-gray-500 hover:text-gray-700">
					Cancel
				</button>
			</div>
		</div>
	{/if}
</div>
