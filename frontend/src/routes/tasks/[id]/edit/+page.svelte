<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { get, put } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import { FormSkeleton, ErrorDisplay } from '$lib/components/ui';
	import { fieldNameToKey } from '$lib/utils/fieldMapping';
	import LookupField from '$lib/components/LookupField.svelte';
	import EditSectionRenderer from '$lib/components/EditSectionRenderer.svelte';
	import type { Task, TaskUpdateInput } from '$lib/types/task';
	import type { FieldDef } from '$lib/types/admin';
	import type { LayoutDataV2 } from '$lib/types/layout';
	import { parseLayoutData, getVisibleSections } from '$lib/types/layout';

	interface LookupRecord {
		id: string;
		name: string;
	}

	// System fields that exist as columns in the tasks table (in camelCase)
	const SYSTEM_FIELDS = new Set([
		'subject', 'description', 'status', 'priority', 'type', 'dueDate',
		'parentId', 'parentType', 'parentName', 'assignedUserId',
		'createdAt', 'modifiedAt', 'createdById', 'modifiedById', 'deleted'
	]);

	// Fields that are handled specially (not in layout renderer)
	const SPECIAL_FIELDS = new Set(['parentId', 'parentType', 'parentName']);

	let taskId = $derived($page.params.id);

	let task = $state<Task | null>(null);
	let fields = $state<FieldDef[]>([]);
	let layout = $state<LayoutDataV2 | null>(null);
	let formData = $state<Record<string, unknown>>({});
	let lookupNames = $state<Record<string, string>>({});
	let multiLookupValues = $state<Record<string, LookupRecord[]>>({});
	let loading = $state(true);
	let saving = $state(false);
	let error = $state<string | null>(null);

	// Parent (Related To) state
	let parentId = $state<string | null>(null);
	let parentType = $state<string | null>(null);
	let parentName = $state('');

	// Get visible sections based on form data
	let visibleSections = $derived(() => layout ? getVisibleSections(layout, formData) : []);

	function isSystemField(fieldName: string): boolean {
		const key = fieldNameToKey(fieldName);
		return SYSTEM_FIELDS.has(key);
	}

	// Filter out special fields from sections
	function filterSection(section: any) {
		return {
			...section,
			fields: section.fields.filter((f: any) => !SPECIAL_FIELDS.has(f.name))
		};
	}

	async function loadData() {
		try {
			loading = true;
			error = null;

			const [taskData, fieldsData] = await Promise.all([
				get<Task & { customFields?: Record<string, unknown> }>(`/tasks/${taskId}`),
				get<FieldDef[]>('/entities/Task/fields').catch(() => [] as FieldDef[])
			]);

			task = taskData;
			fields = fieldsData;

			// Load layout
			try {
				const layoutResponse = await get<{ layoutData: string }>('/entities/Task/layouts/detail');
				layout = parseLayoutData(layoutResponse.layoutData, fieldsData.map(f => f.name));
			} catch {
				// Default to all fields
				layout = parseLayoutData('[]', fieldsData.map(f => f.name));
			}

			// Initialize form data from task
			const data: Record<string, unknown> = {};
			for (const field of fieldsData) {
				if (SPECIAL_FIELDS.has(field.name)) continue; // Skip special fields

				const key = fieldNameToKey(field.name);
				if (isSystemField(field.name)) {
					data[field.name] = (taskData as Record<string, unknown>)[key] ?? '';
				} else {
					data[field.name] = taskData.customFields?.[field.name] ?? '';
				}

				// For link fields, load display name
				if (field.type === 'link') {
					const nameKey = `${key}Name`;
					const nameVal = isSystemField(field.name)
						? (taskData as Record<string, unknown>)[nameKey]
						: taskData.customFields?.[`${field.name}Name`];
					if (nameVal) {
						lookupNames[field.name] = String(nameVal);
					}
				}

				// For linkMultiple fields, load values
				if (field.type === 'linkMultiple') {
					const idsVal = taskData.customFields?.[`${field.name}Ids`];
					const namesVal = taskData.customFields?.[`${field.name}Names`];

					if (idsVal && namesVal && idsVal !== '[]') {
						try {
							const ids = typeof idsVal === 'string' ? JSON.parse(idsVal) : idsVal;
							const names = typeof namesVal === 'string' ? JSON.parse(namesVal) : namesVal;

							if (Array.isArray(ids) && Array.isArray(names)) {
								multiLookupValues[field.name] = ids.map((id: string, i: number) => ({
									id,
									name: names[i] || ''
								}));
							}
						} catch {
							// Not valid JSON, ignore
						}
					}
				}
			}
			formData = data;

			// Initialize parent (Related To) fields
			parentId = taskData.parentId ?? null;
			parentType = taskData.parentType ?? null;
			parentName = taskData.parentName ?? '';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load task';
			toast.error(error);
		} finally {
			loading = false;
		}
	}

	async function handleSubmit(e: Event) {
		e.preventDefault();

		const subject = formData.subject;
		if (!subject || String(subject).trim() === '') {
			toast.error('Subject is required');
			return;
		}

		try {
			saving = true;

			// Separate system fields and custom fields
			const payload: Record<string, unknown> = {};
			const customFields: Record<string, unknown> = {};

			for (const [fieldName, value] of Object.entries(formData)) {
				const key = fieldNameToKey(fieldName);
				if (isSystemField(fieldName)) {
					payload[key] = value;
				} else {
					customFields[fieldName] = value;
				}
			}

			// Add parent (Related To) fields
			payload.parentId = parentId;
			payload.parentType = parentType;
			payload.parentName = parentName;

			// Add custom fields to payload
			if (Object.keys(customFields).length > 0) {
				payload.customFields = customFields;
			}

			await put<Task>(`/tasks/${taskId}`, payload);
			toast.success('Task updated');
			goto(`/tasks/${taskId}`);
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to update task';
			toast.error(message);
		} finally {
			saving = false;
		}
	}

	function handleParentChange(entity: string) {
		return (id: string | null, name: string) => {
			parentId = id;
			parentType = id ? entity : null;
			parentName = name;
		};
	}

	function clearParent() {
		parentId = null;
		parentType = null;
		parentName = '';
	}

	onMount(() => {
		loadData();
	});
