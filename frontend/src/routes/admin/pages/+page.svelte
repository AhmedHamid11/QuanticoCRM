<script lang="ts">
	import { onMount } from 'svelte';
	import { get, post, del } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import type { CustomPageListItem } from '$lib/types/custom-page';

	let pages = $state<CustomPageListItem[]>([]);
	let loading = $state(true);
	let showNewModal = $state(false);
	let newPage = $state({ slug: '', title: '', description: '', isEnabled: true, isPublic: true });
	let creating = $state(false);

	async function loadPages() {
		try {
			loading = true;
			pages = await get<CustomPageListItem[]>('/admin/pages');
		} catch (err) {
			toast.error('Failed to load pages');
		} finally {
			loading = false;
		}
	}

	async function createPage() {
		if (!newPage.slug || !newPage.title) {
			toast.error('Slug and title are required');
			return;
		}

		try {
			creating = true;
			const created = await post<CustomPageListItem>('/admin/pages', newPage);
			pages = [...pages, created];
			showNewModal = false;
			newPage = { slug: '', title: '', description: '', isEnabled: true, isPublic: true };
			toast.success('Page created');
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to create page');
		} finally {
			creating = false;
		}
	}

	async function deletePage(id: string) {
		if (!confirm('Are you sure you want to delete this page?')) return;

		const backup = [...pages];
		pages = pages.filter(p => p.id !== id);

		try {
			await del(`/admin/pages/${id}`);
			toast.success('Page deleted');
		} catch (err) {
			pages = backup;
			toast.error('Failed to delete page');
		}
	}

	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleDateString('en-US', {
			month: 'short',
			day: 'numeric',
			year: 'numeric'
		});
	}

	// Auto-generate slug from title
	function handleTitleChange(e: Event) {
		const title = (e.target as HTMLInputElement).value;
		newPage.title = title;
		if (!newPage.slug || newPage.slug === slugify(newPage.title.slice(0, -1))) {
			newPage.slug = slugify(title);
		}
	}

	function slugify(text: string): string {
		return text
			.toLowerCase()
			.replace(/[^a-z0-9]+/g, '-')
			.replace(/^-|-$/g, '')
			.slice(0, 50);
	}

	onMount(() => {
		loadPages();
	});
</script>

