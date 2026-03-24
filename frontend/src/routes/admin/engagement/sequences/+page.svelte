<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { get, post } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import { isFeatureEnabled } from '$lib/stores/navigation.svelte';

	interface Sequence {
		id: string;
		orgId: string;
		name: string;
		description?: string;
		status: string;
		timezone: string;
		createdBy: string;
		createdAt: string;
		updatedAt: string;
		stepCount?: number;
		enrollmentCount?: number;
	}

	let sequences = $state<Sequence[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let creating = $state(false);
	let cloningId = $state<string | null>(null);

	const statusColors: Record<string, string> = {
		draft: 'bg-gray-100 text-gray-700',
		active: 'bg-green-100 text-green-700',
		paused: 'bg-yellow-100 text-yellow-700',
		archived: 'bg-red-100 text-red-700'
	};

	async function loadSequences() {
		try {
			loading = true;
			error = null;
			sequences = await get<Sequence[]>('/sequences');
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load sequences';
		} finally {
			loading = false;
		}
	}

	async function createNewSequence() {
		creating = true;
		try {
			const seq = await post<Sequence>('/sequences', {
				name: 'New Sequence',
				timezone: 'America/New_York'
			});
			toast.success('Sequence created');
			goto(`/admin/engagement/sequences/${seq.id}/builder`);
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to create sequence');
		} finally {
			creating = false;
		}
	}

	async function cloneSequence(event: MouseEvent, seqId: string) {
		event.stopPropagation();
		if (cloningId) return;
		cloningId = seqId;
		try {
			await post(`/sequences/${seqId}/clone`, {});
			toast.success('Sequence cloned successfully');
			await loadSequences();
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to clone sequence');
		} finally {
			cloningId = null;
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
		loadSequences();
	});
</script>

<svelte:head>
	<title>Sequence Library</title>
</svelte:head>

<div class="p-6 max-w-6xl mx-auto">
	<div class="flex items-center justify-between mb-6">
		<div>
			<h1 class="text-2xl font-bold text-gray-900">Sequence Library</h1>
			<p class="text-sm text-gray-500 mt-1">
				Shared outreach workflows. Clone any sequence to create your own draft copy.
			</p>
		</div>
		<button
			onclick={createNewSequence}
			disabled={creating}
			class="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
		>
			{creating ? 'Creating...' : 'New Sequence'}
		</button>
	</div>

	{#if loading}
		<div class="flex items-center justify-center py-16 text-gray-400">
			<svg class="animate-spin h-6 w-6 mr-2" fill="none" viewBox="0 0 24 24">
				<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
				<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z"></path>
			</svg>
			Loading sequences...
		</div>
	{:else if error}
		<div class="rounded-md bg-red-50 p-4 text-sm text-red-700">
			{error}
			<button onclick={loadSequences} class="ml-2 underline hover:no-underline">Retry</button>
		</div>
	{:else if sequences.length === 0}
		<div class="text-center py-16 text-gray-400">
			<svg class="mx-auto h-12 w-12 mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
			</svg>
			<p class="text-lg font-medium text-gray-600">No sequences yet</p>
			<p class="text-sm mt-1">Create your first sequence to start automating outreach.</p>
			<button
				onclick={createNewSequence}
				disabled={creating}
				class="mt-4 px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 disabled:opacity-50"
			>
				{creating ? 'Creating...' : 'New Sequence'}
			</button>
		</div>
	{:else}
		<div class="bg-white border border-gray-200 rounded-lg overflow-hidden">
			<table class="min-w-full divide-y divide-gray-200">
				<thead class="bg-gray-50">
					<tr>
						<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Name</th>
						<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Status</th>
						<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Steps</th>
						<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Enrollments</th>
						<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Created</th>
						<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Actions</th>
					</tr>
				</thead>
				<tbody class="bg-white divide-y divide-gray-100">
					{#each sequences as seq (seq.id)}
						<tr
							class="hover:bg-gray-50 transition-colors cursor-pointer"
							onclick={() => goto(`/admin/engagement/sequences/${seq.id}/builder`)}
						>
							<td class="px-4 py-3">
								<div class="text-sm font-medium text-gray-900">{seq.name}</div>
								{#if seq.description}
									<div class="text-xs text-gray-400 mt-0.5 truncate max-w-xs">{seq.description}</div>
								{/if}
							</td>
							<td class="px-4 py-3">
								<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium {statusColors[seq.status] ?? 'bg-gray-100 text-gray-600'}">
									{seq.status.charAt(0).toUpperCase() + seq.status.slice(1)}
								</span>
							</td>
							<td class="px-4 py-3 text-sm text-gray-500">
								{seq.stepCount ?? '—'}
							</td>
							<td class="px-4 py-3 text-sm text-gray-500">
								{seq.enrollmentCount ?? '—'}
							</td>
							<td class="px-4 py-3 text-sm text-gray-500">{formatDate(seq.createdAt)}</td>
							<td class="px-4 py-3">
								<button
									onclick={(e) => cloneSequence(e, seq.id)}
									disabled={cloningId === seq.id}
									title="Clone sequence"
									class="inline-flex items-center gap-1 px-2 py-1 text-xs font-medium text-gray-600 bg-gray-100 rounded hover:bg-gray-200 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
								>
									{#if cloningId === seq.id}
										<svg class="animate-spin h-3 w-3" fill="none" viewBox="0 0 24 24">
											<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
											<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z"></path>
										</svg>
										Cloning...
									{:else}
										<svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
											<path stroke-linecap="round" stroke-linejoin="round" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
										</svg>
										Clone
									{/if}
								</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>
