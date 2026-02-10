<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { get, put } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import type { FlowStep, FlowBuilderDefinition, FlowUpdateInput } from '$lib/types/flow';
	import { STEP_TYPES } from '$lib/types/flow';
	import type { EntityDef, FieldDef } from '$lib/types/admin';
	import StepEditor from '$lib/components/flow/StepEditor.svelte';

	let flowId = $derived($page.params.id);

	let entities = $state<EntityDef[]>([]);
	let fields = $state<FieldDef[]>([]);
	let loading = $state(true);
	let saving = $state(false);
	let notFound = $state(false);

	// Form state
	let name = $state('');
	let description = $state('');
	let isActive = $state(true);
	let version = $state(1);
	let triggerType = $state<'manual' | 'record_create' | 'record_update'>('manual');
	let triggerEntity = $state('');
	let buttonLabel = $state('Run Flow');
	let showOn = $state<string[]>(['detail']);
	let refreshOnComplete = $state(false);
	let steps = $state<FlowStep[]>([]);
	let variables = $state<{ name: string; type: string; default: string }[]>([]);

	// Selected step for editing
	let selectedStepId = $state<string | null>(null);

	async function loadFlow() {
		try {
			const result = await get<{
				id: string;
				name: string;
				description?: string;
				version: number;
				isActive: boolean;
				definition: string;
			}>(`/flows/${flowId}`);

			name = result.name;
			description = result.description || '';
			isActive = result.isActive;
			version = result.version;

			// Parse definition
			const def: FlowBuilderDefinition & { refreshOnComplete?: boolean } = JSON.parse(result.definition);
			triggerType = def.trigger.type as typeof triggerType;
			triggerEntity = def.trigger.entityType || '';
			buttonLabel = def.trigger.buttonLabel || 'Run Flow';
			showOn = def.trigger.showOn || ['detail'];
			refreshOnComplete = def.refreshOnComplete || false;
			steps = def.steps || [];
			variables = def.variables
				? Object.entries(def.variables).map(([name, v]) => ({
						name,
						type: v.type,
						default: String(v.default || '')
				  }))
				: [];

			if (steps.length > 0) {
				selectedStepId = steps[0].id;
			}
		} catch (e) {
			notFound = true;
			toast.error('Flow not found');
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

	function addStep(type: FlowStep['type']) {
		const id = `step_${Date.now()}`;
		let newStep: FlowStep;

		switch (type) {
			case 'screen':
				newStep = { id, type: 'screen', name: 'New Screen', fields: [] };
				break;
			case 'decision':
				newStep = { id, type: 'decision', name: 'New Decision', outcomes: [] };
				break;
			case 'assignment':
				newStep = { id, type: 'assignment', name: 'New Assignment', assignments: [] };
				break;
			case 'record_create':
				newStep = { id, type: 'record_create', name: 'Create Record', entity: '', fieldMappings: {} };
				break;
			case 'record_update':
				newStep = { id, type: 'record_update', name: 'Update Record', entity: '', recordId: '', fieldMappings: {} };
				break;
			case 'record_get':
				newStep = { id, type: 'record_get', name: 'Get Record', entity: '', recordId: '', storeResultAs: '' };
				break;
			case 'n8n_webhook':
				newStep = { id, type: 'n8n_webhook', name: 'Webhook', url: '' };
				break;
			case 'end':
				newStep = { id, type: 'end', name: 'End', message: 'Flow completed successfully!' };
				break;
		}

		steps = [...steps, newStep];
		selectedStepId = id;
	}

	function updateStep(step: FlowStep) {
		steps = steps.map(s => s.id === step.id ? step : s);
	}

	function deleteStep(id: string) {
		steps = steps.filter(s => s.id !== id);
		// Clear references to this step
		steps = steps.map(s => {
			if ('next' in s && s.next === id) {
				return { ...s, next: undefined };
			}
			if (s.type === 'decision') {
				return {
					...s,
					outcomes: s.outcomes.map(o => o.next === id ? { ...o, next: undefined } : o),
					defaultNext: s.defaultNext === id ? undefined : s.defaultNext
				};
			}
			return s;
		});
		if (selectedStepId === id) {
			selectedStepId = steps.length > 0 ? steps[0].id : null;
		}
	}

	function addVariable() {
		variables = [...variables, { name: '', type: 'string', default: '' }];
	}

	function updateVariable(index: number, updates: Partial<typeof variables[0]>) {
		variables = variables.map((v, i) => i === index ? { ...v, ...updates } : v);
	}

	function removeVariable(index: number) {
		variables = variables.filter((_, i) => i !== index);
	}

	function moveStep(index: number, direction: 'up' | 'down') {
		const newIndex = direction === 'up' ? index - 1 : index + 1;
		if (newIndex < 0 || newIndex >= steps.length) return;
		const newSteps = [...steps];
		[newSteps[index], newSteps[newIndex]] = [newSteps[newIndex], newSteps[index]];
		steps = newSteps;
	}

	async function handleSubmit() {
		if (!name.trim()) {
			toast.error('Name is required');
			return;
		}
		if (triggerType === 'manual' && !triggerEntity) {
			toast.error('Entity type is required for manual triggers');
			return;
		}
		if (steps.length === 0) {
			toast.error('At least one step is required');
			return;
		}

		// Build definition
		const definition: FlowBuilderDefinition & { refreshOnComplete?: boolean } = {
			trigger: {
				type: triggerType,
				entityType: triggerEntity || undefined,
				buttonLabel: triggerType === 'manual' ? buttonLabel : undefined,
				showOn: triggerType === 'manual' ? showOn : undefined
			},
			variables: variables.length > 0 ? Object.fromEntries(
				variables.filter(v => v.name).map(v => [v.name, { type: v.type as any, default: v.default || undefined }])
			) : undefined,
			refreshOnComplete: refreshOnComplete || undefined,
			steps
		};

		try {
			saving = true;
			const input: FlowUpdateInput = {
				name: name.trim(),
				description: description.trim() || undefined,
				isActive,
				definition
			};
			await put(`/flows/${flowId}`, input);
			toast.success('Flow saved');
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to save flow';
			toast.error(message);
		} finally {
			saving = false;
		}
	}

	$effect(() => {
		if (triggerEntity) {
			loadFields(triggerEntity);
		}
	});

	onMount(async () => {
		await loadEntities();
		await loadFlow();
		loading = false;
	});
</script>

<div class="max-w-6xl mx-auto space-y-6">
	<div class="flex justify-between items-center">
		<div>
			<h1 class="text-2xl font-bold text-gray-900">Edit Screen Flow</h1>
			<p class="text-sm text-gray-500 mt-1">Version {version}</p>
		</div>
		<a
			href="/admin/flows"
			class="text-sm text-gray-600 hover:text-gray-900"
		>
			&larr; Back to Flows
		</a>
	</div>

	{#if loading}
		<div class="text-center py-12 text-gray-500">Loading...</div>
	{:else if notFound}
		<div class="text-center py-12">
			<p class="text-red-500 mb-4">Flow not found</p>
			<a href="/admin/flows" class="text-blue-600 hover:underline">Back to flows</a>
		</div>
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
							class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500"
							placeholder="e.g., Lead Qualification Flow"
						/>
					</div>

					<div>
						<label for="triggerEntity" class="block text-sm font-medium text-gray-700">Entity Type *</label>
						<select
							id="triggerEntity"
							bind:value={triggerEntity}
							class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500"
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
						class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500"
						placeholder="Optional description for this flow"
					></textarea>
				</div>

				<div class="flex items-center gap-6">
					<div class="flex items-center gap-2">
						<input
							type="checkbox"
							id="isActive"
							bind:checked={isActive}
							class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded"
						/>
						<label for="isActive" class="text-sm text-gray-700">Active</label>
					</div>
					<div class="flex items-center gap-2">
						<input
							type="checkbox"
							id="refreshOnComplete"
							bind:checked={refreshOnComplete}
							class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded"
						/>
						<label for="refreshOnComplete" class="text-sm text-gray-700">Refresh page on complete</label>
					</div>
				</div>
			</div>

			<!-- Trigger Configuration -->
			<div class="bg-white shadow rounded-lg p-6 space-y-4">
				<h2 class="text-lg font-medium text-gray-900 border-b pb-2">Trigger</h2>

				<div class="grid grid-cols-1 md:grid-cols-3 gap-4">
					<div>
						<label class="block text-sm font-medium text-gray-700 mb-1">Trigger Type</label>
						<select
							bind:value={triggerType}
							class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500"
						>
							<option value="manual">Manual (Button)</option>
							<option value="record_create">Record Create</option>
							<option value="record_update">Record Update</option>
						</select>
					</div>

					{#if triggerType === 'manual'}
						<div>
							<label class="block text-sm font-medium text-gray-700 mb-1">Button Label</label>
							<input
								type="text"
								bind:value={buttonLabel}
								class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500"
								placeholder="Run Flow"
							/>
						</div>

						<div>
							<label class="block text-sm font-medium text-gray-700 mb-1">Show On</label>
							<div class="flex gap-4 mt-2">
								<label class="flex items-center text-sm">
									<input
										type="checkbox"
										checked={showOn.includes('detail')}
										onchange={(e) => {
											if (e.currentTarget.checked) {
												showOn = [...showOn, 'detail'];
											} else {
												showOn = showOn.filter(s => s !== 'detail');
											}
										}}
										class="mr-2 rounded"
									/>
									Detail Page
								</label>
								<label class="flex items-center text-sm">
									<input
										type="checkbox"
										checked={showOn.includes('list')}
										onchange={(e) => {
											if (e.currentTarget.checked) {
												showOn = [...showOn, 'list'];
											} else {
												showOn = showOn.filter(s => s !== 'list');
											}
										}}
										class="mr-2 rounded"
									/>
									List View
								</label>
							</div>
						</div>
					{/if}
				</div>
			</div>

			<!-- Variables -->
			<div class="bg-white shadow rounded-lg p-6 space-y-4">
				<div class="flex justify-between items-center border-b pb-2">
					<h2 class="text-lg font-medium text-gray-900">Variables</h2>
					<button
						type="button"
						onclick={addVariable}
						class="text-sm text-blue-600 hover:text-blue-800"
					>
						+ Add Variable
					</button>
				</div>

				{#if variables.length === 0}
					<p class="text-sm text-gray-500 text-center py-4">No variables defined. Variables store data throughout the flow.</p>
				{:else}
					<div class="space-y-2">
						{#each variables as variable, index}
							<div class="flex gap-2 items-center">
								<input
									type="text"
									value={variable.name}
									onchange={(e) => updateVariable(index, { name: e.currentTarget.value })}
									placeholder="Variable name"
									class="flex-1 rounded-md border-gray-300 text-sm"
								/>
								<select
									value={variable.type}
									onchange={(e) => updateVariable(index, { type: e.currentTarget.value })}
									class="rounded-md border-gray-300 text-sm"
								>
									<option value="string">String</option>
									<option value="number">Number</option>
									<option value="boolean">Boolean</option>
									<option value="date">Date</option>
									<option value="record">Record</option>
									<option value="list">List</option>
								</select>
								<input
									type="text"
									value={variable.default}
									onchange={(e) => updateVariable(index, { default: e.currentTarget.value })}
									placeholder="Default value"
									class="flex-1 rounded-md border-gray-300 text-sm"
								/>
								<button
									type="button"
									onclick={() => removeVariable(index)}
									class="text-red-600 hover:text-red-800"
								>
									&times;
								</button>
							</div>
						{/each}
					</div>
				{/if}
			</div>

			<!-- Steps -->
			<div class="bg-white shadow rounded-lg p-6 space-y-4">
				<div class="flex justify-between items-center border-b pb-2">
					<h2 class="text-lg font-medium text-gray-900">Steps</h2>
				</div>

				<!-- Step type selector -->
				<div class="flex flex-wrap gap-2">
					{#each STEP_TYPES as stepType}
						<button
							type="button"
							onclick={() => addStep(stepType.value)}
							class="inline-flex items-center px-3 py-1.5 border border-gray-300 text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50"
							title={stepType.description}
						>
							<svg class="w-4 h-4 mr-1.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={stepType.icon} />
							</svg>
							{stepType.label}
						</button>
					{/each}
				</div>

				{#if steps.length === 0}
					<div class="text-center py-8 text-gray-500 border-2 border-dashed border-gray-200 rounded-lg">
						<p>No steps defined. Click a button above to add a step.</p>
						<p class="text-xs mt-1">Start with a Screen step to collect user input.</p>
					</div>
				{:else}
					<div class="space-y-4">
						<!-- Step list -->
						<div class="flex gap-4">
							<!-- Step sidebar -->
							<div class="w-64 shrink-0 space-y-1">
								{#each steps as step, index (step.id)}
									<div
										class="flex items-center gap-2 p-2 rounded cursor-pointer transition-colors {selectedStepId === step.id ? 'bg-blue-100 border border-blue-300' : 'bg-gray-50 hover:bg-gray-100 border border-transparent'}"
										onclick={() => selectedStepId = step.id}
									>
										<span class="text-xs text-gray-400 w-5">{index + 1}</span>
										<span class="flex-1 text-sm truncate">{step.name}</span>
										<span class="text-xs text-gray-400 px-1.5 py-0.5 bg-gray-200 rounded">{step.type}</span>
										<div class="flex flex-col">
											<button
												type="button"
												onclick={(e) => { e.stopPropagation(); moveStep(index, 'up'); }}
												disabled={index === 0}
												class="text-gray-400 hover:text-gray-600 disabled:opacity-30"
											>
												<svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
													<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 15l7-7 7 7" />
												</svg>
											</button>
											<button
												type="button"
												onclick={(e) => { e.stopPropagation(); moveStep(index, 'down'); }}
												disabled={index === steps.length - 1}
												class="text-gray-400 hover:text-gray-600 disabled:opacity-30"
											>
												<svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
													<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
												</svg>
											</button>
										</div>
									</div>
								{/each}
							</div>

							<!-- Step editor -->
							<div class="flex-1">
								{#if selectedStepId}
									{@const selectedStep = steps.find(s => s.id === selectedStepId)}
									{#if selectedStep}
										<StepEditor
											step={selectedStep}
											allSteps={steps}
											{fields}
											entities={entities.map(e => ({ name: e.name, label: e.label }))}
											onUpdate={updateStep}
											onDelete={() => deleteStep(selectedStep.id)}
										/>
									{/if}
								{:else}
									<div class="text-center py-8 text-gray-500 border border-gray-200 rounded-lg">
										<p>Select a step from the list to edit it.</p>
									</div>
								{/if}
							</div>
						</div>
					</div>
				{/if}
			</div>

			<!-- Actions -->
			<div class="flex justify-end gap-3">
				<a
					href="/admin/flows"
					class="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-50"
				>
					Cancel
				</a>
				<button
					type="submit"
					disabled={saving}
					class="px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-600/90 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					{saving ? 'Saving...' : 'Save Flow'}
				</button>
			</div>
		</form>
	{/if}
</div>
