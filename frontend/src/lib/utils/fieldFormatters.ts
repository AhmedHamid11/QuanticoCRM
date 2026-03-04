/**
 * Field Formatters — Utility functions for type-aware field rendering
 *
 * Used by FieldDisplay and InlineFieldEditor components.
 * Pure functions, no side effects, no dependencies on Svelte.
 */

import type { FieldDef } from '$lib/types/admin';
import { fieldNameToKey } from '$lib/utils/fieldMapping';

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

export const SYSTEM_READONLY_FIELDS = new Set([
	'id',
	'createdAt',
	'modifiedAt',
	'createdBy',
	'modifiedBy',
	'created_at',
	'modified_at',
	'created_by',
	'modified_by'
]);

export const READONLY_FIELD_TYPES = new Set<string>(['rollup', 'textBlock', 'stream']);

export const EDIT_PAGE_FIELD_TYPES = new Set<string>(['link', 'linkMultiple', 'address']);

// ---------------------------------------------------------------------------
// Color palette for enum badges (deterministic, 8-slot)
// ---------------------------------------------------------------------------

const ENUM_COLORS: Array<{ bg: string; text: string }> = [
	{ bg: 'bg-blue-100', text: 'text-blue-800' },
	{ bg: 'bg-green-100', text: 'text-green-800' },
	{ bg: 'bg-yellow-100', text: 'text-yellow-800' },
	{ bg: 'bg-purple-100', text: 'text-purple-800' },
	{ bg: 'bg-pink-100', text: 'text-pink-800' },
	{ bg: 'bg-indigo-100', text: 'text-indigo-800' },
	{ bg: 'bg-orange-100', text: 'text-orange-800' },
	{ bg: 'bg-teal-100', text: 'text-teal-800' }
];

// ---------------------------------------------------------------------------
// 1. formatCurrency
// ---------------------------------------------------------------------------

/**
 * Format a numeric value as a currency string.
 * Returns '—' for null/undefined/NaN.
 */
export function formatCurrency(value: unknown, currency?: string): string {
	if (value === null || value === undefined || value === '') return '—';
	const num = typeof value === 'number' ? value : parseFloat(String(value));
	if (isNaN(num)) return '—';
	return new Intl.NumberFormat('en-US', {
		style: 'currency',
		currency: currency || 'USD',
		minimumFractionDigits: 2
	}).format(num);
}

// ---------------------------------------------------------------------------
// 2. formatRelativeDate
// ---------------------------------------------------------------------------

/**
 * Format a date string as a relative time string ("2 days ago")
 * or absolute date ("Mar 3, 2026") beyond the threshold.
 *
 * @param dateStr ISO date string
 * @param thresholdDays Number of days within which relative format is used (default 7)
 */
export function formatRelativeDate(dateStr: string, thresholdDays = 7): string {
	if (!dateStr) return '—';
	const date = new Date(dateStr);
	if (isNaN(date.getTime())) return String(dateStr);

	const now = new Date();
	const diffMs = now.getTime() - date.getTime();
	const diffDays = diffMs / (1000 * 60 * 60 * 24);

	if (Math.abs(diffDays) <= thresholdDays) {
		const rtf = new Intl.RelativeTimeFormat('en', { numeric: 'auto' });

		if (Math.abs(diffDays) < 1) {
			const diffHours = diffMs / (1000 * 60 * 60);
			return rtf.format(-Math.round(Math.abs(diffHours)), 'hour');
		}
		return rtf.format(-Math.round(Math.abs(diffDays)), 'day');
	}

	return date.toLocaleDateString('en-US', {
		month: 'short',
		day: 'numeric',
		year: 'numeric'
	});
}

// ---------------------------------------------------------------------------
// 3. formatDateTooltip
// ---------------------------------------------------------------------------

/**
 * Return a full human-readable date string for hover tooltips.
 *
 * @param dateStr ISO date string
 * @param includeTime Whether to include the time portion
 */
export function formatDateTooltip(dateStr: string, includeTime: boolean): string {
	if (!dateStr) return '';
	const date = new Date(dateStr);
	if (isNaN(date.getTime())) return String(dateStr);

	if (includeTime) {
		return date.toLocaleDateString('en-US', {
			month: 'long',
			day: 'numeric',
			year: 'numeric',
			hour: 'numeric',
			minute: '2-digit'
		});
	}
	return date.toLocaleDateString('en-US', {
		month: 'long',
		day: 'numeric',
		year: 'numeric'
	});
}

// ---------------------------------------------------------------------------
// 4. getEnumColor
// ---------------------------------------------------------------------------

/**
 * Return deterministic Tailwind color classes for an enum option.
 * Uses a simple hash so the same option always gets the same color.
 */
export function getEnumColor(option: string): { bg: string; text: string } {
	let hash = 0;
	for (let i = 0; i < option.length; i++) {
		hash = (hash * 31 + option.charCodeAt(i)) & 0xffff;
	}
	return ENUM_COLORS[hash % ENUM_COLORS.length];
}

// ---------------------------------------------------------------------------
// 5. getEnumOptions
// ---------------------------------------------------------------------------

/**
 * Parse enum/multiEnum options from a FieldDef's options string.
 * Accepts JSON array format ("[\"a\",\"b\"]") or comma-separated ("a, b").
 */
export function getEnumOptions(field: { options?: string | null }): string[] {
	if (!field.options) return [];
	const opts = field.options.trim();
	if (opts.startsWith('[')) {
		try {
			return JSON.parse(opts) as string[];
		} catch {
			return [];
		}
	}
	return opts.split(',').map((o) => o.trim());
}

// ---------------------------------------------------------------------------
// 6. parseMultiEnumValue
// ---------------------------------------------------------------------------

/**
 * Parse a multiEnum value that may be stored as a JSON array string or actual array.
 * Returns [] for falsy values.
 */
export function parseMultiEnumValue(value: unknown): string[] {
	if (!value) return [];
	if (Array.isArray(value)) return value as string[];
	if (typeof value === 'string' && value.startsWith('[')) {
		try {
			return JSON.parse(value) as string[];
		} catch {
			return [];
		}
	}
	return [];
}

// ---------------------------------------------------------------------------
// 7. getInputType
// ---------------------------------------------------------------------------

/**
 * Map a FieldType to the corresponding HTML input type attribute.
 */
export function getInputType(fieldType: string): string {
	switch (fieldType) {
		case 'email':
			return 'email';
		case 'url':
			return 'url';
		case 'phone':
			return 'tel';
		case 'int':
		case 'float':
		case 'currency':
			return 'number';
		case 'date':
			return 'date';
		case 'datetime':
			return 'datetime-local';
		default:
			return 'text';
	}
}

// ---------------------------------------------------------------------------
// 8. isInlineEditable
// ---------------------------------------------------------------------------

/**
 * Determine whether a field supports inline editing on the detail page.
 *
 * Returns false for:
 * - Fields marked isReadOnly
 * - System metadata fields (id, createdAt, etc.)
 * - Read-only field types (rollup, textBlock, stream)
 * - Field types that require the full edit page (link, linkMultiple, address)
 */
export function isInlineEditable(field: FieldDef, fieldName: string): boolean {
	if (field.isReadOnly) return false;
	if (SYSTEM_READONLY_FIELDS.has(fieldName)) return false;
	if (SYSTEM_READONLY_FIELDS.has(fieldNameToKey(fieldName))) return false;
	if (READONLY_FIELD_TYPES.has(field.type)) return false;
	if (EDIT_PAGE_FIELD_TYPES.has(field.type)) return false;
	return true;
}
