// Layout schema types for v2 multi-section layouts

import { fieldNameToKey, getRecordValue } from '$lib/utils/fieldMapping';

export type LayoutVersion = 1 | 2 | 3;

export type VisibilityType = 'always' | 'conditional' | 'never';

export type VisibilityOperator =
	| 'EQUALS'
	| 'NOT_EQUALS'
	| 'IN'
	| 'NOT_IN'
	| 'IS_EMPTY'
	| 'IS_NOT_EMPTY'
	| 'GREATER_THAN'
	| 'LESS_THAN'
	| 'GREATER_EQUAL'
	| 'LESS_EQUAL'
	| 'IS_TRUE'
	| 'IS_FALSE'
	| 'CONTAINS'
	| 'STARTS_WITH'
	| 'ENDS_WITH';

export interface VisibilityCondition {
	id: string;
	field: string;
	operator: VisibilityOperator;
	value?: unknown;
	values?: string[]; // For IN, NOT_IN operators
}

export interface VisibilityRule {
	type: VisibilityType;
	conditions?: VisibilityCondition[];
	logic?: 'AND' | 'OR'; // Defaults to 'AND'
}

export interface LayoutFieldV2 {
	name: string;
	visibility: VisibilityRule;
}

// Section card types
export type SectionCardType = 'field' | 'activity' | 'relatedList' | 'customPage';

// Card-specific configuration interfaces
export interface ActivityCardConfig {
	// No extra config — entity + id come from parent component props
}

export interface RelatedListCardConfig {
	relatedListConfigId: string; // FK into entity's related_list_configs
}

export interface CustomPageCardConfig {
	mode: 'iframe' | 'html';
	url?: string; // for iframe mode — supports {{recordId}}, {{entityName}}, {{fieldName}} templates
	height?: number; // for iframe mode, default 400
	content?: string; // for html mode
}

export type SectionCardConfig = ActivityCardConfig | RelatedListCardConfig | CustomPageCardConfig | null;

// Individual card within a multi-card section container
export interface SectionCardV3 {
	id: string;
	cardType: SectionCardType;
	order: number;
	label?: string;
	fields?: LayoutFieldV2[];
	cardConfig?: SectionCardConfig;
	columns?: 1 | 2 | 3; // internal field grid columns for field cards
	column?: number; // which section grid column this card sits in (1-indexed, default 1)
}

export interface LayoutSectionV2 {
	id: string;
	label: string;
	order: number;
	collapsible: boolean;
	collapsed: boolean; // Default collapsed state
	columns: 1 | 2 | 3;
	visibility: VisibilityRule;
	fields: LayoutFieldV2[];
	cardType?: SectionCardType; // DEPRECATED: use cards[].cardType
	cardConfig?: SectionCardConfig; // DEPRECATED: use cards[].cardConfig
	cards?: SectionCardV3[]; // Multi-card container
}

export interface LayoutDataV2 {
	version: LayoutVersion;
	sections: LayoutSectionV2[];
}

// Response types from API
export interface LayoutV2Response {
	entityName: string;
	layoutType: string;
	layout: LayoutDataV2;
	exists: boolean;
}

// V3 layout types — mirrors backend entity/layout.go
export interface LayoutTabV3 {
	id: string;
	label: string;
	order: number;
	sectionIds: string[];
}

export interface LayoutSidebarCardV3 {
	id: string;
	label: string;
	order: number;
	fields: string[];
}

export interface LayoutSidebarV3 {
	cards: LayoutSidebarCardV3[];
}

export interface LayoutHeaderV3 {
	fields: string[];
}

export interface LayoutDataV3 {
	version: 3;
	sections: LayoutSectionV2[];
	tabs: LayoutTabV3[];
	sidebar: LayoutSidebarV3;
	header: LayoutHeaderV3;
	conditions: null;
}

export interface LayoutV3Response {
	entityName: string;
	layoutType: string;
	layout: LayoutDataV3;
	exists: boolean;
}

// Helper functions

export function createDefaultVisibility(): VisibilityRule {
	return { type: 'always' };
}

