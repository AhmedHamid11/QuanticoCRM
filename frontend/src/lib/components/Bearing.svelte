<script lang="ts">
	import { patch } from '$lib/utils/api';
	import { addToast } from '$lib/stores/toast.svelte';
	import type { BearingWithStages } from '$lib/types/bearing';

	interface Props {
		bearing: BearingWithStages;
		currentValue: string | null;
		recordId: string;
		entityType: string;
		fieldName: string;
		readOnly?: boolean;
		onUpdate?: (newValue: string) => void;
	}

	let { bearing, currentValue, recordId, entityType, fieldName, readOnly = false, onUpdate }: Props = $props();

	let updating = $state(false);
	let showConfirmDialog = $state(false);
	let pendingStage = $state<string | null>(null);

	// Read allowUpdates from bearing config (defaults to true for backwards compatibility)
	let allowUpdates = $derived(bearing.allowUpdates ?? true);

	// Find current stage index
	let currentIndex = $derived(
		bearing.stages.findIndex(s => s.value === currentValue)
	);

	function getStageStatus(index: number): 'completed' | 'current' | 'upcoming' {
		if (currentIndex === -1) return 'upcoming'; // No current value
		if (index < currentIndex) return 'completed';
		if (index === currentIndex) return 'current';
		return 'upcoming';
	}

	async function handleStageClick(stage: typeof bearing.stages[0], index: number) {
		if (readOnly || updating || !allowUpdates) return;
		if (stage.value === currentValue) return;

		// Check if this is a backward movement
		if (bearing.confirmBackward && index < currentIndex) {
			pendingStage = stage.value;
			showConfirmDialog = true;
			return;
		}

		await updateStage(stage.value);
	}

	async function confirmBackward() {
		if (pendingStage) {
			await updateStage(pendingStage);
		}
		showConfirmDialog = false;
		pendingStage = null;
	}

	function cancelBackward() {
		showConfirmDialog = false;
		pendingStage = null;
	}

	async function updateStage(newValue: string) {
		updating = true;
		try {
			// Build the URL for updating the record (using generic entity API)
			const url = `/entities/${entityType}/records/${recordId}`;
			await patch(url, { [fieldName]: newValue });

			// Notify parent component
			if (onUpdate) {
				onUpdate(newValue);
			}

			addToast(`${bearing.name} updated`, 'success');
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to update stage';
			addToast(message, 'error');
		} finally {
			updating = false;
		}
	}
</script>

