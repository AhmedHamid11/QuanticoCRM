<script lang="ts">
	import { goto } from '$app/navigation';
	import { getHomePage, getNavigationTabs, getOrgSettings } from '$lib/stores/navigation.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import LoginForm from '$lib/components/LoginForm.svelte';

	let isRedirecting = $state(false);
	let hasChecked = $state(false);

	// Watch for org settings to be loaded, then redirect if needed (only when authenticated)
	$effect(() => {
		if (!auth.isAuthenticated) return;

		const settings = getOrgSettings();
		// Only check once when settings are loaded
		if (settings && !hasChecked) {
			hasChecked = true;
			const homePage = settings.homePage;
			if (homePage && homePage !== '/') {
				isRedirecting = true;
				goto(homePage);
			}
		}
	});

	// Get first navigation tab as fallback link
	let firstTab = $derived(getNavigationTabs()[0]);
</script>

<svelte:head>
	{#if !auth.isAuthenticated && !auth.isLoading}
		<title>Login - Quantico CRM</title>
	{/if}
</svelte:head>

{#if auth.isLoading}
	<!-- Loading state handled by layout -->
{:else if !auth.isAuthenticated}
	<!-- Show login form for unauthenticated users -->
	<LoginForm />
{:else if isRedirecting}
	<div class="text-center py-12">
		<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500 mx-auto"></div>
		<p class="mt-4 text-gray-600">Redirecting...</p>
	</div>
{:else}
	<div class="text-center py-12">
		<h1 class="text-4xl font-bold text-gray-900 mb-4">Welcome to <span class="text-red-800">Quantico</span><span class="text-amber-600">CRM</span></h1>
		<p class="text-lg text-gray-600 mb-8">A CRM built with discipline and precision</p>
		<a
			href={firstTab?.href || '/contacts'}
			class="inline-flex items-center px-6 py-3 border border-transparent text-base font-medium rounded-md shadow-sm text-white bg-blue-600 hover:bg-blue-600/90"
		>
			View {firstTab?.label || 'Contacts'}
		</a>
	</div>
{/if}
