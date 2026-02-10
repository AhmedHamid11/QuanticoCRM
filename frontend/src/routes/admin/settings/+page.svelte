<script lang="ts">
	import { onMount } from 'svelte';
	import { get, put } from '$lib/utils/api';
	import { addToast } from '$lib/stores/toast.svelte';
	import { getNavigationTabs, loadNavigation } from '$lib/stores/navigation.svelte';

	interface OrgSettings {
		orgId: string;
		homePage: string;
	}

	let settings = $state<OrgSettings | null>(null);
	let loading = $state(true);
	let saving = $state(false);
	let selectedHomePage = $state('/');

	// Get navigation tabs for homepage options
	let navTabs = $derived(getNavigationTabs());

	onMount(async () => {
		try {
			// Ensure navigation tabs are loaded for the homepage selector
			await loadNavigation();
			settings = await get<OrgSettings>('/settings');
			selectedHomePage = settings?.homePage || '/';
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to load settings';
			addToast(message, 'error');
		} finally {
			loading = false;
		}
	});

	async function saveHomePage() {
		saving = true;
		try {
			const result = await put<OrgSettings>('/admin/settings/homepage', {
				homePage: selectedHomePage
			});
			settings = result;
			addToast('Homepage updated successfully', 'success');
			// Reload navigation to update cached settings
			await loadNavigation();
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to save homepage';
			addToast(message, 'error');
		} finally {
			saving = false;
		}
	}
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<h1 class="text-2xl font-bold text-gray-900">Organization Settings</h1>
		<a href="/admin" class="text-sm text-blue-600 hover:text-blue-800">
			&larr; Back to Admin
		</a>
	</div>

	{#if loading}
		<div class="bg-white shadow rounded-lg p-6">
			<div class="animate-pulse space-y-4">
				<div class="h-4 bg-gray-200 rounded w-1/4"></div>
				<div class="h-10 bg-gray-200 rounded w-1/2"></div>
			</div>
		</div>
	{:else}
		<div class="bg-white shadow rounded-lg p-6">
			<h2 class="text-lg font-medium text-gray-900 mb-4">Homepage</h2>
			<p class="text-sm text-gray-600 mb-4">
				Choose which page users see when they first log in or navigate to the root URL.
			</p>

			<div class="flex items-end gap-4">
				<div class="flex-1 max-w-md">
					<label for="homepage" class="block text-sm font-medium text-gray-700 mb-1">
						Default Homepage
					</label>
					<select
						id="homepage"
						bind:value={selectedHomePage}
						class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
					>
						<option value="/">Welcome Page (default)</option>
						{#each navTabs as tab (tab.id)}
							<option value={tab.href}>{tab.label}</option>
						{/each}
					</select>
				</div>

				<button
					onclick={saveHomePage}
					disabled={saving || selectedHomePage === (settings?.homePage || '/')}
					class="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-600/90 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					{saving ? 'Saving...' : 'Save'}
				</button>
			</div>

			{#if settings?.homePage && settings.homePage !== '/'}
				<p class="mt-4 text-sm text-gray-500">
					Current homepage: <code class="bg-gray-100 px-1 rounded">{settings.homePage}</code>
				</p>
			{/if}
		</div>
	{/if}
</div>
