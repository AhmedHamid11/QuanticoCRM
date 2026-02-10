import { ApiError } from '$lib/utils/api';
import type { FieldValidationError } from '$lib/types/validation';

export interface FormErrors {
	fieldErrors: Map<string, string>;
	generalError: string | null;
	getFieldError: (fieldName: string) => string | null;
	setFieldError: (fieldName: string, message: string) => void;
	setFromApiError: (error: unknown) => void;
	clearField: (fieldName: string) => void;
	clearAll: () => void;
	hasErrors: () => boolean;
	hasFieldError: (fieldName: string) => boolean;
}

/**
 * Creates a reactive form errors state object.
 *
 * Usage:
 * ```svelte
 * <script>
 *   import { useFormErrors } from '$lib/hooks/useFormErrors.svelte';
 *
 *   const formErrors = useFormErrors();
 *
 *   async function handleSubmit() {
 *     formErrors.clearAll();
 *     try {
 *       await api.post('/contacts', data);
 *     } catch (err) {
 *       formErrors.setFromApiError(err);
 *     }
 *   }
 * </script>
 *
 * <FormField error={formErrors.getFieldError('email')}>
 *   <input type="email" bind:value={email} />
 * </FormField>
 * ```
 */
export function useFormErrors(): FormErrors {
	let fieldErrors = $state(new Map<string, string>());
	let generalError = $state<string | null>(null);

	function getFieldError(fieldName: string): string | null {
		return fieldErrors.get(fieldName) || null;
	}

	function setFieldError(fieldName: string, message: string): void {
		const newMap = new Map(fieldErrors);
		newMap.set(fieldName, message);
		fieldErrors = newMap;
	}

	function setFromApiError(error: unknown): void {
		// Clear previous errors
		fieldErrors = new Map();
		generalError = null;

		if (error instanceof ApiError) {
			// Handle field-level validation errors (422)
			if (error.fieldErrors && error.fieldErrors.length > 0) {
				const newMap = new Map<string, string>();
				for (const fieldError of error.fieldErrors) {
					// Use the field name as key - might need to convert to camelCase
					const fieldName = fieldError.field;
					newMap.set(fieldName, fieldError.message);
				}
				fieldErrors = newMap;
				generalError = error.message;
			} else {
				// General API error
				generalError = error.message;
			}
		} else if (error instanceof Error) {
			generalError = error.message;
		} else {
			generalError = 'An unexpected error occurred';
		}
	}

	function clearField(fieldName: string): void {
		if (fieldErrors.has(fieldName)) {
			const newMap = new Map(fieldErrors);
			newMap.delete(fieldName);
			fieldErrors = newMap;
		}
	}

	function clearAll(): void {
		fieldErrors = new Map();
		generalError = null;
	}

	function hasErrors(): boolean {
		return fieldErrors.size > 0 || generalError !== null;
	}

	function hasFieldError(fieldName: string): boolean {
		return fieldErrors.has(fieldName);
	}

	return {
		get fieldErrors() {
			return fieldErrors;
		},
		get generalError() {
			return generalError;
		},
		getFieldError,
		setFieldError,
		setFromApiError,
		clearField,
		clearAll,
		hasErrors,
		hasFieldError
	};
}