export function createDefaultSection(id: string, label: string, order: number): LayoutSectionV2 {
	return {
		id,
		label,
		order,
		collapsible: true,
		collapsed: false,
		columns: 2,
		visibility: createDefaultVisibility(),
		fields: []
	};
}

export function createDefaultField(name: string): LayoutFieldV2 {
	return {
		name,
		visibility: createDefaultVisibility()
	};
}

// Convert v1 layout (flat field array) to v2 format
export function convertV1ToV2(fields: string[]): LayoutDataV2 {
	return {
		version: 2,
		sections: [
			{
				id: 'section_general',
				label: 'General Information',
				order: 1,
				collapsible: false,
				collapsed: false,
				columns: 2,
				visibility: createDefaultVisibility(),
				fields: fields.map((name) => createDefaultField(name))
			}
		]
	};
}

// Section array format from provisioning (legacy format)
interface LegacySection {
	label?: string;
	rows?: Array<Array<{ field: string }>>;
}

// Parse layout data from any format (v1 flat array, v2, or legacy section array)
export function parseLayoutData(layoutJson: string, fallbackFields: string[] = []): LayoutDataV2 {
	try {
		const parsed = JSON.parse(layoutJson || '[]');

		// Check if it's v2 format (has version and sections)
		if (parsed && typeof parsed === 'object' && 'version' in parsed && parsed.version === 2 && Array.isArray(parsed.sections)) {
			return parsed as LayoutDataV2;
		}

		if (Array.isArray(parsed) && parsed.length > 0) {
			// Check if it's a section array format (has label/rows)
			if (typeof parsed[0] === 'object' && ('label' in parsed[0] || 'rows' in parsed[0])) {
				// Convert section array to v2 format
				return {
					version: 2,
					sections: (parsed as LegacySection[]).map((section, idx) => ({
						id: `section_${idx}`,
						label: section.label || `Section ${idx + 1}`,
						order: idx + 1,
						collapsible: false,
						collapsed: false,
						columns: 2,
						visibility: { type: 'always' as const },
						fields: (section.rows || []).flat().map((cell) => ({
							name: cell.field,
							visibility: { type: 'always' as const }
						}))
					}))
				};
			}

			// v1 format - flat array of field names
			return convertV1ToV2(parsed as string[]);
		}

		// Unknown or empty format, use fallback
		return convertV1ToV2(fallbackFields);
	} catch {
		return convertV1ToV2(fallbackFields);
	}
}

// Evaluate visibility rule against record data (client-side)
export function evaluateVisibility(rule: VisibilityRule, record: Record<string, unknown>): boolean {
	if (!rule) return true;
	if (rule.type === 'always') return true;
	if (rule.type === 'never') return false;

	if (!rule.conditions || rule.conditions.length === 0) return true;

	const isAnd = rule.logic !== 'OR';

	for (const cond of rule.conditions) {
		const result = evaluateCondition(cond, record);
		if (isAnd && !result) return false;
		if (!isAnd && result) return true;
	}

	return isAnd;
}

function evaluateCondition(cond: VisibilityCondition, record: Record<string, unknown>): boolean {
	const fieldValue = getRecordValue(record, cond.field);

	switch (cond.operator) {
		case 'IS_EMPTY':
			return fieldValue === null || fieldValue === undefined || fieldValue === '';
		case 'IS_NOT_EMPTY':
			return fieldValue !== null && fieldValue !== undefined && fieldValue !== '';
		case 'IS_TRUE':
			return toBool(fieldValue) === true;
		case 'IS_FALSE':
			return toBool(fieldValue) === false;
		case 'EQUALS':
			return String(fieldValue) === String(cond.value);
		case 'NOT_EQUALS':
			return String(fieldValue) !== String(cond.value);
		case 'IN':
			return cond.values?.includes(String(fieldValue)) ?? false;
		case 'NOT_IN':
			return !(cond.values?.includes(String(fieldValue)) ?? false);
		case 'CONTAINS':
			return String(fieldValue).toLowerCase().includes(String(cond.value).toLowerCase());
		case 'STARTS_WITH':
			return String(fieldValue).toLowerCase().startsWith(String(cond.value).toLowerCase());
		case 'ENDS_WITH':
			return String(fieldValue).toLowerCase().endsWith(String(cond.value).toLowerCase());
		case 'GREATER_THAN':
			return toNumber(fieldValue) > toNumber(cond.value);
		case 'LESS_THAN':
			return toNumber(fieldValue) < toNumber(cond.value);
		case 'GREATER_EQUAL':
			return toNumber(fieldValue) >= toNumber(cond.value);
		case 'LESS_EQUAL':
			return toNumber(fieldValue) <= toNumber(cond.value);
		default:
			return true;
	}
}

