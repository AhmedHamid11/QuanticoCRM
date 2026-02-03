<script lang="ts">
	import type { ScreenStep, SubmitScreenRequest } from '$lib/types/flow';
	import FlowField from './FlowField.svelte';
	import Spinner from '$lib/components/ui/Spinner.svelte';

	interface Props {
		screen: ScreenStep;
		variables: Record<string, unknown>;
		submitting: boolean;
		onSubmit: (data: SubmitScreenRequest) => void;
	}

	let { screen, variables, submitting, onSubmit }: Props = $props();

	let formData = $state<Record<string, unknown>>({});
	let errors = $state<Record<string, string>>({});

	// Interpolate template variables like {{variableName}}
	function interpolate(template: string | undefined): string {
		if (!template) return '';
		return template.replace(/\{\{(\w+)\}\}/g, (_, varName) => {
			const value = variables[varName];
			if (value === undefined || value === null) return '';
			if (typeof value === 'boolean') return value ? 'Yes' : 'No';
			return String(value);
		});
	}

	// Initialize form data with defaults
	$effect(() => {
		const initial: Record<string, unknown> = {};
		for (const field of screen.fields) {
			if (field.type === 'display') {
				// For display fields, evaluate the template value
				initial[field.name] = interpolate(field.value as string);
			} else {
				// Use default value, or variable value, or empty string
				initial[field.name] = field.defaultValue || variables[field.name] || '';
			}
		}
		formData = initial;
		errors = {};
	});

	// Interpolated header message
	let headerMessage = $derived(screen.header ? interpolate(screen.header.message) : '');

	function validate(): boolean {
		const newErrors: Record<string, string> = {};

		for (const field of screen.fields) {
			// Skip validation for display-only fields
			if (field.type === 'display') continue;

			const value = formData[field.name];

			if (field.required && (value === '' || value === null || value === undefined)) {
				newErrors[field.name] = `${field.label} is required`;
				continue;
			}

			// Type-specific validation
			if (value !== '' && value !== null && value !== undefined) {
				if (field.type === 'email' && typeof value === 'string') {
					if (!value.match(/^[^\s@]+@[^\s@]+\.[^\s@]+$/)) {
						newErrors[field.name] = 'Please enter a valid email address';
					}
				}
				if (field.type === 'url' && typeof value === 'string') {
					try {
						new URL(value);
					} catch {
						newErrors[field.name] = 'Please enter a valid URL';
					}
				}
				if ((field.type === 'number' || field.type === 'currency' || field.type === 'percent') && typeof value === 'number') {
					if (field.minValue !== undefined && value < field.minValue) {
						newErrors[field.name] = `Minimum value is ${field.minValue}`;
					}
					if (field.maxValue !== undefined && value > field.maxValue) {
						newErrors[field.name] = `Maximum value is ${field.maxValue}`;
					}
				}
			}
		}

		errors = newErrors;
		return Object.keys(newErrors).length === 0;
	}

	function handleSubmit(e: Event) {
		e.preventDefault();
		if (validate()) {
			onSubmit(formData as SubmitScreenRequest);
		}
	}

	function updateField(name: string, value: unknown) {
		formData = { ...formData, [name]: value };
		// Clear error when field is edited
		if (errors[name]) {
			const newErrors = { ...errors };
			delete newErrors[name];
			errors = newErrors;
		}
	}

	const headerClasses: Record<string, string> = {
		success: 'bg-green-50 text-green-800 border-green-200',
		warning: 'bg-yellow-50 text-yellow-800 border-yellow-200',
		error: 'bg-red-50 text-red-800 border-red-200',
		info: 'bg-blue-50 text-blue-800 border-blue-200'
	};

	const headerIcons: Record<string, string> = {
		success: 'M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z',
		warning: 'M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z',
		error: 'M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z',
		info: 'M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z'
	};
</script>

<form onsubmit={handleSubmit} class="space-y-6">
	<!-- Header alert if present -->
	{#if screen.header}
		<div class="p-4 rounded-lg border flex items-start gap-3 {headerClasses[screen.header.variant] || headerClasses.info}">
			<svg class="w-5 h-5 flex-shrink-0 mt-0.5" fill="currentColor" viewBox="0 0 20 20">
				<path fill-rule="evenodd" d={headerIcons[screen.header.variant] || headerIcons.info} clip-rule="evenodd" />
			</svg>
			<p class="text-sm font-medium">{headerMessage}</p>
		</div>
	{/if}

	<!-- Fields -->
	<div class="space-y-4">
		{#each screen.fields as field (field.name)}
			<FlowField
				{field}
				value={formData[field.name]}
				error={errors[field.name]}
				onchange={(value) => updateField(field.name, value)}
			/>
		{/each}
	</div>

	<!-- Actions -->
	<div class="flex justify-end gap-3 pt-4 border-t">
		<button
			type="submit"
			disabled={submitting}
			class="inline-flex items-center px-4 py-2 text-sm font-medium text-black bg-primary rounded-md hover:bg-primary/90 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary disabled:opacity-50 disabled:cursor-not-allowed"
		>
			{#if submitting}
				<Spinner size="sm" class="mr-2" />
				Processing...
			{:else}
				Next
				<svg class="ml-2 w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
				</svg>
			{/if}
		</button>
	</div>
</form>
