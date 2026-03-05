<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { beforeNavigate } from '$app/navigation';
	import { get, put } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import type { EntityDef, FieldDef } from '$lib/types/admin';
	import type { LayoutDataV3, LayoutSectionV2, LayoutFieldV2, LayoutTabV3, LayoutV3Response, SectionCardType, SectionCardV3, RelatedListCardConfig, CustomPageCardConfig } from '$lib/types/layout';
	import { createDefaultVisibility, createDefaultField, createDefaultCard, migrateSectionToCards } from '$lib/types/layout';
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

	// Per-card field drag state
	let draggedFieldInfo = $state<{ sectionIndex: number; cardIndex: number; fieldIndex: number } | null>(null);
	let dragOverFieldInfo = $state<{ sectionIndex: number; cardIndex: number; fieldIndex: number } | null>(null);

	// Tab drag state
	let draggedTabIndex = $state<number | null>(null);
	let dragOverTabIndex = $state<number | null>(null);

	// Section card drag state (between columns)
	let draggedSectionCardId = $state<string | null>(null);

	// Sidebar card drag state
	let draggedCardIndex = $state<number | null>(null);
	let dragOverCardIndex = $state<number | null>(null);

	// Header field drag state
	let draggedHeaderFieldIndex = $state<number | null>(null);
	let dragOverHeaderFieldIndex = $state<number | null>(null);

	// UI state — which sections are expanded
	let expandedSections = $state<Set<string>>(new Set());

	// Which cards within sections are expanded
	let expandedCardIds = $state<Set<string>>(new Set());

	// Field picker dropdown state
	let fieldPickerCardId = $state<string | null>(null);
	let fieldPickerSearch = $state('');

	// Panel collapse state
	let tabsPanelCollapsed = $state(false);
	let headerPanelCollapsed = $state(false);
	let sidebarPanelCollapsed = $state(false);

	// Sidebar card expanded state
	let expandedCards = $state<Set<string>>(new Set());

	// Track all used field names across all sections' cards
	let usedFieldNames = $derived(() => {
		if (!editorLayout) return new Set<string>();
		const used = new Set<string>();
		for (const section of editorLayout.sections) {
			for (const card of section.cards ?? []) {
				for (const field of card.fields ?? []) {
					used.add(field.name);
				}
			}
		}
		return used;
	});

	// Fields not assigned to any card
	let unassignedFields = $derived(fields.filter((f) => !usedFieldNames().has(f.name)));

	// Sections not assigned to any tab
	let unassignedSections = $derived(() => {
		if (!editorLayout) return [];
		return editorLayout.sections.filter(
			(s) => !editorLayout!.tabs.some((t) => t.sectionIds.includes(s.id))
		);
	});

	// Navigation guard
	beforeNavigate(({ cancel }) => {
		if (isDirty && !confirm('You have unsaved changes. Leave anyway?')) cancel();
	});

	function handleBeforeUnload(e: BeforeUnloadEvent) {
		if (isDirty) {
			e.preventDefault();
		}
	}

	function handleDocumentClick() {
		if (fieldPickerCardId) closeFieldPicker();
	}

	onMount(() => {
		window.addEventListener('beforeunload', handleBeforeUnload);
		document.addEventListener('click', handleDocumentClick);
		loadData();
	});

	onDestroy(() => {
		window.removeEventListener('beforeunload', handleBeforeUnload);
		document.removeEventListener('click', handleDocumentClick);
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

			// Deep clone and migrate all sections to cards format
			const rawLayout = structuredClone(layoutResponse.layout);
			rawLayout.sections = rawLayout.sections.map(migrateSectionToCards);
			editorLayout = rawLayout;
			isDirty = false;

			// Start with all sections expanded
			expandedSections = new Set(editorLayout.sections.map((s) => s.id));
			expandedCardIds = new Set();
			expandedCards = new Set();

			// Fetch related list configs
			try {
				relatedListConfigs = await get<RelatedListConfig[]>(`/entities/${entityName}/related-list-configs`);
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

			// Normalize: clear deprecated fields, only use cards
			const toSave = JSON.parse(JSON.stringify($state.snapshot(editorLayout))) as LayoutDataV3;
			for (const section of toSave.sections) {
				// Clear deprecated single-card fields
				delete (section as unknown as Record<string, unknown>).cardType;
				delete (section as unknown as Record<string, unknown>).cardConfig;
				// Keep section.fields empty (all fields live in cards now)
				section.fields = [];
			}

			await put(`/admin/entities/${entityName}/layouts/detail/v3`, toSave);
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
			columns: 1,
			visibility: createDefaultVisibility(),
			fields: [],
			cards: []
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
		if (!confirm('Delete this section and all its cards?')) return;

		const sectionId = editorLayout.sections[index].id;
		mutate((l) => {
			l.sections = l.sections.filter((_, i) => i !== index);
			l.sections.forEach((s, i) => { s.order = i + 1; });
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

	function toggleSectionColumns(index: number, columns: 1 | 2 | 3) {
		mutate((l) => {
			l.sections[index].columns = columns;
			// Clamp card columns that exceed the new column count
			for (const card of l.sections[index].cards ?? []) {
				if ((card.column ?? 1) > columns) card.column = 1;
			}
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

	// ---- Card operations within sections ----

	// Card type selector
	let showCardTypeSelector = $state(false);
	let cardTypeSelectorSectionIndex = $state<number>(0);
	let cardTypeSelectorColumn = $state<number>(1);
	let cardTypeSelectorTarget = $state<'section' | 'sidebar'>('section');
	let relatedListConfigs = $state<RelatedListConfig[]>([]);

	function openAddCard(sectionIndex: number, column: number = 1) {
		cardTypeSelectorSectionIndex = sectionIndex;
		cardTypeSelectorColumn = column;
		cardTypeSelectorTarget = 'section';
		showCardTypeSelector = true;
	}

	function selectCardTypeForSection(cardType: SectionCardType) {
		showCardTypeSelector = false;
		if (cardTypeSelectorTarget === 'sidebar') {
			addSidebarCardOfType(cardType);
		} else {
			addCardToSection(cardTypeSelectorSectionIndex, cardType, cardTypeSelectorColumn);
		}
	}

	function addCardToSection(sectionIndex: number, cardType: SectionCardType, column: number = 1) {
		mutate((l) => {
			const section = l.sections[sectionIndex];
			if (!section.cards) section.cards = [];
			const order = section.cards.length + 1;
			const card = createDefaultCard(cardType, order);
			card.label = cardType === 'activity' ? 'Activities' : cardType === 'relatedList' ? 'Related Records' : cardType === 'customPage' ? 'Custom Content' : 'Fields';
			card.column = column;
			section.cards = [...section.cards, card];
		});
		// Expand the new card
		const section = editorLayout?.sections[sectionIndex];
		if (section?.cards) {
			const newCard = section.cards[section.cards.length - 1];
			expandedCardIds = new Set([...expandedCardIds, newCard.id]);
		}
	}

	function deleteCard(sectionIndex: number, cardIndex: number) {
		if (!confirm('Delete this card?')) return;
		mutate((l) => {
			const section = l.sections[sectionIndex];
			if (!section.cards) return;
			const cardId = section.cards[cardIndex].id;
			section.cards = section.cards.filter((_, i) => i !== cardIndex);
			section.cards.forEach((c, i) => { c.order = i + 1; });
			const next = new Set(expandedCardIds);
			next.delete(cardId);
			expandedCardIds = next;
		});
	}

	function updateCardLabel(sectionIndex: number, cardIndex: number, label: string) {
		mutate((l) => {
			const card = l.sections[sectionIndex].cards?.[cardIndex];
			if (card) card.label = label;
		});
	}

	function toggleFieldPicker(cardId: string) {
		if (fieldPickerCardId === cardId) {
			fieldPickerCardId = null;
			fieldPickerSearch = '';
		} else {
			fieldPickerCardId = cardId;
			fieldPickerSearch = '';
		}
	}

	function closeFieldPicker() {
		fieldPickerCardId = null;
		fieldPickerSearch = '';
	}

	function toggleCardExpand(cardId: string) {
		const next = new Set(expandedCardIds);
		if (next.has(cardId)) {
			next.delete(cardId);
		} else {
			next.add(cardId);
		}
		expandedCardIds = next;
	}

	function toggleCardColumns(sectionIndex: number, cardIndex: number, columns: 1 | 2 | 3) {
		mutate((l) => {
			const card = l.sections[sectionIndex].cards?.[cardIndex];
			if (card) card.columns = columns;
		});
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
		sections.forEach((s, i) => { s.order = i + 1; });

		mutate((l) => { l.sections = sections; });

		draggedSectionIndex = null;
		dragOverSectionIndex = null;
	}

	function handleSectionDragEnd() {
		draggedSectionIndex = null;
		dragOverSectionIndex = null;
	}

	// ---- Section card column drag-and-drop ----

	function getCardsForColumn(section: LayoutSectionV2, colNum: number): SectionCardV3[] {
		return [...(section.cards ?? [])]
			.filter(c => (c.column ?? 1) === colNum)
			.sort((a, b) => a.order - b.order);
	}

	function findCardIndex(sectionIndex: number, cardId: string): number {
		return (editorLayout?.sections[sectionIndex]?.cards ?? []).findIndex(c => c.id === cardId);
	}

	function moveCardToColumn(sectionIndex: number, cardId: string, targetCol: number) {
		mutate((l) => {
			const card = l.sections[sectionIndex].cards?.find(c => c.id === cardId);
			if (card) card.column = targetCol;
		});
	}

	function handleSectionCardDragStart(e: DragEvent, cardId: string) {
		draggedSectionCardId = cardId;
		e.dataTransfer?.setData('text/plain', cardId);
	}

	function handleColumnDragOver(e: DragEvent) {
		if (!draggedSectionCardId) return;
		e.preventDefault();
	}

	function handleColumnDrop(e: DragEvent, sectionIndex: number, targetCol: number) {
		e.preventDefault();
		if (!draggedSectionCardId) return;
		moveCardToColumn(sectionIndex, draggedSectionCardId, targetCol);
		draggedSectionCardId = null;
	}

	function handleSectionCardDragEnd() {
		draggedSectionCardId = null;
	}

	// ---- Field operations (within cards) ----

	function addFieldToCard(sectionIndex: number, cardIndex: number, fieldName: string) {
		mutate((l) => {
			const card = l.sections[sectionIndex].cards?.[cardIndex];
			if (card) {
				if (!card.fields) card.fields = [];
				card.fields = [...card.fields, createDefaultField(fieldName)];
			}
		});
	}

	function removeFieldFromCard(sectionIndex: number, cardIndex: number, fieldIndex: number) {
		mutate((l) => {
			const card = l.sections[sectionIndex].cards?.[cardIndex];
			if (card && card.fields) {
				card.fields = card.fields.filter((_, i) => i !== fieldIndex);
			}
		});
	}

	// ---- Field drag-and-drop (within cards) ----

	function handleFieldDragStart(sectionIndex: number, cardIndex: number, fieldIndex: number) {
		draggedFieldInfo = { sectionIndex, cardIndex, fieldIndex };
	}

	function handleFieldDragOver(e: DragEvent, sectionIndex: number, cardIndex: number, fieldIndex: number) {
		e.preventDefault();
		dragOverFieldInfo = { sectionIndex, cardIndex, fieldIndex };
	}

	function handleFieldDragLeave() {
		dragOverFieldInfo = null;
	}

	function handleFieldDrop(e: DragEvent, dropSectionIndex: number, dropCardIndex: number, dropFieldIndex: number) {
		e.preventDefault();
		if (!draggedFieldInfo || !editorLayout) {
			draggedFieldInfo = null;
			dragOverFieldInfo = null;
			return;
		}

		const { sectionIndex: srcSection, cardIndex: srcCard, fieldIndex: srcField } = draggedFieldInfo;

		// Only support reorder within same card
		if (srcSection !== dropSectionIndex || srcCard !== dropCardIndex || srcField === dropFieldIndex) {
			draggedFieldInfo = null;
			dragOverFieldInfo = null;
			return;
		}

		const card = editorLayout.sections[srcSection].cards?.[srcCard];
		if (!card?.fields) {
			draggedFieldInfo = null;
			dragOverFieldInfo = null;
			return;
		}

		const cardFields = [...card.fields];
		const [dragged] = cardFields.splice(srcField, 1);
		cardFields.splice(dropFieldIndex, 0, dragged);

		mutate((l) => {
			const c = l.sections[srcSection].cards?.[srcCard];
			if (c) c.fields = cardFields;
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
			for (const tab of l.tabs) {
				tab.sectionIds = tab.sectionIds.filter((id) => id !== sectionId);
			}
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
		// Open the card type selector modal for sidebar
		cardTypeSelectorTarget = 'sidebar';
		showCardTypeSelector = true;
	}

	function addSidebarCardOfType(cardType: SectionCardType) {
		if (!editorLayout) return;
		const maxOrder = editorLayout.sidebar.cards.length > 0
			? Math.max(...editorLayout.sidebar.cards.map((c) => c.order))
			: 0;
		const newCard = createDefaultCard(cardType, maxOrder + 1);
		newCard.label = cardType === 'activity' ? 'Activities' : cardType === 'relatedList' ? 'Related Records' : cardType === 'customPage' ? 'Custom Content' : 'Fields';
		// Sidebar cards are single-column (narrow sidebar)
		if (cardType === 'field') newCard.columns = 1;
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
			l.sidebar.cards.sort((a, b) => a.order - b.order).forEach((c, i) => { c.order = i + 1; });
		});
		const next = new Set(expandedCards);
		next.delete(cardId);
		expandedCards = next;
	}

	function updateSidebarCardLabel(cardId: string, label: string) {
		mutate((l) => {
			const card = l.sidebar.cards.find((c) => c.id === cardId);
			if (card) card.label = label;
		});
	}

	function addFieldToSidebarCard(cardId: string, fieldName: string) {
		mutate((l) => {
			const card = l.sidebar.cards.find((c) => c.id === cardId);
			if (card) {
				if (!card.fields) card.fields = [];
				if (!card.fields.some((f) => f.name === fieldName)) {
					card.fields = [...card.fields, { name: fieldName, visibility: { type: 'always' } }];
				}
			}
		});
	}

	function removeFieldFromSidebarCard(cardId: string, fieldName: string) {
		mutate((l) => {
			const card = l.sidebar.cards.find((c) => c.id === cardId);
			if (card) {
				card.fields = (card.fields ?? []).filter((f) => f.name !== fieldName);
			}
		});
	}

	function toggleSidebarCardExpand(cardId: string) {
		const next = new Set(expandedCards);
		if (next.has(cardId)) {
			next.delete(cardId);
		} else {
			next.add(cardId);
		}
		expandedCards = next;
	}

	// ---- Sidebar card drag-and-drop ----

	function handleSidebarCardDragStart(index: number) {
		draggedCardIndex = index;
	}

	function handleSidebarCardDragOver(e: DragEvent, index: number) {
		e.preventDefault();
		dragOverCardIndex = index;
	}

	function handleSidebarCardDragLeave() {
		dragOverCardIndex = null;
	}

	function handleSidebarCardDrop(e: DragEvent, dropIndex: number) {
		e.preventDefault();
		if (draggedCardIndex === null || draggedCardIndex === dropIndex || !editorLayout) {
			draggedCardIndex = null;
			dragOverCardIndex = null;
			return;
		}

		const cards = [...editorLayout.sidebar.cards].sort((a, b) => a.order - b.order);
		const [dragged] = cards.splice(draggedCardIndex, 1);
		cards.splice(dropIndex, 0, dragged);
		cards.forEach((c, i) => { c.order = i + 1; });

		mutate((l) => { l.sidebar.cards = cards; });

		draggedCardIndex = null;
		dragOverCardIndex = null;
	}

	function handleSidebarCardDragEnd() {
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

	// Available fields for a specific card (not used by any other card)
	function availableFieldsForCard(sectionIndex: number, cardIndex: number): FieldDef[] {
		return fields.filter((f) => !usedFieldNames().has(f.name));
	}

	function availableHeaderFields(): FieldDef[] {
		if (!editorLayout) return [];
		return fields.filter((f) => !editorLayout!.header.fields.includes(f.name));
	}

	function availableSidebarCardFields(cardId: string): FieldDef[] {
		if (!editorLayout) return [];
		const card = editorLayout.sidebar.cards.find((c) => c.id === cardId);
		if (!card || card.cardType !== 'field') return [];
		const cardFieldNames = (card.fields ?? []).map((f) => f.name);
		return fields.filter((f) => !cardFieldNames.includes(f.name));
	}

	function getCardTypeBadge(cardType: SectionCardType): { label: string; classes: string } {
		switch (cardType) {
			case 'activity': return { label: 'Activity', classes: 'bg-green-100 text-green-700' };
			case 'relatedList': return { label: 'Related List', classes: 'bg-purple-100 text-purple-700' };
			case 'customPage': return { label: 'Custom Page', classes: 'bg-amber-100 text-amber-700' };
			default: return { label: 'Fields', classes: 'bg-blue-100 text-blue-700' };
		}
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
			<span class="text-gray-900">Layout Editor</span>
		</nav>

		<!-- Top bar -->
		<div class="flex items-center justify-between">
			<div>
				<h1 class="text-2xl font-bold text-gray-900">
					{entity?.label || entityName} — Layout Editor
				</h1>
				<p class="text-sm text-gray-500 mt-1">
					Configure tabs, header fields, sidebar cards, and sections with multiple cards. Save when done.
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
								<div class="flex-shrink-0 cursor-move text-gray-400 hover:text-gray-600">
									<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8h16M4 16h16" />
									</svg>
								</div>

								<input
									type="text"
									value={tab.label}
									oninput={(e) => updateTabLabel(tab.id, (e.target as HTMLInputElement).value)}
									class="flex-1 text-sm font-medium text-gray-900 bg-transparent border-0 focus:outline-none focus:ring-1 focus:ring-blue-500 rounded px-1 py-0.5"
									placeholder="Tab name"
								/>

								<span class="flex-shrink-0 text-xs px-2 py-0.5 bg-blue-100 text-blue-700 rounded-full">
									{tab.sectionIds.length} section{tab.sectionIds.length !== 1 ? 's' : ''}
								</span>

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
							{@const sideBadge = getCardTypeBadge(card.cardType)}

							<div
								class="border rounded-lg transition-all
									{isCardDragging ? 'opacity-50 border-blue-300' : 'border-gray-200'}
									{isCardDropTarget ? 'border-blue-500' : ''}"
							>
								<div
									draggable="true"
									ondragstart={() => handleSidebarCardDragStart(cardIndex)}
									ondragover={(e) => handleSidebarCardDragOver(e, cardIndex)}
									ondragleave={handleSidebarCardDragLeave}
									ondrop={(e) => handleSidebarCardDrop(e, cardIndex)}
									ondragend={handleSidebarCardDragEnd}
									class="flex items-center gap-3 px-3 py-2 {isCardExpanded ? 'border-b border-gray-100' : ''}"
								>
									<div class="flex-shrink-0 cursor-move text-gray-400 hover:text-gray-600">
										<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
											<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8h16M4 16h16" />
										</svg>
									</div>

									<input
										type="text"
										value={card.label ?? ''}
										oninput={(e) => updateSidebarCardLabel(card.id, (e.target as HTMLInputElement).value)}
										class="flex-1 text-sm font-medium text-gray-900 bg-transparent border-0 focus:outline-none focus:ring-1 focus:ring-blue-500 rounded px-1 py-0.5"
										placeholder="Card name"
									/>

									<span class="flex-shrink-0 text-xs px-2 py-0.5 rounded-full {sideBadge.classes}">
										{sideBadge.label}
									</span>

									{#if card.cardType === 'field'}
										<span class="flex-shrink-0 text-xs px-2 py-0.5 bg-purple-100 text-purple-700 rounded-full">
											{(card.fields ?? []).length} field{(card.fields ?? []).length !== 1 ? 's' : ''}
										</span>
									{/if}

									<button
										type="button"
										onclick={() => toggleSidebarCardExpand(card.id)}
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

								{#if isCardExpanded}
									<div class="p-3">
										{#if card.cardType === 'field'}
											<div class="grid grid-cols-2 gap-4">
												<div>
													<h4 class="text-xs font-medium text-gray-600 uppercase tracking-wide mb-2">Available Fields</h4>
													<div class="space-y-1 max-h-48 overflow-y-auto">
														{#if availableSidebarCardFields(card.id).length === 0}
															<p class="text-xs text-gray-400 italic py-2">All fields added to card</p>
														{:else}
															{#each availableSidebarCardFields(card.id) as field (field.name)}
																<div class="flex items-center gap-2 px-2 py-1 rounded hover:bg-gray-50">
																	<span class="flex-1 text-xs text-gray-800">{field.label}</span>
																	{#if field.type}
																		<span class="text-xs px-1 py-0.5 bg-gray-100 text-gray-500 rounded">{field.type}</span>
																	{/if}
																	<button
																		type="button"
																		onclick={() => addFieldToSidebarCard(card.id, field.name)}
																		class="text-xs px-1.5 py-0.5 bg-blue-600 text-white rounded hover:bg-blue-700 transition-colors"
																	>
																		Add &gt;
																	</button>
																</div>
															{/each}
														{/if}
													</div>
												</div>

												<div>
													<h4 class="text-xs font-medium text-gray-600 uppercase tracking-wide mb-2">
														Selected ({(card.fields ?? []).length})
													</h4>
													<div class="space-y-1 max-h-48 overflow-y-auto">
														{#if (card.fields ?? []).length === 0}
															<p class="text-xs text-gray-400 italic py-2">No fields in this card</p>
														{:else}
															{#each (card.fields ?? []) as fieldLayout (fieldLayout.name)}
																<div class="flex items-center gap-2 px-2 py-1 rounded hover:bg-gray-50">
																	<span class="flex-1 text-xs text-gray-800">{getFieldLabel(fieldLayout.name)}</span>
																	{#if getFieldType(fieldLayout.name)}
																		<span class="text-xs px-1 py-0.5 bg-gray-100 text-gray-500 rounded">{getFieldType(fieldLayout.name)}</span>
																	{/if}
																	<button
																		type="button"
																		onclick={() => removeFieldFromSidebarCard(card.id, fieldLayout.name)}
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
										{:else if card.cardType === 'activity'}
											<div class="px-4 py-3 bg-green-50">
												<p class="text-sm text-green-700">Activity Card — displays tasks, calls, and meetings linked to this record. No additional configuration required.</p>
											</div>
										{:else if card.cardType === 'relatedList'}
											{@const rlConfig = (card.cardConfig ?? { relatedListConfigId: '' }) as RelatedListCardConfig}
											<div class="px-4 py-3 bg-purple-50">
												<label class="block text-xs font-medium text-gray-600 mb-1">Related Entity</label>
												<select
													class="w-full text-sm border border-gray-300 rounded px-2 py-1.5"
													value={rlConfig.relatedListConfigId}
													onchange={(e) => {
														mutate((l) => {
															const c = l.sidebar.cards.find(sc => sc.id === card.id);
															if (c) c.cardConfig = { relatedListConfigId: (e.target as HTMLSelectElement).value };
														});
													}}
												>
													<option value="">-- Select related entity --</option>
													{#each relatedListConfigs.filter(c => c.enabled) as rlc (rlc.id)}
														<option value={rlc.id}>{rlc.label} ({rlc.relatedEntity})</option>
													{/each}
												</select>
											</div>
										{:else if card.cardType === 'customPage'}
											{@const cpConfig = (card.cardConfig ?? { mode: 'iframe', url: '', height: 400 }) as CustomPageCardConfig}
											<div class="px-4 py-3 bg-amber-50 space-y-2">
												<div class="flex gap-2">
													<button
														type="button"
														onclick={() => mutate(l => { const c = l.sidebar.cards.find(sc => sc.id === card.id); if (c) c.cardConfig = { ...cpConfig, mode: 'iframe' }; })}
														class="px-3 py-1 text-xs rounded {cpConfig.mode === 'iframe' ? 'bg-amber-200 text-amber-800' : 'bg-white text-gray-600 border border-gray-300'}"
													>
														Iframe
													</button>
													<button
														type="button"
														onclick={() => mutate(l => { const c = l.sidebar.cards.find(sc => sc.id === card.id); if (c) c.cardConfig = { ...cpConfig, mode: 'html' }; })}
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
															oninput={(e) => mutate(l => { const c = l.sidebar.cards.find(sc => sc.id === card.id); if (c) c.cardConfig = { ...cpConfig, url: (e.target as HTMLInputElement).value }; })}
														/>
													</div>
													<div>
														<label class="block text-xs font-medium text-gray-600 mb-1">Height (px)</label>
														<input type="number" class="w-32 text-sm border border-gray-300 rounded px-2 py-1.5" placeholder="400"
															value={cpConfig.height ?? 400}
															oninput={(e) => mutate(l => { const c = l.sidebar.cards.find(sc => sc.id === card.id); if (c) c.cardConfig = { ...cpConfig, height: parseInt((e.target as HTMLInputElement).value) || 400 }; })}
														/>
													</div>
												{:else}
													<div>
														<label class="block text-xs font-medium text-gray-600 mb-1">HTML Content <span class="text-gray-400">(admin-trusted)</span></label>
														<textarea class="w-full text-sm border border-gray-300 rounded px-2 py-1.5 font-mono" rows="6" placeholder="<div>Your custom HTML here</div>"
															value={cpConfig.content ?? ''}
															oninput={(e) => mutate(l => { const c = l.sidebar.cards.find(sc => sc.id === card.id); if (c) c.cardConfig = { ...cpConfig, content: (e.target as HTMLTextAreaElement).value }; })}
														></textarea>
													</div>
												{/if}
											</div>
										{/if}
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
				<p class="text-xs text-gray-500">Sections are containers. Add cards inside each section.</p>
			</div>

			<div class="space-y-3">
				{#each [...editorLayout.sections].sort((a, b) => a.order - b.order) as section, index (section.id)}
					{@const isSectionExpanded = expandedSections.has(section.id)}
					{@const isDraggingThis = draggedSectionIndex === index}
					{@const isDragTarget = dragOverSectionIndex === index && draggedSectionIndex !== index}

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

							<!-- Section label -->
							<input
								type="text"
								value={section.label}
								oninput={(e) => updateSectionLabel(index, (e.target as HTMLInputElement).value)}
								class="flex-1 text-sm font-medium text-gray-900 bg-transparent border-0 focus:outline-none focus:ring-1 focus:ring-blue-500 rounded px-1 py-0.5"
								placeholder="Section name"
							/>

							<!-- Tab assignment -->
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

							<!-- Section column toggle -->
							<div class="flex-shrink-0 flex items-center gap-1 border border-gray-200 rounded-md overflow-hidden">
								{#each [1, 2, 3] as col}
									<button
										type="button"
										onclick={() => toggleSectionColumns(index, col as 1 | 2 | 3)}
										class="px-2.5 py-1 text-xs font-medium transition-colors
											{col > 1 ? 'border-l border-gray-200' : ''}
											{section.columns === col
												? 'bg-blue-600 text-white'
												: 'text-gray-600 hover:bg-gray-100'}"
										title="{col} column{col !== 1 ? 's' : ''}"
									>
										{col} col
									</button>
								{/each}
							</div>

							<!-- Card count badge -->
							<span class="flex-shrink-0 text-xs text-gray-400 tabular-nums">
								{(section.cards ?? []).length} card{(section.cards ?? []).length !== 1 ? 's' : ''}
							</span>

							<!-- Expand/collapse -->
							<button
								type="button"
								onclick={() => toggleSectionExpand(section.id)}
								class="flex-shrink-0 text-gray-400 hover:text-gray-600"
								title={isSectionExpanded ? 'Collapse section' : 'Expand section'}
							>
								<svg
									class="w-5 h-5 transition-transform {isSectionExpanded ? 'rotate-180' : ''}"
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

						<!-- Expanded: cards list -->
						{#if isSectionExpanded}
							{#snippet cardEditor(card: SectionCardV3, sectionIdx: number, isDraggableColumn: boolean)}
								{@const cardIdx = findCardIndex(sectionIdx, card.id)}
								{@const badge = getCardTypeBadge(card.cardType)}
								{@const isCardExpanded = expandedCardIds.has(card.id)}

								<div
									class="border border-gray-200 rounded-lg overflow-hidden transition-all
										{draggedSectionCardId === card.id ? 'opacity-50 ring-2 ring-blue-400' : ''}"
									draggable={isDraggableColumn ? 'true' : undefined}
									ondragstart={isDraggableColumn ? (e: DragEvent) => handleSectionCardDragStart(e, card.id) : undefined}
									ondragend={isDraggableColumn ? handleSectionCardDragEnd : undefined}
								>
									<!-- Card header -->
									<div class="flex items-center gap-2 px-3 py-2 bg-gray-50">
										{#if isDraggableColumn}
											<div class="flex-shrink-0 cursor-grab text-gray-400" title="Drag to another column">
												<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
													<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8h16M4 16h16" />
												</svg>
											</div>
										{/if}

										<!-- Card type badge -->
										<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium {badge.classes}">
											{badge.label}
										</span>

										<!-- Card label -->
										<input
											type="text"
											value={card.label ?? ''}
											oninput={(e) => updateCardLabel(sectionIdx, cardIdx, (e.target as HTMLInputElement).value)}
											class="flex-1 text-sm text-gray-700 bg-transparent border-0 focus:outline-none focus:ring-1 focus:ring-blue-500 rounded px-1 py-0.5"
											placeholder="Card label (optional)"
										/>

										{#if card.cardType === 'field'}
											<!-- Internal column toggle for field cards -->
											<div class="flex-shrink-0 flex items-center gap-0.5 border border-gray-200 rounded overflow-hidden">
												{#each [1, 2] as col}
													<button
														type="button"
														onclick={() => toggleCardColumns(sectionIdx, cardIdx, col as 1 | 2)}
														class="px-2 py-0.5 text-xs transition-colors
															{col > 1 ? 'border-l border-gray-200' : ''}
															{(card.columns ?? 2) === col
																? 'bg-blue-600 text-white'
																: 'text-gray-500 hover:bg-gray-100'}"
														title="{col} col internal"
													>
														{col}
													</button>
												{/each}
											</div>

											<span class="text-xs text-gray-400 tabular-nums">
												{(card.fields ?? []).length} field{(card.fields ?? []).length !== 1 ? 's' : ''}
											</span>

											<!-- Add field button + dropdown -->
											{@const pickerAvailFields = availableFieldsForCard(sectionIdx, cardIdx)}
											{#if pickerAvailFields.length > 0}
												<div class="relative flex-shrink-0">
													<button
														type="button"
														onclick={(e) => { e.stopPropagation(); toggleFieldPicker(card.id); }}
														class="inline-flex items-center gap-0.5 px-1.5 py-0.5 text-xs font-medium rounded transition-colors
															{fieldPickerCardId === card.id
																? 'bg-blue-600 text-white'
																: 'bg-blue-50 text-blue-700 hover:bg-blue-100'}"
														title="Add field to this card"
													>
														<svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
															<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
														</svg>
														Field
													</button>
													{#if fieldPickerCardId === card.id}
														<div
															class="absolute right-0 top-full mt-1 z-50 bg-white border border-gray-200 rounded-lg shadow-lg w-64"
															onclick={(e) => e.stopPropagation()}
														>
															<div class="p-2 border-b border-gray-100">
																<input
																	type="text"
																	placeholder="Search fields..."
																	bind:value={fieldPickerSearch}
																	class="w-full text-xs border border-gray-200 rounded px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-blue-500"
																/>
															</div>
															<div class="max-h-48 overflow-y-auto p-1">
																{#each pickerAvailFields.filter(f =>
																	!fieldPickerSearch ||
																	f.label.toLowerCase().includes(fieldPickerSearch.toLowerCase()) ||
																	f.name.toLowerCase().includes(fieldPickerSearch.toLowerCase())
																) as f (f.id)}
																	<button
																		type="button"
																		onclick={() => { addFieldToCard(sectionIdx, cardIdx, f.name); closeFieldPicker(); }}
																		class="w-full text-left px-2 py-1.5 text-xs rounded hover:bg-blue-50 transition-colors flex items-center justify-between gap-2"
																	>
																		<span class="text-gray-800 truncate">{f.label}</span>
																		<span class="text-gray-400 font-mono text-[10px] flex-shrink-0">{f.type}</span>
																	</button>
																{:else}
																	<p class="text-xs text-gray-400 text-center py-2">No matching fields</p>
																{/each}
															</div>
														</div>
													{/if}
												</div>
											{/if}
										{/if}

										<!-- Expand/collapse card -->
										<button
											type="button"
											onclick={() => toggleCardExpand(card.id)}
											class="flex-shrink-0 text-gray-400 hover:text-gray-600"
											title={isCardExpanded ? 'Collapse' : 'Expand'}
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
											onclick={() => deleteCard(sectionIdx, cardIdx)}
											class="flex-shrink-0 text-gray-400 hover:text-red-500"
											title="Delete card"
										>
											<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
												<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
											</svg>
										</button>
									</div>

									<!-- Card config panel (expanded) -->
									{#if isCardExpanded}
										{#if card.cardType === 'field'}
											<!-- Field card: field list + add pills -->
											<div class="p-3 space-y-2">
												{#if (card.fields ?? []).length === 0}
													<p class="text-sm text-gray-400 italic text-center py-2">
														No fields. Add fields below.
													</p>
												{:else}
													{#each (card.fields ?? []) as field, fieldIndex (field.name)}
														{@const isFieldDragging = draggedFieldInfo?.sectionIndex === sectionIdx && draggedFieldInfo?.cardIndex === cardIdx && draggedFieldInfo?.fieldIndex === fieldIndex}
														{@const isFieldDropTarget = dragOverFieldInfo?.sectionIndex === sectionIdx && dragOverFieldInfo?.cardIndex === cardIdx && dragOverFieldInfo?.fieldIndex === fieldIndex && draggedFieldInfo?.sectionIndex === sectionIdx && draggedFieldInfo?.cardIndex === cardIdx && draggedFieldInfo?.fieldIndex !== fieldIndex}

														<div
															draggable="true"
															ondragstart={() => handleFieldDragStart(sectionIdx, cardIdx, fieldIndex)}
															ondragover={(e) => handleFieldDragOver(e, sectionIdx, cardIdx, fieldIndex)}
															ondragleave={handleFieldDragLeave}
															ondrop={(e) => handleFieldDrop(e, sectionIdx, cardIdx, fieldIndex)}
															ondragend={handleFieldDragEnd}
															class="flex items-center gap-2 px-3 py-2 bg-gray-50 rounded border transition-all
																{isFieldDragging ? 'opacity-50 border-blue-300' : 'border-gray-200'}
																{isFieldDropTarget ? 'border-blue-500 bg-blue-50' : ''}"
														>
															<div class="flex-shrink-0 cursor-move text-gray-400">
																<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
																	<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8h16M4 16h16" />
																</svg>
															</div>
															<span class="flex-1 text-sm text-gray-800">{getFieldLabel(field.name)}</span>
															<span class="text-xs text-gray-400 font-mono">{field.name}</span>
															{#if getFieldType(field.name)}
																<span class="flex-shrink-0 text-xs px-1.5 py-0.5 bg-gray-200 text-gray-600 rounded">
																	{getFieldType(field.name)}
																</span>
															{/if}
															<button
																type="button"
																onclick={() => removeFieldFromCard(sectionIdx, cardIdx, fieldIndex)}
																class="flex-shrink-0 text-gray-400 hover:text-red-500"
																title="Remove field"
															>
																<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
																	<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
																</svg>
															</button>
														</div>
													{/each}
												{/if}

												<!-- Add field pills -->
												{#if availableFieldsForCard(sectionIdx, cardIdx).length > 0}
												{@const availFields = availableFieldsForCard(sectionIdx, cardIdx)}
													<div class="pt-2 border-t border-gray-100">
														<p class="text-xs text-gray-500 mb-2">Add field:</p>
														<div class="flex flex-wrap gap-1.5">
															{#each availFields as availField (availField.id)}
																<button
																	type="button"
																	onclick={() => addFieldToCard(sectionIdx, cardIdx, availField.name)}
																	class="text-xs px-2 py-1 bg-blue-50 text-blue-700 border border-blue-200 rounded hover:bg-blue-100 transition-colors"
																>
																	+ {availField.label}
																</button>
															{/each}
														</div>
													</div>
												{/if}
											</div>
										{:else if card.cardType === 'relatedList'}
											<!-- Related list config -->
											{@const rlConfig = (card.cardConfig ?? { relatedListConfigId: '' }) as RelatedListCardConfig}
											<div class="px-4 py-3 bg-purple-50">
												<label class="block text-xs font-medium text-gray-600 mb-1">Related Entity</label>
												<select
													class="w-full text-sm border border-gray-300 rounded px-2 py-1.5"
													value={rlConfig.relatedListConfigId}
													onchange={(e) => {
														mutate((l) => {
															const c = l.sections[sectionIdx].cards?.[cardIdx];
															if (c) c.cardConfig = { relatedListConfigId: (e.target as HTMLSelectElement).value };
														});
													}}
												>
													<option value="">-- Select related entity --</option>
													{#each relatedListConfigs.filter(c => c.enabled) as rlc (rlc.id)}
														<option value={rlc.id}>{rlc.label} ({rlc.relatedEntity})</option>
													{/each}
												</select>
											</div>
										{:else if card.cardType === 'customPage'}
											<!-- Custom page config -->
											{@const cpConfig = (card.cardConfig ?? { mode: 'iframe', url: '', height: 400 }) as CustomPageCardConfig}
											<div class="px-4 py-3 bg-amber-50 space-y-2">
												<div class="flex gap-2">
													<button
														type="button"
														onclick={() => mutate(l => { const c = l.sections[sectionIdx].cards?.[cardIdx]; if (c) c.cardConfig = { ...cpConfig, mode: 'iframe' }; })}
														class="px-3 py-1 text-xs rounded {cpConfig.mode === 'iframe' ? 'bg-amber-200 text-amber-800' : 'bg-white text-gray-600 border border-gray-300'}"
													>
														Iframe
													</button>
													<button
														type="button"
														onclick={() => mutate(l => { const c = l.sections[sectionIdx].cards?.[cardIdx]; if (c) c.cardConfig = { ...cpConfig, mode: 'html' }; })}
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
															oninput={(e) => mutate(l => { const c = l.sections[sectionIdx].cards?.[cardIdx]; if (c) c.cardConfig = { ...cpConfig, url: (e.target as HTMLInputElement).value }; })}
														/>
													</div>
													<div>
														<label class="block text-xs font-medium text-gray-600 mb-1">Height (px)</label>
														<input type="number" class="w-32 text-sm border border-gray-300 rounded px-2 py-1.5" placeholder="400"
															value={cpConfig.height ?? 400}
															oninput={(e) => mutate(l => { const c = l.sections[sectionIdx].cards?.[cardIdx]; if (c) c.cardConfig = { ...cpConfig, height: parseInt((e.target as HTMLInputElement).value) || 400 }; })}
														/>
													</div>
												{:else}
													<div>
														<label class="block text-xs font-medium text-gray-600 mb-1">HTML Content <span class="text-gray-400">(admin-trusted)</span></label>
														<textarea class="w-full text-sm border border-gray-300 rounded px-2 py-1.5 font-mono" rows="6" placeholder="<div>Your custom HTML here</div>"
															value={cpConfig.content ?? ''}
															oninput={(e) => mutate(l => { const c = l.sections[sectionIdx].cards?.[cardIdx]; if (c) c.cardConfig = { ...cpConfig, content: (e.target as HTMLTextAreaElement).value }; })}
														></textarea>
													</div>
												{/if}
											</div>
										{:else if card.cardType === 'activity'}
											<div class="px-4 py-3 bg-green-50">
												<p class="text-sm text-green-700">Activity Card — displays tasks, calls, and meetings linked to this record. No additional configuration required.</p>
											</div>
										{/if}
									{/if}
								</div>
							{/snippet}

							{#if section.columns > 1}
								<!-- Multi-column view: cards arranged in columns with drag-and-drop -->
								<div class="p-4">
									<div class="grid gap-3" style="grid-template-columns: repeat({section.columns}, minmax(0, 1fr))">
										{#each Array.from({ length: section.columns }, (_, i) => i + 1) as colNum}
											{@const colCards = getCardsForColumn(section, colNum)}
											<div
												class="space-y-2 p-2 rounded-lg border-2 border-dashed min-h-[80px] transition-colors
													{draggedSectionCardId ? 'border-blue-300 bg-blue-50/30' : 'border-gray-200'}"
												ondragover={(e) => handleColumnDragOver(e)}
												ondrop={(e) => handleColumnDrop(e, index, colNum)}
											>
												<div class="text-xs font-medium text-gray-400 text-center pb-1 border-b border-gray-100">
													Column {colNum}
												</div>
												{#if colCards.length === 0}
													<p class="text-xs text-gray-300 italic text-center py-4">
														Drop cards here
													</p>
												{:else}
													{#each colCards as card (card.id)}
														{@render cardEditor(card, index, true)}
													{/each}
												{/if}
												<button
													type="button"
													onclick={() => openAddCard(index, colNum)}
													class="w-full px-2 py-1.5 border-2 border-dashed border-gray-300 text-gray-400 rounded hover:border-blue-400 hover:text-blue-600 transition-colors text-xs"
												>
													+ Add
												</button>
											</div>
										{/each}
									</div>
								</div>
							{:else}
								<!-- Single column: flat card list (original behavior) -->
								<div class="p-4 space-y-3">
									{#if (section.cards ?? []).length === 0}
										<p class="text-sm text-gray-400 italic text-center py-3">
											No cards in this section. Add a card below.
										</p>
									{:else}
										{#each [...(section.cards ?? [])].sort((a, b) => a.order - b.order) as card (card.id)}
											{@render cardEditor(card, index, false)}
										{/each}
									{/if}

									<!-- Add Card button -->
									<button
										type="button"
										onclick={() => openAddCard(index)}
										class="w-full px-3 py-2 border-2 border-dashed border-gray-300 text-gray-500 rounded-lg hover:border-blue-400 hover:text-blue-600 transition-colors text-xs font-medium"
									>
										+ Add Card
									</button>
								</div>
							{/if}
						{/if}
					</div>
				{/each}
			</div>

			<!-- Add section button -->
			<div class="mt-3">
				<button
					type="button"
					onclick={addSection}
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
					{unassignedFields.length} field{unassignedFields.length !== 1 ? 's' : ''} not in any card:
				</h3>
				<div class="flex flex-wrap gap-2">
					{#each unassignedFields as field (field.id)}
						<span class="px-2 py-1 text-xs bg-yellow-100 text-yellow-800 rounded">
							{field.label}
						</span>
					{/each}
				</div>
				<p class="mt-2 text-xs text-yellow-700">
					These fields won't appear on the detail page until added to a field card in a section.
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
				<h3 class="text-lg font-semibold text-gray-900 mb-4">Add Card</h3>
				<div class="grid grid-cols-2 gap-3">
					<!-- Field Card -->
					<button onclick={() => selectCardTypeForSection('field')} class="flex flex-col items-center gap-2 p-4 rounded-lg border-2 border-gray-200 hover:border-blue-500 hover:bg-blue-50 transition-colors text-center">
						<div class="w-10 h-10 rounded-lg bg-blue-100 flex items-center justify-center">
							<svg class="w-5 h-5 text-blue-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
								<path stroke-linecap="round" stroke-linejoin="round" d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25H12" />
							</svg>
						</div>
						<span class="text-sm font-medium text-gray-900">Field Card</span>
						<span class="text-xs text-gray-500">Display record fields</span>
					</button>
					<!-- Activity Card -->
					<button onclick={() => selectCardTypeForSection('activity')} class="flex flex-col items-center gap-2 p-4 rounded-lg border-2 border-gray-200 hover:border-green-500 hover:bg-green-50 transition-colors text-center">
						<div class="w-10 h-10 rounded-lg bg-green-100 flex items-center justify-center">
							<svg class="w-5 h-5 text-green-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
								<path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z" />
							</svg>
						</div>
						<span class="text-sm font-medium text-gray-900">Activity Card</span>
						<span class="text-xs text-gray-500">Tasks, calls &amp; meetings</span>
					</button>
					<!-- Related List Card -->
					<button onclick={() => selectCardTypeForSection('relatedList')} class="flex flex-col items-center gap-2 p-4 rounded-lg border-2 border-gray-200 hover:border-purple-500 hover:bg-purple-50 transition-colors text-center">
						<div class="w-10 h-10 rounded-lg bg-purple-100 flex items-center justify-center">
							<svg class="w-5 h-5 text-purple-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
								<path stroke-linecap="round" stroke-linejoin="round" d="M3.75 12h16.5m-16.5 3.75h16.5M3.75 19.5h16.5M5.625 4.5h12.75a1.875 1.875 0 010 3.75H5.625a1.875 1.875 0 010-3.75z" />
							</svg>
						</div>
						<span class="text-sm font-medium text-gray-900">Related List</span>
						<span class="text-xs text-gray-500">Inline related records</span>
					</button>
					<!-- Custom Page Card -->
					<button onclick={() => selectCardTypeForSection('customPage')} class="flex flex-col items-center gap-2 p-4 rounded-lg border-2 border-gray-200 hover:border-amber-500 hover:bg-amber-50 transition-colors text-center">
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
