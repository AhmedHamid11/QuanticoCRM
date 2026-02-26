<script lang="ts">
	import { page } from '$app/stores';

	let slug = $derived($page.params.slug);
	let guestName = $derived($page.url.searchParams.get('name') || 'Guest');
	let startTimeStr = $derived($page.url.searchParams.get('time') || '');
	let pageTitle = $derived($page.url.searchParams.get('title') || 'Meeting');

	function formatDateTime(isoStr: string): string {
		if (!isoStr) return '';
		try {
			const d = new Date(isoStr);
			return d.toLocaleString([], {
				weekday: 'long',
				month: 'long',
				day: 'numeric',
				year: 'numeric',
				hour: 'numeric',
				minute: '2-digit',
				hour12: true
			});
		} catch {
			return isoStr;
		}
	}
</script>

<svelte:head>
	<title>Booking Confirmed</title>
</svelte:head>

<div class="min-h-screen bg-gray-100 flex items-center justify-center px-4 py-12">
	<div class="max-w-md w-full bg-white rounded-xl shadow p-8 text-center">
		<!-- Checkmark icon -->
		<div class="flex justify-center mb-6">
			<div class="w-16 h-16 bg-green-100 rounded-full flex items-center justify-center">
				<svg class="w-9 h-9 text-green-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M5 13l4 4L19 7" />
				</svg>
			</div>
		</div>

		<h1 class="text-2xl font-bold text-gray-900 mb-2">You're booked!</h1>
		<p class="text-gray-500 text-sm mb-6">A confirmation has been sent to your email.</p>

		<!-- Meeting details card -->
		<div class="bg-gray-50 border border-gray-200 rounded-lg p-5 mb-6 text-left">
			<div class="space-y-3">
				<div class="flex items-start gap-3">
					<svg class="w-5 h-5 text-blue-500 mt-0.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
					</svg>
					<div>
						<p class="text-xs text-gray-500 uppercase tracking-wide font-medium mb-0.5">Meeting</p>
						<p class="text-sm font-medium text-gray-900">{pageTitle}</p>
					</div>
				</div>

				{#if startTimeStr}
					<div class="flex items-start gap-3">
						<svg class="w-5 h-5 text-blue-500 mt-0.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
						</svg>
						<div>
							<p class="text-xs text-gray-500 uppercase tracking-wide font-medium mb-0.5">Date & Time</p>
							<p class="text-sm font-medium text-gray-900">{formatDateTime(startTimeStr)}</p>
						</div>
					</div>
				{/if}

				<div class="flex items-start gap-3">
					<svg class="w-5 h-5 text-blue-500 mt-0.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
					</svg>
					<div>
						<p class="text-xs text-gray-500 uppercase tracking-wide font-medium mb-0.5">Guest</p>
						<p class="text-sm font-medium text-gray-900">{guestName}</p>
					</div>
				</div>
			</div>
		</div>

		<p class="text-xs text-gray-400 mb-6">
			A calendar invite has been sent to your email address.
		</p>

		<a
			href="/book/{slug}"
			class="inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-blue-600 border border-blue-300 rounded-lg hover:bg-blue-50 transition-colors"
		>
			Schedule another time
		</a>
	</div>
</div>
