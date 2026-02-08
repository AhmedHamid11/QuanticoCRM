<script lang="ts">
	import { onMount } from 'svelte';
	import { get, post, put, del } from '$lib/utils/api';
	import { addToast } from '$lib/stores/toast.svelte';
	import { PUBLIC_API_URL } from '$env/static/public';

	const API_BASE = PUBLIC_API_URL || '/api/v1';

	// Type definitions (inline since data-quality.ts may not exist yet)
	interface ScanSchedule {
		id: string;
		orgId: string;
		entityType: string;
		frequency: 'daily' | 'weekly' | 'monthly';
		dayOfWeek?: number; // 0=Sun, 1=Mon, etc.
		dayOfMonth?: number; // 1-28
		hour: number; // 0-23
		minute: number;
		isEnabled: boolean;
		lastRunAt?: string;
		nextRunAt?: string;
		createdAt: string;
		updatedAt: string;
	}

	interface ScanJob {
		id: string;
		orgId: string;
		entityType: string;
		status: 'pending' | 'running' | 'completed' | 'failed';
		totalRecords: number;
		processedRecords: number;
		duplicatesFound: number;
		errorMessage?: string;
		startedAt: string;
		completedAt?: string;
		createdAt: string;
		updatedAt: string;
	}

	interface ProgressEvent {
		jobId: string;
		entityType: string;
		status: 'running' | 'completed' | 'failed';
		percentage: number;
		processedRecords: number;
		totalRecords: number;
		duplicatesFound?: number;
		errorMessage?: string;
	}

	interface ScheduleEditForm {
		frequency: 'daily' | 'weekly' | 'monthly';
		dayOfWeek: number;
		dayOfMonth: number;
		hour: number;
		minute: number;
		isEnabled: boolean;
	}

	interface EntityDef {
		id: string;
		name: string;
		label: string;
	}

	// State
	let schedules = $state<ScanSchedule[]>([]);
	let jobs = $state<ScanJob[]>([]);
	let entities = $state<EntityDef[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// Edit state
	let editingScheduleEntity = $state<string | null>(null);
	let editForm = $state<ScheduleEditForm>({
		frequency: 'daily',
		dayOfWeek: 1, // Monday
		dayOfMonth: 1,
		hour: 2,
		minute: 0,
		isEnabled: true
	});

	// New schedule state
	let isAddingNew = $state(false);
	let newEntityType = $state('');

	// Run Now state
	let showRunModal = $state(false);
	let runEntityType = $state('');
	let isRunning = $state(false);

	// Progress tracking
	let runningJobs = $state<Map<string, ProgressEvent>>(new Map());

	// Job history pagination
	let jobPage = $state(1);
	let jobPageSize = $state(10);
	let jobTotal = $state(0);

	// Available entities for new schedule (entities without schedules)
	let availableEntities = $derived.by(() => {
		const scheduledTypes = new Set(schedules.map(s => s.entityType));
		return entities.filter(e => !scheduledTypes.has(e.name));
	});

	// Day names for weekly schedule
	const dayNames = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];

	// Combined schedule + job status for table display
	let scheduleWithStatus = $derived.by(() => {
		return schedules.map(schedule => {
			// Find latest job for this entity
			const latestJob = jobs
				.filter(j => j.entityType === schedule.entityType)
				.sort((a, b) => new Date(b.startedAt).getTime() - new Date(a.startedAt).getTime())[0];

			// Check if job is currently running
			const progress = Array.from(runningJobs.values()).find(p => p.entityType === schedule.entityType);
			const isRunning = progress?.status === 'running';

			return {
				schedule,
				latestJob,
				isRunning,
				progress
			};
		}).sort((a, b) => {
			// Sort by next run time
			if (!a.schedule.nextRunAt) return 1;
			if (!b.schedule.nextRunAt) return -1;
			return new Date(a.schedule.nextRunAt).getTime() - new Date(b.schedule.nextRunAt).getTime();
		});
	});

	// Recent jobs with pagination
	let paginatedJobs = $derived.by(() => {
		const sorted = [...jobs].sort((a, b) =>
			new Date(b.startedAt).getTime() - new Date(a.startedAt).getTime()
		);
		return sorted.slice((jobPage - 1) * jobPageSize, jobPage * jobPageSize);
	});

	let totalJobPages = $derived(Math.ceil(jobTotal / jobPageSize));

	// Helper functions
	function formatSchedule(schedule: ScanSchedule): string {
		const time = `${schedule.hour.toString().padStart(2, '0')}:${schedule.minute.toString().padStart(2, '0')}`;

		if (schedule.frequency === 'daily') {
			return `Daily at ${time}`;
		} else if (schedule.frequency === 'weekly' && schedule.dayOfWeek !== undefined) {
			return `Weekly (${dayNames[schedule.dayOfWeek]}) at ${time}`;
		} else if (schedule.frequency === 'monthly' && schedule.dayOfMonth !== undefined) {
			return `Monthly (${schedule.dayOfMonth}th) at ${time}`;
		}
		return schedule.frequency;
	}

	function formatRelativeTime(dateStr?: string): string {
		if (!dateStr) return 'Never';

		const date = new Date(dateStr);
		const now = new Date();
		const diff = now.getTime() - date.getTime();
		const seconds = Math.floor(diff / 1000);
		const minutes = Math.floor(seconds / 60);
		const hours = Math.floor(minutes / 60);
		const days = Math.floor(hours / 24);

		if (days > 1) return `${days} days ago`;
		if (days === 1) return 'Yesterday';
		if (hours > 0) return `${hours} hour${hours > 1 ? 's' : ''} ago`;
		if (minutes > 0) return `${minutes} minute${minutes > 1 ? 's' : ''} ago`;
		return 'Just now';
	}

	function formatDuration(startStr: string, endStr?: string): string {
		if (!endStr) return '-';

		const start = new Date(startStr);
		const end = new Date(endStr);
		const diff = end.getTime() - start.getTime();
		const seconds = Math.floor(diff / 1000);
		const minutes = Math.floor(seconds / 60);

		if (minutes > 0) {
			const secs = seconds % 60;
			return `${minutes}m ${secs}s`;
		}
		return `${seconds}s`;
	}

	function formatDateTime(dateStr?: string): string {
		if (!dateStr) return 'N/A';
		const date = new Date(dateStr);
		return date.toLocaleString('en-US', {
			month: 'short',
			day: 'numeric',
			hour: '2-digit',
			minute: '2-digit'
		});
	}

	// Update job progress from SSE
	function updateJobProgress(event: ProgressEvent) {
		runningJobs.set(event.jobId, event);

		// Update job in jobs array
		jobs = jobs.map(j => {
			if (j.id === event.jobId) {
				return {
					...j,
					status: event.status,
					processedRecords: event.processedRecords,
					totalRecords: event.totalRecords,
					duplicatesFound: event.duplicatesFound || j.duplicatesFound
				};
			}
			return j;
		});

		// Reload if completed or failed
		if (event.status === 'completed' || event.status === 'failed') {
			runningJobs.delete(event.jobId);
			loadData();
		}
	}

	// API calls
	async function loadData() {
		try {
			loading = true;
			error = null;

			const [schedulesData, jobsData, entitiesData] = await Promise.all([
				get<ScanSchedule[]>('/scan-jobs/schedules'),
				get<{ data: ScanJob[], total: number }>(`/scan-jobs?page=${jobPage}&pageSize=${jobPageSize}`),
				get<EntityDef[]>('/admin/entities')
			]);

			schedules = schedulesData || [];
			jobs = jobsData.data || [];
			jobTotal = jobsData.total || 0;
			entities = entitiesData;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load scan jobs';
			addToast(error, 'error');
		} finally {
			loading = false;
		}
	}

	function startEdit(entityType: string) {
		const schedule = schedules.find(s => s.entityType === entityType);
		if (!schedule) return;

		editingScheduleEntity = entityType;
		editForm = {
			frequency: schedule.frequency,
			dayOfWeek: schedule.dayOfWeek || 1,
			dayOfMonth: schedule.dayOfMonth || 1,
			hour: schedule.hour,
			minute: schedule.minute,
			isEnabled: schedule.isEnabled
		};
	}

	function cancelEdit() {
		editingScheduleEntity = null;
	}

	async function saveSchedule(entityType: string) {
		try {
			const payload: Partial<ScanSchedule> = {
				entityType,
				frequency: editForm.frequency,
				hour: editForm.hour,
				minute: editForm.minute,
				isEnabled: editForm.isEnabled
			};

			if (editForm.frequency === 'weekly') {
				payload.dayOfWeek = editForm.dayOfWeek;
			} else if (editForm.frequency === 'monthly') {
				payload.dayOfMonth = editForm.dayOfMonth;
			}

			await put(`/scan-jobs/schedules/${entityType}`, payload);
			addToast('Schedule saved successfully', 'success');
			editingScheduleEntity = null;
			await loadData();
		} catch (e) {
			addToast(e instanceof Error ? e.message : 'Failed to save schedule', 'error');
		}
	}

	async function deleteSchedule(entityType: string) {
		if (!confirm(`Delete scan schedule for ${entityType}?`)) return;

		try {
			await del(`/scan-jobs/schedules/${entityType}`);
			addToast('Schedule deleted', 'success');
			await loadData();
		} catch (e) {
			addToast(e instanceof Error ? e.message : 'Failed to delete schedule', 'error');
		}
	}

	async function toggleEnabled(entityType: string) {
		const schedule = schedules.find(s => s.entityType === entityType);
		if (!schedule) return;

		try {
			await put(`/scan-jobs/schedules/${entityType}`, {
				...schedule,
				isEnabled: !schedule.isEnabled
			});
			addToast(`Schedule ${!schedule.isEnabled ? 'enabled' : 'disabled'}`, 'success');
			await loadData();
		} catch (e) {
			addToast(e instanceof Error ? e.message : 'Failed to update schedule', 'error');
		}
	}

	function openNewSchedule() {
		isAddingNew = true;
		newEntityType = availableEntities[0]?.name || '';
		editForm = {
			frequency: 'daily',
			dayOfWeek: 1,
			dayOfMonth: 1,
			hour: 2,
			minute: 0,
			isEnabled: true
		};
	}

	function cancelNewSchedule() {
		isAddingNew = false;
		newEntityType = '';
	}

	async function createSchedule() {
		if (!newEntityType) {
			addToast('Please select an entity type', 'error');
			return;
		}

		await saveSchedule(newEntityType);
		isAddingNew = false;
		newEntityType = '';
	}

	function openRunModal() {
		showRunModal = true;
		runEntityType = entities[0]?.name || '';
	}

	function closeRunModal() {
		showRunModal = false;
		runEntityType = '';
	}

	async function runNow() {
		if (!runEntityType) {
			addToast('Please select an entity type', 'error');
			return;
		}

		try {
			isRunning = true;
			const result = await post<{ jobId: string }>('/scan-jobs/run', { entityType: runEntityType });
			addToast(`Scan started for ${runEntityType}`, 'success');
			closeRunModal();
			await loadData();
		} catch (e) {
			addToast(e instanceof Error ? e.message : 'Failed to start scan', 'error');
		} finally {
			isRunning = false;
		}
	}

	async function retryJob(jobId: string) {
		try {
			await post(`/scan-jobs/${jobId}/retry`, {});
			addToast('Scan retry started', 'success');
			await loadData();
		} catch (e) {
			addToast(e instanceof Error ? e.message : 'Failed to retry scan', 'error');
		}
	}

	// SSE connection
	onMount(() => {
		loadData();

		// Connect to SSE for real-time progress
		const eventSource = new EventSource(`${API_BASE}/scan-jobs/progress/stream`, {
			withCredentials: true
		});

		eventSource.addEventListener('progress', (event) => {
			const data: ProgressEvent = JSON.parse(event.data);
			updateJobProgress(data);
		});

		eventSource.onerror = (error) => {
			console.error('SSE error:', error);
			// EventSource auto-reconnects, no manual retry needed
		};

		// Cleanup on unmount
		return () => {
			eventSource.close();
		};
	});
