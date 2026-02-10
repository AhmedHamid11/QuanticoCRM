<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { get, put, post } from '$lib/utils/api';
	import { addToast } from '$lib/stores/toast.svelte';
	import type { FieldDef } from '$lib/types/admin';
	import type {
		ValidationRule,
		ValidationRuleUpdateInput,
		ValidationCondition,
		ValidationAction,
		ValidationResult
	} from '$lib/types/validation';
	import {
		VALIDATION_OPERATORS as OPERATORS,
		VALIDATION_ACTION_TYPES as ACTION_TYPES,
		CONDITION_LOGIC_OPTIONS as LOGIC_OPTIONS
	} from '$lib/types/validation';

	let entityName = $derived($page.params.entity);
	let ruleId = $derived($page.params.id);

	let fields = $state<FieldDef[]>([]);
	let loading = $state(true);
	let saving = $state(false);
	let testing = $state(false);
	let testResult = $state<ValidationResult | null>(null);

	// Form state
	let name = $state('');
	let description = $state('');
	let enabled = $state(true);
	let triggerOnCreate = $state(false);
	let triggerOnUpdate = $state(true);
	let triggerOnDelete = $state(false);
	let conditionLogic = $state('AND');
	let errorMessage = $state('');
	let priority = $state(100);

	let conditions = $state<ValidationCondition[]>([]);
	let actions = $state<ValidationAction[]>([]);

	// Test data
	let testOperation = $state<'CREATE' | 'UPDATE' | 'DELETE'>('UPDATE');
	let testOldRecord = $state('{}');
	let testNewRecord = $state('{}');

	async function loadData() {
		try {
			loading = true;

			const [rule, fieldsData] = await Promise.all([
				get<ValidationRule>(`/admin/entities/${entityName}/validation-rules/${ruleId}`),
				get<FieldDef[]>(`/admin/entities/${entityName}/fields`)
			]);

			fields = fieldsData;

			// Populate form
			name = rule.name;
			description = rule.description || '';
			enabled = rule.enabled;
			triggerOnCreate = rule.triggerOnCreate;
			triggerOnUpdate = rule.triggerOnUpdate;
			triggerOnDelete = rule.triggerOnDelete;
			conditionLogic = rule.conditionLogic;
			errorMessage = rule.errorMessage || '';
			priority = rule.priority;
			conditions = rule.conditions || [];
			actions = rule.actions || [];
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to load rule';
			addToast(message, 'error');
			goto(`/admin/entity-manager/${entityName}/validation-rules`);
		} finally {
			loading = false;
		}
	}

	function addCondition() {
		conditions = [
			...conditions,
			{
				id: crypto.randomUUID(),
				fieldName: '',
				operator: 'EQUALS',
				value: ''
			}
		];
	}

	function removeCondition(id: string) {
		conditions = conditions.filter((c) => c.id !== id);
	}

	function addAction() {
		actions = [
			...actions,
			{
				type: 'BLOCK_SAVE',
				errorMessage: ''
			}
		];
	}

	function removeAction(index: number) {
		actions = actions.filter((_, i) => i !== index);
	}

	function getOperatorInfo(operator: string) {
		return OPERATORS.find((o) => o.value === operator);
	}

	function getActionTypeInfo(type: string) {
		return ACTION_TYPES.find((t) => t.value === type);
	}

	async function saveRule() {
		if (!name.trim()) {
			addToast('Name is required', 'error');
			return;
		}

		saving = true;
		try {
			const input: ValidationRuleUpdateInput = {
				name: name.trim(),
				description: description.trim() || undefined,
				enabled,
				triggerOnCreate,
				triggerOnUpdate,
				triggerOnDelete,
				conditionLogic,
				conditions: conditions.filter((c) => c.fieldName),
				actions,
				errorMessage: errorMessage.trim() || undefined,
				priority
			};

			await put<ValidationRule>(`/admin/entities/${entityName}/validation-rules/${ruleId}`, input);
			addToast('Validation rule updated', 'success');
			goto(`/admin/entity-manager/${entityName}/validation-rules`);
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to update rule';
			addToast(message, 'error');
		} finally {
			saving = false;
		}
	}

	async function testRule() {
		testing = true;
		testResult = null;

		try {
			let oldRecord = {};
			let newRecord = {};

			try {
				oldRecord = JSON.parse(testOldRecord);
			} catch {
				addToast('Invalid JSON in old record', 'error');
				testing = false;
				return;
			}

			try {
				newRecord = JSON.parse(testNewRecord);
			} catch {
				addToast('Invalid JSON in new record', 'error');
				testing = false;
				return;
			}

			const input = {
				rule: {
					name: name.trim() || 'Test Rule',
					entityType: entityName,
					triggerOnCreate,
					triggerOnUpdate,
					triggerOnDelete,
					conditionLogic,
					conditions: conditions.filter((c) => c.fieldName),
					actions,
					errorMessage: errorMessage.trim() || undefined
				},
				operation: testOperation,
				oldRecord: testOperation !== 'CREATE' ? oldRecord : undefined,
				newRecord: testOperation !== 'DELETE' ? newRecord : undefined
			};

			testResult = await post<ValidationResult>(
				`/admin/entities/${entityName}/validation-rules/test`,
				input
			);
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Test failed';
			addToast(message, 'error');
		} finally {
			testing = false;
		}
	}

	onMount(() => {
		loadData();
	});
