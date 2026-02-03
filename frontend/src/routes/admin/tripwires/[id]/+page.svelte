<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { get, put, post } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import type { Tripwire, TripwireCondition, ConditionType, TripwireUpdateInput } from '$lib/types/tripwire';
	import type { EntityDef, FieldDef } from '$lib/types/admin';
	import { CONDITION_TYPES, CONDITION_LOGIC_OPTIONS } from '$lib/types/tripwire';

	let tripwire = $state<Tripwire | null>(null);
	let entities = $state<EntityDef[]>([]);
	let fields = $state<FieldDef[]>([]);
	let loading = $state(true);
	let saving = $state(false);
	let testing = $state(false);

	// Form state
	let name = $state('');
	let description = $state('');
	let entityType = $state('');
	let endpointUrl = $state('');
	let enabled = $state(true);
	let conditionLogic = $state('AND');
	let conditions = $state<TripwireCondition[]>([]);

	const tripwireId = $derived($page.params.id);

	async function loadTripwire() {
		try {
			tripwire = await get<Tripwire>(`/tripwires/${tripwireId}`);
			// Populate form state
			name = tripwire.name;
			description = tripwire.description || '';
			entityType = tripwire.entityType;
			endpointUrl = tripwire.endpointUrl;
			enabled = tripwire.enabled;
			conditionLogic = tripwire.conditionLogic || 'AND';
			conditions = tripwire.conditions || [];
		} catch (e) {
			toast.error('Failed to load tripwire');
			goto('/admin/tripwires');
		}
	}

	async function loadEntities() {
		try {
			const result = await get<EntityDef[]>('/admin/entities');
			entities = result;
		} catch (e) {
			toast.error('Failed to load entities');
		}
	}

	async function loadFields(entity: string) {
		if (!entity) {
			fields = [];
			return;
		}
		try {
			const result = await get<FieldDef[]>(`/admin/entities/${entity}/fields`);
			fields = result;
		} catch (e) {
			toast.error('Failed to load fields');
		}
	}

	function addCondition() {
		conditions = [...conditions, {
			id: crypto.randomUUID(),
			type: 'ISNEW' as ConditionType,
			fieldName: undefined,
			value: undefined,
			fromValue: undefined,
			toValue: undefined
		}];
	}

	function removeCondition(id: string) {
		conditions = conditions.filter(c => c.id !== id);
	}

	function updateCondition(id: string, updates: Partial<TripwireCondition>) {
		conditions = conditions.map(c => c.id === id ? { ...c, ...updates } : c);
	}

	function needsFieldSelector(type: ConditionType): boolean {
		return ['ISCHANGED', 'FIELD_EQUALS', 'FIELD_CHANGED_TO'].includes(type);
	}

	function needsValueInput(type: ConditionType): boolean {
		return type === 'FIELD_EQUALS';
	}

	function needsFromToInputs(type: ConditionType): boolean {
		return type === 'FIELD_CHANGED_TO';
	}

	async function handleSubmit() {
		if (!name.trim()) {
			toast.error('Name is required');
			return;
		}
		if (!entityType) {
			toast.error('Entity type is required');
			return;
		}
		if (!endpointUrl.trim()) {
			toast.error('Endpoint URL is required');
			return;
		}
		if (conditions.length === 0) {
			toast.error('At least one condition is required');
			return;
		}

		// Validate conditions
		for (const cond of conditions) {
			if (needsFieldSelector(cond.type) && !cond.fieldName) {
				toast.error('Field is required for the selected condition type');
				return;
			}
			if (needsValueInput(cond.type) && !cond.value) {
				toast.error('Value is required for FIELD_EQUALS condition');
				return;
			}
		}

		try {
			saving = true;
			const input: TripwireUpdateInput = {
				name: name.trim(),
				description: description.trim() || undefined,
				entityType,
				endpointUrl: endpointUrl.trim(),
				enabled,
				conditionLogic,
				conditions
			};
			await put(`/tripwires/${tripwireId}`, input);
			toast.success('Tripwire updated');
			goto('/admin/tripwires');
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to update tripwire';
			toast.error(message);
		} finally {
			saving = false;
		}
	}

	interface TestResult {
		success: boolean;
		statusCode?: number;
		statusText?: string;
		durationMs?: number;
		responseBody?: string;
		error?: string;
	}

	async function testConnection() {
		if (!endpointUrl.trim()) {
			toast.error('Please enter an endpoint URL first');
			return;
		}

		try {
			testing = true;
			const result = await post<TestResult>(`/tripwires/${tripwireId}/test`, {});

			if (result.success) {
				toast.success(`Connection successful! Status: ${result.statusCode} (${result.durationMs}ms)`);
			} else {
				const errorMsg = result.error || `Failed with status ${result.statusCode}`;
				toast.error(`Connection failed: ${errorMsg}`);
			}
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to test connection';
			toast.error(message);
		} finally {
			testing = false;
		}
	}

	$effect(() => {
		if (entityType) {
			loadFields(entityType);
		}
	});

	// Reload data when tripwire ID changes (handles navigation between tripwire edit pages)
	$effect(() => {
		// Track tripwireId to trigger reload on navigation
		const _id = tripwireId;

		// Reset state
		tripwire = null;
		entities = [];
		fields = [];
		loading = true;
		name = '';
		description = '';
		entityType = '';
		endpointUrl = '';
		enabled = true;
		conditionLogic = 'AND';
		conditions = [];

		// Load data
		(async () => {
			await Promise.all([loadEntities(), loadTripwire()]);
			loading = false;
		})();
	});
