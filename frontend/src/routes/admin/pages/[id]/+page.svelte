<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { get, put } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import type { CustomPage, PageComponent, ComponentType, ComponentWidth, IframeConfig, TextConfig, HTMLConfig, LinkGroupConfig, StatsConfig, EntityListConfig } from '$lib/types/custom-page';
	import { COMPONENT_TYPES, COMPONENT_WIDTHS } from '$lib/types/custom-page';

	let pageId = $derived($page.params.id);
	let customPage = $state<CustomPage | null>(null);
	let loading = $state(true);
	let saving = $state(false);

	// Component editor state
	let showComponentModal = $state(false);
	let editingComponentIndex = $state<number | null>(null);
	let componentForm = $state<Partial<PageComponent>>({
		type: 'iframe',
		title: '',
		width: 'full',
		config: {}
	});

	async function loadPage() {
		try {
			loading = true;
			customPage = await get<CustomPage>(`/admin/pages/${pageId}`);
		} catch (err) {
			toast.error('Failed to load page');
			goto('/admin/pages');
		} finally {
			loading = false;
		}
	}

	async function savePage() {
		if (!customPage) return;

		try {
			saving = true;
			await put(`/admin/pages/${pageId}`, {
				title: customPage.title,
				slug: customPage.slug,
				description: customPage.description,
				icon: customPage.icon,
				isEnabled: customPage.isEnabled,
				isPublic: customPage.isPublic,
				layout: customPage.layout,
				components: customPage.components
			});
			toast.success('Page saved');
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to save page');
		} finally {
			saving = false;
		}
	}

	function openAddComponent() {
		editingComponentIndex = null;
		componentForm = {
			type: 'iframe',
			title: '',
			width: 'full',
			config: { url: '', height: 400 }
		};
		showComponentModal = true;
	}

	function openEditComponent(index: number) {
		if (!customPage) return;
		const comp = customPage.components[index];
		editingComponentIndex = index;
		componentForm = {
			type: comp.type,
			title: comp.title || '',
			width: comp.width,
			config: { ...comp.config }
		};
		showComponentModal = true;
	}

	function saveComponent() {
		if (!customPage || !componentForm.type) return;

		const newComponent: PageComponent = {
			id: editingComponentIndex !== null
				? customPage.components[editingComponentIndex].id
				: `comp_${Date.now()}`,
			type: componentForm.type as ComponentType,
			title: componentForm.title || '',
			width: (componentForm.width as ComponentWidth) || 'full',
			order: editingComponentIndex !== null
				? customPage.components[editingComponentIndex].order
				: customPage.components.length,
			config: componentForm.config as any
		};

		if (editingComponentIndex !== null) {
			customPage.components[editingComponentIndex] = newComponent;
		} else {
			customPage.components = [...customPage.components, newComponent];
		}

		showComponentModal = false;
		componentForm = { type: 'iframe', title: '', width: 'full', config: {} };
		editingComponentIndex = null;
	}

	function deleteComponent(index: number) {
		if (!customPage) return;
		if (!confirm('Delete this component?')) return;
		customPage.components = customPage.components.filter((_, i) => i !== index);
	}

	function moveComponent(index: number, direction: 'up' | 'down') {
		if (!customPage) return;
		const newIndex = direction === 'up' ? index - 1 : index + 1;
		if (newIndex < 0 || newIndex >= customPage.components.length) return;

		const components = [...customPage.components];
		[components[index], components[newIndex]] = [components[newIndex], components[index]];
		// Update order values
		components.forEach((c, i) => c.order = i);
		customPage.components = components;
	}

	// Initialize config based on component type
	function handleTypeChange(type: ComponentType) {
		componentForm.type = type;
		switch (type) {
			case 'iframe':
				componentForm.config = { url: '', height: 400 };
				break;
			case 'text':
			case 'markdown':
				componentForm.config = { content: '' };
				break;
			case 'html':
				componentForm.config = { content: '' };
				break;
			case 'link_group':
				componentForm.config = { links: [] };
				break;
			case 'stats':
				componentForm.config = { items: [] };
				break;
			case 'entity_list':
				componentForm.config = { entity: 'contacts', pageSize: 10 };
				break;
		}
	}

	// Link group helpers
	function addLink() {
		const config = componentForm.config as LinkGroupConfig;
		config.links = [...(config.links || []), { label: '', href: '', icon: 'link' }];
		componentForm.config = config;
	}

	function removeLink(index: number) {
		const config = componentForm.config as LinkGroupConfig;
		config.links = config.links.filter((_, i) => i !== index);
		componentForm.config = config;
	}

	// Stats helpers
	function addStat() {
		const config = componentForm.config as StatsConfig;
		config.items = [...(config.items || []), { label: '', value: '', icon: 'chart', color: 'blue' }];
		componentForm.config = config;
	}

	function removeStat(index: number) {
		const config = componentForm.config as StatsConfig;
		config.items = config.items.filter((_, i) => i !== index);
		componentForm.config = config;
	}

	function getComponentTypeLabel(type: string): string {
		return COMPONENT_TYPES.find(t => t.value === type)?.label || type;
	}

	onMount(() => {
		loadPage();
	});
