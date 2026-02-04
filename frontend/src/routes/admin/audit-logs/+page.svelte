<script lang="ts">
	import { onMount } from 'svelte';
	import { get } from '$lib/utils/api';
	import { auth } from '$lib/stores/auth.svelte';
	import { addToast } from '$lib/stores/toast.svelte';

	interface AuditLogEntry {
		id: string;
		eventType: string;
		actorEmail: string;
		actorId: string;
		targetType?: string;
		targetId?: string;
		details?: string;
		success: boolean;
		ipAddress?: string;
		userAgent?: string;
		errorMsg?: string;
		createdAt: string;
	}

	interface AuditLogListResponse {
		data: AuditLogEntry[];
		total: number;
		page: number;
		pageSize: number;
		hasMore: boolean;
	}

	interface EventType {
		value: string;
		label: string;
	}

	interface FilterState {
		eventType: string;
		userId: string;
		dateFrom: string;
		dateTo: string;
	}

	let logs = $state<AuditLogEntry[]>([]);
	let loading = $state(true);
	let total = $state(0);
	let page = $state(1);
	let pageSize = 50;
	let eventTypes = $state<EventType[]>([]);
	let hasMore = $state(false);

	let filters = $state<FilterState>({
		eventType: '',
		userId: '',
		dateFrom: '',
		dateTo: ''
	});

	let verifyingChain = $state(false);
	let showVerifyModal = $state(false);
	let verifyResult = $state<{
		valid: boolean;
		entriesVerified: number;
		errors?: string[];
		firstEntryId?: string;
		lastEntryId?: string;
		firstBrokenEntry?: string;
	} | null>(null);

	// Load event types for filter dropdown
	async function loadEventTypes() {
		try {
			const response = await get<{ eventTypes: EventType[] }>('/admin/audit-logs/event-types');
			eventTypes = response.eventTypes || [];
		} catch (err) {
			console.error('Failed to load event types:', err);
		}
	}

	// Load logs with filters
	async function loadLogs() {
		loading = true;
		try {
			// Build query string
			const params = new URLSearchParams();
			params.set('page', page.toString());
			params.set('pageSize', pageSize.toString());

			if (filters.eventType) {
				params.set('eventTypes', filters.eventType);
			}
			if (filters.userId) {
				params.set('userId', filters.userId);
			}
			if (filters.dateFrom) {
				params.set('dateFrom', filters.dateFrom);
			}
			if (filters.dateTo) {
				params.set('dateTo', filters.dateTo);
			}

			const response = await get<AuditLogListResponse>(`/admin/audit-logs?${params.toString()}`);
			logs = response.data || [];
			total = response.total;
			hasMore = response.hasMore;
		} catch (err) {
			const message = err instanceof Error ? err.message : 'Failed to load audit logs';
			addToast(message, 'error');
		} finally {
			loading = false;
		}
	}

	// Apply filters
	function applyFilters() {
		page = 1; // Reset to first page
		loadLogs();
	}

	// Reset filters
	function resetFilters() {
		filters.eventType = '';
		filters.userId = '';
		filters.dateFrom = '';
		filters.dateTo = '';
		page = 1;
		loadLogs();
	}

	// Pagination
	function goToPage(newPage: number) {
		page = newPage;
		loadLogs();
	}

	// Export logs
	async function exportLogs(format: 'csv' | 'json') {
		try {
			// Build query string with filters
			const params = new URLSearchParams();
			params.set('format', format);

			if (filters.eventType) {
				params.set('eventTypes', filters.eventType);
			}
			if (filters.userId) {
				params.set('userId', filters.userId);
			}
			if (filters.dateFrom) {
				params.set('dateFrom', filters.dateFrom);
			}
			if (filters.dateTo) {
				params.set('dateTo', filters.dateTo);
			}

			// Trigger download
			const url = `/api/v1/admin/audit-logs/export?${params.toString()}`;
			const link = document.createElement('a');
			link.href = url;
			link.download = `audit-logs-export.${format}`;
			document.body.appendChild(link);
			link.click();
			document.body.removeChild(link);

			addToast(`Exporting audit logs as ${format.toUpperCase()}...`, 'success');
		} catch (err) {
			const message = err instanceof Error ? err.message : 'Failed to export audit logs';
			addToast(message, 'error');
		}
	}

	// Verify chain integrity
	async function verifyChain() {
		verifyingChain = true;
		try {
			const result = await get<{
				valid: boolean;
				entriesVerified: number;
				errors?: string[];
				firstEntryId?: string;
				lastEntryId?: string;
				firstBrokenEntry?: string;
			}>('/admin/audit-logs/verify');

			verifyResult = result;
			showVerifyModal = true;
		} catch (err) {
			const message = err instanceof Error ? err.message : 'Failed to verify chain';
			addToast(message, 'error');
		} finally {
			verifyingChain = false;
		}
	}

	// Get event description
	function getEventDescription(log: AuditLogEntry): string {
		let details: Record<string, unknown> = {};
		if (log.details) {
			try {
				details = JSON.parse(log.details);
			} catch {
				// Ignore parse errors
			}
		}

		const descriptions: Record<string, (log: AuditLogEntry, d: Record<string, unknown>) => string> = {
			'LOGIN_SUCCESS': () => 'logged in successfully',
			'LOGIN_FAILED': () => 'failed to log in',
			'LOGOUT': () => 'logged out',
			'PASSWORD_RESET': () => 'reset their password',
			'PASSWORD_CHANGE': () => 'changed their password',
			'USER_CREATE': () => 'created a new user',
			'USER_UPDATE': () => 'updated a user',
			'USER_DELETE': () => 'deleted a user',
			'USER_INVITE': () => 'invited a new user',
			'ROLE_CHANGE': (l, d) => `changed role to ${d.newRole || 'unknown'}`,
			'USER_STATUS_CHANGE': (l, d) => `${d.newStatus === 'active' ? 'activated' : 'deactivated'} a user`,
			'IMPERSONATION_START': () => 'started impersonation session',
			'IMPERSONATION_STOP': () => 'ended impersonation session',
			'API_TOKEN_CREATE': () => 'created an API token',
			'API_TOKEN_REVOKE': () => 'revoked an API token',
			'AUTHORIZATION_DENIED': (l, d) => `was denied access to ${d.path || 'a resource'}`,
			'ORG_SETTINGS_CHANGE': (l, d) => {
				const fields = d.changedFields as string[] | undefined;
				return `changed settings: ${fields?.join(', ') || 'unknown'}`;
			},
		};

		const fn = descriptions[log.eventType];
		return fn ? fn(log, details) : log.eventType.toLowerCase().replace(/_/g, ' ');
	}

	// Get event icon and color
	function getEventIcon(log: AuditLogEntry): { icon: string; color: string } {
		const eventType = log.eventType;
		const success = log.success;

		// Failures are always red
		if (!success) {
			return {
				icon: 'M6 18L18 6M6 6l12 12', // X icon
				color: 'text-red-600 bg-red-100'
			};
		}

		// Success events - categorized by type
		if (eventType.includes('LOGIN') || eventType.includes('LOGOUT')) {
			return {
				icon: 'M11 16l-4-4m0 0l4-4m-4 4h14m-5 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h7a3 3 0 013 3v1', // Login icon
				color: 'text-blue-600 bg-blue-100'
			};
		}

		if (eventType.includes('PASSWORD')) {
			return {
				icon: 'M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z', // Key icon
				color: 'text-amber-600 bg-amber-100'
			};
		}

		if (eventType.includes('USER') || eventType.includes('ROLE')) {
			return {
				icon: 'M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z', // Users icon
				color: 'text-green-600 bg-green-100'
			};
		}

		if (eventType.includes('IMPERSONATION')) {
			return {
				icon: 'M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4', // Switch icon
				color: 'text-purple-600 bg-purple-100'
			};
		}

		if (eventType.includes('API_TOKEN')) {
			return {
				icon: 'M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z', // Key icon
				color: 'text-indigo-600 bg-indigo-100'
			};
		}

		if (eventType.includes('AUTHORIZATION_DENIED')) {
			return {
				icon: 'M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z', // Lock icon
				color: 'text-red-600 bg-red-100'
			};
		}

		if (eventType.includes('SETTINGS')) {
			return {
				icon: 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z', // Settings icon
				color: 'text-gray-600 bg-gray-100'
			};
		}

		// Default
		return {
			icon: 'M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z', // Info icon
			color: 'text-gray-600 bg-gray-100'
		};
	}

	// Format timestamp
	function formatTimestamp(timestamp: string): string {
		const date = new Date(timestamp);
		return date.toLocaleString('en-US', {
			year: 'numeric',
			month: 'short',
			day: 'numeric',
			hour: '2-digit',
			minute: '2-digit',
			second: '2-digit'
		});
	}

	onMount(() => {
		loadEventTypes();
		loadLogs();
	});
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold text-gray-900">Audit Logs</h1>
			<p class="mt-1 text-sm text-gray-500">
				Security events and compliance audit trail
			</p>
		</div>
		<div class="flex items-center space-x-3">
			<button
				onclick={() => exportLogs('csv')}
				class="inline-flex items-center px-3 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50"
			>
				<svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
				</svg>
				Export CSV
			</button>
			<button
				onclick={() => exportLogs('json')}
				class="inline-flex items-center px-3 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50"
			>
				<svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
				</svg>
				Export JSON
			</button>
			<button
				onclick={verifyChain}
				disabled={verifyingChain}
				class="inline-flex items-center px-4 py-2 text-sm font-medium text-white bg-blue-600 border border-transparent rounded-md hover:bg-blue-600/90 disabled:opacity-50 disabled:cursor-not-allowed"
			>
				<svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
				</svg>
				{verifyingChain ? 'Verifying...' : 'Verify Chain'}
			</button>
			<a
				href="/admin"
				class="inline-flex items-center px-3 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50"
			>
				<svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 19l-7-7m0 0l7-7m-7 7h18" />
				</svg>
				Back to Admin
			</a>
		</div>
	</div>

	<!-- Filters -->
	<div class="bg-white shadow rounded-lg p-6">
		<h2 class="text-lg font-medium text-gray-900 mb-4">Filters</h2>
		<div class="grid grid-cols-1 md:grid-cols-4 gap-4">
			<div>
				<label for="eventType" class="block text-sm font-medium text-gray-700 mb-1">
					Event Type
				</label>
				<select
					id="eventType"
					bind:value={filters.eventType}
					class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 text-sm"
				>
					<option value="">All Events</option>
					{#each eventTypes as type}
						<option value={type.value}>{type.label}</option>
					{/each}
				</select>
			</div>

			<div>
				<label for="dateFrom" class="block text-sm font-medium text-gray-700 mb-1">
					From Date
				</label>
				<input
					type="date"
					id="dateFrom"
					bind:value={filters.dateFrom}
					class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 text-sm"
				/>
			</div>

			<div>
				<label for="dateTo" class="block text-sm font-medium text-gray-700 mb-1">
					To Date
				</label>
				<input
					type="date"
					id="dateTo"
					bind:value={filters.dateTo}
					class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 text-sm"
				/>
			</div>

			<div class="flex items-end space-x-2">
				<button
					onclick={applyFilters}
					class="flex-1 inline-flex justify-center items-center px-4 py-2 text-sm font-medium text-white bg-blue-600 border border-transparent rounded-md hover:bg-blue-600/90"
				>
					Apply
				</button>
				<button
					onclick={resetFilters}
					class="flex-1 inline-flex justify-center items-center px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50"
				>
					Reset
				</button>
			</div>
		</div>
	</div>

	<!-- Activity Timeline -->
	<div class="bg-white shadow rounded-lg overflow-hidden">
		{#if loading}
			<div class="flex items-center justify-center py-12">
				<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500"></div>
			</div>
		{:else if logs.length === 0}
			<div class="text-center py-12">
				<svg class="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
				</svg>
				<h3 class="mt-2 text-sm font-medium text-gray-900">No audit logs</h3>
				<p class="mt-1 text-sm text-gray-500">No events match your filters.</p>
			</div>
		{:else}
			<div class="divide-y divide-gray-200">
				{#each logs as log (log.id)}
					{@const eventStyle = getEventIcon(log)}
					<div class="px-6 py-4 hover:bg-gray-50">
						<div class="flex items-start space-x-3">
							<!-- Event Icon -->
							<div class="flex-shrink-0">
								<div class="w-10 h-10 rounded-full {eventStyle.color} flex items-center justify-center">
									<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={eventStyle.icon} />
									</svg>
								</div>
							</div>

							<!-- Event Details -->
							<div class="flex-1 min-w-0">
								<p class="text-sm text-gray-900">
									<span class="font-medium">{log.actorEmail || 'System'}</span>
									<span class="text-gray-600"> {getEventDescription(log)}</span>
								</p>
								<div class="mt-1 flex items-center space-x-4 text-xs text-gray-500">
									<span>{formatTimestamp(log.createdAt)}</span>
									{#if log.ipAddress}
										<span>IP: {log.ipAddress}</span>
									{/if}
									{#if !log.success && log.errorMsg}
										<span class="text-red-600">Error: {log.errorMsg}</span>
									{/if}
								</div>
							</div>

							<!-- Success/Failure Badge -->
							<div class="flex-shrink-0">
								{#if log.success}
									<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">
										Success
									</span>
								{:else}
									<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-800">
										Failed
									</span>
								{/if}
							</div>
						</div>
					</div>
				{/each}
			</div>

			<!-- Pagination -->
			<div class="bg-gray-50 px-4 py-3 flex items-center justify-between border-t border-gray-200 sm:px-6">
				<div class="flex-1 flex justify-between sm:hidden">
					<button
						onclick={() => goToPage(page - 1)}
						disabled={page === 1}
						class="relative inline-flex items-center px-4 py-2 border border-gray-300 text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
					>
						Previous
					</button>
					<button
						onclick={() => goToPage(page + 1)}
						disabled={!hasMore}
						class="ml-3 relative inline-flex items-center px-4 py-2 border border-gray-300 text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
					>
						Next
					</button>
				</div>
				<div class="hidden sm:flex-1 sm:flex sm:items-center sm:justify-between">
					<div>
						<p class="text-sm text-gray-700">
							Showing <span class="font-medium">{(page - 1) * pageSize + 1}</span> to <span class="font-medium">{Math.min(page * pageSize, total)}</span> of <span class="font-medium">{total}</span> events
						</p>
					</div>
					<div>
						<nav class="relative z-0 inline-flex rounded-md shadow-sm -space-x-px" aria-label="Pagination">
							<button
								onclick={() => goToPage(page - 1)}
								disabled={page === 1}
								class="relative inline-flex items-center px-2 py-2 rounded-l-md border border-gray-300 bg-white text-sm font-medium text-gray-500 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
							>
								<span class="sr-only">Previous</span>
								<svg class="h-5 w-5" fill="currentColor" viewBox="0 0 20 20">
									<path fill-rule="evenodd" d="M12.707 5.293a1 1 0 010 1.414L9.414 10l3.293 3.293a1 1 0 01-1.414 1.414l-4-4a1 1 0 010-1.414l4-4a1 1 0 011.414 0z" clip-rule="evenodd" />
								</svg>
							</button>
							<span class="relative inline-flex items-center px-4 py-2 border border-gray-300 bg-white text-sm font-medium text-gray-700">
								Page {page}
							</span>
							<button
								onclick={() => goToPage(page + 1)}
								disabled={!hasMore}
								class="relative inline-flex items-center px-2 py-2 rounded-r-md border border-gray-300 bg-white text-sm font-medium text-gray-500 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
							>
								<span class="sr-only">Next</span>
								<svg class="h-5 w-5" fill="currentColor" viewBox="0 0 20 20">
									<path fill-rule="evenodd" d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z" clip-rule="evenodd" />
								</svg>
							</button>
						</nav>
					</div>
				</div>
			</div>
		{/if}
	</div>
</div>

<!-- Verify Chain Modal -->
{#if showVerifyModal && verifyResult}
	<div class="fixed inset-0 z-50 overflow-y-auto">
		<div class="flex min-h-full items-end justify-center p-4 text-center sm:items-center sm:p-0">
			<!-- Background overlay -->
			<div
				class="fixed inset-0 bg-gray-500 bg-opacity-75 transition-opacity"
				onclick={() => showVerifyModal = false}
			></div>

			<!-- Modal panel -->
			<div class="relative transform overflow-hidden rounded-lg bg-white text-left shadow-xl transition-all sm:my-8 sm:w-full sm:max-w-lg">
				<div class="bg-white px-4 pb-4 pt-5 sm:p-6 sm:pb-4">
					<div class="sm:flex sm:items-start">
						<div class="mx-auto flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-full {verifyResult.valid ? 'bg-green-100' : 'bg-red-100'} sm:mx-0 sm:h-10 sm:w-10">
							{#if verifyResult.valid}
								<svg class="h-6 w-6 text-green-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
								</svg>
							{:else}
								<svg class="h-6 w-6 text-red-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
								</svg>
							{/if}
						</div>
						<div class="mt-3 text-center sm:ml-4 sm:mt-0 sm:text-left flex-1">
							<h3 class="text-lg font-semibold leading-6 text-gray-900">
								Chain Verification Result
							</h3>
							<div class="mt-4 space-y-3">
								<p class="text-sm text-gray-700">
									<span class="font-medium">Status:</span>
									{#if verifyResult.valid}
										<span class="text-green-600 font-medium">Valid - No tampering detected</span>
									{:else}
										<span class="text-red-600 font-medium">Invalid - Tampering detected!</span>
									{/if}
								</p>
								<p class="text-sm text-gray-700">
									<span class="font-medium">Entries Verified:</span> {verifyResult.entriesVerified}
								</p>
								{#if verifyResult.firstEntryId}
									<p class="text-sm text-gray-700">
										<span class="font-medium">First Entry:</span> {verifyResult.firstEntryId}
									</p>
								{/if}
								{#if verifyResult.lastEntryId}
									<p class="text-sm text-gray-700">
										<span class="font-medium">Last Entry:</span> {verifyResult.lastEntryId}
									</p>
								{/if}
								{#if !verifyResult.valid && verifyResult.errors && verifyResult.errors.length > 0}
									<div class="mt-3">
										<p class="text-sm font-medium text-red-600 mb-2">Errors:</p>
										<div class="bg-red-50 rounded p-3 max-h-40 overflow-y-auto">
											{#each verifyResult.errors as error}
												<p class="text-xs text-red-800 mb-1">{error}</p>
											{/each}
										</div>
									</div>
								{/if}
							</div>
						</div>
					</div>
				</div>
				<div class="bg-gray-50 px-4 py-3 sm:flex sm:flex-row-reverse sm:px-6">
					<button
						type="button"
						onclick={() => showVerifyModal = false}
						class="inline-flex w-full justify-center rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-blue-600 sm:ml-3 sm:w-auto"
					>
						Close
					</button>
				</div>
			</div>
		</div>
	</div>
{/if}
