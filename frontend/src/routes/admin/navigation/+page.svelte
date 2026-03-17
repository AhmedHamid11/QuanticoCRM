<script lang="ts">
	import { onMount } from 'svelte';
	import { get, post, put, del } from '$lib/utils/api';
	import { addToast } from '$lib/stores/toast.svelte';
	import { loadNavigation } from '$lib/stores/navigation.svelte';

	interface NavigationTab {
		id: string;
		label: string;
		href: string;
		icon: string;
		entityName?: string;
		sortOrder: number;
		isVisible: boolean;
		isSystem: boolean;
	}

	interface Entity {
		name: string;
		label: string;
		labelPlural: string;
	}

	let tabs = $state<NavigationTab[]>([]);
	let entities = $state<Entity[]>([]);
	let loading = $state(true);
	let saving = $state(false);
	let draggedIndex = $state<number | null>(null);

	// Modal state
	let showModal = $state(false);
	let editingTab = $state<NavigationTab | null>(null);
	let formData = $state({ label: '', href: '', icon: '', isVisible: true, entityName: '' });

	onMount(async () => {
		await Promise.all([loadTabs(), loadEntities()]);
	});

	async function loadEntities() {
		try {
			entities = await get<Entity[]>('/admin/entities');
		} catch (e) {
			console.error('Failed to load entities:', e);
		}
	}

	function handleEntitySelect(entityName: string) {
		if (!entityName) {
			// Custom URL selected
			formData.entityName = '';
			formData.href = '/';
			formData.label = '';
			return;
		}

		const entity = entities.find(e => e.name === entityName);
		if (entity) {
			formData.entityName = entity.name;
			formData.href = '/' + entity.labelPlural.toLowerCase().replace(/\s+/g, '-');
			// Only set label if it's empty (don't override user's custom label)
			if (!formData.label) {
				formData.label = entity.labelPlural;
			}
		}
	}

	async function loadTabs() {
		loading = true;
		try {
			tabs = await get<NavigationTab[]>('/admin/navigation');
		} catch (e) {
			addToast('Failed to load navigation tabs', 'error');
		} finally {
			loading = false;
		}
	}

	function openAddModal() {
		editingTab = null;
		formData = { label: '', href: '/', icon: '', isVisible: true, entityName: '' };
		showModal = true;
	}

	function openEditModal(tab: NavigationTab) {
		editingTab = tab;
		formData = { label: tab.label, href: tab.href, icon: tab.icon, isVisible: tab.isVisible, entityName: tab.entityName || '' };
		showModal = true;
	}

	function closeModal() {
		showModal = false;
		editingTab = null;
	}

	async function saveTab() {
		if (!formData.label || !formData.href) {
			addToast('Label and URL are required', 'error');
			return;
		}

		saving = true;
		try {
			if (editingTab) {
				await put(`/admin/navigation/${editingTab.id}`, formData);
				addToast('Tab updated successfully', 'success');
			} else {
				await post('/admin/navigation', formData);
				addToast('Tab created successfully', 'success');
			}
			closeModal();
			await loadTabs();
			await loadNavigation(); // Refresh the navbar
		} catch (e) {
			addToast('Failed to save tab', 'error');
		} finally {
			saving = false;
		}
	}

	async function deleteTab(tab: NavigationTab) {
		if (!confirm(`Delete "${tab.label}" tab?`)) return;

		try {
			await del(`/admin/navigation/${tab.id}`);
			addToast('Tab deleted successfully', 'success');
			await loadTabs();
			await loadNavigation();
		} catch (e) {
			addToast('Failed to delete tab', 'error');
		}
	}

	async function toggleVisibility(tab: NavigationTab) {
		try {
			await put(`/admin/navigation/${tab.id}`, { isVisible: !tab.isVisible });
			await loadTabs();
			await loadNavigation();
		} catch (e) {
			addToast('Failed to update visibility', 'error');
		}
	}

	// Drag and drop handlers
	function handleDragStart(index: number) {
		draggedIndex = index;
	}

	function handleDragOver(e: DragEvent, index: number) {
		e.preventDefault();
		if (draggedIndex === null || draggedIndex === index) return;

		const newTabs = [...tabs];
		const [draggedTab] = newTabs.splice(draggedIndex, 1);
		newTabs.splice(index, 0, draggedTab);
		tabs = newTabs;
		draggedIndex = index;
	}

	async function handleDragEnd() {
		if (draggedIndex === null) return;

		try {
			const tabIds = tabs.map((t) => t.id);
			await post('/admin/navigation/reorder', { tabIds });
			await loadNavigation();
			addToast('Order saved', 'success');
		} catch (e) {
			addToast('Failed to save order', 'error');
			await loadTabs(); // Reload original order
		}
		draggedIndex = null;
	}
</script>

