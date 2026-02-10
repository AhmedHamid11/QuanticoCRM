<script lang="ts">
	import Button from './Button.svelte';

	interface Props {
		message?: string;
		title?: string;
		onRetry?: () => void;
		showHomeLink?: boolean;
		class?: string;
	}

	let {
		message = 'Something went wrong. Please try again.',
		title = 'Error',
		onRetry,
		showHomeLink = false,
		class: className = ''
	}: Props = $props();
</script>

<div class="text-center py-12 px-4 {className}">
	<!-- Error icon -->
	<div class="mx-auto w-16 h-16 rounded-full bg-red-100 flex items-center justify-center mb-4">
		<svg class="w-8 h-8 text-red-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
			<path
				stroke-linecap="round"
				stroke-linejoin="round"
				stroke-width="2"
				d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
			/>
		</svg>
	</div>

	<h3 class="text-lg font-medium text-gray-900 mb-2">{title}</h3>
	<p class="text-gray-500 mb-6 max-w-md mx-auto">{message}</p>

	<div class="flex justify-center gap-3">
		{#if onRetry}
			<Button variant="primary" onclick={onRetry}>
				Try Again
			</Button>
		{/if}
		{#if showHomeLink}
			<Button variant="secondary" onclick={() => window.location.href = '/'}>
				Go Home
			</Button>
		{/if}
	</div>
</div>
