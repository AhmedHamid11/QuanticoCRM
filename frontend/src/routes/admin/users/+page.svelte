<script lang="ts">
	import { onMount } from 'svelte';
	import { auth, authFetch } from '$lib/stores/auth.svelte';
	import type { UserWithMembership, UserListResponse } from '$lib/types/auth';
	import { addToast } from '$lib/stores/toast.svelte';

	// State
	let users = $state<UserWithMembership[]>([]);
	let total = $state(0);
	let page = $state(1);
	let pageSize = $state(20);
	let totalPages = $state(0);
	let isLoading = $state(true);
	let editingUserId = $state<string | null>(null);
	let editingRole = $state<string>('user');

	// Invite modal state
	let showInviteModal = $state(false);
	let inviteEmail = $state('');
	let inviteRole = $state<'admin' | 'user'>('user');
	let isInviting = $state(false);

	// Pending invitations
	interface Invitation {
		id: string;
		email: string;
		role: string;
		token: string;
		expiresAt: string;
		createdAt: string;
		inviterName: string;
	}
	let pendingInvitations = $state<Invitation[]>([]);
	let isLoadingInvitations = $state(true);

	// Role display mapping
	const roleLabels: Record<string, { label: string; color: string }> = {
		owner: { label: 'Owner', color: 'bg-purple-100 text-purple-800' },
		admin: { label: 'Admin', color: 'bg-blue-100 text-blue-800' },
		user: { label: 'User', color: 'bg-gray-100 text-gray-800' }
	};

	// Load users
	async function loadUsers() {
		isLoading = true;
		try {
			const response = await authFetch<UserListResponse>(`/users?page=${page}&pageSize=${pageSize}`);
			users = response.data || [];
			total = response.total;
			totalPages = response.totalPages;
		} catch (err) {
			addToast('Failed to load users', 'error');
		} finally {
			isLoading = false;
		}
	}

	// Load pending invitations
	async function loadInvitations() {
		isLoadingInvitations = true;
		try {
			const response = await authFetch<{ invitations: Invitation[] }>('/auth/invitations');
			pendingInvitations = response.invitations || [];
		} catch (err) {
			console.error('Failed to load invitations:', err);
		} finally {
			isLoadingInvitations = false;
		}
	}

	// Cancel invitation
	async function cancelInvitation(invitation: Invitation) {
		if (!confirm(`Cancel invitation for ${invitation.email}?`)) return;

		try {
			await authFetch(`/auth/invitations/${invitation.id}`, { method: 'DELETE' });
			addToast('Invitation cancelled', 'success');
			await loadInvitations();
		} catch (err: any) {
			addToast(err.message || 'Failed to cancel invitation', 'error');
		}
	}

	// Copy invite link to clipboard
	async function copyInviteLink(token: string) {
		const link = `${window.location.origin}/accept-invite?token=${token}`;
		try {
			await navigator.clipboard.writeText(link);
			addToast('Invite link copied to clipboard', 'success');
		} catch (err) {
			addToast('Failed to copy link', 'error');
		}
	}

	// Start editing role
	function startEditRole(user: UserWithMembership) {
		editingUserId = user.id;
		editingRole = user.role;
	}

	// Cancel editing
	function cancelEdit() {
		editingUserId = null;
		editingRole = 'user';
	}

	// Save role change
	async function saveRole(userId: string) {
		try {
			await authFetch(`/users/${userId}/role`, {
				method: 'PUT',
				body: { role: editingRole }
			});
			addToast('Role updated successfully', 'success');
			cancelEdit();
			await loadUsers();
		} catch (err: any) {
			addToast(err.message || 'Failed to update role', 'error');
		}
	}

	// Remove user from organization
	async function removeUser(user: UserWithMembership) {
		if (!confirm(`Are you sure you want to remove ${user.firstName} ${user.lastName} (${user.email}) from this organization?`)) {
			return;
		}

		try {
			await authFetch(`/users/${user.id}`, {
				method: 'DELETE'
			});
			addToast('User removed from organization', 'success');
			await loadUsers();
		} catch (err: any) {
			addToast(err.message || 'Failed to remove user', 'error');
		}
	}

	// Toggle user active status
	async function toggleUserStatus(user: UserWithMembership) {
		const newStatus = !user.isActive;
		const action = newStatus ? 'activate' : 'deactivate';

		if (!confirm(`Are you sure you want to ${action} ${user.firstName || ''} ${user.lastName || ''} (${user.email})? ${!newStatus ? 'They will be logged out immediately and unable to log in.' : ''}`)) {
			return;
		}

		try {
			await authFetch(`/users/${user.id}/status`, {
				method: 'PUT',
				body: { isActive: newStatus }
			});
			addToast(`User ${newStatus ? 'activated' : 'deactivated'} successfully`, 'success');
			await loadUsers();
		} catch (err: any) {
			addToast(err.message || `Failed to ${action} user`, 'error');
		}
	}

	// Format date
	function formatDate(dateStr: string | null): string {
		if (!dateStr) return 'Never';
		return new Date(dateStr).toLocaleDateString('en-US', {
			year: 'numeric',
			month: 'short',
			day: 'numeric'
		});
	}

	// Pagination
	function goToPage(newPage: number) {
		if (newPage >= 1 && newPage <= totalPages) {
			page = newPage;
			loadUsers();
		}
	}

	// Open invite modal
	function openInviteModal() {
		inviteEmail = '';
		inviteRole = 'user';
		showInviteModal = true;
	}

	// Close invite modal
	function closeInviteModal() {
		showInviteModal = false;
		inviteEmail = '';
		inviteRole = 'user';
	}

	// Send invitation
	async function sendInvitation() {
		if (!inviteEmail.trim()) {
			addToast('Please enter an email address', 'error');
			return;
		}

		isInviting = true;
		try {
			await authFetch('/auth/invite', {
				method: 'POST',
				body: { email: inviteEmail.trim(), role: inviteRole }
			});
			addToast(`Invitation sent to ${inviteEmail}`, 'success');
			closeInviteModal();
			await loadInvitations();
		} catch (err: any) {
			addToast(err.message || 'Failed to send invitation', 'error');
		} finally {
			isInviting = false;
		}
	}

	onMount(() => {
		loadUsers();
		loadInvitations();
	});
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold text-gray-900">User Management</h1>
			<p class="mt-1 text-sm text-gray-500">
				Manage users and their roles in your organization
			</p>
		</div>
		<div class="flex items-center space-x-3">
			<button
				onclick={openInviteModal}
				class="inline-flex items-center px-4 py-2 text-sm font-medium text-white bg-blue-600 border border-transparent rounded-md hover:bg-blue-600/90 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
			>
				<svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
				</svg>
				Invite User
			</button>
			<a
				href="/admin"
				class="inline-flex items-center px-3 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50"
			>
				<svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 19l-7-7m0 0l7-7m-7 7h18" />
				</svg>
				Back to Setup
			</a>
		</div>
	</div>

	<!-- Pending Invitations -->
	{#if pendingInvitations.length > 0}
		<div class="bg-white shadow rounded-lg overflow-hidden">
			<div class="px-6 py-4 border-b border-gray-200 bg-amber-50">
				<h2 class="text-lg font-medium text-amber-800">Pending Invitations ({pendingInvitations.length})</h2>
				<p class="text-sm text-amber-600">These users have been invited but haven't accepted yet. Share the invite link with them.</p>
			</div>
			<table class="min-w-full divide-y divide-gray-200">
				<thead class="bg-gray-50">
					<tr>
						<th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Email</th>
						<th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Role</th>
						<th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Invited</th>
						<th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Expires</th>
						<th scope="col" class="relative px-6 py-3"><span class="sr-only">Actions</span></th>
					</tr>
				</thead>
				<tbody class="bg-white divide-y divide-gray-200">
					{#each pendingInvitations as invitation (invitation.id)}
						<tr class="hover:bg-gray-50">
							<td class="px-6 py-4 whitespace-nowrap">
								<div class="text-sm font-medium text-gray-900">{invitation.email}</div>
							</td>
							<td class="px-6 py-4 whitespace-nowrap">
								<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium {roleLabels[invitation.role]?.color || 'bg-gray-100 text-gray-800'}">
									{roleLabels[invitation.role]?.label || invitation.role}
								</span>
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{formatDate(invitation.createdAt)}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{formatDate(invitation.expiresAt)}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium space-x-2">
								<button
									onclick={() => copyInviteLink(invitation.token)}
									class="text-blue-600 hover:text-blue-900"
									title="Copy invite link"
								>
									Copy Link
								</button>
								<button
									onclick={() => cancelInvitation(invitation)}
									class="text-red-600 hover:text-red-900"
								>
									Cancel
								</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}

	<!-- Users table -->
	<div class="bg-white shadow rounded-lg overflow-hidden">
		{#if isLoading}
			<div class="flex items-center justify-center py-12">
				<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500"></div>
			</div>
		{:else if users.length === 0}
			<div class="text-center py-12">
				<svg class="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
				</svg>
				<h3 class="mt-2 text-sm font-medium text-gray-900">No users</h3>
				<p class="mt-1 text-sm text-gray-500">There are no users in this organization yet.</p>
			</div>
		{:else}
			<table class="min-w-full divide-y divide-gray-200">
				<thead class="bg-gray-50">
					<tr>
						<th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							User
						</th>
						<th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Role
						</th>
						<th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Status
						</th>
						<th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Joined
						</th>
						<th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Last Login
						</th>
						<th scope="col" class="relative px-6 py-3">
							<span class="sr-only">Actions</span>
						</th>
					</tr>
				</thead>
				<tbody class="bg-white divide-y divide-gray-200">
					{#each users as user (user.id)}
						<tr class="hover:bg-gray-50">
							<td class="px-6 py-4 whitespace-nowrap">
								<div class="flex items-center">
									<div class="flex-shrink-0 h-10 w-10">
										<div class="h-10 w-10 rounded-full bg-blue-100 flex items-center justify-center">
											<span class="text-blue-600 font-medium text-sm">
												{(user.firstName?.[0] || '') + (user.lastName?.[0] || '') || user.email[0].toUpperCase()}
											</span>
										</div>
									</div>
									<div class="ml-4">
										<div class="text-sm font-medium text-gray-900">
											{user.firstName} {user.lastName}
											{#if user.isPlatformAdmin}
												<span class="ml-2 inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-purple-100 text-purple-800">
													Platform Admin
												</span>
											{/if}
										</div>
										<div class="text-sm text-gray-500">{user.email}</div>
									</div>
								</div>
							</td>
							<td class="px-6 py-4 whitespace-nowrap">
								{#if editingUserId === user.id}
									<div class="flex items-center space-x-2">
										<select
											bind:value={editingRole}
											class="block w-24 rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 text-sm"
										>
											{#if auth.isOwner}
												<option value="owner">Owner</option>
											{/if}
											<option value="admin">Admin</option>
											<option value="user">User</option>
										</select>
										<button
											onclick={() => saveRole(user.id)}
											class="text-green-600 hover:text-green-900"
											title="Save"
										>
											<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
												<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
											</svg>
										</button>
										<button
											onclick={cancelEdit}
											class="text-gray-600 hover:text-gray-900"
											title="Cancel"
										>
											<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
												<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
											</svg>
										</button>
									</div>
								{:else}
									<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium {roleLabels[user.role]?.color || 'bg-gray-100 text-gray-800'}">
										{roleLabels[user.role]?.label || user.role}
									</span>
								{/if}
							</td>
							<td class="px-6 py-4 whitespace-nowrap">
								{#if user.isActive}
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
								{formatDate(user.joinedAt)}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{formatDate(user.lastLoginAt)}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
								{#if auth.isAdmin && editingUserId !== user.id}
									<!-- Owners can edit anyone's role, admins can only edit non-owners -->
									{#if auth.isOwner || user.role !== 'owner'}
										<button
											onclick={() => startEditRole(user)}
											class="text-blue-600 hover:text-blue-900 mr-4"
										>
											Edit Role
										</button>
									{/if}
								{/if}
								{#if auth.isAdmin && user.id !== auth.user?.id}
									<!-- Admins can toggle status, but admins can't toggle owners -->
									{#if !(auth.role === 'admin' && user.role === 'owner')}
										<button
											onclick={() => toggleUserStatus(user)}
											class="{user.isActive ? 'text-amber-600 hover:text-amber-900' : 'text-green-600 hover:text-green-900'} mr-4"
										>
											{user.isActive ? 'Deactivate' : 'Activate'}
										</button>
										<button
											onclick={() => removeUser(user)}
											class="text-red-600 hover:text-red-900"
										>
											Remove
										</button>
									{/if}
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>

			<!-- Pagination -->
			{#if totalPages > 1}
				<div class="bg-white px-4 py-3 flex items-center justify-between border-t border-gray-200 sm:px-6">
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
							disabled={page === totalPages}
							class="ml-3 relative inline-flex items-center px-4 py-2 border border-gray-300 text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
						>
							Next
						</button>
					</div>
					<div class="hidden sm:flex-1 sm:flex sm:items-center sm:justify-between">
						<div>
							<p class="text-sm text-gray-700">
								Showing <span class="font-medium">{(page - 1) * pageSize + 1}</span> to <span class="font-medium">{Math.min(page * pageSize, total)}</span> of <span class="font-medium">{total}</span> users
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
									Page {page} of {totalPages}
								</span>
								<button
									onclick={() => goToPage(page + 1)}
									disabled={page === totalPages}
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
		{/if}
	</div>

	<!-- Role descriptions -->
	<div class="bg-blue-50 rounded-lg p-4">
		<h3 class="text-sm font-medium text-blue-800 mb-2">Role Descriptions</h3>
		<div class="space-y-1 text-sm text-blue-700">
			<p><span class="font-medium">Owner:</span> Full organization control including delete org, transfer ownership, and all admin capabilities</p>
			<p><span class="font-medium">Admin:</span> Access to Setup (entity manager, navigation, tripwires, etc.) and user management</p>
			<p><span class="font-medium">User:</span> Access to CRM objects only (contacts, accounts, tasks) - cannot access Setup</p>
		</div>
	</div>
</div>

<!-- Invite User Modal -->
{#if showInviteModal}
	<div class="fixed inset-0 z-50 overflow-y-auto">
		<div class="flex min-h-full items-end justify-center p-4 text-center sm:items-center sm:p-0">
			<!-- Background overlay -->
			<div
				class="fixed inset-0 bg-gray-500 bg-opacity-75 transition-opacity"
				onclick={closeInviteModal}
			></div>

			<!-- Modal panel -->
			<div class="relative transform overflow-hidden rounded-lg bg-white text-left shadow-xl transition-all sm:my-8 sm:w-full sm:max-w-lg">
				<div class="bg-white px-4 pb-4 pt-5 sm:p-6 sm:pb-4">
					<div class="sm:flex sm:items-start">
						<div class="mx-auto flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-full bg-blue-100 sm:mx-0 sm:h-10 sm:w-10">
							<svg class="h-6 w-6 text-blue-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M18 9v3m0 0v3m0-3h3m-3 0h-3m-2-5a4 4 0 11-8 0 4 4 0 018 0zM3 20a6 6 0 0112 0v1H3v-1z" />
							</svg>
						</div>
						<div class="mt-3 text-center sm:ml-4 sm:mt-0 sm:text-left flex-1">
							<h3 class="text-lg font-semibold leading-6 text-gray-900">
								Invite User
							</h3>
							<div class="mt-4 space-y-4">
								<div>
									<label for="invite-email" class="block text-sm font-medium text-gray-700">
										Email Address
									</label>
									<input
										type="email"
										id="invite-email"
										bind:value={inviteEmail}
										placeholder="user@example.com"
										class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
									/>
								</div>
								<div>
									<label for="invite-role" class="block text-sm font-medium text-gray-700">
										Role
									</label>
									<select
										id="invite-role"
										bind:value={inviteRole}
										class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
									>
										<option value="user">User - CRM access only</option>
										<option value="admin">Admin - Full Setup access</option>
									</select>
								</div>
							</div>
						</div>
					</div>
				</div>
				<div class="bg-gray-50 px-4 py-3 sm:flex sm:flex-row-reverse sm:px-6">
					<button
						type="button"
						onclick={sendInvitation}
						disabled={isInviting || !inviteEmail.trim()}
						class="inline-flex w-full justify-center rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-blue-600 sm:ml-3 sm:w-auto disabled:opacity-50 disabled:cursor-not-allowed"
					>
						{#if isInviting}
							<svg class="animate-spin -ml-1 mr-2 h-4 w-4 text-white" fill="none" viewBox="0 0 24 24">
								<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
								<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
							</svg>
							Sending...
						{:else}
							Send Invitation
						{/if}
					</button>
					<button
						type="button"
						onclick={closeInviteModal}
						class="mt-3 inline-flex w-full justify-center rounded-md bg-white px-3 py-2 text-sm font-semibold text-gray-900 shadow-sm ring-1 ring-inset ring-gray-300 hover:bg-gray-50 sm:mt-0 sm:w-auto"
					>
						Cancel
					</button>
				</div>
			</div>
		</div>
	</div>
{/if}
