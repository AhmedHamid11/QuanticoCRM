<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { get, del } from '$lib/utils/api';
	import { addToast } from '$lib/stores/toast.svelte';
	import RelatedList from '$lib/components/RelatedList.svelte';
	import ActivitiesStream from '$lib/components/ActivitiesStream.svelte';
	import Bearing from '$lib/components/Bearing.svelte';
	import SectionRenderer from '$lib/components/SectionRenderer.svelte';
	import DetailPageAlertWrapper from '$lib/components/DetailPageAlertWrapper.svelte';
	import type { Task } from '$lib/types/task';
	import type { RelatedListConfig } from '$lib/types/related-list';
	import type { BearingWithStages } from '$lib/types/bearing';
	import type { EntityDef, FieldDef } from '$lib/types/admin';
	import type { LayoutDataV2 } from '$lib/types/layout';
	import { parseLayoutData, getVisibleSections } from '$lib/types/layout';

	type TabId = 'details' | 'activities';

	// Read initial tab from URL query param
	let initialTab = $derived($page.url.searchParams.get('tab') as TabId | null);
	let activeTab = $state<TabId>('details');

	// Set initial tab from URL on first load
	$effect(() => {
		if (initialTab && (initialTab === 'details' || initialTab === 'activities')) {
			activeTab = initialTab;
		}
	});

	// System fields that exist as columns in the tasks table
	const SYSTEM_FIELDS = new Set([
		'subject', 'description', 'status', 'priority', 'type', 'dueDate',
		'parentId', 'parentType', 'parentName', 'assignedUserId',
		'createdAt', 'modifiedAt', 'createdById', 'modifiedById', 'deleted'
	]);

	function isSystemField(fieldName: string): boolean {
		return SYSTEM_FIELDS.has(fieldName);
	}

	let task = $state<(Task & { customFields?: Record<string, unknown> }) | null>(null);
	let relatedListConfigs = $state<RelatedListConfig[]>([]);
	let bearings = $state<BearingWithStages[]>([]);
	let fields = $state<FieldDef[]>([]);
	let layout = $state<LayoutDataV2 | null>(null);
	let entityDef = $state<EntityDef | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	let taskId = $derived($page.params.id);

	// Filter to only enabled configs, sorted by sortOrder
	let enabledRelatedLists = $derived(
		relatedListConfigs
			.filter((c) => c.enabled)
			.sort((a, b) => a.sortOrder - b.sortOrder)
	);

	// Get record data as a flat object for visibility evaluation
	let recordData = $derived(() => {
		if (!task) return {};
		const data: Record<string, unknown> = {};

		// Add system fields
		for (const fieldName of SYSTEM_FIELDS) {
			if (fieldName in task) {
				data[fieldName] = (task as unknown as Record<string, unknown>)[fieldName];
			}
		}

		// Add custom fields
		if (task.customFields) {
			for (const [key, value] of Object.entries(task.customFields)) {
				data[key] = value;
			}
		}

		// Add rollup and other dynamic fields from the task object
		for (const field of fields) {
			if (!SYSTEM_FIELDS.has(field.name) && field.name in task && !(field.name in data)) {
				data[field.name] = (task as unknown as Record<string, unknown>)[field.name];
			}
		}

		return data;
	});

	// Get visible sections based on record data
	let visibleSections = $derived(() => {
		if (!layout) return [];
		return getVisibleSections(layout, recordData());
	});

	async function loadTask() {
		try {
			loading = true;
			error = null;
			const [taskData, configsData, bearingsData, fieldsData, entityDefData] = await Promise.all([
				get<Task>(`/tasks/${taskId}`),
				get<RelatedListConfig[]>(`/entities/Task/related-list-configs`).catch(() => []),
				get<BearingWithStages[]>(`/entities/Task/bearings`).catch(() => []),
				get<FieldDef[]>(`/entities/Task/fields`).catch(() => []),
				get<EntityDef>(`/entities/Task/def`).catch(() => null)
			]);
			task = taskData;
			relatedListConfigs = configsData;
			bearings = bearingsData;
			fields = fieldsData;
			entityDef = entityDefData;

			// Load layout (may be v1, v2, or legacy section array format)
			try {
				const layoutResponse = await get<{ layoutData: string }>(
					'/entities/Task/layouts/detail'
				);
				layout = parseLayoutData(layoutResponse.layoutData, fieldsData.map(f => f.name));
			} catch {
				// Default to field-based layout if no layout exists
				layout = parseLayoutData('[]', fieldsData.map(f => f.name));
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load task';
			addToast(error, 'error');
		} finally {
			loading = false;
		}
	}

	async function deleteTask() {
		if (!confirm('Are you sure you want to delete this task?')) return;

		try {
			await del(`/tasks/${taskId}`);
			addToast('Task deleted', 'success');
			goto('/tasks');
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to delete task';
			addToast(message, 'error');
		}
	}

	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleString();
	}

	function getFieldDef(fieldName: string): FieldDef | undefined {
		return fields.find(f => f.name === fieldName);
	}

	function formatFieldValue(fieldName: string, value: unknown): string {
		if (value === null || value === undefined || value === '') return '-';

		const field = getFieldDef(fieldName);
		if (!field) return String(value);

		// Format rollup fields with decimal places
		if (field.type === 'rollup' && field.rollupResultType === 'numeric' && typeof value === 'number') {
			const decimalPlaces = field.rollupDecimalPlaces ?? 2;
			return value.toFixed(decimalPlaces);
		}

		switch (field.type) {
			case 'bool':
				return value ? 'Yes' : 'No';
			case 'date':
				return new Date(String(value)).toLocaleDateString();
			case 'datetime':
				return new Date(String(value)).toLocaleString();
			case 'link':
				if (field.linkEntity === 'User' && task) {
					const nameField = fieldName.replace(/Id$/, 'Name');
					const name = (task as unknown as Record<string, unknown>)[nameField];
					return name ? String(name) : '-';
				}
				return String(value);
			case 'text':
				return String(value);
			default:
				return String(value);
		}
	}

	function getLinkInfo(fieldName: string, value: unknown): { href: string; text: string } | null {
		if (!value) return null;

		// Special handling for polymorphic parent relationship
		if (fieldName === 'parentName' && task?.parentType && task?.parentId) {
			const entityPath = task.parentType.toLowerCase() + 's';
			return { href: `/${entityPath}/${task.parentId}`, text: String(value) };
		}

		const field = getFieldDef(fieldName);
		if (!field) return null;

		switch (field.type) {
			case 'email':
				return { href: `mailto:${value}`, text: String(value) };
			case 'phone':
				return { href: `tel:${value}`, text: String(value) };
			case 'url':
				return { href: String(value), text: String(value) };
			case 'link':
				if (field.linkEntity && task) {
					if (field.linkEntity === 'User') return null;
					const id = (task as unknown as Record<string, unknown>)[fieldName];
					if (!id) return null;
					const entityPath = field.linkEntity.toLowerCase() + 's';
					return { href: `/${entityPath}/${id}`, text: String(value) };
				}
				return null;
			default:
				return null;
		}
	}

	function handleBearingUpdate(fieldName: string, newValue: string) {
		if (task) {
			// Update local state optimistically
			if (isSystemField(fieldName)) {
				(task as unknown as Record<string, unknown>)[fieldName] = newValue;
			} else {
				if (!task.customFields) {
					task.customFields = {};
				}
				task.customFields[fieldName] = newValue;
			}
			task = { ...task }; // Trigger reactivity
		}
	}

	function getStatusColor(status: string): string {
		switch (status) {
			case 'Open': return 'bg-blue-100 text-blue-800';
			case 'In Progress': return 'bg-yellow-100 text-yellow-800';
			case 'Completed': return 'bg-green-100 text-green-800';
			case 'Deferred': return 'bg-gray-100 text-gray-800';
			case 'Cancelled': return 'bg-red-100 text-red-800';
			default: return 'bg-gray-100 text-gray-800';
		}
	}

	function getPriorityColor(priority: string): string {
		switch (priority) {
			case 'Urgent': return 'bg-red-100 text-red-800';
			case 'High': return 'bg-orange-100 text-orange-800';
			case 'Normal': return 'bg-gray-100 text-gray-800';
			case 'Low': return 'bg-gray-50 text-gray-600';
			default: return 'bg-gray-100 text-gray-800';
		}
	}

	function getTypeIcon(type: string): string {
		switch (type) {
			case 'Call': return 'M3 5a2 2 0 012-2h3.28a1 1 0 01.948.684l1.498 4.493a1 1 0 01-.502 1.21l-2.257 1.13a11.042 11.042 0 005.516 5.516l1.13-2.257a1 1 0 011.21-.502l4.493 1.498a1 1 0 01.684.949V19a2 2 0 01-2 2h-1C9.716 21 3 14.284 3 6V5z';
			case 'Email': return 'M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z';
			case 'Meeting': return 'M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z';
			case 'Todo': return 'M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4';
			default: return 'M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2';
		}
	}

	onMount(() => {
		loadTask();
	});
