<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { get, post, put, del } from '$lib/utils/api';
	import { addToast } from '$lib/stores/toast.svelte';
	import type { SchedulingPage, GoogleCalendarStatus, WeeklyAvailability, TimeWindow } from '$lib/types/scheduling';
	import { DAYS_OF_WEEK, TIMEZONES } from '$lib/types/scheduling';

	// State
	let googleStatus = $state<GoogleCalendarStatus>('not_connected');
	let loadingGoogleStatus = $state(true);
	let connectingGoogle = $state(false);
	let disconnectingGoogle = $state(false);

	let pages = $state<SchedulingPage[]>([]);
	let loadingPages = $state(true);

	let showCreateForm = $state(false);
	let editingPageId = $state<string | null>(null);
	let submitting = $state(false);

	// Form state
	let formTitle = $state('');
	let formSlug = $state('');
	let formDescription = $state('');
	let formDuration = $state(30);
	let formBuffer = $state(0);
	let formMaxDays = $state(30);
	let formTimezone = $state('America/New_York');
	let formAvailability = $state<WeeklyAvailability>({
		monday: [{ start: '09:00', end: '17:00' }],
		tuesday: [{ start: '09:00', end: '17:00' }],
		wednesday: [{ start: '09:00', end: '17:00' }],
		thursday: [{ start: '09:00', end: '17:00' }],
		friday: [{ start: '09:00', end: '17:00' }]
	});

	const API_BASE = '/api/v1';

	// Check for ?connected=true on mount
	onMount(async () => {
		const connected = $page.url.searchParams.get('connected');
		if (connected === 'true') {
			addToast('Google Calendar connected successfully!', 'success');
		}
		const error = $page.url.searchParams.get('error');
		if (error) {
			addToast(`Failed to connect Google Calendar: ${error.replace(/_/g, ' ')}`, 'error');
		}

		await Promise.all([loadGoogleStatus(), loadPages()]);
	});

	async function loadGoogleStatus() {
		loadingGoogleStatus = true;
		try {
			const result = await get<{ status: GoogleCalendarStatus }>('/scheduling/google/status');
			googleStatus = result.status;
		} catch {
			// Not fatal — just show not_connected
			googleStatus = 'not_connected';
		} finally {
			loadingGoogleStatus = false;
		}
	}

	async function connectGoogle() {
		connectingGoogle = true;
		try {
			const result = await get<{ authUrl: string }>('/scheduling/google/connect');
			window.location.href = result.authUrl;
		} catch (err) {
			addToast('Failed to get authorization URL', 'error');
			connectingGoogle = false;
		}
	}

	async function disconnectGoogle() {
		if (!confirm('Disconnect Google Calendar? Your scheduling pages will still work but will not check your calendar for conflicts.')) return;
		disconnectingGoogle = true;
		try {
			await del('/scheduling/google/disconnect');
			googleStatus = 'not_connected';
			addToast('Google Calendar disconnected', 'info');
		} catch {
			addToast('Failed to disconnect Google Calendar', 'error');
		} finally {
			disconnectingGoogle = false;
		}
	}

	async function loadPages() {
		loadingPages = true;
		try {
			const result = await get<{ pages: SchedulingPage[] }>('/scheduling/pages');
			pages = result.pages || [];
		} catch {
			pages = [];
		} finally {
			loadingPages = false;
		}
	}

	function slugify(title: string): string {
		return title
			.toLowerCase()
			.replace(/[^a-z0-9\s-]/g, '')
			.replace(/\s+/g, '-')
			.replace(/-+/g, '-')
			.replace(/^-|-$/g, '')
			.substring(0, 50);
	}

	function onTitleInput() {
		if (!editingPageId) {
			formSlug = slugify(formTitle);
		}
	}

	function resetForm() {
		formTitle = '';
		formSlug = '';
		formDescription = '';
		formDuration = 30;
		formBuffer = 0;
		formMaxDays = 30;
		formTimezone = 'America/New_York';
		formAvailability = {
			monday: [{ start: '09:00', end: '17:00' }],
			tuesday: [{ start: '09:00', end: '17:00' }],
			wednesday: [{ start: '09:00', end: '17:00' }],
			thursday: [{ start: '09:00', end: '17:00' }],
			friday: [{ start: '09:00', end: '17:00' }]
		};
		showCreateForm = false;
		editingPageId = null;
	}

	function startCreate() {
		resetForm();
		showCreateForm = true;
		editingPageId = null;
	}

	function startEdit(p: SchedulingPage) {
		editingPageId = p.id;
		formTitle = p.title;
		formSlug = p.slug;
		formDescription = p.description;
		formDuration = p.durationMinutes;
		formBuffer = p.bufferMinutes;
		formMaxDays = p.maxDaysAhead;
		formTimezone = p.timezone;
		formAvailability = { ...p.availability };
		showCreateForm = true;
	}

	async function submitForm() {
		if (!formTitle.trim() || !formSlug.trim()) {
			addToast('Title and slug are required', 'error');
			return;
		}

		submitting = true;
		try {
			const payload = {
				title: formTitle.trim(),
				slug: formSlug.trim(),
				description: formDescription.trim(),
				durationMinutes: formDuration,
				bufferMinutes: formBuffer,
				maxDaysAhead: formMaxDays,
				timezone: formTimezone,
				availability: formAvailability
			};

			if (editingPageId) {
				await put(`/scheduling/pages/${editingPageId}`, payload);
				addToast('Scheduling page updated', 'success');
			} else {
				await post('/scheduling/pages', payload);
				addToast('Scheduling page created', 'success');
			}

			resetForm();
			await loadPages();
		} catch (err) {
			addToast(err instanceof Error ? err.message : 'Failed to save scheduling page', 'error');
		} finally {
			submitting = false;
		}
	}

	async function deletePage(id: string) {
		if (!confirm('Delete this scheduling page? All bookings will remain but the page will be inaccessible.')) return;
		try {
			await del(`/scheduling/pages/${id}`);
			addToast('Scheduling page deleted', 'info');
			pages = pages.filter(p => p.id !== id);
		} catch {
			addToast('Failed to delete scheduling page', 'error');
		}
	}

	function copyLink(slug: string) {
		const url = `${window.location.origin}/book/${slug}`;
		navigator.clipboard.writeText(url).then(() => {
			addToast('Link copied to clipboard!', 'success');
		}).catch(() => {
			addToast(`Booking link: ${url}`, 'info');
		});
	}

	function isDayEnabled(day: string): boolean {
		const windows = (formAvailability as any)[day];
		return windows && windows.length > 0;
	}

	function toggleDay(day: string) {
		const key = day as keyof WeeklyAvailability;
		if (isDayEnabled(day)) {
			const updated = { ...formAvailability };
			delete (updated as any)[day];
			formAvailability = updated;
		} else {
			formAvailability = { ...formAvailability, [key]: [{ start: '09:00', end: '17:00' }] };
		}
	}

	function addWindow(day: string) {
		const key = day as keyof WeeklyAvailability;
		const existing = (formAvailability as any)[day] || [];
		formAvailability = { ...formAvailability, [key]: [...existing, { start: '09:00', end: '17:00' }] };
	}

	function removeWindow(day: string, index: number) {
		const key = day as keyof WeeklyAvailability;
		const existing: TimeWindow[] = [...((formAvailability as any)[day] || [])];
		existing.splice(index, 1);
		if (existing.length === 0) {
			const updated = { ...formAvailability };
			delete (updated as any)[day];
			formAvailability = updated;
		} else {
			formAvailability = { ...formAvailability, [key]: existing };
		}
	}

	function updateWindowTime(day: string, index: number, field: 'start' | 'end', value: string) {
		const key = day as keyof WeeklyAvailability;
		const existing: TimeWindow[] = [...((formAvailability as any)[day] || [])];
		existing[index] = { ...existing[index], [field]: value };
		formAvailability = { ...formAvailability, [key]: existing };
	}

	function formatStatusLabel(status: GoogleCalendarStatus): string {
		if (status === 'connected') return 'Connected';
		if (status === 'expired') return 'Token Expired';
		return 'Not Connected';
	}

	function statusBadgeClass(status: GoogleCalendarStatus): string {
		if (status === 'connected') return 'bg-green-100 text-green-800';
		if (status === 'expired') return 'bg-yellow-100 text-yellow-800';
		return 'bg-gray-100 text-gray-600';
	}

	function capitalize(s: string): string {
		return s.charAt(0).toUpperCase() + s.slice(1);
	}
