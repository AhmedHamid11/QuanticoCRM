<script lang="ts">
	import type { FieldDef, TextBlockVariant } from '$lib/types/admin';
	import type { SectionCardV3, RelatedListCardConfig, CustomPageCardConfig } from '$lib/types/layout';
	import { fieldNameToKey, getRecordValue } from '$lib/utils/fieldMapping';
	import { put, get, del } from '$lib/utils/api';
	import StreamField from './StreamField.svelte';
	import FieldDisplay from './FieldDisplay.svelte';
	import InlineFieldEditor from './InlineFieldEditor.svelte';
	import ActivityCard from './ActivityCard.svelte';
	import RelatedList from './RelatedList.svelte';
	import { isInlineEditable } from '$lib/utils/fieldFormatters';
	import { toast } from '$lib/stores/toast.svelte';
	import type { RelatedListConfig } from '$lib/types/related-list';
	import { evaluateVisibility } from '$lib/types/layout';

	interface Props {
		card: SectionCardV3;
		fields: FieldDef[];
		record: Record<string, unknown>;
		formatValue: (fieldName: string, value: unknown) => string;
		renderLink?: (fieldName: string, value: unknown) => { href: string; text: string } | null;
		entityName?: string;
		recordId?: string;
		onRecordUpdate?: (updatedRecord: Record<string, unknown>) => void;
		relatedListConfigs?: RelatedListConfig[];
	}

	let { card, fields, record, formatValue, renderLink, entityName, recordId, onRecordUpdate, relatedListConfigs }: Props = $props();

	// Inline edit state
	let editingField = $state<string | null>(null);
	let editingValue = $state<unknown>(null);
	let savedValue = $state<unknown>(null);
	let flashSuccessField = $state<string | null>(null);

	// Visible fields for field cards
	let visibleFields = $derived(
		(card.fields ?? []).filter((f) => evaluateVisibility(f.visibility, record))
	);

	function getFieldDef(fieldName: string): FieldDef | undefined {
		return fields.find((f) => f.name === fieldName);
	}

	function getFieldValue(fieldName: string): unknown {
		return getRecordValue(record, fieldName);
	}

	function startEdit(fieldName: string, currentValue: unknown, field: FieldDef) {
		if (!isInlineEditable(field, fieldName)) return;
		if (!entityName || !recordId) return;
		editingField = fieldName;
		editingValue = currentValue;
		savedValue = currentValue;
	}

	async function commitEdit(fieldName: string, newValue: unknown) {
		if (editingField !== fieldName) return;
		if (!entityName || !recordId) return;

		editingField = null;

		if (newValue === savedValue) return;

		const key = fieldNameToKey(fieldName);
		const updateData: Record<string, unknown> = { [key]: newValue };

		try {
			await put<Record<string, unknown>>(
				`/entities/${entityName}/records/${recordId}`,
				updateData
			);

			const fullRecord = await get<Record<string, unknown>>(
				`/entities/${entityName}/records/${recordId}`
			);

			if (onRecordUpdate) {
				onRecordUpdate(fullRecord);
			}

			flashSuccessField = fieldName;
			setTimeout(() => { flashSuccessField = null; }, 800);
		} catch (e) {
			try {
				const fullRecord = await get<Record<string, unknown>>(
					`/entities/${entityName}/records/${recordId}`
				);
				if (onRecordUpdate) {
					onRecordUpdate(fullRecord);
				}
			} catch {
				// If refetch also fails, just leave it
			}
			toast.error(e instanceof Error ? e.message : 'Save failed');
		}
	}

	function cancelEdit() {
		editingField = null;
		editingValue = null;
	}

	async function toggleBool(fieldName: string, currentValue: unknown) {
		if (!entityName || !recordId) return;
		const newValue = !toBoolValue(currentValue);
		const key = fieldNameToKey(fieldName);

		try {
			await put<Record<string, unknown>>(
				`/entities/${entityName}/records/${recordId}`,
				{ [key]: newValue }
			);
			const fullRecord = await get<Record<string, unknown>>(
				`/entities/${entityName}/records/${recordId}`
			);
			if (onRecordUpdate) onRecordUpdate(fullRecord);
			flashSuccessField = fieldName;
			setTimeout(() => { flashSuccessField = null; }, 800);
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Save failed');
		}
	}

	function toBoolValue(v: unknown): boolean {
		if (typeof v === 'boolean') return v;
		if (typeof v === 'number') return v !== 0;
		if (typeof v === 'string') return v === 'true' || v === '1' || v === 'yes';
		return false;
	}

	function interpolateContent(content: string, rec: Record<string, unknown>): string {
		return content.replace(/\{\{(\w+)\}\}/g, (match, fieldName) => {
			const value = getRecordValue(rec, fieldName);
			if (value === null || value === undefined) return '';
			return String(value);
		});
	}

	function getTextBlockClasses(variant: TextBlockVariant | null | undefined): string {
		switch (variant) {
			case 'warning':
				return 'bg-amber-50 border-amber-200 text-amber-800';
			case 'error':
				return 'bg-red-50 border-red-200 text-red-800';
			case 'success':
				return 'bg-green-50 border-green-200 text-green-800';
			case 'info':
			default:
				return 'bg-blue-50 border-blue-200 text-blue-800';
		}
	}

	function getTextBlockIcon(variant: TextBlockVariant | null | undefined): string {
		switch (variant) {
			case 'warning':
				return 'M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z';
			case 'error':
				return 'M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z';
			case 'success':
				return 'M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z';
			case 'info':
			default:
				return 'M11.25 11.25l.041-.02a.75.75 0 011.063.852l-.708 2.836a.75.75 0 001.063.853l.041-.021M21 12a9 9 0 11-18 0 9 9 0 0118 0zm-9-3.75h.008v.008H12V8.25z';
		}
	}

	function createStreamSubmitHandler(fieldName: string): ((entry: string) => Promise<void>) | undefined {
		if (!entityName || !recordId) return undefined;

		return async (entry: string) => {
			const updateData: Record<string, unknown> = {
				[fieldName]: entry
			};

			await put<Record<string, unknown>>(
				`/entities/${entityName}/records/${recordId}`,
				updateData
			);

			const fullRecord = await get<Record<string, unknown>>(
				`/entities/${entityName}/records/${recordId}`
			);

			if (onRecordUpdate) {
				onRecordUpdate(fullRecord);
			}
		};
	}

	function createStreamDeleteHandler(fieldName: string): ((entryIndex: number) => Promise<void>) | undefined {
		if (!entityName || !recordId) return undefined;

		return async (entryIndex: number) => {
			await del(`/entities/${entityName}/records/${recordId}/stream/${fieldName}?index=${entryIndex}`);

			const fullRecord = await get<Record<string, unknown>>(
				`/entities/${entityName}/records/${recordId}`
			);

			if (onRecordUpdate) {
				onRecordUpdate(fullRecord);
			}
		};
	}
