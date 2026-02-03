<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { get, post, del } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import { TableSkeleton, ErrorDisplay } from '$lib/components/ui';
	import type { PdfTemplate } from '$lib/types/pdf-template';
	import { BASE_DESIGNS } from '$lib/types/pdf-template';

	let templates = $state<PdfTemplate[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let showCreateModal = $state(false);
	let creating = $state(false);

	let newName = $state('');
	let newBaseDesign = $state('professional');
	let newEntityType = $state('Quote');

	async function loadTemplates() {
		try {
			loading = true;
			error = null;
			templates = await get<PdfTemplate[]>('/pdf-templates?entityType=Quote');
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load templates';
		} finally {
			loading = false;
		}
	}

	async function createTemplate() {
		if (!newName.trim()) {
			toast.error('Name is required');
			return;
		}
		creating = true;
		try {
			const tpl = await post<PdfTemplate>('/pdf-templates', {
				name: newName.trim(),
				entityType: newEntityType,
				baseDesign: newBaseDesign
			});
			toast.success('Template created');
			showCreateModal = false;
			newName = '';
			newBaseDesign = 'professional';
			goto(`/admin/pdf-templates/${tpl.id}`);
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to create template');
		} finally {
			creating = false;
		}
	}

	async function setDefault(id: string) {
		try {
			await post<void>(`/pdf-templates/${id}/set-default`, {});
			templates = templates.map(t => ({ ...t, isDefault: t.id === id }));
			toast.success('Default template updated');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to set default');
		}
	}

	async function deleteTemplate(id: string) {
		if (!confirm('Are you sure you want to delete this template?')) return;
		const backup = [...templates];
		templates = templates.filter(t => t.id !== id);
		try {
			await del(`/pdf-templates/${id}`);
			toast.success('Template deleted');
		} catch (e) {
			templates = backup;
			toast.error(e instanceof Error ? e.message : 'Failed to delete template');
		}
	}

	function getDesignLabel(design: string): string {
		return BASE_DESIGNS.find(d => d.value === design)?.label ?? design;
	}

	onMount(() => loadTemplates());
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold text-gray-900">PDF Templates</h1>
			<p class="mt-1 text-sm text-gray-500">Manage PDF templates for quotes and other documents</p>
		</div>
		<div class="flex items-center gap-3">
			<a href="/admin" class="text-sm text-gray-600 hover:text-gray-900">&larr; Back to Admin</a>
			<button
				onclick={() => showCreateModal = true}
				class="inline-flex items-center px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-600/90"
			>
				<svg class="w-4 h-4 mr-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
				</svg>
				New Template
			</button>
		</div>
	</div>

	{#if loading}
		<TableSkeleton />
	{:else if error}
		<ErrorDisplay message={error} onRetry={loadTemplates} />
	{:else if templates.length === 0}
		<div class="text-center py-12 bg-white rounded-lg shadow">
			<svg class="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
			</svg>
			<h3 class="mt-2 text-sm font-medium text-gray-900">No templates</h3>
			<p class="mt-1 text-sm text-gray-500">Create a template to customize your PDF output.</p>
			<div class="mt-4">
				<button onclick={() => showCreateModal = true}
					class="inline-flex items-center px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-600/90">
					New Template
				</button>
			</div>
		</div>
	{:else}
		<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
			{#each templates as tpl (tpl.id)}
				<div class="bg-white shadow rounded-lg overflow-hidden hover:shadow-md transition-shadow">
					<div class="p-6">
						<div class="flex items-start justify-between">
							<div class="flex-1 min-w-0">
								<div class="flex items-center gap-2">
									<h3 class="text-lg font-medium text-gray-900 truncate">{tpl.name}</h3>
									{#if tpl.isDefault}
										<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-100 text-blue-800">Default</span>
									{/if}
									{#if tpl.isSystem}
										<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-600">System</span>
									{/if}
								</div>
								<p class="mt-1 text-sm text-gray-500">Base: {getDesignLabel(tpl.baseDesign)}</p>
								<p class="text-xs text-gray-400 mt-1">{tpl.pageSize} &middot; {tpl.orientation}</p>
							</div>
						</div>

						{#if tpl.branding}
							<div class="mt-4 flex items-center gap-2">
								{#if tpl.branding.primaryColor}
									<div class="w-6 h-6 rounded border border-gray-200" style="background-color: {tpl.branding.primaryColor}"></div>
								{/if}
								{#if tpl.branding.accentColor}
									<div class="w-6 h-6 rounded border border-gray-200" style="background-color: {tpl.branding.accentColor}"></div>
								{/if}
								{#if tpl.branding.companyName}
									<span class="text-xs text-gray-500">{tpl.branding.companyName}</span>
								{/if}
							</div>
						{/if}

						<div class="mt-4 text-xs text-gray-400">
							{tpl.sections?.filter(s => s.enabled).length ?? 0} / {tpl.sections?.length ?? 0} sections enabled
						</div>
					</div>

					<div class="px-6 py-3 bg-gray-50 border-t border-gray-100 flex items-center justify-between">
						<div class="flex items-center gap-2">
							<a href="/admin/pdf-templates/{tpl.id}"
								class="text-sm text-blue-600 hover:text-blue-800 font-medium">
								Edit
							</a>
							{#if !tpl.isDefault}
								<button onclick={() => setDefault(tpl.id)}
									class="text-sm text-gray-500 hover:text-gray-700">
									Set Default
								</button>
							{/if}
						</div>
						{#if !tpl.isSystem}
							<button onclick={() => deleteTemplate(tpl.id)}
								class="text-sm text-red-500 hover:text-red-700">
								Delete
							</button>
						{/if}
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>

<!-- Create Modal -->
{#if showCreateModal}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div class="fixed inset-0 bg-gray-500/75 flex items-center justify-center z-50"
		onclick={(e) => { if (e.target === e.currentTarget) showCreateModal = false; }}>
		<div class="bg-white rounded-lg shadow-xl w-full max-w-md mx-4">
			<div class="px-6 py-4 border-b border-gray-200">
				<h2 class="text-lg font-medium text-gray-900">New PDF Template</h2>
			</div>
			<form onsubmit={(e) => { e.preventDefault(); createTemplate(); }} class="p-6 space-y-4">
				<div>
					<label for="tplName" class="block text-sm font-medium text-gray-700 mb-1">
						Name <span class="text-red-500">*</span>
					</label>
					<input id="tplName" type="text" bind:value={newName} required
						placeholder="e.g. Corporate Quote"
						class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500" />
				</div>

				<div>
					<label class="block text-sm font-medium text-gray-700 mb-2">Base Design</label>
					<div class="space-y-2">
						{#each BASE_DESIGNS as design}
							<label class="flex items-start gap-3 p-3 border rounded-lg cursor-pointer hover:bg-gray-50 {newBaseDesign === design.value ? 'border-blue-500 bg-blue-50' : 'border-gray-200'}">
								<input type="radio" name="baseDesign" value={design.value} bind:group={newBaseDesign}
									class="mt-0.5" />
								<div>
									<div class="text-sm font-medium text-gray-900">{design.label}</div>
									<div class="text-xs text-gray-500">{design.description}</div>
								</div>
							</label>
						{/each}
					</div>
				</div>

				<div class="flex justify-end gap-3 pt-2">
					<button type="button" onclick={() => showCreateModal = false}
						class="px-4 py-2 text-gray-700 bg-gray-100 rounded-md hover:bg-gray-200">
						Cancel
					</button>
					<button type="submit" disabled={creating}
						class="px-4 py-2 text-white bg-blue-600 rounded-md hover:bg-blue-600/90 disabled:opacity-50">
						{creating ? 'Creating...' : 'Create Template'}
					</button>
				</div>
			</form>
		</div>
	</div>
{/if}
