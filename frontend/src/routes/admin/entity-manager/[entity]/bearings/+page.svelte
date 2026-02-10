<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { get, post, put, del } from '$lib/utils/api';
	import { addToast } from '$lib/stores/toast.svelte';
	import type { BearingConfig, BearingConfigCreateInput } from '$lib/types/bearing';
	import type { FieldDef } from '$lib/types/admin';

	let entityName = $derived($page.params.entity);

	let bearings = $state<BearingConfig[]>([]);
	let picklistFields = $state<FieldDef[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let saving = $state(false);

	// New bearing form
	let showCreateForm = $state(false);
	let newBearing = $state<BearingConfigCreateInput>({
		name: '',
		sourcePicklist: '',
		active: true,
		confirmBackward: false,
		allowUpdates: true
	});

	// Edit mode
	let editingId = $state<string | null>(null);
	let editForm = $state<Partial<BearingConfig>>({});

	async function loadData() {
		try {
			loading = true;
			error = null;

			const [bearingsData, fieldsData] = await Promise.all([
				get<BearingConfig[]>(`/entities/${entityName}/bearing-configs`),
				get<FieldDef[]>(`/entities/${entityName}/picklist-fields`)
			]);

			bearings = bearingsData;
			picklistFields = fieldsData;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load data';
			addToast(error, 'error');
		} finally {
			loading = false;
		}
	}

	async function createBearing() {
		if (!newBearing.name || !newBearing.sourcePicklist) {
			addToast('Name and Source Picklist are required', 'error');
			return;
		}

		saving = true;
		try {
			const created = await post<BearingConfig>(`/entities/${entityName}/bearing-configs`, newBearing);
			bearings = [...bearings, created];
			addToast('Bearing created', 'success');

			// Reset form
			newBearing = { name: '', sourcePicklist: '', active: true, confirmBackward: false, allowUpdates: true };
			showCreateForm = false;
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to create bearing';
			addToast(message, 'error');
		} finally {
			saving = false;
		}
	}

	function startEdit(bearing: BearingConfig) {
		editingId = bearing.id;
		editForm = { ...bearing };
	}

	function cancelEdit() {
		editingId = null;
		editForm = {};
	}

	async function saveBearing() {
		if (!editingId) return;

		saving = true;
		try {
			const updated = await put<BearingConfig>(`/entities/${entityName}/bearing-configs/${editingId}`, editForm);
			bearings = bearings.map(b => b.id === editingId ? updated : b);
			addToast('Bearing updated', 'success');
			cancelEdit();
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to update bearing';
			addToast(message, 'error');
		} finally {
			saving = false;
		}
	}

	async function deleteBearing(id: string) {
		if (!confirm('Are you sure you want to delete this bearing?')) return;

		try {
			await del(`/entities/${entityName}/bearing-configs/${id}`);
			bearings = bearings.filter(b => b.id !== id);
			addToast('Bearing deleted', 'success');
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to delete bearing';
			addToast(message, 'error');
		}
	}

	async function toggleActive(bearing: BearingConfig) {
		try {
			const updated = await put<BearingConfig>(`/entities/${entityName}/bearing-configs/${bearing.id}`, {
				active: !bearing.active
			});
			bearings = bearings.map(b => b.id === bearing.id ? updated : b);
			addToast(`Bearing ${updated.active ? 'activated' : 'deactivated'}`, 'success');
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to toggle bearing';
			addToast(message, 'error');
		}
	}

	function getFieldLabel(fieldName: string): string {
		const field = picklistFields.find(f => f.name === fieldName);
		return field?.label || fieldName;
	}

	onMount(() => {
		loadData();
	});
</script>

<div class="space-y-6">
	<!-- Breadcrumb -->
	<nav class="text-sm text-gray-500 mb-2">
		<a href="/admin" class="hover:text-gray-700">Administration</a>
		<span class="mx-2">/</span>
		<a href="/admin/entity-manager" class="hover:text-gray-700">Entity Manager</a>
		<span class="mx-2">/</span>
		<a href="/admin/entity-manager/{entityName}" class="hover:text-gray-700">{entityName}</a>
		<span class="mx-2">/</span>
		<span class="text-gray-900">Bearings</span>
	</nav>

	<!-- Header -->
	<div class="flex justify-between items-center">
		<div>
			<h1 class="text-2xl font-bold text-gray-900">Bearings</h1>
			<p class="text-sm text-gray-500 mt-1">
				Visual stage progress indicators for {entityName} records
			</p>
		</div>
		<button
			onclick={() => showCreateForm = true}
			disabled={showCreateForm || picklistFields.length === 0}
			class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-600/90 disabled:opacity-50 disabled:cursor-not-allowed"
		>
			+ New Bearing
		</button>
	</div>

	{#if loading}
		<div class="text-center py-12 text-gray-500">Loading...</div>
	{:else if error}
		<div class="text-center py-12 text-red-500">{error}</div>
	{:else}
		<!-- No picklist fields warning -->
		{#if picklistFields.length === 0}
			<div class="bg-yellow-50 border border-yellow-200 rounded-lg p-4">
				<div class="flex">
					<svg class="h-5 w-5 text-yellow-400" viewBox="0 0 20 20" fill="currentColor">
						<path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd" />
					</svg>
					<div class="ml-3">
						<h3 class="text-sm font-medium text-yellow-800">No Picklist Fields Available</h3>
						<p class="mt-1 text-sm text-yellow-700">
							Bearings require a Dropdown (enum) field to drive the stages.
							<a href="/admin/entity-manager/{entityName}/fields" class="underline">Add a dropdown field</a> first.
						</p>
					</div>
				</div>
			</div>
		{/if}

		<!-- Create Form -->
		{#if showCreateForm}
			<div class="bg-white shadow rounded-lg p-6">
				<h2 class="text-lg font-medium text-gray-900 mb-4">Create New Bearing</h2>

				<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
					<div>
						<label class="block text-sm font-medium text-gray-700 mb-1">
							Bearing Name <span class="text-red-500">*</span>
						</label>
						<input
							type="text"
							bind:value={newBearing.name}
							placeholder="e.g., Sales Stage, Onboarding Status"
							class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
						/>
					</div>

					<div>
						<label class="block text-sm font-medium text-gray-700 mb-1">
							Source Picklist <span class="text-red-500">*</span>
						</label>
						<select
							bind:value={newBearing.sourcePicklist}
							class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
						>
							<option value="">Select a picklist field...</option>
							{#each picklistFields as field (field.id)}
								<option value={field.name}>{field.label}</option>
							{/each}
						</select>
					</div>

					<div class="flex items-center gap-6">
						<label class="flex items-center gap-2">
							<input
								type="checkbox"
								bind:checked={newBearing.active}
								class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
							/>
							<span class="text-sm text-gray-700">Active</span>
						</label>

						<label class="flex items-center gap-2">
							<input
								type="checkbox"
								bind:checked={newBearing.allowUpdates}
								class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
							/>
							<span class="text-sm text-gray-700">Allow Updates</span>
						</label>

						<label class="flex items-center gap-2">
							<input
								type="checkbox"
								bind:checked={newBearing.confirmBackward}
								class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
							/>
							<span class="text-sm text-gray-700">Confirm Backward Movement</span>
						</label>
					</div>
				</div>

				<div class="flex justify-end gap-3 mt-6">
					<button
						onclick={() => showCreateForm = false}
						class="px-4 py-2 border border-gray-300 rounded-md text-gray-700 hover:bg-gray-50"
					>
						Cancel
					</button>
					<button
						onclick={createBearing}
						disabled={saving || !newBearing.name || !newBearing.sourcePicklist}
						class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-600/90 disabled:opacity-50 disabled:cursor-not-allowed"
					>
						{saving ? 'Creating...' : 'Create Bearing'}
					</button>
				</div>
			</div>
		{/if}

		<!-- Bearings List -->
		{#if bearings.length === 0 && !showCreateForm}
			<div class="bg-white shadow rounded-lg p-12 text-center">
				<svg class="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
				</svg>
				<h3 class="mt-4 text-lg font-medium text-gray-900">No Bearings Configured</h3>
				<p class="mt-2 text-sm text-gray-500">
					Create a bearing to show a visual stage progress indicator on record detail pages.
				</p>
				{#if picklistFields.length > 0}
					<button
						onclick={() => showCreateForm = true}
						class="mt-4 px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-600/90"
					>
						Create Your First Bearing
					</button>
				{/if}
			</div>
		{:else if bearings.length > 0}
			<div class="bg-white shadow rounded-lg overflow-hidden">
				<table class="min-w-full divide-y divide-gray-200">
					<thead class="bg-gray-50">
						<tr>
							<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Name</th>
							<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Source Field</th>
							<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Order</th>
							<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Status</th>
							<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Options</th>
							<th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">Actions</th>
						</tr>
					</thead>
					<tbody class="bg-white divide-y divide-gray-200">
						{#each bearings.toSorted((a, b) => a.displayOrder - b.displayOrder) as bearing (bearing.id)}
							{#if editingId === bearing.id}
								<!-- Edit Row -->
								<tr class="bg-blue-50">
									<td class="px-6 py-4">
										<input
											type="text"
											bind:value={editForm.name}
											class="w-full px-2 py-1 border border-gray-300 rounded text-sm"
										/>
									</td>
									<td class="px-6 py-4">
										<select
											bind:value={editForm.sourcePicklist}
											class="w-full px-2 py-1 border border-gray-300 rounded text-sm"
										>
											{#each picklistFields as field (field.id)}
												<option value={field.name}>{field.label}</option>
											{/each}
										</select>
									</td>
									<td class="px-6 py-4">
										<input
											type="number"
											bind:value={editForm.displayOrder}
											min="1"
											max="12"
											class="w-16 px-2 py-1 border border-gray-300 rounded text-sm"
										/>
									</td>
									<td class="px-6 py-4">
										<label class="flex items-center gap-2">
											<input
												type="checkbox"
												bind:checked={editForm.active}
												class="rounded border-gray-300 text-blue-600"
											/>
											<span class="text-sm">Active</span>
										</label>
									</td>
									<td class="px-6 py-4 space-y-1">
										<label class="flex items-center gap-2">
											<input
												type="checkbox"
												bind:checked={editForm.allowUpdates}
												class="rounded border-gray-300 text-blue-600"
											/>
											<span class="text-sm text-gray-600">Allow updates</span>
										</label>
										<label class="flex items-center gap-2">
											<input
												type="checkbox"
												bind:checked={editForm.confirmBackward}
												class="rounded border-gray-300 text-blue-600"
											/>
											<span class="text-sm text-gray-600">Confirm backward</span>
										</label>
									</td>
									<td class="px-6 py-4 text-right">
										<button
											onclick={saveBearing}
											disabled={saving}
											class="text-sm text-green-600 hover:text-green-800 mr-3"
										>
											Save
										</button>
										<button
											onclick={cancelEdit}
											class="text-sm text-gray-600 hover:text-gray-800"
										>
											Cancel
										</button>
									</td>
								</tr>
							{:else}
								<!-- Display Row -->
								<tr class="hover:bg-gray-50">
									<td class="px-6 py-4 text-sm font-medium text-gray-900">{bearing.name}</td>
									<td class="px-6 py-4 text-sm text-gray-500">{getFieldLabel(bearing.sourcePicklist)}</td>
									<td class="px-6 py-4 text-sm text-gray-500">{bearing.displayOrder}</td>
									<td class="px-6 py-4">
										<button
											onclick={() => toggleActive(bearing)}
											class="px-2 py-1 text-xs rounded-full {bearing.active ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-600'}"
										>
											{bearing.active ? 'Active' : 'Inactive'}
										</button>
									</td>
									<td class="px-6 py-4 text-sm text-gray-500">
										{#if bearing.allowUpdates || bearing.confirmBackward}
											<div class="space-y-0.5">
												{#if bearing.allowUpdates}
													<div>Allow updates</div>
												{/if}
												{#if bearing.confirmBackward}
													<div>Confirm backward</div>
												{/if}
											</div>
										{:else}
											-
										{/if}
									</td>
									<td class="px-6 py-4 text-right text-sm">
										<button
											onclick={() => startEdit(bearing)}
											class="text-blue-600 hover:text-blue-800 mr-3"
										>
											Edit
										</button>
										<button
											onclick={() => deleteBearing(bearing.id)}
											class="text-red-600 hover:text-red-800"
										>
											Delete
										</button>
									</td>
								</tr>
							{/if}
						{/each}
					</tbody>
				</table>
			</div>

			<p class="text-sm text-gray-500">
				Maximum of 12 bearings per entity. Currently using {bearings.length}/12.
			</p>
		{/if}
	{/if}
</div>
