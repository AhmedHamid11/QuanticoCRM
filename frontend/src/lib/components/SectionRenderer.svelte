<script lang="ts">
	import type { FieldDef } from '$lib/types/admin';
	import type { LayoutSectionV2 } from '$lib/types/layout';
	import { evaluateVisibility } from '$lib/types/layout';
	import CardRenderer from './CardRenderer.svelte';
	import type { RelatedListConfig } from '$lib/types/related-list';

	interface Props {
		section: LayoutSectionV2;
		fields: FieldDef[];
		record: Record<string, unknown>;
		formatValue: (fieldName: string, value: unknown) => string;
		renderLink?: (fieldName: string, value: unknown) => { href: string; text: string } | null;
		entityName?: string;
		recordId?: string;
		onRecordUpdate?: (updatedRecord: Record<string, unknown>) => void;
		relatedListConfigs?: RelatedListConfig[];
	}

	let { section, fields, record, formatValue, renderLink, entityName, recordId, onRecordUpdate, relatedListConfigs }: Props = $props();

	// Track collapsed state
	let isCollapsed = $state(section.collapsed);

	// Section-level visibility
	let isSectionVisible = $derived(evaluateVisibility(section.visibility, record));

	// Sort cards by order
	let sortedCards = $derived(
		[...(section.cards ?? [])].sort((a, b) => a.order - b.order)
	);

	// Group cards by column for multi-column rendering
	let columnGroups = $derived(() => {
		const cols = section.columns ?? 1;
		const groups: typeof sortedCards[] = [];
		for (let i = 1; i <= cols; i++) {
			groups.push(sortedCards.filter(c => (c.column ?? 1) === i));
		}
		return groups;
	});

	// Check if section has any visible cards
	let hasVisibleCards = $derived(() => {
		if (sortedCards.length === 0) return false;
		return sortedCards.some((card) => {
			if (card.cardType !== 'field') return true;
			if (!card.fields || card.fields.length === 0) return false;
			return card.fields.some((f) => evaluateVisibility(f.visibility, record));
		});
	});

	function toggleCollapse() {
		if (section.collapsible) {
			isCollapsed = !isCollapsed;
		}
	}
</script>

{#if isSectionVisible && hasVisibleCards()}
	<div class="crm-card overflow-hidden">
		<!-- Section Header -->
		<div
			class="px-6 py-4 bg-gray-50/80 border-b border-gray-200/60 flex items-center justify-between {section.collapsible
				? 'cursor-pointer hover:bg-gray-100/80'
				: ''}"
			onclick={toggleCollapse}
			onkeypress={(e) => e.key === 'Enter' && toggleCollapse()}
			role={section.collapsible ? 'button' : undefined}
			tabindex={section.collapsible ? 0 : undefined}
		>
			<h2 class="text-lg font-medium text-gray-900">{section.label}</h2>
			{#if section.collapsible}
				<svg
					class="w-5 h-5 text-gray-400 transition-transform {isCollapsed ? '' : 'rotate-180'}"
					fill="none"
					viewBox="0 0 24 24"
					stroke="currentColor"
				>
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
				</svg>
			{/if}
		</div>

		<!-- Section Content -->
		{#if !isCollapsed}
			{#if section.columns > 1}
				<!-- Multi-column: group cards into column containers -->
				<div class="grid items-start" style="grid-template-columns: repeat({section.columns}, minmax(0, 1fr))">
					{#each columnGroups() as colCards, colIdx}
						<div>
							{#each colCards as card (card.id)}
								<CardRenderer
									{card}
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
						</div>
					{/each}
				</div>
			{:else}
				<!-- Single column: render cards sequentially -->
				{#each sortedCards as card (card.id)}
					<CardRenderer
						{card}
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
			{/if}
		{/if}
	</div>
{/if}
