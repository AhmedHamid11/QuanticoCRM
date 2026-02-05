<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import ImportWizard from '$lib/components/ImportWizard.svelte';
	import { api } from '$lib/utils/api';
	import { addToast } from '$lib/stores/toast.svelte';

	interface EntityDef {
		id: string;
		name: string;
		label: string;
		labelPlural: string;
		icon: string;
	}

	let selectedEntity = $state('');
	let entities: EntityDef[] = $state([]);
	let showWizard = $state(false);
	let loading = $state(true);
	let error = $state('');

	onMount(async () => {
		try {
			// Fetch available entities
			const response = await api<{ entities: EntityDef[] }>('/admin/entities');
			entities = response.entities || [];
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load entities';
			addToast('error', 'Failed to load entities');
		} finally {
			loading = false;
		}
	});

	function startImport() {
		if (selectedEntity) {
			showWizard = true;
		}
	}

	function handleComplete() {
		showWizard = false;
		// Navigate to the entity list
		const entityLower = selectedEntity.toLowerCase();
		goto(`/${entityLower}s`);
	}

	function handleCancel() {
		showWizard = false;
		selectedEntity = '';
	}
</script>

<svelte:head>
	<title>Import Data - Admin</title>
</svelte:head>

<div class="p-6">
	<div class="mb-6">
		<h1 class="text-2xl font-bold text-gray-900">Import Data</h1>
		<p class="text-gray-600 mt-1">Bulk import records from CSV files</p>
	</div>

	{#if error}
		<div class="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded mb-4">
			{error}
		</div>
	{/if}

	{#if loading}
		<div class="flex items-center justify-center py-12">
			<div class="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600"></div>
		</div>
	{:else if !showWizard}
		<div class="max-w-md bg-white shadow rounded-lg p-6">
			<label class="block mb-2 font-medium text-gray-700">Select Entity</label>
			<select
				bind:value={selectedEntity}
				class="w-full border border-gray-300 rounded-md p-2 mb-4 focus:ring-blue-500 focus:border-blue-500"
			>
				<option value="">Choose entity...</option>
				{#each entities as entity}
					<option value={entity.name}>{entity.labelPlural || entity.label}</option>
				{/each}
			</select>

			<button
				onclick={startImport}
				disabled={!selectedEntity}
				class="w-full bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
			>
				Start Import
			</button>
		</div>
	{:else}
		<ImportWizard
			entityName={selectedEntity}
			onComplete={handleComplete}
			onCancel={handleCancel}
		/>
	{/if}
</div>