function toBool(v: unknown): boolean {
	if (typeof v === 'boolean') return v;
	if (typeof v === 'number') return v !== 0;
	if (typeof v === 'string') return v === 'true' || v === '1' || v === 'yes';
	return false;
}

function toNumber(v: unknown): number {
	if (typeof v === 'number') return v;
	if (typeof v === 'string') return parseFloat(v) || 0;
	return 0;
}

// --- Multi-card migration helpers ---

let cardIdCounter = 0;
function generateCardId(): string {
	return 'card_' + Date.now().toString(36) + '_' + (cardIdCounter++).toString(36);
}

// Create a default card with type-appropriate defaults
export function createDefaultCard(cardType: SectionCardType, order: number): SectionCardV3 {
	const card: SectionCardV3 = {
		id: generateCardId(),
		cardType,
		order,
		column: 1
	};
	if (cardType === 'field') {
		card.fields = [];
		card.columns = 2;
	} else if (cardType === 'activity') {
		card.cardConfig = {};
	} else if (cardType === 'relatedList') {
		card.cardConfig = { relatedListConfigId: '' };
	} else if (cardType === 'customPage') {
		card.cardConfig = { mode: 'iframe', url: '', height: 400 };
	}
	return card;
}

// Convert old single-card section to multi-card format
export function migrateSectionToCards(section: LayoutSectionV2): LayoutSectionV2 {
	// Already migrated
	if (section.cards && section.cards.length > 0) return section;

	const cards: SectionCardV3[] = [];
	const effectiveType = section.cardType || 'field';

	if (effectiveType === 'field') {
		// Old field section → single field card with the section's fields
		if (section.fields && section.fields.length > 0) {
			cards.push({
				id: generateCardId(),
				cardType: 'field',
				order: 1,
				fields: [...section.fields],
				columns: section.columns || 2
			});
		}
	} else {
		// Non-field card type → wrap in a card
		cards.push({
			id: generateCardId(),
			cardType: effectiveType,
			order: 1,
			cardConfig: section.cardConfig ?? undefined
		});
	}

	return {
		...section,
		cards,
		// For multi-card sections, columns controls the section grid (how cards flow)
		// Default to 1 so migrated sections render identically (full-width single card)
		columns: effectiveType === 'field' ? 1 : section.columns
	};
}

// Migrate all sections in a V3 layout to multi-card format
export function migrateLayoutV3(layout: LayoutDataV3): LayoutDataV3 {
	return {
		...layout,
		sections: layout.sections.map(migrateSectionToCards)
	};
}

// Check if a card has visible content
function isCardVisible(card: SectionCardV3, record: Record<string, unknown>): boolean {
	if (card.cardType !== 'field') return true;
	if (!card.fields || card.fields.length === 0) return false;
	return card.fields.some((f) => evaluateVisibility(f.visibility, record));
}

// Filter visible fields within cards
function filterCardFields(card: SectionCardV3, record: Record<string, unknown>): SectionCardV3 {
	if (card.cardType !== 'field' || !card.fields) return card;
	return {
		...card,
		fields: card.fields.filter((f) => evaluateVisibility(f.visibility, record))
	};
}

