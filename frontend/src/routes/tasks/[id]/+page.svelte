<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { get, del } from '$lib/utils/api';
	import { addToast } from '$lib/stores/toast.svelte';
	import type { Task } from '$lib/types/task';

	let task = $state<Task | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	let taskId = $derived($page.params.id);

	async function loadTask() {
		try {
			loading = true;
			error = null;
			task = await get<Task>(`/tasks/${taskId}`);
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

	function formatDueDate(dateStr: string | null): string {
		if (!dateStr) return '-';
		return new Date(dateStr).toLocaleDateString();
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
	<div class="text-center py-12 text-red-500">{error}</div>
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

		<!-- Task Details -->
		<div class="bg-white shadow rounded-lg overflow-hidden">
			<div class="px-6 py-4 border-b border-gray-200">
				<h2 class="text-lg font-medium text-gray-900">Task Information</h2>
			</div>
			<div class="px-6 py-4">
				<dl class="grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-4">
					<div>
						<dt class="text-sm font-medium text-gray-500">Subject</dt>
						<dd class="mt-1 text-sm text-gray-900">{task.subject}</dd>
					</div>
					<div>
						<dt class="text-sm font-medium text-gray-500">Type</dt>
						<dd class="mt-1 text-sm text-gray-900">{task.type}</dd>
					</div>
					<div>
						<dt class="text-sm font-medium text-gray-500">Status</dt>
						<dd class="mt-1">
							<span class="px-2 py-1 text-xs font-medium rounded-full {getStatusColor(task.status)}">
								{task.status}
							</span>
						</dd>
					</div>
					<div>
						<dt class="text-sm font-medium text-gray-500">Priority</dt>
						<dd class="mt-1">
							<span class="px-2 py-1 text-xs font-medium rounded-full {getPriorityColor(task.priority)}">
								{task.priority}
							</span>
						</dd>
					</div>
					<div>
						<dt class="text-sm font-medium text-gray-500">Due Date</dt>
						<dd class="mt-1 text-sm text-gray-900">{formatDueDate(task.dueDate)}</dd>
					</div>
					<div>
						<dt class="text-sm font-medium text-gray-500">Related To</dt>
						<dd class="mt-1 text-sm text-gray-900">
							{#if task.parentType && task.parentId}
								<a
									href="/{task.parentType.toLowerCase()}s/{task.parentId}"
									class="text-blue-600 hover:underline"
								>
									{task.parentName || `${task.parentType} ${task.parentId}`}
								</a>
							{:else}
								-
							{/if}
						</dd>
					</div>
					{#if task.description}
						<div class="md:col-span-2">
							<dt class="text-sm font-medium text-gray-500">Description</dt>
							<dd class="mt-1 text-sm text-gray-900 whitespace-pre-wrap">{task.description}</dd>
						</div>
					{/if}
				</dl>
			</div>
		</div>

		<!-- System Info -->
		<div class="bg-white shadow rounded-lg overflow-hidden">
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
	</div>
{/if}
