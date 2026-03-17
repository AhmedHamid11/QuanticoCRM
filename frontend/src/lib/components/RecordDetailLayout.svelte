<script lang="ts">
	import type { LayoutDataV3 } from '$lib/types/layout';
	import { getSectionsForTab, migrateLayoutV3 } from '$lib/types/layout';
	import type { FieldDef } from '$lib/types/admin';
	import type { RelatedListConfig } from '$lib/types/related-list';
	import SectionRenderer from './SectionRenderer.svelte';
	import SidebarCardRenderer from './SidebarCardRenderer.svelte';
	import HeaderStrip from './HeaderStrip.svelte';
	import TabBar from './TabBar.svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';

	interface Props {
		layout: LayoutDataV3;
		fields: FieldDef[];
		record: Record<string, unknown>;
		entityName: string;
		recordId: string;
		relatedListConfigs: RelatedListConfig[];
		formatValue: (fieldName: string, value: unknown) => string;
		renderLink?: (fieldName: string, value: unknown) => { href: string; text: string } | null;
		onRecordUpdate?: (record: Record<string, unknown>) => void;
		children?: import('svelte').Snippet;
	}

	let {
		layout: rawLayout,
		fields,
		record,
		entityName,
		recordId,
		relatedListConfigs,
		formatValue,
		renderLink,
		onRecordUpdate,
		children
	}: Props = $props();

	// Migrate layout to multi-card format transparently
	let layout = $derived(migrateLayoutV3(rawLayout));

	let tabs = $derived(
		[...layout.tabs].sort((a, b) => a.order - b.order)
	);

	let activeTabId = $derived(
		$page.url.searchParams.get('tab') || (tabs.length > 0 ? tabs[0].id : 'overview')
	);

	let activeSections = $derived(getSectionsForTab(layout, activeTabId, record));

	let hasSidebar = $derived(layout.sidebar.cards.length > 0);

	function switchTab(tabId: string) {
		const url = new URL($page.url);
		url.searchParams.set('tab', tabId);
		goto(url.pathname + url.search, { replaceState: true, noScroll: true });
	}
</script>

<!-- Header strip (always visible, suppressed when no header fields) -->
{#if layout.header.fields.length > 0}
	<HeaderStrip
		headerFields={layout.header.fields}
		fieldDefs={fields}
		{record}
		{formatValue}
		{renderLink}
	/>
{/if}

<!-- Slot for Bearings, alert banners, system info, etc. -->
{@render children?.()}

<!-- Tab bar -->
<TabBar {tabs} {activeTabId} onTabChange={switchTab} />

<!-- Main 2-column grid -->
<div class="grid grid-cols-1 items-start gap-6 {hasSidebar ? 'lg:grid-cols-[minmax(0,1fr)_280px]' : ''}">
	<!-- Main content area -->
	<div class="space-y-4">
		{#each activeSections as section (section.id)}
			<SectionRenderer
				{section}
				{fields}
				{record}
				{formatValue}
				{renderLink}
				{entityName}
				{recordId}
				{onRecordUpdate}
				{relatedListConfigs}
			/>
		{/each}
		{#if activeSections.length === 0}
			<p class="text-sm text-gray-500">No content configured for this tab.</p>
		{/if}
	</div>

	<!-- Right sidebar (sticky on desktop) -->
	{#if hasSidebar}
		<div class="space-y-4 lg:sticky lg:top-4">
			<SidebarCardRenderer
				cards={layout.sidebar.cards}
				fieldDefs={fields}
				{record}
				{formatValue}
				{renderLink}
				{entityName}
				{recordId}
				{onRecordUpdate}
				{relatedListConfigs}
			/>
		</div>
	{/if}
</div>