</script>

<div class="space-y-6 max-w-4xl">
	<!-- Breadcrumb -->
	<nav class="text-sm text-gray-500 mb-2">
		<a href="/admin" class="hover:text-gray-700">Administration</a>
		<span class="mx-2">/</span>
		<a href="/admin/entity-manager" class="hover:text-gray-700">Entity Manager</a>
		<span class="mx-2">/</span>
		<a href="/admin/entity-manager/{entityName}" class="hover:text-gray-700">{entityName}</a>
		<span class="mx-2">/</span>
		<a href="/admin/entity-manager/{entityName}/validation-rules" class="hover:text-gray-700"
			>Validation Rules</a
		>
		<span class="mx-2">/</span>
		<span class="text-gray-900">Edit Rule</span>
	</nav>

	<h1 class="text-2xl font-bold text-gray-900">Edit Validation Rule</h1>

	{#if loading}
		<div class="text-center py-12 text-gray-500">Loading...</div>
	{:else}
		<!-- Basic Info -->
		<div class="bg-white shadow rounded-lg p-6 space-y-4">
			<h2 class="text-lg font-medium text-gray-900">Basic Information</h2>

			<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
				<div>
					<label class="block text-sm font-medium text-gray-700 mb-1">
						Rule Name <span class="text-red-500">*</span>
					</label>
					<input
						type="text"
						bind:value={name}
						placeholder="e.g., Lock fields when closed"
						class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
					/>
				</div>

				<div>
					<label class="block text-sm font-medium text-gray-700 mb-1">Priority</label>
					<input
						type="number"
						bind:value={priority}
						min="1"
						max="1000"
						class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
					/>
					<p class="text-xs text-gray-500 mt-1">Lower number = higher priority</p>
				</div>
			</div>

			<div>
				<label class="block text-sm font-medium text-gray-700 mb-1">Description</label>
				<textarea
					bind:value={description}
					rows="2"
					placeholder="Optional description of what this rule does"
					class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
				></textarea>
			</div>

			<div>
				<label class="block text-sm font-medium text-gray-700 mb-1">Default Error Message</label>
				<input
					type="text"
					bind:value={errorMessage}
					placeholder="e.g., This operation is not allowed"
					class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
				/>
			</div>

			<div class="flex items-center gap-6">
				<label class="flex items-center gap-2">
					<input
						type="checkbox"
						bind:checked={enabled}
						class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
					/>
					<span class="text-sm text-gray-700">Enabled</span>
				</label>
			</div>
		</div>

		<!-- Triggers -->
		<div class="bg-white shadow rounded-lg p-6 space-y-4">
			<h2 class="text-lg font-medium text-gray-900">Triggers</h2>
			<p class="text-sm text-gray-500">When should this rule be evaluated?</p>

			<div class="flex items-center gap-6">
				<label class="flex items-center gap-2">
					<input
						type="checkbox"
						bind:checked={triggerOnCreate}
						class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
					/>
					<span class="text-sm text-gray-700">On Create</span>
				</label>

				<label class="flex items-center gap-2">
					<input
						type="checkbox"
						bind:checked={triggerOnUpdate}
						class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
					/>
					<span class="text-sm text-gray-700">On Update</span>
				</label>

				<label class="flex items-center gap-2">
					<input
						type="checkbox"
						bind:checked={triggerOnDelete}
						class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
					/>
					<span class="text-sm text-gray-700">On Delete</span>
				</label>
			</div>
		</div>

		<!-- Conditions -->
		<div class="bg-white shadow rounded-lg p-6 space-y-4">
			<div class="flex justify-between items-center">
				<div>
					<h2 class="text-lg font-medium text-gray-900">Conditions</h2>
					<p class="text-sm text-gray-500">When should the actions be executed?</p>
				</div>
				<button
					onclick={addCondition}
					class="px-3 py-1 text-sm bg-gray-100 text-gray-700 rounded-md hover:bg-gray-200"
				>
					+ Add Condition
				</button>
			</div>

			{#if conditions.length > 1}
				<div>
					<label class="block text-sm font-medium text-gray-700 mb-1">Condition Logic</label>
					<select
						bind:value={conditionLogic}
						class="px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
					>
						{#each LOGIC_OPTIONS as option}
							<option value={option.value}>{option.label}</option>
						{/each}
					</select>
				</div>
			{/if}

			{#if conditions.length === 0}
				<p class="text-sm text-gray-500 py-4 text-center bg-gray-50 rounded">
					No conditions added. Actions will always execute when triggered.
				</p>
			{:else}
				<div class="space-y-3">
					{#each conditions as condition, index (condition.id)}
						<div class="flex items-start gap-3 p-3 bg-gray-50 rounded-lg">
							<div class="flex-1 grid grid-cols-1 md:grid-cols-3 gap-3">
								<div>
									<label class="block text-xs font-medium text-gray-500 mb-1">Field</label>
									<select
										bind:value={condition.fieldName}
										class="w-full px-2 py-1.5 text-sm border border-gray-300 rounded-md"
									>
										<option value="">Select field...</option>
										{#each fields as field (field.id)}
											<option value={field.name}>{field.label}</option>
										{/each}
									</select>
								</div>

								<div>
									<label class="block text-xs font-medium text-gray-500 mb-1">Operator</label>
									<select
										bind:value={condition.operator}
										class="w-full px-2 py-1.5 text-sm border border-gray-300 rounded-md"
									>
										{#each OPERATORS as op}
											<option value={op.value}>{op.label}</option>
										{/each}
									</select>
								</div>

								{#if getOperatorInfo(condition.operator)?.requiresValue}
									<div>
										<label class="block text-xs font-medium text-gray-500 mb-1">Value</label>
										<input
											type="text"
											bind:value={condition.value}
											placeholder="Value"
											class="w-full px-2 py-1.5 text-sm border border-gray-300 rounded-md"
										/>
									</div>
								{/if}

								{#if getOperatorInfo(condition.operator)?.requiresValues}
									<div>
										<label class="block text-xs font-medium text-gray-500 mb-1"
											>Values (comma-separated)</label
										>
										<input
											type="text"
											value={condition.values?.join(', ') || ''}
											oninput={(e) => {
												condition.values = (e.target as HTMLInputElement).value
													.split(',')
													.map((v) => v.trim())
													.filter((v) => v);
											}}
											placeholder="Value1, Value2, Value3"
											class="w-full px-2 py-1.5 text-sm border border-gray-300 rounded-md"
										/>
									</div>
								{/if}
							</div>

							<button
								onclick={() => removeCondition(condition.id)}
								class="p-1 text-gray-400 hover:text-red-600"
								title="Remove condition"
							>
								<svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
									<path
										stroke-linecap="round"
										stroke-linejoin="round"
										stroke-width="2"
										d="M6 18L18 6M6 6l12 12"
									/>
								</svg>
							</button>
						</div>
					{/each}
				</div>
			{/if}
		</div>

		<!-- Actions -->
		<div class="bg-white shadow rounded-lg p-6 space-y-4">
			<div class="flex justify-between items-center">
				<div>
					<h2 class="text-lg font-medium text-gray-900">Actions</h2>
					<p class="text-sm text-gray-500">What should happen when conditions are met?</p>
				</div>
				<button
					onclick={addAction}
					class="px-3 py-1 text-sm bg-gray-100 text-gray-700 rounded-md hover:bg-gray-200"
				>
					+ Add Action
				</button>
			</div>

			{#if actions.length === 0}
				<p class="text-sm text-gray-500 py-4 text-center bg-gray-50 rounded">
					No actions added. Add at least one action for the rule to have effect.
				</p>
			{:else}
				<div class="space-y-3">
					{#each actions as action, index}
						<div class="flex items-start gap-3 p-3 bg-gray-50 rounded-lg">
							<div class="flex-1 space-y-3">
								<div class="grid grid-cols-1 md:grid-cols-2 gap-3">
									<div>
										<label class="block text-xs font-medium text-gray-500 mb-1">Action Type</label>
										<select
											bind:value={action.type}
											class="w-full px-2 py-1.5 text-sm border border-gray-300 rounded-md"
										>
											{#each ACTION_TYPES as type}
												<option value={type.value}>{type.label}</option>
											{/each}
										</select>
										<p class="text-xs text-gray-400 mt-1">
											{getActionTypeInfo(action.type)?.description}
										</p>
									</div>

									{#if getActionTypeInfo(action.type)?.requiresFields}
										<div>
											<label class="block text-xs font-medium text-gray-500 mb-1">Fields</label>
											<select
												multiple
												value={action.fields || []}
												onchange={(e) => {
													const select = e.target as HTMLSelectElement;
													action.fields = Array.from(select.selectedOptions).map((o) => o.value);
												}}
												class="w-full px-2 py-1.5 text-sm border border-gray-300 rounded-md min-h-[80px]"
											>
												{#each fields as field (field.id)}
													<option value={field.name}>{field.label}</option>
												{/each}
											</select>
											<p class="text-xs text-gray-400 mt-1">Ctrl+click to select multiple</p>
										</div>
									{/if}

									{#if getActionTypeInfo(action.type)?.requiresFieldName}
										<div>
											<label class="block text-xs font-medium text-gray-500 mb-1">Field</label>
											<select
												bind:value={action.fieldName}
												class="w-full px-2 py-1.5 text-sm border border-gray-300 rounded-md"
											>
												<option value="">Select field...</option>
												{#each fields as field (field.id)}
													<option value={field.name}>{field.label}</option>
												{/each}
											</select>
										</div>
									{/if}

									{#if getActionTypeInfo(action.type)?.requiresValue}
										<div>
											<label class="block text-xs font-medium text-gray-500 mb-1"
												>Required Value</label
											>
											<input
												type="text"
												bind:value={action.value}
												placeholder="Value"
												class="w-full px-2 py-1.5 text-sm border border-gray-300 rounded-md"
											/>
										</div>
									{/if}
								</div>

								<div>
									<label class="block text-xs font-medium text-gray-500 mb-1">Error Message</label>
									<input
										type="text"
										bind:value={action.errorMessage}
										placeholder={`e.g., Cannot modify {{field}} after record is closed`}
										class="w-full px-2 py-1.5 text-sm border border-gray-300 rounded-md"
									/>
									<p class="text-xs text-gray-400 mt-1">Use {"{{field}}"} as placeholder for field name</p>
								</div>
							</div>

							<button
								onclick={() => removeAction(index)}
								class="p-1 text-gray-400 hover:text-red-600"
								title="Remove action"
							>
								<svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
									<path
										stroke-linecap="round"
										stroke-linejoin="round"
										stroke-width="2"
										d="M6 18L18 6M6 6l12 12"
									/>
								</svg>
							</button>
						</div>
					{/each}
				</div>
			{/if}
		</div>

		<!-- Test Section -->
		<div class="bg-white shadow rounded-lg p-6 space-y-4">
			<h2 class="text-lg font-medium text-gray-900">Test Rule</h2>
			<p class="text-sm text-gray-500">Test your rule against sample data.</p>

			<div class="grid grid-cols-1 md:grid-cols-3 gap-4">
				<div>
					<label class="block text-sm font-medium text-gray-700 mb-1">Operation</label>
					<select
						bind:value={testOperation}
						class="w-full px-3 py-2 border border-gray-300 rounded-md"
					>
						<option value="CREATE">Create</option>
						<option value="UPDATE">Update</option>
						<option value="DELETE">Delete</option>
					</select>
				</div>

				{#if testOperation !== 'CREATE'}
					<div>
						<label class="block text-sm font-medium text-gray-700 mb-1">Old Record (JSON)</label>
						<textarea
							bind:value={testOldRecord}
							rows="3"
							placeholder={`{"stage": "Open"}`}
							class="w-full px-3 py-2 border border-gray-300 rounded-md font-mono text-sm"
						></textarea>
					</div>
				{/if}

				{#if testOperation !== 'DELETE'}
					<div>
						<label class="block text-sm font-medium text-gray-700 mb-1">New Record (JSON)</label>
						<textarea
							bind:value={testNewRecord}
							rows="3"
							placeholder={`{"stage": "Closed Won", "amount": 1000}`}
							class="w-full px-3 py-2 border border-gray-300 rounded-md font-mono text-sm"
						></textarea>
					</div>
				{/if}
			</div>

			<button
				onclick={testRule}
				disabled={testing}
				class="px-4 py-2 bg-gray-100 text-gray-700 rounded-md hover:bg-gray-200 disabled:opacity-50"
			>
				{testing ? 'Testing...' : 'Run Test'}
			</button>

			{#if testResult}
				<div
					class="p-4 rounded-lg {testResult.valid
						? 'bg-green-50 border border-green-200'
						: 'bg-red-50 border border-red-200'}"
				>
					<div class="flex items-center gap-2">
						{#if testResult.valid}
							<svg class="w-5 h-5 text-green-500" fill="currentColor" viewBox="0 0 20 20">
								<path
									fill-rule="evenodd"
									d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
									clip-rule="evenodd"
								/>
							</svg>
							<span class="text-green-800 font-medium">Validation Passed</span>
						{:else}
							<svg class="w-5 h-5 text-red-500" fill="currentColor" viewBox="0 0 20 20">
								<path
									fill-rule="evenodd"
									d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z"
									clip-rule="evenodd"
								/>
							</svg>
							<span class="text-red-800 font-medium">Validation Failed</span>
						{/if}
					</div>

					{#if testResult.message}
						<p class="mt-2 text-sm {testResult.valid ? 'text-green-700' : 'text-red-700'}">
							{testResult.message}
						</p>
					{/if}

					{#if testResult.fieldErrors && testResult.fieldErrors.length > 0}
						<ul class="mt-2 text-sm text-red-700 list-disc list-inside">
							{#each testResult.fieldErrors as error}
								<li><strong>{error.field}</strong>: {error.message}</li>
							{/each}
						</ul>
					{/if}
				</div>
			{/if}
		</div>

		<!-- Actions -->
		<div class="flex justify-end gap-3">
			<a
				href="/admin/entity-manager/{entityName}/validation-rules"
				class="px-4 py-2 border border-gray-300 rounded-md text-gray-700 hover:bg-gray-50"
			>
				Cancel
			</a>
			<button
				onclick={saveRule}
				disabled={saving}
				class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-600/90 disabled:opacity-50 disabled:cursor-not-allowed"
			>
				{saving ? 'Saving...' : 'Save Changes'}
			</button>
		</div>
	{/if}
</div>
