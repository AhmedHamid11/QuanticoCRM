<script lang="ts">
	import { onMount } from 'svelte';
	import { get, post, del } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import type {
		APITokenListItem,
		APITokenCreateInput,
		APITokenCreateResponse,
		APITokenListResponse
	} from '$lib/types/api-token';
	import { TOKEN_SCOPES } from '$lib/types/api-token';

	let tokens = $state<APITokenListItem[]>([]);
	let loading = $state(true);

	// Create modal state
	let showCreateModal = $state(false);
	let creating = $state(false);
	let newToken = $state<APITokenCreateInput>({
		name: '',
		scopes: [TOKEN_SCOPES.READ, TOKEN_SCOPES.WRITE],
		expiresIn: null
	});

	// Token created modal state (shows the token once)
	let showTokenCreatedModal = $state(false);
	let createdToken = $state<APITokenCreateResponse | null>(null);
	let tokenCopied = $state(false);

	async function loadTokens() {
		try {
			loading = true;
			const response = await get<APITokenListResponse>('/api-tokens');
			tokens = response.tokens || [];
		} catch (err) {
			toast.error('Failed to load API tokens');
		} finally {
			loading = false;
		}
	}

	async function createToken() {
		if (!newToken.name.trim()) {
			toast.error('Token name is required');
			return;
		}

		try {
			creating = true;
			const response = await post<APITokenCreateResponse>('/api-tokens', {
				name: newToken.name.trim(),
				scopes: newToken.scopes,
				expiresIn: newToken.expiresIn
			});

			// Add to list (without the full token)
			tokens = [
				...tokens,
				{
					id: response.id,
					name: response.name,
					tokenPrefix: response.token.substring(0, 12),
					scopes: response.scopes,
					lastUsedAt: null,
					expiresAt: response.expiresAt,
					isActive: true,
					createdAt: response.createdAt,
					createdBy: ''
				}
			];

			// Show the token (only time it's visible!)
			createdToken = response;
			showCreateModal = false;
			showTokenCreatedModal = true;
			tokenCopied = false;

			// Reset form
			newToken = {
				name: '',
				scopes: [TOKEN_SCOPES.READ, TOKEN_SCOPES.WRITE],
				expiresIn: null
			};

			toast.success('API token created');
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to create token');
		} finally {
			creating = false;
		}
	}

	async function revokeToken(id: string, name: string) {
		if (!confirm(`Are you sure you want to revoke "${name}"? This cannot be undone.`)) return;

		const backup = [...tokens];
		tokens = tokens.map((t) => (t.id === id ? { ...t, isActive: false } : t));

		try {
			await post(`/api-tokens/${id}/revoke`, {});
			toast.success('Token revoked');
		} catch (err) {
			tokens = backup;
			toast.error('Failed to revoke token');
		}
	}

	async function deleteToken(id: string, name: string) {
		if (!confirm(`Are you sure you want to permanently delete "${name}"?`)) return;

		const backup = [...tokens];
		tokens = tokens.filter((t) => t.id !== id);

		try {
			await del(`/api-tokens/${id}`);
			toast.success('Token deleted');
		} catch (err) {
			tokens = backup;
			toast.error('Failed to delete token');
		}
	}

	async function copyToken() {
		if (!createdToken) return;

		try {
			await navigator.clipboard.writeText(createdToken.token);
			tokenCopied = true;
			toast.success('Token copied to clipboard');
		} catch (err) {
			toast.error('Failed to copy token');
		}
	}

	function closeTokenCreatedModal() {
		showTokenCreatedModal = false;
		createdToken = null;
		tokenCopied = false;
	}

	function formatDate(dateStr: string | null): string {
		if (!dateStr) return 'Never';
		return new Date(dateStr).toLocaleDateString('en-US', {
			month: 'short',
			day: 'numeric',
			year: 'numeric'
		});
	}

	function formatDateTime(dateStr: string | null): string {
		if (!dateStr) return 'Never';
		return new Date(dateStr).toLocaleString('en-US', {
			month: 'short',
			day: 'numeric',
			year: 'numeric',
			hour: 'numeric',
			minute: '2-digit'
		});
	}

	function toggleScope(scope: string) {
		if (newToken.scopes?.includes(scope)) {
			newToken.scopes = newToken.scopes.filter((s) => s !== scope);
		} else {
			newToken.scopes = [...(newToken.scopes || []), scope];
		}
	}

	onMount(() => {
		loadTokens();
	});