<div class="bearing-container">
	<div class="bearing-header">
		<span class="bearing-name">{bearing.name}</span>
	</div>

	<div class="bearing-stages" class:updates-disabled={!allowUpdates && !readOnly}>
		{#each bearing.stages as stage, i (stage.value)}
			{@const status = getStageStatus(i)}
			<button
				type="button"
				class="bearing-stage {status}"
				class:first={i === 0}
				class:last={i === bearing.stages.length - 1}
				class:disabled={readOnly || updating || !allowUpdates}
				disabled={readOnly || updating || !allowUpdates}
				onclick={() => handleStageClick(stage, i)}
				title={!allowUpdates ? stage.label : readOnly ? stage.label : `Click to set to ${stage.label}`}
			>
				<span class="stage-content">
					{#if status === 'completed'}
						<svg class="check-icon" viewBox="0 0 20 20" fill="currentColor">
							<path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
						</svg>
					{/if}
					<span class="stage-label">{stage.label}</span>
				</span>
			</button>
		{/each}
	</div>
</div>

<!-- Confirm Backward Dialog -->
{#if showConfirmDialog}
	<div class="confirm-overlay" onclick={cancelBackward}>
		<div class="confirm-dialog" onclick={(e) => e.stopPropagation()}>
			<h3>Confirm Stage Change</h3>
			<p>Are you sure you want to move backward to a previous stage?</p>
			<div class="confirm-actions">
				<button class="btn-cancel" onclick={cancelBackward}>Cancel</button>
				<button class="btn-confirm" onclick={confirmBackward}>
					Yes, Move Backward
				</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.bearing-container {
		margin-bottom: 1rem;
	}

	.bearing-header {
		margin-bottom: 0.5rem;
	}

	.bearing-name {
		font-size: 0.75rem;
		font-weight: 600;
		color: #6b7280;
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.bearing-stages.updates-disabled {
		opacity: 0.7;
	}

	.bearing-stages {
		display: flex;
		overflow-x: auto;
		gap: 0;
	}

	.bearing-stage {
		position: relative;
		flex: 1;
		min-width: 80px;
		padding: 0.75rem 1rem 0.75rem 1.5rem;
		border: none;
		cursor: pointer;
		font-size: 0.875rem;
		font-weight: 500;
		transition: all 0.2s ease;
		clip-path: polygon(0 0, calc(100% - 12px) 0, 100% 50%, calc(100% - 12px) 100%, 0 100%, 12px 50%);
		margin-left: -12px;
	}

	.bearing-stage.first {
		margin-left: 0;
		clip-path: polygon(0 0, calc(100% - 12px) 0, 100% 50%, calc(100% - 12px) 100%, 0 100%);
		border-radius: 4px 0 0 4px;
	}

	.bearing-stage.last {
		clip-path: polygon(0 0, 100% 0, 100% 100%, 0 100%, 12px 50%);
		border-radius: 0 4px 4px 0;
	}

	.bearing-stage.first.last {
		clip-path: none;
		border-radius: 4px;
		margin-left: 0;
	}

	/* Completed stage */
	.bearing-stage.completed {
		background: linear-gradient(135deg, #059669 0%, #10b981 100%);
		color: white;
	}

	.bearing-stage.completed:hover:not(.disabled) {
		background: linear-gradient(135deg, #047857 0%, #059669 100%);
	}

	/* Current stage */
	.bearing-stage.current {
		background: linear-gradient(135deg, #2563eb 0%, #3b82f6 100%);
		color: white;
		box-shadow: 0 2px 8px rgba(37, 99, 235, 0.3);
	}

	/* Upcoming stage */
	.bearing-stage.upcoming {
		background: #f3f4f6;
		color: #6b7280;
	}

	.bearing-stage.upcoming:hover:not(.disabled) {
		background: #e5e7eb;
		color: #374151;
	}

	.bearing-stage.disabled {
		cursor: default;
	}

	.stage-content {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 0.375rem;
	}

	.check-icon {
		width: 1rem;
		height: 1rem;
		flex-shrink: 0;
	}

	.stage-label {
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	/* Confirm Dialog */
	.confirm-overlay {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.5);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 1000;
	}

	.confirm-dialog {
		background: white;
		padding: 1.5rem;
		border-radius: 0.5rem;
		box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04);
		max-width: 400px;
		width: 90%;
	}

	.confirm-dialog h3 {
		font-size: 1.125rem;
		font-weight: 600;
		color: #111827;
		margin: 0 0 0.5rem;
	}

	.confirm-dialog p {
		color: #6b7280;
		margin: 0 0 1rem;
	}

	.confirm-actions {
		display: flex;
		gap: 0.75rem;
		justify-content: flex-end;
	}

	.btn-cancel {
		padding: 0.5rem 1rem;
		border: 1px solid #d1d5db;
		border-radius: 0.375rem;
		background: white;
		color: #374151;
		font-weight: 500;
		cursor: pointer;
	}

	.btn-cancel:hover {
		background: #f9fafb;
	}

	.btn-confirm {
		padding: 0.5rem 1rem;
		border: none;
		border-radius: 0.375rem;
		background: #dc2626;
		color: white;
		font-weight: 500;
		cursor: pointer;
	}

	.btn-confirm:hover {
		background: #b91c1c;
	}

	/* Responsive */
	@media (max-width: 640px) {
		.bearing-stage {
			min-width: 60px;
			padding: 0.5rem 0.75rem 0.5rem 1rem;
			font-size: 0.75rem;
		}

		.check-icon {
			width: 0.875rem;
			height: 0.875rem;
		}
	}
</style>
