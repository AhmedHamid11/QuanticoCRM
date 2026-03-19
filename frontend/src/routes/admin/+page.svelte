<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte';
	import { post } from '$lib/utils/api';
	import { addToast } from '$lib/stores/toast.svelte';

	let isReprovisioning = $state(false);
	let showRepairModal = $state(false);
	let searchQuery = $state('');

	type Tile = {
		title: string;
		description: string;
		href: string;
		borderColor: string;
		iconColor: string;
		iconPath: string;
		iconPath2?: string;
		section: string;
		isButton?: boolean;
		badge?: string;
		platformOnly?: boolean;
	};

	const tiles: Tile[] = [
		// Customization
		{ title: 'Entity Manager', description: 'View and manage entities, their fields, and layouts', href: '/admin/entity-manager', borderColor: 'border-blue-500', iconColor: 'text-blue-600', iconPath: 'M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10', section: 'Customization' },
		{ title: 'Navigation', description: 'Configure toolbar tabs and their order', href: '/admin/navigation', borderColor: 'border-purple-500', iconColor: 'text-purple-500', iconPath: 'M4 6h16M4 12h16M4 18h7', section: 'Customization' },
		{ title: 'Org Settings', description: 'Configure homepage and organization settings', href: '/admin/settings', borderColor: 'border-gray-500', iconColor: 'text-gray-500', iconPath: 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z', iconPath2: 'M15 12a3 3 0 11-6 0 3 3 0 016 0z', section: 'Customization' },
		{ title: 'Custom Pages', description: 'Create and manage custom pages with iframes and components', href: '/admin/pages', borderColor: 'border-cyan-500', iconColor: 'text-cyan-500', iconPath: 'M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z', section: 'Customization' },
		{ title: 'PDF Templates', description: 'Customize PDF output for quotes and documents', href: '/admin/pdf-templates', borderColor: 'border-red-500', iconColor: 'text-red-500', iconPath: 'M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z', section: 'Customization' },
		// Engagement
		{ title: 'Sequences', description: 'Build multi-step outreach sequences for contacts', href: '/admin/engagement/sequences', borderColor: 'border-blue-500', iconColor: 'text-blue-500', iconPath: 'M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01', section: 'Engagement' },
		{ title: 'Email Templates', description: 'Create and manage reusable email templates for sequences', href: '/admin/engagement/templates', borderColor: 'border-indigo-500', iconColor: 'text-indigo-500', iconPath: 'M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z', section: 'Engagement' },
		{ title: 'Task Inbox', description: 'View and manage your daily engagement tasks (calls, LinkedIn, custom)', href: '/engagement/tasks', borderColor: 'border-green-500', iconColor: 'text-green-500', iconPath: 'M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4', section: 'Engagement' },
		// Automation
		{ title: 'Tripwires', description: 'Configure webhook triggers for entity events', href: '/admin/tripwires', borderColor: 'border-orange-500', iconColor: 'text-orange-500', iconPath: 'M13 10V3L4 14h7v7l9-11h-7z', section: 'Automation' },
		{ title: 'Screen Flows', description: 'Create interactive wizards with screens, decisions, and actions', href: '/admin/flows', borderColor: 'border-emerald-500', iconColor: 'text-emerald-500', iconPath: 'M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z', iconPath2: 'M21 12a9 9 0 11-18 0 9 9 0 0118 0z', section: 'Automation' },
		{ title: 'Integrations', description: 'Connect external systems like Salesforce for data sync', href: '/admin/integrations', borderColor: 'border-blue-600', iconColor: 'text-blue-600', iconPath: 'M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z', section: 'Automation' },
		{ title: 'Data Mirror', description: 'Continuous sync from external systems via API — use for ongoing automated data feeds', href: '/admin/mirrors', borderColor: 'border-violet-500', iconColor: 'text-violet-500', iconPath: 'M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4', section: 'Data' },
		// System
		{ title: 'User Management', description: 'Manage users and their roles in your organization', href: '/admin/users', borderColor: 'border-green-500', iconColor: 'text-green-500', iconPath: 'M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z', section: 'System' },
		{ title: 'API Tokens', description: 'Generate tokens for programmatic API access', href: '/admin/api-tokens', borderColor: 'border-indigo-500', iconColor: 'text-indigo-500', iconPath: 'M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z', section: 'System' },
		{ title: 'Data Import', description: 'Upload records from CSV files — use for one-time bulk imports and backfills', href: '/admin/import', borderColor: 'border-sky-500', iconColor: 'text-sky-500', iconPath: 'M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12', section: 'Data' },
		{ title: 'Data Explorer', description: 'Query database and export data', href: '/admin/data-explorer', borderColor: 'border-teal-500', iconColor: 'text-teal-500', iconPath: 'M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4', section: 'Data' },
		{ title: 'Data Quality', description: 'Manage duplicate detection rules, review queue, and merge operations', href: '/admin/data-quality', borderColor: 'border-emerald-500', iconColor: 'text-emerald-500', iconPath: 'M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z', section: 'Data' },
		{ title: 'Changelog', description: 'View platform changes and updates by version', href: '/admin/changelog', borderColor: 'border-slate-500', iconColor: 'text-slate-500', iconPath: 'M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01', section: 'System' },
		{ title: 'Audit Logs', description: 'View security events and compliance audit trail', href: '/admin/audit-logs', borderColor: 'border-yellow-500', iconColor: 'text-yellow-600', iconPath: 'M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z', section: 'System' },
		{ title: 'Repair Metadata', description: 'Re-create default entities, fields, layouts if missing or corrupted', href: '', borderColor: 'border-rose-500', iconColor: 'text-rose-500', iconPath: 'M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15', section: 'System', isButton: true },
		// Platform Administration
		{ title: 'Platform Console', description: 'View all organizations and impersonate customers', href: '/admin/platform', borderColor: 'border-purple-600', iconColor: 'text-purple-600', iconPath: 'M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4', section: 'Platform Administration', badge: 'Admin', platformOnly: true },
	];

	const sectionOrder = ['Customization', 'Engagement', 'Data', 'Automation', 'System', 'Platform Administration'];

	let filteredTiles = $derived.by(() => {
		const q = searchQuery.toLowerCase().trim();
		if (!q) return tiles;
		return tiles.filter(t =>
			t.title.toLowerCase().includes(q) ||
			t.description.toLowerCase().includes(q) ||
			t.section.toLowerCase().includes(q)
		);
	});

	let visibleSections = $derived.by(() => {
		const sections = new Set(filteredTiles.map(t => t.section));
		return sectionOrder.filter(s => sections.has(s));
	});

	function tilesForSection(section: string) {
		return filteredTiles.filter(t => t.section === section);
	}

	function openRepairModal() {
		showRepairModal = true;
	}

	function closeRepairModal() {
		showRepairModal = false;
	}

	async function reprovisionMetadata() {
		showRepairModal = false;
		isReprovisioning = true;
		try {
			const result = await post<{ success: boolean; message: string }>('/admin/reprovision', {});
			if (result.success) {
				addToast(result.message, 'success');
			} else {
				addToast('Reprovisioning completed', 'success');
			}
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to reprovision metadata';
			addToast(message, 'error');
		} finally {
			isReprovisioning = false;
		}
	}
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between gap-4">
		<h1 class="text-2xl font-bold text-gray-900">Administration</h1>
	</div>

	<div class="relative">
		<svg class="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-400 pointer-events-none" fill="none" viewBox="0 0 24 24" stroke="currentColor">
			<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
		</svg>
		<input
			type="text"
			bind:value={searchQuery}
			placeholder="Search admin tools..."
			class="w-full pl-10 pr-4 py-2.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 bg-white"
		/>
		{#if searchQuery}
			<button
				onclick={() => searchQuery = ''}
				class="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600"
			>
				<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
				</svg>
			</button>
		{/if}
	</div>

	{#if filteredTiles.length === 0}
		<p class="text-center text-gray-500 py-8">No admin tools match "{searchQuery}"</p>
	{:else}
		<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
			{#each visibleSections as section}
				{@const sectionTiles = tilesForSection(section)}
				{#if section !== 'Platform Administration' || (auth.isPlatformAdmin && !auth.isImpersonation)}
					<div class="col-span-full" class:mt-6={section !== visibleSections[0]}>
						<h2 class="text-lg font-medium text-gray-700 mb-4">{section}</h2>
					</div>

					{#each sectionTiles as tile}
						{#if tile.platformOnly && !(auth.isPlatformAdmin && !auth.isImpersonation)}
							<!-- skip -->
						{:else if tile.isButton}
							<button
								onclick={openRepairModal}
								disabled={isReprovisioning}
								class="bg-white shadow rounded-lg p-6 hover:shadow-md transition-shadow border-l-4 {tile.borderColor} text-left w-full disabled:opacity-50 disabled:cursor-not-allowed"
							>
								<div class="flex items-start">
									<div class="flex-shrink-0">
										<svg class="h-8 w-8 {tile.iconColor}" fill="none" viewBox="0 0 24 24" stroke="currentColor">
											<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={tile.iconPath} />
										</svg>
									</div>
									<div class="ml-4">
										<h3 class="text-lg font-medium text-gray-900">
											{isReprovisioning ? 'Repairing...' : tile.title}
										</h3>
										<p class="mt-1 text-sm text-gray-500">{tile.description}</p>
									</div>
								</div>
							</button>
						{:else}
							<a
								href={tile.href}
								class="bg-white shadow rounded-lg p-6 hover:shadow-md transition-shadow border-l-4 {tile.borderColor}"
							>
								<div class="flex items-start">
									<div class="flex-shrink-0">
										<svg class="h-8 w-8 {tile.iconColor}" fill="none" viewBox="0 0 24 24" stroke="currentColor">
											<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={tile.iconPath} />
											{#if tile.iconPath2}
												<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={tile.iconPath2} />
											{/if}
										</svg>
									</div>
									<div class="ml-4">
										{#if tile.badge}
											<div class="flex items-center">
												<h3 class="text-lg font-medium text-gray-900">{tile.title}</h3>
												<span class="ml-2 inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-purple-100 text-purple-800">
													{tile.badge}
												</span>
											</div>
										{:else}
											<h3 class="text-lg font-medium text-gray-900">{tile.title}</h3>
										{/if}
										<p class="mt-1 text-sm text-gray-500">{tile.description}</p>
									</div>
								</div>
							</a>
						{/if}
					{/each}
				{/if}
			{/each}
		</div>
	{/if}
</div>

<!-- Repair Metadata Confirmation Modal -->
{#if showRepairModal}
	<div class="fixed inset-0 z-50 overflow-y-auto">
		<div class="flex min-h-full items-end justify-center p-4 text-center sm:items-center sm:p-0">
			<!-- Background overlay -->
			<div
				class="fixed inset-0 bg-gray-500 bg-opacity-75 transition-opacity"
				onclick={closeRepairModal}
			></div>

			<!-- Modal panel -->
			<div class="relative transform overflow-hidden rounded-lg bg-white text-left shadow-xl transition-all sm:my-8 sm:w-full sm:max-w-lg">
				<div class="bg-white px-4 pb-4 pt-5 sm:p-6 sm:pb-4">
					<div class="sm:flex sm:items-start">
						<div class="mx-auto flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-full bg-rose-100 sm:mx-0 sm:h-10 sm:w-10">
							<svg class="h-6 w-6 text-rose-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
							</svg>
						</div>
						<div class="mt-3 text-center sm:ml-4 sm:mt-0 sm:text-left">
							<h3 class="text-lg font-semibold leading-6 text-gray-900">
								Repair Metadata
							</h3>
							<div class="mt-2">
								<p class="text-sm text-gray-500">
									This will re-create default entities, fields, layouts, and navigation for your organization. Existing customizations will be preserved.
								</p>
							</div>
						</div>
					</div>
				</div>
				<div class="bg-gray-50 px-4 py-3 sm:flex sm:flex-row-reverse sm:px-6 gap-2">
					<button
						onclick={reprovisionMetadata}
						class="inline-flex w-full justify-center rounded-md bg-rose-600 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-rose-500 sm:w-auto"
					>
						Repair
					</button>
					<button
						onclick={closeRepairModal}
						class="mt-3 inline-flex w-full justify-center rounded-md bg-white px-3 py-2 text-sm font-semibold text-gray-900 shadow-sm ring-1 ring-inset ring-gray-300 hover:bg-gray-50 sm:mt-0 sm:w-auto"
					>
						Cancel
					</button>
				</div>
			</div>
		</div>
	</div>
{/if}
