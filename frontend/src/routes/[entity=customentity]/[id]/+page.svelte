<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { get, del } from '$lib/utils/api';
	import { getEntityNameFromPath } from '$lib/stores/navigation.svelte';
	import { fieldNameToKey, getRecordValue as getRecordValueUtil } from '$lib/utils/fieldMapping';
	import Bearing from '$lib/components/Bearing.svelte';
	import FlowButton from '$lib/components/flow/FlowButton.svelte';
	import RecordDetailLayout from '$lib/components/RecordDetailLayout.svelte';
	import type { EntityDef, FieldDef } from '$lib/types/admin';
	import type { RelatedListConfig } from '$lib/types/related-list';
	import type { BearingWithStages } from '$lib/types/bearing';
	import type { LayoutDataV3, LayoutV3Response } from '$lib/types/layout';
	import { convertV1ToV2 } from '$lib/types/layout';

	let entitySlug = $derived($page.params.entity!);
	let entityName = $derived(getEntityNameFromPath(entitySlug) || toPascalCase(entitySlug));
	let recordId = $derived($page.params.id!);

	function toPascalCase(slug: string): string {
		let singular = slug;
		if (slug.endsWith('s') && slug.length > 1) {
			singular = slug.slice(0, -1);
		}
		return singular.charAt(0).toUpperCase() + singular.slice(1);
	}

	// Get value from record, using centralized utility for snake_case to camelCase conversion
	function getRecordValue(fieldName: string): unknown {
		if (!record) return undefined;
		return getRecordValueUtil(record, fieldName);
	}

	// Get the display name for the record using the entity's configured displayField
	function getDisplayName(): string {
		if (!record) return recordId;
		const df = entityDef?.displayField || 'name';
		const val = getRecordValue(df);
		if (val) return String(val);
		// Fallback: try 'name' field
		if (df !== 'name' && record.name) return String(record.name);
		return recordId;
	}

	let entityDef = $state<EntityDef | null>(null);
	let fields = $state<FieldDef[]>([]);
	let layout = $state<LayoutDataV3 | null>(null);
	let record = $state<Record<string, unknown> | null>(null);
	let relatedListConfigs = $state<RelatedListConfig[]>([]);
	let bearings = $state<BearingWithStages[]>([]);
	let entityFlows = $state<Array<{ id: string; name: string; buttonLabel?: string; refreshOnComplete?: boolean }>>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// Filter to only enabled configs, sorted by sortOrder
	let enabledRelatedLists = $derived(
		relatedListConfigs
			.filter((c) => c.enabled)
			.sort((a, b) => a.sortOrder - b.sortOrder)
	);

	async function loadEntityDef() {
		try {
			// Use public endpoint (doesn't require admin role)
			entityDef = await get<EntityDef>(`/entities/${entityName}/def`);
		} catch {
			entityDef = null;
		}
	}

	async function loadFields() {
		try {
			// Use public endpoint (doesn't require admin role)
			fields = await get<FieldDef[]>(`/entities/${entityName}/fields`);
		} catch {
			fields = [];
		}
	}

	async function loadLayout() {
		// Use public endpoint (doesn't require admin role)
		try {
			const layoutResponse = await get<LayoutV3Response>(
				`/metadata/entities/${entityName}/layouts/detail/v3`
			);
			layout = layoutResponse.layout;
		} catch {
			layout = null;
		}
	}

	async function loadRelatedListConfigs() {
		try {
			relatedListConfigs = await get<RelatedListConfig[]>(`/entities/${entityName}/related-list-configs`);
		} catch {
			relatedListConfigs = [];
		}
	}

	async function loadBearings() {
		try {
			bearings = await get<BearingWithStages[]>(`/entities/${entityName}/bearings`);
		} catch {
			bearings = [];
		}
	}

	async function loadEntityFlows() {
		try {
			const response = await get<{ flows: Array<{ id: string; name: string; buttonLabel?: string }> }>(`/flows/entity/${entityName}`);
			entityFlows = response.flows || [];
		} catch {
			entityFlows = [];
		}
	}

	async function loadRecord() {
		try {
			loading = true;
			error = null;
			record = await get<Record<string, unknown>>(`/entities/${entityName}/records/${recordId}`);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load record';
		} finally {
			loading = false;
		}
	}

	async function deleteRecord() {
		if (!confirm('Are you sure you want to delete this record?')) return;

		try {
			await del(`/entities/${entityName}/records/${recordId}`);
			goto(`/${entitySlug}`);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to delete record';
		}
	}

	function getFieldDef(fieldName: string): FieldDef | undefined {
		return fields.find(f => f.name === fieldName);
	}

	function formatFieldValue(fieldName: string, value: unknown): string {
		if (value === null || value === undefined || value === '') return '-';

		const field = getFieldDef(fieldName);
		if (!field) return String(value);

		switch (field.type) {
			case 'date':
				return new Date(String(value)).toLocaleDateString();
			case 'datetime':
				return new Date(String(value)).toLocaleString();
			case 'bool':
				return value ? 'Yes' : 'No';
			case 'currency':
				return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(Number(value));
			case 'link':
				if (field.linkEntity === 'User' && record) {
					const nameField = fieldName.replace(/Id$/, 'Name');
					const name = getRecordValue(nameField);
					return name ? String(name) : '-';
				}
				return String(value);
			case 'enum':
			case 'multiEnum':
				const strValue = String(value);
				if (strValue.startsWith('[')) {
					try {
						const parsed = JSON.parse(strValue);
						if (Array.isArray(parsed)) {
							return parsed.join(', ');
						}
					} catch {
						// Not valid JSON
					}
				}
				return strValue;
			default:
				return String(value);
		}
	}

	function getLinkInfo(fieldName: string, value: unknown): { href: string; text: string } | null {
		const field = getFieldDef(fieldName);
		if (!field || !record) return null;

		if (field.type === 'link') {
			// User links render as plain text (no clickable link)
			if (field.linkEntity === 'User') return null;
			// Try camelCase versions of the field name for link/name values
			const linkValue = getRecordValue(`${fieldName}Link`);
			const nameValue = getRecordValue(`${fieldName}Name`);
			const idValue = getRecordValue(`${fieldName}Id`) || value;
			if (linkValue && (nameValue || idValue)) {
				return { href: String(linkValue), text: String(nameValue || idValue) };
			}
			// Fallback: construct link from linked entity and ID
			if (idValue && field.linkEntity) {
				const slug = field.linkEntity.toLowerCase() + 's';
				const displayText = nameValue ? String(nameValue) : String(idValue);
				return { href: `/${slug}/${idValue}`, text: displayText };
			}
		} else if (field.type === 'linkMultiple') {
			// Multi-lookup is handled differently - return null here
			return null;
		} else if (field.type === 'email' && value) {
			return { href: `mailto:${value}`, text: String(value) };
		} else if (field.type === 'phone' && value) {
			return { href: `tel:${value}`, text: String(value) };
		} else if (field.type === 'url' && value) {
			return { href: String(value), text: String(value) };
		}

		return null;
	}

	function handleBearingUpdate(fieldName: string, newValue: string) {
		if (record) {
			// Update local state optimistically
			record[fieldName] = newValue;
			record = { ...record }; // Trigger reactivity
		}
	}

	function handleRecordUpdate(updatedRecord: Record<string, unknown>) {
		// Update the local record state with the server response
		record = updatedRecord;
	}

	// Reload all data when entity or record changes (handles navigation between entities)
	$effect(() => {
		// Track these reactive values to trigger reload on navigation
		const _entity = entityName;
		const _recordId = recordId;

		// Reset state — do NOT reset tab state, it's URL-based inside RecordDetailLayout
		entityDef = null;
		fields = [];
		layout = null;
		record = null;
		relatedListConfigs = [];
		bearings = [];
		entityFlows = [];
		loading = true;
		error = null;

		// Load all data
		(async () => {
			await Promise.all([loadEntityDef(), loadFields(), loadLayout(), loadRelatedListConfigs(), loadBearings(), loadEntityFlows()]);
			// If no layout configured, create a minimal V3 fallback
			if (!layout && fields.length > 0) {
				const v2Fallback = convertV1ToV2(fields.filter(f => f.name !== 'id').map(f => f.name));
				layout = {
					version: 3,
					sections: v2Fallback.sections,
					tabs: [{ id: 'tab_overview', label: 'Overview', order: 1, sectionIds: v2Fallback.sections.map(s => s.id) }],
					sidebar: { cards: [] },
					header: { fields: [] },
					conditions: null
				};
			}
			await loadRecord();
		})();
	});