<div class="space-y-6">
	<div class="flex justify-between items-center">
		<div>
			<h1 class="text-2xl font-bold text-gray-900">Navigation</h1>
			<p class="mt-1 text-sm text-gray-500">Configure the tabs that appear in the toolbar</p>
		</div>
		<button
			onclick={openAddModal}
			class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-600/90 transition-colors"
		>
			Add Tab
		</button>
	</div>

	{#if loading}
		<div class="text-center py-12">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500 mx-auto"></div>
			<p class="mt-2 text-gray-500">Loading...</p>
		</div>
	{:else}
		<div class="crm-card overflow-hidden">
			<div class="px-4 py-3 bg-gray-50 border-b text-sm text-gray-500">
				Drag and drop to reorder tabs
			</div>
			<ul class="divide-y divide-gray-200">
				{#each tabs as tab, index (tab.id)}
					<li
						draggable="true"
						ondragstart={() => handleDragStart(index)}
						ondragover={(e) => handleDragOver(e, index)}
						ondragend={handleDragEnd}
						class="px-4 py-4 flex items-center justify-between hover:bg-gray-50 cursor-move
							{draggedIndex === index ? 'opacity-50 bg-blue-50' : ''}"
					>
						<div class="flex items-center space-x-4">
							<div class="text-gray-400">
								<svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
									<path d="M7 2a2 2 0 1 0 .001 4.001A2 2 0 0 0 7 2zm0 6a2 2 0 1 0 .001 4.001A2 2 0 0 0 7 8zm0 6a2 2 0 1 0 .001 4.001A2 2 0 0 0 7 14zm6-8a2 2 0 1 0-.001-4.001A2 2 0 0 0 13 6zm0 2a2 2 0 1 0 .001 4.001A2 2 0 0 0 13 8zm0 6a2 2 0 1 0 .001 4.001A2 2 0 0 0 13 14z" />
								</svg>
							</div>
							<div>
								<div class="flex items-center space-x-2">
									<span class="font-medium text-gray-900">{tab.label}</span>
									{#if tab.isSystem}
										<span class="px-2 py-0.5 text-xs bg-gray-100 text-gray-600 rounded">System</span>
									{/if}
									{#if !tab.isVisible}
										<span class="px-2 py-0.5 text-xs bg-yellow-100 text-yellow-700 rounded">Hidden</span>
									{/if}
								</div>
								<div class="text-sm text-gray-500">{tab.href}</div>
							</div>
						</div>
						<div class="flex items-center space-x-2">
							<button
								onclick={() => toggleVisibility(tab)}
								class="p-2 text-gray-400 hover:text-gray-600 rounded"
								title={tab.isVisible ? 'Hide' : 'Show'}
							>
								{#if tab.isVisible}
									<svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
									</svg>
								{:else}
									<svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21" />
									</svg>
								{/if}
							</button>
							<button
								onclick={() => openEditModal(tab)}
								class="p-2 text-gray-400 hover:text-blue-600 rounded"
								title="Edit"
							>
								<svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
								</svg>
							</button>
							<button
								onclick={() => deleteTab(tab)}
								class="p-2 text-gray-400 hover:text-red-600 rounded"
								title="Delete"
							>
								<svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
								</svg>
							</button>
						</div>
					</li>
				{/each}
			</ul>
		</div>
	{/if}
</div>

<!-- Modal -->
{#if showModal}
	<div class="fixed inset-0 z-50 overflow-y-auto">
		<div class="flex min-h-screen items-center justify-center p-4">
			<div class="fixed inset-0 bg-black bg-opacity-30" onclick={closeModal}></div>
			<div class="relative bg-white rounded-lg shadow-xl max-w-md w-full p-6">
				<h2 class="text-lg font-medium text-gray-900 mb-4">
					{editingTab ? 'Edit Tab' : 'Add Tab'}
				</h2>
				<form onsubmit={(e) => { e.preventDefault(); saveTab(); }} class="space-y-4">
					<div>
						<label class="block text-sm font-medium text-gray-700 mb-1">Entity</label>
						<select
							value={formData.entityName}
							onchange={(e) => handleEntitySelect(e.currentTarget.value)}
							disabled={editingTab?.isSystem}
							class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-gray-100"
						>
							<option value="">-- Custom URL --</option>
							{#each entities as entity}
								<option value={entity.name}>{entity.labelPlural} ({entity.name})</option>
							{/each}
						</select>
						<p class="mt-1 text-xs text-gray-500">Select an entity or choose "Custom URL" to enter manually</p>
					</div>
					<div>
						<label class="block text-sm font-medium text-gray-700 mb-1">Label</label>
						<input
							type="text"
							bind:value={formData.label}
							class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
							placeholder="e.g., Contacts"
						/>
					</div>
					<div>
						<label class="block text-sm font-medium text-gray-700 mb-1">URL</label>
						<input
							type="text"
							bind:value={formData.href}
							disabled={editingTab?.isSystem || !!formData.entityName}
							class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-gray-100"
							placeholder="e.g., /contacts"
						/>
						{#if editingTab?.isSystem}
							<p class="mt-1 text-xs text-gray-500">System tab URLs cannot be changed</p>
						{:else if formData.entityName}
							<p class="mt-1 text-xs text-gray-500">URL is auto-generated from the selected entity</p>
						{/if}
					</div>
					<div>
						<label class="block text-sm font-medium text-gray-700 mb-1">Icon (optional)</label>
						<input
							type="text"
							bind:value={formData.icon}
							class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
							placeholder="e.g., users"
						/>
					</div>
					<div class="flex items-center">
						<input
							type="checkbox"
							id="isVisible"
							bind:checked={formData.isVisible}
							class="h-4 w-4 text-blue-600 border-gray-300 rounded focus:ring-blue-500"
						/>
						<label for="isVisible" class="ml-2 text-sm text-gray-700">Visible in navigation</label>
					</div>
					<div class="flex justify-end space-x-3 pt-4">
						<button
							type="button"
							onclick={closeModal}
							class="px-4 py-2 text-gray-700 bg-gray-100 rounded-md hover:bg-gray-200"
						>
							Cancel
						</button>
						<button
							type="submit"
							disabled={saving}
							class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-600/90 disabled:opacity-50"
						>
							{saving ? 'Saving...' : 'Save'}
						</button>
					</div>
				</form>
			</div>
		</div>
	</div>
{/if}
