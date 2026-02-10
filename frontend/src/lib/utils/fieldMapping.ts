/**
 * Field Mapping Utilities
 *
 * Handles the conversion between snake_case field names (used in field definitions)
 * and camelCase keys (used in API responses).
 *
 * WHY THIS EXISTS:
 * - Field definitions store names in snake_case (e.g., "first_name")
 * - Backend API returns record data in camelCase (e.g., "firstName")
 * - This mismatch caused bugs where field values wouldn't display after save
 *
 * USAGE:
 * - Import these utilities in any component that loads/saves entity data
 * - Use `fieldNameToKey()` when accessing record values by field name
 * - Use `mapRecordToFormData()` when initializing form state from API response
 */

/**
 * Convert snake_case field name to camelCase key
 * Example: "first_name" -> "firstName"
 */
export function fieldNameToKey(fieldName: string): string {
	return fieldName.replace(/_([a-z])/g, (_, letter) => letter.toUpperCase());
}

/**
 * Convert camelCase key to snake_case field name
 * Example: "firstName" -> "first_name"
 */
export function keyToFieldName(key: string): string {
	return key.replace(/[A-Z]/g, (letter) => `_${letter.toLowerCase()}`);
}

/**
 * Get a value from a record, trying camelCase key first then original field name
 * This handles both API responses (camelCase) and direct field access (snake_case)
 */
export function getRecordValue(record: Record<string, unknown>, fieldName: string): unknown {
	const camelKey = fieldNameToKey(fieldName);
	if (camelKey in record) {
		return record[camelKey];
	}
	return record[fieldName];
}

/**
 * Options for mapRecordToFormData
 */
interface MapOptions {
	/** Set of system field names (in camelCase) that exist directly on the record */
	systemFields?: Set<string>;
	/** Custom fields object from the record (for entities that separate system/custom fields) */
	customFields?: Record<string, unknown>;
}

/**
 * Map API record data to form data keyed by field names
 *
 * This function handles the snake_case/camelCase mismatch by:
 * 1. For each field definition, convert field.name to camelCase
 * 2. Look up the value in the record using the camelCase key
 * 3. Store in formData using the original field.name (snake_case)
 *
 * @param fields - Array of field definitions
 * @param record - Record data from API (uses camelCase keys)
 * @param options - Optional configuration for system/custom field handling
 * @returns Form data object keyed by field names
 */
export function mapRecordToFormData(
	fields: Array<{ name: string; type?: string }>,
	record: Record<string, unknown>,
	options?: MapOptions
): Record<string, unknown> {
	const formData: Record<string, unknown> = {};
	const systemFields = options?.systemFields;
	const customFields = options?.customFields;

	for (const field of fields) {
		const camelKey = fieldNameToKey(field.name);
		const isSystem = systemFields ? systemFields.has(camelKey) : true;

		if (isSystem || !customFields) {
			// System field or no custom fields separation - get from record directly
			formData[field.name] = record[camelKey] ?? record[field.name] ?? '';
		} else {
			// Custom field - try camelCase first, then original name
			formData[field.name] = customFields[camelKey] ?? customFields[field.name] ?? '';
		}

		// Handle lookup field display names
		if (field.type === 'link') {
			const nameKey = `${camelKey}Name`;
			const nameFieldName = `${field.name}Name`;
			if (isSystem || !customFields) {
				formData[nameFieldName] = record[nameKey] ?? record[nameFieldName] ?? '';
			} else {
				formData[nameFieldName] = customFields[nameKey] ?? customFields[nameFieldName] ?? '';
			}
		}

		// Handle multi-lookup field values
		if (field.type === 'linkMultiple') {
			const idsKey = `${camelKey}Ids`;
			const namesKey = `${camelKey}Names`;
			const idsFieldName = `${field.name}Ids`;
			const namesFieldName = `${field.name}Names`;

			const source = (isSystem || !customFields) ? record : customFields;
			formData[idsFieldName] = source[idsKey] ?? source[idsFieldName] ?? '[]';
			formData[namesFieldName] = source[namesKey] ?? source[namesFieldName] ?? '[]';
		}
	}

	return formData;
}

/**
 * Prepare form data for API submission
 *
 * Converts form data (keyed by snake_case field names) to API payload
 * (keyed by camelCase for system fields, original names for custom fields)
 *
 * @param formData - Form data keyed by field names
 * @param systemFields - Set of system field names (in camelCase)
 * @returns Object with { payload, customFields } for API submission
 */
export function preparePayload(
	formData: Record<string, unknown>,
	systemFields?: Set<string>
): { payload: Record<string, unknown>; customFields: Record<string, unknown> } {
	const payload: Record<string, unknown> = {};
	const customFields: Record<string, unknown> = {};

	for (const [fieldName, value] of Object.entries(formData)) {
		const camelKey = fieldNameToKey(fieldName);
		const isSystem = systemFields ? systemFields.has(camelKey) : true;

		if (isSystem || !systemFields) {
			payload[camelKey] = value;
		} else {
			customFields[fieldName] = value;
		}
	}

	return { payload, customFields };
}