<div class="space-y-6">
	<!-- Header -->
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold text-gray-900">Custom Pages</h1>
			<p class="mt-1 text-sm text-gray-500">Create pages with embedded iframes, text, and other components</p>
		</div>
		<button
			onclick={() => showNewModal = true}
			class="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-black bg-primary hover:bg-primary/90"
		>
			<svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
			</svg>
			New Page
		</button>
	</div>

	<!-- Pages List -->
	{#if loading}
		<div class="flex items-center justify-center py-12">
			<svg class="animate-spin h-8 w-8 text-primary" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
				<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
				<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
			</svg>
		</div>
	{:else if pages.length === 0}
		<div class="text-center py-12 bg-gray-50 rounded-lg border-2 border-dashed border-gray-300">
			<svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
			</svg>
			<h3 class="mt-2 text-sm font-medium text-gray-900">No custom pages</h3>
			<p class="mt-1 text-sm text-gray-500">Get started by creating a new page.</p>
			<button
				onclick={() => showNewModal = true}
				class="mt-4 inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-black bg-primary hover:bg-primary/90"
			>
				Create Page
			</button>
		</div>
	{:else}
		<div class="bg-white shadow rounded-lg overflow-hidden">
			<table class="min-w-full divide-y divide-gray-200">
				<thead class="bg-gray-50">
					<tr>
						<th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Page</th>
						<th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">URL</th>
						<th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Status</th>
						<th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Modified</th>
						<th scope="col" class="relative px-6 py-3">
							<span class="sr-only">Actions</span>
						</th>
					</tr>
				</thead>
				<tbody class="bg-white divide-y divide-gray-200">
					{#each pages as page (page.id)}
						<tr class="hover:bg-gray-50">
							<td class="px-6 py-4 whitespace-nowrap">
								<div class="flex items-center">
									<div class="flex-shrink-0 h-10 w-10 rounded-full bg-cyan-100 flex items-center justify-center">
										<svg class="h-5 w-5 text-cyan-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
											<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
										</svg>
									</div>
									<div class="ml-4">
										<div class="text-sm font-medium text-gray-900">{page.title}</div>
										{#if page.description}
											<div class="text-sm text-gray-500 truncate max-w-xs">{page.description}</div>
										{/if}
									</div>
								</div>
							</td>
							<td class="px-6 py-4 whitespace-nowrap">
								<a href="/p/{page.slug}" target="_blank" class="text-sm text-primary hover:text-blue-800 font-mono">
									/p/{page.slug}
								</a>
							</td>
							<td class="px-6 py-4 whitespace-nowrap">
								<div class="flex items-center gap-2">
									{#if page.isEnabled}
										<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">
											Enabled
										</span>
									{:else}
										<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800">
											Disabled
										</span>
									{/if}
									{#if page.isPublic}
										<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800">
											Public
										</span>
									{:else}
										<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-amber-100 text-amber-800">
											Admin Only
										</span>
									{/if}
								</div>
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{formatDate(page.modifiedAt)}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
								<a href="/admin/pages/{page.id}" class="text-primary hover:text-blue-900 mr-4">Edit</a>
								<button onclick={() => deletePage(page.id)} class="text-red-600 hover:text-red-900">Delete</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>

<!-- New Page Modal -->
{#if showNewModal}
	<div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
		<div class="bg-white rounded-lg shadow-xl w-full max-w-md p-6">
			<div class="flex items-center justify-between mb-4">
				<h3 class="text-lg font-medium text-gray-900">Create New Page</h3>
				<button onclick={() => showNewModal = false} class="text-gray-400 hover:text-gray-500">
					<svg class="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
					</svg>
				</button>
			</div>

			<form onsubmit={(e) => { e.preventDefault(); createPage(); }} class="space-y-4">
				<div>
					<label for="title" class="block text-sm font-medium text-gray-700">Title</label>
					<input
						type="text"
						id="title"
						value={newPage.title}
						oninput={handleTitleChange}
						class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
						placeholder="My Dashboard"
						required
					/>
				</div>

				<div>
					<label for="slug" class="block text-sm font-medium text-gray-700">URL Slug</label>
					<div class="mt-1 flex rounded-md shadow-sm">
						<span class="inline-flex items-center px-3 rounded-l-md border border-r-0 border-gray-300 bg-gray-50 text-gray-500 sm:text-sm">
							/p/
						</span>
						<input
							type="text"
							id="slug"
							bind:value={newPage.slug}
							class="flex-1 block w-full rounded-none rounded-r-md border-gray-300 focus:border-primary focus:ring-primary sm:text-sm font-mono"
							placeholder="my-dashboard"
							pattern="[a-z0-9-]+"
							required
						/>
					</div>
					<p class="mt-1 text-xs text-gray-500">Lowercase letters, numbers, and hyphens only</p>
				</div>

				<div>
					<label for="description" class="block text-sm font-medium text-gray-700">Description (optional)</label>
					<textarea
						id="description"
						bind:value={newPage.description}
						rows="2"
						class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
						placeholder="A brief description of this page"
					></textarea>
				</div>

				<div class="flex items-center gap-6">
					<label class="flex items-center">
						<input type="checkbox" bind:checked={newPage.isEnabled} class="rounded border-gray-300 text-primary focus:ring-primary" />
						<span class="ml-2 text-sm text-gray-700">Enabled</span>
					</label>
					<label class="flex items-center">
						<input type="checkbox" bind:checked={newPage.isPublic} class="rounded border-gray-300 text-primary focus:ring-primary" />
						<span class="ml-2 text-sm text-gray-700">Public (visible to all users)</span>
					</label>
				</div>

				<div class="flex justify-end gap-3 pt-4">
					<button
						type="button"
						onclick={() => showNewModal = false}
						class="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-50"
					>
						Cancel
					</button>
					<button
						type="submit"
						disabled={creating}
						class="px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-black bg-primary hover:bg-primary/90 disabled:opacity-50"
					>
						{creating ? 'Creating...' : 'Create Page'}
					</button>
				</div>
			</form>
		</div>
	</div>
{/if}