</script>

<div class="space-y-6">
	<!-- Header -->
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold text-gray-900">API Tokens</h1>
			<p class="mt-1 text-sm text-gray-500">
				Generate tokens for programmatic API access to your organization's data
			</p>
		</div>
		<button
			onclick={() => (showCreateModal = true)}
			class="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-blue-600 hover:bg-blue-600/90"
		>
			<svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
			</svg>
			Generate Token
		</button>
	</div>

	<!-- Security Notice -->
	<div class="bg-amber-50 border border-amber-200 rounded-lg p-4">
		<div class="flex">
			<svg class="h-5 w-5 text-amber-400 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path
					stroke-linecap="round"
					stroke-linejoin="round"
					stroke-width="2"
					d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
				/>
			</svg>
			<div class="ml-3">
				<h3 class="text-sm font-medium text-amber-800">Security Notice</h3>
				<p class="mt-1 text-sm text-amber-700">
					API tokens provide access to your organization's data. Keep them secure and never share
					them publicly. Tokens are only shown once when created.
				</p>
			</div>
		</div>
	</div>

	<!-- Tokens List -->
	{#if loading}
		<div class="flex items-center justify-center py-12">
			<svg
				class="animate-spin h-8 w-8 text-blue-600"
				xmlns="http://www.w3.org/2000/svg"
				fill="none"
				viewBox="0 0 24 24"
			>
				<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"
				></circle>
				<path
					class="opacity-75"
					fill="currentColor"
					d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
				></path>
			</svg>
		</div>
	{:else if tokens.length === 0}
		<div class="text-center py-12 bg-gray-50 rounded-lg border-2 border-dashed border-gray-300">
			<svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path
					stroke-linecap="round"
					stroke-linejoin="round"
					stroke-width="2"
					d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"
				/>
			</svg>
			<h3 class="mt-2 text-sm font-medium text-gray-900">No API tokens</h3>
			<p class="mt-1 text-sm text-gray-500">Get started by generating a new token.</p>
			<button
				onclick={() => (showCreateModal = true)}
				class="mt-4 inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-blue-600 hover:bg-blue-600/90"
			>
				Generate Token
			</button>
		</div>
	{:else}
		<div class="crm-card overflow-hidden">
			<table class="min-w-full divide-y divide-gray-200">
				<thead class="bg-gray-50">
					<tr>
						<th
							scope="col"
							class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
							>Token</th
						>
						<th
							scope="col"
							class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
							>Scopes</th
						>
						<th
							scope="col"
							class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
							>Last Used</th
						>
						<th
							scope="col"
							class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
							>Expires</th
						>
						<th
							scope="col"
							class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
							>Status</th
						>
						<th scope="col" class="relative px-6 py-3">
							<span class="sr-only">Actions</span>
						</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-200">
					{#each tokens as token (token.id)}
						<tr class="hover:bg-gray-50">
							<td class="px-6 py-4 whitespace-nowrap">
								<div class="flex items-center">
									<div
										class="flex-shrink-0 h-10 w-10 rounded-full bg-indigo-100 flex items-center justify-center"
									>
										<svg class="h-5 w-5 text-indigo-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
											<path
												stroke-linecap="round"
												stroke-linejoin="round"
												stroke-width="2"
												d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"
											/>
										</svg>
									</div>
									<div class="ml-4">
										<div class="text-sm font-medium text-gray-900">{token.name}</div>
										<div class="text-sm text-gray-500 font-mono">{token.tokenPrefix}...</div>
									</div>
								</div>
							</td>
							<td class="px-6 py-4 whitespace-nowrap">
								<div class="flex gap-1">
									{#each token.scopes as scope}
										<span
											class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium {scope ===
											'write'
												? 'bg-orange-100 text-orange-800'
												: 'bg-blue-100 text-blue-800'}"
										>
											{scope}
										</span>
									{/each}
								</div>
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{formatDateTime(token.lastUsedAt)}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{formatDate(token.expiresAt)}
							</td>
							<td class="px-6 py-4 whitespace-nowrap">
								{#if token.isActive}
									<span
										class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800"
									>
										Active
									</span>
								{:else}
									<span
										class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800"
									>
										Revoked
									</span>
								{/if}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
								{#if token.isActive}
									<button
										onclick={() => revokeToken(token.id, token.name)}
										class="text-amber-600 hover:text-amber-900 mr-4"
									>
										Revoke
									</button>
								{/if}
								<button
									onclick={() => deleteToken(token.id, token.name)}
									class="text-red-600 hover:text-red-900"
								>
									Delete
								</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>

<!-- Create Token Modal -->
{#if showCreateModal}
	<div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
		<div class="bg-white rounded-lg shadow-xl w-full max-w-md p-6">
			<div class="flex items-center justify-between mb-4">
				<h3 class="text-lg font-medium text-gray-900">Generate API Token</h3>
				<button onclick={() => (showCreateModal = false)} class="text-gray-400 hover:text-gray-500">
					<svg class="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="2"
							d="M6 18L18 6M6 6l12 12"
						/>
					</svg>
				</button>
			</div>

			<form
				onsubmit={(e) => {
					e.preventDefault();
					createToken();
				}}
				class="space-y-4"
			>
				<div>
					<label for="name" class="block text-sm font-medium text-gray-700"
						>Token Name <span class="text-red-500">*</span></label
					>
					<input
						type="text"
						id="name"
						bind:value={newToken.name}
						class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
						placeholder="e.g., Production Integration"
						required
					/>
					<p class="mt-1 text-xs text-gray-500">A descriptive name to identify this token</p>
				</div>

				<div>
					<label class="block text-sm font-medium text-gray-700 mb-2">Permissions</label>
					<div class="space-y-2">
						<label class="flex items-center">
							<input
								type="checkbox"
								checked={newToken.scopes?.includes(TOKEN_SCOPES.READ)}
								onchange={() => toggleScope(TOKEN_SCOPES.READ)}
								class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
							/>
							<span class="ml-2 text-sm text-gray-700">Read</span>
							<span class="ml-2 text-xs text-gray-500">- View data</span>
						</label>
						<label class="flex items-center">
							<input
								type="checkbox"
								checked={newToken.scopes?.includes(TOKEN_SCOPES.WRITE)}
								onchange={() => toggleScope(TOKEN_SCOPES.WRITE)}
								class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
							/>
							<span class="ml-2 text-sm text-gray-700">Write</span>
							<span class="ml-2 text-xs text-gray-500">- Create, update, delete data</span>
						</label>
					</div>
				</div>

				<div>
					<label for="expires" class="block text-sm font-medium text-gray-700">Expiration</label>
					<select
						id="expires"
						bind:value={newToken.expiresIn}
						class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
					>
						<option value={null}>Never expires</option>
						<option value={7}>7 days</option>
						<option value={30}>30 days</option>
						<option value={90}>90 days</option>
						<option value={180}>180 days</option>
						<option value={365}>1 year</option>
					</select>
				</div>

				<div class="flex justify-end gap-3 pt-4">
					<button
						type="button"
						onclick={() => (showCreateModal = false)}
						class="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-50"
					>
						Cancel
					</button>
					<button
						type="submit"
						disabled={creating || !newToken.scopes?.length}
						class="px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-600/90 disabled:opacity-50 disabled:cursor-not-allowed"
					>
						{creating ? 'Generating...' : 'Generate Token'}
					</button>
				</div>
			</form>
		</div>
	</div>
{/if}

<!-- Token Created Modal (shows the token once) -->
{#if showTokenCreatedModal && createdToken}
	<div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
		<div class="bg-white rounded-lg shadow-xl w-full max-w-lg p-6">
			<div class="flex items-center justify-between mb-4">
				<div class="flex items-center">
					<div class="flex-shrink-0 h-10 w-10 rounded-full bg-green-100 flex items-center justify-center">
						<svg class="h-6 w-6 text-green-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path
								stroke-linecap="round"
								stroke-linejoin="round"
								stroke-width="2"
								d="M5 13l4 4L19 7"
							/>
						</svg>
					</div>
					<h3 class="ml-3 text-lg font-medium text-gray-900">Token Created</h3>
				</div>
				<button onclick={closeTokenCreatedModal} class="text-gray-400 hover:text-gray-500">
					<svg class="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="2"
							d="M6 18L18 6M6 6l12 12"
						/>
					</svg>
				</button>
			</div>

			<div class="bg-amber-50 border border-amber-200 rounded-lg p-4 mb-4">
				<div class="flex">
					<svg class="h-5 w-5 text-amber-400 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="2"
							d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
						/>
					</svg>
					<div class="ml-3">
						<p class="text-sm font-medium text-amber-800">Copy this token now!</p>
						<p class="text-sm text-amber-700">
							This is the only time it will be shown. Store it securely.
						</p>
					</div>
				</div>
			</div>

			<div class="space-y-4">
				<div>
					<label class="block text-sm font-medium text-gray-700 mb-1">Your API Token</label>
					<div class="flex">
						<input
							type="text"
							readonly
							value={createdToken.token}
							class="flex-1 block w-full rounded-l-md border-gray-300 bg-gray-50 font-mono text-sm"
						/>
						<button
							onclick={copyToken}
							class="px-4 py-2 border border-l-0 border-gray-300 rounded-r-md bg-gray-50 hover:bg-gray-100 text-sm font-medium text-gray-700"
						>
							{tokenCopied ? 'Copied!' : 'Copy'}
						</button>
					</div>
				</div>

				<div class="grid grid-cols-2 gap-4 text-sm">
					<div>
						<span class="text-gray-500">Name:</span>
						<span class="ml-2 text-gray-900">{createdToken.name}</span>
					</div>
					<div>
						<span class="text-gray-500">Expires:</span>
						<span class="ml-2 text-gray-900">{formatDate(createdToken.expiresAt)}</span>
					</div>
					<div class="col-span-2">
						<span class="text-gray-500">Scopes:</span>
						<span class="ml-2">
							{#each createdToken.scopes as scope}
								<span
									class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium {scope ===
									'write'
										? 'bg-orange-100 text-orange-800'
										: 'bg-blue-100 text-blue-800'} mr-1"
								>
									{scope}
								</span>
							{/each}
						</span>
					</div>
				</div>

				<div class="bg-gray-50 rounded-lg p-4 mt-4">
					<p class="text-sm font-medium text-gray-700 mb-2">Usage Example</p>
					<pre
						class="text-xs bg-gray-800 text-gray-100 rounded p-3 overflow-x-auto"><code>curl -H "Authorization: Bearer {createdToken.token.substring(
							0,
							20
						)}..." \
     https://your-api.com/api/v1/contacts</code></pre>
				</div>
			</div>

			<div class="flex justify-end mt-6">
				<button
					onclick={closeTokenCreatedModal}
					class="px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-600/90"
				>
					Done
				</button>
			</div>
		</div>
	</div>
{/if}
