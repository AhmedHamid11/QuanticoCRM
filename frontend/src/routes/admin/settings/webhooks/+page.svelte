<script lang="ts">
	import { onMount } from 'svelte';
	import { get, put } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import type { OrgWebhookSettings, WebhookAuthType, OrgWebhookSettingsInput } from '$lib/types/tripwire';

	let settings = $state<OrgWebhookSettings | null>(null);
	let loading = $state(true);
	let saving = $state(false);

	// Form state
	let authType = $state<WebhookAuthType>('none');
	let apiKey = $state('');
	let bearerToken = $state('');
	let customHeaderName = $state('');
	let customHeaderValue = $state('');
	let timeoutMs = $state(5000);

	const AUTH_TYPES: { value: WebhookAuthType; label: string; description: string }[] = [
		{ value: 'none', label: 'No Authentication', description: 'Webhooks are sent without any authentication headers' },
		{ value: 'api_key', label: 'API Key', description: 'Send an X-API-Key header with each webhook request' },
		{ value: 'bearer', label: 'Bearer Token', description: 'Send an Authorization: Bearer header with each webhook request' },
		{ value: 'custom_header', label: 'Custom Header', description: 'Send a custom header with each webhook request' }
	];

	async function loadSettings() {
		try {
			settings = await get<OrgWebhookSettings>('/settings/webhooks');
			// Populate form state
			authType = settings.authType || 'none';
			apiKey = settings.apiKey || '';
			bearerToken = settings.bearerToken || '';
			customHeaderName = settings.customHeaderName || '';
			customHeaderValue = settings.customHeaderValue || '';
			timeoutMs = settings.timeoutMs || 5000;
		} catch (e) {
			toast.error('Failed to load webhook settings');
		} finally {
			loading = false;
		}
	}

	async function handleSubmit() {
		// Validate based on auth type
		if (authType === 'api_key' && !apiKey.trim()) {
			toast.error('API Key is required');
			return;
		}
		if (authType === 'bearer' && !bearerToken.trim()) {
			toast.error('Bearer Token is required');
			return;
		}
		if (authType === 'custom_header') {
			if (!customHeaderName.trim()) {
				toast.error('Custom Header Name is required');
				return;
			}
			if (!customHeaderValue.trim()) {
				toast.error('Custom Header Value is required');
				return;
			}
		}
		if (timeoutMs < 1000 || timeoutMs > 30000) {
			toast.error('Timeout must be between 1000ms and 30000ms');
			return;
		}

		try {
			saving = true;
			const input: OrgWebhookSettingsInput = {
				authType,
				apiKey: authType === 'api_key' ? apiKey.trim() : undefined,
				bearerToken: authType === 'bearer' ? bearerToken.trim() : undefined,
				customHeaderName: authType === 'custom_header' ? customHeaderName.trim() : undefined,
				customHeaderValue: authType === 'custom_header' ? customHeaderValue.trim() : undefined,
				timeoutMs
			};
			await put('/settings/webhooks', input);
			toast.success('Webhook settings saved');
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to save settings';
			toast.error(message);
		} finally {
			saving = false;
		}
	}

	onMount(() => {
		loadSettings();
	});
</script>

