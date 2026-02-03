<script lang="ts">
	import { page } from '$app/stores';
	import { Button } from '$lib/components/ui';

	let status = $derived($page.status);
	let message = $derived($page.error?.message || 'Something went wrong');

	const errorInfo: Record<number, { title: string; description: string; icon: string }> = {
		404: {
			title: 'Page Not Found',
			description: 'The page you are looking for does not exist or has been moved.',
			icon: 'search'
		},
		403: {
			title: 'Access Denied',
			description: 'You do not have permission to access this resource.',
			icon: 'lock'
		},
		500: {
			title: 'Server Error',
			description: 'An unexpected error occurred. Please try again later.',
			icon: 'alert'
		}
	};

	let info = $derived(errorInfo[status] || errorInfo[500]);

	function handleRetry() {
		window.location.reload();
	}

	function handleGoHome() {
		window.location.href = '/';
	}
</script>

<div class="min-h-[60vh] flex items-center justify-center px-4">
	<div class="max-w-md w-full text-center">
		<!-- Icon -->
		<div class="mx-auto w-20 h-20 rounded-full bg-gray-100 flex items-center justify-center mb-6">
			{#if info.icon === 'search'}
				<svg class="w-10 h-10 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
				</svg>
			{:else if info.icon === 'lock'}
				<svg class="w-10 h-10 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
				</svg>
			{:else}
				<svg class="w-10 h-10 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
				</svg>
			{/if}
		</div>

		<!-- Status code -->
		<p class="text-6xl font-bold text-gray-300 mb-4">{status}</p>

		<!-- Title -->
		<h1 class="text-2xl font-bold text-gray-900 mb-2">{info.title}</h1>

		<!-- Description -->
		<p class="text-gray-500 mb-8">{info.description}</p>

		{#if message && message !== info.description}
			<p class="text-sm text-gray-400 mb-6 px-4 py-2 bg-gray-50 rounded-md font-mono">
				{message}
			</p>
		{/if}

		<!-- Actions -->
		<div class="flex justify-center gap-3">
			{#if status === 500}
				<Button variant="primary" onclick={handleRetry}>
					Try Again
				</Button>
			{/if}
			<Button variant={status === 500 ? 'secondary' : 'primary'} onclick={handleGoHome}>
				Go Home
			</Button>
		</div>
	</div>
</div>
