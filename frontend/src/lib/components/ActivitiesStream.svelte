<script lang="ts">
	import { goto } from '$app/navigation';
	import { get } from '$lib/utils/api';
	import type { Task, TaskListResponse } from '$lib/types/task';

	interface Props {
		parentEntity: string;
		parentId: string;
		parentName: string;
	}

	let { parentEntity, parentId, parentName }: Props = $props();

	let tasks = $state<Task[]>([]);
	let total = $state(0);
	let loading = $state(true);
	let loadingMore = $state(false);
	let error = $state<string | null>(null);
	let hasMore = $state(false);

	const PAGE_SIZE = 20;

	// Group tasks by month
	let groupedTasks = $derived(() => {
		const groups: { monthLabel: string; tasks: Task[] }[] = [];
		const monthMap = new Map<string, Task[]>();

		for (const task of tasks) {
			const date = new Date(task.createdAt);
			const monthKey = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}`;
			const monthLabel = formatMonthLabel(date);

			if (!monthMap.has(monthKey)) {
				monthMap.set(monthKey, []);
			}
			monthMap.get(monthKey)!.push(task);
		}

		// Convert to array sorted by month (newest first)
		const sortedKeys = Array.from(monthMap.keys()).sort((a, b) => b.localeCompare(a));
		for (const key of sortedKeys) {
			const [year, month] = key.split('-');
			const date = new Date(parseInt(year), parseInt(month) - 1, 1);
			groups.push({
				monthLabel: formatMonthLabel(date),
				tasks: monthMap.get(key)!
			});
		}

		return groups;
	});

	function formatMonthLabel(date: Date): string {
		const month = date.toLocaleString('default', { month: 'long' });
		const year = date.getFullYear();
		return `${month} \u2022 ${year}`;
	}

	function formatDueDate(dueDate: string | null): string {
		if (!dueDate) return '';
		try {
			const date = new Date(dueDate);
			return date.toLocaleDateString('en-US', {
				month: 'short',
				day: 'numeric',
				year: date.getFullYear() !== new Date().getFullYear() ? 'numeric' : undefined
			});
		} catch {
			return '';
		}
	}

	async function loadTasks(append = false) {
		if (append) {
			loadingMore = true;
		} else {
			loading = true;
			tasks = [];
		}
		error = null;

		try {
			const offset = append ? tasks.length : 0;
			// Use generic tasks endpoint with parentType/parentId filter
			const endpoint = `/tasks?parentType=${encodeURIComponent(parentEntity)}&parentId=${encodeURIComponent(parentId)}&pageSize=${PAGE_SIZE}&page=${Math.floor(offset / PAGE_SIZE) + 1}`;
			const result = await get<TaskListResponse>(endpoint);

			if (append) {
				tasks = [...tasks, ...result.data];
			} else {
				tasks = result.data;
			}
			total = result.total;
			hasMore = tasks.length < total;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load tasks';
		} finally {
			loading = false;
			loadingMore = false;
		}
	}

	function handleCreateTask() {
		// Include tab=activities so user returns to the activities tab
		const returnUrl = encodeURIComponent(`${window.location.pathname}?tab=activities`);
		goto(`/tasks/new?parentType=${parentEntity}&parentId=${parentId}&returnUrl=${returnUrl}`);
	}

	function handleRefresh() {
		loadTasks(false);
	}

	function handleLoadMore() {
		loadTasks(true);
	}

	function navigateToTask(taskId: string) {
		goto(`/tasks/${taskId}`);
	}

	// Load tasks on mount
	$effect(() => {
		if (parentId) {
			loadTasks(false);
		}
	});
</script>

<div class="bg-white shadow rounded-lg overflow-hidden">
	<!-- Header -->
	<div class="px-6 py-4 border-b border-gray-200 flex items-center justify-between">
		<div class="flex items-center gap-4">
			<h3 class="text-lg font-medium text-gray-900">
				Activities
				{#if !loading && total > 0}
					<span class="text-sm font-normal text-gray-500">({total})</span>
				{/if}
			</h3>
			<button
				onclick={handleRefresh}
				disabled={loading}
				class="text-sm text-blue-600 hover:text-blue-800 hover:underline disabled:opacity-50"
			>
				Refresh
			</button>
		</div>
		<button
			onclick={handleCreateTask}
			class="inline-flex items-center px-3 py-1.5 text-sm bg-blue-600 text-white rounded-md hover:bg-blue-600/90"
		>
			+ Task
		</button>
	</div>

	<!-- Content -->
	<div class="px-6 py-4">
		{#if loading}
			<div class="py-8 text-center text-gray-500">Loading...</div>
		{:else if error}
			<div class="py-8 text-center text-red-500">{error}</div>
		{:else if tasks.length === 0}
			<div class="py-8 text-center text-gray-500">No tasks to show.</div>
		{:else}
			<!-- Grouped tasks -->
			<div class="space-y-6">
				{#each groupedTasks() as group}
					<!-- Month header -->
					<div class="border-b border-gray-100 pb-2">
						<h4 class="text-sm font-medium text-gray-500">{group.monthLabel}</h4>
					</div>

					<!-- Tasks in this month -->
					<div class="space-y-3">
						{#each group.tasks as task}
							<div class="flex items-start justify-between py-2 hover:bg-gray-50 -mx-2 px-2 rounded cursor-pointer" onclick={() => navigateToTask(task.id)}>
								<div class="flex-1 min-w-0">
									<button
										onclick={(e) => { e.stopPropagation(); navigateToTask(task.id); }}
										class="text-sm font-medium text-blue-600 hover:text-blue-800 hover:underline truncate block text-left"
									>
										{task.subject}
									</button>
									<p class="text-xs text-gray-500 mt-0.5">
										{#if task.assignedUserId}
											<span class="text-gray-600">Owner</span> had a task
										{:else}
											Task created
										{/if}
									</p>
								</div>
								{#if task.dueDate}
									<span class="text-xs text-gray-500 ml-4 whitespace-nowrap">
										{formatDueDate(task.dueDate)}
									</span>
								{/if}
							</div>
						{/each}
					</div>
				{/each}
			</div>

			<!-- Load More -->
			<div class="mt-6 pt-4 border-t border-gray-100 text-center">
				{#if hasMore}
					<button
						onclick={handleLoadMore}
						disabled={loadingMore}
						class="text-sm text-blue-600 hover:text-blue-800 hover:underline disabled:opacity-50"
					>
						{loadingMore ? 'Loading...' : 'Load More'}
					</button>
				{:else if tasks.length > 0}
					<span class="text-sm text-gray-500">No more tasks to load.</span>
				{/if}
			</div>
		{/if}
	</div>
</div>
