<script lang="ts">
	import { get } from '$lib/utils/api';
	import type { Task, TaskListResponse, TaskType } from '$lib/types/task';

	interface Props {
		entityName: string;
		recordId: string;
	}

	let { entityName, recordId }: Props = $props();

	let tasks = $state<Task[]>([]);
	let total = $state(0);
	let loading = $state(false);
	let page = $state(1);

	const PAGE_SIZE = 5;

	async function fetchTasks(resetPage = false) {
		if (!entityName || !recordId) return;
		loading = true;
		try {
			const currentPage = resetPage ? 1 : page;
			const result = await get<TaskListResponse>(
				`/tasks?parentType=${entityName}&parentId=${recordId}&pageSize=${PAGE_SIZE}&page=${currentPage}&sortBy=createdAt&sortDir=desc`
			);
			if (resetPage) {
				tasks = result.data;
				page = 1;
			} else {
				tasks = [...tasks, ...result.data];
			}
			total = result.total;
		} catch {
			// Silently fail — empty state will show
		} finally {
			loading = false;
		}
	}

	async function loadMore() {
		if (loading) return;
		page += 1;
		loading = true;
		try {
			const result = await get<TaskListResponse>(
				`/tasks?parentType=${entityName}&parentId=${recordId}&pageSize=${PAGE_SIZE}&page=${page}&sortBy=createdAt&sortDir=desc`
			);
			tasks = [...tasks, ...result.data];
			total = result.total;
		} catch {
			page -= 1; // revert on error
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		// Re-fetch whenever recordId or entityName changes
		const _r = recordId;
		const _e = entityName;
		tasks = [];
		total = 0;
		page = 1;
		fetchTasks(true);
	});

	function relativeTime(dateStr: string): string {
		const diff = Date.now() - new Date(dateStr).getTime();
		const minutes = Math.floor(diff / 60000);
		if (minutes < 1) return 'Just now';
		if (minutes < 60) return `${minutes}m ago`;
		const hours = Math.floor(minutes / 60);
		if (hours < 24) return `${hours}h ago`;
		const days = Math.floor(hours / 24);
		if (days === 1) return 'Yesterday';
		if (days < 30) return `${days}d ago`;
		return new Date(dateStr).toLocaleDateString();
	}

	interface IconConfig {
		bgClass: string;
		textClass: string;
		path: string;
	}

	function getIconConfig(type: TaskType): IconConfig {
		switch (type) {
			case 'Todo':
				return {
					bgClass: 'bg-blue-100',
					textClass: 'text-blue-600',
					path: 'M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z'
				};
			case 'Call':
				return {
					bgClass: 'bg-green-100',
					textClass: 'text-green-600',
					path: 'M2.25 6.75c0 8.284 6.716 15 15 15h2.25a2.25 2.25 0 002.25-2.25v-1.372c0-.516-.351-.966-.852-1.091l-4.423-1.106c-.44-.11-.902.055-1.173.417l-.97 1.293c-.282.376-.769.542-1.21.38a12.035 12.035 0 01-7.143-7.143c-.162-.441.004-.928.38-1.21l1.293-.97c.363-.271.527-.734.417-1.173L6.963 3.102a1.125 1.125 0 00-1.091-.852H4.5A2.25 2.25 0 002.25 4.5v2.25z'
				};
			case 'Meeting':
				return {
					bgClass: 'bg-purple-100',
					textClass: 'text-purple-600',
					path: 'M6.75 3v2.25M17.25 3v2.25M3 18.75V7.5a2.25 2.25 0 012.25-2.25h13.5A2.25 2.25 0 0121 7.5v11.25m-18 0A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75m-18 0v-7.5A2.25 2.25 0 015.25 9h13.5A2.25 2.25 0 0121 11.25v7.5'
				};
			case 'Email':
			default:
				return {
					bgClass: 'bg-gray-100',
					textClass: 'text-gray-600',
					path: 'M21.75 6.75v10.5a2.25 2.25 0 01-2.25 2.25h-15a2.25 2.25 0 01-2.25-2.25V6.75m19.5 0A2.25 2.25 0 0019.5 4.5h-15a2.25 2.25 0 00-2.25 2.25m19.5 0v.243a2.25 2.25 0 01-1.07 1.916l-7.5 4.615a2.25 2.25 0 01-2.36 0L3.32 8.91a2.25 2.25 0 01-1.07-1.916V6.75'
				};
		}
	}

	function getStatusBadgeClass(status: string): string {
		switch (status) {
			case 'Completed':
				return 'bg-green-100 text-green-700';
			case 'In Progress':
				return 'bg-blue-100 text-blue-700';
			case 'Deferred':
				return 'bg-yellow-100 text-yellow-700';
			case 'Cancelled':
				return 'bg-gray-100 text-gray-500';
			case 'Open':
			default:
				return 'bg-gray-100 text-gray-600';
		}
	}
</script>

<div class="px-4 py-3">
	{#if loading && tasks.length === 0}
		<div class="text-center py-8 text-gray-400 text-sm">Loading activities...</div>
	{:else if tasks.length === 0}
		<div class="text-center py-8">
			<svg
				class="mx-auto h-10 w-10 text-gray-300"
				fill="none"
				viewBox="0 0 24 24"
				stroke="currentColor"
				stroke-width="1.5"
			>
				<path
					stroke-linecap="round"
					stroke-linejoin="round"
					d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z"
				/>
			</svg>
			<p class="mt-2 text-sm text-gray-500">No activities yet</p>
			<p class="text-xs text-gray-400">Tasks, calls, and meetings will appear here</p>
		</div>
	{:else}
		<div class="space-y-0">
			{#each tasks as task (task.id)}
				{@const iconConfig = getIconConfig(task.type)}
				<div class="flex items-start gap-3 py-2.5 border-b border-gray-100 last:border-0">
					<!-- Color-coded icon circle -->
					<div
						class="flex-shrink-0 flex items-center justify-center w-8 h-8 rounded-full {iconConfig.bgClass} {iconConfig.textClass}"
					>
						<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
							<path stroke-linecap="round" stroke-linejoin="round" d={iconConfig.path} />
						</svg>
					</div>

					<!-- Subject + timestamp -->
					<div class="flex-1 min-w-0">
						<p class="text-sm text-gray-900 truncate">{task.subject}</p>
						<p class="text-xs text-gray-400">{relativeTime(task.createdAt)}</p>
					</div>

					<!-- Status badge -->
					<span
						class="flex-shrink-0 inline-flex items-center px-2 py-0.5 rounded text-xs font-medium {getStatusBadgeClass(task.status)}"
					>
						{task.status}
					</span>
				</div>
			{/each}
		</div>

		{#if tasks.length < total}
			<button
				onclick={loadMore}
				disabled={loading}
				class="w-full mt-2 py-2 text-sm text-blue-600 hover:text-blue-800 disabled:opacity-50 disabled:cursor-not-allowed"
			>
				{loading ? 'Loading...' : 'Load More'}
			</button>
		{/if}
	{/if}
</div>
