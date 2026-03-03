<script lang="ts">
	import type { FieldDef } from '$lib/types/admin';
	import { getRecordValue } from '$lib/utils/fieldMapping';

	interface Props {
		headerFields: string[];
		fieldDefs: FieldDef[];
		record: Record<string, unknown>;
		formatValue: (fieldName: string, value: unknown) => string;
		renderLink?: (fieldName: string, value: unknown) => { href: string; text: string } | null;
	}

	let { headerFields, fieldDefs, record, formatValue, renderLink }: Props = $props();
</script>

{#if headerFields.length > 0}
	<div class="crm-card px-6 py-3">
		<dl class="flex flex-wrap gap-6">
			{#each headerFields as fieldName (fieldName)}
				{@const fieldDef = fieldDefs.find((f) => f.name === fieldName)}
				{@const rawValue = getRecordValue(record, fieldName)}
				{@const link = renderLink ? renderLink(fieldName, rawValue) : null}
				<div class="flex min-w-0 flex-col">
					<dt class="text-xs font-medium uppercase tracking-wide text-gray-500">
						{fieldDef?.label ?? fieldName}
					</dt>
					<dd class="mt-0.5 truncate text-sm text-gray-900">
						{#if link}
							<a href={link.href} class="text-blue-600 hover:underline">{link.text}</a>
						{:else}
							{formatValue(fieldName, rawValue)}
						{/if}
					</dd>
				</div>
			{/each}
		</dl>
	</div>
{/if}
