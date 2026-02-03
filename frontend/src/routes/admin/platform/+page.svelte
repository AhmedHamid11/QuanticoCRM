<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { auth, authFetch, impersonate } from '$lib/stores/auth.svelte';

	// Types
	interface Organization {
		id: string;
		name: string;
		slug: string;
		plan: string;
		isActive: boolean;
		createdAt: string;
		modifiedAt: string;
	}

	interface OrgListResponse {
		data: Organization[];
		total: number;
		page: number;
		pageSize: number;
		totalPages: number;
	}

	// State
	let orgs = $state<Organization[]>([]);
	let total = $state(0);
	let page = $state(1);
	let pageSize = $state(20);
	let totalPages = $state(0);
	let isLoading = $state(true);
	let error = $state<string | null>(null);
	let impersonatingOrgId = $state<string | null>(null);
	let deletingOrgId = $state<string | null>(null);
	let confirmDeleteOrg = $state<Organization | null>(null);
	let updatingPlanOrgId = $state<string | null>(null);
	let showCreateModal = $state(false);
	let createOrgName = $state('');
	let createOrgSlug = $state('');
	let isCreating = $state(false);

	// Load organizations
	async function loadOrganizations() {
		isLoading = true;
		error = null;
		try {
			const response = await authFetch<OrgListResponse>(`/platform/orgs?page=${page}&pageSize=${pageSize}`);
			orgs = response.data || [];
			total = response.total;
			totalPages = response.totalPages;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load organizations';
			console.error('Failed to load organizations:', e);
		} finally {
			isLoading = false;
		}
	}

	// Handle impersonation
	async function handleImpersonate(org: Organization) {
		impersonatingOrgId = org.id;
		try {
			await impersonate({ orgId: org.id });
			// Force a full page reload to ensure all components use the new token
			window.location.href = '/';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to impersonate organization';
			console.error('Failed to impersonate:', e);
		} finally {
			impersonatingOrgId = null;
		}
	}

	// Handle plan change (upgrade/downgrade)
	async function handlePlanChange(org: Organization) {
		updatingPlanOrgId = org.id;
		const newPlan = org.plan === 'pro' ? 'free' : 'pro';
		try {
			const updated = await authFetch<Organization>(`/platform/orgs/${org.id}`, {
				method: 'PATCH',
				body: { plan: newPlan }
			});
			// Update in list
			orgs = orgs.map(o => o.id === org.id ? updated : o);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to update organization plan';
			console.error('Failed to update plan:', e);
		} finally {
			updatingPlanOrgId = null;
		}
	}

	// Handle delete
	async function handleDelete(org: Organization) {
		deletingOrgId = org.id;
		try {
			await authFetch(`/platform/orgs/${org.id}`, { method: 'DELETE' });
			// Remove from list
			orgs = orgs.filter(o => o.id !== org.id);
			total--;
			confirmDeleteOrg = null;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to delete organization';
			console.error('Failed to delete:', e);
		} finally {
			deletingOrgId = null;
		}
	}

	// Handle create organization
	async function handleCreateOrg() {
		if (!createOrgName.trim()) return;
		isCreating = true;
		error = null;
		try {
			const newOrg = await authFetch<Organization>('/platform/orgs', {
				method: 'POST',
				body: {
					name: createOrgName.trim(),
					slug: createOrgSlug.trim() || undefined
				}
			});
			orgs = [newOrg, ...orgs];
			total++;
			showCreateModal = false;
			createOrgName = '';
			createOrgSlug = '';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to create organization';
			console.error('Failed to create organization:', e);
		} finally {
			isCreating = false;
		}
	}

	// Pagination
	function goToPage(newPage: number) {
		if (newPage >= 1 && newPage <= totalPages) {
			page = newPage;
			loadOrganizations();
		}
	}

	// Track access verification
	let hasAccess = $state(false);

	// Check platform admin access - must not be impersonating
	onMount(() => {
		if (!auth.isPlatformAdmin || auth.isImpersonation) {
			goto('/admin');
			return;
		}
		hasAccess = true;
		loadOrganizations();
	});

	// Format date
	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleDateString('en-US', {
			year: 'numeric',
			month: 'short',
			day: 'numeric',
			hour: '2-digit',
			minute: '2-digit'
		});
	}
</script>

