<script lang="ts">
	import type { EndRedirect } from '$lib/types/flow';

	interface Props {
		message: string;
		redirect?: EndRedirect;
	}

	let { message, redirect }: Props = $props();
</script>

<div class="text-center py-8">
	<!-- Success icon -->
	<div class="mx-auto flex items-center justify-center h-16 w-16 rounded-full bg-green-100 mb-4">
		<svg class="h-8 w-8 text-green-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
			<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
		</svg>
	</div>

	<!-- Message -->
	<h3 class="text-lg font-medium text-gray-900 mb-2">Complete!</h3>
	<p class="text-gray-600 mb-4">{message}</p>

	<!-- Redirect info -->
	{#if redirect}
		<div class="flex items-center justify-center text-sm text-gray-500">
			<svg class="animate-spin h-4 w-4 mr-2 text-primary" fill="none" viewBox="0 0 24 24">
				<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
				<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
			</svg>
			Redirecting to {redirect.entity}...
		</div>
	{/if}
</div>
