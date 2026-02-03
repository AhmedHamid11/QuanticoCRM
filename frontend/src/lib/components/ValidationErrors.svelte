<script lang="ts">
	import type { FieldValidationError } from '$lib/types/validation';

	interface Props {
		errors: FieldValidationError[];
		onFieldClick?: (fieldName: string) => void;
	}

	let { errors, onFieldClick }: Props = $props();

	// Group errors by field, filtering out form-level errors for the list
	let fieldErrors = $derived(errors.filter((e) => e.field !== '_form'));
	let formErrors = $derived(errors.filter((e) => e.field === '_form'));

	function handleFieldClick(fieldName: string) {
		if (onFieldClick) {
			onFieldClick(fieldName);
		} else {
			// Try to scroll to and focus the field
			const element =
				document.querySelector(`[name="${fieldName}"]`) ||
				document.querySelector(`#${fieldName}`) ||
				document.querySelector(`[data-field="${fieldName}"]`);

			if (element) {
				element.scrollIntoView({ behavior: 'smooth', block: 'center' });
				if (element instanceof HTMLElement) {
					element.focus();
				}
			}
		}
	}
</script>

{#if errors.length > 0}
	<div class="rounded-lg border border-red-200 bg-red-50 p-4 mb-4">
		<div class="flex items-start">
			<div class="flex-shrink-0">
				<svg
					class="h-5 w-5 text-red-400"
					viewBox="0 0 20 20"
					fill="currentColor"
					aria-hidden="true"
				>
					<path
						fill-rule="evenodd"
						d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.28 7.22a.75.75 0 00-1.06 1.06L8.94 10l-1.72 1.72a.75.75 0 101.06 1.06L10 11.06l1.72 1.72a.75.75 0 101.06-1.06L11.06 10l1.72-1.72a.75.75 0 00-1.06-1.06L10 8.94 8.28 7.22z"
						clip-rule="evenodd"
					/>
				</svg>
			</div>
			<div class="ml-3 flex-1">
				<h3 class="text-sm font-medium text-red-800">
					{errors.length === 1 ? 'Validation Error' : `${errors.length} Validation Errors`}
				</h3>

				{#if formErrors.length > 0}
					<div class="mt-2 text-sm text-red-700">
						{#each formErrors as error}
							<p>{error.message}</p>
						{/each}
					</div>
				{/if}

				{#if fieldErrors.length > 0}
					<ul class="mt-2 text-sm text-red-700 list-disc list-inside space-y-1">
						{#each fieldErrors as error}
							<li>
								<button
									type="button"
									class="font-medium text-red-800 underline hover:text-red-600 focus:outline-none focus:ring-2 focus:ring-red-500 focus:ring-offset-1 rounded"
									onclick={() => handleFieldClick(error.field)}
								>
									{error.field}
								</button>: {error.message}
							</li>
						{/each}
					</ul>
				{/if}
			</div>
		</div>
	</div>
{/if}