</script>

<svelte:head>
	<title>{customPage?.title || 'Edit Page'} - Admin - QuanticoCRM</title>
</svelte:head>

{#if loading}
	<div class="flex items-center justify-center py-12">
		<svg class="animate-spin h-8 w-8 text-primary" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
			<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
			<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
		</svg>
	</div>
{:else if customPage}
	<div class="space-y-6">
		<!-- Header -->
		<div class="flex items-center justify-between">
			<div class="flex items-center gap-4">
				<a href="/admin/pages" class="text-gray-500 hover:text-gray-700">
					<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
					</svg>
				</a>
				<h1 class="text-2xl font-bold text-gray-900">Edit Page</h1>
			</div>
			<div class="flex items-center gap-3">
				<a
					href="/p/{customPage.slug}"
					target="_blank"
					class="inline-flex items-center px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
				>
					<svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
					</svg>
					Preview
				</a>
				<button
					onclick={savePage}
					disabled={saving}
					class="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-black bg-primary hover:bg-primary/90 disabled:opacity-50"
				>
					{saving ? 'Saving...' : 'Save Changes'}
				</button>
			</div>
		</div>

		<div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
			<!-- Page Settings -->
			<div class="lg:col-span-1 space-y-6">
				<div class="bg-white shadow rounded-lg p-6">
					<h2 class="text-lg font-medium text-gray-900 mb-4">Page Settings</h2>

					<div class="space-y-4">
						<div>
							<label for="title" class="block text-sm font-medium text-gray-700">Title</label>
							<input
								type="text"
								id="title"
								bind:value={customPage.title}
								class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
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
									bind:value={customPage.slug}
									class="flex-1 block w-full rounded-none rounded-r-md border-gray-300 focus:border-primary focus:ring-primary sm:text-sm font-mono"
								/>
							</div>
						</div>

						<div>
							<label for="description" class="block text-sm font-medium text-gray-700">Description</label>
							<textarea
								id="description"
								bind:value={customPage.description}
								rows="2"
								class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
							></textarea>
						</div>

						<div class="flex items-center gap-6">
							<label class="flex items-center">
								<input type="checkbox" bind:checked={customPage.isEnabled} class="rounded border-gray-300 text-primary focus:ring-primary" />
								<span class="ml-2 text-sm text-gray-700">Enabled</span>
							</label>
							<label class="flex items-center">
								<input type="checkbox" bind:checked={customPage.isPublic} class="rounded border-gray-300 text-primary focus:ring-primary" />
								<span class="ml-2 text-sm text-gray-700">Public</span>
							</label>
						</div>
					</div>
				</div>
			</div>

			<!-- Components Editor -->
			<div class="lg:col-span-2 space-y-4">
				<div class="flex items-center justify-between">
					<h2 class="text-lg font-medium text-gray-900">Components</h2>
					<button
						onclick={openAddComponent}
						class="inline-flex items-center px-3 py-1.5 border border-transparent text-sm font-medium rounded-md text-primary bg-blue-100 hover:bg-blue-200"
					>
						<svg class="w-4 h-4 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
						</svg>
						Add Component
					</button>
				</div>

				{#if customPage.components.length === 0}
					<div class="text-center py-12 bg-gray-50 rounded-lg border-2 border-dashed border-gray-300">
						<svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 5a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM4 13a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H5a1 1 0 01-1-1v-6zM16 13a1 1 0 011-1h2a1 1 0 011 1v6a1 1 0 01-1 1h-2a1 1 0 01-1-1v-6z" />
						</svg>
						<h3 class="mt-2 text-sm font-medium text-gray-900">No components</h3>
						<p class="mt-1 text-sm text-gray-500">Add iframes, text, or other components to this page.</p>
						<button
							onclick={openAddComponent}
							class="mt-4 inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-black bg-primary hover:bg-primary/90"
						>
							Add First Component
						</button>
					</div>
				{:else}
					<div class="space-y-3">
						{#each customPage.components.sort((a, b) => a.order - b.order) as component, index (component.id)}
							<div class="bg-white shadow rounded-lg p-4 border border-gray-200">
								<div class="flex items-center justify-between">
									<div class="flex items-center gap-3">
										<div class="flex flex-col gap-1">
											<button
												onclick={() => moveComponent(index, 'up')}
												disabled={index === 0}
												class="p-1 text-gray-400 hover:text-gray-600 disabled:opacity-30"
											>
												<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
													<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 15l7-7 7 7" />
												</svg>
											</button>
											<button
												onclick={() => moveComponent(index, 'down')}
												disabled={index === customPage.components.length - 1}
												class="p-1 text-gray-400 hover:text-gray-600 disabled:opacity-30"
											>
												<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
													<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
												</svg>
											</button>
										</div>
										<div>
											<span class="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-gray-100 text-gray-700">
												{getComponentTypeLabel(component.type)}
											</span>
											<span class="ml-2 text-sm text-gray-500">({component.width})</span>
										</div>
										{#if component.title}
											<span class="text-sm font-medium text-gray-900">{component.title}</span>
										{/if}
									</div>
									<div class="flex items-center gap-2">
										<button
											onclick={() => openEditComponent(index)}
											class="text-primary hover:text-blue-800 text-sm"
										>
											Edit
										</button>
										<button
											onclick={() => deleteComponent(index)}
											class="text-red-600 hover:text-red-800 text-sm"
										>
											Delete
										</button>
									</div>
								</div>
							</div>
						{/each}
					</div>
				{/if}
			</div>
		</div>
	</div>
{/if}

<!-- Component Editor Modal -->
{#if showComponentModal}
	<div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
		<div class="bg-white rounded-lg shadow-xl w-full max-w-2xl max-h-[90vh] overflow-y-auto">
			<div class="sticky top-0 bg-white border-b border-gray-200 px-6 py-4 flex items-center justify-between">
				<h3 class="text-lg font-medium text-gray-900">
					{editingComponentIndex !== null ? 'Edit Component' : 'Add Component'}
				</h3>
				<button onclick={() => showComponentModal = false} class="text-gray-400 hover:text-gray-500">
					<svg class="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
					</svg>
				</button>
			</div>

			<div class="p-6 space-y-4">
				<!-- Component Type -->
				<div>
					<label class="block text-sm font-medium text-gray-700 mb-2">Component Type</label>
					<div class="grid grid-cols-2 sm:grid-cols-4 gap-2">
						{#each COMPONENT_TYPES as type}
							<button
								type="button"
								onclick={() => handleTypeChange(type.value)}
								class="p-3 rounded-lg border text-center transition-colors {componentForm.type === type.value ? 'border-primary bg-blue-50 text-blue-700' : 'border-gray-200 hover:border-gray-300'}"
							>
								<span class="text-sm font-medium">{type.label}</span>
							</button>
						{/each}
					</div>
				</div>

				<!-- Title & Width -->
				<div class="grid grid-cols-2 gap-4">
					<div>
						<label for="comp-title" class="block text-sm font-medium text-gray-700">Title (optional)</label>
						<input
							type="text"
							id="comp-title"
							bind:value={componentForm.title}
							class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
							placeholder="Component title"
						/>
					</div>
					<div>
						<label for="comp-width" class="block text-sm font-medium text-gray-700">Width</label>
						<select
							id="comp-width"
							bind:value={componentForm.width}
							class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
						>
							{#each COMPONENT_WIDTHS as width}
								<option value={width.value}>{width.label}</option>
							{/each}
						</select>
					</div>
				</div>

				<!-- Type-specific Config -->
				{#if componentForm.type === 'iframe'}
					{@const config = componentForm.config as IframeConfig}
					<div class="space-y-4 pt-4 border-t border-gray-200">
						<div>
							<label for="iframe-url" class="block text-sm font-medium text-gray-700">URL</label>
							<input
								type="url"
								id="iframe-url"
								bind:value={config.url}
								class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
								placeholder="https://example.com"
							/>
						</div>
						<div>
							<label for="iframe-height" class="block text-sm font-medium text-gray-700">Height (px)</label>
							<input
								type="number"
								id="iframe-height"
								bind:value={config.height}
								class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
								placeholder="400"
							/>
						</div>
					</div>

				{:else if componentForm.type === 'text' || componentForm.type === 'markdown'}
					{@const config = componentForm.config as TextConfig}
					<div class="space-y-4 pt-4 border-t border-gray-200">
						<div>
							<label for="text-content" class="block text-sm font-medium text-gray-700">
								Content {componentForm.type === 'markdown' ? '(Markdown supported)' : ''}
							</label>
							<textarea
								id="text-content"
								bind:value={config.content}
								rows="8"
								class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm font-mono"
								placeholder={componentForm.type === 'markdown' ? '# Heading\n\nSome **bold** text...' : 'Enter your text content here...'}
							></textarea>
						</div>
					</div>

				{:else if componentForm.type === 'html'}
					{@const config = componentForm.config as HTMLConfig}
					<div class="space-y-4 pt-4 border-t border-gray-200">
						<div>
							<label for="html-content" class="block text-sm font-medium text-gray-700">HTML Content</label>
							<textarea
								id="html-content"
								bind:value={config.content}
								rows="8"
								class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm font-mono"
								placeholder="<div>Your HTML here...</div>"
							></textarea>
							<p class="mt-1 text-xs text-amber-600">Scripts and styles will be stripped for security.</p>
						</div>
					</div>

				{:else if componentForm.type === 'entity_list'}
					{@const config = componentForm.config as EntityListConfig}
					<div class="space-y-4 pt-4 border-t border-gray-200">
						<div class="grid grid-cols-2 gap-4">
							<div>
								<label for="entity" class="block text-sm font-medium text-gray-700">Entity</label>
								<select
									id="entity"
									bind:value={config.entity}
									class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
								>
									<option value="contacts">Contacts</option>
									<option value="accounts">Accounts</option>
									<option value="tasks">Tasks</option>
								</select>
							</div>
							<div>
								<label for="pageSize" class="block text-sm font-medium text-gray-700">Page Size</label>
								<input
									type="number"
									id="pageSize"
									bind:value={config.pageSize}
									class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
									min="1"
									max="50"
								/>
							</div>
						</div>
					</div>

				{:else if componentForm.type === 'link_group'}
					{@const config = componentForm.config as LinkGroupConfig}
					<div class="space-y-4 pt-4 border-t border-gray-200">
						<div class="flex items-center justify-between">
							<span class="text-sm font-medium text-gray-700">Links</span>
							<button
								type="button"
								onclick={addLink}
								class="text-sm text-primary hover:text-blue-800"
							>
								+ Add Link
							</button>
						</div>
						{#each config.links || [] as link, i}
							<div class="flex items-start gap-2 p-3 bg-gray-50 rounded-lg">
								<div class="flex-1 grid grid-cols-2 gap-2">
									<input
										type="text"
										bind:value={link.label}
										placeholder="Label"
										class="block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
									/>
									<input
										type="text"
										bind:value={link.href}
										placeholder="URL or path"
										class="block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
									/>
									<input
										type="text"
										bind:value={link.description}
										placeholder="Description (optional)"
										class="col-span-2 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
									/>
								</div>
								<button
									type="button"
									onclick={() => removeLink(i)}
									class="text-red-500 hover:text-red-700 p-1"
								>
									<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
									</svg>
								</button>
							</div>
						{/each}
					</div>

				{:else if componentForm.type === 'stats'}
					{@const config = componentForm.config as StatsConfig}
					<div class="space-y-4 pt-4 border-t border-gray-200">
						<div class="flex items-center justify-between">
							<span class="text-sm font-medium text-gray-700">Stats</span>
							<button
								type="button"
								onclick={addStat}
								class="text-sm text-primary hover:text-blue-800"
							>
								+ Add Stat
							</button>
						</div>
						{#each config.items || [] as stat, i}
							<div class="flex items-start gap-2 p-3 bg-gray-50 rounded-lg">
								<div class="flex-1 grid grid-cols-2 gap-2">
									<input
										type="text"
										bind:value={stat.label}
										placeholder="Label"
										class="block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
									/>
									<input
										type="text"
										bind:value={stat.value}
										placeholder="Value"
										class="block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
									/>
									<select
										bind:value={stat.color}
										class="block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
									>
										<option value="blue">Blue</option>
										<option value="green">Green</option>
										<option value="red">Red</option>
										<option value="yellow">Yellow</option>
										<option value="purple">Purple</option>
									</select>
									<input
										type="text"
										bind:value={stat.icon}
										placeholder="Icon (e.g., chart, users)"
										class="block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
									/>
								</div>
								<button
									type="button"
									onclick={() => removeStat(i)}
									class="text-red-500 hover:text-red-700 p-1"
								>
									<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
									</svg>
								</button>
							</div>
						{/each}
					</div>
				{/if}
			</div>

			<div class="sticky bottom-0 bg-gray-50 border-t border-gray-200 px-6 py-4 flex justify-end gap-3">
				<button
					type="button"
					onclick={() => showComponentModal = false}
					class="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-50"
				>
					Cancel
				</button>
				<button
					type="button"
					onclick={saveComponent}
					class="px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-black bg-primary hover:bg-primary/90"
				>
					{editingComponentIndex !== null ? 'Update Component' : 'Add Component'}
				</button>
			</div>
		</div>
	</div>
{/if}