<div class="max-w-2xl mx-auto space-y-6">
	<div class="flex justify-between items-center">
		<div>
			<h1 class="text-2xl font-bold text-gray-900">Webhook Settings</h1>
			<p class="text-sm text-gray-500 mt-1">Configure authentication and timeout settings for outgoing webhooks</p>
		</div>
		<a
			href="/admin/tripwires"
			class="text-sm text-gray-600 hover:text-gray-900"
		>
			&larr; Back to Tripwires
		</a>
	</div>

	{#if loading}
		<div class="text-center py-12 text-gray-500">Loading...</div>
	{:else}
		<form onsubmit={(e) => { e.preventDefault(); handleSubmit(); }} class="space-y-6">
			<!-- Authentication -->
			<div class="bg-white shadow rounded-lg p-6 space-y-4">
				<h2 class="text-lg font-medium text-gray-900 border-b pb-2">Authentication</h2>

				<div>
					<label class="block text-sm font-medium text-gray-700 mb-3">Authentication Type</label>
					<div class="space-y-2">
						{#each AUTH_TYPES as at}
							<label class="flex items-start gap-3 p-3 border rounded-lg cursor-pointer hover:bg-gray-50 {authType === at.value ? 'border-primary bg-blue-50' : 'border-gray-200'}">
								<input
									type="radio"
									name="authType"
									value={at.value}
									bind:group={authType}
									class="mt-0.5 h-4 w-4 text-primary focus:ring-primary border-gray-300"
								/>
								<div>
									<div class="font-medium text-gray-900">{at.label}</div>
									<div class="text-sm text-gray-500">{at.description}</div>
								</div>
							</label>
						{/each}
					</div>
				</div>

				<!-- API Key Input -->
				{#if authType === 'api_key'}
					<div>
						<label for="apiKey" class="block text-sm font-medium text-gray-700">API Key</label>
						<input
							type="password"
							id="apiKey"
							bind:value={apiKey}
							class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary"
							placeholder="Enter your API key"
						/>
						<p class="mt-1 text-xs text-gray-500">This will be sent as: X-API-Key: [your-key]</p>
					</div>
				{/if}

				<!-- Bearer Token Input -->
				{#if authType === 'bearer'}
					<div>
						<label for="bearerToken" class="block text-sm font-medium text-gray-700">Bearer Token</label>
						<input
							type="password"
							id="bearerToken"
							bind:value={bearerToken}
							class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary"
							placeholder="Enter your bearer token"
						/>
						<p class="mt-1 text-xs text-gray-500">This will be sent as: Authorization: Bearer [your-token]</p>
					</div>
				{/if}

				<!-- Custom Header Inputs -->
				{#if authType === 'custom_header'}
					<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
						<div>
							<label for="customHeaderName" class="block text-sm font-medium text-gray-700">Header Name</label>
							<input
								type="text"
								id="customHeaderName"
								bind:value={customHeaderName}
								class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary"
								placeholder="X-Custom-Auth"
							/>
						</div>
						<div>
							<label for="customHeaderValue" class="block text-sm font-medium text-gray-700">Header Value</label>
							<input
								type="password"
								id="customHeaderValue"
								bind:value={customHeaderValue}
								class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary"
								placeholder="Enter header value"
							/>
						</div>
					</div>
					<p class="text-xs text-gray-500">This will be sent as: [Header-Name]: [Header-Value]</p>
				{/if}
			</div>

			<!-- Request Settings -->
			<div class="bg-white shadow rounded-lg p-6 space-y-4">
				<h2 class="text-lg font-medium text-gray-900 border-b pb-2">Request Settings</h2>

				<div>
					<label for="timeoutMs" class="block text-sm font-medium text-gray-700">Request Timeout (ms)</label>
					<input
						type="number"
						id="timeoutMs"
						bind:value={timeoutMs}
						min="1000"
						max="30000"
						step="100"
						class="mt-1 block w-full md:w-48 rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary"
					/>
					<p class="mt-1 text-xs text-gray-500">How long to wait for a response before timing out (1000-30000ms)</p>
				</div>
			</div>

			<!-- Info Box -->
			<div class="bg-blue-50 border border-blue-200 rounded-lg p-4">
				<h3 class="text-sm font-medium text-blue-800">Webhook Payload Format</h3>
				<p class="mt-1 text-sm text-blue-700">All webhook requests are sent as POST with a JSON body:</p>
				<pre class="mt-2 text-xs bg-blue-100 p-2 rounded overflow-x-auto">{`{
  "tripwireId": "0Tw_xxx",
  "event": "CREATE" | "UPDATE" | "DELETE",
  "entityType": "Contact",
  "recordId": "003_xxx"
}`}</pre>
			</div>

			<!-- Actions -->
			<div class="flex justify-end gap-3">
				<a
					href="/admin/tripwires"
					class="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-50"
				>
					Cancel
				</a>
				<button
					type="submit"
					disabled={saving}
					class="px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-black bg-primary hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					{saving ? 'Saving...' : 'Save Settings'}
				</button>
			</div>
		</form>
	{/if}
</div>
