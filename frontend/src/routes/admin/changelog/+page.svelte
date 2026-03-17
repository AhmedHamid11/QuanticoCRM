<script lang="ts">
	import { onMount } from 'svelte';
	import { get, post } from '$lib/utils/api';
	import { auth } from '$lib/stores/auth.svelte';

	// Types
	interface ChangelogEntry {
		category: 'Added' | 'Changed' | 'Fixed' | 'Removed' | 'Deprecated' | 'Security';
		description: string;
	}

	interface VersionChangelog {
		version: string;
		entries: ChangelogEntry[];
	}

	interface ChangelogResponse {
		fromVersion: string;
		toVersion: string;
		changelogs: VersionChangelog[];
	}

	interface MigrationStatus {
		platformVersion: string;
		totalOrgs: number;
		upToDateCount: number;
		failedCount: number;
		failedOrgs: FailedOrg[];
		lastRunAt: string | null;
	}

	interface FailedOrg {
		orgId: string;
		orgName: string;
		errorMessage: string;
		failedAt: string;
		attemptedVersion: string;
	}

	interface RetryResponse {
		success: boolean;
		run: {
			status: string;
			errorMessage?: string;
		};
	}

	// State
	let changelogs = $state<VersionChangelog[]>([]);
	let isLoading = $state(true);
	let error = $state<string | null>(null);

	let migrationStatus = $state<MigrationStatus | null>(null);
	let migrationLoading = $state(true);
	let migrationError = $state<string | null>(null);
	let retryingOrg = $state<string | null>(null);
	let retryingAll = $state(false);

	// Category badge styling
	const categoryStyles: Record<string, { bg: string; text: string }> = {
		Added: { bg: 'bg-green-100', text: 'text-green-800' },
		Changed: { bg: 'bg-blue-100', text: 'text-blue-800' },
		Fixed: { bg: 'bg-amber-100', text: 'text-amber-800' },
		Removed: { bg: 'bg-red-100', text: 'text-red-800' },
		Deprecated: { bg: 'bg-gray-100', text: 'text-gray-800' },
		Security: { bg: 'bg-purple-100', text: 'text-purple-800' }
	};

	// Get style for category
	function getCategoryStyle(category: string): { bg: string; text: string } {
		return categoryStyles[category] || { bg: 'bg-gray-100', text: 'text-gray-800' };
	}

	// Load changelog data
	async function loadChangelog() {
		isLoading = true;
		error = null;
		try {
			const response = await get<ChangelogResponse>('/version/changelog/since?from=v0.0.0');
			changelogs = response.changelogs || [];
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load changelog';
			console.error('Failed to load changelog:', e);
		} finally {
			isLoading = false;
		}
	}

	// Load migration status
	async function loadMigrationStatus() {
		migrationLoading = true;
		migrationError = null;
		try {
			migrationStatus = await get<MigrationStatus>('/version/migration-status');
		} catch (e) {
			migrationError = e instanceof Error ? e.message : 'Failed to load migration status';
		} finally {
			migrationLoading = false;
		}
	}

	// Retry migration for a specific org
	async function retryOrg(orgId: string) {
		retryingOrg = orgId;
		try {
			const result = await post<RetryResponse>(`/version/migration-retry/${orgId}`, {});
			if (result.success) {
				// Refresh status after successful retry
				await loadMigrationStatus();
			} else {
				alert(`Retry failed: ${result.run.errorMessage || 'Unknown error'}`);
			}
		} catch (e) {
			alert(`Retry failed: ${e instanceof Error ? e.message : 'Unknown error'}`);
		} finally {
			retryingOrg = null;
		}
	}

	// Retry all failed org migrations
	async function retryAll() {
		retryingAll = true;
		try {
			const result = await post<{ successCount: number; failedCount: number }>('/version/migration-retry-all', {});
			alert(`Retry complete: ${result.successCount} succeeded, ${result.failedCount} failed`);
			await loadMigrationStatus();
		} catch (e) {
			alert(`Retry failed: ${e instanceof Error ? e.message : 'Unknown error'}`);
		} finally {
			retryingAll = false;
		}
	}

	onMount(() => {
		loadChangelog();
		loadMigrationStatus();
	});
</script>

