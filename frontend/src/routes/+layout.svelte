<script lang="ts">
	import '../app.css';
	import Toast from '$lib/components/Toast.svelte';
	import { NavigationProgress } from '$lib/components/ui';
	import { onMount } from 'svelte';
	import { loadNavigation, getNavigationTabs, getAccentColor } from '$lib/stores/navigation.svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { auth, logout, initAuth, switchOrg, stopImpersonation } from '$lib/stores/auth.svelte';

	let { children } = $props();

	// Grain texture canvas
	let grainCanvas = $state<HTMLCanvasElement | null>(null);

	function drawGrain() {
		if (!grainCanvas) return;
		const ctx = grainCanvas.getContext('2d');
		if (!ctx) return;
		grainCanvas.width = window.innerWidth;
		grainCanvas.height = window.innerHeight;
		const img = ctx.createImageData(grainCanvas.width, grainCanvas.height);
		const d = img.data;
		for (let i = 0; i < d.length; i += 4) {
			const v = Math.random() * 255;
			d[i] = v; d[i+1] = v; d[i+2] = v; d[i+3] = 60;
		}
		ctx.putImageData(img, 0, 0);
	}

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
		$page.url.pathname.startsWith('/accept-invite') ||
		$page.url.pathname.startsWith('/book/')
	);

	// Initialize auth on mount + grain texture
	onMount(() => {
		initAuth();
		drawGrain();
		window.addEventListener('resize', drawGrain);
		return () => window.removeEventListener('resize', drawGrain);
	});

	// Load navigation when authenticated (and reload when org changes)
	$effect(() => {
		const currentOrgId = auth.currentOrg?.orgId;
		if (!isAuthPage && auth.isAuthenticated && !auth.isLoading && currentOrgId && currentOrgId !== lastLoadedOrgId) {
			lastLoadedOrgId = currentOrgId;
			loadNavigation();
		}
	});

	// Redirect to login if not authenticated (after loading) - skip for auth pages and root
	// Root page handles its own unauthenticated state (shows login form)
	$effect(() => {
		const isRoot = $page.url.pathname === '/';
		if (!isAuthPage && !isRoot && !auth.isLoading && !auth.isAuthenticated) {
			goto('/');
		}
	});

	// Get current path for active state
	let currentPath = $derived($page.url.pathname);

	// Get accent color for gradient background
	let accentColor = $derived(getAccentColor());

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
			<div class="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-500 mx-auto"></div>
			<p class="mt-4 text-gray-600">Loading...</p>
		</div>
	</div>
{:else if auth.isAuthenticated}
	<div class="crm-gradient-bg" style="--crm-accent-color: {accentColor}">
		<canvas class="crm-grain" bind:this={grainCanvas}></canvas>
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

		<nav class="crm-nav">
			<div class="w-full px-6 lg:px-8">
				<div class="flex items-center justify-between h-16">
					<!-- Left: Logo -->
					<div class="flex items-center flex-shrink-0">
						<a href="/" class="flex-shrink-0 flex items-center">
							<img src="/logo.png" alt="Quantico CRM" class="h-10 w-auto" />
						</a>
					</div>

					<!-- Center: Navigation tabs in pill container -->
					<div class="crm-nav-tabs rounded-full px-1.5 py-1.5 flex items-center space-x-0.5">
						{#each getNavigationTabs() as tab (tab.id)}
							<a
								href={tab.href}
								class="px-4 py-1.5 text-sm rounded-full transition-all
									{isActive(tab.href)
										? 'bg-white shadow-sm text-gray-900 font-semibold'
										: 'text-gray-500 hover:text-gray-700'}"
							>
								{tab.label}
							</a>
						{/each}
					</div>

					<!-- Right: Org switcher + icons + user menu -->
					<div class="flex items-center space-x-2 flex-shrink-0">
						<!-- Org Switcher -->
						{#if auth.memberships.length > 1}
							<div class="relative org-switcher-container">
								<button
									onclick={() => showOrgSwitcher = !showOrgSwitcher}
									class="crm-nav-icon flex items-center px-3 py-1.5 text-sm text-gray-700 rounded-full transition-colors"
								>
									<span class="font-medium whitespace-nowrap truncate max-w-[200px]">{auth.currentOrg?.orgName}</span>
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
							<span class="text-sm text-gray-600 whitespace-nowrap truncate max-w-[200px]">{auth.currentOrg.orgName}</span>
						{/if}

						{#if auth.canAccessSetup}
							{#if currentEntitySetupLink}
								<a
									href={currentEntitySetupLink}
									class="crm-nav-icon w-9 h-9 rounded-full flex items-center justify-center text-gray-500 hover:text-gray-700 transition-colors"
									title="Edit Object"
								>
									<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
									</svg>
								</a>
							{/if}
							<a
								href="/admin"
								class="crm-nav-icon w-9 h-9 rounded-full flex items-center justify-center text-gray-500 hover:text-gray-700 transition-colors"
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
								class="flex items-center p-0.5 rounded-full hover:opacity-80 transition-opacity"
							>
								<div class="w-9 h-9 rounded-full bg-blue-600 ring-2 ring-white shadow-sm flex items-center justify-center text-white text-sm font-medium">
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

		<main class="relative z-[1] w-full px-6 lg:px-8 py-6">
			<div class="crm-card p-6">
				{@render children()}
			</div>
		</main>
	</div>
{:else}
	<!-- Not authenticated, not on auth page - render children (root page shows login) -->
	{@render children()}
{/if}

<Toast />
