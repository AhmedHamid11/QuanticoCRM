<script lang="ts">
	import type { SectionCardV3 } from '$lib/types/layout';
	import type { FieldDef } from '$lib/types/admin';
	import type { RelatedListConfig } from '$lib/types/related-list';
	import CardRenderer from './CardRenderer.svelte';

	interface Props {
		cards: SectionCardV3[];
		fieldDefs: FieldDef[];
		record: Record<string, unknown>;
		formatValue: (fieldName: string, value: unknown) => string;
		renderLink?: (fieldName: string, value: unknown) => { href: string; text: string } | null;
		entityName?: string;
		recordId?: string;
		onRecordUpdate?: (updatedRecord: Record<string, unknown>) => void;
		relatedListConfigs?: RelatedListConfig[];
	}

	let { cards, fieldDefs, record, formatValue, renderLink, entityName, recordId, onRecordUpdate, relatedListConfigs }: Props = $props();

	let sortedCards = $derived([...cards].sort((a, b) => a.order - b.order));
</script>

{#each sortedCards as card (card.id)}
	<div class="crm-card overflow-hidden">
		<CardRenderer
			{card}
			fields={fieldDefs}
			{record}
			{formatValue}
			{renderLink}
			{entityName}
			{recordId}
			{onRecordUpdate}
			{relatedListConfigs}
		/>
	</div>
{/each}