{#if !hasAccess}
	<!-- Show nothing while checking access - prevents flash of content for unauthorized users -->
	<div class="flex items-center justify-center py-12">
		<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
	</div>
{:else}
<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold text-gray-900">Platform Console</h1>
			<p class="mt-1 text-sm text-gray-500">Manage all organizations on the platform</p>
		</div>
		<div class="flex items-center space-x-2">
			<span class="inline-flex items-center px-3 py-1 rounded-full text-sm font-medium bg-purple-100 text-purple-800">
				Platform Admin
			</span>
		</div>
	</div>

	{#if error}
		<div class="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg">
			{error}
		</div>
	{/if}

	<div class="bg-white shadow rounded-lg overflow-hidden">
		<div class="px-6 py-4 border-b border-gray-200">
			<div class="flex items-center justify-between">
				<h2 class="text-lg font-medium text-gray-900">Organizations ({total})</h2>
				<div class="flex items-center space-x-2">
				<button
					onclick={() => showCreateModal = true}
					class="inline-flex items-center px-3 py-1.5 text-sm font-medium text-black bg-primary rounded-md hover:bg-primary/90"
				>
					<svg class="w-4 h-4 mr-1.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
					</svg>
					New Organization
				</button>
				<button
					onclick={() => loadOrganizations()}
					class="inline-flex items-center px-3 py-1.5 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50"
					disabled={isLoading}
				>
					{#if isLoading}
						<svg class="animate-spin -ml-0.5 mr-2 h-4 w-4 text-gray-500" fill="none" viewBox="0 0 24 24">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
						</svg>
						Loading...
					{:else}
						Refresh
					{/if}
				</button>
				</div>
			</div>
		</div>

		{#if isLoading && orgs.length === 0}
			<div class="px-6 py-12 text-center">
				<svg class="animate-spin mx-auto h-8 w-8 text-gray-400" fill="none" viewBox="0 0 24 24">
					<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
					<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
				</svg>
				<p class="mt-4 text-gray-500">Loading organizations...</p>
			</div>
		{:else if orgs.length === 0}
			<div class="px-6 py-12 text-center">
				<svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
				</svg>
				<h3 class="mt-2 text-sm font-medium text-gray-900">No organizations</h3>
				<p class="mt-1 text-sm text-gray-500">No organizations have been created yet.</p>
			</div>
		{:else}
			<div class="overflow-x-auto">
			<table class="min-w-full divide-y divide-gray-200">
				<thead class="bg-gray-50">
					<tr>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Organization
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Slug
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Plan
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Status
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Created
						</th>
						<th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
							Actions
						</th>
					</tr>
				</thead>
				<tbody class="bg-white divide-y divide-gray-200">
					{#each orgs as org (org.id)}
						<tr class="hover:bg-gray-50">
							<td class="px-6 py-4 whitespace-nowrap">
								<div class="flex items-center">
									<div class="flex-shrink-0 h-10 w-10 rounded-full bg-blue-100 flex items-center justify-center">
										<span class="text-primary font-medium text-sm">
											{org.name.slice(0, 2).toUpperCase()}
										</span>
									</div>
									<div class="ml-4">
										<div class="text-sm font-medium text-gray-900">{org.name}</div>
										<div class="text-xs text-gray-500">{org.id}</div>
									</div>
								</div>
							</td>
							<td class="px-6 py-4 whitespace-nowrap">
								<code class="text-sm text-gray-600 bg-gray-100 px-2 py-1 rounded">{org.slug}</code>
							</td>
							<td class="px-6 py-4 whitespace-nowrap">
								<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium
									{org.plan === 'free' ? 'bg-gray-100 text-gray-800' :
									 org.plan === 'pro' ? 'bg-blue-100 text-blue-800' :
									 'bg-purple-100 text-purple-800'}">
									{org.plan}
								</span>
							</td>
							<td class="px-6 py-4 whitespace-nowrap">
								{#if org.isActive}
									<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">
										Active
									</span>
								{:else}
									<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-800">
										Inactive
									</span>
								{/if}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{formatDate(org.createdAt)}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium space-x-2">
								<button
									onclick={() => handlePlanChange(org)}
									disabled={updatingPlanOrgId === org.id}
									class="inline-flex items-center px-3 py-1.5 text-sm font-medium rounded-md
										{org.plan === 'pro'
											? 'text-gray-700 bg-gray-50 hover:bg-gray-100 border border-gray-200'
											: 'text-blue-700 bg-blue-50 hover:bg-blue-100 border border-blue-200'}"
								>
									{#if updatingPlanOrgId === org.id}
										<svg class="animate-spin -ml-0.5 mr-2 h-4 w-4" fill="none" viewBox="0 0 24 24">
											<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
											<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
										</svg>
										Updating...
									{:else if org.plan === 'pro'}
										<svg class="w-4 h-4 mr-1.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
											<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 14l-7 7m0 0l-7-7m7 7V3" />
										</svg>
										Downgrade
									{:else}
										<svg class="w-4 h-4 mr-1.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
											<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 10l7-7m0 0l7 7m-7-7v18" />
										</svg>
										Upgrade
									{/if}
								</button>
								<button
									onclick={() => handleImpersonate(org)}
									disabled={impersonatingOrgId === org.id || !org.isActive}
									class="inline-flex items-center px-3 py-1.5 text-sm font-medium rounded-md
										{org.isActive
											? 'text-amber-700 bg-amber-50 hover:bg-amber-100 border border-amber-200'
											: 'text-gray-400 bg-gray-50 cursor-not-allowed border border-gray-200'}"
								>
									{#if impersonatingOrgId === org.id}
										<svg class="animate-spin -ml-0.5 mr-2 h-4 w-4" fill="none" viewBox="0 0 24 24">
											<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
											<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
										</svg>
										Impersonating...
									{:else}
										<svg class="w-4 h-4 mr-1.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
											<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
											<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
										</svg>
										Impersonate
									{/if}
								</button>
								<button
									onclick={() => confirmDeleteOrg = org}
									disabled={deletingOrgId === org.id}
									class="inline-flex items-center px-3 py-1.5 text-sm font-medium rounded-md text-red-700 bg-red-50 hover:bg-red-100 border border-red-200"
								>
									{#if deletingOrgId === org.id}
										<svg class="animate-spin -ml-0.5 mr-2 h-4 w-4" fill="none" viewBox="0 0 24 24">
											<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
											<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
										</svg>
										Deleting...
									{:else}
										<svg class="w-4 h-4 mr-1.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
											<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
										</svg>
										Delete
									{/if}
								</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
			</div>

			<!-- Pagination -->
			{#if totalPages > 1}
				<div class="px-6 py-4 border-t border-gray-200 flex items-center justify-between">
					<div class="text-sm text-gray-700">
						Showing <span class="font-medium">{(page - 1) * pageSize + 1}</span> to <span class="font-medium">{Math.min(page * pageSize, total)}</span> of <span class="font-medium">{total}</span> organizations
					</div>
					<div class="flex items-center space-x-2">
						<button
							onclick={() => goToPage(page - 1)}
							disabled={page <= 1}
							class="px-3 py-1 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
						>
							Previous
						</button>
						<span class="text-sm text-gray-700">
							Page {page} of {totalPages}
						</span>
						<button
							onclick={() => goToPage(page + 1)}
							disabled={page >= totalPages}
							class="px-3 py-1 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
						>
							Next
						</button>
					</div>
				</div>
			{/if}
		{/if}
	</div>

	<!-- Quick stats -->
	<div class="grid grid-cols-1 md:grid-cols-4 gap-6">
		<div class="bg-white shadow rounded-lg p-6">
			<div class="flex items-center">
				<div class="flex-shrink-0">
					<svg class="h-8 w-8 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
					</svg>
				</div>
				<div class="ml-4">
					<p class="text-sm font-medium text-gray-500">Total Orgs</p>
					<p class="text-2xl font-semibold text-gray-900">{total}</p>
				</div>
			</div>
		</div>
		<div class="bg-white shadow rounded-lg p-6">
			<div class="flex items-center">
				<div class="flex-shrink-0">
					<svg class="h-8 w-8 text-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 3v4M3 5h4M6 17v4m-2-2h4m5-16l2.286 6.857L21 12l-5.714 2.143L13 21l-2.286-6.857L5 12l5.714-2.143L13 3z" />
					</svg>
				</div>
				<div class="ml-4">
					<p class="text-sm font-medium text-gray-500">Pro Tier</p>
					<p class="text-2xl font-semibold text-gray-900">{orgs.filter(o => o.plan === 'pro').length}</p>
				</div>
			</div>
		</div>
		<div class="bg-white shadow rounded-lg p-6">
			<div class="flex items-center">
				<div class="flex-shrink-0">
					<svg class="h-8 w-8 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
					</svg>
				</div>
				<div class="ml-4">
					<p class="text-sm font-medium text-gray-500">Active</p>
					<p class="text-2xl font-semibold text-gray-900">{orgs.filter(o => o.isActive).length}</p>
				</div>
			</div>
		</div>
		<div class="bg-white shadow rounded-lg p-6">
			<div class="flex items-center">
				<div class="flex-shrink-0">
					<svg class="h-8 w-8 text-amber-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
					</svg>
				</div>
				<div class="ml-4">
					<p class="text-sm font-medium text-gray-500">Impersonation</p>
					<p class="text-2xl font-semibold text-gray-900">{auth.isImpersonation ? 'Active' : 'None'}</p>
				</div>
			</div>
		</div>
	</div>
</div>

<!-- Delete Confirmation Modal -->
{#if confirmDeleteOrg}
	<div class="fixed inset-0 bg-gray-500 bg-opacity-75 flex items-center justify-center z-50">
		<div class="bg-white rounded-lg shadow-xl max-w-md w-full mx-4">
			<div class="p-6">
				<div class="flex items-center justify-center w-12 h-12 mx-auto bg-red-100 rounded-full">
					<svg class="w-6 h-6 text-red-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
					</svg>
				</div>
				<h3 class="mt-4 text-lg font-medium text-center text-gray-900">Delete Organization</h3>
				<p class="mt-2 text-sm text-center text-gray-500">
					Are you sure you want to delete <strong>{confirmDeleteOrg.name}</strong>? This will permanently delete all data including contacts, accounts, tasks, users, and settings. This action cannot be undone.
				</p>
			</div>
			<div class="px-6 py-4 bg-gray-50 rounded-b-lg flex justify-end space-x-3">
				<button
					onclick={() => confirmDeleteOrg = null}
					class="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50"
				>
					Cancel
				</button>
				<button
					onclick={() => confirmDeleteOrg && handleDelete(confirmDeleteOrg)}
					disabled={deletingOrgId !== null}
					class="px-4 py-2 text-sm font-medium text-white bg-red-600 border border-transparent rounded-md hover:bg-red-700 disabled:opacity-50"
				>
					{#if deletingOrgId}
						Deleting...
					{:else}
						Delete Organization
					{/if}
				</button>
			</div>
		</div>
	</div>
{/if}

<!-- Create Organization Modal -->
{#if showCreateModal}
	<div class="fixed inset-0 bg-gray-500 bg-opacity-75 flex items-center justify-center z-50">
		<div class="bg-white rounded-lg shadow-xl max-w-md w-full mx-4">
			<div class="p-6">
				<div class="flex items-center justify-center w-12 h-12 mx-auto bg-blue-100 rounded-full">
					<svg class="w-6 h-6 text-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
					</svg>
				</div>
				<h3 class="mt-4 text-lg font-medium text-center text-gray-900">New Organization</h3>
				<p class="mt-2 text-sm text-center text-gray-500">
					Create a new organization. Default entities, fields, and layouts will be provisioned automatically.
				</p>
				<form onsubmit={(e) => { e.preventDefault(); handleCreateOrg(); }} class="mt-4 space-y-4">
					<div>
						<label for="org-name" class="block text-sm font-medium text-gray-700">Organization Name</label>
						<input
							id="org-name"
							type="text"
							bind:value={createOrgName}
							placeholder="Acme Corp"
							class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary text-sm px-3 py-2 border"
							required
						/>
					</div>
					<div>
						<label for="org-slug" class="block text-sm font-medium text-gray-700">Slug (optional)</label>
						<input
							id="org-slug"
							type="text"
							bind:value={createOrgSlug}
							placeholder="acme-corp (auto-generated if empty)"
							class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary text-sm px-3 py-2 border"
						/>
						<p class="mt-1 text-xs text-gray-500">URL-safe identifier. Auto-generated from name if left blank.</p>
					</div>
				</form>
			</div>
			<div class="px-6 py-4 bg-gray-50 rounded-b-lg flex justify-end space-x-3">
				<button
					onclick={() => { showCreateModal = false; createOrgName = ''; createOrgSlug = ''; }}
					class="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50"
					disabled={isCreating}
				>
					Cancel
				</button>
				<button
					onclick={() => handleCreateOrg()}
					disabled={isCreating || !createOrgName.trim()}
					class="px-4 py-2 text-sm font-medium text-black bg-primary border border-transparent rounded-md hover:bg-primary/90 disabled:opacity-50"
				>
					{#if isCreating}
						Creating...
					{:else}
						Create Organization
					{/if}
				</button>
			</div>
		</div>
	</div>
{/if}
{/if}
