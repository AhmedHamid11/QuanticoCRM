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

export interface LayoutSectionV2 {
	id: string;
	label: string;
	order: number;
	collapsible: boolean;
	collapsed: boolean; // Default collapsed state
	columns: 1 | 2 | 3;
	visibility: VisibilityRule;
	fields: LayoutFieldV2[];
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

// Get all visible sections with visible fields based on record data
export function getVisibleSections(
	layout: LayoutDataV2,
	record: Record<string, unknown>
): LayoutSectionV2[] {
	return layout.sections
		.filter((section) => evaluateVisibility(section.visibility, record))
		.map((section) => ({
			...section,
			fields: section.fields.filter((field) => evaluateVisibility(field.visibility, record))
		}))
		.filter((section) => section.fields.length > 0)
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
		.filter((s) => tab.sectionIds.includes(s.id) && evaluateVisibility(s.visibility, record))
		.map((s) => ({
			...s,
			fields: s.fields.filter((f) => evaluateVisibility(f.visibility, record))
		}))
		.filter((s) => s.fields.length > 0)
		.sort((a, b) => a.order - b.order);
}

// Get all field names from a layout
export function getAllFieldNames(layout: LayoutDataV2): string[] {
	return layout.sections.flatMap((section) => section.fields.map((field) => field.name));
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
