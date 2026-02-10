<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { post, get } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import LookupField from '$lib/components/LookupField.svelte';
	import type { Task, TaskCreateInput, TASK_STATUSES, TASK_PRIORITIES, TASK_TYPES } from '$lib/types/task';

	// Pre-fill from query params (when creating from related list)
	let parentType = $derived($page.url.searchParams.get('parentType'));
	let parentId = $derived($page.url.searchParams.get('parentId'));
	let returnUrl = $derived($page.url.searchParams.get('returnUrl'));

	// Get today's date in YYYY-MM-DD format for the date input
	const today = new Date().toISOString().split('T')[0];

	let formData = $state<TaskCreateInput>({
		subject: '',
		description: '',
		status: 'Open',
		priority: 'Normal',
		type: 'Todo',
		dueDate: today,
		parentId: null,
		parentType: null,
		parentName: ''
	});

	let parentName = $state('');
	let saving = $state(false);
	let loadingParent = $state(false);

	// Load parent name if parentId/parentType provided
	$effect(() => {
		if (parentType && parentId && !parentName) {
			loadParentName();
		}
	});

	async function loadParentName() {
		if (!parentType || !parentId) return;

		loadingParent = true;
		try {
			const endpoint = `/${parentType.toLowerCase()}s/${parentId}`;
			const data = await get<{ name?: string; firstName?: string; lastName?: string }>(endpoint);

			// Handle different entity name patterns
			if (data.name) {
				parentName = data.name;
			} else if (data.firstName || data.lastName) {
				parentName = [data.firstName, data.lastName].filter(Boolean).join(' ');
			}

			formData.parentId = parentId;
			formData.parentType = parentType;
			formData.parentName = parentName;
		} catch (e) {
			console.error('Failed to load parent name:', e);
		} finally {
			loadingParent = false;
		}
	}

	async function handleSubmit(e: Event) {
		e.preventDefault();

		if (!formData.subject?.trim()) {
			toast.error('Subject is required');
			return;
		}

		try {
			saving = true;
			const result = await post<Task>('/tasks', formData);
			toast.success('Task created');
			goto(returnUrl || `/tasks/${result.id}`);
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to create task';
			toast.error(message);
		} finally {
			saving = false;
		}
	}

	function handleParentChange(entity: string) {
		return (id: string | null, name: string) => {
			formData.parentId = id;
			formData.parentType = id ? entity : null;
			formData.parentName = name;
			parentName = name;
		};
	}

	function clearParent() {
		formData.parentId = null;
		formData.parentType = null;
		formData.parentName = '';
		parentName = '';
	}
</script>

<div class="max-w-2xl mx-auto">
	<div class="mb-6">
		<nav class="text-sm text-gray-500 mb-2">
			<a href="/tasks" class="hover:text-gray-700">Tasks</a>
			<span class="mx-2">/</span>
			<span class="text-gray-900">New Task</span>
		</nav>
		<h1 class="text-2xl font-bold text-gray-900">Create Task</h1>
	</div>

	<form onsubmit={handleSubmit} class="bg-white shadow rounded-lg p-6 space-y-6">
		<!-- Subject -->
		<div>
			<label for="subject" class="block text-sm font-medium text-gray-700 mb-1">
				Subject <span class="text-red-500">*</span>
			</label>
			<input
				type="text"
				id="subject"
				bind:value={formData.subject}
				required
				class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
				placeholder="Enter task subject"
			/>
		</div>

		<!-- Type and Status Row -->
		<div class="grid grid-cols-2 gap-4">
			<div>
				<label for="type" class="block text-sm font-medium text-gray-700 mb-1">Type</label>
				<select
					id="type"
					bind:value={formData.type}
					class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
				>
					<option value="Call">Call</option>
					<option value="Email">Email</option>
					<option value="Meeting">Meeting</option>
					<option value="Todo">Todo</option>
				</select>
			</div>
			<div>
				<label for="status" class="block text-sm font-medium text-gray-700 mb-1">Status</label>
				<select
					id="status"
					bind:value={formData.status}
					class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
				>
					<option value="Open">Open</option>
					<option value="In Progress">In Progress</option>
					<option value="Completed">Completed</option>
					<option value="Deferred">Deferred</option>
					<option value="Cancelled">Cancelled</option>
				</select>
			</div>
		</div>

		<!-- Priority and Due Date Row -->
		<div class="grid grid-cols-2 gap-4">
			<div>
				<label for="priority" class="block text-sm font-medium text-gray-700 mb-1">Priority</label>
				<select
					id="priority"
					bind:value={formData.priority}
					class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
				>
					<option value="Low">Low</option>
					<option value="Normal">Normal</option>
					<option value="High">High</option>
					<option value="Urgent">Urgent</option>
				</select>
			</div>
			<div>
				<label for="dueDate" class="block text-sm font-medium text-gray-700 mb-1">Due Date</label>
				<input
					type="date"
					id="dueDate"
					bind:value={formData.dueDate}
					class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
				/>
			</div>
		</div>

		<!-- Related To -->
		<div>
			<label class="block text-sm font-medium text-gray-700 mb-1">Related To</label>
			{#if formData.parentId && formData.parentType}
				<!-- Show selected parent -->
				<div class="flex items-center gap-2 px-3 py-2 border border-gray-300 rounded-md bg-gray-50">
					<span class="text-xs text-gray-500 uppercase">{formData.parentType}:</span>
					<a
						href="/{formData.parentType.toLowerCase()}s/{formData.parentId}"
						class="text-blue-600 hover:underline flex-1"
					>
						{parentName || formData.parentName || 'Loading...'}
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
						value={formData.parentType === 'Account' ? (formData.parentId ?? null) : null}
						valueName={formData.parentType === 'Account' ? parentName : ''}
						label="Account"
						onchange={handleParentChange('Account')}
					/>
					<LookupField
						entity="Contact"
						value={formData.parentType === 'Contact' ? (formData.parentId ?? null) : null}
						valueName={formData.parentType === 'Contact' ? parentName : ''}
						label="Contact"
						onchange={handleParentChange('Contact')}
					/>
				</div>
			{/if}
		</div>

		<!-- Description -->
		<div>
			<label for="description" class="block text-sm font-medium text-gray-700 mb-1">Description</label>
			<textarea
				id="description"
				bind:value={formData.description}
				rows="4"
				class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
				placeholder="Enter description"
			></textarea>
		</div>

		<!-- Actions -->
		<div class="flex justify-end gap-3 pt-4 border-t">
			<a
				href={returnUrl || '/tasks'}
				class="px-4 py-2 border border-gray-300 rounded-md text-gray-700 hover:bg-gray-50"
			>
				Cancel
			</a>
			<button
				type="submit"
				disabled={saving}
				class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-600/90 disabled:opacity-50"
			>
				{saving ? 'Creating...' : 'Create Task'}
			</button>
		</div>
	</form>
</div>
