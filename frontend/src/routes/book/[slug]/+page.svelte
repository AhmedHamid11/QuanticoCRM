<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { PUBLIC_API_URL } from '$env/static/public';
	import type { SchedulingPagePublicView, AvailableSlot, WeeklyAvailability } from '$lib/types/scheduling';
	import { DAYS_OF_WEEK } from '$lib/types/scheduling';

	// Public API — no auth headers
	const API_BASE = PUBLIC_API_URL || '/api/v1';

	let slug = $derived($page.params.slug);

	let pageInfo = $state<SchedulingPagePublicView | null>(null);
	let loadingPage = $state(true);
	let pageError = $state<string | null>(null);

	// Calendar state
	let currentYear = $state(new Date().getFullYear());
	let currentMonth = $state(new Date().getMonth()); // 0-indexed
	let selectedDate = $state<string | null>(null); // YYYY-MM-DD

	// Slots state
	let slots = $state<AvailableSlot[]>([]);
	let loadingSlots = $state(false);
	let selectedSlot = $state<AvailableSlot | null>(null);

	// Booking form state
	let showBookingForm = $state(false);
	let guestName = $state('');
	let guestEmail = $state('');
	let guestNotes = $state('');
	let submitting = $state(false);
	let bookingError = $state<string | null>(null);

	onMount(async () => {
		await loadPageInfo();
	});

	async function loadPageInfo() {
		loadingPage = true;
		pageError = null;
		try {
			const resp = await fetch(`${API_BASE}/public/scheduling/${slug}`);
			if (!resp.ok) {
				if (resp.status === 404) {
					pageError = 'This scheduling page was not found or is no longer active.';
				} else {
					pageError = 'Failed to load scheduling page.';
				}
				return;
			}
			pageInfo = await resp.json();
		} catch {
			pageError = 'Could not connect to the server.';
		} finally {
			loadingPage = false;
		}
	}

	async function loadSlots(dateStr: string) {
		loadingSlots = true;
		slots = [];
		selectedSlot = null;
		showBookingForm = false;
		try {
			const resp = await fetch(`${API_BASE}/public/scheduling/${slug}/slots?date=${dateStr}`);
			if (resp.ok) {
				const data = await resp.json();
				slots = data.slots || [];
			}
		} catch {
			slots = [];
		} finally {
			loadingSlots = false;
		}
	}

	async function selectDate(dateStr: string) {
		selectedDate = dateStr;
		await loadSlots(dateStr);
	}

	function selectSlot(slot: AvailableSlot) {
		selectedSlot = slot;
		showBookingForm = true;
		bookingError = null;
	}

	async function submitBooking(e: Event) {
		e.preventDefault();
		if (!selectedSlot) return;

		bookingError = null;
		submitting = true;
		try {
			const resp = await fetch(`${API_BASE}/public/scheduling/${slug}/book`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					guestName: guestName.trim(),
					guestEmail: guestEmail.trim(),
					guestNotes: guestNotes.trim(),
					startTime: selectedSlot.start
				})
			});

			if (!resp.ok) {
				const data = await resp.json().catch(() => ({}));
				bookingError = data.error || 'Failed to create booking.';
				return;
			}

			// Redirect to confirmation page
			const params = new URLSearchParams({
				name: guestName.trim(),
				time: selectedSlot.start,
				title: pageInfo?.title || ''
			});
			goto(`/book/${slug}/confirm?${params.toString()}`);
		} catch {
			bookingError = 'Network error. Please try again.';
		} finally {
			submitting = false;
		}
	}

	// Calendar helpers
	function getDaysInMonth(year: number, month: number): number {
		return new Date(year, month + 1, 0).getDate();
	}

	function getFirstDayOfMonth(year: number, month: number): number {
		// 0=Sunday, convert to Monday-first
		const day = new Date(year, month, 1).getDay();
		return day === 0 ? 6 : day - 1;
	}

	function isDateAvailable(year: number, month: number, day: number): boolean {
		if (!pageInfo) return false;
		const date = new Date(year, month, day);
		const today = new Date();
		today.setHours(0, 0, 0, 0);
		if (date < today) return false;

		const maxDate = new Date();
		maxDate.setDate(maxDate.getDate() + (pageInfo.maxDaysAhead || 30));
		if (date > maxDate) return false;

		const dayName = DAYS_OF_WEEK[date.getDay() === 0 ? 6 : date.getDay() - 1];
		const avail = pageInfo.availability as WeeklyAvailability;
		return !!(avail[dayName] && avail[dayName]!.length > 0);
	}

	function formatDateStr(year: number, month: number, day: number): string {
		return `${year}-${String(month + 1).padStart(2, '0')}-${String(day).padStart(2, '0')}`;
	}

	function prevMonth() {
		if (currentMonth === 0) {
			currentMonth = 11;
			currentYear--;
		} else {
			currentMonth--;
		}
		selectedDate = null;
		slots = [];
		selectedSlot = null;
		showBookingForm = false;
	}

	function nextMonth() {
		if (currentMonth === 11) {
			currentMonth = 0;
			currentYear++;
		} else {
			currentMonth++;
		}
		selectedDate = null;
		slots = [];
		selectedSlot = null;
		showBookingForm = false;
	}

	function formatTime(isoStr: string): string {
		const d = new Date(isoStr);
		return d.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit', hour12: true });
	}

	function formatSelectedDate(dateStr: string): string {
		const [year, month, day] = dateStr.split('-').map(Number);
		const d = new Date(year, month - 1, day);
		return d.toLocaleDateString([], { weekday: 'long', month: 'long', day: 'numeric', year: 'numeric' });
	}

	const MONTH_NAMES = ['January', 'February', 'March', 'April', 'May', 'June',
		'July', 'August', 'September', 'October', 'November', 'December'];
	const DAY_LABELS = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];
