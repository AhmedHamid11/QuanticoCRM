<script lang="ts">
	import '../app.css';
	import Toast from '$lib/components/Toast.svelte';
	import { NavigationProgress } from '$lib/components/ui';
	import { onMount } from 'svelte';
	import { loadNavigation, getNavigationTabs } from '$lib/stores/navigation.svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { auth, logout, initAuth, switchOrg, stopImpersonation } from '$lib/stores/auth.svelte';

	let { children } = $props();

	// User menu state
	let showUserMenu = $state(false);
	let showOrgSwitcher = $state(false);
	let lastLoadedOrgId = $state<string | null>(null);

	// Check if on auth page (login, register, password reset)
	let isAuthPage = $derived(
		$page.url.pathname === '/login' ||
		$page.url.pathname === '/register' ||
		$page.url.pathname === '/forgot-password' ||
		$page.url.pathname === '/reset-password' ||
		$page.url.pathname.startsWith('/accept-invite')
	);

	// Initialize auth on mount
	onMount(() => {
		initAuth();
	});

	// Load navigation when authenticated (and reload when org changes)
	$effect(() => {
		const currentOrgId = auth.currentOrg?.orgId;
		if (!isAuthPage && auth.isAuthenticated && !auth.isLoading && currentOrgId && currentOrgId !== lastLoadedOrgId) {
			lastLoadedOrgId = currentOrgId;
			loadNavigation();
		}
	});

	// Redirect to login if not authenticated (after loading) - skip for auth pages
	$effect(() => {
		if (!isAuthPage && !auth.isLoading && !auth.isAuthenticated) {
			goto('/login');
		}
	});

	// Get current path for active state
	let currentPath = $derived($page.url.pathname);

	// Check if a nav item is active
	function isActive(href: string): boolean {
		if (href === '/') return currentPath === '/';
		return currentPath.startsWith(href);
	}

	// Reserved routes that are NOT custom entities
	const RESERVED_ROUTES = ['contacts', 'accounts', 'admin', 'settings', 'tasks', 'services', 'accept-invite', 'login', 'register', 'quotes'];

	// Normalize a URL path segment for comparison (decode URL, lowercase, spaces/underscores to hyphens)
	function normalizePathSegment(segment: string): string {
		try {
			// Decode URL-encoded characters (e.g., %20 -> space)
			const decoded = decodeURIComponent(segment);
			return decoded.toLowerCase().replace(/[\s_]+/g, '-');
		} catch {
			// If decoding fails, just normalize as-is
			return segment.toLowerCase().replace(/[\s_]+/g, '-');
		}
	}

	// Detect current entity from URL for quick setup link (fully dynamic)
	let currentEntitySetupLink = $derived.by(() => {
		const segments = currentPath.split('/').filter(Boolean);
		if (segments.length < 2) return null;

		// Skip admin pages
		if (segments[0] === 'admin') return null;

		const firstSegment = segments[0];
		const secondSegment = segments[1];

		// Check if second segment looks like an ID (not 'new', 'edit', etc.)
		const isDetailPage = secondSegment && !['new', 'edit'].includes(secondSegment);
		if (!isDetailPage) return null;

		// Find matching navigation tab by href prefix (with normalization for flexible matching)
		const normalizedSegment = normalizePathSegment(firstSegment);
		const matchingTab = getNavigationTabs().find((tab) => {
			if (!tab.entityName) return false;
			const normalizedHref = normalizePathSegment(tab.href.replace(/^\//, ''));
			return normalizedHref === normalizedSegment;
		});

		if (matchingTab?.entityName) {
			return `/admin/entity-manager/${matchingTab.entityName}`;
		}

		// Skip reserved routes that have dedicated handlers
		if (RESERVED_ROUTES.includes(firstSegment.toLowerCase())) {
			return null;
		}

		return null;
	});

	// Handle logout
	async function handleLogout() {
		showUserMenu = false;
		await logout();
		goto('/login');
	}

	// Handle org switch
	async function handleSwitchOrg(orgId: string) {
		showOrgSwitcher = false;

		// If in impersonation mode and switching away from the impersonated org,
		// exit impersonation instead of switching
		if (auth.isImpersonation && orgId !== auth.currentOrg?.orgId) {
			await handleStopImpersonation();
			return;
		}

		await switchOrg({ orgId });
	}

	// Handle stop impersonation
	async function handleStopImpersonation() {
		await stopImpersonation();
		// Navigate back to platform admin page after stopping impersonation
		window.location.href = '/admin/platform';
	}

	// Close menus on click outside
	function handleClickOutside(event: MouseEvent) {
		const target = event.target as HTMLElement;
		if (!target.closest('.user-menu-container')) {
			showUserMenu = false;
		}
		if (!target.closest('.org-switcher-container')) {
			showOrgSwitcher = false;
		}
	}

	// User initials for avatar
	let userInitials = $derived(() => {
		if (!auth.user) return '?';
		const first = auth.user.firstName?.[0] || '';
		const last = auth.user.lastName?.[0] || '';
		return (first + last).toUpperCase() || auth.user.email[0].toUpperCase();
	});
</script>

<svelte:window onclick={handleClickOutside} />

<NavigationProgress />

{#if isAuthPage}
	<!-- Auth pages (login, register) - render without nav -->
	{@render children()}
{:else if auth.isLoading}
	<!-- Loading state -->
	<div class="min-h-screen flex items-center justify-center bg-gray-50">
		<div class="text-center">
			<div class="animate-spin rounded-full h-12 w-12 border-b-2 border-primary mx-auto"></div>
			<p class="mt-4 text-gray-600">Loading...</p>
		</div>
	</div>
{:else if auth.isAuthenticated}
	<div class="min-h-screen bg-gray-50">
		<!-- Impersonation banner -->
		{#if auth.isImpersonation}
			<div class="bg-amber-500 text-white px-4 py-2 text-center text-sm">
				<span class="font-medium">Impersonation Mode</span> - You are viewing as {auth.currentOrg?.orgName}
				<button
					onclick={handleStopImpersonation}
					class="ml-4 underline hover:no-underline"
				>
					Exit Impersonation
				</button>
			</div>
		{/if}

		<nav class="bg-white shadow-sm border-b border-gray-200">
			<div class="w-full max-w-[75%] mx-auto px-4 sm:px-6 lg:px-8">
				<div class="flex justify-between h-14">
					<div class="flex items-center">
						<a href="/" class="flex items-center text-xl font-bold">
							<span class="text-red-800">Quantico</span><span class="text-amber-600">CRM</span>
						</a>
						<div class="ml-10 flex space-x-1">
							{#each getNavigationTabs() as tab (tab.id)}
								<a
									href={tab.href}
									class="px-3 py-2 text-sm font-medium rounded-md transition-colors
										{isActive(tab.href)
											? 'bg-blue-50 text-blue-700'
											: 'text-gray-600 hover:text-gray-900 hover:bg-gray-50'}"
								>
									{tab.label}
								</a>
							{/each}
						</div>
					</div>
					<div class="flex items-center space-x-2">
						<!-- Org Switcher -->
						{#if auth.memberships.length > 1}
							<div class="relative org-switcher-container">
								<button
									onclick={() => showOrgSwitcher = !showOrgSwitcher}
									class="flex items-center px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-100 rounded-md transition-colors"
								>
									<span class="font-medium">{auth.currentOrg?.orgName}</span>
									<svg class="ml-1 w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
									</svg>
								</button>
								{#if showOrgSwitcher}
									<div class="absolute right-0 mt-2 w-56 bg-white rounded-md shadow-lg ring-1 ring-black ring-opacity-5 z-50">
										<div class="py-1">
											<div class="px-4 py-2 text-xs font-medium text-gray-500 uppercase">Switch Organization</div>
											{#each auth.memberships as membership (membership.id)}
												<button
													onclick={() => handleSwitchOrg(membership.orgId)}
													class="w-full text-left px-4 py-2 text-sm hover:bg-gray-100 flex items-center justify-between
														{membership.orgId === auth.currentOrg?.orgId ? 'bg-blue-50 text-blue-700' : 'text-gray-700'}"
												>
													<span>{membership.orgName}</span>
													{#if membership.orgId === auth.currentOrg?.orgId}
														<svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
															<path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
														</svg>
													{/if}
												</button>
											{/each}
										</div>
									</div>
								{/if}
							</div>
						{:else if auth.currentOrg}
							<span class="text-sm text-gray-600">{auth.currentOrg.orgName}</span>
						{/if}

						{#if auth.canAccessSetup}
							{#if currentEntitySetupLink}
								<a
									href={currentEntitySetupLink}
									class="p-2 text-gray-500 hover:text-gray-700 hover:bg-gray-100 rounded-md transition-colors"
									title="Edit Object"
								>
									<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
									</svg>
								</a>
							{/if}
							<a
								href="/admin"
								class="p-2 text-gray-500 hover:text-gray-700 hover:bg-gray-100 rounded-md transition-colors"
								title="Setup"
							>
								<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"></path>
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"></path>
								</svg>
							</a>
						{/if}

						<!-- User Menu -->
						<div class="relative user-menu-container">
							<button
								onclick={() => showUserMenu = !showUserMenu}
								class="flex items-center space-x-2 p-1 rounded-full hover:bg-gray-100 transition-colors"
							>
								<div class="w-8 h-8 rounded-full bg-primary flex items-center justify-center text-white text-sm font-medium">
									{userInitials()}
								</div>
							</button>
							{#if showUserMenu}
								<div class="absolute right-0 mt-2 w-56 bg-white rounded-md shadow-lg ring-1 ring-black ring-opacity-5 z-50">
									<div class="py-1">
										<div class="px-4 py-2 border-b border-gray-100">
											<p class="text-sm font-medium text-gray-900">
												{auth.user?.firstName} {auth.user?.lastName}
											</p>
											<p class="text-xs text-gray-500">{auth.user?.email}</p>
											{#if auth.isPlatformAdmin}
												<span class="inline-flex items-center px-2 py-0.5 mt-1 rounded text-xs font-medium bg-purple-100 text-purple-800">
													Platform Admin
												</span>
											{/if}
										</div>
										{#if auth.isPlatformAdmin && !auth.isImpersonation}
											<a
												href="/admin/platform"
												class="block px-4 py-2 text-sm text-gray-700 hover:bg-gray-100"
												onclick={() => showUserMenu = false}
											>
												Platform Console
											</a>
										{/if}
										<a
											href="/settings/profile"
											class="block px-4 py-2 text-sm text-gray-700 hover:bg-gray-100"
											onclick={() => showUserMenu = false}
										>
											Profile Settings
										</a>
										<button
											onclick={handleLogout}
											class="w-full text-left px-4 py-2 text-sm text-red-600 hover:bg-gray-100"
										>
											Sign out
										</button>
									</div>
								</div>
							{/if}
						</div>
					</div>
				</div>
			</div>
		</nav>

		<main class="w-full max-w-[75%] mx-auto px-4 sm:px-6 lg:px-8 py-6">
			{@render children()}
		</main>
	</div>
{/if}

<Toast />