</script>

<div class="space-y-6">
	<!-- Header -->
	<div>
		<h1 class="text-2xl font-bold text-gray-900">Scheduling</h1>
		<p class="mt-1 text-sm text-gray-500">Create booking pages and connect your Google Calendar</p>
	</div>

	<!-- Google Calendar Connection -->
	<div class="crm-card overflow-hidden">
		<div class="px-6 py-4 border-b border-gray-200">
			<h2 class="text-lg font-medium text-gray-900">Google Calendar</h2>
			<p class="mt-1 text-sm text-gray-500">Connect to automatically check your availability and create calendar events for new bookings</p>
		</div>
		<div class="px-6 py-4">
			{#if loadingGoogleStatus}
				<div class="flex items-center gap-2 text-sm text-gray-500">
					<svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
						<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
					</svg>
					Checking status...
				</div>
			{:else}
				<div class="flex items-center justify-between">
					<div class="flex items-center gap-3">
						<svg class="w-8 h-8" viewBox="0 0 24 24" fill="none">
							<path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" fill="#4285F4"/>
							<path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853"/>
							<path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l3.66-2.84z" fill="#FBBC05"/>
							<path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335"/>
						</svg>
						<div>
							<div class="flex items-center gap-2">
								<span class="text-sm font-medium text-gray-900">Google Calendar</span>
								<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium {statusBadgeClass(googleStatus)}">
									{formatStatusLabel(googleStatus)}
								</span>
							</div>
							<p class="text-xs text-gray-500 mt-0.5">
								{#if googleStatus === 'connected'}
									Your calendar is connected. Available slots will exclude busy times.
								{:else if googleStatus === 'expired'}
									Your connection has expired. Please reconnect.
								{:else}
									Not connected. Slots will be based on your availability windows only.
								{/if}
							</p>
						</div>
					</div>
					<div class="flex gap-2">
						{#if googleStatus === 'connected'}
							<button
								onclick={disconnectGoogle}
								disabled={disconnectingGoogle}
								class="px-3 py-1.5 text-sm font-medium text-red-600 border border-red-300 rounded-md hover:bg-red-50 disabled:opacity-50 transition-colors"
							>
								{disconnectingGoogle ? 'Disconnecting...' : 'Disconnect'}
							</button>
						{:else}
							<button
								onclick={connectGoogle}
								disabled={connectingGoogle}
								class="px-3 py-1.5 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-600/90 disabled:opacity-50 transition-colors"
							>
								{connectingGoogle ? 'Redirecting...' : 'Connect Google Calendar'}
							</button>
						{/if}
					</div>
				</div>
			{/if}
		</div>
	</div>

	<!-- Scheduling Pages -->
	<div class="crm-card overflow-hidden">
		<div class="px-6 py-4 border-b border-gray-200 flex items-center justify-between">
			<div>
				<h2 class="text-lg font-medium text-gray-900">Scheduling Pages</h2>
				<p class="mt-1 text-sm text-gray-500">Share a booking link so people can schedule time with you</p>
			</div>
			{#if !showCreateForm}
				<button
					onclick={startCreate}
					class="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-600/90 transition-colors"
				>
					+ Create Page
				</button>
			{/if}
		</div>

		<!-- Create/Edit Form -->
		{#if showCreateForm}
			<div class="px-6 py-5 border-b border-gray-200 bg-gray-50">
				<h3 class="text-base font-medium text-gray-900 mb-4">
					{editingPageId ? 'Edit Scheduling Page' : 'New Scheduling Page'}
				</h3>
				<div class="space-y-4">
					<!-- Title -->
					<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
						<div>
							<label class="block text-sm font-medium text-gray-700 mb-1">Title *</label>
							<input
								type="text"
								bind:value={formTitle}
								oninput={onTitleInput}
								placeholder="30 Minute Meeting"
								class="w-full px-3 py-2 border border-gray-300 rounded-md text-sm focus:ring-blue-500 focus:border-blue-500"
							/>
						</div>
						<div>
							<label class="block text-sm font-medium text-gray-700 mb-1">Slug *</label>
							<div class="flex items-center">
								<span class="text-sm text-gray-400 mr-1">/book/</span>
								<input
									type="text"
									bind:value={formSlug}
									placeholder="30-min-meeting"
									class="flex-1 px-3 py-2 border border-gray-300 rounded-md text-sm focus:ring-blue-500 focus:border-blue-500"
								/>
							</div>
							<p class="mt-1 text-xs text-gray-400">Lowercase letters, numbers, hyphens only</p>
						</div>
					</div>

					<!-- Description -->
					<div>
						<label class="block text-sm font-medium text-gray-700 mb-1">Description</label>
						<textarea
							bind:value={formDescription}
							rows="2"
							placeholder="A brief meeting to discuss..."
							class="w-full px-3 py-2 border border-gray-300 rounded-md text-sm focus:ring-blue-500 focus:border-blue-500"
						></textarea>
					</div>

					<!-- Duration, Buffer, Max Days, Timezone -->
					<div class="grid grid-cols-2 md:grid-cols-4 gap-4">
						<div>
							<label class="block text-sm font-medium text-gray-700 mb-1">Duration</label>
							<select
								bind:value={formDuration}
								class="w-full px-3 py-2 border border-gray-300 rounded-md text-sm focus:ring-blue-500 focus:border-blue-500"
							>
								<option value={15}>15 min</option>
								<option value={30}>30 min</option>
								<option value={45}>45 min</option>
								<option value={60}>60 min</option>
								<option value={90}>90 min</option>
							</select>
						</div>
						<div>
							<label class="block text-sm font-medium text-gray-700 mb-1">Buffer</label>
							<select
								bind:value={formBuffer}
								class="w-full px-3 py-2 border border-gray-300 rounded-md text-sm focus:ring-blue-500 focus:border-blue-500"
							>
								<option value={0}>No buffer</option>
								<option value={5}>5 min</option>
								<option value={10}>10 min</option>
								<option value={15}>15 min</option>
							</select>
						</div>
						<div>
							<label class="block text-sm font-medium text-gray-700 mb-1">Max Days Ahead</label>
							<input
								type="number"
								bind:value={formMaxDays}
								min="1"
								max="365"
								class="w-full px-3 py-2 border border-gray-300 rounded-md text-sm focus:ring-blue-500 focus:border-blue-500"
							/>
						</div>
						<div>
							<label class="block text-sm font-medium text-gray-700 mb-1">Timezone</label>
							<select
								bind:value={formTimezone}
								class="w-full px-3 py-2 border border-gray-300 rounded-md text-sm focus:ring-blue-500 focus:border-blue-500"
							>
								{#each TIMEZONES as tz}
									<option value={tz}>{tz}</option>
								{/each}
							</select>
						</div>
					</div>

					<!-- Weekly Availability -->
					<div>
						<label class="block text-sm font-medium text-gray-700 mb-2">Weekly Availability</label>
						<div class="space-y-2">
							{#each DAYS_OF_WEEK as day}
								<div class="flex items-start gap-3">
									<div class="flex items-center mt-2 w-28">
										<input
											type="checkbox"
											id="day-{day}"
											checked={isDayEnabled(day)}
											onchange={() => toggleDay(day)}
											class="mr-2 h-4 w-4 text-blue-600 rounded border-gray-300"
										/>
										<label for="day-{day}" class="text-sm text-gray-700">{capitalize(day)}</label>
									</div>
									{#if isDayEnabled(day)}
										<div class="flex-1 space-y-1">
											{#each ((formAvailability as any)[day] || []) as window, i}
												<div class="flex items-center gap-2">
													<input
														type="time"
														value={window.start}
														onchange={(e) => updateWindowTime(day, i, 'start', (e.target as HTMLInputElement).value)}
														class="px-2 py-1 border border-gray-300 rounded text-sm focus:ring-blue-500 focus:border-blue-500"
													/>
													<span class="text-gray-400 text-sm">to</span>
													<input
														type="time"
														value={window.end}
														onchange={(e) => updateWindowTime(day, i, 'end', (e.target as HTMLInputElement).value)}
														class="px-2 py-1 border border-gray-300 rounded text-sm focus:ring-blue-500 focus:border-blue-500"
													/>
													<button
														onclick={() => removeWindow(day, i)}
														class="text-red-400 hover:text-red-600 text-sm"
														title="Remove window"
													>
														&times;
													</button>
												</div>
											{/each}
											<button
												onclick={() => addWindow(day)}
												class="text-xs text-blue-600 hover:text-blue-700"
											>
												+ Add time window
											</button>
										</div>
									{/if}
								</div>
							{/each}
						</div>
					</div>

					<!-- Form Actions -->
					<div class="flex gap-3 pt-2">
						<button
							onclick={submitForm}
							disabled={submitting}
							class="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-600/90 disabled:opacity-50 transition-colors"
						>
							{submitting ? 'Saving...' : (editingPageId ? 'Save Changes' : 'Create Page')}
						</button>
						<button
							onclick={resetForm}
							class="px-4 py-2 text-sm font-medium text-gray-700 border border-gray-300 rounded-md hover:bg-gray-50 transition-colors"
						>
							Cancel
						</button>
					</div>
				</div>
			</div>
		{/if}

		<!-- Pages List -->
		<div class="divide-y divide-gray-100">
			{#if loadingPages}
				<div class="px-6 py-8 text-center text-sm text-gray-500">Loading scheduling pages...</div>
			{:else if pages.length === 0}
				<div class="px-6 py-10 text-center">
					<svg class="w-12 h-12 mx-auto text-gray-300 mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
					</svg>
					<p class="text-gray-500 text-sm">No scheduling pages yet</p>
					<p class="text-gray-400 text-xs mt-1">Create a page to share your booking link</p>
				</div>
			{:else}
				{#each pages as page (page.id)}
					<div class="px-6 py-4 hover:bg-gray-50 transition-colors">
						<div class="flex items-start justify-between gap-4">
							<div class="flex-1 min-w-0">
								<div class="flex items-center gap-2 flex-wrap">
									<h3 class="text-sm font-medium text-gray-900">{page.title}</h3>
									<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium {page.isActive ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-600'}">
										{page.isActive ? 'Active' : 'Inactive'}
									</span>
									<span class="text-xs text-gray-400">{page.durationMinutes} min</span>
								</div>
								<p class="text-xs text-gray-500 mt-1 font-mono">/book/{page.slug}</p>
								{#if page.description}
									<p class="text-xs text-gray-500 mt-1 line-clamp-1">{page.description}</p>
								{/if}
							</div>
							<div class="flex items-center gap-2 flex-shrink-0">
								<button
									onclick={() => copyLink(page.slug)}
									class="px-3 py-1.5 text-xs font-medium text-gray-600 border border-gray-300 rounded hover:bg-gray-50 transition-colors"
									title="Copy booking link"
								>
									Copy Link
								</button>
								<a
									href="/book/{page.slug}"
									target="_blank"
									class="px-3 py-1.5 text-xs font-medium text-blue-600 border border-blue-300 rounded hover:bg-blue-50 transition-colors"
								>
									Preview
								</a>
								<button
									onclick={() => startEdit(page)}
									class="px-3 py-1.5 text-xs font-medium text-gray-600 border border-gray-300 rounded hover:bg-gray-50 transition-colors"
								>
									Edit
								</button>
								<button
									onclick={() => deletePage(page.id)}
									class="px-3 py-1.5 text-xs font-medium text-red-600 border border-red-300 rounded hover:bg-red-50 transition-colors"
								>
									Delete
								</button>
							</div>
						</div>
					</div>
				{/each}
			{/if}
		</div>
	</div>
</div>
