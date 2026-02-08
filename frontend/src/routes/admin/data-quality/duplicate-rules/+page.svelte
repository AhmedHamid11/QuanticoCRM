<script lang="ts">
	import { onMount } from 'svelte';
	import { get, post, put, del } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import { getConfidenceBadgeClass } from '$lib/api/dedup';
	import type { EntityDef, FieldDef } from '$lib/types/admin';

	// Types for matching rules (inline until Plan 16-01 completes)
	interface DedupFieldConfig {
		fieldName: string;
		targetFieldName?: string;
		weight: number;
		algorithm: string; // "exact", "jaro_winkler", "email", "phone", "phonetic"
		threshold?: number;
		exactMatchBoost?: boolean;
	}

	interface MatchingRule {
		id: string;
		orgId: string;
		name: string;
		description?: string;
		entityType: string;
		targetEntityType?: string;
		isEnabled: boolean;
		priority: number;
		threshold: number;
		highConfidenceThreshold: number;
		mediumConfidenceThreshold: number;
		blockingStrategy: string;
		fieldConfigs: DedupFieldConfig[];
		mergeDisplayFields?: string[];
		createdAt: string;
		modifiedAt: string;
	}

	interface MatchingRuleCreateInput {
		name: string;
		description?: string;
		entityType: string;
		targetEntityType?: string;
		isEnabled: boolean;
		priority: number;
		threshold: number;
		highConfidenceThreshold: number;
		mediumConfidenceThreshold: number;
		blockingStrategy: string;
		fieldConfigs: DedupFieldConfig[];
		mergeDisplayFields?: string[];
	}

	// State management
	let rules = $state<MatchingRule[]>([]);
	let loading = $state(true);
	let expandedRuleId = $state<string | null>(null);
	let editingRule = $state<Partial<MatchingRule> | null>(null);
	let isCreatingNew = $state(false);
	let entityFilter = $state('');
	let entities = $state<EntityDef[]>([]);
	let entityFields = $state<FieldDef[]>([]);
	let testResults = $state<any>(null);
	let testLoading = $state(false);

	// Filtered rules based on entity filter
	let filteredRules = $derived.by(() => {
		if (!entityFilter) return rules;
		return rules.filter(r => r.entityType === entityFilter);
	});

	// Algorithm options for field config dropdown
	const algorithmOptions = [
		{ value: 'exact', label: 'Exact Match' },
		{ value: 'jaro_winkler', label: 'Fuzzy (Jaro-Winkler)' },
		{ value: 'phonetic', label: 'Phonetic (Soundex)' },
		{ value: 'email', label: 'Email' },
		{ value: 'phone', label: 'Phone' }
	];

	// Blocking strategy options
	const blockingStrategyOptions = [
		{ value: 'none', label: 'None' },
		{ value: 'soundex', label: 'Soundex' },
		{ value: 'prefix', label: 'Prefix' },
		{ value: 'domain', label: 'Domain' },
		{ value: 'phone', label: 'Phone' }
	];

	async function loadData() {
		try {
			loading = true;
			const [rulesResponse, entitiesData] = await Promise.all([
				get<{ data: MatchingRule[] }>('/dedup/rules'),
				get<EntityDef[]>('/admin/entities')
			]);
			rules = rulesResponse.data || [];
			entities = entitiesData;
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to load data');
		} finally {
			loading = false;
		}
	}

	async function loadEntityFields(entityType: string) {
		try {
			entityFields = await get<FieldDef[]>(`/admin/entities/${entityType}/fields`);
		} catch (e) {
			toast.error('Failed to load entity fields');
			entityFields = [];
		}
	}

	function openNewRule() {
		isCreatingNew = true;
		expandedRuleId = null;
		editingRule = {
			name: '',
			description: '',
			entityType: '',
			isEnabled: true,
			priority: 1,
			threshold: 0.85,
			highConfidenceThreshold: 0.95,
			mediumConfidenceThreshold: 0.85,
			blockingStrategy: 'none',
			fieldConfigs: []
		};
		entityFields = [];
	}

	async function expandRule(rule: MatchingRule) {
		if (expandedRuleId === rule.id) {
			// Collapse if already expanded
			expandedRuleId = null;
			editingRule = null;
			isCreatingNew = false;
		} else {
			// Expand and load fields for this entity
			expandedRuleId = rule.id;
			isCreatingNew = false;
			editingRule = { ...rule };
			await loadEntityFields(rule.entityType);
		}
	}

	function cancelEdit() {
		expandedRuleId = null;
		editingRule = null;
		isCreatingNew = false;
		entityFields = [];
		testResults = null;
	}

	async function saveRule() {
		if (!editingRule) return;

		try {
			if (isCreatingNew) {
				// Create new rule
				const input: MatchingRuleCreateInput = {
					name: editingRule.name || '',
					description: editingRule.description || '',
					entityType: editingRule.entityType || '',
					targetEntityType: editingRule.targetEntityType,
					isEnabled: editingRule.isEnabled ?? true,
					priority: editingRule.priority || 1,
					threshold: editingRule.threshold || 0.85,
					highConfidenceThreshold: editingRule.highConfidenceThreshold || 0.95,
					mediumConfidenceThreshold: editingRule.mediumConfidenceThreshold || 0.85,
					blockingStrategy: editingRule.blockingStrategy || 'none',
					fieldConfigs: editingRule.fieldConfigs || [],
					mergeDisplayFields: editingRule.mergeDisplayFields
				};
				const created = await post<MatchingRule>('/dedup/rules', input);
				rules = [...rules, created];
				toast.success('Rule created successfully');
			} else {
				// Update existing rule
				const updated = await put<MatchingRule>(`/dedup/rules/${editingRule.id}`, editingRule);
				rules = rules.map(r => r.id === updated.id ? updated : r);
				toast.success('Rule updated successfully');
			}
			cancelEdit();
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to save rule');
		}
	}

	async function deleteRule(rule: MatchingRule) {
		if (!confirm(`Delete rule '${rule.name}'? This cannot be undone.`)) {
			return;
		}

		// Optimistic UI
		const backup = [...rules];
		rules = rules.filter(r => r.id !== rule.id);

		try {
			await del(`/dedup/rules/${rule.id}`);
			toast.success('Rule deleted successfully');
		} catch (e) {
			// Restore on error
			rules = backup;
			toast.error(e instanceof Error ? e.message : 'Failed to delete rule');
		}
	}

	async function testRule() {
		if (!editingRule?.entityType) {
			toast.error('Select an entity type first');
			return;
		}

		try {
			testLoading = true;
			testResults = null;

			// Call the check endpoint with a sample record
			// This is a lightweight test to see if the rule works
			const response = await post(`/dedup/${editingRule.entityType}/check`, {
				// Empty or minimal record - just testing the rule engine
				id: 'test-record',
				name: 'Test Record'
			});

			testResults = response;
			toast.success('Test completed');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Test failed');
			testResults = null;
		} finally {
			testLoading = false;
		}
	}

	function addFieldConfig() {
		if (!editingRule) return;

		editingRule.fieldConfigs = [
			...(editingRule.fieldConfigs || []),
			{
				fieldName: '',
				weight: 1.0,
				algorithm: 'exact',
				threshold: 0.88
			}
		];
	}

	function removeFieldConfig(index: number) {
		if (!editingRule?.fieldConfigs) return;
		editingRule.fieldConfigs = editingRule.fieldConfigs.filter((_, i) => i !== index);
	}

	async function handleEntityTypeChange() {
		if (editingRule?.entityType) {
			await loadEntityFields(editingRule.entityType);
		}
	}

	onMount(() => {
		loadData();
	});