</script>

{#if loading}
	<div class="text-center py-12 text-gray-500">Loading...</div>
{:else if error}
	<div class="text-center py-12">
		<p class="text-red-500 mb-4">{error}</p>
		<a href="/tasks" class="text-blue-600 hover:underline">Back to Tasks</a>
	</div>
{:else if !task}
	<div class="text-center py-12">
		<p class="text-gray-500 mb-4">Task not found</p>
		<a href="/tasks" class="text-blue-600 hover:underline">Back to Tasks</a>
	</div>
{:else if task}
	<div class="space-y-6">
		<!-- Header -->
		<div class="flex justify-between items-start">
			<div>
				<div class="flex items-center space-x-2 text-sm text-gray-500 mb-2">
					<a href="/tasks" class="hover:text-gray-700">Tasks</a>
					<span>/</span>
					<span>{task.subject}</span>
				</div>
				<div class="flex items-center gap-3">
					<svg class="w-6 h-6 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={getTypeIcon(task.type)} />
					</svg>
					<h1 class="text-2xl font-bold text-gray-900">{task.subject}</h1>
				</div>
				<div class="flex items-center gap-3 mt-2">
					<span class="px-3 py-1 text-sm font-medium rounded-full {getStatusColor(task.status)}">
						{task.status}
					</span>
					<span class="px-3 py-1 text-sm font-medium rounded-full {getPriorityColor(task.priority)}">
						{task.priority}
					</span>
					<span class="text-sm text-gray-500">{task.type}</span>
				</div>
			</div>
			<div class="flex space-x-3">
				<a
					href="/tasks/{task.id}/edit"
					class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-600/90"
				>
					Edit
				</a>
				<button
					onclick={deleteTask}
					class="px-4 py-2 bg-red-600 text-white rounded-md hover:bg-red-700"
				>
					Delete
				</button>
			</div>
		</div>

		<!-- Duplicate alert banner -->
		<DetailPageAlertWrapper
			entityType="Task"
			recordId={task.id}
		/>

		<!-- Tabs (only show if entity has activities enabled) -->
		{#if entityDef?.hasActivities}
			<div class="border-b border-gray-200">
				<nav class="-mb-px flex space-x-8">
					<button
						onclick={() => activeTab = 'details'}
						class="whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm {activeTab === 'details' ? 'border-blue-500 text-blue-600' : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'}"
					>
						Details
					</button>
					<button
						onclick={() => activeTab = 'activities'}
						class="whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm {activeTab === 'activities' ? 'border-blue-500 text-blue-600' : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'}"
					>
						Activities
					</button>
				</nav>
			</div>
		{/if}

		{#if activeTab === 'details'}
		<!-- Bearings (Stage Progress Indicators) -->
		{#if bearings.length > 0}
			<div class="space-y-4">
				{#each bearings.toSorted((a, b) => a.displayOrder - b.displayOrder) as bearing (bearing.id)}
					{@const fieldName = bearing.sourcePicklist}
					{@const currentVal = isSystemField(fieldName)
						? (task as unknown as Record<string, unknown>)[fieldName]
						: task.customFields?.[fieldName]}
					<Bearing
						{bearing}
						currentValue={currentVal as string | null}
						recordId={task.id}
						entityType="Task"
						fieldName={fieldName}
						onUpdate={(newValue) => handleBearingUpdate(fieldName, newValue)}
					/>
				{/each}
			</div>
		{/if}

		<!-- Dynamic Sections from Layout -->
		{#each visibleSections() as section (section.id)}
			<SectionRenderer
				{section}
				{fields}
				record={recordData()}
				formatValue={formatFieldValue}
				renderLink={getLinkInfo}
			/>
		{/each}

		<!-- System Info -->
		<div class="crm-card overflow-hidden">
			<div class="px-6 py-4 border-b border-gray-200">
				<h2 class="text-lg font-medium text-gray-900">System Information</h2>
			</div>
			<div class="px-6 py-4">
				<dl class="grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-4">
					<div>
						<dt class="text-sm font-medium text-gray-500">Created</dt>
						<dd class="mt-1 text-sm text-gray-900">
							{formatDate(task.createdAt)}
							{#if task.createdByName}
								<span class="text-gray-500"> by {task.createdByName}</span>
							{/if}
						</dd>
					</div>
					<div>
						<dt class="text-sm font-medium text-gray-500">Last Modified</dt>
						<dd class="mt-1 text-sm text-gray-900">
							{formatDate(task.modifiedAt)}
							{#if task.modifiedByName}
								<span class="text-gray-500"> by {task.modifiedByName}</span>
							{/if}
						</dd>
					</div>
					<div>
						<dt class="text-sm font-medium text-gray-500">ID</dt>
						<dd class="mt-1 text-sm text-gray-500 font-mono">{task.id}</dd>
					</div>
				</dl>
			</div>
		</div>

		<!-- Related Lists -->
		{#if enabledRelatedLists.length > 0}
			{#each enabledRelatedLists as config (config.id)}
				<RelatedList
					{config}
					parentEntity="Task"
					parentId={task.id}
				/>
			{/each}
		{/if}
		{:else if activeTab === 'activities'}
		<!-- Activities Tab -->
		<ActivitiesStream
			parentEntity="Task"
			parentId={task.id}
			parentName={task.subject}
		/>
		{/if}
	</div>
{/if}
