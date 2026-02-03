<script lang="ts">
	import FieldError from './FieldError.svelte';
	import type { Snippet } from 'svelte';

	interface Props {
		label?: string;
		htmlFor?: string;
		required?: boolean;
		error?: string | null;
		hint?: string;
		class?: string;
		children: Snippet;
	}

	let {
		label,
		htmlFor,
		required = false,
		error = null,
		hint,
		class: className = '',
		children
	}: Props = $props();

	let hasError = $derived(!!error);
</script>

<div class={className}>
	{#if label}
		<label for={htmlFor} class="block text-sm font-medium text-gray-700 mb-1">
			{label}
			{#if required}
				<span class="text-red-500">*</span>
			{/if}
		</label>
	{/if}

	<div class={hasError ? '[&_input]:border-red-300 [&_input]:focus:border-red-500 [&_input]:focus:ring-red-500 [&_textarea]:border-red-300 [&_select]:border-red-300' : ''}>
		{@render children()}
	</div>

	<FieldError message={error} />

	{#if hint && !error}
		<p class="mt-1 text-xs text-gray-500">{hint}</p>
	{/if}
</div>
