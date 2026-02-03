<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { get } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import PageComponentRenderer from '$lib/components/PageComponentRenderer.svelte';
	import type { CustomPage } from '$lib/types/custom-page';

	let slug = $derived($page.params.slug);
	let customPage = $state<CustomPage | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	async function loadPage() {
		try {
			loading = true;
			error = null;
			customPage = await get<CustomPage>(`/pages/by-slug/${slug}`);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load page';
			toast.error(error);
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		loadPage();
	});

	// Reload when slug changes
	$effect(() => {
		if (slug) {
			loadPage();
		}
	});

	// Get grid class based on component width
	function getWidthClass(width: string): string {
		switch (width) {
			case '1/2':
				return 'md:col-span-6';
			case '1/3':
				return 'md:col-span-4';
			case '2/3':
				return 'md:col-span-8';
			default:
				return 'md:col-span-12';
		}
	}
</script>

<svelte:head>
	<title>{customPage?.title || 'Loading...'} - QuanticoCRM</title>
</svelte:head>

{#if loading}
	<div class="flex items-center justify-center min-h-[400px]">
		<div class="text-center">
			<svg class="animate-spin h-8 w-8 text-primary mx-auto mb-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
				<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
				<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
			</svg>
			<p class="text-gray-500">Loading page...</p>
		</div>
	</div>
{:else if error}
	<div class="flex items-center justify-center min-h-[400px]">
		<div class="text-center">
			<svg class="h-12 w-12 text-red-500 mx-auto mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
			</svg>
			<h2 class="text-xl font-semibold text-gray-900 mb-2">Page Not Found</h2>
			<p class="text-gray-500">{error}</p>
			<a href="/" class="mt-4 inline-block text-primary hover:text-blue-800">Go to Home</a>
		</div>
	</div>
{:else if customPage}
	<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
		<!-- Page Header -->
		<div class="mb-6">
			<h1 class="text-2xl font-bold text-gray-900">{customPage.title}</h1>
			{#if customPage.description}
				<p class="mt-1 text-sm text-gray-500">{customPage.description}</p>
			{/if}
		</div>

		<!-- Components Grid -->
		{#if customPage.components && customPage.components.length > 0}
			<div class="grid grid-cols-12 gap-6">
				{#each customPage.components.sort((a, b) => a.order - b.order) as component (component.id)}
					<div class="col-span-12 {getWidthClass(component.width)}">
						<PageComponentRenderer {component} />
					</div>
				{/each}
			</div>
		{:else}
			<div class="text-center py-12 bg-gray-50 rounded-lg border-2 border-dashed border-gray-300">
				<svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 13h6m-3-3v6m5 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
				</svg>
				<h3 class="mt-2 text-sm font-medium text-gray-900">No components</h3>
				<p class="mt-1 text-sm text-gray-500">This page has no components yet.</p>
			</div>
		{/if}
	</div>
{/if}