</script>

<div class="max-w-4xl mx-auto">
	<div class="flex items-center justify-between mb-6">
		<div>
			<nav class="text-sm text-gray-500 mb-2">
				<a href="/tasks" class="hover:text-gray-700">Tasks</a>
				<span class="mx-2">/</span>
				{#if task}
					<a href="/tasks/{task.id}" class="hover:text-gray-700">{task.subject}</a>
					<span class="mx-2">/</span>
				{/if}
				<span class="text-gray-900">Edit</span>
			</nav>
			<h1 class="text-2xl font-bold text-gray-900">Edit Task</h1>
		</div>
		<a href="/tasks/{taskId}" class="text-gray-600 hover:text-gray-900 text-sm">
			← Back to Task
		</a>
	</div>

	{#if loading}
		<FormSkeleton fields={6} />
	{:else if error}
		<ErrorDisplay message={error} onRetry={loadData} />
	{:else if task}
		<form onsubmit={handleSubmit} class="space-y-6">
			<!-- Render layout sections -->
			{#each visibleSections() as section (section.id)}
				{@const filteredSection = filterSection(section)}
				{#if filteredSection.fields.length > 0}
					<EditSectionRenderer
						section={filteredSection}
						{fields}
						bind:formData
						{lookupNames}
						{multiLookupValues}
						getFieldError={() => undefined}
						onLookupChange={(fieldName, id, name) => {
							formData[`${fieldName}Id`] = id;
							formData[`${fieldName}Name`] = name;
							lookupNames[fieldName] = name;
						}}
						onMultiLookupChange={(fieldName, values) => {
							multiLookupValues[fieldName] = values;
							formData[`${fieldName}Ids`] = JSON.stringify(values.map(v => v.id));
							formData[`${fieldName}Names`] = JSON.stringify(values.map(v => v.name));
						}}
					/>
				{/if}
			{/each}

			<!-- Related To (Special Field - not in layout system) -->
			<div class="bg-white shadow rounded-lg overflow-hidden">
				<div class="px-6 py-4 bg-gray-50 border-b border-gray-200">
					<h2 class="text-lg font-medium text-gray-900">Related To</h2>
				</div>
				<div class="p-6">
					{#if parentId && parentType}
						<!-- Show selected parent -->
						<div class="flex items-center gap-2 px-3 py-2 border border-gray-300 rounded-md bg-gray-50">
							<span class="text-xs text-gray-500 uppercase">{parentType}:</span>
							<a
								href="/{parentType.toLowerCase()}s/{parentId}"
								class="text-blue-600 hover:underline flex-1"
							>
								{parentName || 'Loading...'}
							</a>
							<button
								type="button"
								onclick={clearParent}
								class="text-gray-400 hover:text-gray-600"
								aria-label="Clear selection"
							>
								<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
								</svg>
							</button>
						</div>
					{:else}
						<!-- Show lookup options -->
						<div class="grid grid-cols-2 gap-4">
							<LookupField
								entity="Account"
								value={parentType === 'Account' ? parentId : null}
								valueName={parentType === 'Account' ? parentName : ''}
								label="Account"
								onchange={handleParentChange('Account')}
							/>
							<LookupField
								entity="Contact"
								value={parentType === 'Contact' ? parentId : null}
								valueName={parentType === 'Contact' ? parentName : ''}
								label="Contact"
								onchange={handleParentChange('Contact')}
							/>
						</div>
					{/if}
				</div>
			</div>

			<!-- Actions -->
			<div class="bg-white shadow rounded-lg p-6">
				<div class="flex justify-end gap-3">
					<a
						href="/tasks/{task.id}"
						class="px-4 py-2 border border-gray-300 rounded-md text-gray-700 hover:bg-gray-50"
					>
						Cancel
					</a>
					<button
						type="submit"
						disabled={saving}
						class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-600/90 disabled:opacity-50"
					>
						{saving ? 'Saving...' : 'Save Changes'}
					</button>
				</div>
			</div>
		</form>
	{/if}
</div>
