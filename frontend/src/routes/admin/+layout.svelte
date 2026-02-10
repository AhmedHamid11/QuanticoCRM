<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';

	let { children } = $props();

	// Track whether we've done the initial check
	let hasCheckedAccess = $state(false);

	// Redirect non-admins to home page
	$effect(() => {
		// Wait for auth to finish loading before checking
		if (!auth.isLoading && auth.isAuthenticated) {
			if (!auth.canAccessSetup) {
				goto('/');
			} else {
				hasCheckedAccess = true;
			}
		}
	});
</script>

{#if auth.isLoading}
	<!-- Loading state -->
	<div class="flex items-center justify-center py-12">
		<div class="text-center">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500 mx-auto"></div>
			<p class="mt-2 text-gray-600">Loading...</p>
		</div>
	</div>
{:else if !auth.canAccessSetup}
	<!-- Access denied -->
	<div class="flex items-center justify-center py-12">
		<div class="text-center">
			<svg class="mx-auto h-12 w-12 text-red-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
			</svg>
			<h2 class="mt-4 text-lg font-medium text-gray-900">Access Denied</h2>
			<p class="mt-2 text-gray-600">You don't have permission to access the Setup area.</p>
			<p class="mt-1 text-sm text-gray-500">Only organization admins and owners can access this section.</p>
			<a
				href="/"
				class="mt-4 inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md text-white bg-blue-600 hover:bg-blue-600/90"
			>
				Return to Home
			</a>
		</div>
	</div>
{:else}
	{@render children()}
{/if}
