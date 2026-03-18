<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { get, del } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';

	// Connection status shape returned by GET /gmail/status
	interface GmailStatus {
		connected: boolean;
		gmailAddress?: string;
		connectedAt?: string;
		dnsspfValid: number;
		dnsdkimValid: number;
		dnsdmarcValid: number;
	}

	let status = $state<GmailStatus | null>(null);
	let isLoading = $state(true);
	let isConnecting = $state(false);
	let isDisconnecting = $state(false);

	onMount(async () => {
		// Handle OAuth callback query params
		const params = new URLSearchParams(window.location.search);
		if (params.get('connected') === 'true') {
			toast.success('Gmail connected successfully');
			goto('/admin/integrations/gmail', { replaceState: true });
		}
		const errorParam = params.get('error');
		if (errorParam) {
			const errorMessages: Record<string, string> = {
				missing_code: 'Authorization failed: missing code from Google',
				missing_state: 'Authorization failed: missing state parameter',
				invalid_state: 'Authorization failed: invalid state token',
				connection_failed: 'Failed to complete Gmail connection',
				tenant_resolution_failed: 'Failed to resolve account — please try again'
			};
			toast.error(errorMessages[errorParam] ?? `Connection failed: ${errorParam}`);
			goto('/admin/integrations/gmail', { replaceState: true });
		}

		await loadStatus();
	});

	async function loadStatus() {
		try {
			status = await get<GmailStatus>('/gmail/status');
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to load Gmail status';
			toast.error(message);
			status = { connected: false, dnsspfValid: 0, dnsdkimValid: 0, dnsdmarcValid: 0 };
		} finally {
			isLoading = false;
		}
	}

	async function connectGmail() {
		isConnecting = true;
		try {
			const response = await get<{ authUrl: string }>('/gmail/connect');
			if (response.authUrl) {
				window.location.href = response.authUrl;
			} else {
				toast.error('Failed to generate authorization URL');
			}
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to initiate Gmail connection';
			toast.error(message);
		} finally {
			isConnecting = false;
		}
	}

	async function disconnectGmail() {
		if (!confirm('Are you sure you want to disconnect Gmail? Sequences using this account will stop sending.')) {
			return;
		}

		isDisconnecting = true;
		try {
			await del('/gmail/disconnect');
			toast.success('Gmail disconnected');
			await loadStatus();
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to disconnect Gmail';
			toast.error(message);
		} finally {
			isDisconnecting = false;
		}
	}

	// DNS badge helpers
	function dnsBadge(valid: number, label: string): { label: string; color: string; icon: string; title: string } {
		if (valid === 1) {
			return {
				label,
				color: 'bg-green-100 text-green-800',
				icon: 'check',
				title: `${label} record found — sending authentication is configured`
			};
		}
		return {
			label,
			color: 'bg-yellow-100 text-yellow-800',
			icon: 'warning',
			title: `${label} record not detected — emails may be treated as spam by some recipients`
		};
	}

	const spfBadge = $derived(status ? dnsBadge(status.dnsspfValid, 'SPF') : null);
	const dkimBadge = $derived(status ? dnsBadge(status.dnsdkimValid, 'DKIM') : null);
	const dmarcBadge = $derived(status ? dnsBadge(status.dnsdmarcValid, 'DMARC') : null);

	const anyDnsWarning = $derived(
		status?.connected &&
		(status.dnsspfValid !== 1 || status.dnsdkimValid !== 1 || status.dnsdmarcValid !== 1)
	);
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<h1 class="text-2xl font-bold text-gray-900">Gmail Integration</h1>
		<a href="/admin/integrations" class="text-sm text-blue-600 hover:text-blue-800">
			&larr; Back to Integrations
		</a>
	</div>

	{#if isLoading}
		<div class="bg-white shadow rounded-lg p-6">
			<div class="animate-pulse space-y-4">
				<div class="h-4 bg-gray-200 rounded w-1/4"></div>
				<div class="h-10 bg-gray-200 rounded w-1/2"></div>
			</div>
		</div>
	{:else}
		<!-- Connection Status Section -->
		<div class="bg-white shadow rounded-lg p-6">
			<h2 class="text-lg font-medium text-gray-900 mb-1">Connection Status</h2>
			<p class="text-sm text-gray-500 mb-6">
				Connect your Gmail account to send outreach emails from sequences. Uses Google OAuth — your password is never stored.
			</p>

			{#if status?.connected}
				<!-- Connected state -->
				<div class="flex items-start space-x-3 mb-6">
					<div class="flex-shrink-0">
						<div class="w-3 h-3 rounded-full bg-green-500 mt-1.5"></div>
					</div>
					<div>
						<p class="text-sm font-medium text-gray-900">Connected</p>
						{#if status.gmailAddress}
							<p class="text-sm text-gray-500">{status.gmailAddress}</p>
						{/if}
						{#if status.connectedAt}
							<p class="text-xs text-gray-400 mt-1">
								Connected on {new Date(status.connectedAt).toLocaleDateString()}
							</p>
						{/if}
					</div>
				</div>

				<!-- DNS Validation Results -->
				<div class="mb-6">
					<h3 class="text-sm font-medium text-gray-900 mb-3">Email Authentication (DNS)</h3>
					<div class="flex items-center gap-3 flex-wrap">
						{#each [spfBadge, dkimBadge, dmarcBadge] as badge}
							{#if badge}
								<span
									class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-medium {badge.color}"
									title={badge.title}
								>
									{#if badge.icon === 'check'}
										<svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
											<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M5 13l4 4L19 7"/>
										</svg>
									{:else}
										<svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
											<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v4m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"/>
										</svg>
									{/if}
									{badge.label}
								</span>
							{/if}
						{/each}
					</div>
					{#if anyDnsWarning}
						<p class="mt-3 text-xs text-yellow-700 bg-yellow-50 border border-yellow-200 rounded p-2">
							One or more DNS authentication records are missing. Emails may be flagged as spam by some mail providers.
							Sending is not blocked — contact your IT admin to configure the missing records.
						</p>
					{/if}
				</div>

				<!-- Disconnect button -->
				<button
					onclick={disconnectGmail}
					disabled={isDisconnecting}
					class="px-4 py-2 text-sm font-medium text-white bg-red-600 rounded-md hover:bg-red-700 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					{isDisconnecting ? 'Disconnecting...' : 'Disconnect Gmail'}
				</button>
			{:else}
				<!-- Not connected state -->
				<div class="flex items-start space-x-3 mb-6">
					<div class="flex-shrink-0">
						<div class="w-3 h-3 rounded-full bg-gray-400 mt-1.5"></div>
					</div>
					<div>
						<p class="text-sm font-medium text-gray-900">Not Connected</p>
						<p class="text-sm text-gray-500">Connect your Gmail account to enable email sequences</p>
					</div>
				</div>

				<button
					onclick={connectGmail}
					disabled={isConnecting}
					class="inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-white bg-red-600 rounded-md hover:bg-red-700 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					{#if isConnecting}
						<svg class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
						</svg>
						Connecting...
					{:else}
						<svg class="h-4 w-4" fill="currentColor" viewBox="0 0 24 24">
							<path d="M20 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm0 4l-8 5-8-5V6l8 5 8-5v2z"/>
						</svg>
						Connect Gmail
					{/if}
				</button>
			{/if}
		</div>

		<!-- About section -->
		<div class="bg-white shadow rounded-lg p-6">
			<h2 class="text-lg font-medium text-gray-900 mb-3">About Gmail Integration</h2>
			<ul class="space-y-2 text-sm text-gray-600">
				<li class="flex items-start gap-2">
					<svg class="h-4 w-4 text-green-500 mt-0.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
					</svg>
					Emails are sent from your personal Gmail address — not a shared inbox
				</li>
				<li class="flex items-start gap-2">
					<svg class="h-4 w-4 text-green-500 mt-0.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
					</svg>
					Tokens are encrypted at rest using AES-256-GCM
				</li>
				<li class="flex items-start gap-2">
					<svg class="h-4 w-4 text-green-500 mt-0.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
					</svg>
					Requires Gmail or Google Workspace account
				</li>
				<li class="flex items-start gap-2">
					<svg class="h-4 w-4 text-blue-500 mt-0.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/>
					</svg>
					Gmail API scopes requested: send + metadata (read receipts in a future release)
				</li>
			</ul>
		</div>
	{/if}
</div>