</script>

<svelte:head>
	<title>{pageInfo?.title || 'Schedule a Meeting'}</title>
</svelte:head>

<div class="min-h-screen bg-gray-100 py-8 px-4">
	<div class="max-w-lg mx-auto">
		{#if loadingPage}
			<div class="bg-white rounded-xl shadow p-8 text-center">
				<div class="flex justify-center mb-4">
					<svg class="w-8 h-8 animate-spin text-blue-500" fill="none" viewBox="0 0 24 24">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
						<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
					</svg>
				</div>
				<p class="text-gray-500">Loading scheduling page...</p>
			</div>
		{:else if pageError}
			<div class="bg-white rounded-xl shadow p-8 text-center">
				<svg class="w-12 h-12 mx-auto text-gray-300 mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
				</svg>
				<h2 class="text-lg font-semibold text-gray-700 mb-2">Page Not Found</h2>
				<p class="text-gray-500 text-sm">{pageError}</p>
			</div>
		{:else if pageInfo}
			<!-- Page Header -->
			<div class="bg-white rounded-xl shadow mb-4 p-6">
				<h1 class="text-xl font-bold text-gray-900">{pageInfo.title}</h1>
				{#if pageInfo.description}
					<p class="mt-1 text-sm text-gray-600">{pageInfo.description}</p>
				{/if}
				<div class="mt-3 flex items-center gap-3">
					<span class="inline-flex items-center gap-1.5 text-sm text-gray-500">
						<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
						</svg>
						{pageInfo.durationMinutes} min
					</span>
					<span class="text-gray-300">|</span>
					<span class="text-sm text-gray-500">{pageInfo.timezone}</span>
				</div>
			</div>

			{#if !showBookingForm}
				<!-- Calendar -->
				<div class="bg-white rounded-xl shadow mb-4 p-6">
					<div class="flex items-center justify-between mb-4">
						<button onclick={prevMonth} class="p-1.5 rounded hover:bg-gray-100 transition-colors" aria-label="Previous month">
							<svg class="w-5 h-5 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
							</svg>
						</button>
						<span class="text-sm font-semibold text-gray-900">
							{MONTH_NAMES[currentMonth]} {currentYear}
						</span>
						<button onclick={nextMonth} class="p-1.5 rounded hover:bg-gray-100 transition-colors" aria-label="Next month">
							<svg class="w-5 h-5 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
							</svg>
						</button>
					</div>

					<!-- Day headers -->
					<div class="grid grid-cols-7 mb-2">
						{#each DAY_LABELS as label}
							<div class="text-center text-xs font-medium text-gray-400 py-1">{label}</div>
						{/each}
					</div>

					<!-- Calendar grid -->
					<div class="grid grid-cols-7 gap-0.5">
						{#each Array(getFirstDayOfMonth(currentYear, currentMonth)) as _, i}
							<div></div>
						{/each}
						{#each Array(getDaysInMonth(currentYear, currentMonth)) as _, i}
							{@const day = i + 1}
							{@const dateStr = formatDateStr(currentYear, currentMonth, day)}
							{@const available = isDateAvailable(currentYear, currentMonth, day)}
							{@const isSelected = selectedDate === dateStr}
							<button
								onclick={() => available && selectDate(dateStr)}
								disabled={!available}
								class="
									aspect-square flex items-center justify-center text-sm rounded-full
									transition-colors
									{isSelected
										? 'bg-blue-600 text-white font-semibold'
										: available
											? 'text-gray-800 hover:bg-blue-100 cursor-pointer font-medium'
											: 'text-gray-300 cursor-not-allowed'}
								"
							>
								{day}
							</button>
						{/each}
					</div>
				</div>

				<!-- Time Slots -->
				{#if selectedDate}
					<div class="bg-white rounded-xl shadow p-6">
						<h2 class="text-sm font-semibold text-gray-700 mb-3">
							{formatSelectedDate(selectedDate)}
						</h2>
						{#if loadingSlots}
							<div class="flex justify-center py-6">
								<svg class="w-5 h-5 animate-spin text-blue-500" fill="none" viewBox="0 0 24 24">
									<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
									<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
								</svg>
							</div>
						{:else if slots.length === 0}
							<p class="text-center text-sm text-gray-500 py-4">No available slots on this day</p>
						{:else}
							<div class="grid grid-cols-3 gap-2">
								{#each slots as slot}
									<button
										onclick={() => selectSlot(slot)}
										class="px-3 py-2 text-sm font-medium text-blue-600 border border-blue-300 rounded-lg hover:bg-blue-50 transition-colors text-center"
									>
										{formatTime(slot.start)}
									</button>
								{/each}
							</div>
						{/if}
					</div>
				{/if}
			{:else}
				<!-- Booking Form -->
				<div class="bg-white rounded-xl shadow p-6">
					<button
						onclick={() => { showBookingForm = false; selectedSlot = null; }}
						class="flex items-center gap-1 text-sm text-blue-600 hover:text-blue-700 mb-4"
					>
						<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
						</svg>
						Back to time slots
					</button>

					{#if selectedSlot}
						<div class="bg-blue-50 border border-blue-200 rounded-lg p-3 mb-4">
							<p class="text-sm font-medium text-blue-800">{pageInfo.title}</p>
							<p class="text-sm text-blue-600 mt-0.5">
								{selectedDate ? formatSelectedDate(selectedDate) : ''} at {formatTime(selectedSlot.start)}
							</p>
							<p class="text-xs text-blue-500 mt-0.5">{pageInfo.durationMinutes} minutes &bull; {pageInfo.timezone}</p>
						</div>
					{/if}

					<form onsubmit={submitBooking} class="space-y-4">
						{#if bookingError}
							<div class="p-3 bg-red-50 border border-red-200 rounded-lg text-sm text-red-600">
								{bookingError}
							</div>
						{/if}

						<div>
							<label for="guestName" class="block text-sm font-medium text-gray-700 mb-1">Your Name *</label>
							<input
								id="guestName"
								type="text"
								bind:value={guestName}
								required
								class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-blue-500 focus:border-blue-500"
								placeholder="Jane Smith"
							/>
						</div>

						<div>
							<label for="guestEmail" class="block text-sm font-medium text-gray-700 mb-1">Email Address *</label>
							<input
								id="guestEmail"
								type="email"
								bind:value={guestEmail}
								required
								class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-blue-500 focus:border-blue-500"
								placeholder="jane@example.com"
							/>
						</div>

						<div>
							<label for="guestNotes" class="block text-sm font-medium text-gray-700 mb-1">Notes (optional)</label>
							<textarea
								id="guestNotes"
								bind:value={guestNotes}
								rows="3"
								class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-blue-500 focus:border-blue-500"
								placeholder="What would you like to discuss?"
							></textarea>
						</div>

						<button
							type="submit"
							disabled={submitting}
							class="w-full px-4 py-3 text-sm font-medium text-white bg-blue-600 rounded-lg hover:bg-blue-600/90 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
						>
							{submitting ? 'Booking...' : 'Confirm Booking'}
						</button>
					</form>
				</div>
			{/if}
		{/if}
	</div>
</div>
