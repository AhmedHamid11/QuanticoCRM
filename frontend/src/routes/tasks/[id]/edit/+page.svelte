<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { get, put } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import LookupField from '$lib/components/LookupField.svelte';
	import type { Task, TaskUpdateInput } from '$lib/types/task';

	let taskId = $derived($page.params.id);

	let task = $state<Task | null>(null);
	let formData = $state<TaskUpdateInput>({});
	let parentName = $state('');
	let loading = $state(true);
	let saving = $state(false);
	let error = $state<string | null>(null);

	async function loadTask() {
		try {
			loading = true;
			error = null;
			task = await get<Task>(`/tasks/${taskId}`);

			// Initialize form data from task
			formData = {
				subject: task.subject,
				description: task.description,
				status: task.status,
				priority: task.priority,
				type: task.type,
				dueDate: task.dueDate,
				parentId: task.parentId,
				parentType: task.parentType,
				parentName: task.parentName
			};
			parentName = task.parentName || '';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load task';
			toast.error(error);
		} finally {
			loading = false;
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
			await put<Task>(`/tasks/${taskId}`, formData);
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

	onMount(() => {
		loadTask();
	});
</script>

{#if loading}
	<div class="text-center py-12 text-gray-500">Loading...</div>
{:else if error}
	<div class="text-center py-12 text-red-500">{error}</div>
{:else if task}
	<div class="max-w-2xl mx-auto">
		<div class="mb-6">
			<nav class="text-sm text-gray-500 mb-2">
				<a href="/tasks" class="hover:text-gray-700">Tasks</a>
				<span class="mx-2">/</span>
				<a href="/tasks/{task.id}" class="hover:text-gray-700">{task.subject}</a>
				<span class="mx-2">/</span>
				<span class="text-gray-900">Edit</span>
			</nav>
			<h1 class="text-2xl font-bold text-gray-900">Edit Task</h1>
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
					class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary focus:border-primary"
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
						class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary focus:border-primary"
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
						class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary focus:border-primary"
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
						class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary focus:border-primary"
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
						class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary focus:border-primary"
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
							class="text-primary hover:underline flex-1"
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
							value={formData.parentType === 'Account' ? formData.parentId ?? null : null}
							valueName={formData.parentType === 'Account' ? parentName : ''}
							label="Account"
							onchange={handleParentChange('Account')}
						/>
						<LookupField
							entity="Contact"
							value={formData.parentType === 'Contact' ? formData.parentId ?? null : null}
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
					class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary focus:border-primary"
					placeholder="Enter description"
				></textarea>
			</div>

			<!-- Actions -->
			<div class="flex justify-end gap-3 pt-4 border-t">
				<a
					href="/tasks/{task.id}"
					class="px-4 py-2 border border-gray-300 rounded-md text-gray-700 hover:bg-gray-50"
				>
					Cancel
				</a>
				<button
					type="submit"
					disabled={saving}
					class="px-4 py-2 bg-primary text-black rounded-md hover:bg-primary/90 disabled:opacity-50"
				>
					{saving ? 'Saving...' : 'Save Changes'}
				</button>
			</div>
		</form>
	</div>
{/if}
