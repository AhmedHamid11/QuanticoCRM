<script lang="ts">
	import type { FieldDef, TextBlockVariant } from '$lib/types/admin';
	import type { LayoutSectionV2 } from '$lib/types/layout';
	import { evaluateVisibility } from '$lib/types/layout';
	import { fieldNameToKey, getRecordValue } from '$lib/utils/fieldMapping';
	import { put, get, del } from '$lib/utils/api';
	import StreamField from './StreamField.svelte';
	import FieldDisplay from './FieldDisplay.svelte';
	import InlineFieldEditor from './InlineFieldEditor.svelte';
	import { isInlineEditable } from '$lib/utils/fieldFormatters';
	import { toast } from '$lib/stores/toast.svelte';

	interface Props {
		section: LayoutSectionV2;
		fields: FieldDef[];
		record: Record<string, unknown>;
		formatValue: (fieldName: string, value: unknown) => string;
		renderLink?: (fieldName: string, value: unknown) => { href: string; text: string } | null;
		entityName?: string;
		recordId?: string;
		onRecordUpdate?: (updatedRecord: Record<string, unknown>) => void;
	}

	let { section, fields, record, formatValue, renderLink, entityName, recordId, onRecordUpdate }: Props = $props();

	// Track collapsed state (starts with section default)
	let isCollapsed = $state(section.collapsed);

	// Inline edit state — only one field edits at a time
	let editingField = $state<string | null>(null);
	let editingValue = $state<unknown>(null);
	let savedValue = $state<unknown>(null); // backup for revert on failure
	let flashSuccessField = $state<string | null>(null); // for green flash

	// Determine if section should be visible based on record data
	let isSectionVisible = $derived(evaluateVisibility(section.visibility, record));

	// Get visible fields within the section
	let visibleFields = $derived(
		section.fields.filter((f) => evaluateVisibility(f.visibility, record))
	);

	function getFieldDef(fieldName: string): FieldDef | undefined {
		return fields.find((f) => f.name === fieldName);
	}

	function getFieldValue(fieldName: string): unknown {
		// Use centralized utility to handle snake_case to camelCase conversion
		return getRecordValue(record, fieldName);
	}

	function toggleCollapse() {
		if (section.collapsible) {
			isCollapsed = !isCollapsed;
		}
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

		// Skip save if value unchanged
		if (newValue === savedValue) return;

		// Build the PUT payload using camelCase key
		const key = fieldNameToKey(fieldName);
		const updateData: Record<string, unknown> = { [key]: newValue };

		try {
			await put<Record<string, unknown>>(
				`/entities/${entityName}/records/${recordId}`,
				updateData
			);

			// Refetch full record to get server-computed values (modifiedAt, etc.)
			const fullRecord = await get<Record<string, unknown>>(
				`/entities/${entityName}/records/${recordId}`
			);

			if (onRecordUpdate) {
				onRecordUpdate(fullRecord);
			}

			// Show success flash
			flashSuccessField = fieldName;
			setTimeout(() => { flashSuccessField = null; }, 800);
		} catch (e) {
			// Revert: refetch the full record to restore old value
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

	// Special handler for boolean toggle (save immediately, no edit mode)
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

	// Interpolate {{fieldName}} placeholders in text with record values
	function interpolateContent(content: string, record: Record<string, unknown>): string {
		return content.replace(/\{\{(\w+)\}\}/g, (match, fieldName) => {
			// Use centralized utility to handle snake_case to camelCase conversion
			const value = getRecordValue(record, fieldName);
			if (value === null || value === undefined) return '';
			return String(value);
		});
	}

	// Get variant-specific CSS classes for textBlock
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

	// Get icon for textBlock variant
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

	// Create a handler for inline stream entry submission
	function createStreamSubmitHandler(fieldName: string): ((entry: string) => Promise<void>) | undefined {
		if (!entityName || !recordId) return undefined;

		return async (entry: string) => {
			// Send just the stream entry field to the API
			// The backend will append it to the log and clear the entry
			const updateData: Record<string, unknown> = {
				[fieldName]: entry
			};

			await put<Record<string, unknown>>(
				`/entities/${entityName}/records/${recordId}`,
				updateData
			);

			// Fetch the full record to get all fields (PUT only returns changed fields)
			const fullRecord = await get<Record<string, unknown>>(
				`/entities/${entityName}/records/${recordId}`
			);

			// Notify parent of the update so it can refresh the record state
			if (onRecordUpdate) {
				onRecordUpdate(fullRecord);
			}
		};
	}

	// Create a handler for deleting a stream entry
	function createStreamDeleteHandler(fieldName: string): ((entryIndex: number) => Promise<void>) | undefined {
		if (!entityName || !recordId) return undefined;

		return async (entryIndex: number) => {
			// Call the delete endpoint
			await del(`/entities/${entityName}/records/${recordId}/stream/${fieldName}?index=${entryIndex}`);

			// Fetch the full record to refresh the log
			const fullRecord = await get<Record<string, unknown>>(
				`/entities/${entityName}/records/${recordId}`
			);

			// Notify parent of the update
			if (onRecordUpdate) {
				onRecordUpdate(fullRecord);
			}
		};
	}
</script>

{#if isSectionVisible && visibleFields.length > 0}
	<div class="bg-white shadow rounded-lg overflow-hidden">
		<!-- Section Header -->
		<div
			class="px-6 py-4 bg-gray-50 border-b border-gray-200 flex items-center justify-between {section.collapsible
				? 'cursor-pointer hover:bg-gray-100'
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
			<div class="p-6">
				<dl
					class="grid gap-x-8 gap-y-4"
					style="grid-template-columns: repeat({section.columns}, minmax(0, 1fr))"
				>
					{#each visibleFields as fieldLayout (fieldLayout.name)}
						{@const field = getFieldDef(fieldLayout.name)}
						{@const value = getFieldValue(fieldLayout.name)}
						{#if field}
							{#if field.type === 'textBlock'}
								<!-- Text Block - styled message display -->
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
								<!-- Stream field - with inline entry support -->
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
								<!-- Regular field -->
								<div class="grid grid-cols-3 gap-4">
									<dt class="text-sm font-medium text-gray-500">{field.label}</dt>
									<dd class="col-span-2 text-sm text-gray-900">
										{#if renderLink}
											{@const linkInfo = renderLink(field.name, value)}
											{#if linkInfo}
												<a href={linkInfo.href} class="text-blue-600 hover:underline">
													{linkInfo.text}
												</a>
											{:else}
												{formatValue(field.name, value)}
											{/if}
										{:else}
											{formatValue(field.name, value)}
										{/if}
									</dd>
								</div>
							{/if}
						{/if}
					{/each}
				</dl>
			</div>
		{/if}
	</div>
{/if}