</script>

<div class="max-w-7xl mx-auto px-4 py-8">
	<!-- Header -->
	<div class="mb-8 flex items-center justify-between">
		<div>
			<h1 class="text-3xl font-bold text-gray-900">Duplicate Rules</h1>
			<p class="mt-2 text-sm text-gray-600">
				Configure matching rules to detect duplicate records
			</p>
		</div>
		<button
			onclick={openNewRule}
			class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors"
		>
			New Rule
		</button>
	</div>

	<!-- Entity Filter -->
	<div class="mb-6">
		<label class="block text-sm font-medium text-gray-700 mb-2">
			Filter by Entity Type
		</label>
		<select
			bind:value={entityFilter}
			class="w-64 px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
		>
			<option value="">All Entities</option>
			{#each entities as entity}
				<option value={entity.name}>{entity.label}</option>
			{/each}
		</select>
	</div>

	{#if loading}
		<div class="text-center py-12 text-gray-500">
			Loading rules...
		</div>
	{:else if filteredRules.length === 0 && !isCreatingNew}
		<div class="text-center py-12 border-2 border-dashed border-gray-300 rounded-lg">
			<p class="text-gray-500 mb-4">No rules configured</p>
			<button
				onclick={openNewRule}
				class="text-blue-600 hover:text-blue-700 font-medium"
			>
				Create your first rule
			</button>
		</div>
	{:else}
		<!-- New Rule Form (if creating) -->
		{#if isCreatingNew}
			<div class="mb-6 border border-gray-300 rounded-lg p-6 bg-gray-50">
				<h3 class="text-lg font-semibold mb-4">New Rule</h3>
				{@render editForm()}
			</div>
		{/if}

		<!-- Rules Table -->
		<div class="bg-white border border-gray-200 rounded-lg overflow-hidden">
			<table class="min-w-full divide-y divide-gray-200">
				<thead class="bg-gray-50">
					<tr>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Name
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Entity Type
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Status
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Priority
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Threshold
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Fields
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Actions
						</th>
					</tr>
				</thead>
				<tbody class="bg-white divide-y divide-gray-200">
					{#each filteredRules as rule}
						<tr
							onclick={() => expandRule(rule)}
							class="cursor-pointer hover:bg-gray-50 transition-colors"
						>
							<td class="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">
								{rule.name}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{entities.find(e => e.name === rule.entityType)?.label || rule.entityType}
							</td>
							<td class="px-6 py-4 whitespace-nowrap">
								<span class="px-2 inline-flex text-xs leading-5 font-semibold rounded-full {rule.isEnabled ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-800'}">
									{rule.isEnabled ? 'Enabled' : 'Disabled'}
								</span>
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{rule.priority}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{Math.round(rule.threshold * 100)}%
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{rule.fieldConfigs.length} fields
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								<button
									onclick={(e) => {
										e.stopPropagation();
										deleteRule(rule);
									}}
									class="text-red-600 hover:text-red-800"
								>
									Delete
								</button>
							</td>
						</tr>

						<!-- Expanded Edit Form -->
						{#if expandedRuleId === rule.id}
							<tr>
								<td colspan="7" class="px-6 py-4 bg-gray-50">
									{@render editForm()}
								</td>
							</tr>
						{/if}
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>

{#snippet editForm()}
	{#if editingRule}
		<div class="space-y-6">
			<!-- Basic Info -->
			<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
				<div>
					<label class="block text-sm font-medium text-gray-700 mb-1">
						Name <span class="text-red-500">*</span>
					</label>
					<input
						type="text"
						bind:value={editingRule.name}
						class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
						placeholder="e.g., Contact Email Match"
					/>
				</div>

				<div>
					<label class="block text-sm font-medium text-gray-700 mb-1">
						Entity Type <span class="text-red-500">*</span>
					</label>
					<select
						bind:value={editingRule.entityType}
						onchange={handleEntityTypeChange}
						disabled={!isCreatingNew}
						class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 {!isCreatingNew ? 'bg-gray-100' : ''}"
					>
						<option value="">Select entity...</option>
						{#each entities as entity}
							<option value={entity.name}>{entity.label}</option>
						{/each}
					</select>
				</div>
			</div>

			<div>
				<label class="block text-sm font-medium text-gray-700 mb-1">
					Description
				</label>
				<input
					type="text"
					bind:value={editingRule.description}
					class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
					placeholder="Optional description"
				/>
			</div>

			<!-- Status and Priority -->
			<div class="grid grid-cols-1 md:grid-cols-3 gap-4">
				<div>
					<label class="flex items-center space-x-2">
						<input
							type="checkbox"
							bind:checked={editingRule.isEnabled}
							class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
						/>
						<span class="text-sm font-medium text-gray-700">Enabled</span>
					</label>
				</div>

				<div>
					<label class="block text-sm font-medium text-gray-700 mb-1">
						Priority
					</label>
					<input
						type="number"
						bind:value={editingRule.priority}
						min="1"
						class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
					/>
				</div>

				<div>
					<label class="block text-sm font-medium text-gray-700 mb-1">
						Blocking Strategy
					</label>
					<select
						bind:value={editingRule.blockingStrategy}
						class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
					>
						{#each blockingStrategyOptions as option}
							<option value={option.value}>{option.label}</option>
						{/each}
					</select>
				</div>
			</div>

			<!-- Confidence Thresholds -->
			<div>
				<h4 class="text-sm font-medium text-gray-700 mb-3">Confidence Thresholds</h4>
				<div class="grid grid-cols-1 md:grid-cols-3 gap-4">
					<div>
						<label class="block text-sm text-gray-600 mb-1">
							Overall Threshold
						</label>
						<input
							type="number"
							bind:value={editingRule.threshold}
							min="0"
							max="1"
							step="0.05"
							class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
						/>
						<p class="text-xs text-gray-500 mt-1">
							Minimum score to detect duplicate
						</p>
					</div>

					<div>
						<label class="block text-sm text-gray-600 mb-1 flex items-center gap-2">
							High Confidence
							<span class="px-2 py-0.5 text-xs rounded {getConfidenceBadgeClass('high')} border">
								High
							</span>
						</label>
						<input
							type="number"
							bind:value={editingRule.highConfidenceThreshold}
							min="0"
							max="1"
							step="0.05"
							class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
						/>
						<p class="text-xs text-gray-500 mt-1">
							≥ {Math.round((editingRule.highConfidenceThreshold || 0.95) * 100)}%
						</p>
					</div>

					<div>
						<label class="block text-sm text-gray-600 mb-1 flex items-center gap-2">
							Medium Confidence
							<span class="px-2 py-0.5 text-xs rounded {getConfidenceBadgeClass('medium')} border">
								Medium
							</span>
						</label>
						<input
							type="number"
							bind:value={editingRule.mediumConfidenceThreshold}
							min="0"
							max="1"
							step="0.05"
							class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
						/>
						<p class="text-xs text-gray-500 mt-1">
							{Math.round((editingRule.mediumConfidenceThreshold || 0.85) * 100)}% - {Math.round((editingRule.highConfidenceThreshold || 0.95) * 100 - 1)}%
						</p>
					</div>
				</div>
				<div class="mt-2">
					<div class="flex items-center gap-2 text-sm text-gray-600">
						<span class="px-2 py-0.5 text-xs rounded {getConfidenceBadgeClass('low')} border">
							Low
						</span>
						<span>
							&lt; {Math.round((editingRule.mediumConfidenceThreshold || 0.85) * 100)}%
						</span>
					</div>
				</div>
			</div>

			<!-- Field Configurations -->
			<div>
				<div class="flex items-center justify-between mb-3">
					<h4 class="text-sm font-medium text-gray-700">Field Configurations</h4>
					<button
						onclick={addFieldConfig}
						class="text-sm text-blue-600 hover:text-blue-700 font-medium"
					>
						+ Add Field
					</button>
				</div>

				{#if editingRule.fieldConfigs && editingRule.fieldConfigs.length > 0}
					<div class="space-y-3">
						{#each editingRule.fieldConfigs as config, index}
							<div class="flex flex-col md:flex-row gap-3 items-start border border-gray-200 rounded-md p-3 bg-white">
								<div class="flex-1">
									<label class="block text-xs text-gray-600 mb-1">Field</label>
									<select
										bind:value={config.fieldName}
										class="w-full px-2 py-1.5 text-sm border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500"
									>
										<option value="">Select field...</option>
										{#each entityFields as field}
											<option value={field.name}>{field.label}</option>
										{/each}
									</select>
								</div>

								<div class="flex-1">
									<label class="block text-xs text-gray-600 mb-1">Match Type</label>
									<select
										bind:value={config.algorithm}
										class="w-full px-2 py-1.5 text-sm border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500"
									>
										{#each algorithmOptions as option}
											<option value={option.value}>{option.label}</option>
										{/each}
									</select>
								</div>

								<div class="w-24">
									<label class="block text-xs text-gray-600 mb-1">Weight</label>
									<input
										type="number"
										bind:value={config.weight}
										min="0"
										max="100"
										step="1"
										class="w-full px-2 py-1.5 text-sm border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500"
									/>
								</div>

								<button
									onclick={() => removeFieldConfig(index)}
									class="mt-5 text-red-600 hover:text-red-800 text-sm font-medium"
								>
									Remove
								</button>
							</div>
						{/each}
					</div>
				{:else}
					<p class="text-sm text-gray-500 italic">
						No fields configured. Click "Add Field" to get started.
					</p>
				{/if}
			</div>

			<!-- Test Rule -->
			{#if !isCreatingNew}
				<div class="border-t pt-4">
					<button
						onclick={testRule}
						disabled={testLoading}
						class="px-4 py-2 bg-gray-100 text-gray-700 rounded-md hover:bg-gray-200 transition-colors disabled:opacity-50"
					>
						{testLoading ? 'Testing...' : 'Test Rule'}
					</button>

					{#if testResults}
						<div class="mt-4 p-4 bg-blue-50 border border-blue-200 rounded-md">
							<h5 class="font-medium text-sm text-blue-900 mb-2">Test Results</h5>
							<pre class="text-xs text-blue-800 overflow-auto">{JSON.stringify(testResults, null, 2)}</pre>
						</div>
					{/if}
				</div>
			{/if}

			<!-- Action Buttons -->
			<div class="flex items-center gap-3 border-t pt-4">
				<button
					onclick={saveRule}
					class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors"
				>
					Save
				</button>
				<button
					onclick={cancelEdit}
					class="px-4 py-2 bg-gray-100 text-gray-700 rounded-md hover:bg-gray-200 transition-colors"
				>
					Cancel
				</button>
			</div>
		</div>
	{/if}
{/snippet}
