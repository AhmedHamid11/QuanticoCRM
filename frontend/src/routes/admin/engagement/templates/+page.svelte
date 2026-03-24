<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { get, post, del } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import { isFeatureEnabled } from '$lib/stores/navigation.svelte';

	interface EmailTemplate {
		id: string;
		orgId: string;
		name: string;
		subject: string;
		bodyHtml: string;
		bodyText: string;
		hasComplianceFooter: number;
		createdBy: string;
		createdAt: string;
		updatedAt: string;
	}

	let templates = $state<EmailTemplate[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let showCreateModal = $state(false);
	let creating = $state(false);
	let deletingId = $state<string | null>(null);
	let newName = $state('');

	async function loadTemplates() {
		try {
			loading = true;
			error = null;
			templates = await get<EmailTemplate[]>('/gmail/email-templates');
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load templates';
		} finally {
			loading = false;
		}
	}

	async function createTemplate() {
		if (!newName.trim()) {
			toast.error('Template name is required');
			return;
		}
		creating = true;
		try {
			const tmpl = await post<EmailTemplate>('/gmail/email-templates', {
				name: newName.trim(),
				subject: '',
				bodyHtml: '',
				bodyText: ''
			});
			toast.success('Template created');
			showCreateModal = false;
			newName = '';
			goto(`/admin/engagement/templates/${tmpl.id}`);
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to create template');
		} finally {
			creating = false;
		}
	}

	async function deleteTemplate(id: string, name: string) {
		if (!confirm(`Delete template "${name}"? This cannot be undone.`)) return;
		deletingId = id;
		try {
			await del(`/gmail/email-templates/${id}`);
			templates = templates.filter((t) => t.id !== id);
			toast.success('Template deleted');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to delete template');
		} finally {
			deletingId = null;
		}
	}

	function formatDate(dateStr: string) {
		if (!dateStr) return '—';
		try {
			return new Date(dateStr).toLocaleDateString(undefined, {
				year: 'numeric',
				month: 'short',
				day: 'numeric'
			});
		} catch {
			return dateStr;
		}
	}

	onMount(() => {
		if (!isFeatureEnabled('cadences')) {
			goto('/admin');
			return;
		}
		loadTemplates();
	});
</script>

<svelte:head>
	<title>Email Templates</title>
</svelte:head>

<div class="p-6 max-w-5xl mx-auto">
	<div class="flex items-center justify-between mb-6">
		<div>
			<h1 class="text-2xl font-bold text-gray-900">Email Templates</h1>
			<p class="text-sm text-gray-500 mt-1">
				Create and manage reusable email templates for outreach sequences.
			</p>
		</div>
		<button
			onclick={() => (showCreateModal = true)}
			class="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 transition-colors"
		>
			New Template
		</button>
	</div>

	{#if loading}
		<div class="flex items-center justify-center py-16 text-gray-400">
			<svg class="animate-spin h-6 w-6 mr-2" fill="none" viewBox="0 0 24 24">
				<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
				<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z"></path>
			</svg>
			Loading templates...
		</div>
	{:else if error}
		<div class="rounded-md bg-red-50 p-4 text-sm text-red-700">
			{error}
			<button onclick={loadTemplates} class="ml-2 underline hover:no-underline">Retry</button>
		</div>
	{:else if templates.length === 0}
		<div class="text-center py-16 text-gray-400">
			<svg class="mx-auto h-12 w-12 mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
			</svg>
			<p class="text-lg font-medium text-gray-600">No templates yet</p>
			<p class="text-sm mt-1">Create your first email template to get started.</p>
			<button
				onclick={() => (showCreateModal = true)}
				class="mt-4 px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700"
			>
				New Template
			</button>
		</div>
	{:else}
		<div class="bg-white border border-gray-200 rounded-lg overflow-hidden">
			<table class="min-w-full divide-y divide-gray-200">
				<thead class="bg-gray-50">
					<tr>
						<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Name</th>
						<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Subject</th>
						<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Last Updated</th>
						<th class="px-4 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">Actions</th>
					</tr>
				</thead>
				<tbody class="bg-white divide-y divide-gray-100">
					{#each templates as tmpl (tmpl.id)}
						<tr class="hover:bg-gray-50 transition-colors">
							<td class="px-4 py-3 text-sm font-medium text-gray-900">{tmpl.name}</td>
							<td class="px-4 py-3 text-sm text-gray-500 max-w-xs truncate">
								{#if tmpl.subject}
									{tmpl.subject}
								{:else}
									<em class="text-gray-300">No subject</em>
								{/if}
							</td>
							<td class="px-4 py-3 text-sm text-gray-500">{formatDate(tmpl.updatedAt)}</td>
							<td class="px-4 py-3 text-right space-x-2">
								<a
									href="/admin/engagement/templates/{tmpl.id}"
									class="text-sm text-blue-600 hover:text-blue-800 font-medium"
								>
									Edit
								</a>
								<button
									onclick={() => deleteTemplate(tmpl.id, tmpl.name)}
									disabled={deletingId === tmpl.id}
									class="text-sm text-red-500 hover:text-red-700 font-medium disabled:opacity-50"
								>
									{deletingId === tmpl.id ? 'Deleting...' : 'Delete'}
								</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>

{#if showCreateModal}
	<!-- Modal backdrop -->
	<div
		class="fixed inset-0 bg-black bg-opacity-40 flex items-center justify-center z-50"
		onclick={() => (showCreateModal = false)}
		role="dialog"
		aria-modal="true"
		aria-label="Create new template"
	>
		<!-- Modal panel -->
		<div
			class="bg-white rounded-lg shadow-xl w-full max-w-md mx-4 p-6"
			onclick={(e) => e.stopPropagation()}
			role="presentation"
		>
			<h2 class="text-lg font-semibold text-gray-900 mb-4">New Email Template</h2>

			<label class="block text-sm font-medium text-gray-700 mb-1">Template Name</label>
			<input
				type="text"
				bind:value={newName}
				placeholder="e.g., Cold Outreach - Week 1"
				class="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
				onkeydown={(e) => e.key === 'Enter' && createTemplate()}
			/>

			<div class="flex justify-end gap-2 mt-4">
				<button
					onclick={() => { showCreateModal = false; newName = ''; }}
					class="px-4 py-2 text-sm text-gray-700 border border-gray-300 rounded-md hover:bg-gray-50"
				>
					Cancel
				</button>
				<button
					onclick={createTemplate}
					disabled={creating || !newName.trim()}
					class="px-4 py-2 text-sm bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					{creating ? 'Creating...' : 'Create Template'}
				</button>
			</div>
		</div>
	</div>
{/if}