</script>

{#if card.cardType === 'activity'}
	<!-- Activity Card -->
	{#if entityName && recordId}
		<ActivityCard {entityName} {recordId} />
	{:else}
		<div class="p-6 text-sm text-gray-400">Activity card requires a saved record.</div>
	{/if}
{:else if card.cardType === 'relatedList'}
	<!-- Related List Card -->
	{@const rlConfig = relatedListConfigs?.find(c => c.id === (card.cardConfig as RelatedListCardConfig)?.relatedListConfigId)}
	{#if rlConfig && entityName && recordId}
		<div class="p-4">
			<RelatedList config={rlConfig} parentEntity={entityName} parentId={recordId} />
		</div>
	{:else}
		<div class="p-6 text-sm text-gray-400">Related list not configured or record not saved.</div>
	{/if}
{:else if card.cardType === 'customPage'}
	<!-- Custom Page Card (iframe or HTML) -->
	{@const cpConfig = card.cardConfig as CustomPageCardConfig}
	{#if cpConfig?.mode === 'iframe' && cpConfig.url}
		{@const interpolationContext = { ...record, recordId: recordId ?? '', entityName: entityName ?? '' }}
		{@const interpolatedUrl = interpolateContent(cpConfig.url, interpolationContext)}
		<div class="p-0">
			<iframe
				src={interpolatedUrl}
				style="width: 100%; height: {cpConfig.height ?? 400}px; border: 0;"
				sandbox="allow-scripts allow-same-origin allow-forms allow-popups"
				title={card.label ?? 'Custom Page'}
				loading="lazy"
			></iframe>
		</div>
	{:else if cpConfig?.mode === 'html' && cpConfig.content}
		<div class="p-4">
			{@html cpConfig.content}
		</div>
	{:else}
		<div class="p-6 text-sm text-gray-400">Custom page not configured.</div>
	{/if}
{:else}
	<!-- Default: Field Card -->
	{#if visibleFields.length > 0}
		<div class="p-6">
			<dl
				class="grid gap-x-8 gap-y-4"
				style="grid-template-columns: repeat({card.columns ?? 2}, minmax(0, 1fr))"
			>
				{#each visibleFields as fieldLayout (fieldLayout.name)}
					{@const field = getFieldDef(fieldLayout.name)}
					{@const value = getFieldValue(fieldLayout.name)}
					{#if field}
						{#if field.type === 'textBlock'}
							<!-- Text Block -->
							<div class="col-span-full">
								<div class="rounded-md border p-4 {getTextBlockClasses(field.variant)}">
									<div class="flex">
										<div class="flex-shrink-0">
											<svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
												<path stroke-linecap="round" stroke-linejoin="round" d={getTextBlockIcon(field.variant)} />
											</svg>
										</div>
										<div class="ml-3">
											{#if field.label}
												<h3 class="text-sm font-medium">{field.label}</h3>
											{/if}
											{#if field.content}
												<div class="text-sm {field.label ? 'mt-1' : ''}">
													{interpolateContent(field.content, record)}
												</div>
											{/if}
										</div>
									</div>
								</div>
							</div>
						{:else if field.type === 'stream'}
							<!-- Stream field -->
							{@const logValue = getFieldValue(fieldLayout.name + 'Log')}
							{@const logStr = logValue ? String(logValue) : ''}
							<div class="col-span-full">
								<StreamField
									label={field.label}
									entry=""
									log={logStr}
									readonly={true}
									onsubmit={createStreamSubmitHandler(field.name)}
									ondelete={createStreamDeleteHandler(field.name)}
								/>
							</div>
						{:else}
							<!-- Regular field with inline edit -->
							{@const editable = isInlineEditable(field, fieldLayout.name)}
							{@const isFlashing = flashSuccessField === fieldLayout.name}
							<div class="grid grid-cols-3 gap-4 rounded px-1 -mx-1 transition-colors duration-700 {isFlashing ? 'bg-green-50 ring-1 ring-green-200' : ''}">
								<dt class="text-sm font-medium text-gray-500">{field.label}</dt>
								<dd class="col-span-2 text-sm text-gray-900">
									{#if editingField === fieldLayout.name}
										<InlineFieldEditor
											{field}
											value={editingValue}
											oncommit={(newVal) => commitEdit(fieldLayout.name, newVal)}
											oncancel={cancelEdit}
										/>
									{:else if field.type === 'bool' && editable && entityName && recordId}
										<FieldDisplay
											{field}
											{value}
											{renderLink}
											isEditable={true}
											onclick={() => toggleBool(fieldLayout.name, value)}
										/>
									{:else}
										<FieldDisplay
											{field}
											{value}
											{renderLink}
											isEditable={editable && !!entityName && !!recordId}
											onclick={() => startEdit(fieldLayout.name, value, field)}
										/>
									{/if}
								</dd>
							</div>
						{/if}
					{/if}
				{/each}
			</dl>
		</div>
	{/if}
{/if}
