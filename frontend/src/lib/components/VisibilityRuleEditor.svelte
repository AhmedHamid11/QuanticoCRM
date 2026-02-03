<script lang="ts">
	import type { FieldDef } from '$lib/types/admin';
	import type { VisibilityRule, VisibilityCondition, VisibilityOperator } from '$lib/types/layout';
	import { VISIBILITY_OPERATORS } from '$lib/types/layout';

	interface Props {
		rule: VisibilityRule;
		fields: FieldDef[];
		onchange: (rule: VisibilityRule) => void;
	}

	let { rule, fields, onchange }: Props = $props();

	function generateId(): string {
		return 'cond_' + Math.random().toString(36).substr(2, 9);
	}

	function setType(type: 'always' | 'conditional' | 'never') {
		if (type === 'conditional' && (!rule.conditions || rule.conditions.length === 0)) {
			onchange({
				type,
				conditions: [{ id: generateId(), field: fields[0]?.name || '', operator: 'EQUALS' }],
				logic: 'AND'
			});
		} else {
			onchange({ ...rule, type });
		}
	}

	function addCondition() {
		const conditions = [...(rule.conditions || [])];
		conditions.push({
			id: generateId(),
			field: fields[0]?.name || '',
			operator: 'EQUALS'
		});
		onchange({ ...rule, conditions });
	}

	function removeCondition(id: string) {
		const conditions = (rule.conditions || []).filter((c) => c.id !== id);
		if (conditions.length === 0) {
			onchange({ type: 'always' });
		} else {
			onchange({ ...rule, conditions });
		}
	}

	function updateCondition(id: string, updates: Partial<VisibilityCondition>) {
		const conditions = (rule.conditions || []).map((c) =>
			c.id === id ? { ...c, ...updates } : c
		);
		onchange({ ...rule, conditions });
	}

	function setLogic(logic: 'AND' | 'OR') {
		onchange({ ...rule, logic });
	}

	function needsValue(operator: VisibilityOperator): boolean {
		return !['IS_EMPTY', 'IS_NOT_EMPTY', 'IS_TRUE', 'IS_FALSE'].includes(operator);
	}

	function needsMultipleValues(operator: VisibilityOperator): boolean {
		return ['IN', 'NOT_IN'].includes(operator);
	}

	function getFieldOptions(fieldName: string): string[] | null {
		const field = fields.find((f) => f.name === fieldName);
		if (field?.options) {
			try {
				return JSON.parse(field.options);
			} catch {
				return null;
			}
		}
		return null;
	}
</script>

<div class="space-y-4">
	<div class="flex items-center gap-4">
		<label class="text-sm font-medium text-gray-700">Visibility:</label>
		<select
			value={rule.type}
			onchange={(e) => setType(e.currentTarget.value as 'always' | 'conditional' | 'never')}
			class="rounded-md border-gray-300 shadow-sm text-sm focus:border-primary focus:ring-primary"
		>
			<option value="always">Always Visible</option>
			<option value="conditional">Conditional</option>
			<option value="never">Never Visible</option>
		</select>
	</div>

	{#if rule.type === 'conditional'}
		<div class="ml-4 space-y-3 p-4 bg-gray-50 rounded-lg border border-gray-200">
			{#if rule.conditions && rule.conditions.length > 1}
				<div class="flex items-center gap-2 text-sm">
					<span class="text-gray-600">Show when</span>
					<select
						value={rule.logic || 'AND'}
						onchange={(e) => setLogic(e.currentTarget.value as 'AND' | 'OR')}
						class="rounded-md border-gray-300 shadow-sm text-sm focus:border-primary focus:ring-primary"
					>
						<option value="AND">ALL conditions match</option>
						<option value="OR">ANY condition matches</option>
					</select>
				</div>
			{/if}

			{#each rule.conditions || [] as condition, index (condition.id)}
				<div class="flex items-start gap-2 p-3 bg-white rounded border border-gray-200">
					<div class="flex-1 grid grid-cols-1 sm:grid-cols-3 gap-2">
						<!-- Field selector -->
						<select
							value={condition.field}
							onchange={(e) => updateCondition(condition.id, { field: e.currentTarget.value })}
							class="rounded-md border-gray-300 shadow-sm text-sm focus:border-primary focus:ring-primary"
						>
							{#each fields as field (field.id)}
								<option value={field.name}>{field.label}</option>
							{/each}
						</select>

						<!-- Operator selector -->
						<select
							value={condition.operator}
							onchange={(e) =>
								updateCondition(condition.id, {
									operator: e.currentTarget.value as VisibilityOperator,
									value: undefined,
									values: undefined
								})}
							class="rounded-md border-gray-300 shadow-sm text-sm focus:border-primary focus:ring-primary"
						>
							{#each VISIBILITY_OPERATORS as op (op.value)}
								<option value={op.value}>{op.label}</option>
							{/each}
						</select>

						<!-- Value input -->
						{#if needsValue(condition.operator)}
							{@const fieldOptions = getFieldOptions(condition.field)}
							{#if needsMultipleValues(condition.operator)}
								{#if fieldOptions}
									<select
										multiple
										value={condition.values || []}
										onchange={(e) => {
											const selected = Array.from(e.currentTarget.selectedOptions, (opt) => opt.value);
											updateCondition(condition.id, { values: selected });
										}}
										class="rounded-md border-gray-300 shadow-sm text-sm focus:border-primary focus:ring-primary h-20"
									>
										{#each fieldOptions as opt}
											<option value={opt}>{opt}</option>
										{/each}
									</select>
								{:else}
									<input
										type="text"
										placeholder="value1, value2, ..."
										value={(condition.values || []).join(', ')}
										onchange={(e) =>
											updateCondition(condition.id, {
												values: e.currentTarget.value.split(',').map((s) => s.trim())
											})}
										class="rounded-md border-gray-300 shadow-sm text-sm focus:border-primary focus:ring-primary"
									/>
								{/if}
							{:else if fieldOptions}
								<select
									value={condition.value as string || ''}
									onchange={(e) => updateCondition(condition.id, { value: e.currentTarget.value })}
									class="rounded-md border-gray-300 shadow-sm text-sm focus:border-primary focus:ring-primary"
								>
									<option value="">Select value...</option>
									{#each fieldOptions as opt}
										<option value={opt}>{opt}</option>
									{/each}
								</select>
							{:else}
								<input
									type="text"
									placeholder="Value"
									value={condition.value as string || ''}
									onchange={(e) => updateCondition(condition.id, { value: e.currentTarget.value })}
									class="rounded-md border-gray-300 shadow-sm text-sm focus:border-primary focus:ring-primary"
								/>
							{/if}
						{/if}
					</div>

					<button
						onclick={() => removeCondition(condition.id)}
						class="p-1 text-red-500 hover:text-red-700 hover:bg-red-50 rounded"
						title="Remove condition"
					>
						<svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
						</svg>
					</button>
				</div>
			{/each}

			<button
				onclick={addCondition}
				class="flex items-center gap-1 text-sm text-primary hover:text-blue-800"
			>
				<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
				</svg>
				Add Condition
			</button>
		</div>
	{/if}
</div>