</script>

<div class="max-w-4xl mx-auto space-y-6">
	<div class="flex justify-between items-center">
		<div>
			<h1 class="text-2xl font-bold text-gray-900">Edit Tripwire</h1>
			<p class="text-sm text-gray-500 mt-1">Modify the webhook trigger configuration</p>
		</div>
		<a
			href="/admin/tripwires"
			class="text-sm text-gray-600 hover:text-gray-900"
		>
			&larr; Back to Tripwires
		</a>
	</div>

	{#if loading}
		<div class="text-center py-12 text-gray-500">Loading...</div>
	{:else if !tripwire}
		<div class="text-center py-12 text-red-500">Tripwire not found</div>
	{:else}
		<form onsubmit={(e) => { e.preventDefault(); handleSubmit(); }} class="space-y-6">
			<!-- Basic Information -->
			<div class="bg-white shadow rounded-lg p-6 space-y-4">
				<h2 class="text-lg font-medium text-gray-900 border-b pb-2">Basic Information</h2>

				<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
					<div>
						<label for="name" class="block text-sm font-medium text-gray-700">Name *</label>
						<input
							type="text"
							id="name"
							bind:value={name}
							class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary"
							placeholder="e.g., Notify on Lead Create"
						/>
					</div>

					<div>
						<label for="entityType" class="block text-sm font-medium text-gray-700">Entity Type *</label>
						<select
							id="entityType"
							bind:value={entityType}
							class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary"
						>
							<option value="">Select an entity</option>
							{#each entities as entity}
								<option value={entity.name}>{entity.label}</option>
							{/each}
						</select>
					</div>
				</div>

				<div>
					<label for="description" class="block text-sm font-medium text-gray-700">Description</label>
					<textarea
						id="description"
						bind:value={description}
						rows={2}
						class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary"
						placeholder="Optional description for this tripwire"
					></textarea>
				</div>

				<div>
					<label for="endpointUrl" class="block text-sm font-medium text-gray-700">Endpoint URL *</label>
					<div class="mt-1 flex gap-2">
						<input
							type="url"
							id="endpointUrl"
							bind:value={endpointUrl}
							class="block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary"
							placeholder="https://your-webhook-endpoint.com/webhook"
						/>
						<button
							type="button"
							onclick={testConnection}
							disabled={testing || !endpointUrl.trim()}
							class="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap"
						>
							{testing ? 'Testing...' : 'Test'}
						</button>
					</div>
					<p class="mt-1 text-xs text-gray-500">The webhook endpoint that will receive POST requests when conditions are met</p>
				</div>

				<div class="flex items-center gap-2">
					<input
						type="checkbox"
						id="enabled"
						bind:checked={enabled}
						class="h-4 w-4 text-primary focus:ring-primary border-gray-300 rounded"
					/>
					<label for="enabled" class="text-sm text-gray-700">Enabled</label>
				</div>
			</div>

			<!-- Conditions -->
			<div class="bg-white shadow rounded-lg p-6 space-y-4">
				<div class="flex justify-between items-center border-b pb-2">
					<h2 class="text-lg font-medium text-gray-900">Conditions</h2>
					<button
						type="button"
						onclick={addCondition}
						class="text-sm text-primary hover:text-blue-800"
					>
						+ Add Condition
					</button>
				</div>

				{#if conditions.length === 0}
					<div class="text-center py-8 text-gray-500">
						<p>No conditions defined. Add at least one condition.</p>
						<button
							type="button"
							onclick={addCondition}
							class="mt-2 text-primary hover:text-blue-800"
						>
							+ Add Condition
						</button>
					</div>
				{:else}
					<div>
						<label for="conditionLogic" class="block text-sm font-medium text-gray-700 mb-2">Condition Logic</label>
						<select
							id="conditionLogic"
							bind:value={conditionLogic}
							class="w-full md:w-auto rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary"
						>
							{#each CONDITION_LOGIC_OPTIONS as opt}
								<option value={opt.value}>{opt.label}</option>
							{/each}
						</select>
					</div>

					<div class="space-y-4">
						{#each conditions as condition, index (condition.id)}
							<div class="border border-gray-200 rounded-lg p-4 bg-gray-50">
								<div class="flex justify-between items-start mb-3">
									<span class="text-sm font-medium text-gray-500">Condition {index + 1}</span>
									<button
										type="button"
										onclick={() => removeCondition(condition.id)}
										class="text-red-600 hover:text-red-800 text-sm"
									>
										Remove
									</button>
								</div>

								<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
									<div>
										<label class="block text-sm font-medium text-gray-700 mb-1">Type</label>
										<select
											bind:value={condition.type}
											onchange={() => updateCondition(condition.id, { type: condition.type, fieldName: undefined, value: undefined, fromValue: undefined, toValue: undefined })}
											class="w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary"
										>
											{#each CONDITION_TYPES as ct}
												<option value={ct.value}>{ct.label}</option>
											{/each}
										</select>
										<p class="mt-1 text-xs text-gray-500">
											{CONDITION_TYPES.find(ct => ct.value === condition.type)?.description}
										</p>
									</div>

									{#if needsFieldSelector(condition.type)}
										<div>
											<label class="block text-sm font-medium text-gray-700 mb-1">Field</label>
											<select
												bind:value={condition.fieldName}
												class="w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary"
												disabled={!entityType}
											>
												<option value="">Select a field</option>
												{#each fields as field}
													<option value={field.name}>{field.label}</option>
												{/each}
											</select>
										</div>
									{/if}

									{#if needsValueInput(condition.type)}
										<div>
											<label class="block text-sm font-medium text-gray-700 mb-1">Value</label>
											<input
												type="text"
												bind:value={condition.value}
												class="w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary"
												placeholder="Enter value"
											/>
										</div>
									{/if}

									{#if needsFromToInputs(condition.type)}
										<div>
											<label class="block text-sm font-medium text-gray-700 mb-1">From Value (optional)</label>
											<input
												type="text"
												bind:value={condition.fromValue}
												class="w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary"
												placeholder="Previous value"
											/>
										</div>
										<div>
											<label class="block text-sm font-medium text-gray-700 mb-1">To Value (optional)</label>
											<input
												type="text"
												bind:value={condition.toValue}
												class="w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary"
												placeholder="New value"
											/>
										</div>
									{/if}
								</div>
							</div>
						{/each}
					</div>
				{/if}
			</div>

			<!-- Actions -->
			<div class="flex justify-end gap-3">
				<a
					href="/admin/tripwires"
					class="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-50"
				>
					Cancel
				</a>
				<button
					type="submit"
					disabled={saving}
					class="px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-black bg-primary hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					{saving ? 'Saving...' : 'Save Changes'}
				</button>
			</div>
		</form>
	{/if}
</div>