// Get all visible sections with visible cards based on record data
export function getVisibleSections(
	layout: LayoutDataV2,
	record: Record<string, unknown>
): LayoutSectionV2[] {
	return layout.sections
		.map(migrateSectionToCards)
		.filter((section) => evaluateVisibility(section.visibility, record))
		.map((section) => ({
			...section,
			fields: section.fields.filter((field) => evaluateVisibility(field.visibility, record)),
			cards: (section.cards ?? []).map((c) => filterCardFields(c, record))
		}))
		.filter((section) => {
			// Section visible if any card has content
			const cards = section.cards ?? [];
			if (cards.length > 0) return cards.some((c) => isCardVisible(c, record));
			// Fallback for legacy: check fields directly
			if (!section.cardType || section.cardType === 'field') return section.fields.length > 0;
			return true;
		})
		.sort((a, b) => a.order - b.order);
}

// Get visible sections assigned to a specific V3 tab
export function getSectionsForTab(
	layout: LayoutDataV3,
	tabId: string,
	record: Record<string, unknown>
): LayoutSectionV2[] {
	const tab = layout.tabs.find((t) => t.id === tabId);
	if (!tab) return [];
	return layout.sections
		.map(migrateSectionToCards)
		.filter((s) => tab.sectionIds.includes(s.id) && evaluateVisibility(s.visibility, record))
		.map((s) => ({
			...s,
			fields: s.fields.filter((f) => evaluateVisibility(f.visibility, record)),
			cards: (s.cards ?? []).map((c) => filterCardFields(c, record))
		}))
		.filter((s) => {
			const cards = s.cards ?? [];
			if (cards.length > 0) return cards.some((c) => isCardVisible(c, record));
			if (!s.cardType || s.cardType === 'field') return s.fields.length > 0;
			return true;
		})
		.sort((a, b) => a.order - b.order);
}

// Get all field names from a layout (scans cards arrays)
export function getAllFieldNames(layout: LayoutDataV2): string[] {
	const names: string[] = [];
	for (const section of layout.sections) {
		// Scan cards array
		if (section.cards) {
			for (const card of section.cards) {
				if (card.fields) {
					for (const f of card.fields) names.push(f.name);
				}
			}
		}
		// Also scan legacy fields for backward compat
		for (const field of section.fields) {
			names.push(field.name);
		}
	}
	// Deduplicate
	return [...new Set(names)];
}

// Operator labels for UI
export const VISIBILITY_OPERATORS: { value: VisibilityOperator; label: string; description: string }[] = [
	{ value: 'EQUALS', label: 'Equals', description: 'Field value equals the specified value' },
	{ value: 'NOT_EQUALS', label: 'Not Equals', description: 'Field value does not equal the specified value' },
	{ value: 'IN', label: 'In List', description: 'Field value is in the specified list' },
	{ value: 'NOT_IN', label: 'Not In List', description: 'Field value is not in the specified list' },
	{ value: 'IS_EMPTY', label: 'Is Empty', description: 'Field has no value' },
	{ value: 'IS_NOT_EMPTY', label: 'Is Not Empty', description: 'Field has a value' },
	{ value: 'IS_TRUE', label: 'Is True', description: 'Checkbox/boolean is checked' },
	{ value: 'IS_FALSE', label: 'Is False', description: 'Checkbox/boolean is unchecked' },
	{ value: 'CONTAINS', label: 'Contains', description: 'Field value contains the specified text' },
	{ value: 'STARTS_WITH', label: 'Starts With', description: 'Field value starts with the specified text' },
	{ value: 'ENDS_WITH', label: 'Ends With', description: 'Field value ends with the specified text' },
	{ value: 'GREATER_THAN', label: 'Greater Than', description: 'Number is greater than the specified value' },
	{ value: 'LESS_THAN', label: 'Less Than', description: 'Number is less than the specified value' },
	{ value: 'GREATER_EQUAL', label: 'Greater or Equal', description: 'Number is greater than or equal to the specified value' },
	{ value: 'LESS_EQUAL', label: 'Less or Equal', description: 'Number is less than or equal to the specified value' }
];