<div class="space-y-6">
	<!-- Header -->
	<div class="flex items-center justify-between">
		<div>
			<div class="flex items-center space-x-4">
				<a
					href="/admin"
					class="text-gray-500 hover:text-gray-700 transition-colors"
				>
					<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 19l-7-7m0 0l7-7m-7 7h18" />
					</svg>
				</a>
				<div>
					<h1 class="text-2xl font-bold text-gray-900">Changelog</h1>
					<p class="mt-1 text-sm text-gray-500">Platform changes and updates by version</p>
				</div>
			</div>
		</div>
	</div>

	<!-- Error state -->
	{#if error}
		<div class="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg">
			{error}
		</div>
	{/if}

	<!-- Migration Status Card -->
	{#if !migrationLoading && migrationStatus}
		<div class="bg-white shadow rounded-lg overflow-hidden border-l-4 {auth.isPlatformAdmin && migrationStatus.failedCount > 0 ? 'border-amber-500' : 'border-green-500'}">
			<div class="px-6 py-4">
				<div class="flex items-center justify-between">
					<div>
						<h2 class="text-lg font-semibold text-gray-900">Migration Status</h2>
						<p class="text-sm text-gray-500">
							{#if auth.isPlatformAdmin}
								{migrationStatus.upToDateCount} of {migrationStatus.totalOrgs} organizations up to date
							{:else}
								Migration status up to date
							{/if}
							{#if migrationStatus.lastRunAt}
								<span class="ml-2">
									Last run: {new Date(migrationStatus.lastRunAt).toLocaleString()}
								</span>
							{/if}
						</p>
					</div>
					<div class="flex items-center space-x-2">
						{#if auth.isPlatformAdmin && migrationStatus.failedCount > 0}
							<span class="inline-flex items-center px-3 py-1 rounded-full text-sm font-medium bg-amber-100 text-amber-800">
								{migrationStatus.failedCount} failed
							</span>
						{:else}
							<span class="inline-flex items-center px-3 py-1 rounded-full text-sm font-medium bg-green-100 text-green-800">
								{auth.isPlatformAdmin ? 'All up to date' : 'Up to date'}
							</span>
						{/if}
					</div>
				</div>

				<!-- Failed Orgs Section (platform admins only) -->
				{#if auth.isPlatformAdmin && migrationStatus.failedCount > 0}
					<div class="mt-4 border-t pt-4">
						<div class="flex items-center justify-between mb-3">
							<h3 class="text-sm font-medium text-gray-700">Failed Organizations</h3>
							<button
								onclick={() => retryAll()}
								disabled={retryingAll}
								class="text-sm text-blue-600 hover:text-blue-800 disabled:opacity-50"
							>
								{retryingAll ? 'Retrying...' : 'Retry All'}
							</button>
						</div>
						<div class="space-y-2">
							{#each migrationStatus.failedOrgs as org (org.orgId)}
								<div class="flex items-center justify-between p-3 bg-amber-50 rounded-lg">
									<div class="flex-1 min-w-0">
										<p class="text-sm font-medium text-gray-900 truncate">{org.orgName}</p>
										<p class="text-xs text-gray-500 truncate">{org.errorMessage}</p>
										<p class="text-xs text-gray-400">
											Failed: {new Date(org.failedAt).toLocaleString()}
										</p>
									</div>
									<button
										onclick={() => retryOrg(org.orgId)}
										disabled={retryingOrg === org.orgId}
										class="ml-3 px-3 py-1 text-sm bg-blue-600 text-white rounded hover:bg-blue-600/90 disabled:opacity-50"
									>
										{retryingOrg === org.orgId ? 'Retrying...' : 'Retry'}
									</button>
								</div>
							{/each}
						</div>
					</div>
				{/if}
			</div>
		</div>
	{:else if migrationLoading}
		<div class="bg-white shadow rounded-lg p-6">
			<div class="animate-pulse flex items-center space-x-4">
				<div class="h-4 bg-gray-200 rounded w-48"></div>
			</div>
		</div>
	{:else if migrationError}
		<div class="bg-amber-50 border border-amber-200 text-amber-700 px-4 py-3 rounded-lg">
			Migration status unavailable: {migrationError}
		</div>
	{/if}

	<!-- Main content -->
	<div class="crm-card overflow-hidden">
		{#if isLoading}
			<!-- Loading state -->
			<div class="px-6 py-12 text-center">
				<svg class="animate-spin mx-auto h-8 w-8 text-gray-400" fill="none" viewBox="0 0 24 24">
					<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
					<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
				</svg>
				<p class="mt-4 text-gray-500">Loading changelog...</p>
			</div>
		{:else if changelogs.length === 0}
			<!-- Empty state -->
			<div class="px-6 py-12 text-center">
				<svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
				</svg>
				<h3 class="mt-2 text-sm font-medium text-gray-900">No changelog entries</h3>
				<p class="mt-1 text-sm text-gray-500">No version updates have been documented yet.</p>
			</div>
		{:else}
			<!-- Version list -->
			<div class="divide-y divide-gray-200">
				{#each changelogs as changelog (changelog.version)}
					<div class="px-6 py-4">
						<!-- Version header -->
						<div class="flex items-center space-x-3 mb-3">
							<span class="inline-flex items-center px-3 py-1 rounded-full text-sm font-semibold bg-gray-100 text-gray-800">
								{changelog.version}
							</span>
						</div>

						<!-- Entries -->
						<ul class="space-y-2 ml-1">
							{#each changelog.entries as entry}
								{@const style = getCategoryStyle(entry.category)}
								<li class="flex items-start space-x-3">
									<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium {style.bg} {style.text} flex-shrink-0 mt-0.5">
										{entry.category}
									</span>
									<span class="text-sm text-gray-700">{entry.description}</span>
								</li>
							{/each}
						</ul>
					</div>
				{/each}
			</div>
		{/if}
	</div>
</div>
