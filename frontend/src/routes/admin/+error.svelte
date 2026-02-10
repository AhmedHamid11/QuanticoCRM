<script lang="ts">
	import { page } from '$app/stores';
	import { Button } from '$lib/components/ui';

	let status = $derived($page.status);
	let message = $derived($page.error?.message || 'Something went wrong');

	const errorInfo: Record<number, { title: string; description: string }> = {
		404: {
			title: 'Page Not Found',
			description: 'The admin page you are looking for does not exist.'
		},
		403: {
			title: 'Access Denied',
			description: 'You do not have permission to access this admin feature.'
		},
		500: {
			title: 'Server Error',
			description: 'An unexpected error occurred in the admin panel.'
		}
	};

	let info = $derived(errorInfo[status] || errorInfo[500]);

	function handleRetry() {
		window.location.reload();
	}

	function handleGoBack() {
		window.history.back();
	}

	function handleGoToAdmin() {
		window.location.href = '/admin';
	}
</script>

<div class="max-w-lg mx-auto py-12 text-center">
	<!-- Icon -->
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

	<!-- Status badge -->
	<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-800 mb-4">
		Error {status}
	</span>

	<!-- Title -->
	<h1 class="text-xl font-bold text-gray-900 mb-2">{info.title}</h1>

	<!-- Description -->
	<p class="text-gray-500 mb-6">{info.description}</p>

	{#if message && message !== info.description}
		<div class="mb-6 px-4 py-3 bg-gray-50 rounded-md">
			<p class="text-sm text-gray-600 font-mono">{message}</p>
		</div>
	{/if}

	<!-- Actions -->
	<div class="flex justify-center gap-3">
		{#if status === 500}
			<Button variant="primary" onclick={handleRetry}>
				Try Again
			</Button>
		{/if}
		<Button variant="secondary" onclick={handleGoBack}>
			Go Back
		</Button>
		<Button variant="ghost" onclick={handleGoToAdmin}>
			Admin Home
		</Button>
	</div>
</div>
