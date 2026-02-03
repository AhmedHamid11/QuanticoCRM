<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { get, post } from '$lib/utils/api';
	import type { EntityDef, EntityDefCreateInput } from '$lib/types/admin';

	let entities = $state<EntityDef[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// Modal state
	let showModal = $state(false);
	let saving = $state(false);
	let saveError = $state<string | null>(null);

	// Form state
	let formLabel = $state('');
	let formName = $state('');
	let formLabelPlural = $state('');
	let formColor = $state('#6366f1');
	let formHasStream = $state(false);
	let formHasActivities = $state(false);

	// Available colors for entity
	const colors = [
		'#3b82f6', // blue
		'#6366f1', // indigo
		'#8b5cf6', // violet
		'#ec4899', // pink
		'#ef4444', // red
		'#f97316', // orange
		'#eab308', // yellow
		'#22c55e', // green
		'#14b8a6', // teal
		'#06b6d4', // cyan
		'#64748b', // slate
	];

	async function loadEntities() {
		try {
			loading = true;
			entities = await get<EntityDef[]>('/admin/entities');
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load entities';
		} finally {
			loading = false;
		}
	}

	function openModal() {
		formLabel = '';
		formName = '';
		formLabelPlural = '';
		formColor = '#6366f1';
		formHasStream = false;
		formHasActivities = false;
		saveError = null;
		showModal = true;
	}

	function closeModal() {
		showModal = false;
	}

	// Auto-generate name from label
	function handleLabelChange() {
		// Convert label to PascalCase name (e.g., "My Custom Object" -> "MyCustomObject")
		formName = formLabel
			.split(/\s+/)
			.map(word => word.charAt(0).toUpperCase() + word.slice(1).toLowerCase())
			.join('');
	}

	// Auto-generate plural from label
	function handleLabelBlur() {
		if (!formLabelPlural && formLabel) {
			formLabelPlural = formLabel + 's';
		}
	}

	async function handleSubmit() {
		if (!formLabel.trim() || !formName.trim()) {
			saveError = 'Label and Name are required';
			return;
		}

		// Validate name format (alphanumeric, PascalCase)
		if (!/^[A-Z][a-zA-Z0-9]*$/.test(formName)) {
			saveError = 'Name must start with uppercase letter and contain only letters/numbers';
			return;
		}

		saving = true;
		saveError = null;

		// Optimistic update
		const tempEntity: EntityDef = {
			id: 'temp-' + Date.now(),
			name: formName,
			label: formLabel,
			labelPlural: formLabelPlural || formLabel + 's',
			icon: 'folder',
			color: formColor,
			isCustom: true,
			isCustomizable: true,
			hasStream: formHasStream,
			hasActivities: formHasActivities,
			createdAt: new Date().toISOString(),
			modifiedAt: new Date().toISOString(),
		};
		const backup = [...entities];
		entities = [...entities, tempEntity];
		closeModal();

		try {
			const input: EntityDefCreateInput = {
				name: formName,
				label: formLabel,
				labelPlural: formLabelPlural || undefined,
				color: formColor,
				hasStream: formHasStream,
				hasActivities: formHasActivities,
			};

			const created = await post<EntityDef>('/admin/entities', input);
			// Replace temp entity with real one
			entities = entities.map(e => e.id === tempEntity.id ? created : e);
		} catch (e) {
			// Rollback on failure
			entities = backup;
			saveError = e instanceof Error ? e.message : 'Failed to create entity';
			showModal = true;
		} finally {
			saving = false;
		}
	}

	onMount(() => {
		loadEntities();
	});
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<nav class="text-sm text-gray-500 mb-2">
				<a href="/admin" class="hover:text-gray-700">Administration</a>
				<span class="mx-2">/</span>
				<span class="text-gray-900">Entity Manager</span>
			</nav>
			<h1 class="text-2xl font-bold text-gray-900">Entity Manager</h1>
		</div>
		<button
			onclick={openModal}
			class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-600/90 transition-colors flex items-center gap-2"
		>
			<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
			</svg>
			New Entity
		</button>
	</div>

	{#if loading}
		<div class="text-center py-12 text-gray-500">Loading entities...</div>
	{:else if error}
		<div class="text-center py-12 text-red-500">{error}</div>
	{:else}
		<div class="bg-white shadow rounded-lg overflow-hidden">
			<table class="min-w-full divide-y divide-gray-200">
				<thead class="bg-gray-50">
					<tr>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Entity
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Type
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Customizable
						</th>
						<th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
							Actions
						</th>
					</tr>
				</thead>
				<tbody class="bg-white divide-y divide-gray-200">
					{#each entities as entity (entity.id)}
						<tr
							class="hover:bg-gray-50 cursor-pointer"
							onclick={() => goto(`/admin/entity-manager/${entity.name}`)}
						>
							<td class="px-6 py-4 whitespace-nowrap">
								<div class="flex items-center">
									<div
										class="w-8 h-8 rounded flex items-center justify-center text-white text-sm"
										style="background-color: {entity.color}"
									>
										{entity.label.charAt(0)}
									</div>
									<div class="ml-3">
										<div class="text-sm font-medium text-gray-900">{entity.label}</div>
										<div class="text-sm text-gray-500">{entity.name}</div>
									</div>
								</div>
							</td>
							<td class="px-6 py-4 whitespace-nowrap">
								<span
									class="px-2 py-1 text-xs rounded-full"
									class:bg-blue-100={!entity.isCustom}
									class:text-blue-800={!entity.isCustom}
									class:bg-green-100={entity.isCustom}
									class:text-green-800={entity.isCustom}
								>
									{entity.isCustom ? 'Custom' : 'System'}
								</span>
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{entity.isCustomizable ? 'Yes' : 'No'}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-right">
								<button
									onclick={(e) => { e.stopPropagation(); goto(`/admin/entity-manager/${entity.name}`); }}
									class="p-2 text-gray-400 hover:text-blue-600 hover:bg-blue-50 rounded-lg transition-colors"
									title="Edit {entity.label} settings"
								>
									<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
									</svg>
								</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>

<!-- New Entity Modal -->
{#if showModal}
	<div class="fixed inset-0 z-50 overflow-y-auto">
		<div class="flex min-h-screen items-center justify-center p-4">
			<!-- Backdrop -->
			<div class="fixed inset-0 bg-black bg-opacity-50" onclick={closeModal}></div>

			<!-- Modal content -->
			<div class="relative bg-white rounded-lg shadow-xl max-w-md w-full p-6">
				<div class="flex items-center justify-between mb-4">
					<h2 class="text-xl font-semibold text-gray-900">New Entity</h2>
					<button
						onclick={closeModal}
						class="text-gray-400 hover:text-gray-600"
					>
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

				<form onsubmit={(e) => { e.preventDefault(); handleSubmit(); }}>
					<div class="space-y-4">
						<!-- Label -->
						<div>
							<label for="label" class="block text-sm font-medium text-gray-700 mb-1">
								Label <span class="text-red-500">*</span>
							</label>
							<input
								type="text"
								id="label"
								bind:value={formLabel}
								oninput={handleLabelChange}
								onblur={handleLabelBlur}
								placeholder="e.g., Product, Invoice, Project"
								class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
							/>
						</div>

						<!-- Name (auto-generated) -->
						<div>
							<label for="name" class="block text-sm font-medium text-gray-700 mb-1">
								Name <span class="text-red-500">*</span>
							</label>
							<input
								type="text"
								id="name"
								bind:value={formName}
								placeholder="PascalCase, e.g., MyEntity"
								class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
							/>
							<p class="mt-1 text-xs text-gray-500">API name (auto-generated from label)</p>
						</div>

						<!-- Plural Label -->
						<div>
							<label for="labelPlural" class="block text-sm font-medium text-gray-700 mb-1">
								Plural Label
							</label>
							<input
								type="text"
								id="labelPlural"
								bind:value={formLabelPlural}
								placeholder="e.g., Products, Invoices"
								class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
							/>
						</div>

						<!-- Color picker -->
						<div>
							<label class="block text-sm font-medium text-gray-700 mb-2">
								Color
							</label>
							<div class="flex flex-wrap gap-2">
								{#each colors as color}
									<button
										type="button"
										onclick={() => formColor = color}
										class="w-8 h-8 rounded-full border-2 transition-transform hover:scale-110"
										class:border-gray-900={formColor === color}
										class:border-transparent={formColor !== color}
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
									bind:checked={formHasStream}
									class="w-4 h-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
								/>
								<span class="text-sm text-gray-700">Enable Activity Stream</span>
							</label>
							<label class="flex items-center gap-2 cursor-pointer">
								<input
									type="checkbox"
									bind:checked={formHasActivities}
									class="w-4 h-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
								/>
								<span class="text-sm text-gray-700">Enable Activities (Tasks, Meetings, Calls)</span>
							</label>
						</div>
					</div>

					<div class="mt-6 flex justify-end gap-3">
						<button
							type="button"
							onclick={closeModal}
							class="px-4 py-2 text-gray-700 bg-gray-100 rounded-lg hover:bg-gray-200 transition-colors"
						>
							Cancel
						</button>
						<button
							type="submit"
							disabled={saving}
							class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-600/90 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
						>
							{saving ? 'Creating...' : 'Create Entity'}
						</button>
					</div>
				</form>
			</div>
		</div>
	</div>
{/if}
