<script lang="ts">
	import { goto } from '$app/navigation';
	import { post } from '$lib/utils/api';
	import type { FlowExecution, FlowDefinition, SubmitScreenRequest } from '$lib/types/flow';
	import ScreenRenderer from './ScreenRenderer.svelte';
	import FlowComplete from './FlowComplete.svelte';
	import Spinner from '$lib/components/ui/Spinner.svelte';

	interface Props {
		flowId: string;
		flowDefinition?: FlowDefinition;
		entity?: string;
		recordId?: string;
		refreshOnComplete?: boolean;
		onClose: () => void;
	}

	let { flowId, flowDefinition, entity, recordId, refreshOnComplete = false, onClose }: Props = $props();

	let execution = $state<FlowExecution | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let submitting = $state(false);

	// Start flow on mount
	$effect(() => {
		startFlow();
	});

	async function startFlow() {
		loading = true;
		error = null;

		try {
			execution = await post<FlowExecution>(`/flows/${flowId}/start`, {
				entity,
				recordId
			});
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to start flow';
		} finally {
			loading = false;
		}
	}

	async function handleScreenSubmit(data: SubmitScreenRequest) {
		if (!execution) return;

		submitting = true;
		error = null;

		try {
			execution = await post<FlowExecution>(`/flows/executions/${execution.id}/submit`, data);

			// Handle completion
			if (execution.status === 'completed') {
				if (execution.redirect) {
					// Redirect to specified record
					setTimeout(() => {
						const entityPath = execution!.redirect!.entity.toLowerCase();
						goto(`/${entityPath}/${execution!.redirect!.recordId}`);
						onClose();
					}, 2000);
				} else if (refreshOnComplete || flowDefinition?.refreshOnComplete) {
					// Refresh current page
					setTimeout(() => {
						onClose();
						window.location.reload();
					}, 2000);
				}
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to submit';
		} finally {
			submitting = false;
		}
	}
</script>

<div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50" role="dialog" aria-modal="true">
	<div class="bg-white rounded-lg shadow-xl w-full max-w-2xl max-h-[90vh] overflow-hidden">
		<!-- Header -->
		<div class="flex items-center justify-between px-6 py-4 border-b bg-gray-50">
			<div>
				<h2 class="text-lg font-semibold text-gray-900">
					{execution?.flowName || 'Loading...'}
				</h2>
				{#if execution?.screenDef}
					<p class="text-sm text-gray-500">{execution.screenDef.name}</p>
				{/if}
			</div>
			<button
				onclick={onClose}
				class="text-gray-400 hover:text-gray-600 p-1 rounded-full hover:bg-gray-200 transition-colors"
				aria-label="Close"
			>
				<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
				</svg>
			</button>
		</div>

		<!-- Content -->
		<div class="p-6 overflow-y-auto max-h-[calc(90vh-130px)]">
			{#if loading}
				<div class="flex items-center justify-center py-12">
					<Spinner size="lg" />
				</div>
			{:else if error}
				<div class="bg-red-50 text-red-700 p-4 rounded-lg border border-red-200">
					<div class="flex items-start gap-3">
						<svg class="w-5 h-5 flex-shrink-0 mt-0.5" fill="currentColor" viewBox="0 0 20 20">
							<path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
						</svg>
						<div>
							<h3 class="font-medium">Error</h3>
							<p class="mt-1 text-sm">{error}</p>
						</div>
					</div>
					<button
						onclick={startFlow}
						class="mt-3 text-sm text-red-600 hover:text-red-800 underline"
					>
						Try again
					</button>
				</div>
			{:else if execution?.status === 'paused_at_screen' && execution.screenDef}
				<ScreenRenderer
					screen={execution.screenDef}
					variables={execution.variables}
					{submitting}
					onSubmit={handleScreenSubmit}
				/>
			{:else if execution?.status === 'completed'}
				<FlowComplete
					message={execution.endMessage || 'Flow completed successfully.'}
					redirect={execution.redirect}
				/>
			{:else if execution?.status === 'failed'}
				<div class="bg-red-50 text-red-700 p-4 rounded-lg border border-red-200">
					<div class="flex items-start gap-3">
						<svg class="w-5 h-5 flex-shrink-0 mt-0.5" fill="currentColor" viewBox="0 0 20 20">
							<path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
						</svg>
						<div>
							<h3 class="font-medium">Flow Failed</h3>
							<p class="mt-1 text-sm">{execution.error || 'An unexpected error occurred.'}</p>
						</div>
					</div>
				</div>
			{:else}
				<div class="flex items-center justify-center py-12">
					<Spinner size="lg" />
				</div>
			{/if}
		</div>

		<!-- Footer -->
		{#if execution?.status === 'completed' || execution?.status === 'failed'}
			<div class="px-6 py-4 border-t bg-gray-50 flex justify-end">
				<button
					onclick={onClose}
					class="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50"
				>
					Close
				</button>
			</div>
		{/if}
	</div>
</div>
