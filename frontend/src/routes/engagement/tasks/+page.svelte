<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { get, post, put } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';

	interface TaskView {
		ExecutionID: string;
		StepType: string;
		StepNumber: number;
		ConfigJSON?: string;
		ScheduledAt: string;
		SequenceID: string;
		SequenceName: string;
		ContactID: string;
		ContactName: string;
		ContactEmail: string;
		ContactPhone?: string;
		EnrollmentID: string;
		LastOpenAt?: string;
		LastReplyAt?: string;
		SmsOptedOut?: boolean;
	}

	interface StepConfig {
		linkedinSubType?: string;
		suggestedMessage?: string;
		script?: string;
		description?: string;
		continueWithoutCompleting?: boolean;
		linkedinUrl?: string;
		body?: string;
	}

	const CALL_DISPOSITIONS = ['Connected', 'Voicemail', 'No Answer', 'Wrong Number', 'Not Interested'] as const;
	type CallDisposition = typeof CALL_DISPOSITIONS[number];

	let tasks = $state<TaskView[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let expandedRows = $state<Set<string>>(new Set());
	let actionInProgress = $state<string | null>(null);
	let rescheduleTarget = $state<string | null>(null);
	let rescheduleDate = $state('');
	let refreshInterval: ReturnType<typeof setInterval> | null = null;

	// Disposition picker state
	let dispositionTarget = $state<string | null>(null);
	let selectedDisposition = $state<CallDisposition | null>(null);
	let dispositionNotes = $state('');
	let dispositionContactId = $state<string | null>(null);

	function parseConfig(task: TaskView): StepConfig {
		if (!task.ConfigJSON) return {};
		try {
			return JSON.parse(task.ConfigJSON) as StepConfig;
		} catch {
			return {};
		}
	}

	function toggleRow(execId: string) {
		const next = new Set(expandedRows);
		if (next.has(execId)) {
			next.delete(execId);
		} else {
			next.add(execId);
		}
		expandedRows = next;
	}

	function getTaskLabel(task: TaskView): string {
		const config = parseConfig(task);
		if (task.StepType === 'linkedin' && config.linkedinSubType) {
			const subtypes: Record<string, string> = {
				view_profile: 'LinkedIn: View Profile',
				connect: 'LinkedIn: Connect',
				message: 'LinkedIn: Send Message',
				interact: 'LinkedIn: Interact with Post'
			};
			return subtypes[config.linkedinSubType] ?? 'LinkedIn Task';
		}
		const labels: Record<string, string> = {
			call: 'Call Task',
			sms: 'SMS Task',
			linkedin: 'LinkedIn Task',
			custom: 'Custom Task'
		};
		return labels[task.StepType] ?? task.StepType;
	}

	function getRelativeDueDate(scheduledAt: string): { label: string; overdue: boolean } {
		if (!scheduledAt) return { label: 'Unknown', overdue: false };
		const due = new Date(scheduledAt);
		const now = new Date();
		const diffMs = due.getTime() - now.getTime();
		const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

		if (diffDays < 0) {
			const overdueDays = Math.abs(diffDays);
			return {
				label: overdueDays === 0 ? 'Overdue today' : `Overdue ${overdueDays} day${overdueDays !== 1 ? 's' : ''}`,
				overdue: true
			};
		} else if (diffDays === 0) {
			return { label: 'Due today', overdue: false };
		} else if (diffDays === 1) {
			return { label: 'Due tomorrow', overdue: false };
		} else {
			return { label: `Due in ${diffDays} days`, overdue: false };
		}
	}

	function getStepTypeIcon(stepType: string): string {
		switch (stepType) {
			case 'call': return 'M3 5a2 2 0 012-2h3.28a1 1 0 01.948.684l1.498 4.493a1 1 0 01-.502 1.21l-2.257 1.13a11.042 11.042 0 005.516 5.516l1.13-2.257a1 1 0 011.21-.502l4.493 1.498a1 1 0 01.684.949V19a2 2 0 01-2 2h-1C9.716 21 3 14.284 3 6V5z';
			case 'sms': return 'M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z';
			case 'linkedin': return 'M16 8a6 6 0 016 6v7h-4v-7a2 2 0 00-2-2 2 2 0 00-2 2v7h-4v-7a6 6 0 016-6zM2 9h4v12H2z M4 6a2 2 0 100-4 2 2 0 000 4z';
			default: return 'M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2';
		}
	}

	async function loadTasks() {
		try {
			loading = true;
			error = null;
			const result = await get<TaskView[]>('/engagement/tasks');
			tasks = result ?? [];
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load tasks';
		} finally {
			loading = false;
		}
	}

	// Opens the disposition picker for call tasks
	function openDispositionPicker(task: TaskView) {
		dispositionTarget = task.ExecutionID;
		dispositionContactId = task.ContactID;
		selectedDisposition = null;
		dispositionNotes = '';
	}

	// Submits a call task completion with disposition
	async function submitCallComplete() {
		if (!dispositionTarget || !selectedDisposition) return;
		const execId = dispositionTarget;
		actionInProgress = execId;
		try {
			await post(`/engagement/tasks/${execId}/complete`, {
				disposition: selectedDisposition,
				notes: dispositionNotes || undefined,
				contactId: dispositionContactId || undefined
			});
			tasks = tasks.filter((t) => t.ExecutionID !== execId);
			toast.success('Call logged successfully');
			dispositionTarget = null;
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to complete task');
		} finally {
			actionInProgress = null;
		}
	}

	// Completes non-call tasks without disposition
	async function completeTask(execId: string) {
		actionInProgress = execId;
		try {
			await post(`/engagement/tasks/${execId}/complete`, {});
			tasks = tasks.filter((t) => t.ExecutionID !== execId);
			toast.success('Task completed');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to complete task');
		} finally {
			actionInProgress = null;
		}
	}

	async function skipTask(execId: string) {
		if (!confirm('Skip this task? The sequence will continue to the next step.')) return;
		actionInProgress = execId;
		try {
			await post(`/engagement/tasks/${execId}/skip`, {});
			tasks = tasks.filter((t) => t.ExecutionID !== execId);
			toast.success('Task skipped');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to skip task');
		} finally {
			actionInProgress = null;
		}
	}

	function openReschedule(execId: string) {
		rescheduleTarget = execId;
		// Default to tomorrow
		const tomorrow = new Date();
		tomorrow.setDate(tomorrow.getDate() + 1);
		rescheduleDate = tomorrow.toISOString().slice(0, 16);
	}

	async function confirmReschedule() {
		if (!rescheduleTarget || !rescheduleDate) return;
		const execId = rescheduleTarget;
		actionInProgress = execId;
		try {
			const scheduledAt = new Date(rescheduleDate).toISOString();
			await put(`/engagement/tasks/${execId}/reschedule`, { scheduledAt });
			// Update task in list
			tasks = tasks.map((t) =>
				t.ExecutionID === execId ? { ...t, ScheduledAt: scheduledAt } : t
			);
			toast.success('Task rescheduled');
			rescheduleTarget = null;
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to reschedule task');
		} finally {
			actionInProgress = null;
		}
	}

	onMount(() => {
		loadTasks();
		// Auto-refresh every 30 seconds
		refreshInterval = setInterval(loadTasks, 30000);
	});

	onDestroy(() => {
		if (refreshInterval) clearInterval(refreshInterval);
	});
</script>

<svelte:head>
	<title>Task Inbox</title>
</svelte:head>

<div class="p-6 max-w-5xl mx-auto">
	<div class="flex items-center justify-between mb-6">
		<div>
			<h1 class="text-2xl font-bold text-gray-900">Task Inbox</h1>
			<p class="text-sm text-gray-500 mt-1">Your due manual tasks from active sequences.</p>
		</div>
		<button
			onclick={loadTasks}
			class="flex items-center gap-1.5 px-3 py-1.5 text-sm text-gray-600 border border-gray-300 rounded-md hover:bg-gray-50 transition-colors"
		>
			<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
			</svg>
			Refresh
		</button>
	</div>

	{#if loading}
		<div class="flex items-center justify-center py-16 text-gray-400">
			<svg class="animate-spin h-6 w-6 mr-2" fill="none" viewBox="0 0 24 24">
				<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
				<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z"></path>
			</svg>
			Loading tasks...
		</div>
	{:else if error}
		<div class="rounded-md bg-red-50 p-4 text-sm text-red-700">
			{error}
			<button onclick={loadTasks} class="ml-2 underline hover:no-underline">Retry</button>
		</div>
	{:else if tasks.length === 0}
		<div class="text-center py-16 text-gray-400">
			<svg class="mx-auto h-12 w-12 mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
			</svg>
			<p class="text-lg font-medium text-gray-600">No tasks due</p>
			<p class="text-sm mt-1">You're all caught up! Check back later.</p>
		</div>
	{:else}
		<div class="bg-white border border-gray-200 rounded-lg overflow-hidden divide-y divide-gray-100">
			{#each tasks as task (task.ExecutionID)}
				{@const config = parseConfig(task)}
				{@const dueInfo = getRelativeDueDate(task.ScheduledAt)}
				{@const isExpanded = expandedRows.has(task.ExecutionID)}
				{@const isBusy = actionInProgress === task.ExecutionID}
				{@const isSmsBlocked = task.StepType === 'sms' && task.SmsOptedOut}

				<div class="group">
					<!-- Main row -->
					<div
						class="flex items-center gap-4 px-4 py-3 hover:bg-gray-50 cursor-pointer transition-colors"
						onclick={() => toggleRow(task.ExecutionID)}
					>
						<!-- Step type icon -->
						<div class="shrink-0 w-8 h-8 rounded-full {task.StepType === 'sms' ? 'bg-purple-100' : 'bg-blue-100'} flex items-center justify-center">
							<svg class="h-4 w-4 {task.StepType === 'sms' ? 'text-purple-600' : 'text-blue-600'}" fill="none" viewBox="0 0 24 24" stroke="currentColor">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={getStepTypeIcon(task.StepType)} />
							</svg>
						</div>

						<!-- Contact + task info -->
						<div class="flex-1 min-w-0">
							<div class="flex items-center gap-2 flex-wrap">
								<a
									href="/contacts/{task.ContactID}"
									onclick={(e) => e.stopPropagation()}
									class="text-sm font-medium text-gray-900 hover:text-blue-600 hover:underline"
								>
									{task.ContactName}
								</a>
								<span class="text-xs text-gray-400">|</span>
								<span class="text-xs font-medium text-gray-600">{getTaskLabel(task)}</span>
								{#if isSmsBlocked}
									<span class="inline-flex items-center px-1.5 py-0.5 rounded text-xs bg-yellow-100 text-yellow-800 font-medium">
										SMS Opted Out
									</span>
								{/if}
								{#if task.LastOpenAt}
									<span class="inline-flex items-center px-1.5 py-0.5 rounded text-xs bg-blue-50 text-blue-600 font-medium">
										Opened email
									</span>
								{/if}
								{#if task.LastReplyAt}
									<span class="inline-flex items-center px-1.5 py-0.5 rounded text-xs bg-green-50 text-green-600 font-medium">
										Replied
									</span>
								{/if}
							</div>
							<div class="flex items-center gap-2 mt-0.5">
								<span class="text-xs text-gray-400">{task.SequenceName}</span>
								<span class="text-xs text-gray-300">·</span>
								<span class="text-xs {dueInfo.overdue ? 'text-red-500 font-medium' : 'text-gray-400'}">
									{dueInfo.label}
								</span>
							</div>
						</div>

						<!-- Action buttons -->
						<div class="flex items-center gap-1.5 shrink-0" onclick={(e) => e.stopPropagation()}>
							<!-- Complete: call tasks open picker, SMS blocked tasks hide it, others complete directly -->
							{#if !isSmsBlocked}
								<button
									onclick={() => task.StepType === 'call' ? openDispositionPicker(task) : completeTask(task.ExecutionID)}
									disabled={isBusy}
									title={task.StepType === 'call' ? 'Log call disposition' : 'Mark complete'}
									class="p-1.5 rounded text-gray-400 hover:text-green-600 hover:bg-green-50 transition-colors disabled:opacity-50"
								>
									<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
									</svg>
								</button>
							{/if}
							<!-- Skip -->
							<button
								onclick={() => skipTask(task.ExecutionID)}
								disabled={isBusy}
								title={isSmsBlocked ? 'Skip to advance sequence' : 'Skip task'}
								class="p-1.5 rounded {isSmsBlocked ? 'text-yellow-600 bg-yellow-50 hover:bg-yellow-100' : 'text-gray-400 hover:text-yellow-600 hover:bg-yellow-50'} transition-colors disabled:opacity-50"
							>
								<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 5l7 7-7 7M5 5l7 7-7 7" />
								</svg>
							</button>
							<!-- Reschedule -->
							<button
								onclick={() => openReschedule(task.ExecutionID)}
								disabled={isBusy}
								title="Reschedule"
								class="p-1.5 rounded text-gray-400 hover:text-blue-600 hover:bg-blue-50 transition-colors disabled:opacity-50"
							>
								<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
								</svg>
							</button>
						</div>
					</div>

					<!-- SMS opt-out blocking banner -->
					{#if isSmsBlocked}
						<div class="px-4 py-2 bg-yellow-50 border-t border-yellow-100 flex items-center gap-2">
							<svg class="h-4 w-4 text-yellow-600 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
							</svg>
							<p class="text-xs text-yellow-800">
								Contact has opted out of SMS. You can skip this task to advance the sequence.
							</p>
						</div>
					{/if}

					<!-- Expanded detail row -->
					{#if isExpanded}
						<div class="px-4 pb-4 pt-1 bg-gray-50 border-t border-gray-100">
							{#if task.StepType === 'call'}
								<div class="space-y-2">
									{#if task.ContactPhone}
										<div>
											<span class="text-xs font-medium text-gray-500">Phone:</span>
											<a href="tel:{task.ContactPhone}" class="text-sm text-blue-600 hover:underline ml-1">{task.ContactPhone}</a>
										</div>
									{/if}
									{#if task.ContactEmail}
										<div>
											<span class="text-xs font-medium text-gray-500">Email:</span>
											<span class="text-sm text-gray-700 ml-1">{task.ContactEmail}</span>
										</div>
									{/if}
									{#if config.script}
										<div>
											<span class="text-xs font-medium text-gray-500">Script:</span>
											<p class="text-sm text-gray-700 mt-1 p-2 bg-white rounded border border-gray-200 whitespace-pre-wrap">{config.script}</p>
										</div>
									{/if}
								</div>

							{:else if task.StepType === 'sms'}
								<div class="space-y-2">
									{#if task.ContactPhone}
										<div>
											<span class="text-xs font-medium text-gray-500">Send to:</span>
											<a href="tel:{task.ContactPhone}" class="text-sm text-blue-600 hover:underline ml-1 font-medium">{task.ContactPhone}</a>
										</div>
									{:else}
										<div>
											<span class="text-xs text-gray-400 italic">No phone number on file</span>
										</div>
									{/if}
									{#if config.body}
										<div>
											<span class="text-xs font-medium text-gray-500">Message:</span>
											<p class="text-sm text-gray-700 mt-1 p-2 bg-white rounded border border-gray-200 whitespace-pre-wrap">{config.body}</p>
										</div>
									{/if}
								</div>

							{:else if task.StepType === 'linkedin'}
								<div class="space-y-2">
									{#if config.linkedinUrl}
										<div>
											<span class="text-xs font-medium text-gray-500">LinkedIn:</span>
											<a href={config.linkedinUrl} target="_blank" rel="noopener noreferrer" class="text-sm text-blue-600 hover:underline ml-1 break-all">
												{config.linkedinUrl}
											</a>
										</div>
									{/if}
									{#if config.suggestedMessage}
										<div>
											<span class="text-xs font-medium text-gray-500">Suggested Message:</span>
											<p class="text-sm text-gray-700 mt-1 p-2 bg-white rounded border border-gray-200 whitespace-pre-wrap">{config.suggestedMessage}</p>
										</div>
									{/if}
								</div>

							{:else if task.StepType === 'custom'}
								{#if config.description}
									<div>
										<span class="text-xs font-medium text-gray-500">Task:</span>
										<p class="text-sm text-gray-700 mt-1 p-2 bg-white rounded border border-gray-200 whitespace-pre-wrap">{config.description}</p>
									</div>
								{/if}
							{/if}
						</div>
					{/if}
				</div>
			{/each}
		</div>
	{/if}
</div>

<!-- Reschedule modal -->
{#if rescheduleTarget}
	<div
		class="fixed inset-0 bg-black bg-opacity-40 flex items-center justify-center z-50"
		onclick={() => (rescheduleTarget = null)}
		role="dialog"
		aria-modal="true"
		aria-label="Reschedule task"
	>
		<div
			class="bg-white rounded-lg shadow-xl w-full max-w-sm mx-4 p-6"
			onclick={(e) => e.stopPropagation()}
			role="presentation"
		>
			<h2 class="text-lg font-semibold text-gray-900 mb-4">Reschedule Task</h2>
			<label class="block text-sm font-medium text-gray-700 mb-1">New Date and Time</label>
			<input
				type="datetime-local"
				bind:value={rescheduleDate}
				class="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
			/>
			<div class="flex justify-end gap-2 mt-4">
				<button
					onclick={() => (rescheduleTarget = null)}
					class="px-4 py-2 text-sm text-gray-700 border border-gray-300 rounded-md hover:bg-gray-50"
				>
					Cancel
				</button>
				<button
					onclick={confirmReschedule}
					disabled={!rescheduleDate}
					class="px-4 py-2 text-sm bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					Reschedule
				</button>
			</div>
		</div>
	</div>
{/if}

<!-- Call disposition picker modal -->
{#if dispositionTarget}
	<div
		class="fixed inset-0 bg-black bg-opacity-40 flex items-center justify-center z-50"
		onclick={() => (dispositionTarget = null)}
		role="dialog"
		aria-modal="true"
		aria-label="Log call disposition"
	>
		<div
			class="bg-white rounded-lg shadow-xl w-full max-w-md mx-4 p-6"
			onclick={(e) => e.stopPropagation()}
			role="presentation"
		>
			<h2 class="text-lg font-semibold text-gray-900 mb-1">Log Call Outcome</h2>
			<p class="text-sm text-gray-500 mb-4">Select what happened on this call before completing the task.</p>

			<!-- Disposition options -->
			<div class="grid grid-cols-1 gap-2 mb-4">
				{#each CALL_DISPOSITIONS as disposition}
					{@const isSelected = selectedDisposition === disposition}
					<button
						onclick={() => (selectedDisposition = disposition)}
						class="flex items-center gap-3 px-3 py-2.5 rounded-lg border text-left transition-colors {isSelected
							? 'border-blue-500 bg-blue-50 text-blue-900'
							: 'border-gray-200 hover:border-gray-300 hover:bg-gray-50 text-gray-700'}"
					>
						<div class="w-4 h-4 rounded-full border-2 shrink-0 flex items-center justify-center {isSelected ? 'border-blue-500 bg-blue-500' : 'border-gray-300'}">
							{#if isSelected}
								<div class="w-1.5 h-1.5 rounded-full bg-white"></div>
							{/if}
						</div>
						<span class="text-sm font-medium">{disposition}</span>
					</button>
				{/each}
			</div>

			<!-- Notes -->
			<div class="mb-5">
				<label class="block text-sm font-medium text-gray-700 mb-1">Notes <span class="text-gray-400 font-normal">(optional)</span></label>
				<textarea
					bind:value={dispositionNotes}
					rows="2"
					placeholder="e.g. Left voicemail, will call back Thursday..."
					class="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 resize-none"
				></textarea>
			</div>

			<div class="flex justify-end gap-2">
				<button
					onclick={() => (dispositionTarget = null)}
					class="px-4 py-2 text-sm text-gray-700 border border-gray-300 rounded-md hover:bg-gray-50"
				>
					Cancel
				</button>
				<button
					onclick={submitCallComplete}
					disabled={!selectedDisposition || actionInProgress === dispositionTarget}
					class="px-4 py-2 text-sm bg-green-600 text-white rounded-md hover:bg-green-700 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					{actionInProgress === dispositionTarget ? 'Saving...' : 'Complete Call'}
				</button>
			</div>
		</div>
	</div>
{/if}