</script>

<div class="space-y-6">
	<!-- Header -->
	<div class="flex items-center justify-between">
		<div>
			<nav class="text-sm text-gray-500 mb-2">
				<a href="/admin" class="hover:text-gray-700">Administration</a>
				<span class="mx-2">/</span>
				<a href="/admin/data-quality" class="hover:text-gray-700">Data Quality</a>
				<span class="mx-2">/</span>
				<span class="text-gray-900">Scan Jobs</span>
			</nav>
			<h1 class="text-2xl font-bold text-gray-900">Scan Jobs</h1>
			<p class="mt-1 text-sm text-gray-500">
				Manage scheduled duplicate detection scans and view job history
			</p>
		</div>
		<button
			onclick={openRunModal}
			class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-600/90 transition-colors flex items-center gap-2"
		>
			<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
			</svg>
			Run Now
		</button>
	</div>

	{#if loading}
		<div class="text-center py-12 text-gray-500">Loading scan jobs...</div>
	{:else if error}
		<div class="text-center py-12 text-red-500">{error}</div>
	{:else}
		<!-- Schedules Table -->
		<div class="bg-white shadow rounded-lg overflow-hidden">
			<div class="px-6 py-4 border-b border-gray-200 flex items-center justify-between">
				<h2 class="text-lg font-medium text-gray-900">Scan Schedules</h2>
				<button
					onclick={openNewSchedule}
					class="px-3 py-1.5 text-sm bg-blue-50 text-blue-600 rounded hover:bg-blue-100 transition-colors"
				>
					Add Schedule
				</button>
			</div>

			<table class="min-w-full divide-y divide-gray-200">
				<thead class="bg-gray-50">
					<tr>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Entity Type
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Schedule
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Last Run
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Status
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Next Run
						</th>
						<th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
							Actions
						</th>
					</tr>
				</thead>
				<tbody class="bg-white divide-y divide-gray-200">
					{#if isAddingNew}
						<tr class="bg-blue-50">
							<td class="px-6 py-4">
								<select
									bind:value={newEntityType}
									class="px-3 py-1.5 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500"
								>
									{#each availableEntities as entity}
										<option value={entity.name}>{entity.label}</option>
									{/each}
								</select>
							</td>
							<td colspan="4" class="px-6 py-4">
								<div class="flex items-center gap-4">
									<select
										bind:value={editForm.frequency}
										class="px-3 py-1.5 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500"
									>
										<option value="daily">Daily</option>
										<option value="weekly">Weekly</option>
										<option value="monthly">Monthly</option>
									</select>

									{#if editForm.frequency === 'weekly'}
										<select
											bind:value={editForm.dayOfWeek}
											class="px-3 py-1.5 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500"
										>
											{#each dayNames as day, i}
												<option value={i}>{day}</option>
											{/each}
										</select>
									{:else if editForm.frequency === 'monthly'}
										<input
											type="number"
											bind:value={editForm.dayOfMonth}
											min="1"
											max="28"
											class="w-20 px-3 py-1.5 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500"
										/>
										<span class="text-sm text-gray-500">Day of month</span>
									{/if}

									<input
										type="number"
										bind:value={editForm.hour}
										min="0"
										max="23"
										class="w-16 px-3 py-1.5 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500"
									/>
									<span class="text-sm text-gray-500">:</span>
									<input
										type="number"
										bind:value={editForm.minute}
										min="0"
										max="59"
										class="w-16 px-3 py-1.5 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500"
									/>
								</div>
							</td>
							<td class="px-6 py-4 text-right">
								<div class="flex items-center justify-end gap-2">
									<button
										onclick={createSchedule}
										class="px-3 py-1 text-sm bg-blue-600 text-white rounded hover:bg-blue-700"
									>
										Save
									</button>
									<button
										onclick={cancelNewSchedule}
										class="px-3 py-1 text-sm bg-gray-100 text-gray-700 rounded hover:bg-gray-200"
									>
										Cancel
									</button>
								</div>
							</td>
						</tr>
					{/if}

					{#each scheduleWithStatus as { schedule, latestJob, isRunning, progress } (schedule.entityType)}
						{#if editingScheduleEntity === schedule.entityType}
							<tr class="bg-blue-50">
								<td class="px-6 py-4 font-medium text-gray-900">
									{schedule.entityType}
								</td>
								<td colspan="4" class="px-6 py-4">
									<div class="flex items-center gap-4">
										<select
											bind:value={editForm.frequency}
											class="px-3 py-1.5 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500"
										>
											<option value="daily">Daily</option>
											<option value="weekly">Weekly</option>
											<option value="monthly">Monthly</option>
										</select>

										{#if editForm.frequency === 'weekly'}
											<select
												bind:value={editForm.dayOfWeek}
												class="px-3 py-1.5 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500"
											>
												{#each dayNames as day, i}
													<option value={i}>{day}</option>
												{/each}
											</select>
										{:else if editForm.frequency === 'monthly'}
											<input
												type="number"
												bind:value={editForm.dayOfMonth}
												min="1"
												max="28"
												class="w-20 px-3 py-1.5 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500"
											/>
											<span class="text-sm text-gray-500">Day of month</span>
										{/if}

										<input
											type="number"
											bind:value={editForm.hour}
											min="0"
											max="23"
											class="w-16 px-3 py-1.5 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500"
										/>
										<span class="text-sm text-gray-500">:</span>
										<input
											type="number"
											bind:value={editForm.minute}
											min="0"
											max="59"
											class="w-16 px-3 py-1.5 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500"
										/>
									</div>
								</td>
								<td class="px-6 py-4 text-right">
									<div class="flex items-center justify-end gap-2">
										<button
											onclick={() => saveSchedule(schedule.entityType)}
											class="px-3 py-1 text-sm bg-blue-600 text-white rounded hover:bg-blue-700"
										>
											Save
										</button>
										<button
											onclick={cancelEdit}
											class="px-3 py-1 text-sm bg-gray-100 text-gray-700 rounded hover:bg-gray-200"
										>
											Cancel
										</button>
									</div>
								</td>
							</tr>
						{:else}
							<tr class="hover:bg-gray-50">
								<td class="px-6 py-4 font-medium text-gray-900">
									{schedule.entityType}
								</td>
								<td class="px-6 py-4 text-sm text-gray-500">
									{formatSchedule(schedule)}
								</td>
								<td class="px-6 py-4 text-sm text-gray-500">
									{formatRelativeTime(latestJob?.startedAt)}
								</td>
								<td class="px-6 py-4">
									{#if isRunning && progress}
										<div class="space-y-1">
											<div class="flex items-center justify-between text-xs text-gray-600 mb-1">
												<span>Running</span>
												<span>{progress.percentage}% ({progress.processedRecords.toLocaleString()} / {progress.totalRecords.toLocaleString()} records)</span>
											</div>
											<div class="w-full bg-gray-200 rounded-full h-2">
												<div
													class="bg-blue-500 h-2 rounded-full transition-all duration-300"
													style="width: {progress.percentage}%"
												></div>
											</div>
										</div>
									{:else if schedule.isEnabled}
										<span class="px-2 py-1 text-xs rounded-full bg-green-100 text-green-800">
											Enabled
										</span>
									{:else}
										<span class="px-2 py-1 text-xs rounded-full bg-gray-100 text-gray-800">
											Disabled
										</span>
									{/if}
								</td>
								<td class="px-6 py-4 text-sm text-gray-500">
									{formatDateTime(schedule.nextRunAt)}
								</td>
								<td class="px-6 py-4 text-right">
									<div class="flex items-center justify-end gap-2">
										<button
											onclick={() => toggleEnabled(schedule.entityType)}
											class="p-2 text-gray-400 hover:text-blue-600 hover:bg-blue-50 rounded-lg transition-colors"
											title={schedule.isEnabled ? 'Disable' : 'Enable'}
										>
											{#if schedule.isEnabled}
												<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
													<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 9v6m4-6v6m7-3a9 9 0 11-18 0 9 9 0 0118 0z" />
												</svg>
											{:else}
												<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
													<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
												</svg>
											{/if}
										</button>
										<button
											onclick={() => startEdit(schedule.entityType)}
											class="p-2 text-gray-400 hover:text-blue-600 hover:bg-blue-50 rounded-lg transition-colors"
											title="Edit schedule"
										>
											<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
												<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
											</svg>
										</button>
										<button
											onclick={() => deleteSchedule(schedule.entityType)}
											class="p-2 text-gray-400 hover:text-red-600 hover:bg-red-50 rounded-lg transition-colors"
											title="Delete schedule"
										>
											<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
												<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
											</svg>
										</button>
									</div>
								</td>
							</tr>
						{/if}
					{:else}
						<tr>
							<td colspan="6" class="px-6 py-8 text-center text-gray-500">
								No scan schedules configured. Click "Add Schedule" to create one.
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<!-- Recent Jobs -->
		<div class="bg-white shadow rounded-lg overflow-hidden">
			<div class="px-6 py-4 border-b border-gray-200">
				<h2 class="text-lg font-medium text-gray-900">Recent Jobs</h2>
			</div>

			<table class="min-w-full divide-y divide-gray-200">
				<thead class="bg-gray-50">
					<tr>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Date
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Entity Type
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Status
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Records Processed
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Duplicates Found
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Duration
						</th>
						<th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
							Actions
						</th>
					</tr>
				</thead>
				<tbody class="bg-white divide-y divide-gray-200">
					{#each paginatedJobs as job (job.id)}
						<tr class="hover:bg-gray-50">
							<td class="px-6 py-4 text-sm text-gray-500">
								{formatDateTime(job.startedAt)}
							</td>
							<td class="px-6 py-4 text-sm font-medium text-gray-900">
								{job.entityType}
							</td>
							<td class="px-6 py-4">
								{#if job.status === 'completed'}
									<span class="px-2 py-1 text-xs rounded-full bg-green-100 text-green-800">
										Completed
									</span>
								{:else if job.status === 'failed'}
									<span
										class="px-2 py-1 text-xs rounded-full bg-red-100 text-red-800 cursor-help"
										title={job.errorMessage || 'Unknown error'}
									>
										Failed
									</span>
								{:else if job.status === 'running'}
									<span class="px-2 py-1 text-xs rounded-full bg-blue-100 text-blue-800 animate-pulse">
										Running
									</span>
								{:else}
									<span class="px-2 py-1 text-xs rounded-full bg-gray-100 text-gray-800">
										{job.status}
									</span>
								{/if}
							</td>
							<td class="px-6 py-4 text-sm text-gray-500">
								{job.processedRecords.toLocaleString()} / {job.totalRecords.toLocaleString()}
							</td>
							<td class="px-6 py-4 text-sm text-gray-500">
								{job.duplicatesFound.toLocaleString()}
							</td>
							<td class="px-6 py-4 text-sm text-gray-500">
								{formatDuration(job.startedAt, job.completedAt)}
							</td>
							<td class="px-6 py-4 text-right">
								{#if job.status === 'failed'}
									<button
										onclick={() => retryJob(job.id)}
										class="px-3 py-1 text-sm bg-blue-50 text-blue-600 rounded hover:bg-blue-100 transition-colors"
									>
										Retry
									</button>
								{/if}
							</td>
						</tr>
					{:else}
						<tr>
							<td colspan="7" class="px-6 py-8 text-center text-gray-500">
								No job history available
							</td>
						</tr>
					{/each}
				</tbody>
			</table>

			<!-- Pagination -->
			{#if totalJobPages > 1}
				<div class="px-6 py-4 border-t border-gray-200 flex items-center justify-between">
					<div class="text-sm text-gray-500">
						Showing {((jobPage - 1) * jobPageSize) + 1} to {Math.min(jobPage * jobPageSize, jobTotal)} of {jobTotal} jobs
					</div>
					<div class="flex gap-2">
						<button
							onclick={() => jobPage = Math.max(1, jobPage - 1)}
							disabled={jobPage === 1}
							class="px-3 py-1 text-sm border border-gray-300 rounded hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
						>
							Previous
						</button>
						<button
							onclick={() => jobPage = Math.min(totalJobPages, jobPage + 1)}
							disabled={jobPage === totalJobPages}
							class="px-3 py-1 text-sm border border-gray-300 rounded hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
						>
							Next
						</button>
					</div>
				</div>
			{/if}
		</div>
	{/if}
</div>

<!-- Run Now Modal -->
{#if showRunModal}
	<div class="fixed inset-0 z-50 overflow-y-auto">
		<div class="flex min-h-screen items-center justify-center p-4">
			<!-- Backdrop -->
			<div class="fixed inset-0 bg-black bg-opacity-50" onclick={closeRunModal}></div>

			<!-- Modal content -->
			<div class="relative bg-white rounded-lg shadow-xl max-w-md w-full p-6">
				<div class="flex items-center justify-between mb-4">
					<h2 class="text-xl font-semibold text-gray-900">Run Scan Now</h2>
					<button
						onclick={closeRunModal}
						class="text-gray-400 hover:text-gray-600"
					>
						<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
						</svg>
					</button>
				</div>

				<form onsubmit={(e) => { e.preventDefault(); runNow(); }}>
					<div class="mb-4">
						<label for="runEntityType" class="block text-sm font-medium text-gray-700 mb-2">
							Select Entity Type
						</label>
						<select
							id="runEntityType"
							bind:value={runEntityType}
							class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500"
						>
							{#each entities as entity}
								<option value={entity.name}>{entity.label}</option>
							{/each}
						</select>
						<p class="mt-2 text-sm text-gray-500">
							This will scan all {runEntityType} records for duplicates
						</p>
					</div>

					<div class="flex justify-end gap-3">
						<button
							type="button"
							onclick={closeRunModal}
							class="px-4 py-2 text-gray-700 bg-gray-100 rounded-lg hover:bg-gray-200 transition-colors"
						>
							Cancel
						</button>
						<button
							type="submit"
							disabled={isRunning}
							class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-600/90 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
						>
							{isRunning ? 'Starting...' : 'Start Scan'}
						</button>
					</div>
				</form>
			</div>
		</div>
	</div>
{/if}
