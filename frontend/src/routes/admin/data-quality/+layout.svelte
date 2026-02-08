<script lang="ts">
	import { page } from '$app/stores';

	const tabs = [
		{ id: 'rules', label: 'Duplicate Rules', href: '/admin/data-quality/duplicate-rules' },
		{ id: 'queue', label: 'Review Queue', href: '/admin/data-quality/review-queue' },
		{ id: 'history', label: 'Merge History', href: '/admin/data-quality/merge-history' },
		{ id: 'scans', label: 'Scan Jobs', href: '/admin/data-quality/scan-jobs' }
	];

	let currentPath = $derived($page.url.pathname);

	function isActive(href: string): boolean {
		return currentPath === href || currentPath.startsWith(href + '/');
	}
</script>

<div class="space-y-6">
	<!-- Page Header -->
	<div>
		<h1 class="text-2xl font-bold text-gray-900">Data Quality</h1>
		<p class="mt-1 text-sm text-gray-500">
			Manage duplicate detection rules, review queue, and merge operations
		</p>
	</div>

	<!-- Tab Navigation -->
	<div class="bg-white shadow rounded-lg">
		<div class="border-b border-gray-200">
			<nav class="-mb-px flex space-x-8 px-6" aria-label="Tabs">
				{#each tabs as tab}
					<a
						href={tab.href}
						class="border-b-2 py-4 px-1 text-sm font-medium transition-colors {isActive(tab.href)
							? 'border-blue-500 text-blue-600'
							: 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'}"
					>
						{tab.label}
					</a>
				{/each}
			</nav>
		</div>

		<!-- Content Area -->
		<div class="p-6">
			<slot />
		</div>
	</div>
</div>
