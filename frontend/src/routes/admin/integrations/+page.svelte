<script lang="ts">
	import { onMount } from 'svelte';
	import { get } from '$lib/utils/api';
	import { addToast } from '$lib/stores/toast.svelte';
	import type { SalesforceConfig } from '$lib/types/salesforce';

	let salesforceStatus = $state<string>('loading');
	let gmailConnected = $state<boolean | null>(null);
	let isLoading = $state(true);

	onMount(async () => {
		try {
			const [sfStatus, gmailStatus] = await Promise.allSettled([
				get<SalesforceConfig>('/salesforce/status'),
				get<{ connected: boolean }>('/gmail/status')
			]);

			if (sfStatus.status === 'fulfilled') {
				salesforceStatus = sfStatus.value.status;
			} else {
				salesforceStatus = 'not_configured';
			}

			if (gmailStatus.status === 'fulfilled') {
				gmailConnected = gmailStatus.value.connected;
			} else {
				gmailConnected = false;
			}
		} catch (e) {
			salesforceStatus = 'not_configured';
			gmailConnected = false;
		} finally {
			isLoading = false;
		}
	});

	function getSalesforceBadge(status: string): { text: string; color: string } {
		switch (status) {
			case 'connected':
				return { text: 'Connected', color: 'bg-green-100 text-green-800' };
			case 'configured':
				return { text: 'Configured', color: 'bg-yellow-100 text-yellow-800' };
			case 'expired':
				return { text: 'Expired', color: 'bg-red-100 text-red-800' };
			case 'not_configured':
				return { text: 'Not Configured', color: 'bg-gray-100 text-gray-800' };
			default:
				return { text: 'Loading...', color: 'bg-gray-100 text-gray-800' };
		}
	}

	const sfBadge = $derived(getSalesforceBadge(salesforceStatus));
	const gmailBadge = $derived(
		gmailConnected === null
			? { text: 'Loading...', color: 'bg-gray-100 text-gray-800' }
			: gmailConnected
			? { text: 'Connected', color: 'bg-green-100 text-green-800' }
			: { text: 'Not Connected', color: 'bg-gray-100 text-gray-800' }
	);
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<h1 class="text-2xl font-bold text-gray-900">Integrations</h1>
		<a href="/admin" class="text-sm text-blue-600 hover:text-blue-800">
			&larr; Back to Admin
		</a>
	</div>

	<p class="text-sm text-gray-600">
		Connect external systems to sync data and automate workflows.
	</p>

	<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
		<!-- Salesforce Integration Card -->
		<a
			href="/admin/integrations/salesforce"
			class="bg-white shadow rounded-lg p-6 hover:shadow-md transition-shadow border-l-4 border-blue-500"
		>
			<div class="flex items-start">
				<div class="flex-shrink-0">
					<!-- Salesforce cloud icon -->
					<div class="h-12 w-12 bg-blue-500 rounded-lg flex items-center justify-center">
						<svg class="h-8 w-8 text-white" fill="currentColor" viewBox="0 0 24 24">
							<path d="M11.5 3a5.5 5.5 0 0 1 4.9 3h.1a4.5 4.5 0 0 1 0 9h-10a4.5 4.5 0 0 1-.4-9 5.5 5.5 0 0 1 5.4-3z"/>
						</svg>
					</div>
				</div>
				<div class="ml-4 flex-1">
					<div class="flex items-center justify-between">
						<h3 class="text-lg font-medium text-gray-900">Salesforce</h3>
						{#if !isLoading}
							<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium {sfBadge.color}">
								{sfBadge.text}
							</span>
						{/if}
					</div>
					<p class="mt-1 text-sm text-gray-500">
						Sync merge instructions to Salesforce for seamless data integration
					</p>
					<div class="mt-3 text-sm text-blue-600 font-medium">
						Configure &rarr;
					</div>
				</div>
			</div>
		</a>

		<!-- Gmail Integration Card -->
		<a
			href="/admin/integrations/gmail"
			class="bg-white shadow rounded-lg p-6 hover:shadow-md transition-shadow border-l-4 border-red-500"
		>
			<div class="flex items-start">
				<div class="flex-shrink-0">
					<!-- Gmail envelope icon -->
					<div class="h-12 w-12 bg-red-500 rounded-lg flex items-center justify-center">
						<svg class="h-7 w-7 text-white" fill="currentColor" viewBox="0 0 24 24">
							<path d="M20 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm0 4l-8 5-8-5V6l8 5 8-5v2z"/>
						</svg>
					</div>
				</div>
				<div class="ml-4 flex-1">
					<div class="flex items-center justify-between">
						<h3 class="text-lg font-medium text-gray-900">Gmail</h3>
						{#if !isLoading}
							<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium {gmailBadge.color}">
								{gmailBadge.text}
							</span>
						{/if}
					</div>
					<p class="mt-1 text-sm text-gray-500">
						Connect your Gmail account to send emails from sequences
					</p>
					<div class="mt-3 text-sm text-red-600 font-medium">
						Configure &rarr;
					</div>
				</div>
			</div>
		</a>
	</div>
</div>
