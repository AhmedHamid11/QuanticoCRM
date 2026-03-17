<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { get, post, del } from '$lib/utils/api';
	import { addToast } from '$lib/stores/toast.svelte';
	import type {
		ValidationRule,
		ValidationRuleListResponse,
		VALIDATION_ACTION_TYPES
	} from '$lib/types/validation';

	let entityName = $derived($page.params.entity);

	let rules = $state<ValidationRule[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let total = $state(0);
	let currentPage = $state(1);
	let pageSize = $state(20);

	async function loadRules() {
		try {
			loading = true;
			error = null;

			const response = await get<ValidationRuleListResponse>(
				`/admin/entities/${entityName}/validation-rules?page=${currentPage}&pageSize=${pageSize}`
			);

			rules = response.data;
			total = response.total;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load validation rules';
			addToast(error, 'error');
		} finally {
			loading = false;
		}
	}

	async function toggleEnabled(rule: ValidationRule) {
		try {
			const updated = await post<ValidationRule>(
				`/admin/entities/${entityName}/validation-rules/${rule.id}/toggle`,
				{}
			);
			rules = rules.map((r) => (r.id === rule.id ? updated : r));
			addToast(`Rule ${updated.enabled ? 'enabled' : 'disabled'}`, 'success');
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to toggle rule';
			addToast(message, 'error');
		}
	}

	async function deleteRule(rule: ValidationRule) {
		if (!confirm(`Are you sure you want to delete the rule "${rule.name}"?`)) return;

		try {
			await del(`/admin/entities/${entityName}/validation-rules/${rule.id}`);
			rules = rules.filter((r) => r.id !== rule.id);
			total = total - 1;
			addToast('Rule deleted', 'success');
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to delete rule';
			addToast(message, 'error');
		}
	}

	function getTriggerBadges(rule: ValidationRule): string[] {
		const triggers: string[] = [];
		if (rule.triggerOnCreate) triggers.push('Create');
		if (rule.triggerOnUpdate) triggers.push('Update');
		if (rule.triggerOnDelete) triggers.push('Delete');
		return triggers;
	}

	function getActionSummary(rule: ValidationRule): string {
		if (rule.actions.length === 0) return 'No actions';
		const actionTypes = rule.actions.map((a) => {
			switch (a.type) {
				case 'BLOCK_SAVE':
					return 'Block';
				case 'LOCK_FIELDS':
					return 'Lock';
				case 'REQUIRE_VALUE':
					return 'Require';
				case 'ENFORCE_VALUE':
					return 'Enforce';
				default:
					return a.type;
			}
		});
		return [...new Set(actionTypes)].join(', ');
	}

	onMount(() => {
		loadRules();
	});
</script>

<div class="space-y-6">
	<!-- Breadcrumb -->
	<nav class="text-sm text-gray-500 mb-2">
		<a href="/admin" class="hover:text-gray-700">Administration</a>
		<span class="mx-2">/</span>
		<a href="/admin/entity-manager" class="hover:text-gray-700">Entity Manager</a>
		<span class="mx-2">/</span>
		<a href="/admin/entity-manager/{entityName}" class="hover:text-gray-700">{entityName}</a>
		<span class="mx-2">/</span>
		<span class="text-gray-900">Validation Rules</span>
	</nav>

	<!-- Header -->
	<div class="flex justify-between items-center">
		<div>
			<h1 class="text-2xl font-bold text-gray-900">Validation Rules</h1>
			<p class="text-sm text-gray-500 mt-1">
				Rules that validate {entityName} records before save operations
			</p>
		</div>
		<a
			href="/admin/entity-manager/{entityName}/validation-rules/new"
			class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-600/90"
		>
			+ New Rule
		</a>
	</div>

	{#if loading}
		<div class="text-center py-12 text-gray-500">Loading...</div>
	{:else if error}
		<div class="text-center py-12 text-red-500">{error}</div>
	{:else if rules.length === 0}
		<div class="bg-white shadow rounded-lg p-12 text-center">
			<svg
				class="mx-auto h-12 w-12 text-gray-400"
				fill="none"
				viewBox="0 0 24 24"
				stroke="currentColor"
			>
				<path
					stroke-linecap="round"
					stroke-linejoin="round"
					stroke-width="2"
					d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
				/>
			</svg>
			<h3 class="mt-4 text-lg font-medium text-gray-900">No Validation Rules</h3>
			<p class="mt-2 text-sm text-gray-500">
				Create validation rules to enforce business logic and data integrity.
			</p>
			<a
				href="/admin/entity-manager/{entityName}/validation-rules/new"
				class="mt-4 inline-block px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-600/90"
			>
				Create Your First Rule
			</a>
		</div>
	{:else}
		<div class="crm-card overflow-hidden">
			<table class="min-w-full divide-y divide-gray-200">
				<thead class="bg-gray-50">
					<tr>
						<th
							class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
							>Name</th
						>
						<th
							class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
							>Triggers</th
						>
						<th
							class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
							>Actions</th
						>
						<th
							class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
							>Priority</th
						>
						<th
							class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
							>Status</th
						>
						<th
							class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider"
							>Actions</th
						>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-200">
					{#each rules as rule (rule.id)}
						<tr class="hover:bg-gray-50">
							<td class="px-6 py-4">
								<div class="text-sm font-medium text-gray-900">{rule.name}</div>
								{#if rule.description}
									<div class="text-sm text-gray-500">{rule.description}</div>
								{/if}
							</td>
							<td class="px-6 py-4">
								<div class="flex flex-wrap gap-1">
									{#each getTriggerBadges(rule) as trigger}
										<span class="px-2 py-0.5 text-xs rounded-full bg-blue-100 text-blue-800">
											{trigger}
										</span>
									{/each}
								</div>
							</td>
							<td class="px-6 py-4 text-sm text-gray-500">
								{getActionSummary(rule)}
								<div class="text-xs text-gray-400">
									{rule.conditions.length} condition{rule.conditions.length !== 1 ? 's' : ''} ({rule.conditionLogic})
								</div>
							</td>
							<td class="px-6 py-4 text-sm text-gray-500">
								{rule.priority}
							</td>
							<td class="px-6 py-4">
								<button
									onclick={() => toggleEnabled(rule)}
									class="px-2 py-1 text-xs rounded-full {rule.enabled
										? 'bg-green-100 text-green-800'
										: 'bg-gray-100 text-gray-600'}"
								>
									{rule.enabled ? 'Active' : 'Inactive'}
								</button>
							</td>
							<td class="px-6 py-4 text-right text-sm">
								<a
									href="/admin/entity-manager/{entityName}/validation-rules/{rule.id}"
									class="text-blue-600 hover:text-blue-800 mr-3"
								>
									Edit
								</a>
								<button
									onclick={() => deleteRule(rule)}
									class="text-red-600 hover:text-red-800"
								>
									Delete
								</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		{#if total > pageSize}
			<div class="flex justify-between items-center text-sm text-gray-500">
				<span>Showing {rules.length} of {total} rules</span>
				<div class="flex gap-2">
					<button
						onclick={() => {
							currentPage--;
							loadRules();
						}}
						disabled={currentPage <= 1}
						class="px-3 py-1 border rounded disabled:opacity-50"
					>
						Previous
					</button>
					<button
						onclick={() => {
							currentPage++;
							loadRules();
						}}
						disabled={rules.length < pageSize}
						class="px-3 py-1 border rounded disabled:opacity-50"
					>
						Next
					</button>
				</div>
			</div>
		{/if}
	{/if}
</div>