</script>

<div class="space-y-6">
	<!-- Breadcrumb & Actions -->
	<div class="flex justify-between items-start">
		<div>
			<nav class="text-sm text-gray-500 mb-2">
				<a href="/{entitySlug}" class="hover:text-gray-700">{entityDef?.labelPlural || entityName + 's'}</a>
				<span class="mx-2">/</span>
				<span class="text-gray-900">{getDisplayName()}</span>
			</nav>
			<div class="flex items-center gap-3">
				{#if entityDef}
					<div
						class="w-10 h-10 rounded flex items-center justify-center text-white text-lg font-semibold"
						style="background-color: {entityDef.color}"
					>
						{entityDef.label.charAt(0)}
					</div>
				{/if}
				<h1 class="text-2xl font-bold text-gray-900">
					{#if record}
						{getDisplayName()}
					{:else}
						Loading...
					{/if}
				</h1>
			</div>
		</div>
		<div class="flex gap-2">
			<a
				href="/admin/entity-manager/{entityName}"
				class="p-2 text-gray-500 hover:text-gray-700 hover:bg-gray-100 rounded-lg transition-colors"
				title="Entity Settings"
			>
				<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
				</svg>
			</a>
			{#each entityFlows as flow (flow.id)}
				<FlowButton
					flowId={flow.id}
					label={flow.buttonLabel || flow.name}
					entity={entityName}
					recordId={recordId}
					variant="secondary"
					refreshOnComplete={flow.refreshOnComplete}
				/>
			{/each}
			<a
				href="/{entitySlug}/{recordId}/edit"
				class="px-4 py-2 border border-gray-300 rounded-lg hover:bg-gray-50 transition-colors"
			>
				Edit
			</a>
			<button
				onclick={deleteRecord}
				class="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 transition-colors"
			>
				Delete
			</button>
		</div>
	</div>

	{#if loading}
		<div class="text-center py-12 text-gray-500">Loading...</div>
	{:else if error}
		<div class="text-center py-12 text-red-500">{error}</div>
	{:else if record && layout}
		<RecordDetailLayout
			{layout}
			{fields}
			{record}
			entityName={entityName}
			recordId={String(record.id)}
			{relatedListConfigs}
			formatValue={formatFieldValue}
			renderLink={getLinkInfo}
			onRecordUpdate={handleRecordUpdate}
		>
			<!-- Bearings -->
			{#if bearings.length > 0}
				<div class="space-y-4">
					{#each bearings.toSorted((a, b) => a.displayOrder - b.displayOrder) as bearing (bearing.id)}
						<Bearing
							{bearing}
							currentValue={getRecordValue(bearing.sourcePicklist) as string | null}
							recordId={String(record.id)}
							entityType={entityName}
							fieldName={bearing.sourcePicklist}
							onUpdate={(newValue) => handleBearingUpdate(bearing.sourcePicklist, newValue)}
						/>
					{/each}
				</div>
			{/if}

			<!-- System Info -->
			<div class="crm-card overflow-hidden">
				<div class="px-6 py-4 border-b border-gray-200">
					<h2 class="text-lg font-medium text-gray-900">System Information</h2>
				</div>
				<div class="px-6 py-4">
					<dl class="grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-4">
						<div>
							<dt class="text-sm font-medium text-gray-500">Created</dt>
							<dd class="mt-1 text-sm text-gray-900">
								{record.createdAt ? new Date(String(record.createdAt)).toLocaleString() : '-'}
								{#if record.createdByName}
									<span class="text-gray-500"> by {record.createdByName}</span>
								{/if}
							</dd>
						</div>
						<div>
							<dt class="text-sm font-medium text-gray-500">Modified</dt>
							<dd class="mt-1 text-sm text-gray-900">
								{record.modifiedAt ? new Date(String(record.modifiedAt)).toLocaleString() : '-'}
								{#if record.modifiedByName}
									<span class="text-gray-500"> by {record.modifiedByName}</span>
								{/if}
							</dd>
						</div>
						<div>
							<dt class="text-sm font-medium text-gray-500">ID</dt>
							<dd class="mt-1 text-sm text-gray-500 font-mono">{record.id}</dd>
						</div>
					</dl>
				</div>
			</div>
		</RecordDetailLayout>
	{/if}
</div>
