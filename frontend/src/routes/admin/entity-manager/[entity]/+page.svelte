<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { get, patch, del } from '$lib/utils/api';
	import type { EntityDef, FieldDef, EntityDefUpdateInput } from '$lib/types/admin';

	let entityName = $derived($page.params.entity);
	let entity = $state<EntityDef | null>(null);
	let fields = $state<FieldDef[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// Edit settings state
	let showSettingsModal = $state(false);
	let saving = $state(false);
	let saveError = $state<string | null>(null);
	let editLabel = $state('');
	let editLabelPlural = $state('');
	let editColor = $state('');
	let editHasActivities = $state(false);

	// Delete state
	let showDeleteModal = $state(false);
	let deleting = $state(false);
	let deleteError = $state<string | null>(null);

	// Available colors
	const colors = [
		'#3b82f6', '#6366f1', '#8b5cf6', '#ec4899', '#ef4444',
		'#f97316', '#eab308', '#22c55e', '#14b8a6', '#06b6d4', '#64748b'
	];

	async function loadData() {
		try {
			loading = true;
			const [entityData, fieldsData] = await Promise.all([
				get<EntityDef>(`/admin/entities/${entityName}`),
				get<FieldDef[]>(`/admin/entities/${entityName}/fields`)
			]);
			entity = entityData;
			fields = fieldsData;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load data';
		} finally {
			loading = false;
		}
	}

	function openSettingsModal() {
		if (!entity) return;
		editLabel = entity.label;
		editLabelPlural = entity.labelPlural;
		editColor = entity.color;
		editHasActivities = entity.hasActivities;
		saveError = null;
		showSettingsModal = true;
	}

	function closeSettingsModal() {
		showSettingsModal = false;
	}

	async function saveSettings() {
		if (!entity) return;

		saving = true;
		saveError = null;

		// Optimistic update
		const backup = { ...entity };
		entity = {
			...entity,
			label: editLabel,
			labelPlural: editLabelPlural,
			color: editColor,
			hasActivities: editHasActivities
		};
		closeSettingsModal();

		try {
			const input: EntityDefUpdateInput = {
				label: editLabel,
				labelPlural: editLabelPlural,
				color: editColor,
				hasActivities: editHasActivities
			};
			const updated = await patch<EntityDef>(`/admin/entities/${entityName}`, input);
			entity = updated;
		} catch (e) {
			entity = backup;
			saveError = e instanceof Error ? e.message : 'Failed to save settings';
			showSettingsModal = true;
		} finally {
			saving = false;
		}
	}

	function openDeleteModal() {
		deleteError = null;
		showDeleteModal = true;
	}

	function closeDeleteModal() {
		showDeleteModal = false;
	}

	async function confirmDelete() {
		if (!entity) return;

		deleting = true;
		deleteError = null;

		try {
			await del(`/admin/entities/${entityName}`);
			goto('/admin/entity-manager');
		} catch (e) {
			deleteError = e instanceof Error ? e.message : 'Failed to delete entity';
		} finally {
			deleting = false;
		}
	}

	onMount(() => {
		loadData();
	});
</script>

<div class="space-y-6">
	<div>
		<nav class="text-sm text-gray-500 mb-2">
			<a href="/admin" class="hover:text-gray-700">Administration</a>
			<span class="mx-2">/</span>
			<a href="/admin/entity-manager" class="hover:text-gray-700">Entity Manager</a>
			<span class="mx-2">/</span>
			<span class="text-gray-900">{entity?.label || entityName}</span>
		</nav>
		<div class="flex items-center gap-4">
			{#if entity}
				<div
					class="w-12 h-12 rounded-lg flex items-center justify-center text-white text-xl font-bold"
					style="background-color: {entity.color}"
				>
					{entity.label.charAt(0)}
				</div>
			{/if}
			<div>
				<h1 class="text-2xl font-bold text-gray-900">{entity?.label || entityName}</h1>
				<p class="text-sm text-gray-500">{entity?.labelPlural || ''}</p>
			</div>
		</div>
	</div>

	{#if loading}
		<div class="text-center py-12 text-gray-500">Loading...</div>
	{:else if error}
		<div class="text-center py-12 text-red-500">{error}</div>
	{:else}
		<!-- Management Options -->
		<div class="grid grid-cols-1 md:grid-cols-2 gap-6">
			<a
				href="/admin/entity-manager/{entityName}/fields"
				class="bg-white shadow rounded-lg p-6 hover:shadow-md transition-shadow border-l-4 border-green-500"
			>
				<div class="flex items-start">
					<div class="flex-shrink-0">
						<svg class="h-10 w-10 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h7" />
						</svg>
					</div>
					<div class="ml-4">
						<h3 class="text-lg font-medium text-gray-900">Fields</h3>
						<p class="mt-1 text-sm text-gray-500">
							Add, edit, and remove fields. Configure field types, validation, and options.
						</p>
						<p class="mt-2 text-sm text-green-600 font-medium">
							{fields.length} fields configured
						</p>
					</div>
				</div>
			</a>

			<a
				href="/admin/entity-manager/{entityName}/layouts"
				class="bg-white shadow rounded-lg p-6 hover:shadow-md transition-shadow border-l-4 border-purple-500"
			>
				<div class="flex items-start">
					<div class="flex-shrink-0">
						<svg class="h-10 w-10 text-purple-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 5a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM4 13a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H5a1 1 0 01-1-1v-6zM16 13a1 1 0 011-1h2a1 1 0 011 1v6a1 1 0 01-1 1h-2a1 1 0 01-1-1v-6z" />
						</svg>
					</div>
					<div class="ml-4">
						<h3 class="text-lg font-medium text-gray-900">Layouts</h3>
						<p class="mt-1 text-sm text-gray-500">
							Customize list view columns, detail form layout, and filter panels.
						</p>
						<p class="mt-2 text-sm text-purple-600 font-medium">
							List, Detail, Layout Editor, Filters
						</p>
					</div>
				</div>
			</a>

			<a
				href="/admin/entity-manager/{entityName}/bearings"
				class="bg-white shadow rounded-lg p-6 hover:shadow-md transition-shadow border-l-4 border-amber-500"
			>
				<div class="flex items-start">
					<div class="flex-shrink-0">
						<svg class="h-10 w-10 text-amber-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
						</svg>
					</div>
					<div class="ml-4">
						<h3 class="text-lg font-medium text-gray-900">Bearings</h3>
						<p class="mt-1 text-sm text-gray-500">
							Configure visual stage progress indicators that appear on record detail pages.
						</p>
						<p class="mt-2 text-sm text-amber-600 font-medium">
							Stage Path Indicators
						</p>
					</div>
				</div>
			</a>

			<a
				href="/admin/entity-manager/{entityName}/validation-rules"
				class="bg-white shadow rounded-lg p-6 hover:shadow-md transition-shadow border-l-4 border-red-500"
			>
				<div class="flex items-start">
					<div class="flex-shrink-0">
						<svg class="h-10 w-10 text-red-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
						</svg>
					</div>
					<div class="ml-4">
						<h3 class="text-lg font-medium text-gray-900">Validation Rules</h3>
						<p class="mt-1 text-sm text-gray-500">
							Configure rules to validate records before save operations and enforce business logic.
						</p>
						<p class="mt-2 text-sm text-red-600 font-medium">
							Data Integrity & Business Rules
						</p>
					</div>
				</div>
			</a>
		</div>

		<!-- Entity Info -->
		<div class="bg-white shadow rounded-lg p-6">
			<div class="flex items-center justify-between mb-4">
				<h2 class="text-lg font-medium text-gray-900">Entity Settings</h2>
				<div class="flex items-center gap-2">
					<button
						onclick={openSettingsModal}
						class="px-3 py-1.5 text-sm bg-gray-100 text-gray-700 rounded-lg hover:bg-gray-200 transition-colors flex items-center gap-1"
					>
						<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
						</svg>
						Edit Settings
					</button>
					{#if entity?.isCustom}
						<button
							onclick={openDeleteModal}
							class="px-3 py-1.5 text-sm bg-red-50 text-red-700 rounded-lg hover:bg-red-100 transition-colors flex items-center gap-1"
						>
							<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
							</svg>
							Delete Entity
						</button>
					{/if}
				</div>
			</div>
			<dl class="grid grid-cols-1 md:grid-cols-2 gap-4">
				<div>
					<dt class="text-sm font-medium text-gray-500">Name</dt>
					<dd class="mt-1 text-sm text-gray-900 font-mono">{entity?.name}</dd>
				</div>
				<div>
					<dt class="text-sm font-medium text-gray-500">Label</dt>
					<dd class="mt-1 text-sm text-gray-900">{entity?.label}</dd>
				</div>
				<div>
					<dt class="text-sm font-medium text-gray-500">Plural Label</dt>
					<dd class="mt-1 text-sm text-gray-900">{entity?.labelPlural}</dd>
				</div>
				<div>
					<dt class="text-sm font-medium text-gray-500">Type</dt>
					<dd class="mt-1">
						<span
							class="px-2 py-1 text-xs rounded-full"
							class:bg-blue-100={!entity?.isCustom}
							class:text-blue-800={!entity?.isCustom}
							class:bg-green-100={entity?.isCustom}
							class:text-green-800={entity?.isCustom}
						>
							{entity?.isCustom ? 'Custom' : 'System'}
						</span>
					</dd>
				</div>
				<div>
					<dt class="text-sm font-medium text-gray-500">Activities (Tasks, Meetings, Calls)</dt>
					<dd class="mt-1">
						<span class="px-2 py-1 text-xs rounded-full" class:bg-green-100={entity?.hasActivities} class:text-green-800={entity?.hasActivities} class:bg-gray-100={!entity?.hasActivities} class:text-gray-600={!entity?.hasActivities}>
							{entity?.hasActivities ? 'Enabled' : 'Disabled'}
						</span>
					</dd>
				</div>
			</dl>
		</div>

		<!-- Fields Preview -->
		<div class="bg-white shadow rounded-lg p-6">
			<div class="flex items-center justify-between mb-4">
				<h2 class="text-lg font-medium text-gray-900">Fields Overview</h2>
				<a href="/admin/entity-manager/{entityName}/fields" class="text-sm text-blue-600 hover:text-blue-800">
					Manage Fields →
				</a>
			</div>
			<div class="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-3">
				{#each fields as field (field.id)}
					<div class="px-3 py-2 bg-gray-50 rounded text-sm">
						<div class="font-medium text-gray-900">{field.label}</div>
						<div class="text-xs text-gray-500">{field.type}{field.isRequired ? ' • Required' : ''}</div>
					</div>
				{/each}
			</div>
		</div>
	{/if}
</div>

<!-- Edit Settings Modal -->
{#if showSettingsModal}
	<div class="fixed inset-0 z-50 overflow-y-auto">
		<div class="flex min-h-screen items-center justify-center p-4">
			<!-- Backdrop -->
			<div class="fixed inset-0 bg-black bg-opacity-50" onclick={closeSettingsModal}></div>

			<!-- Modal content -->
			<div class="relative bg-white rounded-lg shadow-xl max-w-md w-full p-6">
				<div class="flex items-center justify-between mb-4">
					<h2 class="text-xl font-semibold text-gray-900">Edit Entity Settings</h2>
					<button onclick={closeSettingsModal} class="text-gray-400 hover:text-gray-600">
						<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
						</svg>
					</button>
				</div>

				{#if saveError}
					<div class="mb-4 p-3 bg-red-50 border border-red-200 rounded text-red-700 text-sm">
						{saveError}
					</div>
				{/if}

				<form onsubmit={(e) => { e.preventDefault(); saveSettings(); }}>
					<div class="space-y-4">
						<!-- Label -->
						<div>
							<label for="editLabel" class="block text-sm font-medium text-gray-700 mb-1">Label</label>
							<input
								type="text"
								id="editLabel"
								bind:value={editLabel}
								class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
							/>
						</div>

						<!-- Plural Label -->
						<div>
							<label for="editLabelPlural" class="block text-sm font-medium text-gray-700 mb-1">Plural Label</label>
							<input
								type="text"
								id="editLabelPlural"
								bind:value={editLabelPlural}
								class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
							/>
						</div>

						<!-- Color picker -->
						<div>
							<label class="block text-sm font-medium text-gray-700 mb-2">Color</label>
							<div class="flex flex-wrap gap-2">
								{#each colors as color}
									<button
										type="button"
										onclick={() => editColor = color}
										class="w-8 h-8 rounded-full border-2 transition-transform hover:scale-110"
										class:border-gray-900={editColor === color}
										class:border-transparent={editColor !== color}
										style="background-color: {color}"
									></button>
								{/each}
							</div>
						</div>

						<!-- Options -->
						<div class="space-y-2">
							<label class="flex items-center gap-2 cursor-pointer">
								<input
									type="checkbox"
									bind:checked={editHasActivities}
									class="w-4 h-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
								/>
								<span class="text-sm text-gray-700">Enable Activities (Tasks, Meetings, Calls)</span>
							</label>
						</div>
					</div>

					<div class="mt-6 flex justify-end gap-3">
						<button
							type="button"
							onclick={closeSettingsModal}
							class="px-4 py-2 text-gray-700 bg-gray-100 rounded-lg hover:bg-gray-200 transition-colors"
						>
							Cancel
						</button>
						<button
							type="submit"
							disabled={saving}
							class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-600/90 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
						>
							{saving ? 'Saving...' : 'Save Changes'}
						</button>
					</div>
				</form>
			</div>
		</div>
	</div>
{/if}

<!-- Delete Confirmation Modal -->
{#if showDeleteModal}
	<div class="fixed inset-0 z-50 overflow-y-auto">
		<div class="flex min-h-screen items-center justify-center p-4">
			<div class="fixed inset-0 bg-black bg-opacity-50" onclick={closeDeleteModal}></div>
			<div class="relative bg-white rounded-lg shadow-xl max-w-md w-full p-6">
				<div class="flex items-center justify-between mb-4">
					<h2 class="text-xl font-semibold text-gray-900">Delete Entity</h2>
					<button onclick={closeDeleteModal} class="text-gray-400 hover:text-gray-600">
						<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
						</svg>
					</button>
				</div>

				<div class="mb-4">
					<div class="flex items-center gap-3 p-3 bg-red-50 border border-red-200 rounded-lg">
						<svg class="w-6 h-6 text-red-600 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
						</svg>
						<div class="text-sm text-red-700">
							<p class="font-medium">Are you sure you want to delete "{entity?.label}"?</p>
							<p class="mt-1">This entity will be hidden from the system. Existing records will be preserved but no longer accessible.</p>
						</div>
					</div>
				</div>

				{#if deleteError}
					<div class="mb-4 p-3 bg-red-50 border border-red-200 rounded text-red-700 text-sm">
						{deleteError}
					</div>
				{/if}

				<div class="flex justify-end gap-3">
					<button
						type="button"
						onclick={closeDeleteModal}
						class="px-4 py-2 text-gray-700 bg-gray-100 rounded-lg hover:bg-gray-200 transition-colors"
					>
						Cancel
					</button>
					<button
						type="button"
						onclick={confirmDelete}
						disabled={deleting}
						class="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
					>
						{deleting ? 'Deleting...' : 'Delete Entity'}
					</button>
				</div>
			</div>
		</div>
	</div>
{/if}
