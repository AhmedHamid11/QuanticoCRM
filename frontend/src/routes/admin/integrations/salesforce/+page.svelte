<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { get, post, put, del } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import type { SalesforceConfig, SalesforceConfigInput, SyncJob, SyncJobListResponse } from '$lib/types/salesforce';

	let config = $state<SalesforceConfig | null>(null);
	let jobs = $state<SyncJob[]>([]);
	let isLoading = $state(true);
	let isSaving = $state(false);
	let isTriggeringDelivery = $state(false);
	let isTestingConnection = $state(false);

	// Form fields
	let clientId = $state('');
	let clientSecret = $state('');
	let redirectUrl = $state('');

	onMount(async () => {
		// Handle OAuth callback
		const params = new URLSearchParams(window.location.search);
		if (params.get('status') === 'connected') {
			toast.success('Successfully connected to Salesforce');
			goto('/admin/integrations/salesforce', { replaceState: true });
		}
		if (params.get('status') === 'error') {
			toast.error(params.get('message') || 'Failed to connect to Salesforce');
			goto('/admin/integrations/salesforce', { replaceState: true });
		}

		// Set default redirect URL
		if (typeof window !== 'undefined') {
			redirectUrl = `${window.location.origin}/api/v1/salesforce/oauth/callback`;
		}

		await loadData();
	});

	async function loadData() {
		try {
			// Load configuration
			try {
				const configData = await get<SalesforceConfig>('/salesforce/config');
				config = configData;
				clientId = configData.clientId || '';
				redirectUrl = configData.redirectUrl || redirectUrl;
			} catch (e) {
				// Config doesn't exist yet, that's okay
				config = {
					clientId: '',
					redirectUrl: redirectUrl,
					instanceUrl: '',
					isEnabled: false,
					status: 'not_configured',
					connectedAt: null
				};
			}

			// Load status
			try {
				const statusData = await get<SalesforceConfig>('/salesforce/status');
				if (config) {
					config.status = statusData.status;
					config.connectedAt = statusData.connectedAt;
					config.instanceUrl = statusData.instanceUrl;
					config.isEnabled = statusData.isEnabled;
				}
			} catch (e) {
				// Status endpoint may not exist if not configured
			}

			// Load recent jobs
			try {
				const jobsData = await get<SyncJobListResponse>('/salesforce/jobs?limit=10');
				jobs = jobsData.jobs || [];
			} catch (e) {
				// Jobs may not exist yet
				jobs = [];
			}
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to load data';
			toast.error(message);
		} finally {
			isLoading = false;
		}
	}

	async function saveConfiguration() {
		if (!clientId || !clientSecret || !redirectUrl) {
			toast.error('Please fill in all fields');
			return;
		}

		isSaving = true;
		try {
			const input: SalesforceConfigInput = {
				clientId,
				clientSecret,
				redirectUrl
			};
			await post<SalesforceConfig>('/salesforce/config', input);
			toast.success('Configuration saved successfully');
			await loadData();
			// Clear the secret field after saving
			clientSecret = '';
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to save configuration';
			toast.error(message);
		} finally {
			isSaving = false;
		}
	}

	async function connectToSalesforce() {
		try {
			const response = await post<{ authUrl: string }>('/salesforce/oauth/authorize', {});
			if (response.authUrl) {
				window.location.href = response.authUrl;
			} else {
				toast.error('Failed to generate authorization URL');
			}
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to initiate OAuth flow';
			toast.error(message);
		}
	}

	async function disconnect() {
		if (!confirm('Are you sure you want to disconnect from Salesforce? This will disable sync.')) {
			return;
		}

		try {
			await post('/salesforce/disconnect', {});
			toast.success('Disconnected from Salesforce');
			await loadData();
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to disconnect';
			toast.error(message);
		}
	}

	async function toggleSync() {
		try {
			const newEnabled = !config?.isEnabled;
			await put<SalesforceConfig>('/salesforce/toggle', { enabled: newEnabled });
			toast.success(newEnabled ? 'Sync enabled' : 'Sync disabled');
			await loadData();
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to toggle sync';
			toast.error(message);
		}
	}

	async function triggerDelivery() {
		if (!confirm('Trigger manual delivery of all pending merge instructions to Salesforce?')) {
			return;
		}

		isTriggeringDelivery = true;
		try {
			await post('/salesforce/trigger', { all: true });
			toast.success('Delivery triggered successfully');
			await loadData();
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to trigger delivery';
			toast.error(message);
		} finally {
			isTriggeringDelivery = false;
		}
	}

	async function retryJob(jobId: string) {
		try {
			await post(`/salesforce/jobs/${jobId}/retry`, {});
			toast.success('Job retry initiated');
			await loadData();
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to retry job';
			toast.error(message);
		}
	}

	async function testConnection() {
		isTestingConnection = true;
		try {
			const response = await get<{ status: string }>('/salesforce/status');
			if (response.status === 'connected') {
				toast.success('Connection test passed - Salesforce is connected');
			} else if (response.status === 'expired') {
				toast.error('Connection test failed - token expired, reconnect required');
			} else {
				toast.error(`Connection test result: ${response.status}`);
			}
			// Refresh config to update status display
			await loadData();
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Connection test failed';
			toast.error(message);
		} finally {
			isTestingConnection = false;
		}
	}

	function getStatusDisplay(status: string): { text: string; dotColor: string; description: string } {
		switch (status) {
			case 'connected':
				return {
					text: 'Connected',
					dotColor: 'bg-green-500',
					description: config?.connectedAt ? `Connected on ${new Date(config.connectedAt).toLocaleDateString()}` : 'Connected to Salesforce'
				};
			case 'configured':
				return {
					text: 'Configured',
					dotColor: 'bg-yellow-500',
					description: 'Click Connect to authorize with Salesforce'
				};
			case 'expired':
				return {
					text: 'Expired',
					dotColor: 'bg-red-500',
					description: 'Token expired, reconnect required'
				};
			case 'not_configured':
			default:
				return {
					text: 'Not Configured',
					dotColor: 'bg-gray-400',
					description: 'Enter Connected App details above'
				};
		}
	}

	function getJobStatusBadge(status: string): { text: string; color: string } {
		switch (status) {
			case 'completed':
				return { text: 'Completed', color: 'bg-green-100 text-green-800' };
			case 'pending':
				return { text: 'Pending', color: 'bg-yellow-100 text-yellow-800' };
			case 'running':
				return { text: 'Running', color: 'bg-blue-100 text-blue-800' };
			case 'failed':
				return { text: 'Failed', color: 'bg-red-100 text-red-800' };
			default:
				return { text: status, color: 'bg-gray-100 text-gray-800' };
		}
	}

	const statusDisplay = $derived(getStatusDisplay(config?.status || 'not_configured'));
	const isConnected = $derived(config?.status === 'connected');
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<h1 class="text-2xl font-bold text-gray-900">Salesforce Integration</h1>
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
		<!-- Section 1: Connection Configuration -->
		<div class="bg-white shadow rounded-lg p-6">
			<h2 class="text-lg font-medium text-gray-900 mb-4">Salesforce Connected App</h2>
			<p class="text-sm text-gray-600 mb-4">
				Configure your Salesforce Connected App credentials. You'll need to create a Connected App in Salesforce Setup first.
			</p>

			<div class="space-y-4">
				<div>
					<label for="clientId" class="block text-sm font-medium text-gray-700 mb-1">
						Client ID <span class="text-red-500">*</span>
					</label>
					<input
						type="text"
						id="clientId"
						bind:value={clientId}
						placeholder="Enter Client ID from Salesforce Connected App"
						class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
					/>
				</div>

				<div>
					<label for="clientSecret" class="block text-sm font-medium text-gray-700 mb-1">
						Client Secret <span class="text-red-500">*</span>
					</label>
					<input
						type="password"
						id="clientSecret"
						bind:value={clientSecret}
						placeholder="Enter Client Secret from Salesforce Connected App"
						class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
					/>
					<p class="mt-1 text-xs text-gray-500">Secret is only required when saving configuration</p>
				</div>

				<div>
					<label for="redirectUrl" class="block text-sm font-medium text-gray-700 mb-1">
						Redirect URL <span class="text-red-500">*</span>
					</label>
					<input
						type="text"
						id="redirectUrl"
						bind:value={redirectUrl}
						class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
					/>
					<p class="mt-1 text-xs text-gray-500">Use this URL in your Salesforce Connected App callback settings</p>
				</div>

				<div class="flex justify-end">
					<button
						onclick={saveConfiguration}
						disabled={isSaving}
						class="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-600/90 disabled:opacity-50 disabled:cursor-not-allowed"
					>
						{isSaving ? 'Saving...' : 'Save Configuration'}
					</button>
				</div>
			</div>
		</div>

		<!-- Section 2: Connection Status & OAuth -->
		<div class="bg-white shadow rounded-lg p-6">
			<h2 class="text-lg font-medium text-gray-900 mb-4">Connection Status</h2>

			<div class="flex items-start space-x-3 mb-6">
				<div class="flex-shrink-0">
					<div class="w-3 h-3 rounded-full {statusDisplay.dotColor} mt-1"></div>
				</div>
				<div>
					<p class="text-sm font-medium text-gray-900">{statusDisplay.text}</p>
					<p class="text-sm text-gray-500">{statusDisplay.description}</p>
					{#if config?.instanceUrl}
						<p class="text-xs text-gray-400 mt-1">Instance: {config.instanceUrl}</p>
					{/if}
				</div>
			</div>

			<div class="flex items-center space-x-3">
				{#if !isConnected}
					<button
						onclick={connectToSalesforce}
						disabled={config?.status === 'not_configured'}
						class="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-600/90 disabled:opacity-50 disabled:cursor-not-allowed"
					>
						Connect to Salesforce
					</button>
				{:else}
					<button
						onclick={disconnect}
						class="px-4 py-2 text-sm font-medium text-white bg-red-600 rounded-md hover:bg-red-600/90"
					>
						Disconnect
					</button>
					<button
						onclick={testConnection}
						disabled={isTestingConnection}
						class="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
					>
						{isTestingConnection ? 'Testing...' : 'Test Connection'}
					</button>
				{/if}

				{#if isConnected}
					<label class="flex items-center cursor-pointer">
						<input
							type="checkbox"
							checked={config?.isEnabled}
							onchange={toggleSync}
							class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
						/>
						<span class="ml-2 text-sm text-gray-700">Enable Sync</span>
					</label>
				{/if}
			</div>
		</div>

		<!-- Section 3: Delivery Management -->
		<div class="bg-white shadow rounded-lg p-6">
			<h2 class="text-lg font-medium text-gray-900 mb-4">Merge Instruction Delivery</h2>

			{#if isConnected}
				<div class="mb-6">
					<button
						onclick={triggerDelivery}
						disabled={isTriggeringDelivery}
						class="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-600/90 disabled:opacity-50 disabled:cursor-not-allowed"
					>
						{isTriggeringDelivery ? 'Triggering...' : 'Trigger Delivery'}
					</button>
					<p class="mt-2 text-sm text-gray-500">
						Manually trigger delivery of all pending merge instructions to Salesforce
					</p>
					<div class="mt-4 pt-4 border-t border-gray-200">
						<a
							href="/admin/audit-logs?eventTypes=SALESFORCE_MERGE_DELIVERY,SALESFORCE_MERGE_DELIVERY_ERROR,SALESFORCE_MERGE_DELIVERY_RETRY"
							class="text-sm text-blue-600 hover:text-blue-800"
						>
							View Salesforce Delivery Audit Logs &rarr;
						</a>
					</div>
				</div>
			{/if}

			<h3 class="text-sm font-medium text-gray-900 mb-3">Recent Sync Jobs</h3>

			{#if jobs.length === 0}
				<p class="text-sm text-gray-500">No sync jobs yet</p>
			{:else}
				<div class="overflow-x-auto">
					<table class="min-w-full divide-y divide-gray-200">
						<thead>
							<tr>
								<th class="px-3 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Batch ID</th>
								<th class="px-3 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Entity</th>
								<th class="px-3 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Status</th>
								<th class="px-3 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Instructions</th>
								<th class="px-3 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Created</th>
								<th class="px-3 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Actions</th>
							</tr>
						</thead>
						<tbody class="divide-y divide-gray-200">
							{#each jobs as job (job.id)}
								{@const badge = getJobStatusBadge(job.status)}
								<tr>
									<td class="px-3 py-4 whitespace-nowrap text-sm font-mono text-gray-900">
										{job.batchId}
									</td>
									<td class="px-3 py-4 whitespace-nowrap text-sm text-gray-900">
										{job.entityType}
									</td>
									<td class="px-3 py-4 whitespace-nowrap">
										<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium {badge.color}">
											{badge.text}
										</span>
									</td>
									<td class="px-3 py-4 whitespace-nowrap text-sm text-gray-500">
										{job.deliveredInstructions}/{job.totalInstructions}
										{#if job.failedInstructions > 0}
											<span class="text-red-600">({job.failedInstructions} failed)</span>
										{/if}
									</td>
									<td class="px-3 py-4 whitespace-nowrap text-sm text-gray-500">
										{new Date(job.createdAt).toLocaleString()}
									</td>
									<td class="px-3 py-4 whitespace-nowrap text-sm">
										{#if job.status === 'failed'}
											<button
												onclick={() => retryJob(job.id)}
												class="text-blue-600 hover:text-blue-800"
											>
												Retry
											</button>
										{/if}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</div>
	{/if}
</div>
